package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	gcstorage "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSBucket implements Bucket using Google Cloud Storage.
type GCSBucket struct {
	client *gcstorage.Client
	bucket string
	prefix string
}

// NewGCSBucket creates a GCS-backed Bucket from the given Config.
func NewGCSBucket(cfg Config) (*GCSBucket, error) {
	ctx := context.Background()

	var opts []option.ClientOption
	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, cfg.CredentialsFile))
	}

	client, err := gcstorage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("storage/gcs: create client: %w", err)
	}

	return &GCSBucket{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

func (b *GCSBucket) fullKey(key string) string {
	return b.prefix + key
}

func (b *GCSBucket) obj(key string) *gcstorage.ObjectHandle {
	return b.client.Bucket(b.bucket).Object(b.fullKey(key))
}

func (b *GCSBucket) Put(ctx context.Context, key string, r io.Reader, opts *PutOptions) error {
	w := b.obj(key).NewWriter(ctx)
	if opts != nil && opts.ContentType != "" {
		w.ContentType = opts.ContentType
	}
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return fmt.Errorf("storage/gcs: put %q: %w", key, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("storage/gcs: put %q close: %w", key, err)
	}
	return nil
}

func (b *GCSBucket) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	rc, err := b.obj(key).NewReader(ctx)
	if err != nil {
		if errors.Is(err, gcstorage.ErrObjectNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage/gcs: get %q: %w", key, err)
	}
	return rc, nil
}

func (b *GCSBucket) Delete(ctx context.Context, key string) error {
	err := b.obj(key).Delete(ctx)
	if err != nil {
		if errors.Is(err, gcstorage.ErrObjectNotExist) {
			return nil
		}
		return fmt.Errorf("storage/gcs: delete %q: %w", key, err)
	}
	return nil
}

func (b *GCSBucket) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.obj(key).Attrs(ctx)
	if err != nil {
		if errors.Is(err, gcstorage.ErrObjectNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("storage/gcs: exists %q: %w", key, err)
	}
	return true, nil
}

func (b *GCSBucket) List(ctx context.Context, prefix string, opts *ListOptions) (*ListResult, error) {
	maxKeys := 1000
	if opts != nil && opts.MaxKeys > 0 {
		maxKeys = opts.MaxKeys
	}

	query := &gcstorage.Query{Prefix: b.fullKey(prefix)}
	it := b.client.Bucket(b.bucket).Objects(ctx, query)

	result := &ListResult{}
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("storage/gcs: list prefix %q: %w", prefix, err)
		}
		key := strings.TrimPrefix(attrs.Name, b.prefix)
		result.Keys = append(result.Keys, key)
		if len(result.Keys) >= maxKeys {
			result.IsTruncated = true
			break
		}
	}
	return result, nil
}

// SignedPutURL returns a pre-signed URL for uploading an object.
func (b *GCSBucket) SignedPutURL(_ context.Context, key string, expiry time.Duration) (string, error) {
	url, err := b.client.Bucket(b.bucket).SignedURL(b.fullKey(key), &gcstorage.SignedURLOptions{
		Method:  "PUT",
		Expires: time.Now().Add(expiry),
	})
	if err != nil {
		return "", fmt.Errorf("storage/gcs: signed put url %q: %w", key, err)
	}
	return url, nil
}

// SignedGetURL returns a pre-signed URL for downloading an object.
func (b *GCSBucket) SignedGetURL(_ context.Context, key string, expiry time.Duration) (string, error) {
	url, err := b.client.Bucket(b.bucket).SignedURL(b.fullKey(key), &gcstorage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	})
	if err != nil {
		return "", fmt.Errorf("storage/gcs: signed get url %q: %w", key, err)
	}
	return url, nil
}
