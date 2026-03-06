package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/service"
)

// ListProjectAPIKeys godoc
//
//	@Summary		List project API keys
//	@Description	Returns all API keys for a project
//	@Tags			projects,api-keys
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Success		200			{object}	[]APIKeyResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/api-keys [get]
func (h *Handler) ListProjectAPIKeys(w http.ResponseWriter, r *http.Request) {
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

	// Check user has access to project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to view API keys
	if !role.HasPermission(models.PermissionAPIKeysView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view API keys"))
		return
	}

	keys, err := h.store.APIKeys.ListByProject(ctx, projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list API keys"))
		return
	}

	resp := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, apiKeyModelToResponse(&k))
	}

	_ = response.Ok(w, r, "Success", resp)
}

// CreateProjectAPIKey godoc
//
//	@Summary		Create project API key
//	@Description	Creates a new API key for a project
//	@Tags			projects,api-keys
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string				true	"Project ID"	format(uuid)
//	@Param			body		body		CreateAPIKeyRequest	true	"Create API key request"
//	@Success		201			{object}	APIKeyCreatedResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/api-keys [post]
func (h *Handler) CreateProjectAPIKey(w http.ResponseWriter, r *http.Request) {
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

	// Check user has access to project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to manage API keys
	if !role.HasPermission(models.PermissionAPIKeysManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to create API keys"))
		return
	}

	var req CreateAPIKeyRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.Name == "" {
		_ = response.BadRequest(w, r, errors.New("API key name is required"))
		return
	}

	// Parse expiration
	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		duration, err := parseDuration(req.ExpiresIn)
		if err != nil {
			_ = response.BadRequest(w, r, errors.New("invalid expiration format: use format like '90d', '30d', '1y'"))
			return
		}
		exp := time.Now().Add(duration)
		expiresAt = &exp
	}

	// Create API key
	key := &models.APIKey{
		UserID:    user.ID,
		ProjectID: &projectID,
		Name:      req.Name,
		Scope:     models.APIKeyScopeProject,
		ExpiresAt: expiresAt,
	}

	rawKey, err := h.store.APIKeys.Create(ctx, key)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create API key"))
		return
	}

	// Audit log: API key created
	if h.auditService != nil {
		project, _ := h.store.Projects.GetByID(ctx, projectID)
		if project != nil && project.TeamID != nil {
			h.auditService.Log(ctx, service.AuditEntry{
				TeamID:       *project.TeamID,
				ProjectID:    &projectID,
				Action:       models.AuditActionAPIKeyCreated,
				Category:     models.AuditCategoryAPIKey,
				ResourceType: "api_key",
				ResourceID:   key.ID.String(),
				Description:  "API key created: " + key.Name,
			})
		}
	}

	_ = response.Created(w, r, "Created successfully", APIKeyCreatedResponse{
		APIKeyResponse: apiKeyModelToResponse(key),
		Key:            rawKey,
	})
}

// RevokeProjectAPIKey godoc
//
//	@Summary		Revoke project API key
//	@Description	Revokes an API key for a project
//	@Tags			projects,api-keys
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			projectId	path	string	true	"Project ID"	format(uuid)
//	@Param			keyID		path	string	true	"API Key ID"	format(uuid)
//	@Success		204			"No Content"
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/api-keys/{keyID} [delete]
func (h *Handler) RevokeProjectAPIKey(w http.ResponseWriter, r *http.Request) {
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
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid API key ID"))
		return
	}

	// Check user has access to project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to manage API keys
	if !role.HasPermission(models.PermissionAPIKeysManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to revoke API keys"))
		return
	}

	// Verify the key belongs to this project
	key, err := h.store.APIKeys.GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("API key not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get API key"))
		return
	}

	if key.ProjectID == nil || *key.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("API key not found in this project"))
		return
	}

	if err := h.store.APIKeys.Revoke(ctx, keyID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to revoke API key"))
		return
	}

	// Audit log: API key revoked
	if h.auditService != nil {
		project, _ := h.store.Projects.GetByID(ctx, projectID)
		if project != nil && project.TeamID != nil {
			h.auditService.Log(ctx, service.AuditEntry{
				TeamID:       *project.TeamID,
				ProjectID:    &projectID,
				Action:       models.AuditActionAPIKeyRevoked,
				Category:     models.AuditCategoryAPIKey,
				ResourceType: "api_key",
				ResourceID:   keyID.String(),
				Description:  "API key revoked: " + key.Name,
			})
		}
	}

	_ = response.NoContent(w, r)
}
