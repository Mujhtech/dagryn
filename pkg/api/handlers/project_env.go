package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/secrets"
)

func (h *Handler) secretsProviderConfig() secrets.Config {
	return secrets.Config{
		AWSRegion:           h.envSecretsAWSRegion,
		AWSAccessKeyID:      h.envSecretsAWSAccessKeyID,
		AWSSecretAccessKey:  h.envSecretsAWSSecretKey,
		AWSCredentialsFile:  h.envSecretsAWSCredsFile,
		GCPCredentialsFile:  h.envSecretsGCPCredsFile,
		CloudflareAccountID: h.envSecretsCFAccountID,
		CloudflareAPIToken:  h.envSecretsCFAPIToken,
		CloudflareAPIBase:   h.envSecretsCFAPIBase,
	}
}

// ListProjectEnvVars godoc
//
//	@Summary		List project environment variables
//	@Description	Returns project env metadata (secret values are not included)
//	@Tags			projects,env
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID" format(uuid)
//	@Param			body		body		ListProjectEnvVarsRequest	false	"List filters"
//	@Success		200			{object}	[]ProjectEnvVarResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/env-vars [post]
func (h *Handler) ListProjectEnvVars(w http.ResponseWriter, r *http.Request) {
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
	if !role.HasPermission(models.PermissionEnvView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view project env"))
		return
	}

	var req ListProjectEnvVarsRequest
	_ = ParseJSON(r, &req)

	filter := repo.ProjectEnvFilter{ProjectID: projectID}
	if req.Environment != nil && strings.TrimSpace(*req.Environment) != "" {
		filter.Environment = req.Environment
	}
	if req.Branch != nil && strings.TrimSpace(*req.Branch) != "" {
		filter.Branch = req.Branch
	}
	if req.Key != nil && strings.TrimSpace(*req.Key) != "" {
		filter.Key = req.Key
	}
	// provider is server-managed via env_secrets config; ignore client filter.
	if req.ValueType != nil && strings.TrimSpace(*req.ValueType) != "" {
		vt := models.EnvValueType(*req.ValueType)
		filter.ValueType = &vt
	}

	items, err := h.store.ProjectEnv.ListEnvVars(ctx, filter)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list env vars"))
		return
	}

	out := make([]ProjectEnvVarResponse, 0, len(items))
	for _, item := range items {
		out = append(out, projectEnvVarToResponse(item, false, nil))
	}
	_ = response.Ok(w, r, "Success", out)
}

// SeedProjectEnvVars godoc
//
//	@Summary		Seed project environment variables
//	@Description	Bulk creates or updates environment variables for a project scope.
//	@Tags			projects,env
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string				true	"Project ID" format(uuid)
//	@Param			body		body		SeedProjectEnvVarsRequest	true	"Seed payload"
//	@Success		200			{object}	[]ProjectEnvVarResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/env-vars/seed [post]
func (h *Handler) SeedProjectEnvVars(w http.ResponseWriter, r *http.Request) {
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
	if !role.HasPermission(models.PermissionEnvManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage project env"))
		return
	}

	var req SeedProjectEnvVarsRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if len(req.Items) == 0 {
		_ = response.BadRequest(w, r, errors.New("items are required"))
		return
	}

	out := make([]ProjectEnvVarResponse, 0, len(req.Items))
	for _, item := range req.Items {
		if strings.TrimSpace(item.Key) == "" || item.Value == "" {
			continue
		}
		rec, err := h.upsertProjectEnvVar(ctx, projectID, user.ID, item)
		if err != nil {
			_ = response.InternalServerError(w, r, err)
			return
		}
		out = append(out, projectEnvVarToResponse(*rec, false, nil))
	}

	audit := models.EnvAuditEvent{ProjectID: projectID, ActorID: &user.ID, Action: "seed", Metadata: buildEnvAuditMetadata(ctx, map[string]any{"count": len(out)})}
	_ = h.store.ProjectEnv.CreateAuditEvent(ctx, &audit)
	_ = response.Ok(w, r, "Seeded successfully", out)
}

