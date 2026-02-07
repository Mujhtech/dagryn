package storage

import (
	"context"
	"io"
	"time"
)

// PutOptions configures a Put operation.
type PutOptions struct {
	ContentType string
}

// ListOptions configures a List operation.
type ListOptions struct {
	MaxKeys int
}

// ListResult contains the results of a List operation.
type ListResult struct {
	Keys          []string
	IsTruncated   bool
	NextPageToken string
}

// Bucket provides object storage operations.
type Bucket interface {
	Put(ctx context.Context, key string, r io.Reader, opts *PutOptions) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	List(ctx context.Context, prefix string, opts *ListOptions) (*ListResult, error)
}

// SignedURLer is optionally implemented by Bucket implementations that support pre-signed URLs.
type SignedURLer interface {
	SignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	SignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}
