package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

const (
	// defaultMaxArtifactUploadSize is the community-edition per-file limit (200 MB).
	// Applied when no entitlements checker is configured.
	defaultMaxArtifactUploadSize = 200 << 20

	// maxArtifactUploadHardCap is an absolute ceiling for http.MaxBytesReader
	// to prevent DoS regardless of plan. Plan-specific limits are enforced
	// after form parsing.
	maxArtifactUploadHardCap = 2 << 30 // 2 GB
)

// ListRunArtifacts godoc
//
//	@Summary		List run artifacts
//	@Description	Lists artifacts for a workflow run (optionally filtered by task name)
//	@Tags			artifacts
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Param			task		query		string	false	"Task name"
//	@Param			limit		query		int		false	"Max items (<=1000)"
//	@Param			offset		query		int		false	"Offset for pagination"
//	@Success		200			{array}		ArtifactResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/artifacts [get]
func (h *Handler) ListRunArtifacts(w http.ResponseWriter, r *http.Request) {
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

	runID, err := getRunIDFromPath(r)
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
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get run"))
		return
	}
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	if h.artifactService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("artifact service not configured"))
		return
	}

	taskName := r.URL.Query().Get("task")
	limit, offset := parsePagination(r)

	artifacts, err := h.artifactService.List(ctx, runID.String(), taskName, limit, offset)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	resp := make([]ArtifactResponse, 0, len(artifacts))
	for _, a := range artifacts {
		resp = append(resp, artifactModelToResponse(a))
	}
	_ = response.Ok(w, r, "artifacts", resp)
}

// UploadArtifact godoc
//
//	@Summary		Upload an artifact
//	@Description	Uploads an artifact file for a workflow run
//	@Tags			artifacts
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Param			file		formData	file	true	"Artifact file"
//	@Param			name		formData	string	false	"Artifact display name (defaults to filename)"
//	@Param			task_name	formData	string	false	"Task name"
//	@Param			ttl_seconds	formData	int		false	"TTL in seconds"
//	@Success		201			{object}	ArtifactResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		413			{object}	ErrorResponse
//	@Failure		415			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/artifacts [post]
func (h *Handler) UploadArtifact(w http.ResponseWriter, r *http.Request) {
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
	runID, err := getRunIDFromPath(r)
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
	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to upload artifacts"))
		return
	}

	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get run"))
		return
	}
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	if h.artifactService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("artifact service not configured"))
		return
	}

	// Hard cap prevents unbounded reads regardless of plan.
	// Plan-specific upload size limits are enforced after form parsing.
	r.Body = http.MaxBytesReader(w, r.Body, maxArtifactUploadHardCap+1<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB in-memory; larger files spill to temp
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			_ = response.RequestEntityTooLarge(w, r, fmt.Errorf("file exceeds maximum upload size"))
			return
		}
		_ = response.BadRequest(w, r, fmt.Errorf("invalid multipart form: %w", err))
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("file is required"))
		return
	}
	defer func() { _ = file.Close() }()

	// Enforce plan-specific upload size limit.
	if h.entitlements != nil {
		if err := h.entitlements.CheckQuota(ctx, "max_artifact_upload_size", projectID, fileHeader.Size); err != nil {
			if entitlement.IsQuotaError(err) {
				_ = response.RequestEntityTooLarge(w, r, err)
				return
			}
		}
	} else if fileHeader.Size >= defaultMaxArtifactUploadSize {
		_ = response.RequestEntityTooLarge(w, r, fmt.Errorf("file exceeds maximum upload size of %d MB", defaultMaxArtifactUploadSize>>20))
		return
	}

	taskName := r.FormValue("task_name")
	name := r.FormValue("name")
	if name == "" {
		name = fileHeader.Filename
	}

	ttl := time.Duration(0)
	if ttlStr := r.FormValue("ttl_seconds"); ttlStr != "" {
		if secs, err := strconv.Atoi(ttlStr); err == nil && secs > 0 {
			ttl = time.Duration(secs) * time.Second
		}
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if ct := r.FormValue("content_type"); ct != "" {
		contentType = ct
	}
	if !isAllowedContentType(contentType) {
		_ = response.BadRequest(w, r, errors.New("unsupported content type"))
		return
	}

	var extraMetadata json.RawMessage
	if metaStr := r.FormValue("metadata"); metaStr != "" {
		if json.Valid([]byte(metaStr)) {
			extraMetadata = json.RawMessage(metaStr)
		}
	}

	artifact, err := h.artifactService.Upload(ctx, projectID, runID, taskName, name, filepath.Base(fileHeader.Filename), file, fileHeader.Size, ttl, contentType, extraMetadata)
	if err != nil {
		if entitlement.IsQuotaError(err) {
			_ = response.PaymentRequired(w, r, err)
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Created(w, r, "artifact uploaded", artifactModelToResponse(artifact))
}

// GetArtifact godoc
//
//	@Summary		Get artifact metadata
//	@Description	Returns artifact metadata for a run
//	@Tags			artifacts
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Param			artifactID	path		string	true	"Artifact ID"	format(uuid)
//	@Success		200			{object}	ArtifactResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/artifacts/{artifactID} [get]
func (h *Handler) GetArtifact(w http.ResponseWriter, r *http.Request) {
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

	runID, err := getRunIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	artifactID := chi.URLParam(r, "artifactID")

	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get run"))
		return
	}
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	if h.artifactService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("artifact service not configured"))
		return
	}

	artifact, err := h.artifactService.Get(ctx, artifactID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("artifact not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	if artifact.RunID != runID {
		_ = response.NotFound(w, r, errors.New("artifact not found for this run"))
		return
	}

	_ = response.Ok(w, r, "artifact", artifactModelToResponse(artifact))
}

