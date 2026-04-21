package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

type ProjectEnvRepo struct {
	pool *pgxpool.Pool
}

func NewProjectEnvRepo(pool *pgxpool.Pool) ProjectEnvStore {
	return &ProjectEnvRepo{pool: pool}
}

func (r *ProjectEnvRepo) CreateSecretRecord(ctx context.Context, secret *models.SecretRecord) error {
	if secret.ID == uuid.Nil {
		secret.ID = uuid.New()
	}
	now := time.Now()
	secret.CreatedAt = now
	secret.UpdatedAt = now

	err := r.pool.QueryRow(ctx, `
		INSERT INTO secret_records (id, provider, ciphertext, key_ref, checksum, version, external_ref, status, created_at, updated_at, rotated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING created_at, updated_at
	`, secret.ID, secret.Provider, secret.Ciphertext, secret.KeyRef, secret.Checksum, secret.Version, secret.ExternalRef, secret.Status, secret.CreatedAt, secret.UpdatedAt, secret.RotatedAt).Scan(&secret.CreatedAt, &secret.UpdatedAt)
	if err != nil {
		return fmt.Errorf("project_env: create secret record: %w", err)
	}
	return nil
}

func (r *ProjectEnvRepo) GetSecretRecordByID(ctx context.Context, id uuid.UUID) (*models.SecretRecord, error) {
	var out models.SecretRecord
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider, ciphertext, key_ref, checksum, version, external_ref, status, created_at, updated_at, rotated_at
		FROM secret_records
		WHERE id = $1
	`, id).Scan(
		&out.ID, &out.Provider, &out.Ciphertext, &out.KeyRef, &out.Checksum, &out.Version, &out.ExternalRef,
		&out.Status, &out.CreatedAt, &out.UpdatedAt, &out.RotatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("project_env: get secret record: %w", err)
	}
	return &out, nil
}

func (r *ProjectEnvRepo) UpdateSecretRecord(ctx context.Context, secret *models.SecretRecord) error {
	secret.UpdatedAt = time.Now()
	tag, err := r.pool.Exec(ctx, `
		UPDATE secret_records
		SET provider = $1,
			ciphertext = $2,
			key_ref = $3,
			checksum = $4,
			version = $5,
			external_ref = $6,
			status = $7,
			updated_at = $8,
			rotated_at = $9
		WHERE id = $10
	`, secret.Provider, secret.Ciphertext, secret.KeyRef, secret.Checksum, secret.Version, secret.ExternalRef,
		secret.Status, secret.UpdatedAt, secret.RotatedAt, secret.ID)
	if err != nil {
		return fmt.Errorf("project_env: update secret record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProjectEnvRepo) RevokeSecretRecord(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE secret_records
		SET status = 'revoked', updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("project_env: revoke secret record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProjectEnvRepo) CreateEnvVar(ctx context.Context, envVar *models.ProjectEnvVar) error {
	if envVar.ID == uuid.Nil {
		envVar.ID = uuid.New()
	}
	now := time.Now()
	envVar.CreatedAt = now
	envVar.UpdatedAt = now

	err := r.pool.QueryRow(ctx, `
		INSERT INTO project_env_vars (
			id, project_id, key, value_type, plain_value, environment, branch, required, enabled,
			provider, provider_ref, secret_record_id, description, created_by, updated_by, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)
		RETURNING created_at, updated_at
	`, envVar.ID, envVar.ProjectID, envVar.Key, envVar.ValueType, envVar.PlainValue, envVar.Environment, envVar.Branch,
		envVar.Required, envVar.Enabled, envVar.Provider, envVar.ProviderRef, envVar.SecretRecordID, envVar.Description,
		envVar.CreatedBy, envVar.UpdatedBy, envVar.CreatedAt, envVar.UpdatedAt).Scan(&envVar.CreatedAt, &envVar.UpdatedAt)
	if err != nil {
		return fmt.Errorf("project_env: create env var: %w", err)
	}
	return nil
}

func (r *ProjectEnvRepo) UpdateEnvVar(ctx context.Context, envVar *models.ProjectEnvVar) error {
	envVar.UpdatedAt = time.Now()

	tag, err := r.pool.Exec(ctx, `
		UPDATE project_env_vars
		SET key = $1,
			value_type = $2,
			plain_value = $3,
			environment = $4,
			branch = $5,
			required = $6,
			enabled = $7,
			provider = $8,
			provider_ref = $9,
			secret_record_id = $10,
			description = $11,
			updated_by = $12,
			updated_at = $13
		WHERE id = $14 AND project_id = $15 AND deleted_at IS NULL
	`, envVar.Key, envVar.ValueType, envVar.PlainValue, envVar.Environment, envVar.Branch, envVar.Required, envVar.Enabled,
		envVar.Provider, envVar.ProviderRef, envVar.SecretRecordID, envVar.Description, envVar.UpdatedBy, envVar.UpdatedAt,
		envVar.ID, envVar.ProjectID)
	if err != nil {
		return fmt.Errorf("project_env: update env var: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProjectEnvRepo) SoftDeleteEnvVar(ctx context.Context, projectID, envVarID uuid.UUID, deletedBy *uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE project_env_vars
		SET deleted_at = NOW(), updated_at = NOW(), updated_by = $1
		WHERE id = $2 AND project_id = $3 AND deleted_at IS NULL
	`, deletedBy, envVarID, projectID)
	if err != nil {
		return fmt.Errorf("project_env: soft delete env var: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProjectEnvRepo) GetEnvVarByID(ctx context.Context, projectID, envVarID uuid.UUID) (*models.ProjectEnvVar, error) {
	var out models.ProjectEnvVar
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, key, value_type, plain_value, environment, branch, required, enabled,
			provider, provider_ref, secret_record_id, description, created_by, updated_by, created_at, updated_at, deleted_at
		FROM project_env_vars
		WHERE id = $1 AND project_id = $2
	`, envVarID, projectID).Scan(
		&out.ID, &out.ProjectID, &out.Key, &out.ValueType, &out.PlainValue, &out.Environment, &out.Branch, &out.Required,
		&out.Enabled, &out.Provider, &out.ProviderRef, &out.SecretRecordID, &out.Description, &out.CreatedBy, &out.UpdatedBy,
		&out.CreatedAt, &out.UpdatedAt, &out.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("project_env: get env var by id: %w", err)
	}
	return &out, nil
}

func (r *ProjectEnvRepo) GetEnvVarByScope(ctx context.Context, projectID uuid.UUID, key string, environment, branch *string) (*models.ProjectEnvVar, error) {
	var out models.ProjectEnvVar
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, key, value_type, plain_value, environment, branch, required, enabled,
			provider, provider_ref, secret_record_id, description, created_by, updated_by, created_at, updated_at, deleted_at
		FROM project_env_vars
		WHERE project_id = $1
		  AND key = $2
		  AND environment IS NOT DISTINCT FROM $3
		  AND branch IS NOT DISTINCT FROM $4
		  AND deleted_at IS NULL
		LIMIT 1
	`, projectID, key, environment, branch).Scan(
		&out.ID, &out.ProjectID, &out.Key, &out.ValueType, &out.PlainValue, &out.Environment, &out.Branch, &out.Required,
		&out.Enabled, &out.Provider, &out.ProviderRef, &out.SecretRecordID, &out.Description, &out.CreatedBy, &out.UpdatedBy,
		&out.CreatedAt, &out.UpdatedAt, &out.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("project_env: get env var by scope: %w", err)
	}
	return &out, nil
}

