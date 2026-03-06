package handlers

import (
	"errors"
	"net/http"
	"strconv"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"

	"github.com/google/uuid"
)

// GetTeamAnalytics returns aggregated analytics for all projects in a team.
// GET /api/v1/teams/{teamId}/analytics?days=30
func (h *Handler) GetTeamAnalytics(w http.ResponseWriter, r *http.Request) {
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

	// Verify team membership
	_, err = h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		_ = response.Forbidden(w, r, errors.New("not a member of this team"))
		return
	}

	// Get team projects
	projects, err := h.store.Projects.ListByTeam(ctx, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	projectIDs := make([]uuid.UUID, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
	}

	days := parseDaysParam(r)

	analytics, err := h.store.Analytics.GetTeamAnalytics(ctx, projectIDs, &teamID, days)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "Team analytics retrieved", toTeamAnalyticsResponse(analytics))
}

// GetUserAnalytics returns aggregated analytics for all projects the user has access to.
// GET /api/v1/analytics?days=30
func (h *Handler) GetUserAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	// Get all user's projects
	projects, err := h.store.Projects.ListByUser(ctx, user.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	projectIDs := make([]uuid.UUID, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
	}

	days := parseDaysParam(r)

	analytics, err := h.store.Analytics.GetTeamAnalytics(ctx, projectIDs, nil, days)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "User analytics retrieved", toTeamAnalyticsResponse(analytics))
}

func parseDaysParam(r *http.Request) int {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	return days
}

func toTeamAnalyticsResponse(a *repo.TeamAnalytics) TeamAnalyticsResponse {
	resp := TeamAnalyticsResponse{}

	// Runs
	resp.Runs = RunAnalyticsResponse{
		TotalRuns:     a.Runs.TotalRuns,
		SuccessRuns:   a.Runs.SuccessRuns,
		FailedRuns:    a.Runs.FailedRuns,
		CancelledRuns: a.Runs.CancelledRuns,
		SuccessRate:   a.Runs.SuccessRate,
		AvgDurationMs: a.Runs.AvgDurationMs,
	}
	for _, p := range a.Runs.Chart {
		resp.Runs.Chart = append(resp.Runs.Chart, DailyRunPointResponse{
			Date:          p.Date.Format("2006-01-02"),
			Success:       p.Success,
			Failed:        p.Failed,
			Cancelled:     p.Cancelled,
			AvgDurationMs: p.AvgDurationMs,
		})
	}

	// Cache
	resp.Cache = CacheAnalyticsResponse{
		TotalEntries:         a.Cache.TotalEntries,
		TotalSizeBytes:       a.Cache.TotalSizeBytes,
		TotalHits:            a.Cache.TotalHits,
		TotalMisses:          a.Cache.TotalMisses,
		HitRate:              a.Cache.HitRate,
		TotalBytesUploaded:   a.Cache.TotalBytesUploaded,
		TotalBytesDownloaded: a.Cache.TotalBytesDownloaded,
	}
	for _, p := range a.Cache.Chart {
		resp.Cache.Chart = append(resp.Cache.Chart, DailyCachePointResponse{
			Date:            p.Date.Format("2006-01-02"),
			CacheHits:       p.CacheHits,
			CacheMisses:     p.CacheMisses,
			BytesUploaded:   p.BytesUploaded,
			BytesDownloaded: p.BytesDownloaded,
		})
	}

	// Artifacts
	resp.Artifacts = ArtifactAnalyticsResponse{
		TotalArtifacts: a.Artifacts.TotalArtifacts,
		TotalSizeBytes: a.Artifacts.TotalSizeBytes,
	}
	for _, p := range a.Artifacts.Chart {
		resp.Artifacts.Chart = append(resp.Artifacts.Chart, DailyArtifactPointResponse{
			Date:      p.Date.Format("2006-01-02"),
			Count:     p.Count,
			SizeBytes: p.SizeBytes,
		})
	}

	// Bandwidth
	resp.Bandwidth = BandwidthAnalyticsResponse{
		TotalBytes:    a.Bandwidth.TotalBytes,
		UploadBytes:   a.Bandwidth.UploadBytes,
		DownloadBytes: a.Bandwidth.DownloadBytes,
	}
	for _, p := range a.Bandwidth.Chart {
		resp.Bandwidth.Chart = append(resp.Bandwidth.Chart, DailyBandwidthPointResponse{
			Date:          p.Date.Format("2006-01-02"),
			UploadBytes:   p.UploadBytes,
			DownloadBytes: p.DownloadBytes,
		})
	}

	// AI
	resp.AI = AIAnalyticsResponse{
		TotalAnalyses:      a.AI.TotalAnalyses,
		SuccessAnalyses:    a.AI.SuccessAnalyses,
		FailedAnalyses:     a.AI.FailedAnalyses,
		TotalSuggestions:   a.AI.TotalSuggestions,
		AppliedSuggestions: a.AI.AppliedSuggestions,
	}
	for _, p := range a.AI.Chart {
		resp.AI.Chart = append(resp.AI.Chart, DailyAIPointResponse{
			Date:        p.Date.Format("2006-01-02"),
			Analyses:    p.Analyses,
			Suggestions: p.Suggestions,
		})
	}

	// Audit Log
	resp.AuditLog = AuditLogAnalyticsResponse{
		TotalEvents: a.AuditLog.TotalEvents,
	}
	for _, ac := range a.AuditLog.TopActions {
		resp.AuditLog.TopActions = append(resp.AuditLog.TopActions, AuditActionCountResponse{
			Action: ac.Action,
			Count:  ac.Count,
		})
	}
	for _, ac := range a.AuditLog.TopActors {
		resp.AuditLog.TopActors = append(resp.AuditLog.TopActors, AuditActorCountResponse{
			ActorEmail: ac.ActorEmail,
			Count:      ac.Count,
		})
	}
	for _, p := range a.AuditLog.Chart {
		resp.AuditLog.Chart = append(resp.AuditLog.Chart, DailyAuditPointResponse{
			Date:   p.Date.Format("2006-01-02"),
			Events: p.EventCount,
		})
	}

	// Projects
	for _, p := range a.Projects {
		resp.Projects = append(resp.Projects, ProjectActivityResponse{
			ProjectID:         p.ProjectID.String(),
			ProjectName:       p.ProjectName,
			TotalRuns:         p.TotalRuns,
			SuccessRate:       p.SuccessRate,
			CacheSizeBytes:    p.CacheSize,
			ArtifactSizeBytes: p.ArtifactSize,
			BandwidthBytes:    p.Bandwidth,
		})
	}

	return resp
}