// UpdateProjectEnvVar godoc
//
//	@Summary		Update project environment variable metadata
//	@Description	Updates env var metadata fields such as description, required and enabled flags.
//	@Tags			projects,env
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string				true	"Project ID" format(uuid)
//	@Param			envVarId	path		string				true	"Env Var ID" format(uuid)
//	@Param			body		body		UpdateProjectEnvVarRequest	true	"Update payload"
//	@Success		200			{object}	ProjectEnvVarResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/env-vars/{envVarId} [patch]
func (h *Handler) UpdateProjectEnvVar(w http.ResponseWriter, r *http.Request) {
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
	envVarID, err := getEnvVarIDFromPath(r)
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
	if !role.HasPermission(models.PermissionEnvManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage project env"))
		return
	}

	item, err := h.store.ProjectEnv.GetEnvVarByID(ctx, projectID, envVarID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("env var not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to load env var"))
		return
	}

	var req UpdateProjectEnvVarRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.Description != nil {
		item.Description = trimStringPtr(req.Description)
	}
	if req.Required != nil {
		item.Required = *req.Required
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	item.UpdatedBy = &user.ID

	if err := h.store.ProjectEnv.UpdateEnvVar(ctx, item); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update env var"))
		return
	}

	providerForAudit := item.Provider
	audit := models.EnvAuditEvent{
		ProjectID:   projectID,
		EnvVarID:    &item.ID,
		ActorID:     &user.ID,
		Action:      "update",
		Provider:    &providerForAudit,
		Environment: item.Environment,
		Branch:      item.Branch,
		Metadata:    buildEnvAuditMetadata(ctx, map[string]any{"updated_fields": true}),
	}
	_ = h.store.ProjectEnv.CreateAuditEvent(ctx, &audit)

	_ = response.Ok(w, r, "Updated successfully", projectEnvVarToResponse(*item, false, nil))
}

// RotateProjectEnvVar godoc
//
//	@Summary		Rotate project env secret value
//	@Description	Rotates secret material for an existing secret env var.
//	@Tags			projects,env
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID" format(uuid)
//	@Param			envVarId	path		string					true	"Env Var ID" format(uuid)
//	@Param			body		body		RotateProjectEnvVarRequest	true	"Rotate payload"
//	@Success		200			{object}	ProjectEnvVarResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/env-vars/{envVarId}/rotate [post]
func (h *Handler) RotateProjectEnvVar(w http.ResponseWriter, r *http.Request) {
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
	envVarID, err := getEnvVarIDFromPath(r)
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
	if !role.HasPermission(models.PermissionEnvRotate) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to rotate project env secrets"))
		return
	}

	item, err := h.store.ProjectEnv.GetEnvVarByID(ctx, projectID, envVarID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("env var not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to load env var"))
		return
	}
	if item.ValueType != models.EnvValueTypeSecret {
		_ = response.BadRequest(w, r, errors.New("only secret env vars can be rotated"))
		return
	}

	var req RotateProjectEnvVarRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if item.Provider == models.SecretProviderDB {
		if strings.TrimSpace(req.Value) == "" {
			_ = response.BadRequest(w, r, errors.New("value is required for db provider rotation"))
			return
		}
		if item.SecretRecordID == nil {
			_ = response.InternalServerError(w, r, errors.New("db secret record is missing"))
			return
		}
		record, err := h.store.ProjectEnv.GetSecretRecordByID(ctx, *item.SecretRecordID)
		if err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to load secret record"))
			return
		}
		encrypter := h.Encrypter()
		if encrypter == nil {
			_ = response.InternalServerError(w, r, errors.New("secret encryption is not configured"))
			return
		}
		ciphertext, err := encrypter.Encrypt([]byte(req.Value))
		if err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to encrypt rotated value"))
			return
		}
		sum := sha256.Sum256([]byte(req.Value))
		checksum := hex.EncodeToString(sum[:])
		record.Ciphertext = []byte(ciphertext)
		record.Checksum = &checksum
		now := time.Now()
		record.RotatedAt = &now
		record.Status = models.SecretRecordStatusActive
		if err := h.store.ProjectEnv.UpdateSecretRecord(ctx, record); err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to rotate secret record"))
			return
		}
	} else {
		providerClient, perr := secrets.NewProviderWithConfig(ctx, item.Provider, h.secretsProviderConfig())
		if perr != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to initialize secret provider"))
			return
		}

		if strings.TrimSpace(req.Value) != "" {
			if item.ProviderRef == nil || strings.TrimSpace(*item.ProviderRef) == "" {
				_ = response.BadRequest(w, r, errors.New("existing provider_ref is missing in managed provider config"))
				return
			}
			if err := providerClient.Put(ctx, *item.ProviderRef, req.Value); err != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to rotate value in secret provider"))
				return
			}
		}
		item.UpdatedBy = &user.ID
		if err := h.store.ProjectEnv.UpdateEnvVar(ctx, item); err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to rotate provider reference"))
			return
		}
	}

	providerForAudit := item.Provider
	audit := models.EnvAuditEvent{ProjectID: projectID, EnvVarID: &item.ID, ActorID: &user.ID, Action: "rotate", Provider: &providerForAudit, Environment: item.Environment, Branch: item.Branch, Metadata: buildEnvAuditMetadata(ctx, map[string]any{"rotated": true})}
	_ = h.store.ProjectEnv.CreateAuditEvent(ctx, &audit)
	_ = response.Ok(w, r, "Rotated successfully", projectEnvVarToResponse(*item, false, nil))
}

