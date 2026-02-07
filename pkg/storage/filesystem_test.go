package storage

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesystemBucket_PutAndGet(t *testing.T) {
	dir := t.TempDir()
	b, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	ctx := context.Background()

	// Put
	err = b.Put(ctx, "foo/bar.txt", bytes.NewReader([]byte("hello")), nil)
	require.NoError(t, err)

	// Get
	rc, err := b.Get(ctx, "foo/bar.txt")
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestFilesystemBucket_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	b, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	_, err = b.Get(context.Background(), "no-such-key")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.True(t, IsNotFound(err))
}

func TestFilesystemBucket_Exists(t *testing.T) {
	dir := t.TempDir()
	b, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	ctx := context.Background()

	ok, err := b.Exists(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, ok)

	err = b.Put(ctx, "present", bytes.NewReader([]byte("x")), nil)
	require.NoError(t, err)

	ok, err = b.Exists(ctx, "present")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestFilesystemBucket_Delete(t *testing.T) {
	dir := t.TempDir()
	b, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	ctx := context.Background()

	err = b.Put(ctx, "to-delete", bytes.NewReader([]byte("data")), nil)
	require.NoError(t, err)

	err = b.Delete(ctx, "to-delete")
	require.NoError(t, err)

	ok, err := b.Exists(ctx, "to-delete")
	require.NoError(t, err)
	assert.False(t, ok)

	// Deleting a non-existent key should not error
	err = b.Delete(ctx, "non-existent")
	require.NoError(t, err)
}

func TestFilesystemBucket_List(t *testing.T) {
	dir := t.TempDir()
	b, err := NewFilesystemBucket(dir, "")
	require.NoError(t, err)

	ctx := context.Background()

	// Create some objects
	for _, key := range []string{"a/1.txt", "a/2.txt", "b/3.txt"} {
		err = b.Put(ctx, key, bytes.NewReader([]byte("x")), nil)
		require.NoError(t, err)
	}

	// List with prefix
	result, err := b.List(ctx, "a/", nil)
	require.NoError(t, err)
	assert.Len(t, result.Keys, 2)

	// List all
	result, err = b.List(ctx, "", nil)
	require.NoError(t, err)
	assert.Len(t, result.Keys, 3)

	// List with max keys
	result, err = b.List(ctx, "", &ListOptions{MaxKeys: 1})
	require.NoError(t, err)
	assert.Len(t, result.Keys, 1)
	assert.True(t, result.IsTruncated)
}

func TestFilesystemBucket_WithPrefix(t *testing.T) {
	dir := t.TempDir()
	b, err := NewFilesystemBucket(dir, "cache/v1/")
	require.NoError(t, err)

	ctx := context.Background()

	err = b.Put(ctx, "key1", bytes.NewReader([]byte("val")), nil)
	require.NoError(t, err)

	ok, err := b.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, ok)

	rc, err := b.Get(ctx, "key1")
	require.NoError(t, err)
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	assert.Equal(t, "val", string(data))

	result, err := b.List(ctx, "", nil)
	require.NoError(t, err)
	assert.Len(t, result.Keys, 1)
	assert.Equal(t, "key1", result.Keys[0])
}
