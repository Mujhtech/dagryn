package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/stretchr/testify/assert"
)

func TestCacheEntry_Model(t *testing.T) {
	now := time.Now()
	entry := models.CacheEntry{
		ID:         uuid.New(),
		ProjectID:  uuid.New(),
		TaskName:   "build",
		CacheKey:   "abc123",
		DigestHash: "sha256:deadbeef",
		SizeBytes:  1024,
		HitCount:   5,
		LastHitAt:  &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.Equal(t, "build", entry.TaskName)
	assert.Equal(t, "abc123", entry.CacheKey)
	assert.Equal(t, int64(1024), entry.SizeBytes)
	assert.Equal(t, 5, entry.HitCount)
	assert.NotNil(t, entry.LastHitAt)
	assert.Nil(t, entry.ExpiresAt)
}

func TestCacheBlob_Model(t *testing.T) {
	blob := models.CacheBlob{
		DigestHash: "sha256:deadbeef",
		SizeBytes:  2048,
		RefCount:   3,
		CreatedAt:  time.Now(),
	}

	assert.Equal(t, "sha256:deadbeef", blob.DigestHash)
	assert.Equal(t, int64(2048), blob.SizeBytes)
	assert.Equal(t, 3, blob.RefCount)
}

func TestCacheQuota_Model(t *testing.T) {
	quota := models.CacheQuota{
		ProjectID:        uuid.New(),
		MaxSizeBytes:     5368709120,
		CurrentSizeBytes: 1073741824,
		MaxEntries:       10000,
		CurrentEntries:   500,
		UpdatedAt:        time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, quota.ProjectID)
	assert.Equal(t, int64(5368709120), quota.MaxSizeBytes)
	assert.Equal(t, 500, quota.CurrentEntries)
}

func TestCacheStats_Fields(t *testing.T) {
	stats := CacheStats{
		TotalEntries:   100,
		TotalSizeBytes: 1048576,
		HitCount:       50,
		QuotaUsedPct:   20.5,
		TopTasks: []TaskCacheStats{
			{TaskName: "build", Entries: 10, SizeBytes: 524288, TotalHits: 30},
		},
	}

	assert.Equal(t, 100, stats.TotalEntries)
	assert.Equal(t, int64(1048576), stats.TotalSizeBytes)
	assert.Len(t, stats.TopTasks, 1)
	assert.Equal(t, "build", stats.TopTasks[0].TaskName)
}

func TestListEntriesOpts_Defaults(t *testing.T) {
	opts := ListEntriesOpts{}
	assert.Equal(t, 0, opts.Limit)
	assert.Equal(t, 0, opts.Offset)
}
