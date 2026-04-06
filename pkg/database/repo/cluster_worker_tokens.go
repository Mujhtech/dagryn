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

type ClusterWorkerTokenRepo struct {
	pool *pgxpool.Pool
}

func NewClusterWorkerTokenRepo(pool *pgxpool.Pool) ClusterWorkerTokenStore {
	return &ClusterWorkerTokenRepo{pool: pool}
}

func (r *ClusterWorkerTokenRepo) Create(ctx context.Context, token *models.ClusterWorkerToken) (string, error) {
	token.ID = uuid.New()
	token.CreatedAt = time.Now()

	rawToken, err := generateClusterWorkerToken()
	if err != nil {
		return "", err
	}
	token.KeyHash = hashClusterWorkerToken(rawToken)
	token.KeyPrefix = rawToken[:16]

	_, err = r.pool.Exec(ctx, `
		INSERT INTO cluster_worker_tokens (
			id, name, key_hash, key_prefix, scope_type, team_id, owner_user_id,
			cluster_id, expires_at, created_by_user_id, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, token.ID, token.Name, token.KeyHash, token.KeyPrefix, token.ScopeType,
		token.TeamID, token.OwnerUserID, token.ClusterID, token.ExpiresAt,
		token.CreatedByUserID, token.CreatedAt,
	)
	if err != nil {
		return "", err
	}

	return rawToken, nil
}

func (r *ClusterWorkerTokenRepo) Validate(ctx context.Context, rawToken string) (*models.ClusterWorkerToken, error) {
	if len(rawToken) < 16 {
		return nil, ErrNotFound
	}
	prefix := rawToken[:16]
	hash := hashClusterWorkerToken(rawToken)

	var t models.ClusterWorkerToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, key_hash, key_prefix, scope_type, team_id, owner_user_id,
		       cluster_id, last_used_at, expires_at, created_by_user_id, created_at, revoked_at
		FROM cluster_worker_tokens
		WHERE key_prefix = $1 AND key_hash = $2 AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
	`, prefix, hash).Scan(
		&t.ID, &t.Name, &t.KeyHash, &t.KeyPrefix, &t.ScopeType, &t.TeamID, &t.OwnerUserID,
		&t.ClusterID, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedByUserID, &t.CreatedAt, &t.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	go func(id uuid.UUID) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = r.pool.Exec(ctx, `UPDATE cluster_worker_tokens SET last_used_at = NOW() WHERE id = $1`, id)
	}(t.ID)

	return &t, nil
}

func (r *ClusterWorkerTokenRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.ClusterWorkerToken, error) {
	var t models.ClusterWorkerToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, key_hash, key_prefix, scope_type, team_id, owner_user_id,
		       cluster_id, last_used_at, expires_at, created_by_user_id, created_at, revoked_at
		FROM cluster_worker_tokens
		WHERE id = $1
	`).Scan(
		&t.ID, &t.Name, &t.KeyHash, &t.KeyPrefix, &t.ScopeType, &t.TeamID, &t.OwnerUserID,
		&t.ClusterID, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedByUserID, &t.CreatedAt, &t.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *ClusterWorkerTokenRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.ClusterWorkerToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, key_hash, key_prefix, scope_type, team_id, owner_user_id,
		       cluster_id, last_used_at, expires_at, created_by_user_id, created_at, revoked_at
		FROM cluster_worker_tokens
		WHERE owner_user_id = $1 OR team_id IN (
			SELECT team_id FROM team_members WHERE user_id = $1
		)
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ClusterWorkerToken
	for rows.Next() {
		var t models.ClusterWorkerToken
		if err := rows.Scan(
			&t.ID, &t.Name, &t.KeyHash, &t.KeyPrefix, &t.ScopeType, &t.TeamID, &t.OwnerUserID,
			&t.ClusterID, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedByUserID, &t.CreatedAt, &t.RevokedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *ClusterWorkerTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	res, err := r.pool.Exec(ctx, `UPDATE cluster_worker_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func generateClusterWorkerToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "dg_wkr_" + hex.EncodeToString(b), nil
}

func hashClusterWorkerToken(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
