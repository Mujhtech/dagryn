package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// ClusterStore defines the interface for cluster repository operations.
type ClusterStore interface {
	// Clusters
	CreateCluster(ctx context.Context, cluster *models.Cluster) error
	GetCluster(ctx context.Context, id uuid.UUID) (*models.Cluster, error)
	GetClusterByName(ctx context.Context, name string) (*models.Cluster, error)
	GetClusterByNameInScope(ctx context.Context, name string, teamID *uuid.UUID, ownerUserID *uuid.UUID) (*models.Cluster, error)
	EnsureDefaultCluster(ctx context.Context) (*models.Cluster, error)
	EnsureDefaultClusterForScope(ctx context.Context, teamID *uuid.UUID, ownerUserID *uuid.UUID) (*models.Cluster, error)
	ListClusters(ctx context.Context) ([]models.Cluster, error)
	ListClustersForScope(ctx context.Context, teamID *uuid.UUID, ownerUserID *uuid.UUID) ([]models.Cluster, error)
	UpdateCluster(ctx context.Context, cluster *models.Cluster) error
	DeleteCluster(ctx context.Context, id uuid.UUID) error

	// Workers
	RegisterWorker(ctx context.Context, worker *models.Worker) error
	GetWorker(ctx context.Context, id uuid.UUID) (*models.Worker, error)
	ListWorkers(ctx context.Context, clusterID *uuid.UUID, status *models.WorkerStatus) ([]models.Worker, error)
	UpdateWorkerHeartbeat(ctx context.Context, id uuid.UUID, resources *WorkerResourceUpdate) error
	UpdateWorkerStatus(ctx context.Context, id uuid.UUID, status models.WorkerStatus) error
	DeleteWorker(ctx context.Context, id uuid.UUID) error
	ListStaleWorkers(ctx context.Context, threshold time.Time) ([]models.Worker, error)

	// Task Assignments
	CreateTaskAssignment(ctx context.Context, assignment *models.TaskAssignment) error
	GetTaskAssignment(ctx context.Context, id uuid.UUID) (*models.TaskAssignment, error)
	ListTaskAssignmentsByRun(ctx context.Context, runID uuid.UUID) ([]models.TaskAssignment, error)
	ListTaskAssignmentsByWorker(ctx context.Context, workerID uuid.UUID, limit int) ([]models.TaskAssignment, error)
	UpdateTaskAssignmentStatus(ctx context.Context, id uuid.UUID, status models.TaskAssignmentStatus, result json.RawMessage) error
	ListOrphanedAssignments(ctx context.Context) ([]models.TaskAssignment, error)
	IncrementAssignmentRetry(ctx context.Context, id uuid.UUID) error
}

// WorkerResourceUpdate holds fields to update on heartbeat.
type WorkerResourceUpdate struct {
	CPUMillicoresAvail int64
	MemoryBytesAvail   int64
	DiskBytesAvail     int64
	CPUUsagePercent    float64
	MemoryUsagePercent float64
	ActiveTasks        int
}

// ClusterRepo implements ClusterStore using PostgreSQL.
type ClusterRepo struct {
	pool *pgxpool.Pool
}

