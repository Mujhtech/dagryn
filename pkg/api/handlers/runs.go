package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/notification"
	"github.com/mujhtech/dagryn/pkg/server/sse"
	"github.com/mujhtech/dagryn/pkg/worker"
)

// ListRuns godoc
//
//	@Summary		List project runs
//	@Description	Returns all workflow runs for a project
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"		format(uuid)
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			per_page	query		int		false	"Items per page"	default(20)	maximum(100)
//	@Param			status		query		string	false	"Filter by status"	Enums(pending, running, success, failed, cancelled)
//	@Success		200			{object}	PaginatedResponse{data=[]RunResponse}
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs [get]
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to view runs
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	page, perPage := GetPageParams(r)
	offset := (page - 1) * perPage

	runs, total, err := h.store.Runs.ListByProject(ctx, projectID, perPage, offset)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list runs"))
		return
	}

	// Filter by status if provided
	statusFilter := r.URL.Query().Get("status")
	resp := make([]RunResponse, 0, len(runs))
	for _, run := range runs {
		if statusFilter != "" && string(run.Status) != statusFilter {
			continue
		}
		resp = append(resp, h.runModelToResponseWithUser(ctx, &run))
	}

	_ = response.Ok(w, r, "Success", PaginatedResponse{
		Data: resp,
		Meta: PaginationMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      int64(total),
			TotalPages: CalculateTotalPages(int64(total), perPage),
		},
	})
}

// GetRunDashboardSummary godoc
//
//	@Summary		Get non-paginated run dashboard summary
//	@Description	Returns stable chart and facet data for the project run dashboard.
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"								format(uuid)
//	@Param			days		query		int		false	"Number of trailing days for chart data"	default(30)
//	@Success		200			{object}	RunDashboardSummaryResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/summary [get]
func (h *Handler) GetRunDashboardSummary(w http.ResponseWriter, r *http.Request) {
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

	days := 30
	if rawDays := r.URL.Query().Get("days"); rawDays != "" {
		if parsed, err := strconv.Atoi(rawDays); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	chartPoints, err := h.store.Runs.GetDashboardChartByProject(ctx, projectID, days)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to load run chart summary"))
		return
	}

	facets, err := h.store.Runs.GetDashboardFacetsByProject(ctx, projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to load run facets"))
		return
	}

	resp := RunDashboardSummaryResponse{
		Chart:        make([]RunDashboardChartPointResponse, 0, len(chartPoints)),
		Users:        make([]RunDashboardUserFacetResponse, 0, len(facets.Users)),
		Workflows:    facets.Workflows,
		Branches:     facets.Branches,
		StatusCounts: facets.StatusCount,
	}

	for _, point := range chartPoints {
		resp.Chart = append(resp.Chart, RunDashboardChartPointResponse{
			Date:       point.Date.Format("2006-01-02"),
			Success:    point.Success,
			Failed:     point.Failed,
			DurationMs: point.DurationMs,
		})
	}

	for _, userFacet := range facets.Users {
		userResp := RunDashboardUserFacetResponse{
			ID:   userFacet.ID,
			Name: userFacet.Name,
		}
		if userFacet.AvatarURL != nil {
			userResp.AvatarURL = *userFacet.AvatarURL
		}
		resp.Users = append(resp.Users, userResp)
	}

	_ = response.Ok(w, r, "Success", resp)
}

// GetRun godoc
//
//	@Summary		Get a run
//	@Description	Returns a workflow run by ID
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	RunResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID} [get]
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to view runs
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

	// Verify the run belongs to the project
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	_ = response.Ok(w, r, "Success", h.runModelToResponseWithUser(ctx, run))
}

// GetRunTasks godoc
//
//	@Summary		Get run tasks
//	@Description	Returns all task results for a workflow run
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	[]TaskResultResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/tasks [get]
func (h *Handler) GetRunTasks(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to view runs
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	// Verify the run exists and belongs to the project
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

	tasks, err := h.store.Runs.ListTaskResults(ctx, runID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list task results"))
		return
	}

	resp := make([]TaskResultResponse, 0, len(tasks))
	for _, task := range tasks {
		resp = append(resp, taskResultModelToResponse(&task))
	}

	_ = response.Ok(w, r, "Success", resp)
}

// StreamRunLogs godoc
//
//	@Summary		Stream run logs
//	@Description	Streams logs for a workflow run using Server-Sent Events
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		text/event-stream
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{string}	string	"SSE stream"
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/logs [get]
func (h *Handler) StreamRunLogs(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to view runs
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	// Verify the run exists and belongs to the project
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

	// Subscribe to log events for this run
	topics := []string{
		fmt.Sprintf("logs:%s", runID),
	}
	sse.ServeSSE(w, r, h.sseHub, topics)
}