// DeleteProjectEnvVar godoc
//
//	@Summary		Delete project environment variable
//	@Description	Soft-deletes an environment variable record.
//	@Tags			projects,env
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			projectId	path	string	true	"Project ID" format(uuid)
//	@Param			envVarId	path	string	true	"Env Var ID" format(uuid)
//	@Success		204
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/env-vars/{envVarId} [delete]
func (h *Handler) DeleteProjectEnvVar(w http.ResponseWriter, r *http.Request) {
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
	envVarID, err := getEnvVarIDFromPath(r)
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
	if !role.HasPermission(models.PermissionEnvManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to delete project env vars"))
		return
	}

	item, err := h.store.ProjectEnv.GetEnvVarByID(ctx, projectID, envVarID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("env var not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to load env var"))
		return
	}

	cleanup := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("cleanup_provider")), "true")
	if cleanup && item.ValueType == models.EnvValueTypeSecret {
		switch item.Provider {
		case models.SecretProviderDB:
			if item.SecretRecordID != nil {
				if err := h.store.ProjectEnv.RevokeSecretRecord(ctx, *item.SecretRecordID); err != nil && !errors.Is(err, repo.ErrNotFound) {
					_ = response.InternalServerError(w, r, errors.New("failed to revoke db secret record"))
					return
				}
			}
		default:
			if item.ProviderRef != nil && strings.TrimSpace(*item.ProviderRef) != "" {
				providerClient, perr := secrets.NewProviderWithConfig(ctx, item.Provider, h.secretsProviderConfig())
				if perr != nil {
					_ = response.InternalServerError(w, r, errors.New("failed to initialize secret provider"))
					return
				}
				if derr := providerClient.Delete(ctx, *item.ProviderRef); derr != nil {
					_ = response.InternalServerError(w, r, errors.New("failed to delete secret from provider"))
					return
				}
			}
		}
	}

	if err := h.store.ProjectEnv.SoftDeleteEnvVar(ctx, projectID, envVarID, &user.ID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("env var not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to delete env var"))
		return
	}

	providerForAudit := item.Provider
	audit := models.EnvAuditEvent{ProjectID: projectID, EnvVarID: &envVarID, ActorID: &user.ID, Action: "delete", Provider: &providerForAudit, Environment: item.Environment, Branch: item.Branch, Metadata: buildEnvAuditMetadata(ctx, map[string]any{"cleanup_provider": cleanup})}
	_ = h.store.ProjectEnv.CreateAuditEvent(ctx, &audit)
	_ = response.NoContent(w, r)
}

