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
	Mode     string                     `json:"mode"`
	Edition  string                     `json:"edition"`
	Features map[licensing.Feature]bool `json:"features"`
	Nav      []NavItem                  `json:"nav"`
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
	mode := "self_hosted"
	edition := "community"

	if h.entitlements != nil {
		mode = h.entitlements.Mode()
		edition = h.entitlements.Edition()
	}

	ctx := r.Context()

	features := buildCapabilityFeatures(ctx, h)
	nav := buildNav(h, features)

	_ = response.Ok(w, r, "Capabilities retrieved", CapabilitiesResponse{
		Mode:     mode,
		Edition:  edition,
		Features: features,
		Nav:      nav,
	})
}

func buildCapabilityFeatures(ctx context.Context, h *Handler) map[licensing.Feature]bool {
	allFeatures := []licensing.Feature{
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

	m := make(map[licensing.Feature]bool, len(allFeatures))
	for _, f := range allFeatures {
		if h.entitlements != nil {
			m[f] = h.entitlements.HasFeature(ctx, string(f))
		} else {
			m[f] = false
		}
	}
	return m
}

func buildNav(h *Handler, features map[licensing.Feature]bool) []NavItem {
	isCloud := h.entitlements != nil && h.entitlements.Mode() == "cloud"
	nav := []NavItem{
		{Key: "projects", Label: "Projects", Enabled: true},
		{Key: "runs", Label: "Runs", Enabled: true},
		{Key: "plugins", Label: "Plugins", Enabled: true},
		{Key: "cache", Label: "Cache", Enabled: features[licensing.FeatureCloudCache]},
		{Key: "billing", Label: "Billing", Enabled: isCloud},
		{Key: "license", Label: "License", Enabled: !isCloud},
	}
	return nav
}
