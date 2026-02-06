package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/internal/db/models"
)

// TokenRepo handles token database operations.
type TokenRepo struct {
	pool *pgxpool.Pool
}

// NewTokenRepo creates a new token repository.
func NewTokenRepo(pool *pgxpool.Pool) *TokenRepo {
	return &TokenRepo{pool: pool}
}

// Create creates a new token record.
func (r *TokenRepo) Create(ctx context.Context, token *models.Token) error {
	token.ID = uuid.New()
	token.CreatedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO tokens (id, user_id, project_id, token_type, jti, issued_at, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, token.ID, token.UserID, token.ProjectID, token.TokenType, token.JTI, token.IssuedAt, token.ExpiresAt, token.CreatedAt)

	return err
}

// GetByJTI retrieves a token by its JWT ID.
func (r *TokenRepo) GetByJTI(ctx context.Context, jti string) (*models.Token, error) {
	var token models.Token
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, project_id, token_type, jti, issued_at, expires_at, revoked_at, last_used_at, created_at
		FROM tokens WHERE jti = $1
	`, jti).Scan(&token.ID, &token.UserID, &token.ProjectID, &token.TokenType, &token.JTI,
		&token.IssuedAt, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &token, nil
}

// GetValidByJTI retrieves a valid (non-revoked, non-expired) token by its JWT ID.
func (r *TokenRepo) GetValidByJTI(ctx context.Context, jti string) (*models.Token, error) {
	var token models.Token
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, project_id, token_type, jti, issued_at, expires_at, revoked_at, last_used_at, created_at
		FROM tokens 
		WHERE jti = $1 AND revoked_at IS NULL AND expires_at > NOW()
	`, jti).Scan(&token.ID, &token.UserID, &token.ProjectID, &token.TokenType, &token.JTI,
		&token.IssuedAt, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &token, nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (r *TokenRepo) UpdateLastUsed(ctx context.Context, jti string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET last_used_at = NOW() WHERE jti = $1
	`, jti)
	return err
}

// Revoke revokes a token by its JWT ID.
func (r *TokenRepo) Revoke(ctx context.Context, jti string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE tokens SET revoked_at = NOW() WHERE jti = $1 AND revoked_at IS NULL
	`, jti)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAllForUser revokes all tokens for a user.
func (r *TokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

// RevokeByType revokes all tokens of a specific type for a user.
func (r *TokenRepo) RevokeByType(ctx context.Context, userID uuid.UUID, tokenType models.TokenType) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET revoked_at = NOW() 
		WHERE user_id = $1 AND token_type = $2 AND revoked_at IS NULL
	`, userID, tokenType)
	return err
}

// CleanupExpired deletes expired tokens older than the given duration.
func (r *TokenRepo) CleanupExpired(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.pool.Exec(ctx, `
		DELETE FROM tokens WHERE expires_at < $1
	`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// ListByUser returns all active tokens for a user.
func (r *TokenRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Token, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, project_id, token_type, jti, issued_at, expires_at, revoked_at, last_used_at, created_at
		FROM tokens 
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []models.Token
	for rows.Next() {
		var token models.Token
		if err := rows.Scan(&token.ID, &token.UserID, &token.ProjectID, &token.TokenType, &token.JTI,
			&token.IssuedAt, &token.ExpiresAt, &token.RevokedAt, &token.LastUsedAt, &token.CreatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}
