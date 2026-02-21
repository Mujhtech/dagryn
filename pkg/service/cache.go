package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/rs/zerolog"
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

// CacheAnalytics holds daily usage analytics for a project.
type CacheAnalytics struct {
	Days                 []DailyUsage `json:"days"`
	TotalHits            int          `json:"total_hits"`
	TotalMisses          int          `json:"total_misses"`
	HitRate              float64      `json:"hit_rate"`
	TotalBytesUploaded   int64        `json:"total_bytes_uploaded"`
	TotalBytesDownloaded int64        `json:"total_bytes_downloaded"`
}

// DailyUsage holds a single day's cache usage.
type DailyUsage struct {
	Date            string  `json:"date"`
	BytesUploaded   int64   `json:"bytes_uploaded"`
	BytesDownloaded int64   `json:"bytes_downloaded"`
	CacheHits       int     `json:"cache_hits"`
	CacheMisses     int     `json:"cache_misses"`
	HitRate         float64 `json:"hit_rate"`
}

// GCResult holds the results of a garbage collection run.
type GCResult struct {
	EntriesRemoved int   `json:"entries_removed"`
	BytesFreed     int64 `json:"bytes_freed"`
	BlobsRemoved   int   `json:"blobs_removed"`
}

// CacheService coordinates cache storage and database operations.
type CacheService struct {
	repo         repo.CacheStore
	bucket       storage.Bucket
	logger       zerolog.Logger
	entitlements entitlement.Checker
}

// NewCacheService creates a new cache service.
func NewCacheService(cacheRepo repo.CacheStore, bucket storage.Bucket, logger zerolog.Logger) *CacheService {
	return &CacheService{
		repo:   cacheRepo,
		bucket: bucket,
		logger: logger.With().Str("service", "cache").Logger(),
	}
}

// SetEntitlements sets the entitlement checker for quota enforcement.
func (s *CacheService) SetEntitlements(c entitlement.Checker) {
	s.entitlements = c
}

// Check returns true if a cache entry exists for the given project/task/key.
func (s *CacheService) Check(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string) (bool, error) {
	entry, err := s.repo.FindEntry(ctx, projectID, taskName, cacheKey)
	if err != nil {
		if err == repo.ErrNotFound {
			// Record miss
			_ = s.repo.IncrementUsage(ctx, projectID, 0, 0, 0, 1)
			return false, nil
		}
		return false, err
	}

	// Verify the blob still exists in storage
	blobKey := blobStorageKey(entry.DigestHash)
	exists, err := s.bucket.Exists(ctx, blobKey)
	if err != nil {
		s.logger.Warn().Err(err).Str("digest", entry.DigestHash).Msg("failed to verify blob in storage")
		return false, err
	}

	// Record hit or miss based on storage presence
	if exists {
		_ = s.repo.IncrementUsage(ctx, projectID, 0, 0, 1, 0)
	} else {
		_ = s.repo.IncrementUsage(ctx, projectID, 0, 0, 0, 1)
	}
	return exists, nil
}