// TriggerRun godoc
//
//	@Summary		Trigger a workflow run
//	@Description	Creates and queues a new workflow run for execution
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string				true	"Project ID"	format(uuid)
//	@Param			body		body		TriggerRunRequest	true	"Trigger run request"
//	@Success		201			{object}	TriggerRunResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs [post]
func (h *Handler) TriggerRun(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to trigger runs
	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to trigger runs"))
		return
	}

	var req TriggerRunRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Load project early (needed for git-linked metadata + optional enqueue)
	project, err := h.store.Projects.GetByID(ctx, projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to load project"))
		return
	}

	// Check concurrent runs quota
	if h.entitlements != nil {
		if err := h.entitlements.CheckQuota(ctx, "concurrent_runs", projectID, 0); err != nil {
			if entitlement.IsQuotaError(err) {
				_ = response.PaymentRequired(w, r, err)
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check quota"))
			return
		}
	}

	// Create the run record
	run := &models.Run{
		ID:                uuid.New(),
		ProjectID:         projectID,
		Targets:           req.Targets,
		Status:            models.RunStatusPending,
		TotalTasks:        0, // Will be updated when run starts
		TriggeredBy:       models.TriggerSourceAPI,
		TriggeredByUserID: &user.ID,
		CreatedAt:         time.Now(),
	}

	// Link default workflow snapshot (best-effort)
	if wf, _ := h.store.Workflows.GetDefaultByProject(ctx, projectID); wf != nil {
		run.WorkflowID = &wf.ID
		run.WorkflowName = &wf.Name
	}

	// Add git info if provided
	if req.GitBranch != "" {
		run.GitBranch = &req.GitBranch
	}
	if req.GitCommit != "" {
		run.GitCommit = &req.GitCommit
	}

	// Add host info if provided
	if req.HostOS != "" {
		run.HostOS = &req.HostOS
	}
	if req.HostArch != "" {
		run.HostArch = &req.HostArch
	}
	if req.HostName != "" {
		run.HostName = &req.HostName
	}
	if req.Description != "" {
		run.Description = &req.Description
	}

	// For git-linked projects (GitHub for now), fetch last commit metadata when triggering from dashboard/API.
	// This enriches run rows with commit sha/message/author even when user didn't provide git_commit.
	if project.RepoURL != nil && *project.RepoURL != "" && strings.Contains(*project.RepoURL, "github.com") {
		h.enrichRunWithGitHubCommit(ctx, run, project, user.ID, req.GitBranch)
	}

	// Create the run in the database
	if err := h.store.Runs.Create(ctx, run); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create run"))
		return
	}

	// Enqueue server-side execution when job client is available and project has a repo URL.
	// Skip if SyncOnly is true (CLI is executing locally and just syncing status).
	if h.jobClient != nil && !req.SyncOnly {
		if project.RepoURL != nil && *project.RepoURL != "" {
			repoURL := *project.RepoURL
			payload := worker.ExecuteRunPayload{
				ProjectID: projectID.String(),
				RunID:     run.ID.String(),
				Targets:   req.Targets,
				GitBranch: req.GitBranch,
				GitCommit: req.GitCommit,
				RepoURL:   repoURL,
			}
			data, err := json.Marshal(payload)
			if err == nil {
				// Route to priority queue if the entitlement supports it
				queue := worker.QueueNameDefault
				if h.entitlements != nil && h.entitlements.HasFeature(ctx, "priority_queue") {
					queue = worker.QueueNamePriority
				}
				_ = h.jobClient.Enqueue(queue, worker.ExecuteRunTaskName, &worker.ClientPayload{Data: data})
			}
		}
	}

	// Build stream URLs
	baseURL := getBaseURL(r)
	streamURL := fmt.Sprintf("%s/api/v1/projects/%s/runs/%s/events", baseURL, projectID, run.ID)
	logsURL := fmt.Sprintf("%s/api/v1/projects/%s/runs/%s/logs", baseURL, projectID, run.ID)

	_ = response.Created(w, r, "Created successfully", TriggerRunResponse{
		RunID:     run.ID,
		Status:    string(run.Status),
		Message:   "Run queued successfully",
		StreamURL: streamURL,
		LogsURL:   logsURL,
	})
}

func (h *Handler) enrichRunWithGitHubCommit(ctx context.Context, run *models.Run, project *models.Project, currentUserID uuid.UUID, requestedBranch string) {
	if h.providerEncrypt == nil {
		return
	}
	if project.RepoURL == nil || *project.RepoURL == "" {
		return
	}

	// Prefer the user who linked the repo (stable access), fallback to current user.
	tokenUserID := currentUserID
	if project.RepoLinkedByUserID != nil {
		tokenUserID = *project.RepoLinkedByUserID
	}

	tok, err := h.store.ProviderTokens.GetByUserAndProvider(ctx, tokenUserID, "github")
	if err != nil || tok == nil {
		return
	}
	accessToken, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
	if err != nil {
		return
	}

	owner, repoName, err := parseGitHubOwnerRepo(*project.RepoURL)
	if err != nil {
		return
	}

	branch := strings.TrimSpace(requestedBranch)
	if branch == "" {
		// Resolve default branch
		u := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repoName)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			_, _ = io.ReadAll(resp.Body)
			return
		}

		var repoResp struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&repoResp); err != nil {
			return
		}
		branch = strings.TrimSpace(repoResp.DefaultBranch)
	}
	if branch == "" {
		return
	}

	// Fetch the latest commit for the branch
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repoName, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return
	}

	var commitResp struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
		Author struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"author"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commitResp); err != nil {
		return
	}

	// Populate run fields if missing
	if run.GitBranch == nil || *run.GitBranch == "" {
		run.GitBranch = &branch
	}
	if run.GitCommit == nil || *run.GitCommit == "" {
		if commitResp.SHA != "" {
			sha := commitResp.SHA
			run.GitCommit = &sha
		}
	}
	if run.CommitMessage == nil || *run.CommitMessage == "" {
		if commitResp.Commit.Message != "" {
			msg := commitResp.Commit.Message
			run.CommitMessage = &msg
		}
	}
	if run.CommitAuthorName == nil || *run.CommitAuthorName == "" {
		if commitResp.Commit.Author.Name != "" {
			n := commitResp.Commit.Author.Name
			run.CommitAuthorName = &n
		}
	}
	if run.CommitAuthorEmail == nil || *run.CommitAuthorEmail == "" {
		if commitResp.Commit.Author.Email != "" {
			e := commitResp.Commit.Author.Email
			run.CommitAuthorEmail = &e
		}
	}
	if run.CommitAuthorAvatarURL == nil || *run.CommitAuthorAvatarURL == "" {
		if commitResp.Author.AvatarURL != "" {
			a := commitResp.Author.AvatarURL
			run.CommitAuthorAvatarURL = &a
		}
	}
}

