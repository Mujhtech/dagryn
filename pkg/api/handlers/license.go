package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/rs/zerolog/log"
)

// FeatureEntry represents a single feature and whether it is enabled.
type FeatureEntry struct {
	Feature string `json:"feature"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

// LicenseStatusResponse represents the license status returned by the API.
type LicenseStatusResponse struct {
	Mode          string         `json:"mode"`
	Edition       string         `json:"edition"`
	Licensed      bool           `json:"licensed"`
	Customer      string         `json:"customer,omitempty"`
	Seats         int            `json:"seats"`
	Features      []FeatureEntry `json:"features"`
	Limits        LicenseUsage   `json:"limits"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	DaysRemaining *int           `json:"days_remaining,omitempty"`
	GracePeriod   bool           `json:"grace_period"`
	Expiring      bool           `json:"expiring"`
}

// LicenseUsage holds resource usage entries for the license.
type LicenseUsage struct {
	Projects       UsageEntry `json:"projects"`
	TeamMembers    UsageEntry `json:"team_members"`
	ConcurrentRuns UsageEntry `json:"concurrent_runs"`
}

// UsageEntry holds a resource's current usage and its licensed limit.
type UsageEntry struct {
	Current int64  `json:"current"`
	Limit   *int64 `json:"limit"` // null = unlimited
}

// GetLicenseStatus godoc
//
//	@Summary		Get license status
//	@Description	Returns the current license status including features and limits
//	@Tags			license
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Success		200	{object}	LicenseStatusResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/v1/license [get]
func (h *Handler) GetLicenseStatus(w http.ResponseWriter, r *http.Request) {
	if h.entitlements != nil && h.entitlements.Mode() == "cloud" {
		_ = response.Ok(w, r, "License status retrieved", LicenseStatusResponse{
			Mode:     "cloud",
			Edition:  "cloud",
			Licensed: true,
		})
		return
	}

	gate := h.featureGate

	resp := LicenseStatusResponse{
		Mode:     "self_hosted",
		Edition:  string(licensing.EditionCommunity),
		Licensed: false,
		Seats:    3,
		Features: buildFeatureList(gate),
		Limits: LicenseUsage{
			Projects:       UsageEntry{Limit: intPtr(5)},
			TeamMembers:    UsageEntry{Limit: intPtr(3)},
			ConcurrentRuns: UsageEntry{Limit: intPtr(3)},
		},
	}

	if gate != nil {
		resp.Edition = string(gate.Edition())
		resp.Seats = gate.Seats()
		resp.GracePeriod = gate.InGracePeriod()
		resp.Expiring = gate.IsExpiring()

		if claims := gate.Claims(); claims != nil {
			resp.Licensed = true
			resp.Customer = claims.Subject

			expiry := claims.ExpiryTime()
			resp.ExpiresAt = &expiry

			days := claims.DaysUntilExpiry()
			resp.DaysRemaining = &days

			// Limits from claims
			resp.Limits = buildLicenseLimits(claims)
		}
	}

	_ = response.Ok(w, r, "License status retrieved", resp)
}

// ActivateLicenseRequest is the request body for POST /api/v1/license/activate.
type ActivateLicenseRequest struct {
	LicenseKey string `json:"license_key"`
}

