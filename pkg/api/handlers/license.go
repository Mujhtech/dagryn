package handlers

import (
	"net/http"
	"time"

	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/licensing"
)

// LicenseStatusResponse represents the license status returned by the API.
type LicenseStatusResponse struct {
	Mode          string          `json:"mode"`
	Edition       string          `json:"edition"`
	Licensed      bool            `json:"licensed"`
	Customer      string          `json:"customer,omitempty"`
	Seats         int             `json:"seats"`
	Features      map[string]bool `json:"features"`
	Limits        LicenseUsage    `json:"limits"`
	ExpiresAt     *time.Time      `json:"expires_at,omitempty"`
	DaysRemaining *int            `json:"days_remaining,omitempty"`
	GracePeriod   bool            `json:"grace_period"`
	Expiring      bool            `json:"expiring"`
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
		Features: buildFeatureMap(gate),
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

func buildFeatureMap(gate *licensing.FeatureGate) map[string]bool {
	features := []licensing.Feature{
		licensing.FeatureContainerExecution,
		licensing.FeaturePriorityQueue,
		licensing.FeatureSSO,
		licensing.FeatureAuditLogs,
		licensing.FeatureCustomRBAC,
		licensing.FeatureMultiCluster,
		licensing.FeatureDashboardFull,
		licensing.FeatureCloudCache,
		licensing.FeatureLogRetention,
		licensing.FeatureArtifactRetention,
		licensing.FeatureStorage,
		licensing.FeatureAIAnalysis,
		licensing.FeatureAISuggestions,
		licensing.FeatureCacheTTL,
		licensing.FeatureSaml,
	}

	m := make(map[string]bool, len(features))
	for _, f := range features {
		m[string(f)] = gate != nil && gate.HasFeature(f)
	}
	return m
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
