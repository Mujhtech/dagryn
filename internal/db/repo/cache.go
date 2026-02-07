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

// CacheStats holds aggregate cache statistics for a project.
type CacheStats struct {
	TotalEntries   int              `json:"total_entries"`
	TotalSizeBytes int64            `json:"total_size_bytes"`
	HitCount       int64            `json:"hit_count"`
	QuotaUsedPct   float64          `json:"quota_used_pct"`
	TopTasks       []TaskCacheStats `json:"top_tasks"`
}

// TaskCacheStats holds cache stats for a single task.
type TaskCacheStats struct {
	TaskName  string `json:"task_name"`
	Entries   int    `json:"entries"`
	SizeBytes int64  `json:"size_bytes"`
	TotalHits int64  `json:"total_hits"`
}

// UsageAnalytics holds daily cache usage data for analytics.
type UsageAnalytics struct {
	Date            time.Time `json:"date"`
	BytesUploaded   int64     `json:"bytes_uploaded"`
	BytesDownloaded int64     `json:"bytes_downloaded"`
	CacheHits       int       `json:"cache_hits"`
	CacheMisses     int       `json:"cache_misses"`
	HitRate         float64   `json:"hit_rate"`
}

// ListEntriesOpts configures entry listing.
type ListEntriesOpts struct {
	Limit  int
	Offset int
}

// CacheRepo handles cache database operations.
type CacheRepo struct {
	pool *pgxpool.Pool
}

// NewCacheRepo creates a new cache repository.
func NewCacheRepo(pool *pgxpool.Pool) *CacheRepo {
	return &CacheRepo{pool: pool}
}

// FindEntry looks up a cache entry by project, task, and key.
func (r *CacheRepo) FindEntry(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string) (*models.CacheEntry, error) {
	var e models.CacheEntry
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, task_name, cache_key, digest_hash, size_bytes,
		       hit_count, last_hit_at, created_at, updated_at, expires_at
		FROM cache_entries
		WHERE project_id = $1 AND task_name = $2 AND cache_key = $3
	`, projectID, taskName, cacheKey).Scan(
		&e.ID, &e.ProjectID, &e.TaskName, &e.CacheKey, &e.DigestHash,
		&e.SizeBytes, &e.HitCount, &e.LastHitAt, &e.CreatedAt, &e.UpdatedAt, &e.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

// UpsertEntry creates or updates a cache entry.
func (r *CacheRepo) UpsertEntry(ctx context.Context, entry *models.CacheEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	now := time.Now()
	entry.UpdatedAt = now
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO cache_entries (id, project_id, task_name, cache_key, digest_hash, size_bytes, hit_count, last_hit_at, created_at, updated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (project_id, task_name, cache_key)
		DO UPDATE SET digest_hash = EXCLUDED.digest_hash,
		              size_bytes = EXCLUDED.size_bytes,
		              updated_at = EXCLUDED.updated_at,
		              expires_at = EXCLUDED.expires_at
	`, entry.ID, entry.ProjectID, entry.TaskName, entry.CacheKey, entry.DigestHash,
		entry.SizeBytes, entry.HitCount, entry.LastHitAt, entry.CreatedAt, entry.UpdatedAt, entry.ExpiresAt)
	return err
}

// DeleteEntry removes a cache entry by ID.
func (r *CacheRepo) DeleteEntry(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM cache_entries WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListEntries lists cache entries for a project with pagination.
func (r *CacheRepo) ListEntries(ctx context.Context, projectID uuid.UUID, opts ListEntriesOpts) ([]models.CacheEntry, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, task_name, cache_key, digest_hash, size_bytes,
		       hit_count, last_hit_at, created_at, updated_at, expires_at
		FROM cache_entries
		WHERE project_id = $1
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3
	`, projectID, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.CacheEntry
	for rows.Next() {
		var e models.CacheEntry
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &e.TaskName, &e.CacheKey, &e.DigestHash,
			&e.SizeBytes, &e.HitCount, &e.LastHitAt, &e.CreatedAt, &e.UpdatedAt, &e.ExpiresAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// IncrementHitCount increments the hit counter and updates last_hit_at.
func (r *CacheRepo) IncrementHitCount(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE cache_entries
		SET hit_count = hit_count + 1, last_hit_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertBlob creates or updates a cache blob record.
func (r *CacheRepo) UpsertBlob(ctx context.Context, blob *models.CacheBlob) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cache_blobs (digest_hash, size_bytes, ref_count, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (digest_hash)
		DO UPDATE SET ref_count = cache_blobs.ref_count + 1
	`, blob.DigestHash, blob.SizeBytes, blob.RefCount, time.Now())
	return err
}

