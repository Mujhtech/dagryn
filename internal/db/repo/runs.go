package repo

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/internal/db/models"
)

// RunRepo handles run database operations.
type RunRepo struct {
	pool *pgxpool.Pool
}

// RunDashboardChartPoint is an aggregated daily run summary.
type RunDashboardChartPoint struct {
	Date       time.Time
	Success    int
	Failed     int
	DurationMs int64
}

// RunDashboardUserFacet is a selectable user facet in run dashboards.
type RunDashboardUserFacet struct {
	ID        string
	Name      string
	AvatarURL *string
}

// RunDashboardFacets contains stable, non-paginated run filters.
type RunDashboardFacets struct {
	Users       []RunDashboardUserFacet
	Workflows   []string
	Branches    []string
	StatusCount map[string]int
}

// NewRunRepo creates a new run repository.
func NewRunRepo(pool *pgxpool.Pool) *RunRepo {
	return &RunRepo{pool: pool}
}

// Create creates a new run.
func (r *RunRepo) Create(ctx context.Context, run *models.Run) error {
	run.ID = uuid.New()
	run.CreatedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO runs (id, project_id, targets, status, total_tasks, triggered_by, triggered_by_user_id, git_branch, git_commit, pr_title, pr_number, commit_message, commit_author_name, commit_author_email, workflow_id, workflow_name, host_os, host_arch, host_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`, run.ID, run.ProjectID, run.Targets, run.Status, run.TotalTasks, run.TriggeredBy,
		run.TriggeredByUserID, run.GitBranch, run.GitCommit, run.PRTitle, run.PRNumber, run.CommitMessage, run.CommitAuthorName, run.CommitAuthorEmail, run.WorkflowID, run.WorkflowName, run.HostOS, run.HostArch, run.HostName, run.CreatedAt)

	return err
}

// GetByID retrieves a run by ID.
func (r *RunRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Run, error) {
	var run models.Run
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, targets, status, total_tasks, completed_tasks, failed_tasks, cache_hits,
		       duration_ms, error_message, triggered_by, triggered_by_user_id, git_branch, git_commit,
		       pr_title, pr_number, commit_message, commit_author_name, commit_author_email,
		       workflow_id, workflow_name,
		       github_pr_comment_id, github_check_run_id,
		       host_os, host_arch, host_name,
		       started_at, finished_at, last_heartbeat_at, client_disconnected, created_at
		FROM runs WHERE id = $1
	`, id).Scan(&run.ID, &run.ProjectID, &run.Targets, &run.Status, &run.TotalTasks, &run.CompletedTasks,
		&run.FailedTasks, &run.CacheHits, &run.DurationMs, &run.ErrorMessage, &run.TriggeredBy,
		&run.TriggeredByUserID, &run.GitBranch, &run.GitCommit, &run.PRTitle, &run.PRNumber,
		&run.CommitMessage, &run.CommitAuthorName, &run.CommitAuthorEmail, &run.WorkflowID, &run.WorkflowName,
		&run.GitHubPRCommentID, &run.GitHubCheckRunID,
		&run.HostOS, &run.HostArch, &run.HostName,
		&run.StartedAt, &run.FinishedAt, &run.LastHeartbeatAt, &run.ClientDisconnected, &run.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &run, nil
}

// Update updates a run.
func (r *RunRepo) Update(ctx context.Context, run *models.Run) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE runs SET status = $1, total_tasks = $2, completed_tasks = $3, failed_tasks = $4, cache_hits = $5,
		       duration_ms = $6, error_message = $7, started_at = $8, finished_at = $9,
		       github_pr_comment_id = $10, github_check_run_id = $11, git_commit = $12, git_branch = $13,
		       workflow_id = $14, workflow_name = $15, host_os = $16, host_arch = $17, host_name = $18
		WHERE id = $19
	`, run.Status, run.TotalTasks, run.CompletedTasks, run.FailedTasks, run.CacheHits, run.DurationMs,
		run.ErrorMessage, run.StartedAt, run.FinishedAt, run.GitHubPRCommentID, run.GitHubCheckRunID, run.GitCommit, run.GitBranch, run.WorkflowID, run.WorkflowName, run.HostOS, run.HostArch, run.HostName, run.ID)

	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateGitHubCheckRunID updates the GitHub check run ID for a run.
func (r *RunRepo) UpdateGitHubCheckRunID(ctx context.Context, id uuid.UUID, checkRunID int64) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE runs SET github_check_run_id = $1 WHERE id = $2
	`, checkRunID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateTargets updates the targets for a run.
func (r *RunRepo) UpdateTargets(ctx context.Context, id uuid.UUID, targets []string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE runs SET targets = $1 WHERE id = $2
	`, targets, id)

	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Start marks a run as started.
func (r *RunRepo) Start(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE runs SET status = $1, started_at = $2 WHERE id = $3
	`, models.RunStatusRunning, now, id)
	return err
}

