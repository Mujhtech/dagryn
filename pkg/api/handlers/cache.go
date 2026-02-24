package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/service"
)

// CheckCache godoc
//
//	@Summary		Check cache entry
//	@Description	Checks whether a cache entry exists for the given task and key
//	@Tags			cache
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			taskName	path		string	true	"Task name"
//	@Param			cacheKey	path		string	true	"Cache key"
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/cache/{taskName}/{cacheKey} [get]
func (h *Handler) CheckCache(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	taskName, _ := pathParam(r, TaskNameParam)
	cacheKey, _ := pathParam(r, CacheKeyParam)

	if h.cacheService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("cache service not configured"))
		return
	}

	exists, err := h.cacheService.Check(r.Context(), projectID, taskName, cacheKey)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	if !exists {
		_ = response.NotFound(w, r, errors.New("cache entry not found"))
		return
	}
	_ = response.Ok(w, r, "cache hit", nil)
}

// UploadCache godoc
//
//	@Summary		Upload cache entry
//	@Description	Stores cache content for the given task and key
//	@Tags			cache
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			application/octet-stream
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			taskName	path		string	true	"Task name"
//	@Param			cacheKey	path		string	true	"Cache key"
//	@Success		201			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		402			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/cache/{taskName}/{cacheKey} [put]
func (h *Handler) UploadCache(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	taskName, _ := pathParam(r, TaskNameParam)
	cacheKey, _ := pathParam(r, CacheKeyParam)

	if h.cacheService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("cache service not configured"))
		return
	}

	if r.Body == nil {
		_ = response.BadRequest(w, r, errors.New("request body is required"))
		return
	}
	defer func() { _ = r.Body.Close() }()

	if err := h.cacheService.Upload(r.Context(), projectID, taskName, cacheKey, r.Body, r.ContentLength); err != nil {
		if entitlement.IsQuotaError(err) {
			_ = response.PaymentRequired(w, r, err)
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Created(w, r, "cache entry created", nil)
}

// DownloadCache godoc
//
//	@Summary		Download cache entry
//	@Description	Retrieves cache content for the given task and key as a binary stream
//	@Tags			cache
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		application/octet-stream
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			taskName	path		string	true	"Task name"
//	@Param			cacheKey	path		string	true	"Cache key"
//	@Success		200			{file}		binary
//	@Failure		400			{object}	ErrorResponse
//	@Failure		402			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/cache/{taskName}/{cacheKey}/download [get]
func (h *Handler) DownloadCache(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	taskName, _ := pathParam(r, TaskNameParam)
	cacheKey, _ := pathParam(r, CacheKeyParam)

	if h.cacheService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("cache service not configured"))
		return
	}

	rc, err := h.cacheService.Download(r.Context(), projectID, taskName, cacheKey)
	if err != nil {
		if entitlement.IsQuotaError(err) {
			_ = response.PaymentRequired(w, r, err)
			return
		}
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("cache entry not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

// DeleteCache godoc
//
//	@Summary		Delete cache entry
//	@Description	Removes a cache entry for the given task and key
//	@Tags			cache
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			taskName	path		string	true	"Task name"
//	@Param			cacheKey	path		string	true	"Cache key"
//	@Success		204			{string}	string	"No content"
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/cache/{taskName}/{cacheKey} [delete]
func (h *Handler) DeleteCache(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	taskName, _ := pathParam(r, TaskNameParam)
	cacheKey, _ := pathParam(r, CacheKeyParam)

	if h.cacheService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("cache service not configured"))
		return
	}

	if err := h.cacheService.Delete(r.Context(), projectID, taskName, cacheKey); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("cache entry not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.NoContent(w, r)
}

// GetCacheStats godoc
//
//	@Summary		Get cache statistics
//	@Description	Returns aggregate cache statistics for a project
//	@Tags			cache
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/cache/stats [get]
func (h *Handler) GetCacheStats(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if h.cacheService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("cache service not configured"))
		return
	}

	stats, err := h.cacheService.GetStats(r.Context(), projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "cache stats", stats)
}

// TriggerCacheGC godoc
//
//	@Summary		Trigger cache garbage collection
//	@Description	Triggers garbage collection for a project's cache
//	@Tags			cache
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/cache/gc [post]
func (h *Handler) TriggerCacheGC(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if h.cacheService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("cache service not configured"))
		return
	}

	result, err := h.cacheService.RunGC(r.Context(), projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	// Audit log: cache cleared
	if h.auditService != nil {
		ctx := r.Context()
		user := apiCtx.GetUser(ctx)
		if user != nil {
			project, _ := h.store.Projects.GetByID(ctx, projectID)
			if project != nil && project.TeamID != nil {
				h.auditService.Log(ctx, service.AuditEntry{
					TeamID:       *project.TeamID,
					ProjectID:    &projectID,
					Action:       models.AuditActionCacheCleared,
					Category:     models.AuditCategoryCache,
					ResourceType: "cache",
					ResourceID:   projectID.String(),
					Description:  "Cache garbage collection triggered",
				})
			}
		}
	}

	_ = response.Ok(w, r, "cache GC completed", result)
}

// GetCacheAnalytics godoc
//
//	@Summary		Get cache analytics
//	@Description	Returns daily cache usage analytics for a project
//	@Tags			cache
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"		format(uuid)
//	@Param			days		query		int		false	"Number of days"	default(30)	maximum(365)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/cache/analytics [get]
func (h *Handler) GetCacheAnalytics(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if h.cacheService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("cache service not configured"))
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	analytics, err := h.cacheService.GetAnalytics(r.Context(), projectID, days)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "cache analytics", analytics)
}
