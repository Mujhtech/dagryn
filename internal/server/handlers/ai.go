package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/job"
	serverctx "github.com/mujhtech/dagryn/internal/server/context"
	"github.com/mujhtech/dagryn/internal/server/response"
)

// GetAIAnalysis returns the latest AI analysis for a run.
// GET /api/v1/projects/{projectID}/runs/{runID}/ai-analysis
func (h *Handler) GetAIAnalysis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid run ID"))
		return
	}

	// Check user has access to project.
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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

	if h.aiRepo == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("AI service not configured"))
		return
	}

	analysis, err := h.aiRepo.GetAnalysisByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("no AI analysis found for this run"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI analysis"))
		return
	}

	// Fetch associated publications.
	publications, err := h.aiRepo.ListPublicationsByAnalysis(ctx, analysis.ID)
	if err != nil {
		publications = nil // Non-fatal.
	}

	// Fetch suggestions if available.
	suggestions, err := h.aiRepo.ListSuggestionsByAnalysis(ctx, analysis.ID)
	if err != nil {
		suggestions = nil // Non-fatal.
	}

	_ = response.Ok(w, r, "AI analysis retrieved", map[string]interface{}{
		"analysis":     analysis,
		"publications": publications,
		"suggestions":  suggestions,
	})
}

// ListAIAnalyses returns paginated AI analyses for a project.
// GET /api/v1/projects/{projectID}/ai-analyses
func (h *Handler) ListAIAnalyses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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

	if h.aiRepo == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("AI service not configured"))
		return
	}

	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parsePositiveInt(v); err == nil && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			offset = n
		}
	}

	analyses, total, err := h.aiRepo.ListAnalysesByProject(ctx, projectID, limit, offset)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list AI analyses"))
		return
	}

	_ = response.Ok(w, r, "AI analyses retrieved", map[string]interface{}{
		"analyses": analyses,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// RetryAIAnalysis re-enqueues an AI analysis job for a run.
// POST /api/v1/projects/{projectID}/runs/{runID}/ai-analysis/retry
func (h *Handler) RetryAIAnalysis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid run ID"))
		return
	}

	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}
	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to trigger actions"))
		return
	}

	if h.jobClient == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("job system not configured"))
		return
	}

	// Fetch the run to include metadata in the analysis payload.
	run, err := h.runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch run"))
		return
	}

	// Build and enqueue AI analysis job.
	payload := job.AIAnalysisPayload{
		RunID:     runID.String(),
		ProjectID: projectID.String(),
	}
	if run.GitBranch != nil {
		payload.GitBranch = *run.GitBranch
	}
	if run.GitCommit != nil {
		payload.GitCommit = *run.GitCommit
	}
	if run.WorkflowName != nil {
		payload.WorkflowName = *run.WorkflowName
	}

	data, err := json.Marshal(payload)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to marshal payload"))
		return
	}
	if err := h.jobClient.Enqueue(job.QueueNameDefault, job.AIAnalysisTaskName, &job.ClientPayload{Data: data}); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to enqueue AI analysis job"))
		return
	}

	_ = response.Ok(w, r, "AI analysis retry enqueued", nil)
}

// GetAISuggestions returns AI suggestions for a run.
// GET /api/v1/projects/{projectID}/runs/{runID}/ai-suggestions
func (h *Handler) GetAISuggestions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid run ID"))
		return
	}

	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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

	if h.aiRepo == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("AI service not configured"))
		return
	}

	// Fetch analysis for this run first.
	analysis, err := h.aiRepo.GetAnalysisByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("no AI analysis found for this run"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI analysis"))
		return
	}

	suggestions, err := h.aiRepo.ListSuggestionsByAnalysis(ctx, analysis.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI suggestions"))
		return
	}

	_ = response.Ok(w, r, "AI suggestions retrieved", map[string]interface{}{
		"suggestions": suggestions,
		"analysis_id": analysis.ID,
	})
}

// PostAISuggestions publishes AI suggestions as a GitHub PR review.
// POST /api/v1/projects/{projectID}/runs/{runID}/ai-suggestions/post
func (h *Handler) PostAISuggestions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid run ID"))
		return
	}

	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}
	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to trigger actions"))
		return
	}

	if h.jobClient == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("job system not configured"))
		return
	}

	// Fetch analysis for this run.
	analysis, err := h.aiRepo.GetAnalysisByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("no AI analysis found for this run"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI analysis"))
		return
	}

	// Enqueue suggestion publish job.
	payload := job.AISuggestPublishPayload{
		AnalysisID: analysis.ID.String(),
		RunID:      runID.String(),
		ProjectID:  projectID.String(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to marshal payload"))
		return
	}
	if err := h.jobClient.Enqueue(job.QueueNameDefault, job.AISuggestPublishTaskName, &job.ClientPayload{Data: data}); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to enqueue suggestion publish job"))
		return
	}

	_ = response.Ok(w, r, "AI suggestion publish enqueued", nil)
}

// parsePositiveInt parses a string as a positive integer.
func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n < 0 {
		return 0, errors.New("invalid positive integer")
	}
	return n, nil
}
