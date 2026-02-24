package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalyticsRepo handles cross-project analytics aggregation queries.
type AnalyticsRepo struct {
	pool *pgxpool.Pool
}

// NewAnalyticsRepo creates a new analytics repository.
func NewAnalyticsRepo(pool *pgxpool.Pool) AnalyticsStore {
	return &AnalyticsRepo{pool: pool}
}

// TeamAnalytics contains aggregated analytics across multiple projects.
type TeamAnalytics struct {
	Runs      RunAnalyticsSummary
	Cache     CacheAnalyticsSummary
	Artifacts ArtifactAnalyticsSummary
	Bandwidth BandwidthAnalyticsSummary
	AI        AIAnalyticsSummary
	AuditLog  AuditLogAnalyticsSummary
	Projects  []ProjectActivitySummary
}

// RunAnalyticsSummary aggregates run metrics.
type RunAnalyticsSummary struct {
	TotalRuns     int
	SuccessRuns   int
	FailedRuns    int
	CancelledRuns int
	SuccessRate   float64
	AvgDurationMs int64
	Chart         []DailyRunPoint
}

// DailyRunPoint is a daily run aggregate for charts.
type DailyRunPoint struct {
	Date          time.Time
	Success       int
	Failed        int
	Cancelled     int
	AvgDurationMs int64
}

// CacheAnalyticsSummary aggregates cache metrics.
type CacheAnalyticsSummary struct {
	TotalEntries         int
	TotalSizeBytes       int64
	TotalHits            int64
	TotalMisses          int64
	HitRate              float64
	TotalBytesUploaded   int64
	TotalBytesDownloaded int64
	Chart                []DailyCachePoint
}

// DailyCachePoint is a daily cache aggregate for charts.
type DailyCachePoint struct {
	Date            time.Time
	CacheHits       int
	CacheMisses     int
	BytesUploaded   int64
	BytesDownloaded int64
}

// ArtifactAnalyticsSummary aggregates artifact metrics.
type ArtifactAnalyticsSummary struct {
	TotalArtifacts int
	TotalSizeBytes int64
	Chart          []DailyArtifactPoint
}

// DailyArtifactPoint is a daily artifact aggregate for charts.
type DailyArtifactPoint struct {
	Date      time.Time
	Count     int
	SizeBytes int64
}

// BandwidthAnalyticsSummary aggregates bandwidth metrics.
type BandwidthAnalyticsSummary struct {
	TotalBytes    int64
	UploadBytes   int64
	DownloadBytes int64
	Chart         []DailyBandwidthPoint
}

// DailyBandwidthPoint is a daily bandwidth aggregate for charts.
type DailyBandwidthPoint struct {
	Date          time.Time
	UploadBytes   int64
	DownloadBytes int64
}

// AIAnalyticsSummary aggregates AI analysis metrics.
type AIAnalyticsSummary struct {
	TotalAnalyses      int
	SuccessAnalyses    int
	FailedAnalyses     int
	TotalSuggestions   int
	AppliedSuggestions int
	Chart              []DailyAIPoint
}

// DailyAIPoint is a daily AI analysis aggregate for charts.
type DailyAIPoint struct {
	Date        time.Time
	Analyses    int
	Suggestions int
}

// AuditLogAnalyticsSummary aggregates audit log metrics.
type AuditLogAnalyticsSummary struct {
	TotalEvents int
	TopActions  []AuditActionCount
	TopActors   []AuditActorCount
	Chart       []DailyAuditPoint
}

// AuditActionCount holds an action and its count.
type AuditActionCount struct {
	Action string
	Count  int
}

// AuditActorCount holds an actor email and their event count.
type AuditActorCount struct {
	ActorEmail string
	Count      int
}

// DailyAuditPoint is a daily audit log aggregate for charts.
type DailyAuditPoint struct {
	Date       time.Time
	EventCount int
}

// ProjectActivitySummary summarizes a project's activity for the leaderboard.
type ProjectActivitySummary struct {
	ProjectID    uuid.UUID
	ProjectName  string
	TotalRuns    int
	SuccessRate  float64
	CacheSize    int64
	ArtifactSize int64
	Bandwidth    int64
}

