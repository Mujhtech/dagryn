package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// ProjectRepo handles project database operations.
type ProjectRepo struct {
	pool *pgxpool.Pool
}

// NewProjectRepo creates a new project repository.
func NewProjectRepo(pool *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{pool: pool}
}

// Create creates a new project and adds the creator as owner.
func (r *ProjectRepo) Create(ctx context.Context, project *models.Project, ownerID uuid.UUID) error {
	project.ID = uuid.New()
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Create project
	_, err = tx.Exec(ctx, `
		INSERT INTO projects (id, team_id, name, slug, path_hash, repo_url, repo_linked_by_user_id, github_installation_id, github_repo_id, description, visibility, config_path, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, project.ID, project.TeamID, project.Name, project.Slug, project.PathHash, project.RepoURL, project.RepoLinkedByUserID, project.GitHubInstallationID, project.GitHubRepoID, project.Description,
		project.Visibility, project.ConfigPath, project.CreatedAt, project.UpdatedAt)
	if err != nil {
		return err
	}

	// Add owner as project member
	_, err = tx.Exec(ctx, `
		INSERT INTO project_members (id, project_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), project.ID, ownerID, models.RoleOwner, project.CreatedAt)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetByID retrieves a project by ID.
func (r *ProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	var project models.Project
	err := r.pool.QueryRow(ctx, `
		SELECT id, team_id, name, slug, path_hash, repo_url, repo_linked_by_user_id, github_installation_id, github_repo_id, billing_account_id, description, visibility, config_path, created_at, updated_at, last_run_at
		FROM projects WHERE id = $1
	`, id).Scan(&project.ID, &project.TeamID, &project.Name, &project.Slug, &project.PathHash, &project.RepoURL, &project.RepoLinkedByUserID, &project.GitHubInstallationID, &project.GitHubRepoID,
		&project.BillingAccountID, &project.Description, &project.Visibility, &project.ConfigPath, &project.CreatedAt, &project.UpdatedAt, &project.LastRunAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &project, nil
}

// GetByRepoURL retrieves a project by its exact repo_url (used for provider webhooks).
func (r *ProjectRepo) GetByRepoURL(ctx context.Context, repoURL string) (*models.Project, error) {
	var project models.Project
	err := r.pool.QueryRow(ctx, `
		SELECT id, team_id, name, slug, path_hash, repo_url, repo_linked_by_user_id, github_installation_id, github_repo_id, billing_account_id, description, visibility, config_path, created_at, updated_at, last_run_at
		FROM projects WHERE repo_url = $1
	`, repoURL).Scan(&project.ID, &project.TeamID, &project.Name, &project.Slug, &project.PathHash, &project.RepoURL, &project.RepoLinkedByUserID, &project.GitHubInstallationID, &project.GitHubRepoID,
		&project.BillingAccountID, &project.Description, &project.Visibility, &project.ConfigPath, &project.CreatedAt, &project.UpdatedAt, &project.LastRunAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &project, nil
}

// GetByGitHubRepoID retrieves a project by GitHub installation ID and repository ID.
func (r *ProjectRepo) GetByGitHubRepoID(ctx context.Context, installationID uuid.UUID, repoID int64) (*models.Project, error) {
	var project models.Project
	err := r.pool.QueryRow(ctx, `
		SELECT id, team_id, name, slug, path_hash, repo_url, repo_linked_by_user_id, github_installation_id, github_repo_id, billing_account_id, description, visibility, config_path, created_at, updated_at, last_run_at
		FROM projects WHERE github_installation_id = $1 AND github_repo_id = $2
	`, installationID, repoID).Scan(&project.ID, &project.TeamID, &project.Name, &project.Slug, &project.PathHash, &project.RepoURL, &project.RepoLinkedByUserID, &project.GitHubInstallationID, &project.GitHubRepoID,
		&project.BillingAccountID, &project.Description, &project.Visibility, &project.ConfigPath, &project.CreatedAt, &project.UpdatedAt, &project.LastRunAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &project, nil
}

// GetBySlug retrieves a project by team and slug.
func (r *ProjectRepo) GetBySlug(ctx context.Context, teamID *uuid.UUID, slug string) (*models.Project, error) {
	var project models.Project
	var err error

	if teamID != nil {
		err = r.pool.QueryRow(ctx, `
			SELECT id, team_id, name, slug, path_hash, repo_url, repo_linked_by_user_id, github_installation_id, github_repo_id, billing_account_id, description, visibility, config_path, created_at, updated_at, last_run_at
			FROM projects WHERE team_id = $1 AND slug = $2
		`, teamID, slug).Scan(&project.ID, &project.TeamID, &project.Name, &project.Slug, &project.PathHash, &project.RepoURL, &project.RepoLinkedByUserID, &project.GitHubInstallationID, &project.GitHubRepoID,
			&project.BillingAccountID, &project.Description, &project.Visibility, &project.ConfigPath, &project.CreatedAt, &project.UpdatedAt, &project.LastRunAt)
	} else {
		err = r.pool.QueryRow(ctx, `
			SELECT id, team_id, name, slug, path_hash, repo_url, repo_linked_by_user_id, github_installation_id, github_repo_id, billing_account_id, description, visibility, config_path, created_at, updated_at, last_run_at
			FROM projects WHERE team_id IS NULL AND slug = $1
		`, slug).Scan(&project.ID, &project.TeamID, &project.Name, &project.Slug, &project.PathHash, &project.RepoURL, &project.RepoLinkedByUserID, &project.GitHubInstallationID, &project.GitHubRepoID,
			&project.BillingAccountID, &project.Description, &project.Visibility, &project.ConfigPath, &project.CreatedAt, &project.UpdatedAt, &project.LastRunAt)
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &project, nil
}

// GetByPathHash retrieves a project by path hash (for local linking).
func (r *ProjectRepo) GetByPathHash(ctx context.Context, userID uuid.UUID, pathHash string) (*models.Project, error) {
	var project models.Project
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.team_id, p.name, p.slug, p.path_hash, p.repo_url, p.repo_linked_by_user_id, p.github_installation_id, p.github_repo_id, p.billing_account_id, p.description, p.visibility, p.config_path, p.created_at, p.updated_at, p.last_run_at
		FROM projects p
		JOIN project_members pm ON p.id = pm.project_id
		WHERE pm.user_id = $1 AND p.path_hash = $2
	`, userID, pathHash).Scan(&project.ID, &project.TeamID, &project.Name, &project.Slug, &project.PathHash, &project.RepoURL, &project.RepoLinkedByUserID, &project.GitHubInstallationID, &project.GitHubRepoID,
		&project.BillingAccountID, &project.Description, &project.Visibility, &project.ConfigPath, &project.CreatedAt, &project.UpdatedAt, &project.LastRunAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &project, nil
}

// Update updates a project.
func (r *ProjectRepo) Update(ctx context.Context, project *models.Project) error {
	project.UpdatedAt = time.Now()

	result, err := r.pool.Exec(ctx, `
		UPDATE projects SET name = $1, slug = $2, description = $3, visibility = $4, config_path = $5, repo_url = $6, repo_linked_by_user_id = $7, github_installation_id = $8, github_repo_id = $9, updated_at = $10
		WHERE id = $11
	`, project.Name, project.Slug, project.Description, project.Visibility, project.ConfigPath, project.RepoURL, project.RepoLinkedByUserID, project.GitHubInstallationID, project.GitHubRepoID, project.UpdatedAt, project.ID)

	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLastRunAt updates the last_run_at timestamp.
func (r *ProjectRepo) UpdateLastRunAt(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE projects SET last_run_at = NOW() WHERE id = $1`, id)
	return err
}

// UpdateBillingAccountID sets the billing account for a project.
func (r *ProjectRepo) UpdateBillingAccountID(ctx context.Context, projectID, billingAccountID uuid.UUID) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE projects SET billing_account_id = $1, updated_at = NOW() WHERE id = $2`,
		billingAccountID, projectID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete deletes a project.
func (r *ProjectRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM projects WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser returns all projects a user has access to.
func (r *ProjectRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.ProjectWithMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.team_id, p.name, p.slug, p.path_hash, p.repo_url, p.repo_linked_by_user_id, p.github_installation_id, p.github_repo_id, p.billing_account_id, p.description, p.visibility, p.config_path,
		       p.created_at, p.updated_at, p.last_run_at, pm.role, pm.joined_at
		FROM projects p
		JOIN project_members pm ON p.id = pm.project_id
		WHERE pm.user_id = $1
		ORDER BY p.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.ProjectWithMember
	for rows.Next() {
		var p models.ProjectWithMember
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.Slug, &p.PathHash, &p.RepoURL, &p.RepoLinkedByUserID, &p.GitHubInstallationID, &p.GitHubRepoID, &p.BillingAccountID, &p.Description, &p.Visibility,
			&p.ConfigPath, &p.CreatedAt, &p.UpdatedAt, &p.LastRunAt, &p.Role, &p.JoinedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ListByTeam returns all projects in a team.
func (r *ProjectRepo) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]models.Project, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, team_id, name, slug, path_hash, repo_url, repo_linked_by_user_id, github_installation_id, github_repo_id, billing_account_id, description, visibility, config_path, created_at, updated_at, last_run_at
		FROM projects WHERE team_id = $1
		ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.Slug, &p.PathHash, &p.RepoURL, &p.RepoLinkedByUserID, &p.GitHubInstallationID, &p.GitHubRepoID, &p.BillingAccountID, &p.Description, &p.Visibility,
			&p.ConfigPath, &p.CreatedAt, &p.UpdatedAt, &p.LastRunAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// AddMember adds a user to a project.
func (r *ProjectRepo) AddMember(ctx context.Context, projectID, userID uuid.UUID, role models.Role, invitedBy *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO project_members (id, project_id, user_id, role, invited_by, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), projectID, userID, role, invitedBy, time.Now())
	return err
}

// UpdateMemberRole updates a member's role.
func (r *ProjectRepo) UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.Role) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE project_members SET role = $1 WHERE project_id = $2 AND user_id = $3
	`, role, projectID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveMember removes a user from a project.
func (r *ProjectRepo) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM project_members WHERE project_id = $1 AND user_id = $2
	`, projectID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetMember retrieves a project member.
func (r *ProjectRepo) GetMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error) {
	var member models.ProjectMember
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, user_id, role, invited_by, joined_at
		FROM project_members WHERE project_id = $1 AND user_id = $2
	`, projectID, userID).Scan(&member.ID, &member.ProjectID, &member.UserID, &member.Role, &member.InvitedBy, &member.JoinedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &member, nil
}

// ListMembers returns all members of a project.
func (r *ProjectRepo) ListMembers(ctx context.Context, projectID uuid.UUID) ([]models.ProjectMemberWithUser, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pm.id, pm.project_id, pm.user_id, pm.role, pm.invited_by, pm.joined_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.provider_id, u.created_at, u.updated_at
		FROM project_members pm
		JOIN users u ON pm.user_id = u.id
		WHERE pm.project_id = $1
		ORDER BY pm.joined_at
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.ProjectMemberWithUser
	for rows.Next() {
		var m models.ProjectMemberWithUser
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Role, &m.InvitedBy, &m.JoinedAt,
			&m.User.ID, &m.User.Email, &m.User.Name, &m.User.AvatarURL, &m.User.Provider, &m.User.ProviderID, &m.User.CreatedAt, &m.User.UpdatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// GetUserRole gets a user's effective role for a project (considering team membership).
func (r *ProjectRepo) GetUserRole(ctx context.Context, projectID, userID uuid.UUID) (models.Role, error) {
	// First check direct project membership
	var role models.Role
	err := r.pool.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2
	`, projectID, userID).Scan(&role)

	if err == nil {
		return role, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	// Check team membership if project belongs to a team
	err = r.pool.QueryRow(ctx, `
		SELECT tm.role FROM team_members tm
		JOIN projects p ON p.team_id = tm.team_id
		WHERE p.id = $1 AND tm.user_id = $2
	`, projectID, userID).Scan(&role)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return role, nil
}

// SlugExists checks if a project slug already exists within a team.
func (r *ProjectRepo) SlugExists(ctx context.Context, teamID *uuid.UUID, slug string) (bool, error) {
	var exists bool
	var err error

	if teamID != nil {
		err = r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM projects WHERE team_id = $1 AND slug = $2)", teamID, slug).Scan(&exists)
	} else {
		err = r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM projects WHERE team_id IS NULL AND slug = $1)", slug).Scan(&exists)
	}
	return exists, err
}

// ListPublic returns all public projects with pagination.
func (r *ProjectRepo) ListPublic(ctx context.Context, limit, offset int) ([]models.Project, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, team_id, name, slug, path_hash, repo_url, description, visibility, config_path, created_at, updated_at, last_run_at
		FROM projects 
		WHERE visibility = 'public'
		ORDER BY updated_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.Slug, &p.PathHash, &p.RepoURL, &p.Description, &p.Visibility,
			&p.ConfigPath, &p.CreatedAt, &p.UpdatedAt, &p.LastRunAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ListAll returns all projects with pagination (for admin dashboard).
func (r *ProjectRepo) ListAll(ctx context.Context, limit, offset int) ([]models.Project, int, error) {
	// Get total count
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM projects").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, team_id, name, slug, path_hash, repo_url, description, visibility, config_path, created_at, updated_at, last_run_at
		FROM projects 
		ORDER BY updated_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.Slug, &p.PathHash, &p.RepoURL, &p.Description, &p.Visibility,
			&p.ConfigPath, &p.CreatedAt, &p.UpdatedAt, &p.LastRunAt); err != nil {
			return nil, 0, err
		}
		projects = append(projects, p)
	}
	return projects, total, rows.Err()
}
