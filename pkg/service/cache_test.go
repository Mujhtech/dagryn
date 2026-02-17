package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlobStorageKey(t *testing.T) {
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	key := blobStorageKey(hash)
	assert.Equal(t, "blobs/ab/"+hash, key)
}

func TestBlobStorageKey_ShortHash(t *testing.T) {
	hash := "ab"
	key := blobStorageKey(hash)
	assert.Equal(t, "blobs/ab/ab", key)
}

func TestCacheStats_EmptyTopTasks(t *testing.T) {
	stats := &CacheStats{
		TotalEntries:   0,
		TotalSizeBytes: 0,
		HitCount:       0,
		QuotaUsedPct:   0,
	}
	assert.Nil(t, stats.TopTasks)
	assert.Equal(t, 0, stats.TotalEntries)
}

func TestGCResult_Zero(t *testing.T) {
	result := &GCResult{}
	assert.Equal(t, 0, result.EntriesRemoved)
	assert.Equal(t, int64(0), result.BytesFreed)
	assert.Equal(t, 0, result.BlobsRemoved)
}