// GetTeamAnalytics returns aggregated analytics across the given projects.
// When teamID is non-nil, audit log queries scope by team; otherwise by project IDs.
func (r *AnalyticsRepo) GetTeamAnalytics(ctx context.Context, projectIDs []uuid.UUID, teamID *uuid.UUID, days int) (*TeamAnalytics, error) {
	if days <= 0 {
		days = 30
	}
	if len(projectIDs) == 0 {
		return &TeamAnalytics{}, nil
	}

	result := &TeamAnalytics{}

	// --- Runs ---
	runs, err := r.getRunAnalytics(ctx, projectIDs, days)
	if err != nil {
		return nil, err
	}
	result.Runs = *runs

	// --- Cache ---
	cache, err := r.getCacheAnalytics(ctx, projectIDs, days)
	if err != nil {
		return nil, err
	}
	result.Cache = *cache

	// --- Artifacts ---
	artifacts, err := r.getArtifactAnalytics(ctx, projectIDs, days)
	if err != nil {
		return nil, err
	}
	result.Artifacts = *artifacts

	// --- Bandwidth ---
	result.Bandwidth = r.computeBandwidth(cache, artifacts)

	// --- AI ---
	ai, err := r.getAIAnalytics(ctx, projectIDs, days)
	if err != nil {
		return nil, err
	}
	result.AI = *ai

	// --- Audit Log ---
	audit, err := r.getAuditLogAnalytics(ctx, projectIDs, teamID, days)
	if err != nil {
		return nil, err
	}
	result.AuditLog = *audit

	// --- Project Leaderboard ---
	projects, err := r.getProjectLeaderboard(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	result.Projects = projects

	return result, nil
}

func (r *AnalyticsRepo) getRunAnalytics(ctx context.Context, projectIDs []uuid.UUID, days int) (*RunAnalyticsSummary, error) {
	summary := &RunAnalyticsSummary{}

	// Totals
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'success'),
			COUNT(*) FILTER (WHERE status = 'failed'),
			COUNT(*) FILTER (WHERE status = 'cancelled'),
			COALESCE(ROUND(AVG(duration_ms)), 0)::bigint
		FROM runs
		WHERE project_id = ANY($1)
		  AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
	`, projectIDs, days).Scan(
		&summary.TotalRuns,
		&summary.SuccessRuns,
		&summary.FailedRuns,
		&summary.CancelledRuns,
		&summary.AvgDurationMs,
	)
	if err != nil {
		return nil, err
	}

	if summary.TotalRuns > 0 {
		summary.SuccessRate = float64(summary.SuccessRuns) / float64(summary.TotalRuns) * 100
	}

	// Chart
	rows, err := r.pool.Query(ctx, `
		SELECT DATE(created_at) AS day,
			   COUNT(*) FILTER (WHERE status = 'success') AS success,
			   COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			   COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled,
			   COALESCE(ROUND(AVG(duration_ms)), 0)::bigint AS avg_duration_ms
		FROM runs
		WHERE project_id = ANY($1)
		  AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
		GROUP BY day ORDER BY day ASC
	`, projectIDs, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p DailyRunPoint
		if err := rows.Scan(&p.Date, &p.Success, &p.Failed, &p.Cancelled, &p.AvgDurationMs); err != nil {
			return nil, err
		}
		summary.Chart = append(summary.Chart, p)
	}

	return summary, rows.Err()
}

func (r *AnalyticsRepo) getCacheAnalytics(ctx context.Context, projectIDs []uuid.UUID, days int) (*CacheAnalyticsSummary, error) {
	summary := &CacheAnalyticsSummary{}

	// Totals from cache_quotas
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(current_entries), 0),
			   COALESCE(SUM(current_size_bytes), 0)
		FROM cache_quotas
		WHERE project_id = ANY($1)
	`, projectIDs).Scan(&summary.TotalEntries, &summary.TotalSizeBytes)
	if err != nil {
		return nil, err
	}

	// Totals from cache_usage
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cache_hits), 0),
			   COALESCE(SUM(cache_misses), 0),
			   COALESCE(SUM(bytes_uploaded), 0),
			   COALESCE(SUM(bytes_downloaded), 0)
		FROM cache_usage
		WHERE project_id = ANY($1)
		  AND date >= CURRENT_DATE - $2::int
	`, projectIDs, days).Scan(
		&summary.TotalHits,
		&summary.TotalMisses,
		&summary.TotalBytesUploaded,
		&summary.TotalBytesDownloaded,
	)
	if err != nil {
		return nil, err
	}

	total := summary.TotalHits + summary.TotalMisses
	if total > 0 {
		summary.HitRate = float64(summary.TotalHits) / float64(total) * 100
	}

	// Chart
	rows, err := r.pool.Query(ctx, `
		SELECT date,
			   COALESCE(SUM(cache_hits), 0)::int,
			   COALESCE(SUM(cache_misses), 0)::int,
			   COALESCE(SUM(bytes_uploaded), 0),
			   COALESCE(SUM(bytes_downloaded), 0)
		FROM cache_usage
		WHERE project_id = ANY($1)
		  AND date >= CURRENT_DATE - $2::int
		GROUP BY date ORDER BY date ASC
	`, projectIDs, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p DailyCachePoint
		if err := rows.Scan(&p.Date, &p.CacheHits, &p.CacheMisses, &p.BytesUploaded, &p.BytesDownloaded); err != nil {
			return nil, err
		}
		summary.Chart = append(summary.Chart, p)
	}

	return summary, rows.Err()
}

func (r *AnalyticsRepo) getArtifactAnalytics(ctx context.Context, projectIDs []uuid.UUID, days int) (*ArtifactAnalyticsSummary, error) {
	summary := &ArtifactAnalyticsSummary{}

	// Totals
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM artifacts
		WHERE project_id = ANY($1)
		  AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
	`, projectIDs, days).Scan(&summary.TotalArtifacts, &summary.TotalSizeBytes)
	if err != nil {
		return nil, err
	}

	// Chart
	rows, err := r.pool.Query(ctx, `
		SELECT DATE(created_at) AS day,
			   COUNT(*),
			   COALESCE(SUM(size_bytes), 0)
		FROM artifacts
		WHERE project_id = ANY($1)
		  AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
		GROUP BY day ORDER BY day ASC
	`, projectIDs, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p DailyArtifactPoint
		if err := rows.Scan(&p.Date, &p.Count, &p.SizeBytes); err != nil {
			return nil, err
		}
		summary.Chart = append(summary.Chart, p)
	}

	return summary, rows.Err()
}

