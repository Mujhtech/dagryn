// Package repo provides database repository implementations.
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

// Common errors
var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyExists = errors.New("record already exists")
)

// UserRepo handles user database operations.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a new user repository.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// Create creates a new user.
func (r *UserRepo) Create(ctx context.Context, user *models.User) error {
	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, name, avatar_url, provider, provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, user.ID, user.Email, user.Name, user.AvatarURL, user.Provider, user.ProviderID, user.CreatedAt, user.UpdatedAt)

	return err
}

// GetByID retrieves a user by ID.
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, provider, provider_id, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Provider, &user.ProviderID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a user by email.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, provider, provider_id, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Provider, &user.ProviderID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByProvider retrieves a user by OAuth provider and provider ID.
func (r *UserRepo) GetByProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, provider, provider_id, created_at, updated_at
		FROM users WHERE provider = $1 AND provider_id = $2
	`, provider, providerID).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Provider, &user.ProviderID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

// Update updates a user.
func (r *UserRepo) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()

	result, err := r.pool.Exec(ctx, `
		UPDATE users SET email = $1, name = $2, avatar_url = $3, updated_at = $4
		WHERE id = $5
	`, user.Email, user.Name, user.AvatarURL, user.UpdatedAt, user.ID)

	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertByProvider creates or updates a user based on OAuth provider.
func (r *UserRepo) UpsertByProvider(ctx context.Context, user *models.User) error {
	now := time.Now()

	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, name, avatar_url, provider, provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (provider, provider_id) DO UPDATE SET
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = $7
		RETURNING id, created_at, updated_at
	`, uuid.New(), user.Email, user.Name, user.AvatarURL, user.Provider, user.ProviderID, now).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	return err
}

// Delete deletes a user.
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