// enrichRunWithGitHubPR attempts to populate PR metadata for a run using the GitHub API.
func (h *Handler) enrichRunWithGitHubPR(ctx context.Context, run *models.Run, project *models.Project, currentUserID uuid.UUID) {
	if h.providerEncrypt == nil {
		return
	}
	if project.RepoURL == nil || *project.RepoURL == "" {
		return
	}
	if run.PRNumber != nil {
		return
	}

	// Prefer the user who linked the repo (stable access), fallback to current user.
	tokenUserID := currentUserID
	if project.RepoLinkedByUserID != nil {
		tokenUserID = *project.RepoLinkedByUserID
	}

	tok, err := h.store.ProviderTokens.GetByUserAndProvider(ctx, tokenUserID, "github")
	if err != nil || tok == nil {
		return
	}
	accessToken, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
	if err != nil {
		return
	}

	h.enrichRunWithGitHubPRUsingToken(ctx, run, project, accessToken)
}

// enrichRunWithGitHubPRUsingToken populates PR metadata using a provided GitHub access token.
// It first tries to find a PR by commit SHA, then falls back to searching by branch name.
func (h *Handler) enrichRunWithGitHubPRUsingToken(ctx context.Context, run *models.Run, project *models.Project, accessToken string) {
	if project.RepoURL == nil || *project.RepoURL == "" {
		return
	}
	if run.PRNumber != nil {
		return
	}

	owner, repoName, err := parseGitHubOwnerRepo(*project.RepoURL)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Strategy 1: look up PRs by commit SHA.
	if run.GitCommit != nil && *run.GitCommit != "" {
		if pr := h.fetchPRByCommit(ctx, client, accessToken, owner, repoName, *run.GitCommit); pr != nil {
			run.PRNumber = &pr.Number
			if pr.Title != "" {
				run.PRTitle = &pr.Title
			}
			return
		}
	}

	// Strategy 2: look up open PRs by branch name.
	// Push webhooks may arrive before GitHub indexes the commit→PR association,
	// but searching by head branch is immediate.
	if run.GitBranch != nil && *run.GitBranch != "" {
		if pr := h.fetchPRByBranch(ctx, client, accessToken, owner, repoName, *run.GitBranch); pr != nil {
			run.PRNumber = &pr.Number
			if pr.Title != "" {
				run.PRTitle = &pr.Title
			}
			return
		}
	}
}

type ghPRRef struct {
	Number int
	Title  string
}

func (h *Handler) fetchPRByCommit(ctx context.Context, client *http.Client, token, owner, repo, sha string) *ghPRRef {
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s/pulls", owner, repo, sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return nil
	}

	var prs []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil || len(prs) == 0 || prs[0].Number == 0 {
		return nil
	}
	return &ghPRRef{Number: prs[0].Number, Title: prs[0].Title}
}

func (h *Handler) fetchPRByBranch(ctx context.Context, client *http.Client, token, owner, repo, branch string) *ghPRRef {
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&head=%s:%s&per_page=1",
		owner, repo, owner, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return nil
	}

	var prs []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil || len(prs) == 0 || prs[0].Number == 0 {
		return nil
	}
	return &ghPRRef{Number: prs[0].Number, Title: prs[0].Title}
}

// enrichRunWithGitHubCommitUsingToken enriches a run with commit metadata using a provided GitHub access token.
// This is used when we have an installation token from the GitHub App.
func (h *Handler) enrichRunWithGitHubCommitUsingToken(ctx context.Context, run *models.Run, project *models.Project, accessToken, requestedBranch string) {
	if project.RepoURL == nil || *project.RepoURL == "" {
		return
	}

	owner, repoName, err := parseGitHubOwnerRepo(*project.RepoURL)
	if err != nil {
		return
	}

	branch := strings.TrimSpace(requestedBranch)
	if branch == "" {
		// Resolve default branch
		u := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repoName)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			_, _ = io.ReadAll(resp.Body)
			return
		}

		var repoResp struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&repoResp); err != nil {
			return
		}
		branch = strings.TrimSpace(repoResp.DefaultBranch)
	}
	if branch == "" {
		return
	}

	// Fetch the latest commit for the branch
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repoName, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return
	}

	var commitResp struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
		Author struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"author"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commitResp); err != nil {
		return
	}

	// Populate run fields if missing
	if run.GitBranch == nil || *run.GitBranch == "" {
		run.GitBranch = &branch
	}
	if run.GitCommit == nil || *run.GitCommit == "" {
		if commitResp.SHA != "" {
			sha := commitResp.SHA
			run.GitCommit = &sha
		}
	}
	if run.CommitMessage == nil || *run.CommitMessage == "" {
		if commitResp.Commit.Message != "" {
			msg := commitResp.Commit.Message
			run.CommitMessage = &msg
		}
	}
	if run.CommitAuthorName == nil || *run.CommitAuthorName == "" {
		if commitResp.Commit.Author.Name != "" {
			n := commitResp.Commit.Author.Name
			run.CommitAuthorName = &n
		}
	}
	if run.CommitAuthorEmail == nil || *run.CommitAuthorEmail == "" {
		if commitResp.Commit.Author.Email != "" {
			e := commitResp.Commit.Author.Email
			run.CommitAuthorEmail = &e
		}
	}
	if run.CommitAuthorAvatarURL == nil || *run.CommitAuthorAvatarURL == "" {
		if commitResp.Author.AvatarURL != "" {
			a := commitResp.Author.AvatarURL
			run.CommitAuthorAvatarURL = &a
		}
	}
}