func (r *AnalyticsRepo) computeBandwidth(cache *CacheAnalyticsSummary, artifacts *ArtifactAnalyticsSummary) BandwidthAnalyticsSummary {
	bw := BandwidthAnalyticsSummary{
		UploadBytes:   cache.TotalBytesUploaded + artifacts.TotalSizeBytes,
		DownloadBytes: cache.TotalBytesDownloaded,
	}
	bw.TotalBytes = bw.UploadBytes + bw.DownloadBytes

	// Merge cache chart data into bandwidth chart (artifact bandwidth is upload-only at creation time)
	dateMap := make(map[string]*DailyBandwidthPoint)
	for _, cp := range cache.Chart {
		key := cp.Date.Format("2006-01-02")
		dateMap[key] = &DailyBandwidthPoint{
			Date:          cp.Date,
			UploadBytes:   cp.BytesUploaded,
			DownloadBytes: cp.BytesDownloaded,
		}
	}
	for _, ap := range artifacts.Chart {
		key := ap.Date.Format("2006-01-02")
		if bp, ok := dateMap[key]; ok {
			bp.UploadBytes += ap.SizeBytes
		} else {
			dateMap[key] = &DailyBandwidthPoint{
				Date:        ap.Date,
				UploadBytes: ap.SizeBytes,
			}
		}
	}

	// Collect and sort
	for _, bp := range dateMap {
		bw.Chart = append(bw.Chart, *bp)
	}
	// Sort by date
	sortBandwidthChart(bw.Chart)

	return bw
}

// sortBandwidthChart sorts bandwidth chart points by date ascending.
func sortBandwidthChart(chart []DailyBandwidthPoint) {
	for i := 1; i < len(chart); i++ {
		for j := i; j > 0 && chart[j].Date.Before(chart[j-1].Date); j-- {
			chart[j], chart[j-1] = chart[j-1], chart[j]
		}
	}
}

func (r *AnalyticsRepo) getAIAnalytics(ctx context.Context, projectIDs []uuid.UUID, days int) (*AIAnalyticsSummary, error) {
	summary := &AIAnalyticsSummary{}

	// Totals
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'success'),
			COUNT(*) FILTER (WHERE status = 'failed')
		FROM ai_analyses
		WHERE project_id = ANY($1)
		  AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
	`, projectIDs, days).Scan(
		&summary.TotalAnalyses,
		&summary.SuccessAnalyses,
		&summary.FailedAnalyses,
	)
	if err != nil {
		return nil, err
	}

	// Suggestions totals
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*),
			   COUNT(*) FILTER (WHERE s.status = 'accepted')
		FROM ai_suggestions s
		JOIN ai_analyses a ON a.id = s.analysis_id
		WHERE a.project_id = ANY($1)
		  AND s.created_at >= NOW() - ($2::int * INTERVAL '1 day')
	`, projectIDs, days).Scan(
		&summary.TotalSuggestions,
		&summary.AppliedSuggestions,
	)
	if err != nil {
		return nil, err
	}

	// Chart
	rows, err := r.pool.Query(ctx, `
		SELECT DATE(a.created_at) AS day,
			   COUNT(DISTINCT a.id) AS analyses,
			   COUNT(s.id) AS suggestions
		FROM ai_analyses a
		LEFT JOIN ai_suggestions s ON s.analysis_id = a.id
		WHERE a.project_id = ANY($1)
		  AND a.created_at >= NOW() - ($2::int * INTERVAL '1 day')
		GROUP BY day ORDER BY day ASC
	`, projectIDs, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p DailyAIPoint
		if err := rows.Scan(&p.Date, &p.Analyses, &p.Suggestions); err != nil {
			return nil, err
		}
		summary.Chart = append(summary.Chart, p)
	}

	return summary, rows.Err()
}

