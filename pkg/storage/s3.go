package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Bucket implements Bucket using an S3-compatible object store.
type S3Bucket struct {
	client   *s3.Client
	bucket   string
	prefix   string
	provider ProviderType
}

// NewS3Bucket creates an S3-backed Bucket from the given Config.
func NewS3Bucket(cfg Config) (*S3Bucket, error) {
	ctx := context.Background()

	var optFns []func(*awsconfig.LoadOptions) error

	if cfg.Region != "" {
		optFns = append(optFns, awsconfig.WithRegion(cfg.Region))
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("storage/%s: load config: %w", cfg.Provider, err)
	}

	var s3OptFns []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}
	if cfg.UsePathStyle {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}
	if cfg.DisableChecksum {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		})
	}

	client := s3.NewFromConfig(awsCfg, s3OptFns...)

	return &S3Bucket{
		client:   client,
		bucket:   cfg.Bucket,
		prefix:   cfg.Prefix,
		provider: cfg.Provider,
	}, nil
}

func (b *S3Bucket) fullKey(key string) string {
	return b.prefix + key
}

func (b *S3Bucket) Put(ctx context.Context, key string, r io.Reader, opts *PutOptions) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
		Body:   r,
	}
	if opts != nil {
		if opts.ContentType != "" {
			input.ContentType = aws.String(opts.ContentType)
		}
		if opts.ContentLength > 0 {
			input.ContentLength = aws.Int64(opts.ContentLength)
		}
	}
	_, err := b.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("storage/%s: put %q: %w", b.provider, key, err)
	}
	return nil
}

func (b *S3Bucket) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		// Also check for NotFound in generic API errors
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage/%s: get %q: %w", b.provider, key, err)
	}
	return out.Body, nil
}

func (b *S3Bucket) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	})
	if err != nil {
		return fmt.Errorf("storage/%s: delete %q: %w", b.provider, key, err)
	}
	return nil
}

func (b *S3Bucket) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	})
	if err != nil {
		var nsk *types.NotFound
		if errors.As(err, &nsk) {
			return false, nil
		}
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("storage/%s: head %q: %w", b.provider, key, err)
	}
	return true, nil
}

func (b *S3Bucket) List(ctx context.Context, prefix string, opts *ListOptions) (*ListResult, error) {
	maxKeys := int32(1000)
	if opts != nil && opts.MaxKeys > 0 {
		maxKeys = int32(opts.MaxKeys)
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.bucket),
		Prefix:  aws.String(b.fullKey(prefix)),
		MaxKeys: aws.Int32(maxKeys),
	}

	out, err := b.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("storage/%s: list prefix %q: %w", b.provider, prefix, err)
	}

	result := &ListResult{
		IsTruncated: aws.ToBool(out.IsTruncated),
	}
	if out.NextContinuationToken != nil {
		result.NextPageToken = *out.NextContinuationToken
	}
	for _, obj := range out.Contents {
		key := strings.TrimPrefix(aws.ToString(obj.Key), b.prefix)
		result.Keys = append(result.Keys, key)
	}
	return result, nil
}

// SignedPutURL returns a pre-signed URL for uploading an object.
func (b *S3Bucket) SignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presigner := s3.NewPresignClient(b.client)
	req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("storage/%s: signed put url %q: %w", b.provider, key, err)
	}
	return req.URL, nil
}

// SignedGetURL returns a pre-signed URL for downloading an object.
func (b *S3Bucket) SignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presigner := s3.NewPresignClient(b.client)
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("storage/%s: signed get url %q: %w", b.provider, key, err)
	}
	return req.URL, nil
}