// Upload stores cache content and creates/updates the entry + blob records.
func (s *CacheService) Upload(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string, r io.Reader, size int64) error {
	// Entitlement-based quota checks (unified path for OSS + cloud).
	if s.entitlements != nil {
		if err := s.entitlements.CheckQuota(ctx, "storage", projectID, size); err != nil {
			return err
		}
		if err := s.entitlements.CheckQuota(ctx, "cache_storage", projectID, size); err != nil {
			return err
		}
	}

	// Ensure quota record exists
	if err := s.repo.EnsureQuota(ctx, projectID); err != nil {
		return fmt.Errorf("cache: ensure quota: %w", err)
	}

	// Check quota before uploading
	quota, err := s.repo.GetQuota(ctx, projectID)
	if err != nil {
		return fmt.Errorf("cache: get quota: %w", err)
	}
	if quota != nil {
		if quota.CurrentSizeBytes+size > quota.MaxSizeBytes {
			return fmt.Errorf("cache: quota exceeded (current: %d, max: %d, upload: %d)",
				quota.CurrentSizeBytes, quota.MaxSizeBytes, size)
		}
	}

	// Buffer the stream so the body is seekable for S3 retries, and
	// compute the SHA256 digest in one pass without a temp key.
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("cache: read upload data: %w", err)
	}
	if size <= 0 {
		size = int64(len(data))
	}

	hasher := sha256.New()
	hasher.Write(data)
	digestHash := hex.EncodeToString(hasher.Sum(nil))
	blobKey := blobStorageKey(digestHash)

	// Check if blob already exists (content dedup)
	exists, err := s.bucket.Exists(ctx, blobKey)
	if err != nil {
		return fmt.Errorf("cache: check existing blob: %w", err)
	}

	if !exists {
		putOpts := &storage.PutOptions{ContentLength: size}
		if err := s.bucket.Put(ctx, blobKey, bytes.NewReader(data), putOpts); err != nil {
			return fmt.Errorf("cache: store cas blob: %w", err)
		}
	}

	// Check if we're replacing an existing entry (for quota delta)
	existingEntry, _ := s.repo.FindEntry(ctx, projectID, taskName, cacheKey)
	var sizeDelta int64
	var entryDelta int
	if existingEntry != nil {
		sizeDelta = size - existingEntry.SizeBytes
		// Decrement old blob ref
		if existingEntry.DigestHash != digestHash {
			_ = s.repo.DecrementBlobRef(ctx, existingEntry.DigestHash)
		}
	} else {
		sizeDelta = size
		entryDelta = 1
	}

	// Upsert entry
	entry := &models.CacheEntry{
		ProjectID:  projectID,
		TaskName:   taskName,
		CacheKey:   cacheKey,
		DigestHash: digestHash,
		SizeBytes:  size,
	}
	if err := s.repo.UpsertEntry(ctx, entry); err != nil {
		return fmt.Errorf("cache: upsert entry: %w", err)
	}

	// Upsert blob record
	blob := &models.CacheBlob{
		DigestHash: digestHash,
		SizeBytes:  size,
		RefCount:   1,
	}
	if err := s.repo.UpsertBlob(ctx, blob); err != nil {
		return fmt.Errorf("cache: upsert blob: %w", err)
	}

	// Update quota (size + entries)
	if err := s.repo.UpdateQuotaUsage(ctx, projectID, sizeDelta, entryDelta); err != nil {
		s.logger.Warn().Err(err).Msg("failed to update quota usage")
	}

	// Update bandwidth quota (uploads count towards bandwidth)
	_ = s.repo.IncrementBandwidthUsage(ctx, projectID, size)

	// Record upload usage in analytics
	_ = s.repo.IncrementUsage(ctx, projectID, size, 0, 0, 0)

	s.logger.Debug().
		Str("project", projectID.String()).
		Str("task", taskName).
		Str("key", cacheKey).
		Str("digest", digestHash).
		Int64("size", size).
		Msg("cache entry uploaded")

	return nil
}

// Download retrieves cache content and increments the hit counter.
func (s *CacheService) Download(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string) (io.ReadCloser, error) {
	entry, err := s.repo.FindEntry(ctx, projectID, taskName, cacheKey)
	if err != nil {
		return nil, err
	}

	// Bandwidth quota check.
	if s.entitlements != nil {
		if err := s.entitlements.CheckQuota(ctx, "bandwidth", projectID, entry.SizeBytes); err != nil {
			return nil, err
		}
	}

	blobKey := blobStorageKey(entry.DigestHash)
	rc, err := s.bucket.Get(ctx, blobKey)
	if err != nil {
		return nil, fmt.Errorf("cache: download blob: %w", err)
	}

	// Increment hit count, record download usage, and update bandwidth quota (fire-and-forget)
	go func() {
		bgCtx := context.Background()
		if err := s.repo.IncrementHitCount(bgCtx, entry.ID); err != nil {
			s.logger.Warn().Err(err).Str("entry_id", entry.ID.String()).Msg("failed to increment hit count")
		}
		_ = s.repo.IncrementUsage(bgCtx, projectID, 0, entry.SizeBytes, 0, 0)
		_ = s.repo.IncrementBandwidthUsage(bgCtx, projectID, entry.SizeBytes)
		if s.entitlements != nil {
			s.entitlements.RecordUsage(bgCtx, "bandwidth", projectID, entry.SizeBytes)
		}
	}()

	return rc, nil
}

// Delete removes a cache entry, decrements blob ref, and adjusts quota.
func (s *CacheService) Delete(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string) error {
	entry, err := s.repo.FindEntry(ctx, projectID, taskName, cacheKey)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteEntry(ctx, entry.ID); err != nil {
		return fmt.Errorf("cache: delete entry: %w", err)
	}

	if err := s.repo.DecrementBlobRef(ctx, entry.DigestHash); err != nil {
		s.logger.Warn().Err(err).Str("digest", entry.DigestHash).Msg("failed to decrement blob ref")
	}

	if err := s.repo.UpdateQuotaUsage(ctx, projectID, -entry.SizeBytes, -1); err != nil {
		s.logger.Warn().Err(err).Msg("failed to update quota after delete")
	}

	return nil
}

// GetStats returns aggregate cache statistics for a project.
func (s *CacheService) GetStats(ctx context.Context, projectID uuid.UUID) (*CacheStats, error) {
	repoStats, err := s.repo.GetStats(ctx, projectID)
	if err != nil {
		return nil, err
	}

	stats := &CacheStats{
		TotalEntries:   repoStats.TotalEntries,
		TotalSizeBytes: repoStats.TotalSizeBytes,
		HitCount:       repoStats.HitCount,
		QuotaUsedPct:   repoStats.QuotaUsedPct,
	}
	for _, ts := range repoStats.TopTasks {
		stats.TopTasks = append(stats.TopTasks, TaskCacheStats{
			TaskName:  ts.TaskName,
			Entries:   ts.Entries,
			SizeBytes: ts.SizeBytes,
			TotalHits: ts.TotalHits,
		})
	}
	return stats, nil
}

