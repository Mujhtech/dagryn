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

// ProviderTokenRepo handles provider token database operations.
type ProviderTokenRepo struct {
	pool *pgxpool.Pool
}

// NewProviderTokenRepo creates a new provider token repository.
func NewProviderTokenRepo(pool *pgxpool.Pool) *ProviderTokenRepo {
	return &ProviderTokenRepo{pool: pool}
}

// Upsert creates or updates a provider token for a user.
func (r *ProviderTokenRepo) Upsert(ctx context.Context, userID uuid.UUID, provider, accessTokenEncrypted string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO provider_tokens (id, user_id, provider, access_token_encrypted, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $4)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			access_token_encrypted = EXCLUDED.access_token_encrypted,
			updated_at = EXCLUDED.updated_at
	`, userID, provider, accessTokenEncrypted, now)
	return err
}

// GetByUserAndProvider returns the provider token for a user and provider, if any.
func (r *ProviderTokenRepo) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*models.ProviderToken, error) {
	var t models.ProviderToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, provider, access_token_encrypted, created_at, updated_at
		FROM provider_tokens WHERE user_id = $1 AND provider = $2
	`, userID, provider).Scan(&t.ID, &t.UserID, &t.Provider, &t.AccessTokenEncrypted, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}