// notifyGitHubForRun updates GitHub commit status and posts a PR summary comment
// for runs that originated from GitHub PR events.
func (h *Handler) notifyGitHubForRun(ctx context.Context, projectID, runID uuid.UUID, status models.RunStatus) error {
	// Load run
	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.ProjectID != projectID {
		return nil
	}
	if run.PRNumber == nil {
		return nil
	}

	// Load project
	project, err := h.store.Projects.GetByID(ctx, projectID)
	if err != nil {
		return err
	}
	if project.RepoURL == nil || *project.RepoURL == "" {
		return nil
	}
	if run.GitCommit == nil || *run.GitCommit == "" {
		return nil
	}

	// Obtain access token - prefer GitHub App installation token
	var accessToken string

	// Try GitHub App installation token first
	if project.GitHubInstallationID != nil && h.githubApp != nil {
		instRecord, err := h.store.GitHubInstallations.GetByID(ctx, *project.GitHubInstallationID)
		if err == nil && instRecord != nil {
			token, err := h.githubApp.FetchInstallationToken(ctx, instRecord.InstallationID)
			if err == nil && token != nil {
				accessToken = token.Token
			}
		}
	}

	// Fallback to legacy OAuth token if no installation token was obtained
	if accessToken == "" && h.providerEncrypt != nil {
		tokenUserID := uuid.Nil
		if project.RepoLinkedByUserID != nil {
			tokenUserID = *project.RepoLinkedByUserID
		}
		tok, err := h.store.ProviderTokens.GetByUserAndProvider(ctx, tokenUserID, "github")
		if err == nil && tok != nil {
			decrypted, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
			if err == nil {
				accessToken = decrypted
			}
		}
	}

	// If no token available, cannot notify GitHub
	if accessToken == "" {
		return nil
	}

	owner, repoName, err := parseGitHubOwnerRepo(*project.RepoURL)
	if err != nil {
		return nil
	}

	sha := *run.GitCommit

	// Map Dagryn status to GitHub state
	// state := "pending"
	// switch status {
	// case models.RunStatusSuccess:
	// 	state = "success"
	// case models.RunStatusFailed:
	// 	state = "failure"
	// case models.RunStatusCancelled:
	// 	state = "error"
	// }

	// Build description
	// desc := fmt.Sprintf("Dagryn run %s", status)
	// if run.DurationMs != nil {
	// 	desc = fmt.Sprintf("Dagryn run %s in %s", status, formatDurationMs(*run.DurationMs))
	// }

	// Build target URL (link back to Dagryn run detail)
	targetURL := ""
	if h.baseURL != "" {
		targetURL = fmt.Sprintf("%s/projects/%s/runs/%s", strings.TrimRight(h.baseURL, "/"), projectID, runID)
	}

	// 1) Commit status
	// if err := notification.CommitStatus(ctx, accessToken, owner, repoName, sha, state, desc, targetURL); err != nil {
	// 	slog.Error("github_status_update_failed", "run_id", run.ID, "error", err)
	// }

	// 2) Check run (create/update)
	checkStatus, conclusion := mapGitHubCheckRunState(status)
	checkOutput := buildGitHubCheckRunOutput(run, status)
	if run.GitHubCheckRunID == nil || *run.GitHubCheckRunID == 0 {
		req := notification.CheckRunRequest{
			Name:       "Dagryn / workflow",
			HeadSHA:    sha,
			Status:     checkStatus,
			Conclusion: conclusion,
			DetailsURL: targetURL,
			Output:     checkOutput,
		}
		now := time.Now()
		if checkStatus == "in_progress" {
			req.StartedAt = &now
		}
		if checkStatus == "completed" {
			req.CompletedAt = &now
		}
		checkRunID, err := notification.CreateCheckRun(ctx, accessToken, owner, repoName, req)
		if err != nil {
			slog.Error("github_check_run_create_failed", "run_id", run.ID, "error", err)
		} else if checkRunID != 0 {
			run.GitHubCheckRunID = &checkRunID
			if err := h.store.Runs.UpdateGitHubCheckRunID(ctx, run.ID, checkRunID); err != nil {
				slog.Error("github_check_run_id_persist_failed", "run_id", run.ID, "error", err)
			}
		}
	} else {
		req := notification.CheckRunRequest{
			Status:     checkStatus,
			Conclusion: conclusion,
			DetailsURL: targetURL,
			Output:     checkOutput,
		}
		if checkStatus == "completed" {
			now := time.Now()
			req.CompletedAt = &now
		}
		if err := notification.UpdateCheckRun(ctx, accessToken, owner, repoName, *run.GitHubCheckRunID, req); err != nil {
			slog.Error("github_check_run_update_failed", "run_id", run.ID, "error", err)
		}
	}

	// 2) PR summary comment (create once, then update same comment)
	commentBody := map[string]string{
		"body": buildGitHubPRComment(run, status, targetURL),
	}

	if run.GitHubPRCommentID != nil {
		// Update existing comment
		commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%d", owner, repoName, *run.GitHubPRCommentID)
		if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPatch, commentURL, commentBody, nil); err != nil {
			slog.Error("github_pr_comment_update_failed", "run_id", runID, "error", err)
		}
	} else {
		// Create new comment and persist its ID
		commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repoName, *run.PRNumber)
		var respBody struct {
			ID int64 `json:"id"`
		}
		if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPost, commentURL, commentBody, &respBody); err != nil {
			slog.Error("github_pr_comment_create_failed", "run_id", runID, "error", err)
		}

		slog.Info("github_pr_comment_created", "run_id", runID, "comment_id", respBody.ID)

		if respBody.ID != 0 {
			run.GitHubPRCommentID = &respBody.ID
			if err := h.store.Runs.Update(ctx, run); err != nil {
				slog.Error("github_pr_comment_id_persist_failed", "run_id", runID, "error", err)
			}
		}
	}

	return nil
}

func mapGitHubCheckRunState(status models.RunStatus) (checkStatus string, conclusion string) {
	switch status {
	case models.RunStatusRunning:
		return "in_progress", ""
	case models.RunStatusSuccess:
		return "completed", "success"
	case models.RunStatusFailed:
		return "completed", "failure"
	case models.RunStatusCancelled:
		return "completed", "cancelled"
	default:
		return "queued", ""
	}
}