// GetAnalytics returns daily cache usage analytics for a project.
func (s *CacheService) GetAnalytics(ctx context.Context, projectID uuid.UUID, days int) (*CacheAnalytics, error) {
	rows, err := s.repo.GetUsageAnalytics(ctx, projectID, days)
	if err != nil {
		return nil, fmt.Errorf("cache: get analytics: %w", err)
	}

	analytics := &CacheAnalytics{}
	for _, row := range rows {
		analytics.Days = append(analytics.Days, DailyUsage{
			Date:            row.Date.Format("2006-01-02"),
			BytesUploaded:   row.BytesUploaded,
			BytesDownloaded: row.BytesDownloaded,
			CacheHits:       row.CacheHits,
			CacheMisses:     row.CacheMisses,
			HitRate:         row.HitRate,
		})
		analytics.TotalHits += row.CacheHits
		analytics.TotalMisses += row.CacheMisses
		analytics.TotalBytesUploaded += row.BytesUploaded
		analytics.TotalBytesDownloaded += row.BytesDownloaded
	}

	total := analytics.TotalHits + analytics.TotalMisses
	if total > 0 {
		analytics.HitRate = float64(analytics.TotalHits) / float64(total) * 100
	}

	return analytics, nil
}

// RunGC performs garbage collection: removes expired entries, evicts LRU entries
// when over quota, and deletes orphaned blobs from storage.
func (s *CacheService) RunGC(ctx context.Context, projectID uuid.UUID) (*GCResult, error) {
	result := &GCResult{}

	// 1. Remove expired entries
	expired, err := s.repo.ListExpired(ctx, time.Now())
	if err != nil {
		return nil, fmt.Errorf("cache gc: list expired: %w", err)
	}
	for _, entry := range expired {
		if entry.ProjectID != projectID {
			continue
		}
		if err := s.repo.DeleteEntry(ctx, entry.ID); err != nil {
			s.logger.Warn().Err(err).Str("entry_id", entry.ID.String()).Msg("gc: failed to delete expired entry")
			continue
		}
		_ = s.repo.DecrementBlobRef(ctx, entry.DigestHash)
		_ = s.repo.UpdateQuotaUsage(ctx, projectID, -entry.SizeBytes, -1)
		result.EntriesRemoved++
		result.BytesFreed += entry.SizeBytes
	}

	// 2. Evict LRU when over quota
	quota, err := s.repo.GetQuota(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("cache gc: get quota: %w", err)
	}
	if quota != nil && quota.CurrentSizeBytes > quota.MaxSizeBytes {
		excess := quota.CurrentSizeBytes - quota.MaxSizeBytes
		lruEntries, err := s.repo.ListLRU(ctx, projectID, 100)
		if err != nil {
			return nil, fmt.Errorf("cache gc: list lru: %w", err)
		}
		var freed int64
		for _, entry := range lruEntries {
			if freed >= excess {
				break
			}
			if err := s.repo.DeleteEntry(ctx, entry.ID); err != nil {
				continue
			}
			_ = s.repo.DecrementBlobRef(ctx, entry.DigestHash)
			_ = s.repo.UpdateQuotaUsage(ctx, projectID, -entry.SizeBytes, -1)
			freed += entry.SizeBytes
			result.EntriesRemoved++
			result.BytesFreed += entry.SizeBytes
		}
	}

	// 3. Clean up orphaned blobs
	orphans, err := s.repo.ListOrphanedBlobs(ctx)
	if err != nil {
		s.logger.Warn().Err(err).Msg("gc: failed to list orphaned blobs")
	} else {
		for _, blob := range orphans {
			blobKey := blobStorageKey(blob.DigestHash)
			if err := s.bucket.Delete(ctx, blobKey); err != nil {
				s.logger.Warn().Err(err).Str("digest", blob.DigestHash).Msg("gc: failed to delete orphaned blob from storage")
				continue
			}
			if err := s.repo.DeleteBlob(ctx, blob.DigestHash); err != nil {
				s.logger.Warn().Err(err).Str("digest", blob.DigestHash).Msg("gc: failed to delete blob record")
				continue
			}
			result.BlobsRemoved++
		}
	}

	s.logger.Info().
		Str("project", projectID.String()).
		Int("entries_removed", result.EntriesRemoved).
		Int64("bytes_freed", result.BytesFreed).
		Int("blobs_removed", result.BlobsRemoved).
		Msg("cache GC completed")

	return result, nil
}

func blobStorageKey(digestHash string) string {
	return fmt.Sprintf("blobs/%s/%s", digestHash[:2], digestHash)
}
