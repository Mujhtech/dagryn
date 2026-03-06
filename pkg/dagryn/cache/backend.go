package cache

import "context"

// Backend defines the interface for cache storage implementations.
type Backend interface {
	Check(ctx context.Context, taskName, key string) (bool, error)
	Restore(ctx context.Context, taskName, key string) error
	Save(ctx context.Context, taskName, key string, outputPatterns []string, meta Metadata) error
	Clear(ctx context.Context, taskName string) error
	ClearAll(ctx context.Context) error
}