// StartWithTotal marks a run as started and sets the total task count.
func (r *RunRepo) StartWithTotal(ctx context.Context, id uuid.UUID, totalTasks int) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE runs SET status = $1, started_at = $2, total_tasks = $3 WHERE id = $4
	`, models.RunStatusRunning, now, totalTasks, id)
	return err
}

// Complete marks a run as completed.
func (r *RunRepo) Complete(ctx context.Context, id uuid.UUID, status models.RunStatus, errorMessage *string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE runs SET status = $1, finished_at = $2, error_message = $3,
		       duration_ms = EXTRACT(EPOCH FROM ($2 - started_at)) * 1000
		WHERE id = $4
	`, status, now, errorMessage, id)
	return err
}

// IncrementCompleted increments the completed task count.
func (r *RunRepo) IncrementCompleted(ctx context.Context, id uuid.UUID, cacheHit bool) error {
	if cacheHit {
		_, err := r.pool.Exec(ctx, `
			UPDATE runs SET completed_tasks = completed_tasks + 1, cache_hits = cache_hits + 1 WHERE id = $1
		`, id)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE runs SET completed_tasks = completed_tasks + 1 WHERE id = $1
	`, id)
	return err
}

// IncrementFailed increments the failed task count.
func (r *RunRepo) IncrementFailed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE runs SET failed_tasks = failed_tasks + 1, completed_tasks = completed_tasks + 1 WHERE id = $1
	`, id)
	return err
}

// ListByProject returns runs for a project with pagination.
func (r *RunRepo) ListByProject(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]models.Run, int, error) {
	// Get total count
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM runs WHERE project_id = $1", projectID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get runs
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, targets, status, total_tasks, completed_tasks, failed_tasks, cache_hits,
		       duration_ms, error_message, triggered_by, triggered_by_user_id, git_branch, git_commit,
		       pr_title, pr_number, commit_message, commit_author_name, commit_author_email,
		       workflow_id, workflow_name,
		       github_pr_comment_id, github_check_run_id,
		       host_os, host_arch, host_name,
		       started_at, finished_at, last_heartbeat_at, client_disconnected, created_at
		FROM runs WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, projectID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var runs []models.Run
	for rows.Next() {
		var run models.Run
		if err := rows.Scan(&run.ID, &run.ProjectID, &run.Targets, &run.Status, &run.TotalTasks, &run.CompletedTasks,
			&run.FailedTasks, &run.CacheHits, &run.DurationMs, &run.ErrorMessage, &run.TriggeredBy,
			&run.TriggeredByUserID, &run.GitBranch, &run.GitCommit, &run.PRTitle, &run.PRNumber,
			&run.CommitMessage, &run.CommitAuthorName, &run.CommitAuthorEmail, &run.WorkflowID, &run.WorkflowName,
			&run.GitHubPRCommentID, &run.GitHubCheckRunID,
			&run.HostOS, &run.HostArch, &run.HostName,
			&run.StartedAt, &run.FinishedAt, &run.LastHeartbeatAt, &run.ClientDisconnected, &run.CreatedAt); err != nil {
			return nil, 0, err
		}
		runs = append(runs, run)
	}
	return runs, total, rows.Err()
}

