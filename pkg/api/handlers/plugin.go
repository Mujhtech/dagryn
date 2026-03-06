package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/dagryn/plugin"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

const officialPublisher = "dagryn"

var (
	errPluginAuthRequired = errors.New("authentication required")
	errPluginAccessDenied = errors.New("access denied")
)

// PluginInfo represents plugin metadata for API responses.
type PluginInfo struct {
	Name        string                   `json:"name"`
	Source      string                   `json:"source"`
	Version     string                   `json:"version"`
	Description string                   `json:"description"`
	Type        string                   `json:"type"`
	Author      string                   `json:"author,omitempty"`
	License     string                   `json:"license,omitempty"`
	Installed   bool                     `json:"installed"`
	Homepage    string                   `json:"homepage,omitempty"`
	Readme      string                   `json:"readme,omitempty"`
	LicenseText string                   `json:"license_text,omitempty"`
	Inputs      map[string]InputDefInfo  `json:"inputs,omitempty"`
	Outputs     map[string]OutputDefInfo `json:"outputs,omitempty"`
	Steps       []StepInfo               `json:"steps,omitempty"`
	Cleanup     []StepInfo               `json:"cleanup,omitempty"`
}

// InputDefInfo represents an input definition.
type InputDefInfo struct {
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// OutputDefInfo represents an output definition.
type OutputDefInfo struct {
	Description string `json:"description"`
}

// StepInfo represents a composite step.
type StepInfo struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	If      string `json:"if,omitempty"`
}

// PluginListResponse is the response for listing plugins.
type PluginListResponse struct {
	Plugins []PluginInfo `json:"plugins"`
}

// InstallPluginRequest is the request body for installing a plugin.
type InstallPluginRequest struct {
	Spec string `json:"spec"` // e.g., "dagryn/setup-node@v1" or "local:./plugins/my-plugin"
}

// ListPlugins godoc
//
//	@Summary		List available plugins
//	@Description	Returns plugins from the registry view with optional search and filtering
//	@Tags			plugins
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			q		query		string	false	"Search query"
//	@Param			type	query		string	false	"Filter by plugin type"
//	@Param			sort	query		string	false	"Sort order"	Enums(name, downloads, updated)
//	@Success		200		{object}	PluginListResponse
//	@Failure		400		{object}	ErrorResponse
//	@Router			/api/v1/plugins [get]
func (h *Handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.loadOfficialPlugins()
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
	filterType := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("type")))
	filtered := make([]PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		if q != "" {
			nameMatch := strings.Contains(strings.ToLower(p.Name), q)
			descMatch := strings.Contains(strings.ToLower(p.Description), q)
			if !nameMatch && !descMatch {
				continue
			}
		}
		if filterType != "" && strings.ToLower(p.Type) != filterType {
			continue
		}
		filtered = append(filtered, p)
	}

	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort"))) {
	case "", "name":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	case "downloads", "updated":
		// Reserved for future registry-backed sorting. Keep deterministic output for now.
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	default:
		_ = response.BadRequest(w, r, fmt.Errorf("invalid sort value: use one of downloads, updated, name"))
		return
	}

	_ = response.Ok(w, r, "success", PluginListResponse{Plugins: filtered})
}

// ListOfficialPlugins godoc
//
//	@Summary		List official plugins
//	@Description	Returns all official Dagryn plugins
//	@Tags			plugins
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Success		200	{object}	PluginListResponse
//	@Router			/api/v1/plugins/official [get]
func (h *Handler) ListOfficialPlugins(w http.ResponseWriter, r *http.Request) {
	h.ListPlugins(w, r)
}

// GetPluginManifest godoc
//
//	@Summary		Get plugin manifest
//	@Description	Returns detailed plugin information including inputs, outputs, and steps
//	@Tags			plugins
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			pluginName	path		string	true	"Plugin name"
//	@Success		200			{object}	PluginInfo
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/plugins/{pluginName} [get]
func (h *Handler) GetPluginManifest(w http.ResponseWriter, r *http.Request) {
	pluginName, err := pathParamOrError(r, PluginNameParam)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if pluginName == "" || strings.Contains(pluginName, "/") {
		_ = response.BadRequest(w, r, errors.New("invalid plugin name"))
		return
	}

	info, err := h.loadOfficialPlugin(pluginName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = response.NotFound(w, r, fmt.Errorf("plugin not found: %s", pluginName))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "success", info)
}

