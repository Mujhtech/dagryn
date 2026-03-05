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

// SSORepo handles SSO database operations.
type SSORepo struct {
	pool *pgxpool.Pool
}

// NewSSORepo creates a new SSO repository.
func NewSSORepo(pool *pgxpool.Pool) SSOStore {
	return &SSORepo{pool: pool}
}

func (r *SSORepo) scanConnection(row pgx.Row) (*models.SSOConnection, error) {
	var conn models.SSOConnection
	err := row.Scan(
		&conn.ID, &conn.TeamID,
		&conn.IDPEntityID, &conn.IDPSsoURL, &conn.IDPMetadataURL, &conn.IDPMetadataXML,
		&conn.Certificate,
		&conn.SPEntityID, &conn.SPAcsURL,
		&conn.SCIMEnabled, &conn.SCIMTokenHash,
		&conn.EnforceSSO,
		&conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

const ssoConnectionColumns = `id, team_id, idp_entity_id, idp_sso_url, idp_metadata_url, idp_metadata_xml,
	certificate, sp_entity_id, sp_acs_url, scim_enabled, scim_token_hash, enforce_sso, created_at, updated_at`

// CreateConnection creates a new SSO connection.
func (r *SSORepo) CreateConnection(ctx context.Context, conn *models.SSOConnection) error {
	conn.ID = uuid.New()
	conn.CreatedAt = time.Now()
	conn.UpdatedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO sso_connections (id, team_id, idp_entity_id, idp_sso_url, idp_metadata_url, idp_metadata_xml,
			certificate, sp_entity_id, sp_acs_url, scim_enabled, scim_token_hash, enforce_sso, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, conn.ID, conn.TeamID, conn.IDPEntityID, conn.IDPSsoURL, conn.IDPMetadataURL, conn.IDPMetadataXML,
		conn.Certificate, conn.SPEntityID, conn.SPAcsURL, conn.SCIMEnabled, conn.SCIMTokenHash,
		conn.EnforceSSO, conn.CreatedAt, conn.UpdatedAt)
	return err
}

// GetConnectionByID retrieves an SSO connection by ID.
func (r *SSORepo) GetConnectionByID(ctx context.Context, id uuid.UUID) (*models.SSOConnection, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+ssoConnectionColumns+` FROM sso_connections WHERE id = $1`, id)
	return r.scanConnection(row)
}

// GetConnectionByTeamID retrieves an SSO connection by team ID.
func (r *SSORepo) GetConnectionByTeamID(ctx context.Context, teamID uuid.UUID) (*models.SSOConnection, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+ssoConnectionColumns+` FROM sso_connections WHERE team_id = $1`, teamID)
	return r.scanConnection(row)
}

// GetConnectionBySPEntityID retrieves an SSO connection by SP entity ID.
func (r *SSORepo) GetConnectionBySPEntityID(ctx context.Context, spEntityID string) (*models.SSOConnection, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+ssoConnectionColumns+` FROM sso_connections WHERE sp_entity_id = $1`, spEntityID)
	return r.scanConnection(row)
}

// UpdateConnection updates an SSO connection.
func (r *SSORepo) UpdateConnection(ctx context.Context, conn *models.SSOConnection) error {
	conn.UpdatedAt = time.Now()

	result, err := r.pool.Exec(ctx, `
		UPDATE sso_connections SET
			idp_entity_id = $1, idp_sso_url = $2, idp_metadata_url = $3, idp_metadata_xml = $4,
			certificate = $5, sp_entity_id = $6, sp_acs_url = $7,
			scim_enabled = $8, scim_token_hash = $9, enforce_sso = $10, updated_at = $11
		WHERE id = $12
	`, conn.IDPEntityID, conn.IDPSsoURL, conn.IDPMetadataURL, conn.IDPMetadataXML,
		conn.Certificate, conn.SPEntityID, conn.SPAcsURL,
		conn.SCIMEnabled, conn.SCIMTokenHash, conn.EnforceSSO, conn.UpdatedAt, conn.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteConnection deletes an SSO connection.
func (r *SSORepo) DeleteConnection(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM sso_connections WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateState creates a new SSO relay state.
func (r *SSORepo) CreateState(ctx context.Context, state *models.SSOState) error {
	state.ID = uuid.New()
	state.CreatedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO sso_states (id, connection_id, relay_state, redirect_url, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, state.ID, state.ConnectionID, state.RelayState, state.RedirectURL, state.ExpiresAt, state.CreatedAt)
	return err
}

// GetStateByRelayState retrieves a valid (non-expired) SSO state by relay state token.
func (r *SSORepo) GetStateByRelayState(ctx context.Context, relayState string) (*models.SSOState, error) {
	var state models.SSOState
	err := r.pool.QueryRow(ctx, `
		SELECT id, connection_id, relay_state, redirect_url, expires_at, created_at
		FROM sso_states WHERE relay_state = $1 AND expires_at > NOW()
	`, relayState).Scan(&state.ID, &state.ConnectionID, &state.RelayState, &state.RedirectURL, &state.ExpiresAt, &state.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &state, nil
}

// DeleteState deletes an SSO state.
func (r *SSORepo) DeleteState(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sso_states WHERE id = $1`, id)
	return err
}

// CleanupExpiredStates deletes all expired SSO states.
func (r *SSORepo) CleanupExpiredStates(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx, `DELETE FROM sso_states WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
