package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ListAuditWebhooks godoc
//
//	@Summary		List audit webhooks
//	@Description	Returns all configured audit webhooks for a team
//	@Tags			audit-webhooks
//	@Security		BearerAuth
//	@Produce		json
//	@Param			teamId	path		string	true	"Team ID"
//	@Success		200		{array}		AuditWebhookResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/webhooks [get]
func (h *Handler) ListAuditWebhooks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage audit webhooks"))
		return
	}

	webhooks, err := h.store.AuditLogs.ListWebhooksByTeam(ctx, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list webhooks"))
		return
	}

	_ = response.Ok(w, r, "Success", webhooks)
}

// CreateAuditWebhook godoc
//
//	@Summary		Create audit webhook
//	@Description	Creates a new audit webhook with HMAC-SHA256 signing
//	@Tags			audit-webhooks
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			teamId	path		string						true	"Team ID"
//	@Param			body	body		CreateAuditWebhookRequest	true	"Webhook configuration"
//	@Success		201		{object}	AuditWebhookCreatedResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/webhooks [post]
func (h *Handler) CreateAuditWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage audit webhooks"))
		return
	}

	var req CreateAuditWebhookRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.URL == "" {
		_ = response.BadRequest(w, r, errors.New("url is required"))
		return
	}

	// Generate a random 32-byte HMAC secret.
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to generate secret"))
		return
	}
	secretPlain := base64.StdEncoding.EncodeToString(secretBytes)

	// Encrypt the secret for storage.
	enc := h.Encrypter()
	secretEncrypted, err := enc.Encrypt([]byte(secretPlain))
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to encrypt webhook secret"))
		return
	}

	webhook := &models.AuditWebhook{
		TeamID:          teamID,
		URL:             req.URL,
		SecretEncrypted: secretEncrypted,
		Description:     req.Description,
		EventFilter:     req.EventFilter,
		IsActive:        true,
		CreatedBy:       &user.ID,
	}
	if webhook.EventFilter == nil {
		webhook.EventFilter = []string{}
	}

	if err := h.store.AuditLogs.CreateWebhook(ctx, webhook); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create webhook"))
		return
	}

	// Return the webhook with the secret (shown only once).
	result := map[string]interface{}{
		"id":           webhook.ID,
		"team_id":      webhook.TeamID,
		"url":          webhook.URL,
		"description":  webhook.Description,
		"event_filter": webhook.EventFilter,
		"is_active":    webhook.IsActive,
		"created_at":   webhook.CreatedAt,
		"updated_at":   webhook.UpdatedAt,
		"secret":       secretPlain,
	}

	_ = response.Created(w, r, "Webhook created", result)
}

// GetAuditWebhook godoc
//
//	@Summary		Get audit webhook
//	@Description	Returns a single audit webhook by ID
//	@Tags			audit-webhooks
//	@Security		BearerAuth
//	@Produce		json
//	@Param			teamId		path		string	true	"Team ID"
//	@Param			webhookId	path		string	true	"Webhook ID"
//	@Success		200			{object}	AuditWebhookResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/webhooks/{webhookId} [get]
func (h *Handler) GetAuditWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage audit webhooks"))
		return
	}

	webhookID, err := getWebhookIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	webhook, err := h.store.AuditLogs.GetWebhookByID(ctx, webhookID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("webhook not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get webhook"))
		return
	}

	if webhook.TeamID != teamID {
		_ = response.NotFound(w, r, errors.New("webhook not found"))
		return
	}

	_ = response.Ok(w, r, "Success", webhook)
}

// UpdateAuditWebhook godoc
//
//	@Summary		Update audit webhook
//	@Description	Updates an existing audit webhook
//	@Tags			audit-webhooks
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			teamId		path		string						true	"Team ID"
//	@Param			webhookId	path		string						true	"Webhook ID"
//	@Param			body		body		UpdateAuditWebhookRequest	true	"Webhook update"
//	@Success		200			{object}	AuditWebhookResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/webhooks/{webhookId} [put]
func (h *Handler) UpdateAuditWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage audit webhooks"))
		return
	}

	webhookID, err := getWebhookIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	webhook, err := h.store.AuditLogs.GetWebhookByID(ctx, webhookID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("webhook not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get webhook"))
		return
	}

	if webhook.TeamID != teamID {
		_ = response.NotFound(w, r, errors.New("webhook not found"))
		return
	}

	var req UpdateAuditWebhookRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.URL != nil {
		webhook.URL = *req.URL
	}
	if req.Description != nil {
		webhook.Description = *req.Description
	}
	if req.EventFilter != nil {
		webhook.EventFilter = req.EventFilter
	}
	if req.IsActive != nil {
		webhook.IsActive = *req.IsActive
	}

	if err := h.store.AuditLogs.UpdateWebhook(ctx, webhook); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update webhook"))
		return
	}

	_ = response.Ok(w, r, "Webhook updated", webhook)
}