// GetPluginByPublisherName godoc
//
//	@Summary		Get plugin by publisher and name
//	@Description	Returns plugin details using registry-style publisher/name addressing
//	@Tags			plugins
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			publisher	path		string	true	"Publisher name"
//	@Param			name		path		string	true	"Plugin name"
//	@Success		200			{object}	PluginInfo
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/plugins/{publisher}/{name} [get]
func (h *Handler) GetPluginByPublisherName(w http.ResponseWriter, r *http.Request) {

	publisher, _ := pathParamOrError(r, PublisherParam)
	name, _ := pathParamOrError(r, NameParam)

	if publisher != officialPublisher {
		_ = response.NotFound(w, r, fmt.Errorf("publisher not found: %s", publisher))
		return
	}

	manifest, err := h.loadOfficialPlugin(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = response.NotFound(w, r, fmt.Errorf("plugin not found: %s/%s", publisher, name))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "success", manifest)
}

// ListProjectPlugins godoc
//
//	@Summary		List project plugins
//	@Description	Returns all plugins configured for a project
//	@Tags			plugins
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Success		200			{object}	PluginListResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/plugins [get]
func (h *Handler) ListProjectPlugins(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}
	if !role.HasPermission(models.PermissionProjectView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view project plugins"))
		return
	}

	workflows, err := h.store.Workflows.ListByProjectWithTasks(ctx, projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to load project workflows"))
		return
	}

	officialPlugins, err := h.loadOfficialPluginsByName()
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	pluginSpecs := collectPluginSpecsFromWorkflows(workflows)
	plugins := make([]PluginInfo, 0, len(pluginSpecs))
	for _, spec := range pluginSpecs {
		plugins = append(plugins, buildPluginInfoFromSpec(spec, officialPlugins))
	}

	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Name == plugins[j].Name {
			return plugins[i].Source < plugins[j].Source
		}
		return plugins[i].Name < plugins[j].Name
	})

	_ = response.Ok(w, r, "success", PluginListResponse{Plugins: plugins})
}

// InstallPlugin godoc
//
//	@Summary		Install a plugin
//	@Description	Installs a plugin for a project (not yet implemented)
//	@Tags			plugins
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			body		body		InstallPluginRequest	true	"Install plugin request"
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/plugins [post]
func (h *Handler) InstallPlugin(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	var req InstallPluginRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if strings.TrimSpace(req.Spec) == "" {
		_ = response.BadRequest(w, r, errors.New("plugin spec is required"))
		return
	}

	if err := h.requireProjectEditAccess(r, projectID); err != nil {
		h.writeProjectAccessError(w, r, err)
		return
	}

	_ = response.ServiceUnavailable(w, r, errors.New("plugin install via API is not implemented yet; update dagryn.toml and sync workflow"))
}

// UninstallPlugin godoc
//
//	@Summary		Uninstall a plugin
//	@Description	Uninstalls a plugin from a project (not yet implemented)
//	@Tags			plugins
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			pluginName	path		string	true	"Plugin name"
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/plugins/{pluginName} [delete]
func (h *Handler) UninstallPlugin(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	_, err = pathParamOrError(r, PluginNameParam)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if err := h.requireProjectEditAccess(r, projectID); err != nil {
		h.writeProjectAccessError(w, r, err)
		return
	}

	_ = response.ServiceUnavailable(w, r, errors.New("plugin uninstall via API is not implemented yet; update dagryn.toml and sync workflow"))
}

func (h *Handler) loadOfficialPlugins() ([]PluginInfo, error) {
	entries, err := os.ReadDir(h.officialPluginsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins directory: %w", err)
	}

	plugins := make([]PluginInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := h.loadOfficialPlugin(entry.Name())
		if err != nil {
			// Skip invalid plugin directories instead of failing the full list.
			continue
		}
		plugins = append(plugins, info)
	}

	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	return plugins, nil
}

func (h *Handler) loadOfficialPluginsByName() (map[string]PluginInfo, error) {
	plugins, err := h.loadOfficialPlugins()
	if err != nil {
		return nil, err
	}

	out := make(map[string]PluginInfo, len(plugins))
	for _, p := range plugins {
		out[p.Name] = p
	}
	return out, nil
}

func (h *Handler) loadOfficialPlugin(name string) (PluginInfo, error) {
	pluginDir := filepath.Join(h.officialPluginsDir(), name)
	manifestPath := filepath.Join(pluginDir, "plugin.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return PluginInfo{}, err
	}

	manifest, err := plugin.ParseManifest(data)
	if err != nil {
		return PluginInfo{}, fmt.Errorf("failed to parse plugin manifest for %s: %w", name, err)
	}
	if err := plugin.ValidateManifest(manifest); err != nil {
		return PluginInfo{}, fmt.Errorf("invalid plugin manifest for %s: %w", name, err)
	}

	info := manifestToPluginInfo(manifest, name, "official", false)

	// Read optional README.md and LICENSE files.
	if readme, err := os.ReadFile(filepath.Join(pluginDir, "README.md")); err == nil {
		info.Readme = string(readme)
	}
	if licenseText, err := os.ReadFile(filepath.Join(pluginDir, "LICENSE")); err == nil {
		info.LicenseText = string(licenseText)
	}

	return info, nil
}

