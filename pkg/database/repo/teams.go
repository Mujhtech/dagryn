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

// TeamRepo handles team database operations.
type TeamRepo struct {
	pool *pgxpool.Pool
}

// NewTeamRepo creates a new team repository.
func NewTeamRepo(pool *pgxpool.Pool) TeamStore {
	return &TeamRepo{pool: pool}
}

// Create creates a new team and adds the owner as a member.
func (r *TeamRepo) Create(ctx context.Context, team *models.Team) error {
	team.ID = uuid.New()
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Create team
	_, err = tx.Exec(ctx, `
		INSERT INTO teams (id, name, slug, owner_id, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, team.ID, team.Name, team.Slug, team.OwnerID, team.Description, team.CreatedAt, team.UpdatedAt)
	if err != nil {
		return err
	}

	// Add owner as team member with 'owner' role
	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (id, team_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), team.ID, team.OwnerID, models.RoleOwner, team.CreatedAt)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetByID retrieves a team by ID.
func (r *TeamRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	var team models.Team
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, slug, owner_id, description, created_at, updated_at
		FROM teams WHERE id = $1
	`, id).Scan(&team.ID, &team.Name, &team.Slug, &team.OwnerID, &team.Description, &team.CreatedAt, &team.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &team, nil
}

// GetBySlug retrieves a team by slug.
func (r *TeamRepo) GetBySlug(ctx context.Context, slug string) (*models.Team, error) {
	var team models.Team
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, slug, owner_id, description, created_at, updated_at
		FROM teams WHERE slug = $1
	`, slug).Scan(&team.ID, &team.Name, &team.Slug, &team.OwnerID, &team.Description, &team.CreatedAt, &team.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &team, nil
}

// Update updates a team.
func (r *TeamRepo) Update(ctx context.Context, team *models.Team) error {
	team.UpdatedAt = time.Now()

	result, err := r.pool.Exec(ctx, `
		UPDATE teams SET name = $1, slug = $2, description = $3, updated_at = $4
		WHERE id = $5
	`, team.Name, team.Slug, team.Description, team.UpdatedAt, team.ID)

	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete deletes a team.
func (r *TeamRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM teams WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser returns all teams a user is a member of.
func (r *TeamRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.TeamWithMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.slug, t.owner_id, t.description, t.created_at, t.updated_at,
		       tm.role, tm.joined_at
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.TeamWithMember
	for rows.Next() {
		var team models.TeamWithMember
		if err := rows.Scan(&team.ID, &team.Name, &team.Slug, &team.OwnerID, &team.Description,
			&team.CreatedAt, &team.UpdatedAt, &team.Role, &team.JoinedAt); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

// AddMember adds a user to a team.
func (r *TeamRepo) AddMember(ctx context.Context, teamID, userID uuid.UUID, role models.Role, invitedBy *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO team_members (id, team_id, user_id, role, invited_by, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), teamID, userID, role, invitedBy, time.Now())
	return err
}

// UpdateMemberRole updates a member's role.
func (r *TeamRepo) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.Role) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE team_members SET role = $1 WHERE team_id = $2 AND user_id = $3
	`, role, teamID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveMember removes a user from a team.
func (r *TeamRepo) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM team_members WHERE team_id = $1 AND user_id = $2
	`, teamID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetMember retrieves a team member.
func (r *TeamRepo) GetMember(ctx context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error) {
	var member models.TeamMember
	err := r.pool.QueryRow(ctx, `
		SELECT id, team_id, user_id, role, invited_by, joined_at
		FROM team_members WHERE team_id = $1 AND user_id = $2
	`, teamID, userID).Scan(&member.ID, &member.TeamID, &member.UserID, &member.Role, &member.InvitedBy, &member.JoinedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &member, nil
}

// ListMembers returns all members of a team.
func (r *TeamRepo) ListMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMemberWithUser, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.invited_by, tm.joined_at,
		       u.id, u.email, u.name, u.avatar_url, u.provider, u.provider_id, u.created_at, u.updated_at
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = $1
		ORDER BY tm.joined_at
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.TeamMemberWithUser
	for rows.Next() {
		var m models.TeamMemberWithUser
		if err := rows.Scan(&m.ID, &m.TeamID, &m.UserID, &m.Role, &m.InvitedBy, &m.JoinedAt,
			&m.User.ID, &m.User.Email, &m.User.Name, &m.User.AvatarURL, &m.User.Provider, &m.User.ProviderID, &m.User.CreatedAt, &m.User.UpdatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// SlugExists checks if a team slug already exists.
func (r *TeamRepo) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM teams WHERE slug = $1)", slug).Scan(&exists)
	return exists, err
}
