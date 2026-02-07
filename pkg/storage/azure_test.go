package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAzureBucket_FullKey(t *testing.T) {
	b := &AzureBucket{prefix: "cache/v1/"}
	assert.Equal(t, "cache/v1/foo/bar", b.fullKey("foo/bar"))
}

func TestAzureBucket_FullKey_NoPrefix(t *testing.T) {
	b := &AzureBucket{prefix: ""}
	assert.Equal(t, "foo/bar", b.fullKey("foo/bar"))
}

func TestNewBucket_AzureMissingBucket(t *testing.T) {
	_, err := NewBucket(Config{Provider: ProviderAzure, Bucket: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container name")
}

func TestNewBucket_AzureMissingEndpoint(t *testing.T) {
	_, err := NewBucket(Config{Provider: ProviderAzure, Bucket: "mycontainer"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestConfig_AzureProviderType(t *testing.T) {
	assert.Equal(t, ProviderType("azure"), ProviderAzure)
}