func (r *ProjectEnvRepo) ListEnvVars(ctx context.Context, filter ProjectEnvFilter) ([]models.ProjectEnvVar, error) {
	conditions := []string{"project_id = $1"}
	args := []any{filter.ProjectID}
	idx := 2

	if !filter.IncludeDeleted {
		conditions = append(conditions, "deleted_at IS NULL")
	}
	if filter.Environment != nil {
		conditions = append(conditions, fmt.Sprintf("environment IS NOT DISTINCT FROM $%d", idx))
		args = append(args, *filter.Environment)
		idx++
	}
	if filter.Branch != nil {
		conditions = append(conditions, fmt.Sprintf("branch IS NOT DISTINCT FROM $%d", idx))
		args = append(args, *filter.Branch)
		idx++
	}
	if filter.Key != nil {
		conditions = append(conditions, fmt.Sprintf("key = $%d", idx))
		args = append(args, *filter.Key)
		idx++
	}
	if filter.Provider != nil {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", idx))
		args = append(args, *filter.Provider)
		idx++
	}
	if filter.ValueType != nil {
		conditions = append(conditions, fmt.Sprintf("value_type = $%d", idx))
		args = append(args, *filter.ValueType)
		idx++
	}

	query := fmt.Sprintf(`
		SELECT id, project_id, key, value_type, plain_value, environment, branch, required, enabled,
			provider, provider_ref, secret_record_id, description, created_by, updated_by, created_at, updated_at, deleted_at
		FROM project_env_vars
		WHERE %s
		ORDER BY key ASC, branch DESC NULLS LAST, environment DESC NULLS LAST, updated_at DESC
	`, strings.Join(conditions, " AND "))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("project_env: list env vars: %w", err)
	}
	defer rows.Close()

	out := make([]models.ProjectEnvVar, 0)
	for rows.Next() {
		var item models.ProjectEnvVar
		if err := rows.Scan(
			&item.ID, &item.ProjectID, &item.Key, &item.ValueType, &item.PlainValue, &item.Environment, &item.Branch,
			&item.Required, &item.Enabled, &item.Provider, &item.ProviderRef, &item.SecretRecordID, &item.Description,
			&item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("project_env: scan env var: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *ProjectEnvRepo) ResolveEnv(ctx context.Context, params EnvResolutionParams) ([]ResolvedEnvVar, error) {
	rows, err := r.pool.Query(ctx, `
		WITH candidates AS (
			SELECT key, value_type, provider, environment, branch, required, provider_ref, secret_record_id, plain_value, updated_at,
				CASE WHEN branch = $3 THEN 0 ELSE 1 END AS branch_rank,
				CASE WHEN environment = $2 THEN 0 ELSE 1 END AS env_rank
			FROM project_env_vars
			WHERE project_id = $1
			  AND deleted_at IS NULL
			  AND enabled = true
			  AND (branch = $3 OR branch IS NULL)
			  AND (environment = $2 OR environment IS NULL)
		)
		SELECT DISTINCT ON (key)
			key, value_type, provider, environment, branch, required, provider_ref, secret_record_id, plain_value
		FROM candidates
		ORDER BY key, branch_rank, env_rank, updated_at DESC
	`, params.ProjectID, params.Environment, params.Branch)
	if err != nil {
		return nil, fmt.Errorf("project_env: resolve env: %w", err)
	}
	defer rows.Close()

	out := make([]ResolvedEnvVar, 0)
	for rows.Next() {
		var item ResolvedEnvVar
		if err := rows.Scan(
			&item.Key, &item.ValueType, &item.Provider, &item.Environment, &item.Branch,
			&item.Required, &item.ProviderRef, &item.SecretRecordID, &item.PlainValue,
		); err != nil {
			return nil, fmt.Errorf("project_env: scan resolved env: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *ProjectEnvRepo) CreateAuditEvent(ctx context.Context, event *models.EnvAuditEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if len(event.Metadata) == 0 {
		event.Metadata = []byte("{}")
	}

	err := r.pool.QueryRow(ctx, `
		INSERT INTO env_audit_events (id, project_id, env_var_id, actor_id, action, provider, environment, branch, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, NOW())
		RETURNING created_at
	`, event.ID, event.ProjectID, event.EnvVarID, event.ActorID, event.Action, event.Provider, event.Environment, event.Branch, event.Metadata).Scan(&event.CreatedAt)
	if err != nil {
		return fmt.Errorf("project_env: create audit event: %w", err)
	}
	return nil
}
