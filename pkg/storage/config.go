package storage

import "github.com/kelseyhightower/envconfig"

// ProviderType identifies a storage backend.
type ProviderType string

const (
	ProviderFilesystem ProviderType = "filesystem"
	ProviderS3         ProviderType = "s3"
	ProviderGCS        ProviderType = "gcs"
	ProviderAzure      ProviderType = "azure"
	ProviderR2         ProviderType = "r2"
	ProviderMinIO      ProviderType = "minio"
)

// Config holds configuration for creating a Bucket.
type Config struct {
	Provider        ProviderType
	BasePath        string // filesystem provider: root directory
	Bucket          string // s3/gcs/azure provider: bucket/container name
	Region          string // s3/gcs provider: region
	Endpoint        string // s3/azure provider: custom endpoint (MinIO, R2, Azure account URL)
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool   // s3 provider: use path-style addressing
	DisableChecksum bool   // s3 provider: disable request checksum (for R2/MinIO compat)
	Prefix          string // key prefix applied to all operations
	CredentialsFile string // gcs provider: path to service account JSON
}

// ConfigFromEnv reads storage configuration from environment variables.
func ConfigFromEnv() Config {
	var cfg Config
	_ = envconfig.Process("DAGRYN_STORAGE", &cfg)
	return cfg
}