func buildGitHubCheckRunOutput(run *models.Run, status models.RunStatus) *notification.CheckRunOutput {
	title := fmt.Sprintf("Dagryn run %s", status)
	summary := fmt.Sprintf("Status: %s\nTasks: %d/%d\nFailed: %d\nCache hits: %d",
		status, run.CompletedTasks, run.TotalTasks, run.FailedTasks, run.CacheHits,
	)
	if run.DurationMs != nil {
		summary = fmt.Sprintf("%s\nDuration: %s", summary, formatDurationMs(*run.DurationMs))
	}
	return &notification.CheckRunOutput{
		Title:   title,
		Summary: summary,
	}
}

// buildGitHubPRComment constructs a human-friendly summary comment for a run.
func buildGitHubPRComment(run *models.Run, status models.RunStatus, targetURL string) string {
	var b strings.Builder
	icon := "🟡"
	switch status {
	case models.RunStatusSuccess:
		icon = "✅"
	case models.RunStatusFailed:
		icon = "❌"
	case models.RunStatusCancelled:
		icon = "⚪️"
	}

	fmt.Fprintf(&b, "%s **Dagryn workflow %s**\n\n", icon, strings.ToUpper(string(status)))

	if run.PRTitle != nil && *run.PRTitle != "" {
		fmt.Fprintf(&b, "- **PR**: %s\n", *run.PRTitle)
	}
	if run.CommitMessage != nil && *run.CommitMessage != "" {
		fmt.Fprintf(&b, "- **Commit**: %s\n", *run.CommitMessage)
	}
	if run.GitBranch != nil && *run.GitBranch != "" {
		fmt.Fprintf(&b, "- **Branch**: `%s`\n", *run.GitBranch)
	}
	if run.GitCommit != nil && *run.GitCommit != "" {
		sha := *run.GitCommit
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fmt.Fprintf(&b, "- **SHA**: `%s`\n", sha)
	}
	if run.DurationMs != nil {
		fmt.Fprintf(&b, "- **Duration**: %s\n", formatDurationMs(*run.DurationMs))
	}

	if targetURL != "" {
		fmt.Fprintf(&b, "\n[View run in Dagryn](%s)\n", targetURL)
	}

	return b.String()
}

// formatDurationMs formats a millisecond duration into a human-readable string.
func formatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	sec := ms / 1000
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	min := sec / 60
	sec = sec % 60
	return fmt.Sprintf("%dm %ds", min, sec)
}

// CancelRun godoc
//
//	@Summary		Cancel a workflow run
//	@Description	Cancels a running or pending workflow run
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	CancelRunResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse	"Run is already completed"
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/cancel [post]
func (h *Handler) CancelRun(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to cancel runs (same as trigger permission)
	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to cancel runs"))
		return
	}

	// Get the run
	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get run"))
		return
	}

	// Verify the run belongs to the project
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	// Check if run can be cancelled
	if run.Status.IsTerminal() {
		_ = response.Conflict(w, r, fmt.Errorf("run is already %s and cannot be cancelled", run.Status))
		return
	}

	// Update the run status to cancelled
	now := time.Now()
	run.Status = models.RunStatusCancelled
	run.FinishedAt = &now
	errorMsg := "Cancelled by user"
	run.ErrorMessage = &errorMsg

	if err := h.store.Runs.Update(ctx, run); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to cancel run"))
		return
	}

	if h.cancelManager != nil {
		if err := h.cancelManager.Signal(ctx, runID.String()); err != nil {
			slog.Warn("cancel_run: failed to signal worker", "run_id", runID, "error", err)
		}
	}

	// Publish cancellation event via SSE
	if h.sseHub != nil {
		h.sseHub.PublishRunEvent(sse.EventRunCancelled, run.ID, run.ProjectID, string(run.Status), errorMsg)
	}

	_ = response.Ok(w, r, "Success", CancelRunResponse{
		RunID:       run.ID,
		Status:      string(run.Status),
		Message:     "Run cancelled successfully",
		CancelledAt: now,
	})
}

// StreamRunEvents godoc
//
//	@Summary		Stream run events
//	@Description	Streams status events for a workflow run using Server-Sent Events
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		text/event-stream
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{string}	string	"SSE stream"
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/events [get]
func (h *Handler) StreamRunEvents(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to view runs
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	// Verify the run exists and belongs to the project
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

	// Subscribe to run events
	topics := []string{
		fmt.Sprintf("run:%s", runID),
	}
	sse.ServeSSE(w, r, h.sseHub, topics)
}

// GetRunDetail godoc
//
//	@Summary		Get detailed run information
//	@Description	Returns a workflow run with all task results
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	RunDetailResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/detail [get]
func (h *Handler) GetRunDetail(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to view runs
	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	// Get the run with tasks
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

	// Get task results
	tasks, err := h.store.Runs.ListTaskResults(ctx, runID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to get task results"))
		return
	}

	taskResponses := make([]TaskResultResponse, 0, len(tasks))
	for _, task := range tasks {
		taskResponses = append(taskResponses, taskResultModelToResponse(&task))
	}

	_ = response.Ok(w, r, "Success", RunDetailResponse{
		RunResponse:    h.runModelToResponseWithUser(ctx, run),
		Tasks:          taskResponses,
		CompletedTasks: run.CompletedTasks,
		FailedTasks:    run.FailedTasks,
		CacheHits:      run.CacheHits,
		ErrorMessage:   ptrToString(run.ErrorMessage),
	})
}

