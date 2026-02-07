package remote

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mujhtech/dagryn/internal/cache"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBackend(t *testing.T) (*StorageBackend, string) {
	t.Helper()
	projectRoot := t.TempDir()
	bucketDir := t.TempDir()

	bucket, err := storage.NewFilesystemBucket(bucketDir, "")
	require.NoError(t, err)

	return NewStorageBackend(bucket, projectRoot), projectRoot
}

func TestStorageBackend_SaveAndRestore(t *testing.T) {
	backend, projectRoot := newTestBackend(t)
	ctx := context.Background()

	// Create output files
	distDir := filepath.Join(projectRoot, "dist")
	require.NoError(t, os.MkdirAll(distDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "app.js"), []byte("console.log('hello')"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "style.css"), []byte("body{margin:0}"), 0644))

	meta := cache.Metadata{TaskName: "build", CacheKey: "abc123"}

	// Save
	err := backend.Save(ctx, "build", "abc123", []string{"dist/*"}, meta)
	require.NoError(t, err)

	// Verify it exists
	exists, err := backend.Check(ctx, "build", "abc123")
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete the original files
	require.NoError(t, os.RemoveAll(distDir))

	// Restore
	err = backend.Restore(ctx, "build", "abc123")
	require.NoError(t, err)

	// Verify files were restored
	data, err := os.ReadFile(filepath.Join(projectRoot, "dist", "app.js"))
	require.NoError(t, err)
	assert.Equal(t, "console.log('hello')", string(data))

	data, err = os.ReadFile(filepath.Join(projectRoot, "dist", "style.css"))
	require.NoError(t, err)
	assert.Equal(t, "body{margin:0}", string(data))
}

func TestStorageBackend_CheckMiss(t *testing.T) {
	backend, _ := newTestBackend(t)
	ctx := context.Background()

	exists, err := backend.Check(ctx, "build", "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestStorageBackend_RestoreNotFound(t *testing.T) {
	backend, _ := newTestBackend(t)
	ctx := context.Background()

	err := backend.Restore(ctx, "build", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStorageBackend_Clear(t *testing.T) {
	backend, projectRoot := newTestBackend(t)
	ctx := context.Background()

	// Create and save output
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "out.txt"), []byte("data"), 0644))
	meta := cache.Metadata{TaskName: "build", CacheKey: "key1"}
	require.NoError(t, backend.Save(ctx, "build", "key1", []string{"out.txt"}, meta))

	exists, err := backend.Check(ctx, "build", "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	// Clear
	require.NoError(t, backend.Clear(ctx, "build"))

	exists, err = backend.Check(ctx, "build", "key1")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestStorageBackend_ClearAll(t *testing.T) {
	backend, projectRoot := newTestBackend(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "out.txt"), []byte("data"), 0644))

	meta1 := cache.Metadata{TaskName: "build", CacheKey: "k1"}
	require.NoError(t, backend.Save(ctx, "build", "k1", []string{"out.txt"}, meta1))

	meta2 := cache.Metadata{TaskName: "test", CacheKey: "k2"}
	require.NoError(t, backend.Save(ctx, "test", "k2", []string{"out.txt"}, meta2))

	require.NoError(t, backend.ClearAll(ctx))

	exists1, _ := backend.Check(ctx, "build", "k1")
	exists2, _ := backend.Check(ctx, "test", "k2")
	assert.False(t, exists1)
	assert.False(t, exists2)
}

func TestStorageBackend_ContentDeduplication(t *testing.T) {
	projectRoot := t.TempDir()
	bucketDir := t.TempDir()

	bucket, err := storage.NewFilesystemBucket(bucketDir, "")
	require.NoError(t, err)

	backend := NewStorageBackend(bucket, projectRoot)
	ctx := context.Background()

	// Create two files with identical content
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "a.txt"), []byte("same content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "b.txt"), []byte("same content"), 0644))

	meta := cache.Metadata{TaskName: "build", CacheKey: "dedup"}
	require.NoError(t, backend.Save(ctx, "build", "dedup", []string{"a.txt", "b.txt"}, meta))

	// List CAS entries - should have only 1 blob (same content = same hash)
	result, err := bucket.List(ctx, "cas/", nil)
	require.NoError(t, err)
	assert.Len(t, result.Keys, 1, "identical files should be deduplicated in CAS")
}

func TestDigest_Key(t *testing.T) {
	d := DigestBytes([]byte("hello"))
	key := d.Key()
	assert.Contains(t, key, "cas/")
	assert.Contains(t, key, d.Hash[:2])
	assert.Contains(t, key, d.Hash)
}

func TestActionKey(t *testing.T) {
	key := ActionKey("build", "abc123")
	assert.Equal(t, "ac/build/abc123", key)
}

func TestManifest_MarshalUnmarshal(t *testing.T) {
	d := DigestBytes([]byte("data"))
	m := &Manifest{
		Files: map[string]*Digest{
			"dist/app.js": &d,
		},
	}

	data, err := MarshalManifest(m)
	require.NoError(t, err)

	m2, err := UnmarshalManifest(data)
	require.NoError(t, err)
	assert.Equal(t, m.Files["dist/app.js"].Hash, m2.Files["dist/app.js"].Hash)
}

func TestStorageBackend_CrossInstanceSharing(t *testing.T) {
	// Simulate two project copies sharing a remote cache
	bucketDir := t.TempDir()
	bucket, err := storage.NewFilesystemBucket(bucketDir, "")
	require.NoError(t, err)

	ctx := context.Background()

	// Project A: create and save
	projectA := t.TempDir()
	backendA := NewStorageBackend(bucket, projectA)

	require.NoError(t, os.WriteFile(filepath.Join(projectA, "build.js"), []byte("built output"), 0644))
	meta := cache.Metadata{TaskName: "build", CacheKey: "shared-key"}
	require.NoError(t, backendA.Save(ctx, "build", "shared-key", []string{"build.js"}, meta))

	// Project B: restore from shared cache
	projectB := t.TempDir()
	backendB := NewStorageBackend(bucket, projectB)

	exists, err := backendB.Check(ctx, "build", "shared-key")
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, backendB.Restore(ctx, "build", "shared-key"))

	data, err := os.ReadFile(filepath.Join(projectB, "build.js"))
	require.NoError(t, err)
	assert.Equal(t, "built output", string(data))
}