// SetProjectEnvVar godoc
//
//	@Summary		Create or update a project environment variable
//	@Description	Creates or upserts a project env variable. Secret values are encrypted/ref-managed.
//	@Tags			projects,env
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string				true	"Project ID" format(uuid)
//	@Param			body		body		SetProjectEnvVarRequest	true	"Set env payload"
//	@Success		200			{object}	ProjectEnvVarResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/env-vars/set [post]
func (h *Handler) SetProjectEnvVar(w http.ResponseWriter, r *http.Request) {
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
	if !role.HasPermission(models.PermissionEnvManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage project env"))
		return
	}

	var req SetProjectEnvVarRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if strings.TrimSpace(req.Key) == "" {
		_ = response.BadRequest(w, r, errors.New("key is required"))
		return
	}
	if req.Value == "" {
		_ = response.BadRequest(w, r, errors.New("value is required"))
		return
	}

	provider := models.SecretProvider(h.envSecretsProvider)
	if provider == "" {
		provider = models.SecretProviderDB
	}
	if !isValidSecretProvider(provider) {
		_ = response.BadRequest(w, r, errors.New("invalid provider"))
		return
	}

	key := strings.TrimSpace(req.Key)
	env := trimStringPtr(req.Environment)
	branch := trimStringPtr(req.Branch)
	desc := trimStringPtr(req.Description)

	var existing *models.ProjectEnvVar
	existing, err = h.store.ProjectEnv.GetEnvVarByScope(ctx, projectID, key, env, branch)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		_ = response.InternalServerError(w, r, errors.New("failed to read existing env var"))
		return
	}

	valueType := models.EnvValueTypePlain
	if req.Secret {
		valueType = models.EnvValueTypeSecret
	}

	var secretID *uuid.UUID
	var plainValue *string
	var providerRef *string

	if valueType == models.EnvValueTypeSecret {
		switch provider {
		case models.SecretProviderDB:
			encrypter := h.Encrypter()
			if encrypter == nil {
				_ = response.InternalServerError(w, r, errors.New("secret encryption is not configured"))
				return
			}
			ciphertext, encErr := encrypter.Encrypt([]byte(req.Value))
			if encErr != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to encrypt secret value"))
				return
			}
			sr := &models.SecretRecord{
				Provider:   provider,
				Ciphertext: []byte(ciphertext),
				Status:     models.SecretRecordStatusActive,
			}
			if err := h.store.ProjectEnv.CreateSecretRecord(ctx, sr); err != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to persist secret record"))
				return
			}
			secretID = &sr.ID
		default:
			providerRefVal := h.buildManagedProviderRef(projectID, key, env, branch)
			if strings.TrimSpace(providerRefVal) == "" {
				_ = response.InternalServerError(w, r, errors.New("managed provider_ref could not be derived from server config"))
				return
			}
			providerClient, perr := secrets.NewProviderWithConfig(ctx, provider, h.secretsProviderConfig())
			if perr != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to initialize secret provider"))
				return
			}
			if err := providerClient.Put(ctx, providerRefVal, req.Value); err != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to write value to secret provider"))
				return
			}
			providerRef = stringPtr(providerRefVal)
		}
	} else {
		v := req.Value
		plainValue = &v
	}

	var out *models.ProjectEnvVar
	if existing != nil {
		existing.Key = key
		existing.ValueType = valueType
		existing.PlainValue = plainValue
		existing.Environment = env
		existing.Branch = branch
		existing.Required = req.Required
		existing.Enabled = true
		existing.Provider = provider
		existing.ProviderRef = providerRef
		existing.SecretRecordID = secretID
		existing.Description = desc
		existing.UpdatedBy = &user.ID
		if err := h.store.ProjectEnv.UpdateEnvVar(ctx, existing); err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to update env var"))
			return
		}
		out = existing
	} else {
		item := &models.ProjectEnvVar{
			ProjectID:      projectID,
			Key:            key,
			ValueType:      valueType,
			PlainValue:     plainValue,
			Environment:    env,
			Branch:         branch,
			Required:       req.Required,
			Enabled:        true,
			Provider:       provider,
			ProviderRef:    providerRef,
			SecretRecordID: secretID,
			Description:    desc,
			CreatedBy:      &user.ID,
			UpdatedBy:      &user.ID,
		}
		if err := h.store.ProjectEnv.CreateEnvVar(ctx, item); err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to create env var"))
			return
		}
		out = item
	}

	providerForAudit := out.Provider
	audit := models.EnvAuditEvent{
		ProjectID:   projectID,
		EnvVarID:    &out.ID,
		ActorID:     &user.ID,
		Action:      "set",
		Provider:    &providerForAudit,
		Environment: out.Environment,
		Branch:      out.Branch,
		Metadata:    buildEnvAuditMetadata(ctx, map[string]any{"secret": req.Secret, "required": req.Required}),
	}
	_ = h.store.ProjectEnv.CreateAuditEvent(ctx, &audit)

	_ = response.Ok(w, r, "Updated successfully", projectEnvVarToResponse(*out, false, nil))
}