// getBaseURL extracts the base URL from a request
func getBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Check for forwarded proto header
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// ptrToString safely dereferences a string pointer
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func runModelToResponse(run *models.Run) RunResponse {
	resp := RunResponse{
		ID:            run.ID,
		ProjectID:     run.ProjectID,
		Status:        string(run.Status),
		TriggerSource: string(run.TriggeredBy),
		TaskCount:     run.TotalTasks,
		StartedAt:     run.StartedAt,
		FinishedAt:    run.FinishedAt,
		Duration:      run.DurationMs,
		CreatedAt:     run.CreatedAt,
	}

	if run.WorkflowName != nil {
		resp.WorkflowName = *run.WorkflowName
	}

	if run.Description != nil {
		resp.Description = *run.Description
	}

	if run.GitBranch != nil {
		resp.TriggerRef = *run.GitBranch
	}
	if run.GitCommit != nil {
		resp.CommitSHA = *run.GitCommit
	}
	if run.PRTitle != nil {
		resp.PRTitle = *run.PRTitle
	}
	if run.PRNumber != nil {
		resp.PRNumber = run.PRNumber
	}
	if run.CommitMessage != nil {
		resp.CommitMessage = *run.CommitMessage
	}
	if run.CommitAuthorName != nil {
		resp.CommitAuthorName = *run.CommitAuthorName
	}
	if run.CommitAuthorEmail != nil {
		resp.CommitAuthorEmail = *run.CommitAuthorEmail
	}
	if run.CommitAuthorAvatarURL != nil {
		resp.CommitAuthorAvatarURL = *run.CommitAuthorAvatarURL
	}
	if run.HostOS != nil {
		resp.HostOS = *run.HostOS
	}
	if run.HostArch != nil {
		resp.HostArch = *run.HostArch
	}
	if run.HostName != nil {
		resp.HostName = *run.HostName
	}

	return resp
}

// runModelToResponseWithUser converts a run model to response and includes user info if triggered_by_user_id is set.
func (h *Handler) runModelToResponseWithUser(ctx context.Context, run *models.Run) RunResponse {
	resp := runModelToResponse(run)

	// Fetch user info for local/API-triggered runs
	if run.TriggeredByUserID != nil {
		user, err := h.store.Users.GetByID(ctx, *run.TriggeredByUserID)
		if err == nil && user != nil {
			resp.TriggeredByUser = &UserResponse{
				ID:        user.ID,
				Email:     user.Email,
				Name:      *user.Name,
				AvatarURL: *user.AvatarURL,
			}
		}
	}

	return resp
}

func taskResultModelToResponse(task *models.TaskResult) TaskResultResponse {
	resp := TaskResultResponse{
		ID:         task.ID,
		RunID:      task.RunID,
		TaskName:   task.TaskName,
		Status:     string(task.Status),
		ExitCode:   task.ExitCode,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
		Duration:   task.DurationMs,
		CacheHit:   task.CacheHit,
	}
	if task.CacheKey != nil {
		resp.CacheKey = *task.CacheKey
	}
	return resp
}

func apiKeyWithProjectToResponse(key *models.APIKeyWithProject) APIKeyResponse {
	return APIKeyResponse{
		ID:         key.ID,
		Name:       key.Name,
		Prefix:     key.KeyPrefix,
		Scope:      string(key.Scope),
		ProjectID:  key.ProjectID,
		LastUsedAt: key.LastUsedAt,
		ExpiresAt:  key.ExpiresAt,
		CreatedAt:  key.CreatedAt,
	}
}

// Note: invitationWithDetailsToResponse is defined in users.go
// Note: apiKeyModelToResponse is defined in projects.go
// Note: parseDuration is defined in projects.go
// Note: userModelToResponse is defined in users.go

// parseInt converts a string to int (duplicated here for self-containment)
// func parseIntParam(s string) (int, error) {
// 	return strconv.Atoi(s)
// }

// UpdateRunStatus godoc
//
//	@Summary		Update run status
//	@Description	Updates the status of a workflow run (used by CLI for remote sync)
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			runID		path		string					true	"Run ID"		format(uuid)
//	@Param			body		body		UpdateRunStatusRequest	true	"Update run status request"
//	@Success		200			{object}	RunResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse	"Run is already completed"
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/status [patch]
func (h *Handler) UpdateRunStatus(w http.ResponseWriter, r *http.Request) {
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

	// Check permission to trigger/update runs
	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to update runs"))
		return
	}

	var req UpdateRunStatusRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate status
	validStatuses := map[string]models.RunStatus{
		"running":   models.RunStatusRunning,
		"success":   models.RunStatusSuccess,
		"failed":    models.RunStatusFailed,
		"cancelled": models.RunStatusCancelled,
	}
	newStatus, ok := validStatuses[req.Status]
	if !ok {
		_ = response.BadRequest(w, r, errors.New("invalid status: must be one of: running, success, failed, cancelled"))
		return
	}

	// Get the run
	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("run not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get run"))
		return
	}

	// Verify the run belongs to the project
	if run.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("run not found in this project"))
		return
	}

	// Check if run can be updated
	if run.Status.IsTerminal() {
		_ = response.Conflict(w, r, fmt.Errorf("run is already %s and cannot be updated", run.Status))
		return
	}

	// Use targeted updates so we never clobber atomically-incremented counters
	// (completed_tasks, failed_tasks, cache_hits).
	oldStatus := run.Status
	becameTerminal := !oldStatus.IsTerminal() && newStatus.IsTerminal()

	if newStatus == models.RunStatusRunning && run.Status == models.RunStatusPending {
		// Pending → Running: use Start or StartWithTotal.
		if req.TotalTasks != nil {
			if err := h.store.Runs.StartWithTotal(ctx, run.ID, *req.TotalTasks); err != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to update run"))
				return
			}
		} else {
			if err := h.store.Runs.Start(ctx, run.ID); err != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to update run"))
				return
			}
		}
	} else if newStatus.IsTerminal() {
		// Terminal: use Complete (only sets status, finished_at, duration_ms, error_message).
		if err := h.store.Runs.Complete(ctx, run.ID, newStatus, req.ErrorMessage); err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to update run"))
			return
		}
	} else {
		// Any other transition (e.g. Running → Running with total_tasks update).
		run.Status = newStatus
		if err := h.store.Runs.Update(ctx, run); err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to update run"))
			return
		}
	}

	// Update total_tasks if provided and not already handled by StartWithTotal.
	if req.TotalTasks != nil && (newStatus != models.RunStatusRunning || oldStatus != models.RunStatusPending) {
		if err := h.store.Runs.SetTotalTasks(ctx, run.ID, *req.TotalTasks); err != nil {
			slog.Warn("UpdateRunStatus: failed to set total_tasks", "run_id", run.ID, "error", err)
		}
	}

	// If this run was triggered from a GitHub PR, update commit status and post a summary comment.
	if becameTerminal && run.PRNumber != nil {
		go func(runID, projectID uuid.UUID, status models.RunStatus) {
			// Use background context; best‑effort only.
			bgCtx := context.Background()
			if err := h.notifyGitHubForRun(bgCtx, projectID, runID, status); err != nil {
				slog.Error("github_notify_run_failed", "run_id", runID, "project_id", projectID, "error", err)
			}
		}(run.ID, run.ProjectID, newStatus)
	}

	// Publish SSE event
	if h.sseHub != nil {
		var eventType sse.EventType
		switch newStatus {
		case models.RunStatusRunning:
			eventType = sse.EventRunStarted
		case models.RunStatusSuccess:
			eventType = sse.EventRunCompleted
		case models.RunStatusFailed:
			eventType = sse.EventRunFailed
		case models.RunStatusCancelled:
			eventType = sse.EventRunCancelled
		}
		errorMsg := ""
		if req.ErrorMessage != nil {
			errorMsg = *req.ErrorMessage
		}
		h.sseHub.PublishRunEvent(eventType, run.ID, run.ProjectID, string(run.Status), errorMsg)
	}

	_ = response.Ok(w, r, "Success", h.runModelToResponseWithUser(ctx, run))
}