// NewClusterRepo creates a new cluster repository.
func NewClusterRepo(pool *pgxpool.Pool) *ClusterRepo {
	return &ClusterRepo{pool: pool}
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ── Clusters ──

func (r *ClusterRepo) CreateCluster(ctx context.Context, cluster *models.Cluster) error {
	if cluster.ID == uuid.Nil {
		cluster.ID = uuid.New()
	}
	if cluster.Slug == "" {
		cluster.Slug = strings.ToLower(strings.ReplaceAll(cluster.Name, " ", "-"))
	}
	if cluster.ScopeType == "" {
		cluster.ScopeType = "global"
	}
	now := time.Now()
	cluster.CreatedAt = now
	cluster.UpdatedAt = now
	if cluster.Labels == nil {
		cluster.Labels = json.RawMessage(`{}`)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO clusters (id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		cluster.ID, cluster.Name, cluster.Slug, cluster.Description, cluster.Labels, cluster.ScopeType,
		cluster.TeamID, cluster.OwnerUserID, cluster.SystemDefault, cluster.CreatedAt, cluster.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (r *ClusterRepo) GetCluster(ctx context.Context, id uuid.UUID) (*models.Cluster, error) {
	var c models.Cluster
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at FROM clusters WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return &c, err
}

func (r *ClusterRepo) GetClusterByName(ctx context.Context, name string) (*models.Cluster, error) {
	var c models.Cluster
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at
		 FROM clusters
		 WHERE scope_type = 'global' AND (name = $1 OR slug = $1)
		 ORDER BY created_at LIMIT 1`, name,
	).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return &c, err
}

func (r *ClusterRepo) GetClusterByNameInScope(ctx context.Context, name string, teamID *uuid.UUID, ownerUserID *uuid.UUID) (*models.Cluster, error) {
	if teamID != nil {
		var c models.Cluster
		err := r.pool.QueryRow(ctx,
			`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at
			 FROM clusters WHERE scope_type = 'team' AND team_id = $1 AND (name = $2 OR slug = $2) ORDER BY created_at LIMIT 1`,
			*teamID, name,
		).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt)
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return &c, err
	}

	if ownerUserID != nil {
		var c models.Cluster
		err := r.pool.QueryRow(ctx,
			`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at
			 FROM clusters WHERE scope_type = 'personal' AND owner_user_id = $1 AND (name = $2 OR slug = $2) ORDER BY created_at LIMIT 1`,
			*ownerUserID, name,
		).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt)
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return &c, err
	}

	return r.GetClusterByName(ctx, name)
}

func (r *ClusterRepo) EnsureDefaultCluster(ctx context.Context) (*models.Cluster, error) {
	return r.EnsureDefaultClusterForScope(ctx, nil, nil)
}

func (r *ClusterRepo) EnsureDefaultClusterForScope(ctx context.Context, teamID *uuid.UUID, ownerUserID *uuid.UUID) (*models.Cluster, error) {
	if teamID != nil {
		var c models.Cluster
		err := r.pool.QueryRow(ctx,
			`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at
			 FROM clusters WHERE scope_type = 'team' AND team_id = $1 AND slug = 'default' ORDER BY created_at LIMIT 1`, *teamID,
		).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt)
		if err == nil {
			return &c, nil
		}
		if err != pgx.ErrNoRows {
			return nil, err
		}

		cluster := &models.Cluster{
			Name:          "default",
			Slug:          "default",
			Description:   "Default team cluster",
			Labels:        json.RawMessage(`{"system_default":true}`),
			ScopeType:     "team",
			TeamID:        teamID,
			SystemDefault: true,
		}
		if createErr := r.CreateCluster(ctx, cluster); createErr != nil {
			if !errors.Is(createErr, ErrAlreadyExists) {
				return nil, createErr
			}
			return r.EnsureDefaultClusterForScope(ctx, teamID, nil)
		}
		return cluster, nil
	}

	if ownerUserID != nil {
		var c models.Cluster
		err := r.pool.QueryRow(ctx,
			`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at
			 FROM clusters WHERE scope_type = 'personal' AND owner_user_id = $1 AND slug = 'default' ORDER BY created_at LIMIT 1`, *ownerUserID,
		).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt)
		if err == nil {
			return &c, nil
		}
		if err != pgx.ErrNoRows {
			return nil, err
		}

		cluster := &models.Cluster{
			Name:          "default",
			Slug:          "default",
			Description:   "Default personal cluster",
			Labels:        json.RawMessage(`{"system_default":true}`),
			ScopeType:     "personal",
			OwnerUserID:   ownerUserID,
			SystemDefault: true,
		}
		if createErr := r.CreateCluster(ctx, cluster); createErr != nil {
			if !errors.Is(createErr, ErrAlreadyExists) {
				return nil, createErr
			}
			return r.EnsureDefaultClusterForScope(ctx, nil, ownerUserID)
		}
		return cluster, nil
	}

	cluster, err := r.GetClusterByName(ctx, "default")
	if err == nil {
		return cluster, nil
	}

	c := &models.Cluster{
		Name:          "default",
		Slug:          "default",
		Description:   "Default cluster",
		Labels:        json.RawMessage(`{"system_default":true}`),
		ScopeType:     "global",
		SystemDefault: true,
	}
	if createErr := r.CreateCluster(ctx, c); createErr != nil {
		cluster, retryErr := r.GetClusterByName(ctx, "default")
		if retryErr == nil {
			return cluster, nil
		}
		return nil, createErr
	}
	return c, nil
}

func (r *ClusterRepo) ListClusters(ctx context.Context) ([]models.Cluster, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at FROM clusters ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []models.Cluster
	for rows.Next() {
		var c models.Cluster
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}
	return clusters, rows.Err()
}

func (r *ClusterRepo) ListClustersForScope(ctx context.Context, teamID *uuid.UUID, ownerUserID *uuid.UUID) ([]models.Cluster, error) {
	if teamID != nil {
		rows, err := r.pool.Query(ctx,
			`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at
			 FROM clusters WHERE scope_type = 'team' AND team_id = $1 ORDER BY name`, *teamID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []models.Cluster
		for rows.Next() {
			var c models.Cluster
			if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}

	if ownerUserID != nil {
		rows, err := r.pool.Query(ctx,
			`SELECT id, name, slug, description, labels, scope_type, team_id, owner_user_id, is_system_default, created_at, updated_at
			 FROM clusters WHERE scope_type = 'personal' AND owner_user_id = $1 ORDER BY name`, *ownerUserID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []models.Cluster
		for rows.Next() {
			var c models.Cluster
			if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Labels, &c.ScopeType, &c.TeamID, &c.OwnerUserID, &c.SystemDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}

	return r.ListClusters(ctx)
}

func (r *ClusterRepo) UpdateCluster(ctx context.Context, cluster *models.Cluster) error {
	cluster.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE clusters SET name = $1, slug = $2, description = $3, labels = $4, scope_type = $5, team_id = $6, owner_user_id = $7, is_system_default = $8, updated_at = $9 WHERE id = $10`,
		cluster.Name, cluster.Slug, cluster.Description, cluster.Labels, cluster.ScopeType, cluster.TeamID, cluster.OwnerUserID, cluster.SystemDefault, cluster.UpdatedAt, cluster.ID,
	)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (r *ClusterRepo) DeleteCluster(ctx context.Context, id uuid.UUID) error {
	cluster, err := r.GetCluster(ctx, id)
	if err != nil {
		return err
	}
	if cluster.SystemDefault || cluster.Slug == "default" {
		return fmt.Errorf("cannot delete default cluster")
	}

	var workerCount int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM workers WHERE cluster_id = $1`, id).Scan(&workerCount); err != nil {
		return err
	}
	if workerCount > 0 {
		return fmt.Errorf("cannot delete cluster with registered workers")
	}

	var assignmentCount int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM task_assignments WHERE cluster_id = $1 AND status IN ('pending','assigned','running')`, id).Scan(&assignmentCount); err != nil {
		return err
	}
	if assignmentCount > 0 {
		return fmt.Errorf("cannot delete cluster with active task assignments")
	}

	_, err = r.pool.Exec(ctx, `DELETE FROM clusters WHERE id = $1`, id)
	return err
}

// ── Workers ──

func (r *ClusterRepo) RegisterWorker(ctx context.Context, worker *models.Worker) error {
	if worker.ID == uuid.Nil {
		worker.ID = uuid.New()
	}
	now := time.Now()
	worker.RegisteredAt = now
	worker.LastHeartbeatAt = now
	worker.ResourcesUpdatedAt = now
	if worker.Labels == nil {
		worker.Labels = json.RawMessage(`{}`)
	}

	if worker.ClusterID != nil {
		return r.pool.QueryRow(ctx,
			`INSERT INTO workers (id, hostname, os, arch, environment, labels, capabilities,
			 max_concurrent_tasks, version, status, last_heartbeat_at, registered_at, auth_token_hash,
			 cluster_id, cpu_millicores_available, memory_bytes_available, disk_bytes_available,
			 cpu_usage_percent, memory_usage_percent, active_tasks, resources_updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
			 ON CONFLICT (hostname, cluster_id) WHERE cluster_id IS NOT NULL
			 DO UPDATE SET
			   os = EXCLUDED.os,
			   arch = EXCLUDED.arch,
			   environment = EXCLUDED.environment,
			   labels = EXCLUDED.labels,
			   capabilities = EXCLUDED.capabilities,
			   max_concurrent_tasks = EXCLUDED.max_concurrent_tasks,
			   version = EXCLUDED.version,
			   status = EXCLUDED.status,
			   last_heartbeat_at = EXCLUDED.last_heartbeat_at,
			   auth_token_hash = EXCLUDED.auth_token_hash,
			   cpu_millicores_available = EXCLUDED.cpu_millicores_available,
			   memory_bytes_available = EXCLUDED.memory_bytes_available,
			   disk_bytes_available = EXCLUDED.disk_bytes_available,
			   cpu_usage_percent = EXCLUDED.cpu_usage_percent,
			   memory_usage_percent = EXCLUDED.memory_usage_percent,
			   active_tasks = EXCLUDED.active_tasks,
			   resources_updated_at = EXCLUDED.resources_updated_at
			 RETURNING id, registered_at`,
			worker.ID, worker.Hostname, worker.OS, worker.Arch, worker.Environment,
			worker.Labels, worker.Capabilities, worker.MaxConcurrentTasks, worker.Version,
			worker.Status, worker.LastHeartbeatAt, worker.RegisteredAt, worker.AuthTokenHash,
			worker.ClusterID, worker.CPUMillicoresAvail, worker.MemoryBytesAvail, worker.DiskBytesAvail,
			worker.CPUUsagePercent, worker.MemoryUsagePercent, worker.ActiveTasks, worker.ResourcesUpdatedAt,
		).Scan(&worker.ID, &worker.RegisteredAt)
	}

	return r.pool.QueryRow(ctx,
		`INSERT INTO workers (id, hostname, os, arch, environment, labels, capabilities,
		 max_concurrent_tasks, version, status, last_heartbeat_at, registered_at, auth_token_hash,
		 cluster_id, cpu_millicores_available, memory_bytes_available, disk_bytes_available,
		 cpu_usage_percent, memory_usage_percent, active_tasks, resources_updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		 ON CONFLICT (hostname) WHERE cluster_id IS NULL
		 DO UPDATE SET
		   os = EXCLUDED.os,
		   arch = EXCLUDED.arch,
		   environment = EXCLUDED.environment,
		   labels = EXCLUDED.labels,
		   capabilities = EXCLUDED.capabilities,
		   max_concurrent_tasks = EXCLUDED.max_concurrent_tasks,
		   version = EXCLUDED.version,
		   status = EXCLUDED.status,
		   last_heartbeat_at = EXCLUDED.last_heartbeat_at,
		   auth_token_hash = EXCLUDED.auth_token_hash,
		   cpu_millicores_available = EXCLUDED.cpu_millicores_available,
		   memory_bytes_available = EXCLUDED.memory_bytes_available,
		   disk_bytes_available = EXCLUDED.disk_bytes_available,
		   cpu_usage_percent = EXCLUDED.cpu_usage_percent,
		   memory_usage_percent = EXCLUDED.memory_usage_percent,
		   active_tasks = EXCLUDED.active_tasks,
		   resources_updated_at = EXCLUDED.resources_updated_at
		 RETURNING id, registered_at`,
		worker.ID, worker.Hostname, worker.OS, worker.Arch, worker.Environment,
		worker.Labels, worker.Capabilities, worker.MaxConcurrentTasks, worker.Version,
		worker.Status, worker.LastHeartbeatAt, worker.RegisteredAt, worker.AuthTokenHash,
		worker.ClusterID, worker.CPUMillicoresAvail, worker.MemoryBytesAvail, worker.DiskBytesAvail,
		worker.CPUUsagePercent, worker.MemoryUsagePercent, worker.ActiveTasks, worker.ResourcesUpdatedAt,
	).Scan(&worker.ID, &worker.RegisteredAt)
}

var workerColumns = `id, hostname, os, arch, environment, labels, capabilities,
	max_concurrent_tasks, version, status, last_heartbeat_at, registered_at, auth_token_hash,
	cluster_id, cpu_millicores_available, memory_bytes_available, disk_bytes_available,
	cpu_usage_percent, memory_usage_percent, active_tasks, resources_updated_at`

func scanWorker(row pgx.Row) (*models.Worker, error) {
	var w models.Worker
	err := row.Scan(
		&w.ID, &w.Hostname, &w.OS, &w.Arch, &w.Environment, &w.Labels, &w.Capabilities,
		&w.MaxConcurrentTasks, &w.Version, &w.Status, &w.LastHeartbeatAt, &w.RegisteredAt,
		&w.AuthTokenHash, &w.ClusterID, &w.CPUMillicoresAvail, &w.MemoryBytesAvail,
		&w.DiskBytesAvail, &w.CPUUsagePercent, &w.MemoryUsagePercent, &w.ActiveTasks,
		&w.ResourcesUpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return &w, err
}

func scanWorkers(rows pgx.Rows) ([]models.Worker, error) {
	var workers []models.Worker
	for rows.Next() {
		var w models.Worker
		if err := rows.Scan(
			&w.ID, &w.Hostname, &w.OS, &w.Arch, &w.Environment, &w.Labels, &w.Capabilities,
			&w.MaxConcurrentTasks, &w.Version, &w.Status, &w.LastHeartbeatAt, &w.RegisteredAt,
			&w.AuthTokenHash, &w.ClusterID, &w.CPUMillicoresAvail, &w.MemoryBytesAvail,
			&w.DiskBytesAvail, &w.CPUUsagePercent, &w.MemoryUsagePercent, &w.ActiveTasks,
			&w.ResourcesUpdatedAt,
		); err != nil {
			return nil, err
		}
		workers = append(workers, w)
	}
	return workers, rows.Err()
}

func (r *ClusterRepo) GetWorker(ctx context.Context, id uuid.UUID) (*models.Worker, error) {
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM workers WHERE id = $1`, workerColumns), id)
	return scanWorker(row)
}

func (r *ClusterRepo) ListWorkers(ctx context.Context, clusterID *uuid.UUID, status *models.WorkerStatus) ([]models.Worker, error) {
	query := fmt.Sprintf(`SELECT %s FROM workers WHERE 1=1`, workerColumns)
	args := []any{}
	argIdx := 1

	if clusterID != nil {
		query += fmt.Sprintf(` AND cluster_id = $%d`, argIdx)
		args = append(args, *clusterID)
		argIdx++
	}
	if status != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *status)
	}
	query += ` ORDER BY hostname`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkers(rows)
}

func (r *ClusterRepo) UpdateWorkerHeartbeat(ctx context.Context, id uuid.UUID, res *WorkerResourceUpdate) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE workers SET last_heartbeat_at = $1, cpu_millicores_available = $2,
		 memory_bytes_available = $3, disk_bytes_available = $4, cpu_usage_percent = $5,
		 memory_usage_percent = $6, active_tasks = $7, resources_updated_at = $8,
		 status = 'online' WHERE id = $9`,
		now, res.CPUMillicoresAvail, res.MemoryBytesAvail, res.DiskBytesAvail,
		res.CPUUsagePercent, res.MemoryUsagePercent, res.ActiveTasks, now, id,
	)
	return err
}

func (r *ClusterRepo) UpdateWorkerStatus(ctx context.Context, id uuid.UUID, status models.WorkerStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE workers SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *ClusterRepo) DeleteWorker(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM workers WHERE id = $1`, id)
	return err
}

func (r *ClusterRepo) ListStaleWorkers(ctx context.Context, threshold time.Time) ([]models.Worker, error) {
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM workers WHERE status = 'online' AND last_heartbeat_at < $1`, workerColumns),
		threshold,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkers(rows)
}

// ── Task Assignments ──

func (r *ClusterRepo) CreateTaskAssignment(ctx context.Context, a *models.TaskAssignment) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO task_assignments (id, run_id, task_name, worker_id, cluster_id, status,
		 assigned_at, started_at, completed_at, result, retry_count, max_retries, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		a.ID, a.RunID, a.TaskName, a.WorkerID, a.ClusterID, a.Status,
		a.AssignedAt, a.StartedAt, a.CompletedAt, a.Result, a.RetryCount, a.MaxRetries, a.CreatedAt,
	)
	return err
}

func (r *ClusterRepo) GetTaskAssignment(ctx context.Context, id uuid.UUID) (*models.TaskAssignment, error) {
	var a models.TaskAssignment
	err := r.pool.QueryRow(ctx,
		`SELECT id, run_id, task_name, worker_id, cluster_id, status, assigned_at, started_at,
		 completed_at, result, retry_count, max_retries, created_at
		 FROM task_assignments WHERE id = $1`, id,
	).Scan(&a.ID, &a.RunID, &a.TaskName, &a.WorkerID, &a.ClusterID, &a.Status,
		&a.AssignedAt, &a.StartedAt, &a.CompletedAt, &a.Result, &a.RetryCount, &a.MaxRetries, &a.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return &a, err
}

func (r *ClusterRepo) ListTaskAssignmentsByRun(ctx context.Context, runID uuid.UUID) ([]models.TaskAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, run_id, task_name, worker_id, cluster_id, status, assigned_at, started_at,
		 completed_at, result, retry_count, max_retries, created_at
		 FROM task_assignments WHERE run_id = $1 ORDER BY created_at`, runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []models.TaskAssignment
	for rows.Next() {
		var a models.TaskAssignment
		if err := rows.Scan(&a.ID, &a.RunID, &a.TaskName, &a.WorkerID, &a.ClusterID, &a.Status,
			&a.AssignedAt, &a.StartedAt, &a.CompletedAt, &a.Result, &a.RetryCount, &a.MaxRetries, &a.CreatedAt); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

func (r *ClusterRepo) ListTaskAssignmentsByWorker(ctx context.Context, workerID uuid.UUID, limit int) ([]models.TaskAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, run_id, task_name, worker_id, cluster_id, status, assigned_at, started_at,
		 completed_at, result, retry_count, max_retries, created_at
		 FROM task_assignments WHERE worker_id = $1 ORDER BY created_at DESC LIMIT $2`, workerID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []models.TaskAssignment
	for rows.Next() {
		var a models.TaskAssignment
		if err := rows.Scan(&a.ID, &a.RunID, &a.TaskName, &a.WorkerID, &a.ClusterID, &a.Status,
			&a.AssignedAt, &a.StartedAt, &a.CompletedAt, &a.Result, &a.RetryCount, &a.MaxRetries, &a.CreatedAt); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

func (r *ClusterRepo) UpdateTaskAssignmentStatus(ctx context.Context, id uuid.UUID, status models.TaskAssignmentStatus, result json.RawMessage) error {
	now := time.Now()
	var completedAt *time.Time
	if status == models.TaskAssignmentCompleted || status == models.TaskAssignmentFailed {
		completedAt = &now
	}
	var startedAt *time.Time
	if status == models.TaskAssignmentRunning {
		startedAt = &now
	}

	query := `UPDATE task_assignments SET status = $1, result = $2, completed_at = COALESCE($3, completed_at)`
	args := []any{status, result, completedAt}
	if startedAt != nil {
		query += `, started_at = COALESCE(started_at, $4) WHERE id = $5`
		args = append(args, startedAt, id)
	} else {
		query += ` WHERE id = $4`
		args = append(args, id)
	}

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *ClusterRepo) ListOrphanedAssignments(ctx context.Context) ([]models.TaskAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ta.id, ta.run_id, ta.task_name, ta.worker_id, ta.cluster_id, ta.status,
		 ta.assigned_at, ta.started_at, ta.completed_at, ta.result, ta.retry_count, ta.max_retries, ta.created_at
		 FROM task_assignments ta
		 JOIN workers w ON ta.worker_id = w.id
		 WHERE ta.status IN ('assigned', 'running') AND w.status = 'offline'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []models.TaskAssignment
	for rows.Next() {
		var a models.TaskAssignment
		if err := rows.Scan(&a.ID, &a.RunID, &a.TaskName, &a.WorkerID, &a.ClusterID, &a.Status,
			&a.AssignedAt, &a.StartedAt, &a.CompletedAt, &a.Result, &a.RetryCount, &a.MaxRetries, &a.CreatedAt); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

func (r *ClusterRepo) IncrementAssignmentRetry(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE task_assignments SET retry_count = retry_count + 1, status = 'pending',
		 worker_id = NULL, assigned_at = NULL, started_at = NULL WHERE id = $1`, id,
	)
	return err
}
