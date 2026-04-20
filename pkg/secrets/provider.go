package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awssecrets "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

var ErrUnsupportedProvider = errors.New("unsupported secret provider")

type Provider interface {
	Put(ctx context.Context, ref, value string) error
	Get(ctx context.Context, ref string) (string, error)
	Delete(ctx context.Context, ref string) error
}

type Config struct {
	AWSRegion           string
	AWSAccessKeyID      string
	AWSSecretAccessKey  string
	AWSCredentialsFile  string
	GCPCredentialsFile  string
	CloudflareAccountID string
	CloudflareAPIToken  string
	CloudflareAPIBase   string
}

func NewProvider(ctx context.Context, provider models.SecretProvider) (Provider, error) {
	return NewProviderWithConfig(ctx, provider, Config{})
}

func NewProviderWithConfig(ctx context.Context, provider models.SecretProvider, cfg Config) (Provider, error) {
	switch provider {
	case models.SecretProviderAWS:
		awsOpts := []func(*awsconfig.LoadOptions) error{}
		if cfg.AWSRegion != "" {
			awsOpts = append(awsOpts, awsconfig.WithRegion(cfg.AWSRegion))
		}
		if cfg.AWSAccessKeyID != "" && cfg.AWSSecretAccessKey != "" {
			awsOpts = append(awsOpts, awsconfig.WithCredentialsProvider(
				awscreds.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
			))
		}
		if cfg.AWSCredentialsFile != "" {
			_ = os.Setenv("AWS_SHARED_CREDENTIALS_FILE", cfg.AWSCredentialsFile)
		}
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsOpts...)
		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}
		return &awsProvider{client: awssecrets.NewFromConfig(awsCfg)}, nil
	case models.SecretProviderGCP:
		if cfg.GCPCredentialsFile != "" {
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.GCPCredentialsFile)
		}
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("create gcp secret manager client: %w", err)
		}
		return &gcpProvider{client: client}, nil
	case models.SecretProviderCloudflare:
		return &cloudflareProvider{
			accountID: cfg.CloudflareAccountID,
			token:     cfg.CloudflareAPIToken,
			baseURL:   cfg.CloudflareAPIBase,
		}, nil
	default:
		return nil, ErrUnsupportedProvider
	}
}
