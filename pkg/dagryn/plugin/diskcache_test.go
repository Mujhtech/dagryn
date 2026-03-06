package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiskCache_GetSet(t *testing.T) {
	cache := NewDiskCache(t.TempDir())

	// Miss on empty cache
	data, err := cache.Get("test/key")
	assert.NoError(t, err)
	assert.Nil(t, data)

	// Set and get
	err = cache.Set("test/key", []byte(`{"hello":"world"}`), 1*time.Hour)
	require.NoError(t, err)

	data, err = cache.Get("test/key")
	assert.NoError(t, err)
	assert.Equal(t, `{"hello":"world"}`, string(data))
}

func TestDiskCache_TTLExpiry(t *testing.T) {
	cache := NewDiskCache(t.TempDir())

	// Set with very short TTL
	err := cache.Set("expiring", []byte(`"data"`), 1*time.Millisecond)
	require.NoError(t, err)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	data, err := cache.Get("expiring")
	assert.NoError(t, err)
	assert.Nil(t, data, "expired entry should return nil")
}

func TestDiskCache_Disabled(t *testing.T) {
	cache := NewDiskCache(t.TempDir())
	cache.Disable()

	// Set should be a no-op
	err := cache.Set("test/key", []byte(`"data"`), 1*time.Hour)
	assert.NoError(t, err)

	// Get should always miss
	data, err := cache.Get("test/key")
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestDiskCache_OverwriteExisting(t *testing.T) {
	cache := NewDiskCache(t.TempDir())

	err := cache.Set("key", []byte(`"v1"`), 1*time.Hour)
	require.NoError(t, err)

	err = cache.Set("key", []byte(`"v2"`), 1*time.Hour)
	require.NoError(t, err)

	data, err := cache.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, `"v2"`, string(data))
}

func TestDiskCache_DifferentKeys(t *testing.T) {
	cache := NewDiskCache(t.TempDir())

	err := cache.Set("releases/owner/repo/latest.json", []byte(`"latest"`), 1*time.Hour)
	require.NoError(t, err)

	err = cache.Set("manifests/owner/repo/v1.0.0.toml", []byte(`"manifest"`), 24*time.Hour)
	require.NoError(t, err)

	data1, _ := cache.Get("releases/owner/repo/latest.json")
	data2, _ := cache.Get("manifests/owner/repo/v1.0.0.toml")

	assert.Equal(t, `"latest"`, string(data1))
	assert.Equal(t, `"manifest"`, string(data2))
}
