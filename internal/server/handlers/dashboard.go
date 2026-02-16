package handlers

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
	serverctx "github.com/mujhtech/dagryn/internal/server/context"
	"github.com/mujhtech/dagryn/internal/server/response"
)

// GetDashboardOverview returns a consolidated dashboard overview for the current user.
// @Summary Get dashboard overview
// @Description Returns projects with inline stats and recent runs across all user projects
// @Tags dashboard
// @Security BearerAuth
// @Produce json
// @Success 200 {object} DashboardOverviewResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dashboard/overview [get]
func (h *Handler) GetDashboardOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	// 1. Get user's projects (already scoped to user membership)
	projects, err := h.projects.ListByUser(ctx, user.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list projects"))
		return
	}

	if len(projects) == 0 {
		_ = response.Ok(w, r, "Success", DashboardOverviewResponse{
			Projects:   []DashboardProjectResponse{},
			RecentRuns: []DashboardRunResponse{},
		})
		return
	}

	// 2. Extract project IDs
	projectIDs := make([]uuid.UUID, len(projects))
	projectMap := make(map[uuid.UUID]*models.ProjectWithMember, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
		proj := projects[i]
		projectMap[p.ID] = &proj
	}

	// 3. Fetch recent runs across all projects
	recentRuns, err := h.runs.GetRecentRunsAcrossProjects(ctx, projectIDs, 7)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to load recent runs"))
		return
	}

	// 4. Fetch per-project stats (7-day)
	projectStats, err := h.runs.GetProjectStats(ctx, projectIDs, 7)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to load project stats"))
		return
	}

	// 5. Batch-fetch triggered-by users to avoid N+1
	userIDs := make(map[uuid.UUID]struct{})
	for i := range recentRuns {
		if recentRuns[i].TriggeredByUserID != nil {
			userIDs[*recentRuns[i].TriggeredByUserID] = struct{}{}
		}
	}
	// Also collect user IDs from latest runs in project stats
	for _, stats := range projectStats {
		if stats.LatestRun != nil && stats.LatestRun.TriggeredByUserID != nil {
			userIDs[*stats.LatestRun.TriggeredByUserID] = struct{}{}
		}
	}
	userCache := make(map[uuid.UUID]*UserResponse, len(userIDs))
	for uid := range userIDs {
		u, err := h.users.GetByID(ctx, uid)
		if err == nil && u != nil {
			resp := userModelToResponse(u)
			userCache[uid] = &resp
		}
	}

	// 6. Build response
	resp := DashboardOverviewResponse{
		Projects:   make([]DashboardProjectResponse, 0, len(projects)),
		RecentRuns: make([]DashboardRunResponse, 0, len(recentRuns)),
	}

	// Build project responses with inline stats
	for _, p := range projects {
		dp := DashboardProjectResponse{
			ID:         p.ID,
			Name:       p.Name,
			Slug:       p.Slug,
			Visibility: string(p.Visibility),
			UpdatedAt:  p.UpdatedAt,
			CreatedAt:  p.CreatedAt,
			Chart:      []RunDashboardChartPointResponse{},
		}
		if p.RepoURL != nil {
			dp.RepoURL = *p.RepoURL
		}

		if stats, ok := projectStats[p.ID]; ok {
			dp.TotalRuns7d = stats.TotalRuns7d
			dp.SuccessRuns7d = stats.SuccessRuns7d
			dp.FailedRuns7d = stats.FailedRuns7d
			dp.AvgDurationMs = stats.AvgDurationMs
			dp.TopBranch = stats.TopBranch

			for _, point := range stats.Chart {
				dp.Chart = append(dp.Chart, RunDashboardChartPointResponse{
					Date:       point.Date.Format("2006-01-02"),
					Success:    point.Success,
					Failed:     point.Failed,
					DurationMs: point.DurationMs,
				})
			}

			if stats.LatestRun != nil {
				lr := runToDashboardRun(stats.LatestRun, "", userCache)
				if proj, ok := projectMap[stats.LatestRun.ProjectID]; ok {
					lr.ProjectName = proj.Name
				}
				dp.LatestRun = &lr
			}
		}

		resp.Projects = append(resp.Projects, dp)
	}

	// Build recent runs responses
	for i := range recentRuns {
		run := &recentRuns[i]
		projectName := ""
		if proj, ok := projectMap[run.ProjectID]; ok {
			projectName = proj.Name
		}
		resp.RecentRuns = append(resp.RecentRuns, runToDashboardRun(run, projectName, userCache))
	}

	_ = response.Ok(w, r, "Success", resp)
}

// runToDashboardRun converts a Run model to a DashboardRunResponse.
func runToDashboardRun(run *models.Run, projectName string, userCache map[uuid.UUID]*UserResponse) DashboardRunResponse {
	dr := DashboardRunResponse{
		ID:          run.ID,
		ProjectID:   run.ProjectID,
		ProjectName: projectName,
		Status:      string(run.Status),
		DurationMs:  run.DurationMs,
		CreatedAt:   run.CreatedAt,
	}

	if run.WorkflowName != nil {
		dr.WorkflowName = *run.WorkflowName
	} else if len(run.Targets) > 0 {
		dr.WorkflowName = run.Targets[0]
	}

	if run.GitBranch != nil {
		dr.TriggerRef = *run.GitBranch
	}
	if run.GitCommit != nil {
		dr.CommitSHA = *run.GitCommit
	}
	if run.CommitAuthorName != nil {
		dr.CommitAuthorName = *run.CommitAuthorName
	}
	if run.TriggeredByUserID != nil {
		if u, ok := userCache[*run.TriggeredByUserID]; ok {
			dr.TriggeredByUser = u
		}
	}

	return dr
}
