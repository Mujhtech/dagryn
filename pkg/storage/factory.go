package storage

import "fmt"

// NewBucket creates a Bucket for the given configuration.
func NewBucket(cfg Config) (Bucket, error) {
	switch cfg.Provider {
	case ProviderFilesystem:
		if cfg.BasePath == "" {
			return nil, fmt.Errorf("storage: filesystem provider requires base_path")
		}
		return NewFilesystemBucket(cfg.BasePath, cfg.Prefix)
	case ProviderS3:
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("storage: s3 provider requires bucket name")
		}
		return NewS3Bucket(cfg)
	case ProviderGCS:
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("storage: gcs provider requires bucket name")
		}
		return NewGCSBucket(cfg)
	case ProviderAzure:
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("storage: azure provider requires container name")
		}
		return NewAzureBucket(cfg)
	case ProviderR2:
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("storage: r2 provider requires bucket name")
		}
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("storage: r2 provider requires endpoint (https://<account_id>.r2.cloudflarestorage.com)")
		}
		cfg.UsePathStyle = false
		cfg.DisableChecksum = true
		if cfg.Region == "" {
			cfg.Region = "auto"
		}
		return NewS3Bucket(cfg)
	case ProviderMinIO:
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("storage: minio provider requires bucket name")
		}
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("storage: minio provider requires endpoint")
		}
		cfg.UsePathStyle = true
		cfg.DisableChecksum = true
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
		return NewS3Bucket(cfg)
	default:
		return nil, fmt.Errorf("storage: unknown provider %q", cfg.Provider)
	}
}
