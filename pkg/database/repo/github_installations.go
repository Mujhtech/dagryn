package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// GitHubInstallationRepo handles GitHub App installation database operations.
type GitHubInstallationRepo struct {
	pool *pgxpool.Pool
}

// NewGitHubInstallationRepo creates a new GitHubInstallationRepo.
func NewGitHubInstallationRepo(pool *pgxpool.Pool) *GitHubInstallationRepo {
	return &GitHubInstallationRepo{pool: pool}
}

// UpsertByInstallationID creates or updates an installation row based on installation_id.
func (r *GitHubInstallationRepo) UpsertByInstallationID(ctx context.Context, inst *models.GitHubInstallation) error {
	if inst.ID == uuid.Nil {
		inst.ID = uuid.New()
	}
	now := time.Now()
	inst.UpdatedAt = now
	if inst.CreatedAt.IsZero() {
		inst.CreatedAt = now
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO github_installations (id, installation_id, account_login, account_type, account_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (installation_id) DO UPDATE
		SET account_login = EXCLUDED.account_login,
		    account_type = EXCLUDED.account_type,
		    account_id = EXCLUDED.account_id,
		    updated_at = EXCLUDED.updated_at
	`, inst.ID, inst.InstallationID, inst.AccountLogin, inst.AccountType, inst.AccountID, inst.CreatedAt, inst.UpdatedAt)
	return err
}

// GetByInstallationID retrieves an installation by its GitHub installation ID.
func (r *GitHubInstallationRepo) GetByInstallationID(ctx context.Context, installationID int64) (*models.GitHubInstallation, error) {
	var inst models.GitHubInstallation
	err := r.pool.QueryRow(ctx, `
		SELECT id, installation_id, account_login, account_type, account_id, created_at, updated_at
		FROM github_installations
		WHERE installation_id = $1
	`, installationID).Scan(&inst.ID, &inst.InstallationID, &inst.AccountLogin, &inst.AccountType, &inst.AccountID, &inst.CreatedAt, &inst.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &inst, nil
}

// GetByID retrieves an installation by its Dagryn UUID.
func (r *GitHubInstallationRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.GitHubInstallation, error) {
	var inst models.GitHubInstallation
	err := r.pool.QueryRow(ctx, `
		SELECT id, installation_id, account_login, account_type, account_id, created_at, updated_at
		FROM github_installations
		WHERE id = $1
	`, id).Scan(&inst.ID, &inst.InstallationID, &inst.AccountLogin, &inst.AccountType, &inst.AccountID, &inst.CreatedAt, &inst.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &inst, nil
}

// ListAll retrieves all GitHub App installations.
func (r *GitHubInstallationRepo) ListAll(ctx context.Context) ([]models.GitHubInstallation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, installation_id, account_login, account_type, account_id, created_at, updated_at
		FROM github_installations
		ORDER BY account_login, account_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var installations []models.GitHubInstallation
	for rows.Next() {
		var inst models.GitHubInstallation
		if err := rows.Scan(&inst.ID, &inst.InstallationID, &inst.AccountLogin, &inst.AccountType, &inst.AccountID, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, err
		}
		installations = append(installations, inst)
	}
	return installations, rows.Err()
}
