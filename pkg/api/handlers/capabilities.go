package handlers

import (
	"context"
	"net/http"

	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/licensing"
)

// CapabilitiesResponse describes the server's mode, available features, and
// visible navigation sections. The dashboard uses this to show/hide UI.
type CapabilitiesResponse struct {
	Mode     string         `json:"mode"`
	Edition  string         `json:"edition"`
	Features []FeatureEntry `json:"features"`
	Nav      []NavItem      `json:"nav"`
}

// NavItem represents a top-level navigation entry in the dashboard.
type NavItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

// GetCapabilities godoc
//
//	@Summary		Get server capabilities
//	@Description	Returns the server's mode, available features, and navigation items
//	@Tags			capabilities
//	@Produce		json
//	@Success		200	{object}	CapabilitiesResponse
//	@Router			/api/v1/capabilities [get]
func (h *Handler) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	mode, edition := h.modeAndEdition()
	features := h.buildEntitlementFeatures(r.Context())
	nav := buildNav(mode, features)

	_ = response.Ok(w, r, "Capabilities retrieved", CapabilitiesResponse{
		Mode:     mode,
		Edition:  edition,
		Features: features,
		Nav:      nav,
	})
}

// modeAndEdition returns the deployment mode and edition from the
// entitlement checker. Used by both capabilities and license endpoints.
func (h *Handler) modeAndEdition() (mode, edition string) {
	mode = "self_hosted"
	edition = "community"
	if h.entitlements != nil {
		mode = h.entitlements.Mode()
		edition = h.entitlements.Edition()
	}
	return
}

// buildEntitlementFeatures builds a feature list using the entitlement checker.
// This goes through the full entitlement path (LicenseChecker for OSS,
// BillingChecker for cloud) and is used by GetCapabilities.
func (h *Handler) buildEntitlementFeatures(ctx context.Context) []FeatureEntry {
	entries := make([]FeatureEntry, 0, len(licensing.AllFeatures))
	for _, f := range licensing.AllFeatures {
		enabled := false
		if h.entitlements != nil {
			enabled = h.entitlements.HasFeature(ctx, string(f))
		}
		entries = append(entries, FeatureEntry{
			Feature: string(f),
			Label:   f.DisplayName(),
			Enabled: enabled,
		})
	}
	return entries
}

func buildNav(mode string, features []FeatureEntry) []NavItem {
	isCloud := mode == "cloud"

	// Look up a specific feature's enabled state from the list.
	hasFeature := func(name string) bool {
		for _, f := range features {
			if f.Feature == name {
				return f.Enabled
			}
		}
		return false
	}

	return []NavItem{
		{Key: "projects", Label: "Projects", Enabled: true},
		{Key: "runs", Label: "Runs", Enabled: true},
		{Key: "plugins", Label: "Plugins", Enabled: true},
		{Key: "cache", Label: "Cache", Enabled: hasFeature(string(licensing.FeatureCloudCache))},
		{Key: "billing", Label: "Billing", Enabled: isCloud},
		{Key: "license", Label: "License", Enabled: !isCloud},
	}
}