// GetDashboardChartByProject returns daily aggregated run counts and duration.
func (r *RunRepo) GetDashboardChartByProject(ctx context.Context, projectID uuid.UUID, days int) ([]RunDashboardChartPoint, error) {
	if days <= 0 {
		days = 30
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			DATE(created_at) AS day,
			COUNT(*) FILTER (WHERE status = 'success') AS success_count,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed_count,
			COALESCE(ROUND(AVG(duration_ms)), 0)::bigint AS avg_duration_ms
		FROM runs
		WHERE project_id = $1
		  AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
		GROUP BY day
		ORDER BY day ASC
	`, projectID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]RunDashboardChartPoint, 0)
	for rows.Next() {
		var p RunDashboardChartPoint
		if err := rows.Scan(&p.Date, &p.Success, &p.Failed, &p.DurationMs); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// GetDashboardFacetsByProject returns stable, non-paginated filter facets.
func (r *RunRepo) GetDashboardFacetsByProject(ctx context.Context, projectID uuid.UUID) (*RunDashboardFacets, error) {
	facets := &RunDashboardFacets{
		Users:       make([]RunDashboardUserFacet, 0),
		Workflows:   make([]string, 0),
		Branches:    make([]string, 0),
		StatusCount: map[string]int{},
	}

	// Status counts
	statusRows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*)::int
		FROM runs
		WHERE project_id = $1
		GROUP BY status
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var status string
		var count int
		if err := statusRows.Scan(&status, &count); err != nil {
			return nil, err
		}
		facets.StatusCount[status] = count
	}
	if err := statusRows.Err(); err != nil {
		return nil, err
	}

	// Workflow names: prefer stored workflow_name, fallback to first target.
	workflowRows, err := r.pool.Query(ctx, `
		SELECT DISTINCT COALESCE(NULLIF(workflow_name, ''), NULLIF(targets[1], '')) AS workflow
		FROM runs
		WHERE project_id = $1
		  AND COALESCE(NULLIF(workflow_name, ''), NULLIF(targets[1], '')) IS NOT NULL
		ORDER BY workflow ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer workflowRows.Close()
	for workflowRows.Next() {
		var workflow string
		if err := workflowRows.Scan(&workflow); err != nil {
			return nil, err
		}
		facets.Workflows = append(facets.Workflows, workflow)
	}
	if err := workflowRows.Err(); err != nil {
		return nil, err
	}

	// Branches
	branchRows, err := r.pool.Query(ctx, `
		SELECT DISTINCT git_branch
		FROM runs
		WHERE project_id = $1
		  AND git_branch IS NOT NULL
		  AND git_branch <> ''
		ORDER BY git_branch ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer branchRows.Close()
	for branchRows.Next() {
		var branch string
		if err := branchRows.Scan(&branch); err != nil {
			return nil, err
		}
		facets.Branches = append(facets.Branches, branch)
	}
	if err := branchRows.Err(); err != nil {
		return nil, err
	}

	// Users from triggered_by_user_id joined against users.
	userIndex := make(map[string]RunDashboardUserFacet)
	userRows, err := r.pool.Query(ctx, `
		SELECT DISTINCT
			u.id::text AS id,
			COALESCE(NULLIF(u.name, ''), u.email) AS name,
			u.avatar_url
		FROM runs r
		JOIN users u ON u.id = r.triggered_by_user_id
		WHERE r.project_id = $1
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer userRows.Close()
	for userRows.Next() {
		var user RunDashboardUserFacet
		if err := userRows.Scan(&user.ID, &user.Name, &user.AvatarURL); err != nil {
			return nil, err
		}
		userIndex[user.ID] = user
	}
	if err := userRows.Err(); err != nil {
		return nil, err
	}

	// Commit authors as additional filter users.
	authorRows, err := r.pool.Query(ctx, `
		SELECT DISTINCT
			COALESCE(NULLIF(commit_author_email, ''), NULLIF(commit_author_name, '')) AS id,
			COALESCE(NULLIF(commit_author_name, ''), commit_author_email) AS name
		FROM runs
		WHERE project_id = $1
		  AND COALESCE(NULLIF(commit_author_email, ''), NULLIF(commit_author_name, '')) IS NOT NULL
		  AND COALESCE(NULLIF(commit_author_name, ''), commit_author_email) IS NOT NULL
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer authorRows.Close()
	for authorRows.Next() {
		var id, name string
		if err := authorRows.Scan(&id, &name); err != nil {
			return nil, err
		}
		if _, exists := userIndex[id]; !exists {
			userIndex[id] = RunDashboardUserFacet{
				ID:   id,
				Name: name,
			}
		}
	}
	if err := authorRows.Err(); err != nil {
		return nil, err
	}

	facets.Users = make([]RunDashboardUserFacet, 0, len(userIndex))
	for _, user := range userIndex {
		facets.Users = append(facets.Users, user)
	}
	sort.Slice(facets.Users, func(i, j int) bool {
		return facets.Users[i].Name < facets.Users[j].Name
	})

	return facets, nil
}

// GetActiveByProject returns currently running/pending runs for a project.
func (r *RunRepo) GetActiveByProject(ctx context.Context, projectID uuid.UUID) ([]models.Run, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, targets, status, total_tasks, completed_tasks, failed_tasks, cache_hits,
		       duration_ms, error_message, triggered_by, triggered_by_user_id, git_branch, git_commit,
		       pr_title, pr_number, commit_message, commit_author_name, commit_author_email,
		       workflow_id, workflow_name,
		       github_pr_comment_id, github_check_run_id,
		       host_os, host_arch, host_name,
		       started_at, finished_at, last_heartbeat_at, client_disconnected, created_at
		FROM runs WHERE project_id = $1 AND status IN ('pending', 'running')
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []models.Run
	for rows.Next() {
		var run models.Run
		if err := rows.Scan(&run.ID, &run.ProjectID, &run.Targets, &run.Status, &run.TotalTasks, &run.CompletedTasks,
			&run.FailedTasks, &run.CacheHits, &run.DurationMs, &run.ErrorMessage, &run.TriggeredBy,
			&run.TriggeredByUserID, &run.GitBranch, &run.GitCommit, &run.PRTitle, &run.PRNumber,
			&run.CommitMessage, &run.CommitAuthorName, &run.CommitAuthorEmail, &run.WorkflowID, &run.WorkflowName,
			&run.GitHubPRCommentID, &run.GitHubCheckRunID,
			&run.HostOS, &run.HostArch, &run.HostName,
			&run.StartedAt, &run.FinishedAt, &run.LastHeartbeatAt, &run.ClientDisconnected, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// Delete deletes a run and its task results.
func (r *RunRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM runs WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateTaskResult creates a task result.
func (r *RunRepo) CreateTaskResult(ctx context.Context, result *models.TaskResult) error {
	result.ID = uuid.New()
	result.CreatedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO task_results (id, run_id, task_name, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, result.ID, result.RunID, result.TaskName, result.Status, result.CreatedAt)

	return err
}

// UpdateTaskResult updates a task result.
func (r *RunRepo) UpdateTaskResult(ctx context.Context, result *models.TaskResult) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE task_results SET status = $1, duration_ms = $2, exit_code = $3, output = $4, 
		       error_message = $5, cache_hit = $6, cache_key = $7, started_at = $8, finished_at = $9
		WHERE id = $10
	`, result.Status, result.DurationMs, result.ExitCode, result.Output, result.ErrorMessage,
		result.CacheHit, result.CacheKey, result.StartedAt, result.FinishedAt, result.ID)
	return err
}

// GetTaskResult retrieves a task result by run ID and task name.
func (r *RunRepo) GetTaskResult(ctx context.Context, runID uuid.UUID, taskName string) (*models.TaskResult, error) {
	var result models.TaskResult
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_id, task_name, status, duration_ms, exit_code, output, error_message, 
		       cache_hit, cache_key, started_at, finished_at, created_at
		FROM task_results WHERE run_id = $1 AND task_name = $2
	`, runID, taskName).Scan(&result.ID, &result.RunID, &result.TaskName, &result.Status, &result.DurationMs,
		&result.ExitCode, &result.Output, &result.ErrorMessage, &result.CacheHit, &result.CacheKey,
		&result.StartedAt, &result.FinishedAt, &result.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &result, nil
}

// ListTaskResults returns all task results for a run.
func (r *RunRepo) ListTaskResults(ctx context.Context, runID uuid.UUID) ([]models.TaskResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, task_name, status, duration_ms, exit_code, output, error_message, 
		       cache_hit, cache_key, started_at, finished_at, created_at
		FROM task_results WHERE run_id = $1
		ORDER BY created_at
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.TaskResult
	for rows.Next() {
		var result models.TaskResult
		if err := rows.Scan(&result.ID, &result.RunID, &result.TaskName, &result.Status, &result.DurationMs,
			&result.ExitCode, &result.Output, &result.ErrorMessage, &result.CacheHit, &result.CacheKey,
			&result.StartedAt, &result.FinishedAt, &result.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

// GetRunWithTasks retrieves a run with all its task results.
func (r *RunRepo) GetRunWithTasks(ctx context.Context, id uuid.UUID) (*models.RunWithTasks, error) {
	run, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	tasks, err := r.ListTaskResults(ctx, id)
	if err != nil {
		return nil, err
	}

	return &models.RunWithTasks{
		Run:   *run,
		Tasks: tasks,
	}, nil
}

// CleanupOldRuns deletes runs older than the specified duration.
func (r *RunRepo) CleanupOldRuns(ctx context.Context, olderThan time.Duration, keepMinimum int) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	// Delete old runs but keep at least keepMinimum recent runs per project
	result, err := r.pool.Exec(ctx, `
		DELETE FROM runs r
		WHERE r.created_at < $1
		AND r.id NOT IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at DESC) as rn
				FROM runs
			) ranked
			WHERE rn <= $2
		)
	`, cutoff, keepMinimum)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// UpdateHeartbeat updates the last heartbeat timestamp for a run.
func (r *RunRepo) UpdateHeartbeat(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result, err := r.pool.Exec(ctx, `
		UPDATE runs SET last_heartbeat_at = $1, client_disconnected = false WHERE id = $2
	`, now, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListStaleRuns returns runs that are in "running" status with stale heartbeats.
func (r *RunRepo) ListStaleRuns(ctx context.Context, timeout time.Duration) ([]models.Run, error) {
	cutoff := time.Now().Add(-timeout)
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, targets, status, total_tasks, completed_tasks, failed_tasks, cache_hits,
		       duration_ms, error_message, triggered_by, triggered_by_user_id, git_branch, git_commit,
		       pr_title, pr_number, commit_message, commit_author_name, commit_author_email,
		       workflow_id, workflow_name,
		       host_os, host_arch, host_name,
		       started_at, finished_at, last_heartbeat_at, client_disconnected, created_at
		FROM runs
		WHERE status = 'running'
		  AND last_heartbeat_at IS NOT NULL
		  AND last_heartbeat_at < $1
		  AND client_disconnected = false
		ORDER BY last_heartbeat_at ASC
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []models.Run
	for rows.Next() {
		var run models.Run
		if err := rows.Scan(&run.ID, &run.ProjectID, &run.Targets, &run.Status, &run.TotalTasks, &run.CompletedTasks,
			&run.FailedTasks, &run.CacheHits, &run.DurationMs, &run.ErrorMessage, &run.TriggeredBy,
			&run.TriggeredByUserID, &run.GitBranch, &run.GitCommit, &run.PRTitle, &run.PRNumber,
			&run.CommitMessage, &run.CommitAuthorName, &run.CommitAuthorEmail, &run.WorkflowID, &run.WorkflowName,
			&run.HostOS, &run.HostArch, &run.HostName,
			&run.StartedAt, &run.FinishedAt, &run.LastHeartbeatAt, &run.ClientDisconnected, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// MarkAsStale marks a run as having a disconnected client.
func (r *RunRepo) MarkAsStale(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE runs SET client_disconnected = true WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Run Log Operations ---

// AppendLog appends a single log entry to the database.
func (r *RunRepo) AppendLog(ctx context.Context, log *models.RunLog) error {
	log.CreatedAt = time.Now()

	err := r.pool.QueryRow(ctx, `
		INSERT INTO run_logs (run_id, task_name, stream, line_num, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, log.RunID, log.TaskName, log.Stream, log.LineNum, log.Content, log.CreatedAt).Scan(&log.ID)

	return err
}