func (h *Handler) officialPluginsDir() string {
	projectRoot := os.Getenv("DAGRYN_PROJECT_ROOT")
	if strings.TrimSpace(projectRoot) == "" {
		projectRoot = "."
	}
	return filepath.Join(projectRoot, "plugins")
}

func buildPluginInfoFromSpec(spec string, officialPlugins map[string]PluginInfo) PluginInfo {
	p, err := plugin.Parse(spec)
	if err != nil {
		return PluginInfo{
			Name:      spec,
			Source:    "unknown",
			Installed: true,
		}
	}

	info := PluginInfo{
		Name:      p.Name,
		Source:    string(p.Source),
		Version:   p.Version,
		Installed: true,
	}

	if official, ok := officialPlugins[p.Name]; ok {
		isOfficialLocal := p.Source == plugin.SourceLocal && filepath.Base(p.Repo) == p.Name
		isOfficialGitHub := p.Source == plugin.SourceGitHub && strings.EqualFold(p.Owner, officialPublisher)
		if isOfficialLocal || isOfficialGitHub {
			info = official
			info.Installed = true
			if p.Version != "" {
				info.Version = p.Version
			}
			info.Source = string(p.Source)
			return info
		}
	}

	if p.Source == plugin.SourceGitHub {
		info.Name = p.Repo
	}

	return info
}

func collectPluginSpecsFromWorkflows(workflows []models.WorkflowWithTasks) []string {
	seen := map[string]struct{}{}
	ordered := make([]string, 0)
	add := func(spec string) {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			return
		}
		if _, ok := seen[spec]; ok {
			return
		}
		seen[spec] = struct{}{}
		ordered = append(ordered, spec)
	}

	for _, wf := range workflows {
		if wf.RawConfig != nil && strings.TrimSpace(*wf.RawConfig) != "" {
			cfg, err := config.ParseBytes([]byte(*wf.RawConfig))
			if err == nil {
				names := make([]string, 0, len(cfg.Plugins))
				for name := range cfg.Plugins {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					add(cfg.Plugins[name])
				}
			}
		}

		for _, task := range wf.Tasks {
			for _, spec := range task.Plugins {
				add(spec)
			}
		}
	}

	return ordered
}

func (h *Handler) requireProjectEditAccess(r *http.Request, projectID uuid.UUID) error {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		return errPluginAuthRequired
	}

	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return errPluginAccessDenied
		}
		return fmt.Errorf("access check failed: %w", err)
	}
	if !role.HasPermission(models.PermissionProjectEdit) {
		return errPluginAccessDenied
	}
	return nil
}

func (h *Handler) writeProjectAccessError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, errPluginAuthRequired):
		_ = response.Unauthorized(w, r, err)
	case errors.Is(err, errPluginAccessDenied):
		_ = response.Forbidden(w, r, errors.New("you don't have permission to edit this project"))
	default:
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
	}
}

// manifestToPluginInfo converts a plugin manifest to PluginInfo.
func manifestToPluginInfo(manifest *plugin.Manifest, name, source string, installed bool) PluginInfo {
	info := PluginInfo{
		Name:        name,
		Source:      source,
		Version:     manifest.Plugin.Version,
		Description: manifest.Plugin.Description,
		Type:        manifest.Plugin.Type,
		Author:      manifest.Plugin.Author,
		License:     manifest.Plugin.License,
		Installed:   installed,
		Homepage:    manifest.Plugin.Homepage,
	}

	// Convert inputs
	if len(manifest.Inputs) > 0 {
		info.Inputs = make(map[string]InputDefInfo)
		for key, input := range manifest.Inputs {
			info.Inputs[key] = InputDefInfo{
				Description: input.Description,
				Required:    input.Required,
				Default:     input.Default,
			}
		}
	}

	// Convert outputs
	if len(manifest.Outputs) > 0 {
		info.Outputs = make(map[string]OutputDefInfo)
		for key, output := range manifest.Outputs {
			info.Outputs[key] = OutputDefInfo{
				Description: output.Description,
			}
		}
	}

	// Convert steps (for composite plugins)
	if len(manifest.Steps) > 0 {
		info.Steps = make([]StepInfo, len(manifest.Steps))
		for i, step := range manifest.Steps {
			info.Steps[i] = StepInfo{
				Name:    step.Name,
				Command: step.Command,
				If:      step.If,
			}
		}
	}

	// Convert cleanup steps (for composite plugins)
	if len(manifest.Cleanup) > 0 {
		info.Cleanup = make([]StepInfo, len(manifest.Cleanup))
		for i, step := range manifest.Cleanup {
			info.Cleanup[i] = StepInfo{
				Name:    step.Name,
				Command: step.Command,
				If:      step.If,
			}
		}
	}

	return info
}
