package repo

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/internal/db/models"
)

// WorkflowRepo handles workflow database operations.
type WorkflowRepo struct {
	pool *pgxpool.Pool
}

// NewWorkflowRepo creates a new workflow repository.
func NewWorkflowRepo(pool *pgxpool.Pool) *WorkflowRepo {
	return &WorkflowRepo{pool: pool}
}

// Upsert creates or updates a workflow and returns whether it changed.
func (r *WorkflowRepo) Upsert(ctx context.Context, workflow *models.ProjectWorkflow) (bool, error) {
	query := `
		INSERT INTO project_workflows (project_id, name, version, is_default, config_hash, raw_config, synced_at)
		VALUES ($1, $2, 1, $3, $4, $5, NOW())
		ON CONFLICT (project_id, name) DO UPDATE SET
			version = CASE
				WHEN project_workflows.config_hash IS DISTINCT FROM EXCLUDED.config_hash
				THEN project_workflows.version + 1
				ELSE project_workflows.version
			END,
			is_default = EXCLUDED.is_default,
			config_hash = EXCLUDED.config_hash,
			raw_config = EXCLUDED.raw_config,
			synced_at = NOW()
		RETURNING id, version, synced_at, (xmax = 0) AS inserted`

	var inserted bool
	err := r.pool.QueryRow(ctx, query,
		workflow.ProjectID,
		workflow.Name,
		workflow.IsDefault,
		workflow.ConfigHash,
		workflow.RawConfig,
	).Scan(&workflow.ID, &workflow.Version, &workflow.SyncedAt, &inserted)
	if err != nil {
		return false, err
	}

	return !inserted, nil // returns true if it was an update (changed)
}

// UpsertTasks replaces all tasks for a workflow.
func (r *WorkflowRepo) UpsertTasks(ctx context.Context, workflowID uuid.UUID, tasks []models.WorkflowTask) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Delete existing tasks
	_, err = tx.Exec(ctx, "DELETE FROM workflow_tasks WHERE workflow_id = $1", workflowID)
	if err != nil {
		return err
	}

	// Insert new tasks
	for _, task := range tasks {
		envJSON, err := json.Marshal(task.Env)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO workflow_tasks (workflow_id, name, command, needs, inputs, outputs, plugins, timeout_seconds, workdir, env, group_name, condition_expr)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			workflowID,
			task.Name,
			task.Command,
			task.Needs,
			task.Inputs,
			task.Outputs,
			task.Plugins,
			task.TimeoutSeconds,
			task.Workdir,
			envJSON,
			task.GroupName,
			task.ConditionExpr,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetByID retrieves a workflow by ID with its tasks.
func (r *WorkflowRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.WorkflowWithTasks, error) {
	workflow := &models.WorkflowWithTasks{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, name, version, is_default, config_hash, raw_config, synced_at
		FROM project_workflows WHERE id = $1`, id,
	).Scan(
		&workflow.ID,
		&workflow.ProjectID,
		&workflow.Name,
		&workflow.Version,
		&workflow.IsDefault,
		&workflow.ConfigHash,
		&workflow.RawConfig,
		&workflow.SyncedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	tasks, err := r.getTasksByWorkflowID(ctx, id)
	if err != nil {
		return nil, err
	}
	workflow.Tasks = tasks

	return workflow, nil
}

// GetByProjectAndName retrieves a workflow by project and name.
func (r *WorkflowRepo) GetByProjectAndName(ctx context.Context, projectID uuid.UUID, name string) (*models.WorkflowWithTasks, error) {
	workflow := &models.WorkflowWithTasks{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, name, version, is_default, config_hash, raw_config, synced_at
		FROM project_workflows WHERE project_id = $1 AND name = $2`, projectID, name,
	).Scan(
		&workflow.ID,
		&workflow.ProjectID,
		&workflow.Name,
		&workflow.Version,
		&workflow.IsDefault,
		&workflow.ConfigHash,
		&workflow.RawConfig,
		&workflow.SyncedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	tasks, err := r.getTasksByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return nil, err
	}
	workflow.Tasks = tasks

	return workflow, nil
}

// ListByProject returns all workflows for a project.
func (r *WorkflowRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ProjectWorkflow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, name, version, is_default, config_hash, synced_at
		FROM project_workflows WHERE project_id = $1
		ORDER BY is_default DESC, name ASC`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []models.ProjectWorkflow
	for rows.Next() {
		var w models.ProjectWorkflow
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.Name, &w.Version, &w.IsDefault, &w.ConfigHash, &w.SyncedAt); err != nil {
			return nil, err
		}
		workflows = append(workflows, w)
	}

	return workflows, rows.Err()
}

// ListByProjectWithTasks returns all workflows with their tasks for a project.
func (r *WorkflowRepo) ListByProjectWithTasks(ctx context.Context, projectID uuid.UUID) ([]models.WorkflowWithTasks, error) {
	workflows, err := r.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	result := make([]models.WorkflowWithTasks, len(workflows))
	for i, w := range workflows {
		tasks, err := r.getTasksByWorkflowID(ctx, w.ID)
		if err != nil {
			return nil, err
		}
		result[i] = models.WorkflowWithTasks{
			ProjectWorkflow: w,
			Tasks:           tasks,
		}
	}

	return result, nil
}

// getTasksByWorkflowID retrieves all tasks for a workflow.
func (r *WorkflowRepo) getTasksByWorkflowID(ctx context.Context, workflowID uuid.UUID) ([]models.WorkflowTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workflow_id, name, command, needs, inputs, outputs, plugins, timeout_seconds, workdir, env, group_name, condition_expr
		FROM workflow_tasks WHERE workflow_id = $1
		ORDER BY name ASC`, workflowID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.WorkflowTask
	for rows.Next() {
		var t models.WorkflowTask
		var envJSON []byte
		if err := rows.Scan(
			&t.ID, &t.WorkflowID, &t.Name, &t.Command,
			&t.Needs, &t.Inputs, &t.Outputs, &t.Plugins,
			&t.TimeoutSeconds, &t.Workdir, &envJSON,
			&t.GroupName, &t.ConditionExpr,
		); err != nil {
			return nil, err
		}
		if len(envJSON) > 0 {
			_ = json.Unmarshal(envJSON, &t.Env)
		}
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// GetDefaultByProject retrieves the default workflow for a project.
// Returns nil, nil when no default workflow exists.
func (r *WorkflowRepo) GetDefaultByProject(ctx context.Context, projectID uuid.UUID) (*models.ProjectWorkflow, error) {
	var w models.ProjectWorkflow
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, name, version, is_default, config_hash, synced_at
		FROM project_workflows WHERE project_id = $1 AND is_default = true LIMIT 1`, projectID,
	).Scan(&w.ID, &w.ProjectID, &w.Name, &w.Version, &w.IsDefault, &w.ConfigHash, &w.SyncedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

// Delete removes a workflow and its tasks.
func (r *WorkflowRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM project_workflows WHERE id = $1", id)
	return err
}
