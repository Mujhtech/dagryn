package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// CheckCache checks whether a cache entry exists for the given task/key.
// GET /api/v1/projects/{projectId}/cache/{taskName}/{cacheKey}
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

// UploadCache stores cache content for the given task/key.
// PUT /api/v1/projects/{projectId}/cache/{taskName}/{cacheKey}
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
		if service.IsQuotaExceeded(err) {
			_ = response.PaymentRequired(w, r, err)
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Created(w, r, "cache entry created", nil)
}

// DownloadCache retrieves cache content for the given task/key.
// GET /api/v1/projects/{projectId}/cache/{taskName}/{cacheKey}/download
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
		if service.IsQuotaExceeded(err) {
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

// DeleteCache removes a cache entry.
// DELETE /api/v1/projects/{projectId}/cache/{taskName}/{cacheKey}
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

// GetCacheStats returns aggregate cache statistics for a project.
// GET /api/v1/projects/{projectId}/cache/stats
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

// TriggerCacheGC triggers garbage collection for a project's cache.
// POST /api/v1/projects/{projectId}/cache/gc
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
	_ = response.Ok(w, r, "cache GC completed", result)
}

// GetCacheAnalytics returns daily cache usage analytics for a project.
// GET /api/v1/projects/{projectId}/cache/analytics?days=30
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