// AppendLogs appends multiple log entries in a batch.
func (r *RunRepo) AppendLogs(ctx context.Context, logs []models.RunLog) error {
	if len(logs) == 0 {
		return nil
	}

	now := time.Now()

	// Use COPY for efficient bulk inserts
	rows := make([][]interface{}, len(logs))
	for i, log := range logs {
		rows[i] = []interface{}{log.RunID, log.TaskName, log.Stream, log.LineNum, log.Content, now}
	}

	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"run_logs"},
		[]string{"run_id", "task_name", "stream", "line_num", "content", "created_at"},
		pgx.CopyFromRows(rows),
	)

	return err
}

// GetLogs retrieves logs for a run with pagination.
func (r *RunRepo) GetLogs(ctx context.Context, runID uuid.UUID, limit, offset int) ([]models.RunLog, int, error) {
	// Get total count
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM run_logs WHERE run_id = $1", runID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get logs
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, task_name, stream, line_num, content, created_at
		FROM run_logs WHERE run_id = $1
		ORDER BY id ASC
		LIMIT $2 OFFSET $3
	`, runID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.RunLog
	for rows.Next() {
		var log models.RunLog
		if err := rows.Scan(&log.ID, &log.RunID, &log.TaskName, &log.Stream, &log.LineNum, &log.Content, &log.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}

// GetLogsByTask retrieves logs for a specific task within a run.
func (r *RunRepo) GetLogsByTask(ctx context.Context, runID uuid.UUID, taskName string, limit, offset int) ([]models.RunLog, int, error) {
	// Get total count
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM run_logs WHERE run_id = $1 AND task_name = $2", runID, taskName).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get logs
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, task_name, stream, line_num, content, created_at
		FROM run_logs WHERE run_id = $1 AND task_name = $2
		ORDER BY id ASC
		LIMIT $3 OFFSET $4
	`, runID, taskName, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.RunLog
	for rows.Next() {
		var log models.RunLog
		if err := rows.Scan(&log.ID, &log.RunID, &log.TaskName, &log.Stream, &log.LineNum, &log.Content, &log.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}

// GetLogsSince retrieves logs after a specific ID (for polling/streaming).
func (r *RunRepo) GetLogsSince(ctx context.Context, runID uuid.UUID, afterID int64, limit int) ([]models.RunLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, task_name, stream, line_num, content, created_at
		FROM run_logs WHERE run_id = $1 AND id > $2
		ORDER BY id ASC
		LIMIT $3
	`, runID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.RunLog
	for rows.Next() {
		var log models.RunLog
		if err := rows.Scan(&log.ID, &log.RunID, &log.TaskName, &log.Stream, &log.LineNum, &log.Content, &log.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// DeleteLogs deletes all logs for a run.
func (r *RunRepo) DeleteLogs(ctx context.Context, runID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM run_logs WHERE run_id = $1", runID)
	return err
}

// DeleteLogsOlderThanForProjects removes run logs for runs in the given projects
// that were created before the specified time.
func (r *RunRepo) DeleteLogsOlderThanForProjects(ctx context.Context, projectIDs []uuid.UUID, before time.Time) (int64, error) {
	if len(projectIDs) == 0 {
		return 0, nil
	}
	result, err := r.pool.Exec(ctx, `
		DELETE FROM run_logs
		WHERE run_id IN (
			SELECT id FROM runs WHERE project_id = ANY($1) AND created_at < $2
		)
	`, projectIDs, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
