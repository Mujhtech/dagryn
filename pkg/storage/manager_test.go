package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_RegisterAndGet(t *testing.T) {
	dir := t.TempDir()
	bucket, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	m := NewManager()
	m.Register("cache", bucket)

	got, err := m.Get("cache")
	assert.NoError(t, err)
	assert.Equal(t, bucket, got)
}

func TestManager_GetUnknown(t *testing.T) {
	m := NewManager()
	_, err := m.Get("nope")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestManager_Primary_DefaultsToFirst(t *testing.T) {
	dir := t.TempDir()
	bucket, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	m := NewManager()
	m.Register("cache", bucket)

	assert.Equal(t, bucket, m.Primary())
}

func TestManager_SetPrimary(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	b1, err := NewFilesystemBucket(dir1, "")
	require.NoError(t, err)
	b2, err := NewFilesystemBucket(dir2, "")
	require.NoError(t, err)

	m := NewManager()
	m.Register("a", b1)
	m.Register("b", b2)

	assert.Equal(t, b1, m.Primary())

	m.SetPrimary("b")
	assert.Equal(t, b2, m.Primary())
}

func TestManager_Names(t *testing.T) {
	dir := t.TempDir()
	bucket, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	m := NewManager()
	m.Register("alpha", bucket)
	m.Register("beta", bucket)

	names := m.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
}

func TestManager_Primary_Empty(t *testing.T) {
	m := NewManager()
	assert.Nil(t, m.Primary())
}
