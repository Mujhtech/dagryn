package repo

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// InvitationRepo handles invitation database operations.
type InvitationRepo struct {
	pool *pgxpool.Pool
}

// NewInvitationRepo creates a new invitation repository.
func NewInvitationRepo(pool *pgxpool.Pool) *InvitationRepo {
	return &InvitationRepo{pool: pool}
}

// Create creates a new invitation.
func (r *InvitationRepo) Create(ctx context.Context, inv *models.Invitation) error {
	inv.ID = uuid.New()
	inv.CreatedAt = time.Now()
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = time.Now().Add(models.DefaultInvitationExpiry)
	}

	// Generate secure token
	token, err := generateInviteToken()
	if err != nil {
		return err
	}
	inv.Token = token

	_, err = r.pool.Exec(ctx, `
		INSERT INTO invitations (id, email, team_id, project_id, role, invited_by, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, inv.ID, inv.Email, inv.TeamID, inv.ProjectID, inv.Role, inv.InvitedBy, inv.Token, inv.ExpiresAt, inv.CreatedAt)

	return err
}

// GetByID retrieves an invitation by ID.
func (r *InvitationRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Invitation, error) {
	var inv models.Invitation
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, team_id, project_id, role, invited_by, token, expires_at, accepted_at, created_at
		FROM invitations WHERE id = $1
	`, id).Scan(&inv.ID, &inv.Email, &inv.TeamID, &inv.ProjectID, &inv.Role, &inv.InvitedBy,
		&inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &inv, nil
}

// GetByToken retrieves an invitation by token.
func (r *InvitationRepo) GetByToken(ctx context.Context, token string) (*models.Invitation, error) {
	var inv models.Invitation
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, team_id, project_id, role, invited_by, token, expires_at, accepted_at, created_at
		FROM invitations WHERE token = $1
	`, token).Scan(&inv.ID, &inv.Email, &inv.TeamID, &inv.ProjectID, &inv.Role, &inv.InvitedBy,
		&inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &inv, nil
}

// GetPendingByToken retrieves a pending (not accepted, not expired) invitation by token.
func (r *InvitationRepo) GetPendingByToken(ctx context.Context, token string) (*models.InvitationWithDetails, error) {
	var inv models.InvitationWithDetails
	err := r.pool.QueryRow(ctx, `
		SELECT i.id, i.email, i.team_id, i.project_id, i.role, i.invited_by, i.token, i.expires_at, i.accepted_at, i.created_at,
		       t.name as team_name, t.slug as team_slug,
		       p.name as project_name, p.slug as project_slug,
		       u.name as inviter_name, u.email as inviter_email
		FROM invitations i
		LEFT JOIN teams t ON i.team_id = t.id
		LEFT JOIN projects p ON i.project_id = p.id
		JOIN users u ON i.invited_by = u.id
		WHERE i.token = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
	`, token).Scan(&inv.ID, &inv.Email, &inv.TeamID, &inv.ProjectID, &inv.Role, &inv.InvitedBy,
		&inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
		&inv.TeamName, &inv.TeamSlug, &inv.ProjectName, &inv.ProjectSlug,
		&inv.InviterName, &inv.InviterEmail)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &inv, nil
}

// Accept marks an invitation as accepted.
func (r *InvitationRepo) Accept(ctx context.Context, token string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE invitations SET accepted_at = NOW()
		WHERE token = $1 AND accepted_at IS NULL AND expires_at > NOW()
	`, token)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete deletes an invitation.
func (r *InvitationRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM invitations WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByToken deletes an invitation by token.
func (r *InvitationRepo) DeleteByToken(ctx context.Context, token string) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM invitations WHERE token = $1", token)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPendingByEmail returns all pending invitations for an email.
func (r *InvitationRepo) ListPendingByEmail(ctx context.Context, email string) ([]models.InvitationWithDetails, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.email, i.team_id, i.project_id, i.role, i.invited_by, i.token, i.expires_at, i.accepted_at, i.created_at,
		       t.name as team_name, t.slug as team_slug,
		       p.name as project_name, p.slug as project_slug,
		       u.name as inviter_name, u.email as inviter_email
		FROM invitations i
		LEFT JOIN teams t ON i.team_id = t.id
		LEFT JOIN projects p ON i.project_id = p.id
		JOIN users u ON i.invited_by = u.id
		WHERE i.email = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
		ORDER BY i.created_at DESC
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []models.InvitationWithDetails
	for rows.Next() {
		var inv models.InvitationWithDetails
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.TeamID, &inv.ProjectID, &inv.Role, &inv.InvitedBy,
			&inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
			&inv.TeamName, &inv.TeamSlug, &inv.ProjectName, &inv.ProjectSlug,
			&inv.InviterName, &inv.InviterEmail); err != nil {
			return nil, err
		}
		invitations = append(invitations, inv)
	}
	return invitations, rows.Err()
}

// ListByTeam returns all invitations for a team.
func (r *InvitationRepo) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]models.InvitationWithDetails, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.email, i.team_id, i.project_id, i.role, i.invited_by, i.token, i.expires_at, i.accepted_at, i.created_at,
		       t.name as team_name, t.slug as team_slug,
		       NULL as project_name, NULL as project_slug,
		       u.name as inviter_name, u.email as inviter_email
		FROM invitations i
		LEFT JOIN teams t ON i.team_id = t.id
		JOIN users u ON i.invited_by = u.id
		WHERE i.team_id = $1 AND i.accepted_at IS NULL
		ORDER BY i.created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []models.InvitationWithDetails
	for rows.Next() {
		var inv models.InvitationWithDetails
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.TeamID, &inv.ProjectID, &inv.Role, &inv.InvitedBy,
			&inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
			&inv.TeamName, &inv.TeamSlug, &inv.ProjectName, &inv.ProjectSlug,
			&inv.InviterName, &inv.InviterEmail); err != nil {
			return nil, err
		}
		invitations = append(invitations, inv)
	}
	return invitations, rows.Err()
}

// ListByProject returns all invitations for a project.
func (r *InvitationRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.InvitationWithDetails, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.email, i.team_id, i.project_id, i.role, i.invited_by, i.token, i.expires_at, i.accepted_at, i.created_at,
		       NULL as team_name, NULL as team_slug,
		       p.name as project_name, p.slug as project_slug,
		       u.name as inviter_name, u.email as inviter_email
		FROM invitations i
		LEFT JOIN projects p ON i.project_id = p.id
		JOIN users u ON i.invited_by = u.id
		WHERE i.project_id = $1 AND i.accepted_at IS NULL
		ORDER BY i.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []models.InvitationWithDetails
	for rows.Next() {
		var inv models.InvitationWithDetails
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.TeamID, &inv.ProjectID, &inv.Role, &inv.InvitedBy,
			&inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
			&inv.TeamName, &inv.TeamSlug, &inv.ProjectName, &inv.ProjectSlug,
			&inv.InviterName, &inv.InviterEmail); err != nil {
			return nil, err
		}
		invitations = append(invitations, inv)
	}
	return invitations, rows.Err()
}

// CleanupExpired deletes expired invitations.
func (r *InvitationRepo) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx, "DELETE FROM invitations WHERE expires_at < NOW() AND accepted_at IS NULL")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// generateInviteToken generates a secure invitation token.
func generateInviteToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