// DownloadArtifact godoc
//
//	@Summary		Download artifact content
//	@Description	Downloads the artifact file content
//	@Tags			artifacts
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		application/octet-stream
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Param			artifactID	path		string	true	"Artifact ID"	format(uuid)
//	@Success		200			{string}	string	"Artifact content"
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/artifacts/{artifactID}/download [get]
func (h *Handler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
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
	runID, err := getRunIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	artifactID := chi.URLParam(r, "artifactID")

	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get run"))
		return
	}
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	if h.artifactService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("artifact service not configured"))
		return
	}

	artifact, err := h.artifactService.Get(ctx, artifactID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("artifact not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	if artifact.RunID != runID {
		_ = response.NotFound(w, r, errors.New("artifact not found for this run"))
		return
	}

	rc, err := h.artifactService.Download(ctx, artifactID)
	if err != nil {
		if entitlement.IsQuotaError(err) {
			_ = response.PaymentRequired(w, r, err)
			return
		}
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("artifact not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", artifact.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+artifact.FileName+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

// DeleteArtifact godoc
//
//	@Summary		Delete an artifact
//	@Description	Deletes an artifact and its stored content
//	@Tags			artifacts
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Param			artifactID	path		string	true	"Artifact ID"	format(uuid)
//	@Success		204			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/artifacts/{artifactID} [delete]
func (h *Handler) DeleteArtifact(w http.ResponseWriter, r *http.Request) {
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
	runID, err := getRunIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	artifactID := chi.URLParam(r, "artifactID")

	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}
	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to delete artifacts"))
		return
	}

	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get run"))
		return
	}
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	if h.artifactService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("artifact service not configured"))
		return
	}

	artifact, err := h.artifactService.Get(ctx, artifactID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("artifact not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	if artifact.RunID != runID {
		_ = response.NotFound(w, r, errors.New("artifact not found for this run"))
		return
	}

	if err := h.artifactService.Delete(ctx, artifactID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("artifact not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.NoContent(w, r)
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 100
	offset = 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func artifactModelToResponse(a *models.Artifact) ArtifactResponse {
	resp := ArtifactResponse{
		ID:          a.ID,
		ProjectID:   a.ProjectID,
		RunID:       a.RunID,
		Name:        a.Name,
		FileName:    a.FileName,
		ContentType: a.ContentType,
		SizeBytes:   a.SizeBytes,
		StorageKey:  a.StorageKey,
		CreatedAt:   a.CreatedAt,
		Metadata:    a.Metadata,
	}
	if a.TaskName != nil {
		resp.TaskName = *a.TaskName
	}
	if a.DigestSHA256 != nil {
		resp.DigestSHA256 = *a.DigestSHA256
	}
	if a.ExpiresAt != nil {
		resp.ExpiresAt = a.ExpiresAt
	}
	return resp
}

func isAllowedContentType(contentType string) bool {
	if contentType == "" {
		return true
	}
	if strings.HasPrefix(contentType, "application/") {
		return true
	}
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	if strings.HasPrefix(contentType, "video/") {
		return true
	}
	if strings.HasPrefix(contentType, "audio/") {
		return true
	}
	return false
}
