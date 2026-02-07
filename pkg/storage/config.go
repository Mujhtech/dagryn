package storage

import "os"

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
	Prefix          string // key prefix applied to all operations
	CredentialsFile string // gcs provider: path to service account JSON
}

// ConfigFromEnv reads storage configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Provider:        ProviderType(os.Getenv("DAGRYN_STORAGE_PROVIDER")),
		Bucket:          os.Getenv("DAGRYN_STORAGE_BUCKET"),
		Region:          os.Getenv("DAGRYN_STORAGE_REGION"),
		Endpoint:        os.Getenv("DAGRYN_STORAGE_ENDPOINT"),
		AccessKeyID:     os.Getenv("DAGRYN_STORAGE_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("DAGRYN_STORAGE_SECRET_ACCESS_KEY"),
		Prefix:          os.Getenv("DAGRYN_STORAGE_PREFIX"),
		BasePath:        os.Getenv("DAGRYN_STORAGE_BASE_PATH"),
		CredentialsFile: os.Getenv("DAGRYN_STORAGE_CREDENTIALS_FILE"),
	}
}
