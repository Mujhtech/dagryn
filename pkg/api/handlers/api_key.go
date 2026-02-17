package handlers

import (
	"errors"
	"net/http"
	"time"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ListUserAPIKeys godoc
// @Summary List user API keys
// @Description Returns all API keys for the current user (user-scoped keys)
// @Tags api-keys
// @Security BearerAuth
// @Produce json
// @Success 200 {object} []APIKeyResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/api-keys [get]
func (h *Handler) ListUserAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	keys, err := h.store.APIKeys.ListByUser(ctx, user.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list API keys"))
		return
	}

	resp := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, apiKeyWithProjectToResponse(&k))
	}

	_ = response.Ok(w, r, "Success", resp)
}

// CreateUserAPIKey godoc
// @Summary Create user API key
// @Description Creates a new user-scoped API key (access to all user's projects)
// @Tags api-keys
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body CreateAPIKeyRequest true "Create API key request"
// @Success 201 {object} APIKeyCreatedResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/api-keys [post]
func (h *Handler) CreateUserAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
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

	// Create user-scoped API key
	key := &models.APIKey{
		UserID:    user.ID,
		ProjectID: nil, // User-scoped
		Name:      req.Name,
		Scope:     models.APIKeyScopeUser,
		ExpiresAt: expiresAt,
	}

	rawKey, err := h.store.APIKeys.Create(ctx, key)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create API key"))
		return
	}

	_ = response.Created(w, r, "Created successfully", APIKeyCreatedResponse{
		APIKeyResponse: apiKeyModelToResponse(key),
		Key:            rawKey,
	})
}

// RevokeUserAPIKey godoc
// @Summary Revoke user API key
// @Description Revokes a user-scoped API key
// @Tags api-keys
// @Security BearerAuth
// @Param keyId path string true "API Key ID" format(uuid)
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/api-keys/{keyId} [delete]
func (h *Handler) RevokeUserAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	keyID, err := pathParamOrError(r, KeyIDParam)
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid API key ID"))
		return
	}

	uu, err := ParseUUID(keyID)

	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid API key ID format"))
		return
	}

	// Verify the key belongs to this user
	key, err := h.store.APIKeys.GetByID(ctx, uu)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("API key not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get API key"))
		return
	}

	if key.UserID != user.ID {
		_ = response.NotFound(w, r, errors.New("API key not found"))
		return
	}

	if err := h.store.APIKeys.Revoke(ctx, uu); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to revoke API key"))
		return
	}

	_ = response.NoContent(w, r)
}