// CreateTask godoc
//
//	@Summary		Create a task result
//	@Description	Creates a new task result record for a run
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string				true	"Project ID"	format(uuid)
//	@Param			runID		path		string				true	"Run ID"		format(uuid)
//	@Param			body		body		CreateTaskRequest	true	"Create task request"
//	@Success		201			{object}	TaskResultResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/tasks [post]
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
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

	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to update runs"))
		return
	}

	var req CreateTaskRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.TaskName == "" {
		_ = response.BadRequest(w, r, errors.New("task name is required"))
		return
	}

	// Verify the run exists and belongs to the project
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

	// Create task result
	now := time.Now()
	task := &models.TaskResult{
		RunID:     runID,
		TaskName:  req.TaskName,
		Status:    models.TaskStatusPending,
		StartedAt: &now,
		CreatedAt: now,
	}

	if err := h.store.Runs.CreateTaskResult(ctx, task); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create task"))
		return
	}

	_ = response.Created(w, r, "Created successfully", taskResultModelToResponse(task))
}

// UpdateTaskStatus godoc
//
//	@Summary		Update task status
//	@Description	Updates the status of a task result
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			runId		path		string					true	"Run ID"		format(uuid)
//	@Param			taskName	path		string					true	"Task name"
//	@Param			body		body		UpdateTaskStatusRequest	true	"Update task status request"
//	@Success		200			{object}	TaskResultResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runId}/tasks/{taskName} [patch]
func (h *Handler) UpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
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

	taskName, err := getTaskNameFromPath(r)
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

	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to update runs"))
		return
	}

	var req UpdateTaskStatusRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate status
	validStatuses := map[string]models.TaskStatus{
		"pending":   models.TaskStatusPending,
		"running":   models.TaskStatusRunning,
		"success":   models.TaskStatusSuccess,
		"failed":    models.TaskStatusFailed,
		"cached":    models.TaskStatusCached,
		"skipped":   models.TaskStatusSkipped,
		"cancelled": models.TaskStatusCancelled,
	}
	newStatus, ok := validStatuses[req.Status]
	if !ok {
		_ = response.BadRequest(w, r, errors.New("invalid status"))
		return
	}

	// Verify the run exists and belongs to the project
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

	// Get the task result
	task, err := h.store.Runs.GetTaskResult(ctx, runID, taskName)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("task not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get task"))
		return
	}

	// Update task
	now := time.Now()
	task.Status = newStatus

	// Use client-provided timestamps if available, fall back to server time
	if req.StartedAt != nil {
		task.StartedAt = req.StartedAt
	} else if task.StartedAt == nil && (newStatus == models.TaskStatusRunning || newStatus.IsTerminal()) {
		task.StartedAt = &now
	}

	if newStatus.IsTerminal() {
		if req.FinishedAt != nil {
			task.FinishedAt = req.FinishedAt
		} else {
			task.FinishedAt = &now
		}
		if task.StartedAt != nil {
			duration := task.FinishedAt.Sub(*task.StartedAt).Milliseconds()
			task.DurationMs = &duration
		}
	}

	if req.ExitCode != nil {
		task.ExitCode = req.ExitCode
	}
	if req.DurationMs != nil {
		task.DurationMs = req.DurationMs
	}
	if req.CacheKey != "" {
		task.CacheKey = &req.CacheKey
	}
	task.CacheHit = req.CacheHit
	if req.Output != "" {
		task.Output = &req.Output
	}
	if req.Error != "" {
		task.ErrorMessage = &req.Error
	}

	if err := h.store.Runs.UpdateTaskResult(ctx, task); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update task"))
		return
	}

	// Update run counters if task completed
	if newStatus.IsTerminal() {
		switch newStatus {
		case models.TaskStatusSuccess, models.TaskStatusCached:
			_ = h.store.Runs.IncrementCompleted(ctx, runID, newStatus == models.TaskStatusCached || req.CacheHit)
		case models.TaskStatusFailed:
			_ = h.store.Runs.IncrementFailed(ctx, runID)
		}
	}

	// Publish SSE event
	if h.sseHub != nil {
		var eventType sse.EventType
		switch newStatus {
		case models.TaskStatusRunning:
			eventType = sse.EventTaskStarted
		case models.TaskStatusSuccess:
			eventType = sse.EventTaskCompleted
		case models.TaskStatusFailed:
			eventType = sse.EventTaskFailed
		case models.TaskStatusCached:
			eventType = sse.EventTaskCached
		case models.TaskStatusSkipped:
			eventType = sse.EventTaskSkipped
		default:
			eventType = sse.EventTaskCompleted
		}
		h.sseHub.PublishTaskEvent(eventType, runID, taskName, string(newStatus), req.ExitCode, task.DurationMs, req.CacheHit, req.CacheKey)
	}

	_ = response.Ok(w, r, "Success", taskResultModelToResponse(task))
}

