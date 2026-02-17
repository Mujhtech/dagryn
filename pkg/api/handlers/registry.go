package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// --- Public Registry Endpoints ---

// SearchRegistryPlugins searches the plugin registry.
func (h *Handler) SearchRegistryPlugins(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	pluginType := strings.TrimSpace(r.URL.Query().Get("type"))
	sort := strings.TrimSpace(r.URL.Query().Get("sort"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	result, err := h.registryService.SearchPlugins(r.Context(), q, pluginType, sort, page, perPage)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "success", result)
}

// GetRegistryPlugin returns detailed plugin information.
func (h *Handler) GetRegistryPlugin(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	publisher, _ := pathParam(r, PublisherParam)
	name, _ := pathParam(r, NameParam)

	result, err := h.registryService.GetPluginDetail(r.Context(), publisher, name)
	if err != nil {
		_ = response.NotFound(w, r, errors.New("plugin not found"))
		return
	}

	_ = response.Ok(w, r, "success", result)
}

// GetRegistryPluginVersions returns version list for a plugin.
func (h *Handler) GetRegistryPluginVersions(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	publisher, _ := pathParam(r, PublisherParam)
	name, _ := pathParam(r, NameParam)

	versions, err := h.registryService.ListVersions(r.Context(), publisher, name)
	if err != nil {
		_ = response.NotFound(w, r, errors.New("plugin not found"))
		return
	}

	_ = response.Ok(w, r, "success", versions)
}

// GetRegistryPluginAnalytics returns download analytics for a plugin.
func (h *Handler) GetRegistryPluginAnalytics(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	publisher, _ := pathParam(r, PublisherParam)
	name, _ := pathParam(r, NameParam)
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}

	analytics, err := h.registryService.GetAnalytics(r.Context(), publisher, name, days)
	if err != nil {
		_ = response.NotFound(w, r, errors.New("plugin not found"))
		return
	}

	_ = response.Ok(w, r, "success", analytics)
}

// ListFeaturedPlugins returns featured plugins.
func (h *Handler) ListFeaturedPlugins(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	plugins, err := h.registryService.ListFeatured(r.Context(), limit)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "success", plugins)
}

// ListTrendingPlugins returns trending plugins.
func (h *Handler) ListTrendingPlugins(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	plugins, err := h.registryService.ListTrending(r.Context(), limit)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "success", plugins)
}

// TrackPluginDownload records a download event.
func (h *Handler) TrackPluginDownload(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	publisher, _ := pathParam(r, PublisherParam)
	name, _ := pathParam(r, NameParam)

	var req struct {
		Version string `json:"version"`
	}
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.Version == "" {
		_ = response.BadRequest(w, r, errors.New("version is required"))
		return
	}

	if err := h.registryService.RecordDownload(r.Context(), publisher, name, req.Version, nil, ""); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "download recorded", nil)
}

// CreatePublisher creates a new plugin publisher.
func (h *Handler) CreatePublisher(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Website     string `json:"website"`
	}
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		_ = response.BadRequest(w, r, errors.New("name is required"))
		return
	}

	pub := &models.PluginPublisher{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		UserID:      &user.ID,
	}
	if req.Website != "" {
		pub.Website = &req.Website
	}

	created, err := h.registryService.CreatePublisher(r.Context(), pub)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Created(w, r, "publisher created", created)
}

// GetPublisher returns publisher profile.
func (h *Handler) GetPublisher(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	publisher, _ := pathParam(r, PublisherParam)

	pub, err := h.registryService.GetPublisher(r.Context(), publisher)
	if err != nil {
		_ = response.NotFound(w, r, errors.New("publisher not found"))
		return
	}

	_ = response.Ok(w, r, "success", pub)
}

// PublishPluginVersion publishes a new version of a plugin.
func (h *Handler) PublishPluginVersion(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	publisher, _ := pathParam(r, PublisherParam)
	name, _ := pathParam(r, NameParam)

	var req struct {
		Version      string          `json:"version"`
		Manifest     json.RawMessage `json:"manifest"`
		ReleaseNotes string          `json:"release_notes"`
	}
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.Version == "" {
		_ = response.BadRequest(w, r, errors.New("version is required"))
		return
	}

	if err := h.registryService.PublishVersion(r.Context(), publisher, name, req.Manifest, req.Version, req.ReleaseNotes); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Created(w, r, "version published", nil)
}

// YankPluginVersion marks a version as yanked.
func (h *Handler) YankPluginVersion(w http.ResponseWriter, r *http.Request) {
	if h.registryService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("plugin registry not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	publisher, _ := pathParam(r, PublisherParam)
	name, _ := pathParam(r, NameParam)
	version, _ := pathParam(r, VersionParam)

	if err := h.registryService.YankVersion(r.Context(), publisher, name, version); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "version yanked", nil)
}