// ActivateLicense godoc
//
//	@Summary		Activate a license key
//	@Description	Validates and activates a license key on this instance (self-hosted only)
//	@Tags			license
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ActivateLicenseRequest	true	"License key"
//	@Success		200		{object}	LicenseStatusResponse
//	@Failure		400		{object}	ErrorResponse
//	@Router			/api/v1/license/activate [post]
func (h *Handler) ActivateLicense(w http.ResponseWriter, r *http.Request) {
	// Only available in self_hosted mode
	if h.entitlements != nil && h.entitlements.Mode() == "cloud" {
		_ = response.BadRequest(w, r, errors.New("license activation is not available in cloud mode"))
		return
	}

	var req ActivateLicenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.LicenseKey == "" {
		_ = response.BadRequest(w, r, errors.New("license_key is required"))
		return
	}

	// 1. Validate locally
	keys, err := licensing.ParsePublicKeys()
	if err != nil || len(keys) == 0 {
		_ = response.BadRequest(w, r, errors.New("no license verification keys available in this build"))
		return
	}

	validator := licensing.NewValidator(keys)
	claims, err := validator.Validate(req.LicenseKey)
	if err != nil {
		_ = response.BadRequest(w, r, fmt.Errorf("invalid license key: %w", err))
		return
	}

	// 2. Load or create instance ID
	stored, _ := licensing.LoadStoredLicense()
	if stored.InstanceID == "" {
		stored.InstanceID = "inst_" + uuid.New().String()[:8]
	}
	if stored.InstanceName == "" {
		hostname, _ := os.Hostname()
		stored.InstanceName = hostname
	}

	// 3. Try to register with License Server (non-blocking on failure)
	serverURL := os.Getenv("DAGRYN_LICENSE_SERVER_URL")
	if serverURL == "" {
		serverURL = "https://license.dagryn.dev"
	}

	serverClient := licensing.NewServerClient(licensing.ServerConfig{
		BaseURL: serverURL,
		Timeout: 10 * time.Second,
	})

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, srvErr := serverClient.Activate(ctx, licensing.ActivationRequest{
		LicenseKey:   req.LicenseKey,
		InstanceID:   stored.InstanceID,
		InstanceName: stored.InstanceName,
		Version:      Version,
	})
	if srvErr != nil {
		log.Warn().Err(srvErr).Msg("could not reach License Server during activation")
	} else if !resp.Activated {
		_ = response.BadRequest(w, r, fmt.Errorf("activation failed: %s", resp.Message))
		return
	}

	// 4. Save license locally
	stored.Key = req.LicenseKey
	stored.LicenseID = claims.LicenseID
	stored.ActivatedAt = time.Now()
	if err := licensing.SaveStoredLicense(stored); err != nil {
		_ = response.InternalServerError(w, r, fmt.Errorf("failed to save license: %w", err))
		return
	}

	// 5. Hot-reload FeatureGate.
	// In OSS mode the gate is always non-nil (created by NewLicensingService),
	// so Update() modifies it in place and the entitlement checker — which
	// holds the same pointer — sees the new claims immediately.
	// The else branch is a safety net: if the gate is nil we create one, but
	// note that the LicenseChecker is not re-wired so feature-gated middleware
	// won't see the change until restart.
	gate := h.featureGate
	if gate != nil {
		gate.Update(claims)
	} else {
		gate = licensing.NewFeatureGate(claims, log.Logger)
		h.featureGate = gate
		log.Warn().Msg("FeatureGate was nil during activation — feature-gated middleware may require a server restart")
	}

	// 6. Return updated license status
	statusResp := LicenseStatusResponse{
		Mode:     "self_hosted",
		Edition:  string(gate.Edition()),
		Licensed: true,
		Customer: claims.Subject,
		Seats:    gate.Seats(),
		Features: buildFeatureList(gate),
		Limits:   buildLicenseLimits(claims),
	}

	expiry := claims.ExpiryTime()
	statusResp.ExpiresAt = &expiry
	days := claims.DaysUntilExpiry()
	statusResp.DaysRemaining = &days
	statusResp.GracePeriod = gate.InGracePeriod()
	statusResp.Expiring = gate.IsExpiring()

	_ = response.Ok(w, r, "License activated successfully", statusResp)
}

func buildFeatureList(gate *licensing.FeatureGate) []FeatureEntry {
	entries := make([]FeatureEntry, 0, len(licensing.AllFeatures))
	for _, f := range licensing.AllFeatures {
		entries = append(entries, FeatureEntry{
			Feature: string(f),
			Label:   f.DisplayName(),
			Enabled: gate != nil && gate.HasFeature(f),
		})
	}
	return entries
}

func buildLicenseLimits(claims *licensing.Claims) LicenseUsage {
	usage := LicenseUsage{}
	if claims.Limits.MaxProjects != nil {
		v := int64(*claims.Limits.MaxProjects)
		usage.Projects.Limit = &v
	}
	if claims.Limits.MaxTeamMembers != nil {
		v := int64(*claims.Limits.MaxTeamMembers)
		usage.TeamMembers.Limit = &v
	}
	if claims.Limits.MaxConcurrentRuns != nil {
		v := int64(*claims.Limits.MaxConcurrentRuns)
		usage.ConcurrentRuns.Limit = &v
	}
	return usage
}

func intPtr(v int64) *int64 {
	return &v
}
