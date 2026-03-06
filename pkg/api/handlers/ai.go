package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/worker"
)

// GetAIAnalysis godoc
//
//	@Summary		Get AI analysis for a run
//	@Description	Returns the latest AI analysis for a run, including publications and suggestions
//	@Tags			ai
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runId		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runId}/ai-analysis [get]
func (h *Handler) GetAIAnalysis(w http.ResponseWriter, r *http.Request) {
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

	// Check user has access to project.
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

	if h.store.AI == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("AI service not configured"))
		return
	}

	analysis, err := h.store.AI.GetAnalysisByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("no AI analysis found for this run"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI analysis"))
		return
	}

	// Fetch associated publications.
	publications, err := h.store.AI.ListPublicationsByAnalysis(ctx, analysis.ID)
	if err != nil {
		publications = nil // Non-fatal.
	}

	// Fetch suggestions if available.
	suggestions, err := h.store.AI.ListSuggestionsByAnalysis(ctx, analysis.ID)
	if err != nil {
		suggestions = nil // Non-fatal.
	}

	_ = response.Ok(w, r, "AI analysis retrieved", map[string]interface{}{
		"analysis":     analysis,
		"publications": publications,
		"suggestions":  suggestions,
	})
}

// ListAIAnalyses godoc
//
//	@Summary		List AI analyses for a project
//	@Description	Returns paginated AI analyses for a project
//	@Tags			ai
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"		format(uuid)
//	@Param			limit		query		int		false	"Items per page"	default(20)	maximum(100)
//	@Param			offset		query		int		false	"Offset"			default(0)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/ai-analyses [get]
func (h *Handler) ListAIAnalyses(w http.ResponseWriter, r *http.Request) {
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
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	if h.store.AI == nil {
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

	analyses, total, err := h.store.AI.ListAnalysesByProject(ctx, projectID, limit, offset)
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

// RetryAIAnalysis godoc
//
//	@Summary		Retry AI analysis for a run
//	@Description	Re-enqueues an AI analysis job for a specific run
//	@Tags			ai
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runId		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runId}/ai-analysis/retry [post]
func (h *Handler) RetryAIAnalysis(w http.ResponseWriter, r *http.Request) {
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
		_ = response.Forbidden(w, r, errors.New("you don't have permission to trigger actions"))
		return
	}

	if h.jobClient == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("job system not configured"))
		return
	}

	// Fetch the run to include metadata in the analysis payload.
	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch run"))
		return
	}

	// Build and enqueue AI analysis job.
	payload := worker.AIAnalysisPayload{
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
	if err := h.jobClient.Enqueue(worker.QueueNameDefault, worker.AIAnalysisTaskName, &worker.ClientPayload{Data: data}); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to enqueue AI analysis job"))
		return
	}

	_ = response.Ok(w, r, "AI analysis retry enqueued", nil)
}

// GetAISuggestions godoc
//
//	@Summary		Get AI suggestions for a run
//	@Description	Returns AI-generated suggestions for a specific run
//	@Tags			ai
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runId		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runId}/ai-suggestions [get]
func (h *Handler) GetAISuggestions(w http.ResponseWriter, r *http.Request) {
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

	// Fetch analysis for this run first.
	analysis, err := h.store.AI.GetAnalysisByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("no AI analysis found for this run"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI analysis"))
		return
	}

	suggestions, err := h.store.AI.ListSuggestionsByAnalysis(ctx, analysis.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI suggestions"))
		return
	}

	_ = response.Ok(w, r, "AI suggestions retrieved", map[string]interface{}{
		"suggestions": suggestions,
		"analysis_id": analysis.ID,
	})
}

// PostAISuggestions godoc
//
//	@Summary		Publish AI suggestions
//	@Description	Publishes AI suggestions as a GitHub PR review
//	@Tags			ai
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runId		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runId}/ai-suggestions/post [post]
func (h *Handler) PostAISuggestions(w http.ResponseWriter, r *http.Request) {
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
		_ = response.Forbidden(w, r, errors.New("you don't have permission to trigger actions"))
		return
	}

	if h.jobClient == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("job system not configured"))
		return
	}

	// Fetch analysis for this run.
	analysis, err := h.store.AI.GetAnalysisByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("no AI analysis found for this run"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to fetch AI analysis"))
		return
	}

	// Enqueue suggestion publish job.
	payload := worker.AISuggestPublishPayload{
		AnalysisID: analysis.ID.String(),
		RunID:      runID.String(),
		ProjectID:  projectID.String(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to marshal payload"))
		return
	}
	if err := h.jobClient.Enqueue(worker.QueueNameDefault, worker.AISuggestPublishTaskName, &worker.ClientPayload{Data: data}); err != nil {
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