// ResolveProjectEnvVars godoc
//
//	@Summary		Resolve project environment variables
//	@Description	Resolves effective env vars for branch/environment precedence. Optionally reveals values for CLI use.
//	@Tags			projects,env
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string				true	"Project ID" format(uuid)
//	@Param			body		body		ResolveProjectEnvRequest	true	"Resolve payload"
//	@Success		200			{object}	[]ProjectEnvVarResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/env-vars/resolve [post]
func (h *Handler) ResolveProjectEnvVars(w http.ResponseWriter, r *http.Request) {
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
	if !role.HasPermission(models.PermissionEnvView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to resolve project env"))
		return
	}

	var req ResolveProjectEnvRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.Reveal && !role.HasPermission(models.PermissionEnvReveal) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to reveal project env values"))
		return
	}
	if strings.TrimSpace(req.Environment) == "" {
		_ = response.BadRequest(w, r, errors.New("environment is required"))
		return
	}

	resolved, err := h.store.ProjectEnv.ResolveEnv(ctx, repo.EnvResolutionParams{
		ProjectID:   projectID,
		Environment: req.Environment,
		Branch:      req.Branch,
	})
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to resolve project env"))
		return
	}

	encrypter := h.Encrypter()
	out := make([]ProjectEnvVarResponse, 0, len(resolved))
	missingRequired := make([]string, 0)
	for _, item := range resolved {
		var revealed *string
		if req.Reveal {
			val, ok := h.resolveValueForResponse(ctx, item, encrypter)
			if ok {
				revealed = &val
			} else if item.Required {
				missingRequired = append(missingRequired, item.Key)
			}
		}

		out = append(out, ProjectEnvVarResponse{
			Key:         item.Key,
			ValueType:   string(item.ValueType),
			Environment: item.Environment,
			Branch:      item.Branch,
			Required:    item.Required,
			Enabled:     true,
			Value:       revealed,
		})
	}

	audit := models.EnvAuditEvent{
		ProjectID:   projectID,
		ActorID:     &user.ID,
		Action:      "resolve",
		Environment: &req.Environment,
		Branch:      stringPtr(req.Branch),
		Metadata:    buildEnvAuditMetadata(ctx, map[string]any{"reveal": req.Reveal, "resolved_count": len(out)}),
	}
	_ = h.store.ProjectEnv.CreateAuditEvent(ctx, &audit)

	if len(missingRequired) > 0 {
		w.Header().Set("X-Dagryn-Env-Warnings", fmt.Sprintf("missing_required=%s", strings.Join(missingRequired, ",")))
	}

	_ = response.Ok(w, r, "Success", out)
}

func (h *Handler) resolveValueForResponse(ctx context.Context, item repo.ResolvedEnvVar, encrypter interface {
	Decrypt(ciphertext string) (string, error)
}) (string, bool) {
	if item.ValueType == models.EnvValueTypePlain {
		if item.PlainValue == nil {
			return "", false
		}
		return *item.PlainValue, true
	}

	switch item.Provider {
	case models.SecretProviderDB:
		if item.SecretRecordID == nil || encrypter == nil {
			return "", false
		}
		record, err := h.store.ProjectEnv.GetSecretRecordByID(ctx, *item.SecretRecordID)
		if err != nil {
			return "", false
		}
		decrypted, err := encrypter.Decrypt(string(record.Ciphertext))
		if err != nil {
			return "", false
		}
		return decrypted, true
	default:
		if item.ProviderRef == nil || strings.TrimSpace(*item.ProviderRef) == "" {
			return "", false
		}
		providerClient, err := secrets.NewProviderWithConfig(ctx, item.Provider, h.secretsProviderConfig())
		if err != nil {
			return "", false
		}
		value, err := providerClient.Get(ctx, *item.ProviderRef)
		if err != nil {
			return "", false
		}
		return value, true
	}
}

func isValidSecretProvider(provider models.SecretProvider) bool {
	switch provider {
	case models.SecretProviderDB, models.SecretProviderAWS, models.SecretProviderGCP, models.SecretProviderCloudflare:
		return true
	default:
		return false
	}
}

func trimStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func buildEnvAuditMetadata(ctx context.Context, extra map[string]any) []byte {
	meta := map[string]any{}
	for k, v := range extra {
		meta[k] = v
	}
	if rid := apiCtx.GetRequestID(ctx); rid != "" {
		meta["request_id"] = rid
	}
	if ip := apiCtx.GetIPAddress(ctx); ip != "" {
		meta["ip"] = ip
	}
	if ua := apiCtx.GetUserAgent(ctx); ua != "" {
		meta["user_agent"] = ua
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func (h *Handler) upsertProjectEnvVar(ctx context.Context, projectID, userID uuid.UUID, req SetProjectEnvVarRequest) (*models.ProjectEnvVar, error) {
	provider := models.SecretProvider(h.envSecretsProvider)
	if provider == "" {
		provider = models.SecretProviderDB
	}
	if !isValidSecretProvider(provider) {
		return nil, fmt.Errorf("invalid provider")
	}

	key := strings.TrimSpace(req.Key)
	env := trimStringPtr(req.Environment)
	branch := trimStringPtr(req.Branch)
	desc := trimStringPtr(req.Description)

	existing, err := h.store.ProjectEnv.GetEnvVarByScope(ctx, projectID, key, env, branch)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, fmt.Errorf("failed to read existing env var")
	}

	valueType := models.EnvValueTypePlain
	if req.Secret {
		valueType = models.EnvValueTypeSecret
	}

	var secretID *uuid.UUID
	var plainValue *string
	var providerRef *string

	if valueType == models.EnvValueTypeSecret {
		switch provider {
		case models.SecretProviderDB:
			encrypter := h.Encrypter()
			if encrypter == nil {
				return nil, fmt.Errorf("secret encryption is not configured")
			}
			ciphertext, err := encrypter.Encrypt([]byte(req.Value))
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt secret value")
			}
			sr := &models.SecretRecord{Provider: provider, Ciphertext: []byte(ciphertext), Status: models.SecretRecordStatusActive}
			if err := h.store.ProjectEnv.CreateSecretRecord(ctx, sr); err != nil {
				return nil, fmt.Errorf("failed to persist secret record")
			}
			secretID = &sr.ID
		default:
			providerRefVal := h.buildManagedProviderRef(projectID, key, env, branch)
			if strings.TrimSpace(providerRefVal) == "" {
				return nil, fmt.Errorf("managed provider_ref could not be derived from server config")
			}
			providerClient, err := secrets.NewProviderWithConfig(ctx, provider, h.secretsProviderConfig())
			if err != nil {
				return nil, fmt.Errorf("failed to initialize secret provider")
			}
			if err := providerClient.Put(ctx, providerRefVal, req.Value); err != nil {
				return nil, fmt.Errorf("failed to write value to secret provider")
			}
			providerRef = stringPtr(providerRefVal)
		}
	} else {
		v := req.Value
		plainValue = &v
	}

	if existing != nil {
		existing.Key = key
		existing.ValueType = valueType
		existing.PlainValue = plainValue
		existing.Environment = env
		existing.Branch = branch
		existing.Required = req.Required
		existing.Enabled = true
		existing.Provider = provider
		existing.ProviderRef = providerRef
		existing.SecretRecordID = secretID
		existing.Description = desc
		existing.UpdatedBy = &userID
		if err := h.store.ProjectEnv.UpdateEnvVar(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update env var")
		}
		return existing, nil
	}

	item := &models.ProjectEnvVar{
		ProjectID:      projectID,
		Key:            key,
		ValueType:      valueType,
		PlainValue:     plainValue,
		Environment:    env,
		Branch:         branch,
		Required:       req.Required,
		Enabled:        true,
		Provider:       provider,
		ProviderRef:    providerRef,
		SecretRecordID: secretID,
		Description:    desc,
		CreatedBy:      &userID,
		UpdatedBy:      &userID,
	}
	if err := h.store.ProjectEnv.CreateEnvVar(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to create env var")
	}
	return item, nil
}

func (h *Handler) buildManagedProviderRef(projectID uuid.UUID, key string, environment, branch *string) string {
	provider := models.SecretProvider(h.envSecretsProvider)
	scopeEnv := "default"
	if environment != nil && strings.TrimSpace(*environment) != "" {
		scopeEnv = strings.TrimSpace(*environment)
	}
	scopeBranch := "default"
	if branch != nil && strings.TrimSpace(*branch) != "" {
		scopeBranch = strings.TrimSpace(*branch)
	}

	prefix := strings.TrimSpace(h.envSecretsProviderRefPrefix)
	if prefix == "" {
		prefix = "projects"
	}
	prefix = strings.Trim(prefix, "/")

	base := fmt.Sprintf("%s/%s/%s/%s/%s", prefix, projectID.String(), scopeEnv, scopeBranch, key)

	if provider == models.SecretProviderCloudflare {
		storeID := strings.TrimSpace(h.envSecretsCloudflareStoreID)
		if storeID == "" {
			return ""
		}
		name := strings.NewReplacer("/", "_", " ", "_").Replace(base)
		return fmt.Sprintf("%s:%s", storeID, name)
	}

	return base
}
