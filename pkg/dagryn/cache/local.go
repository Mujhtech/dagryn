package cache

import "context"

// LocalBackend wraps the existing Store as a Backend implementation.
type LocalBackend struct {
	store *Store
}

// NewLocalBackend creates a new local filesystem cache backend.
func NewLocalBackend(projectRoot string) *LocalBackend {
	return &LocalBackend{store: NewStore(projectRoot)}
}

func (b *LocalBackend) Check(_ context.Context, taskName, key string) (bool, error) {
	return b.store.Exists(taskName, key), nil
}

func (b *LocalBackend) Restore(_ context.Context, taskName, key string) error {
	return b.store.Restore(taskName, key)
}

func (b *LocalBackend) Save(_ context.Context, taskName, key string, outputPatterns []string, meta Metadata) error {
	return b.store.Save(taskName, key, outputPatterns, meta)
}

func (b *LocalBackend) Clear(_ context.Context, taskName string) error {
	return b.store.Clear(taskName)
}

func (b *LocalBackend) ClearAll(_ context.Context) error {
	return b.store.ClearAll()
}

// Store returns the underlying Store for direct access (e.g., metadata).
func (b *LocalBackend) Store() *Store {
	return b.store
}