func (r *AnalyticsRepo) getAuditLogAnalytics(ctx context.Context, projectIDs []uuid.UUID, teamID *uuid.UUID, days int) (*AuditLogAnalyticsSummary, error) {
	summary := &AuditLogAnalyticsSummary{}

	// Build where clause based on scope
	var whereClause string
	var args []interface{}
	if teamID != nil {
		whereClause = "team_id = $1 AND created_at >= NOW() - ($2::int * INTERVAL '1 day')"
		args = []interface{}{*teamID, days}
	} else {
		whereClause = "project_id = ANY($1) AND created_at >= NOW() - ($2::int * INTERVAL '1 day')"
		args = []interface{}{projectIDs, days}
	}

	// Total events
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE `+whereClause, args...,
	).Scan(&summary.TotalEvents)
	if err != nil {
		return nil, err
	}

	// Top actions
	rows, err := r.pool.Query(ctx,
		`SELECT action, COUNT(*) AS cnt FROM audit_logs WHERE `+whereClause+` GROUP BY action ORDER BY cnt DESC LIMIT 10`, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var a AuditActionCount
		if err := rows.Scan(&a.Action, &a.Count); err != nil {
			return nil, err
		}
		summary.TopActions = append(summary.TopActions, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Top actors
	rows2, err := r.pool.Query(ctx,
		`SELECT actor_email, COUNT(*) AS cnt FROM audit_logs WHERE `+whereClause+` GROUP BY actor_email ORDER BY cnt DESC LIMIT 10`, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var a AuditActorCount
		if err := rows2.Scan(&a.ActorEmail, &a.Count); err != nil {
			return nil, err
		}
		summary.TopActors = append(summary.TopActors, a)
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	// Chart
	rows3, err := r.pool.Query(ctx,
		`SELECT DATE(created_at) AS day, COUNT(*) AS events FROM audit_logs WHERE `+whereClause+` GROUP BY day ORDER BY day ASC`, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows3.Close()

	for rows3.Next() {
		var p DailyAuditPoint
		if err := rows3.Scan(&p.Date, &p.EventCount); err != nil {
			return nil, err
		}
		summary.Chart = append(summary.Chart, p)
	}

	return summary, rows3.Err()
}

func (r *AnalyticsRepo) getProjectLeaderboard(ctx context.Context, projectIDs []uuid.UUID) ([]ProjectActivitySummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.name,
			   COALESCE(r.total_runs, 0),
			   COALESCE(r.success_rate, 0),
			   COALESCE(c.current_size_bytes, 0),
			   COALESCE(a.total_size, 0),
			   COALESCE(cu.bandwidth, 0)
		FROM projects p
		LEFT JOIN (
			SELECT project_id,
				   COUNT(*) AS total_runs,
				   CASE WHEN COUNT(*) > 0
					   THEN (COUNT(*) FILTER (WHERE status = 'success'))::float / COUNT(*) * 100
					   ELSE 0
				   END AS success_rate
			FROM runs
			GROUP BY project_id
		) r ON r.project_id = p.id
		LEFT JOIN cache_quotas c ON c.project_id = p.id
		LEFT JOIN (
			SELECT project_id, SUM(size_bytes) AS total_size
			FROM artifacts
			GROUP BY project_id
		) a ON a.project_id = p.id
		LEFT JOIN (
			SELECT project_id, SUM(bytes_uploaded + bytes_downloaded) AS bandwidth
			FROM cache_usage
			GROUP BY project_id
		) cu ON cu.project_id = p.id
		WHERE p.id = ANY($1)
		ORDER BY r.total_runs DESC NULLS LAST
	`, projectIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []ProjectActivitySummary
	for rows.Next() {
		var p ProjectActivitySummary
		if err := rows.Scan(
			&p.ProjectID, &p.ProjectName,
			&p.TotalRuns, &p.SuccessRate,
			&p.CacheSize, &p.ArtifactSize,
			&p.Bandwidth,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}

	return projects, rows.Err()
}
