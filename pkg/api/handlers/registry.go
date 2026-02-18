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

// SearchRegistryPlugins godoc
//
//	@Summary		Search registry plugins
//	@Description	Searches the plugin registry with optional filtering and pagination
//	@Tags			registry
//	@Produce		json
//	@Param			q			query		string	false	"Search query"
//	@Param			type		query		string	false	"Filter by plugin type"
//	@Param			sort		query		string	false	"Sort order"
//	@Param			page		query		int		false	"Page number"
//	@Param			per_page	query		int		false	"Items per page"
//	@Success		200			{object}	SuccessResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/plugins [get]
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

// GetRegistryPlugin godoc
//
//	@Summary		Get registry plugin details
//	@Description	Returns detailed information about a plugin in the registry
//	@Tags			registry
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Param			name		path		string	true	"Plugin name"
//	@Success		200			{object}	SuccessResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/plugins/{publisher}/{name} [get]
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

// GetRegistryPluginVersions godoc
//
//	@Summary		List plugin versions
//	@Description	Returns the version list for a registry plugin
//	@Tags			registry
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Param			name		path		string	true	"Plugin name"
//	@Success		200			{object}	SuccessResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/plugins/{publisher}/{name}/versions [get]
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

// GetRegistryPluginAnalytics godoc
//
//	@Summary		Get plugin download analytics
//	@Description	Returns download analytics for a registry plugin
//	@Tags			registry
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Param			name		path		string	true	"Plugin name"
//	@Param			days		query		int		false	"Number of days"	default(30)
//	@Success		200			{object}	SuccessResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/plugins/{publisher}/{name}/analytics [get]
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

// ListFeaturedPlugins godoc
//
//	@Summary		List featured plugins
//	@Description	Returns a list of featured plugins from the registry
//	@Tags			registry
//	@Produce		json
//	@Param			limit	query		int	false	"Number of plugins to return"	default(10)
//	@Success		200		{object}	SuccessResponse
//	@Failure		503		{object}	ErrorResponse
//	@Router			/api/v1/registry/featured [get]
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

// ListTrendingPlugins godoc
//
//	@Summary		List trending plugins
//	@Description	Returns a list of trending plugins from the registry
//	@Tags			registry
//	@Produce		json
//	@Param			limit	query		int	false	"Number of plugins to return"	default(10)
//	@Success		200		{object}	SuccessResponse
//	@Failure		503		{object}	ErrorResponse
//	@Router			/api/v1/registry/trending [get]
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

// TrackPluginDownload godoc
//
//	@Summary		Track plugin download
//	@Description	Records a download event for a registry plugin
//	@Tags			registry
//	@Accept			json
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Param			name		path		string	true	"Plugin name"
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/plugins/{publisher}/{name}/download [post]
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

// CreatePublisher godoc
//
//	@Summary		Create a plugin publisher
//	@Description	Creates a new plugin publisher in the registry
//	@Tags			registry
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Success		201	{object}	SuccessResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		503	{object}	ErrorResponse
//	@Router			/api/v1/registry/publishers [post]
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

// GetPublisher godoc
//
//	@Summary		Get publisher profile
//	@Description	Returns the profile of a plugin publisher
//	@Tags			registry
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Success		200			{object}	SuccessResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/publishers/{publisher} [get]
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

// PublishPluginVersion godoc
//
//	@Summary		Publish a plugin version
//	@Description	Publishes a new version of a plugin to the registry
//	@Tags			registry
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Param			name		path		string	true	"Plugin name"
//	@Success		201			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/plugins/{publisher}/{name}/versions [post]
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

// YankPluginVersion godoc
//
//	@Summary		Yank a plugin version
//	@Description	Marks a plugin version as yanked in the registry
//	@Tags			registry
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Param			name		path		string	true	"Plugin name"
//	@Param			version		path		string	true	"Version string"
//	@Success		200			{object}	SuccessResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/registry/plugins/{publisher}/{name}/versions/{version} [delete]
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