// DecrementBlobRef decrements the reference count for a blob.
func (r *CacheRepo) DecrementBlobRef(ctx context.Context, digestHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE cache_blobs SET ref_count = GREATEST(ref_count - 1, 0) WHERE digest_hash = $1
	`, digestHash)
	return err
}

// ListOrphanedBlobs returns blobs with ref_count = 0.
func (r *CacheRepo) ListOrphanedBlobs(ctx context.Context) ([]models.CacheBlob, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT digest_hash, size_bytes, ref_count, created_at
		FROM cache_blobs WHERE ref_count = 0
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blobs []models.CacheBlob
	for rows.Next() {
		var b models.CacheBlob
		if err := rows.Scan(&b.DigestHash, &b.SizeBytes, &b.RefCount, &b.CreatedAt); err != nil {
			return nil, err
		}
		blobs = append(blobs, b)
	}
	return blobs, rows.Err()
}

// DeleteBlob removes a blob record.
func (r *CacheRepo) DeleteBlob(ctx context.Context, digestHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM cache_blobs WHERE digest_hash = $1`, digestHash)
	return err
}

// GetQuota returns the cache quota for a project, or nil if none exists.
func (r *CacheRepo) GetQuota(ctx context.Context, projectID uuid.UUID) (*models.CacheQuota, error) {
	var q models.CacheQuota
	err := r.pool.QueryRow(ctx, `
		SELECT project_id, max_size_bytes, current_size_bytes, max_entries, current_entries, updated_at
		FROM cache_quotas WHERE project_id = $1
	`, projectID).Scan(
		&q.ProjectID, &q.MaxSizeBytes, &q.CurrentSizeBytes, &q.MaxEntries, &q.CurrentEntries, &q.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

// EnsureQuota creates a default quota record if one doesn't exist.
func (r *CacheRepo) EnsureQuota(ctx context.Context, projectID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cache_quotas (project_id) VALUES ($1) ON CONFLICT DO NOTHING
	`, projectID)
	return err
}

// UpdateQuotaUsage adjusts the quota counters by the given deltas.
func (r *CacheRepo) UpdateQuotaUsage(ctx context.Context, projectID uuid.UUID, sizeDelta int64, entryDelta int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE cache_quotas
		SET current_size_bytes = GREATEST(current_size_bytes + $2, 0),
		    current_entries = GREATEST(current_entries + $3, 0),
		    updated_at = NOW()
		WHERE project_id = $1
	`, projectID, sizeDelta, entryDelta)
	return err
}

// GetStats returns aggregate cache statistics for a project.
func (r *CacheRepo) GetStats(ctx context.Context, projectID uuid.UUID) (*CacheStats, error) {
	stats := &CacheStats{}

	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(size_bytes), 0), COALESCE(SUM(hit_count), 0)
		FROM cache_entries WHERE project_id = $1
	`, projectID).Scan(&stats.TotalEntries, &stats.TotalSizeBytes, &stats.HitCount)
	if err != nil {
		return nil, err
	}

	quota, err := r.GetQuota(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if quota != nil && quota.MaxSizeBytes > 0 {
		stats.QuotaUsedPct = float64(quota.CurrentSizeBytes) / float64(quota.MaxSizeBytes) * 100
	}

	rows, err := r.pool.Query(ctx, `
		SELECT task_name, COUNT(*) as entries, COALESCE(SUM(size_bytes), 0), COALESCE(SUM(hit_count), 0)
		FROM cache_entries WHERE project_id = $1
		GROUP BY task_name ORDER BY SUM(size_bytes) DESC LIMIT 10
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ts TaskCacheStats
		if err := rows.Scan(&ts.TaskName, &ts.Entries, &ts.SizeBytes, &ts.TotalHits); err != nil {
			return nil, err
		}
		stats.TopTasks = append(stats.TopTasks, ts)
	}
	return stats, rows.Err()
}