// AppendLog godoc
//
//	@Summary		Append log lines
//	@Description	Appends log lines to a run (single or batch)
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string			true	"Project ID"	format(uuid)
//	@Param			runID		path		string			true	"Run ID"		format(uuid)
//	@Param			body		body		BatchLogRequest	true	"Log lines to append"
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/logs [post]
func (h *Handler) AppendLog(w http.ResponseWriter, r *http.Request) {
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

	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to update runs"))
		return
	}

	var req BatchLogRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Verify the run exists and belongs to the project
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

	// Persist logs to database
	if len(req.Logs) > 0 {
		dbLogs := make([]models.RunLog, len(req.Logs))
		for i, log := range req.Logs {
			dbLogs[i] = models.RunLog{
				RunID:    runID,
				TaskName: log.TaskName,
				Stream:   models.LogStream(log.Stream),
				LineNum:  log.LineNum,
				Content:  log.Line,
			}
		}

		// Persist logs but don't fail the request if it errors
		// SSE streaming is the primary concern for real-time updates
		_ = h.store.Runs.AppendLogs(ctx, dbLogs)
	}

	// Publish log events via SSE
	if h.sseHub != nil {
		for _, log := range req.Logs {
			h.sseHub.PublishLogEvent(runID, log.TaskName, log.Stream, log.Line, log.LineNum)
		}
	}

	_ = response.Ok(w, r, "Success", SuccessResponse{
		Message: fmt.Sprintf("Appended %d log lines", len(req.Logs)),
	})
}

// GetLogs godoc
//
//	@Summary		Get logs for a run
//	@Description	Returns paginated logs for a workflow run
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Param			task		query		string	false	"Filter by task name"
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			per_page	query		int		false	"Items per page"	default(100)	maximum(1000)
//	@Param			after_id	query		int		false	"Return logs after this ID (for polling)"
//	@Success		200			{object}	PaginatedResponse{data=[]LogResponse}
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/logs/history [get]
func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
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

	if !role.HasPermission(models.PermissionRunView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view runs"))
		return
	}

	// Verify the run exists and belongs to the project
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

	// Check for after_id parameter (for polling/streaming)
	afterIDStr := r.URL.Query().Get("after_id")
	if afterIDStr != "" {
		afterID, err := strconv.ParseInt(afterIDStr, 10, 64)
		if err != nil {
			_ = response.BadRequest(w, r, errors.New("invalid after_id parameter"))
			return
		}

		// Get logs since the given ID
		limit := 1000
		if l := r.URL.Query().Get("per_page"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}

		logs, err := h.store.Runs.GetLogsSince(ctx, runID, afterID, limit)
		if err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to get logs"))
			return
		}

		resp := make([]LogResponse, len(logs))
		for i, log := range logs {
			resp[i] = logModelToResponse(&log)
		}

		_ = response.Ok(w, r, "Success", resp)
		return
	}

	// Standard paginated query
	page, perPage := GetPageParams(r)
	// Allow larger per_page for logs
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if n, err := strconv.Atoi(pp); err == nil && n > 0 && n <= 3000 {
			perPage = n
		}
	}
	offset := (page - 1) * perPage

	taskFilter := r.URL.Query().Get("task")

	var logs []models.RunLog
	var total int

	if taskFilter != "" {
		logs, total, err = h.store.Runs.GetLogsByTask(ctx, runID, taskFilter, perPage, offset)
	} else {
		logs, total, err = h.store.Runs.GetLogs(ctx, runID, perPage, offset)
	}

	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to get logs"))
		return
	}

	resp := make([]LogResponse, len(logs))
	for i, log := range logs {
		resp[i] = logModelToResponse(&log)
	}

	_ = response.Ok(w, r, "Success", PaginatedResponse{
		Data: resp,
		Meta: PaginationMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      int64(total),
			TotalPages: CalculateTotalPages(int64(total), perPage),
		},
	})
}

// logModelToResponse converts a RunLog model to LogResponse.
func logModelToResponse(log *models.RunLog) LogResponse {
	return LogResponse{
		ID:        log.ID,
		TaskName:  log.TaskName,
		Stream:    string(log.Stream),
		LineNum:   log.LineNum,
		Content:   log.Content,
		CreatedAt: log.CreatedAt,
	}
}

// Heartbeat godoc
//
//	@Summary		Send heartbeat for a run
//	@Description	Updates the last heartbeat timestamp for a run (CLI calls this periodically)
//	@Tags			runs
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Param			runID		path		string	true	"Run ID"		format(uuid)
//	@Success		200			{object}	HeartbeatResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/runs/{runID}/heartbeat [post]
func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
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

	if !role.HasPermission(models.PermissionRunTrigger) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to update runs"))
		return
	}

	// Verify the run exists and belongs to the project
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

	// Update the heartbeat timestamp
	if err := h.store.Runs.UpdateHeartbeat(ctx, runID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update heartbeat"))
		return
	}

	_ = response.Ok(w, r, "Success", HeartbeatResponse{
		RunID:           runID,
		Status:          string(run.Status),
		LastHeartbeatAt: time.Now(),
	})
}