// DeleteAuditWebhook godoc
//
//	@Summary		Delete audit webhook
//	@Description	Deletes an audit webhook
//	@Tags			audit-webhooks
//	@Security		BearerAuth
//	@Produce		json
//	@Param			teamId		path		string	true	"Team ID"
//	@Param			webhookId	path		string	true	"Webhook ID"
//	@Success		200			{object}	SuccessResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/webhooks/{webhookId} [delete]
func (h *Handler) DeleteAuditWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage audit webhooks"))
		return
	}

	webhookID, err := getWebhookIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Verify webhook belongs to this team before deleting.
	webhook, err := h.store.AuditLogs.GetWebhookByID(ctx, webhookID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("webhook not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get webhook"))
		return
	}
	if webhook.TeamID != teamID {
		_ = response.NotFound(w, r, errors.New("webhook not found"))
		return
	}

	if err := h.store.AuditLogs.DeleteWebhook(ctx, webhookID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to delete webhook"))
		return
	}

	_ = response.Ok(w, r, "Webhook deleted", nil)
}

// TestAuditWebhook godoc
//
//	@Summary		Test audit webhook
//	@Description	Sends a test ping payload to the webhook and returns the delivery result
//	@Tags			audit-webhooks
//	@Security		BearerAuth
//	@Produce		json
//	@Param			teamId		path		string	true	"Team ID"
//	@Param			webhookId	path		string	true	"Webhook ID"
//	@Success		200			{object}	WebhookTestResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/webhooks/{webhookId}/test [post]
func (h *Handler) TestAuditWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage audit webhooks"))
		return
	}

	webhookID, err := getWebhookIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	webhook, err := h.store.AuditLogs.GetWebhookByID(ctx, webhookID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("webhook not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get webhook"))
		return
	}
	if webhook.TeamID != teamID {
		_ = response.NotFound(w, r, errors.New("webhook not found"))
		return
	}

	// Build a test payload.
	testPayload := map[string]interface{}{
		"id":            uuid.New().String(),
		"team_id":       teamID.String(),
		"actor_type":    "system",
		"actor_email":   user.Email,
		"action":        "webhook.test",
		"category":      "system",
		"resource_type": "audit_webhook",
		"resource_id":   webhookID.String(),
		"description":   "Test ping from Dagryn audit log webhook",
		"created_at":    time.Now().UTC().Format(time.RFC3339),
	}
	payloadBytes, _ := json.Marshal(testPayload)

	// Decrypt the HMAC secret.
	enc := h.Encrypter()
	secret, err := enc.Decrypt(webhook.SecretEncrypted)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to decrypt webhook secret"))
		return
	}

	// Compute HMAC-SHA256 signature.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Send the test request.
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create test request"))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dagryn-Signature", signature)
	req.Header.Set("X-Dagryn-Event", "webhook.test")

	start := time.Now()
	resp, err := client.Do(req)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		_ = response.Ok(w, r, "Test delivery failed", map[string]interface{}{
			"success":     false,
			"error":       err.Error(),
			"duration_ms": durationMs,
		})
		return
	}
	_ = resp.Body.Close()

	_ = response.Ok(w, r, "Test delivery complete", map[string]interface{}{
		"success":     resp.StatusCode < 400,
		"status_code": resp.StatusCode,
		"duration_ms": durationMs,
	})
}

func getWebhookIDFromPath(r *http.Request) (uuid.UUID, error) {
	webhookID, err := pathParamOrError(r, WebhookIDParam)
	if err != nil {
		return uuid.Nil, errors.New("webhook ID is required")
	}
	id, err := uuid.Parse(webhookID)
	if err != nil {
		return uuid.Nil, errors.New("invalid webhook ID")
	}
	return id, nil
}