// ListExpired returns entries that have expired before the given time.
func (r *CacheRepo) ListExpired(ctx context.Context, before time.Time) ([]models.CacheEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, task_name, cache_key, digest_hash, size_bytes,
		       hit_count, last_hit_at, created_at, updated_at, expires_at
		FROM cache_entries
		WHERE expires_at IS NOT NULL AND expires_at < $1
	`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.CacheEntry
	for rows.Next() {
		var e models.CacheEntry
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &e.TaskName, &e.CacheKey, &e.DigestHash,
			&e.SizeBytes, &e.HitCount, &e.LastHitAt, &e.CreatedAt, &e.UpdatedAt, &e.ExpiresAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ListLRU returns the least-recently-used entries for a project.
func (r *CacheRepo) ListLRU(ctx context.Context, projectID uuid.UUID, limit int) ([]models.CacheEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, task_name, cache_key, digest_hash, size_bytes,
		       hit_count, last_hit_at, created_at, updated_at, expires_at
		FROM cache_entries
		WHERE project_id = $1
		ORDER BY COALESCE(last_hit_at, created_at) ASC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.CacheEntry
	for rows.Next() {
		var e models.CacheEntry
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &e.TaskName, &e.CacheKey, &e.DigestHash,
			&e.SizeBytes, &e.HitCount, &e.LastHitAt, &e.CreatedAt, &e.UpdatedAt, &e.ExpiresAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// IncrementUsage upserts a daily usage row, incrementing the given counters.
func (r *CacheRepo) IncrementUsage(ctx context.Context, projectID uuid.UUID, bytesUploaded, bytesDownloaded int64, hits, misses int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cache_usage (project_id, date, bytes_uploaded, bytes_downloaded, cache_hits, cache_misses)
		VALUES ($1, CURRENT_DATE, $2, $3, $4, $5)
		ON CONFLICT (project_id, date)
		DO UPDATE SET bytes_uploaded = cache_usage.bytes_uploaded + EXCLUDED.bytes_uploaded,
		              bytes_downloaded = cache_usage.bytes_downloaded + EXCLUDED.bytes_downloaded,
		              cache_hits = cache_usage.cache_hits + EXCLUDED.cache_hits,
		              cache_misses = cache_usage.cache_misses + EXCLUDED.cache_misses
	`, projectID, bytesUploaded, bytesDownloaded, hits, misses)
	return err
}

// GetUsageAnalytics returns daily cache usage for a project over the given number of days.
func (r *CacheRepo) GetUsageAnalytics(ctx context.Context, projectID uuid.UUID, days int) ([]UsageAnalytics, error) {
	if days <= 0 {
		days = 30
	}
	rows, err := r.pool.Query(ctx, `
		SELECT date, bytes_uploaded, bytes_downloaded, cache_hits, cache_misses
		FROM cache_usage
		WHERE project_id = $1 AND date >= CURRENT_DATE - $2::int
		ORDER BY date ASC
	`, projectID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analytics []UsageAnalytics
	for rows.Next() {
		var a UsageAnalytics
		if err := rows.Scan(&a.Date, &a.BytesUploaded, &a.BytesDownloaded, &a.CacheHits, &a.CacheMisses); err != nil {
			return nil, err
		}
		total := a.CacheHits + a.CacheMisses
		if total > 0 {
			a.HitRate = float64(a.CacheHits) / float64(total) * 100
		}
		analytics = append(analytics, a)
	}
	return analytics, rows.Err()
}
