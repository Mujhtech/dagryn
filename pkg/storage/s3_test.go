package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestS3Bucket_FullKey(t *testing.T) {
	b := &S3Bucket{prefix: "cache/v1/"}
	assert.Equal(t, "cache/v1/foo/bar", b.fullKey("foo/bar"))
}

func TestS3Bucket_FullKey_NoPrefix(t *testing.T) {
	b := &S3Bucket{prefix: ""}
	assert.Equal(t, "foo/bar", b.fullKey("foo/bar"))
}

func TestNewS3Bucket_MissingBucket(t *testing.T) {
	_, err := NewBucket(Config{Provider: ProviderS3, Bucket: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name")
}

func TestNewBucket_UnknownProvider(t *testing.T) {
	_, err := NewBucket(Config{Provider: "unknown"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestNewBucket_FilesystemMissingPath(t *testing.T) {
	_, err := NewBucket(Config{Provider: ProviderFilesystem, BasePath: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base_path")
}

func TestNewBucket_Filesystem(t *testing.T) {
	dir := t.TempDir()
	b, err := NewBucket(Config{Provider: ProviderFilesystem, BasePath: dir})
	assert.NoError(t, err)
	assert.NotNil(t, b)
}

func TestConfig_ProviderTypes(t *testing.T) {
	assert.Equal(t, ProviderType("filesystem"), ProviderFilesystem)
	assert.Equal(t, ProviderType("s3"), ProviderS3)
}
