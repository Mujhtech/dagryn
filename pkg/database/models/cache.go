package models

import (
	"time"

	"github.com/google/uuid"
)

// CacheEntry represents a cached task result.
type CacheEntry struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	ProjectID  uuid.UUID  `json:"project_id" db:"project_id"`
	TaskName   string     `json:"task_name" db:"task_name"`
	CacheKey   string     `json:"cache_key" db:"cache_key"`
	DigestHash string     `json:"digest_hash" db:"digest_hash"`
	SizeBytes  int64      `json:"size_bytes" db:"size_bytes"`
	HitCount   int        `json:"hit_count" db:"hit_count"`
	LastHitAt  *time.Time `json:"last_hit_at,omitempty" db:"last_hit_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// CacheBlob represents a content-addressable blob in cache storage.
type CacheBlob struct {
	DigestHash string    `json:"digest_hash" db:"digest_hash"`
	SizeBytes  int64     `json:"size_bytes" db:"size_bytes"`
	RefCount   int       `json:"ref_count" db:"ref_count"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// CacheUsage tracks daily cache usage metrics per project.
type CacheUsage struct {
	ProjectID       uuid.UUID `json:"project_id" db:"project_id"`
	Date            time.Time `json:"date" db:"date"`
	BytesUploaded   int64     `json:"bytes_uploaded" db:"bytes_uploaded"`
	BytesDownloaded int64     `json:"bytes_downloaded" db:"bytes_downloaded"`
	CacheHits       int       `json:"cache_hits" db:"cache_hits"`
	CacheMisses     int       `json:"cache_misses" db:"cache_misses"`
}

// CacheQuota tracks cache usage limits per project.
type CacheQuota struct {
	ProjectID             uuid.UUID  `json:"project_id" db:"project_id"`
	MaxSizeBytes          int64      `json:"max_size_bytes" db:"max_size_bytes"`
	CurrentSizeBytes      int64      `json:"current_size_bytes" db:"current_size_bytes"`
	MaxEntries            int        `json:"max_entries" db:"max_entries"`
	CurrentEntries        int        `json:"current_entries" db:"current_entries"`
	BillingAccountID      *uuid.UUID `json:"billing_account_id,omitempty" db:"billing_account_id"`
	MaxBandwidthBytes     int64      `json:"max_bandwidth_bytes" db:"max_bandwidth_bytes"`
	CurrentBandwidthBytes int64      `json:"current_bandwidth_bytes" db:"current_bandwidth_bytes"`
	BandwidthResetAt      *time.Time `json:"bandwidth_reset_at,omitempty" db:"bandwidth_reset_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
}
