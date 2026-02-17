package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// ArtifactRepo handles artifact database operations.
type ArtifactRepo struct {
	pool *pgxpool.Pool
}

// NewArtifactRepo creates a new artifact repository.
func NewArtifactRepo(pool *pgxpool.Pool) *ArtifactRepo {
	return &ArtifactRepo{pool: pool}
}

// Create inserts a new artifact record.
func (r *ArtifactRepo) Create(ctx context.Context, artifact *models.Artifact) (*models.Artifact, error) {
	if artifact.ID == uuid.Nil {
		artifact.ID = uuid.New()
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now()
	}
	if artifact.Metadata == nil {
		artifact.Metadata = json.RawMessage(`{}`)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO artifacts (id, project_id, run_id, task_name, name, file_name, content_type,
			size_bytes, storage_key, digest_sha256, expires_at, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, artifact.ID, artifact.ProjectID, artifact.RunID, artifact.TaskName, artifact.Name, artifact.FileName,
		artifact.ContentType, artifact.SizeBytes, artifact.StorageKey, artifact.DigestSHA256, artifact.ExpiresAt,
		artifact.Metadata, artifact.CreatedAt)
	if err != nil {
		return nil, err
	}
	return artifact, nil
}

// GetByID returns an artifact by ID.
func (r *ArtifactRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Artifact, error) {
	var a models.Artifact
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, run_id, task_name, name, file_name, content_type,
		       size_bytes, storage_key, digest_sha256, expires_at, metadata, created_at
		FROM artifacts WHERE id = $1
	`, id).Scan(
		&a.ID, &a.ProjectID, &a.RunID, &a.TaskName, &a.Name, &a.FileName, &a.ContentType,
		&a.SizeBytes, &a.StorageKey, &a.DigestSHA256, &a.ExpiresAt, &a.Metadata, &a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// ListByRun lists artifacts for a run.
func (r *ArtifactRepo) ListByRun(ctx context.Context, runID uuid.UUID, limit, offset int) ([]*models.Artifact, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, run_id, task_name, name, file_name, content_type,
		       size_bytes, storage_key, digest_sha256, expires_at, metadata, created_at
		FROM artifacts
		WHERE run_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, runID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []*models.Artifact
	for rows.Next() {
		var a models.Artifact
		if err := rows.Scan(
			&a.ID, &a.ProjectID, &a.RunID, &a.TaskName, &a.Name, &a.FileName, &a.ContentType,
			&a.SizeBytes, &a.StorageKey, &a.DigestSHA256, &a.ExpiresAt, &a.Metadata, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, &a)
	}
	return artifacts, rows.Err()
}

// ListByRunAndTask lists artifacts for a run and task.
func (r *ArtifactRepo) ListByRunAndTask(ctx context.Context, runID uuid.UUID, taskName string, limit, offset int) ([]*models.Artifact, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, run_id, task_name, name, file_name, content_type,
		       size_bytes, storage_key, digest_sha256, expires_at, metadata, created_at
		FROM artifacts
		WHERE run_id = $1 AND task_name = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, runID, taskName, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []*models.Artifact
	for rows.Next() {
		var a models.Artifact
		if err := rows.Scan(
			&a.ID, &a.ProjectID, &a.RunID, &a.TaskName, &a.Name, &a.FileName, &a.ContentType,
			&a.SizeBytes, &a.StorageKey, &a.DigestSHA256, &a.ExpiresAt, &a.Metadata, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, &a)
	}
	return artifacts, rows.Err()
}

// Delete removes an artifact record.
func (r *ArtifactRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM artifacts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListExpired lists expired artifacts.
func (r *ArtifactRepo) ListExpired(ctx context.Context, limit int) ([]*models.Artifact, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, run_id, task_name, name, file_name, content_type,
		       size_bytes, storage_key, digest_sha256, expires_at, metadata, created_at
		FROM artifacts
		WHERE expires_at IS NOT NULL AND expires_at <= NOW()
		ORDER BY expires_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []*models.Artifact
	for rows.Next() {
		var a models.Artifact
		if err := rows.Scan(
			&a.ID, &a.ProjectID, &a.RunID, &a.TaskName, &a.Name, &a.FileName, &a.ContentType,
			&a.SizeBytes, &a.StorageKey, &a.DigestSHA256, &a.ExpiresAt, &a.Metadata, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, &a)
	}
	return artifacts, rows.Err()
}

// DeleteExpired removes expired artifacts and returns the count removed.
func (r *ArtifactRepo) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx, `DELETE FROM artifacts WHERE expires_at IS NOT NULL AND expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// DeleteOlderThanForProjects removes artifacts created before the given time for the specified projects.
// Returns the number of deleted rows and the storage keys of the deleted artifacts.
func (r *ArtifactRepo) DeleteOlderThanForProjects(ctx context.Context, projectIDs []uuid.UUID, before time.Time) (int64, []string, error) {
	if len(projectIDs) == 0 {
		return 0, nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		DELETE FROM artifacts
		WHERE project_id = ANY($1) AND created_at < $2
		RETURNING storage_key
	`, projectIDs, before)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return 0, nil, err
		}
		keys = append(keys, key)
	}
	return int64(len(keys)), keys, rows.Err()
}

// TotalSizeByProject returns total size of artifacts for a project.
func (r *ArtifactRepo) TotalSizeByProject(ctx context.Context, projectID uuid.UUID) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM artifacts
		WHERE project_id = $1
	`, projectID).Scan(&total)
	return total, err
}
