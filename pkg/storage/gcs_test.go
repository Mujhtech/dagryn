package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGCSBucket_FullKey(t *testing.T) {
	b := &GCSBucket{prefix: "cache/v1/"}
	assert.Equal(t, "cache/v1/foo/bar", b.fullKey("foo/bar"))
}

func TestGCSBucket_FullKey_NoPrefix(t *testing.T) {
	b := &GCSBucket{prefix: ""}
	assert.Equal(t, "foo/bar", b.fullKey("foo/bar"))
}

func TestNewBucket_GCSMissingBucket(t *testing.T) {
	_, err := NewBucket(Config{Provider: ProviderGCS, Bucket: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name")
}

func TestConfig_GCSProviderType(t *testing.T) {
	assert.Equal(t, ProviderType("gcs"), ProviderGCS)
}
