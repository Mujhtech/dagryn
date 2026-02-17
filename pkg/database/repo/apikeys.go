package repo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// APIKeyRepo handles API key database operations.
type APIKeyRepo struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepo creates a new API key repository.
func NewAPIKeyRepo(pool *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{pool: pool}
}

// Create creates a new API key and returns the raw key (only shown once).
func (r *APIKeyRepo) Create(ctx context.Context, key *models.APIKey) (string, error) {
	key.ID = uuid.New()
	key.CreatedAt = time.Now()

	// Generate a cryptographically secure random key
	rawKey, err := generateAPIKey(key.Scope)
	if err != nil {
		return "", err
	}

	// Store the hash, not the key
	key.KeyHash = hashAPIKey(rawKey)
	key.KeyPrefix = rawKey[:10]

	_, err = r.pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, project_id, name, key_hash, key_prefix, scope, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, key.ID, key.UserID, key.ProjectID, key.Name, key.KeyHash, key.KeyPrefix, key.Scope, key.ExpiresAt, key.CreatedAt)

	if err != nil {
		return "", err
	}
	return rawKey, nil
}

// GetByID retrieves an API key by ID.
func (r *APIKeyRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	var key models.APIKey
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, project_id, name, key_hash, key_prefix, scope, last_used_at, expires_at, created_at, revoked_at
		FROM api_keys WHERE id = $1
	`, id).Scan(&key.ID, &key.UserID, &key.ProjectID, &key.Name, &key.KeyHash, &key.KeyPrefix,
		&key.Scope, &key.LastUsedAt, &key.ExpiresAt, &key.CreatedAt, &key.RevokedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &key, nil
}

// ValidateKey validates an API key and returns the associated key record.
func (r *APIKeyRepo) ValidateKey(ctx context.Context, rawKey string) (*models.APIKey, error) {
	if len(rawKey) < 10 {
		return nil, ErrNotFound
	}

	prefix := rawKey[:10]
	hash := hashAPIKey(rawKey)

	var key models.APIKey
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, project_id, name, key_hash, key_prefix, scope, last_used_at, expires_at, created_at, revoked_at
		FROM api_keys 
		WHERE key_prefix = $1 AND key_hash = $2 AND revoked_at IS NULL
		AND (expires_at IS NULL OR expires_at > NOW())
	`, prefix, hash).Scan(&key.ID, &key.UserID, &key.ProjectID, &key.Name, &key.KeyHash, &key.KeyPrefix,
		&key.Scope, &key.LastUsedAt, &key.ExpiresAt, &key.CreatedAt, &key.RevokedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Update last used timestamp asynchronously (don't block validation)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = r.pool.Exec(ctx, "UPDATE api_keys SET last_used_at = NOW() WHERE id = $1", key.ID)
	}()

	return &key, nil
}

// Revoke revokes an API key.
func (r *APIKeyRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser returns all API keys for a user.
func (r *APIKeyRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.APIKeyWithProject, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ak.id, ak.user_id, ak.project_id, ak.name, ak.key_hash, ak.key_prefix, ak.scope, 
		       ak.last_used_at, ak.expires_at, ak.created_at, ak.revoked_at,
		       p.name as project_name, p.slug as project_slug
		FROM api_keys ak
		LEFT JOIN projects p ON ak.project_id = p.id
		WHERE ak.user_id = $1
		ORDER BY ak.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.APIKeyWithProject
	for rows.Next() {
		var k models.APIKeyWithProject
		if err := rows.Scan(&k.ID, &k.UserID, &k.ProjectID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Scope,
			&k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt, &k.ProjectName, &k.ProjectSlug); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListByProject returns all API keys for a project.
func (r *APIKeyRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.APIKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, project_id, name, key_hash, key_prefix, scope, last_used_at, expires_at, created_at, revoked_at
		FROM api_keys WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.ProjectID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Scope,
			&k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListActive returns all active (non-revoked, non-expired) API keys for a user.
func (r *APIKeyRepo) ListActive(ctx context.Context, userID uuid.UUID) ([]models.APIKeyWithProject, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ak.id, ak.user_id, ak.project_id, ak.name, ak.key_hash, ak.key_prefix, ak.scope, 
		       ak.last_used_at, ak.expires_at, ak.created_at, ak.revoked_at,
		       p.name as project_name, p.slug as project_slug
		FROM api_keys ak
		LEFT JOIN projects p ON ak.project_id = p.id
		WHERE ak.user_id = $1 AND ak.revoked_at IS NULL AND (ak.expires_at IS NULL OR ak.expires_at > NOW())
		ORDER BY ak.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.APIKeyWithProject
	for rows.Next() {
		var k models.APIKeyWithProject
		if err := rows.Scan(&k.ID, &k.UserID, &k.ProjectID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Scope,
			&k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt, &k.ProjectName, &k.ProjectSlug); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// generateAPIKey generates a new API key with the appropriate prefix.
func generateAPIKey(scope models.APIKeyScope) (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Determine prefix based on scope
	var prefix string
	switch scope {
	case models.APIKeyScopeProject:
		prefix = models.APIKeyPrefixProject
	default:
		prefix = models.APIKeyPrefixLive
	}

	return prefix + hex.EncodeToString(bytes), nil
}

// hashAPIKey hashes an API key using SHA-256.
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
