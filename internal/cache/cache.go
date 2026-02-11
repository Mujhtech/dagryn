package cache

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mujhtech/dagryn/internal/task"
)

// Cache provides high-level caching operations.
type Cache struct {
	backend Backend
	store   *Store // retained for GetStore() backward compat
	enabled bool
	root    string // project root directory
}

// New creates a new cache instance with the default local backend.
func New(projectRoot string, enabled bool) *Cache {
	lb := NewLocalBackend(projectRoot)
	return &Cache{
		backend: lb,
		store:   lb.Store(),
		enabled: enabled,
		root:    projectRoot,
	}
}

// NewWithBackend creates a cache instance using the supplied backend.
func NewWithBackend(projectRoot string, enabled bool, backend Backend) *Cache {
	c := &Cache{
		backend: backend,
		enabled: enabled,
		root:    projectRoot,
	}
	// If the backend is a LocalBackend, expose its Store for backward compat
	if lb, ok := backend.(*LocalBackend); ok {
		c.store = lb.Store()
	}
	return c
}

// IsEnabled returns whether caching is enabled.
func (c *Cache) IsEnabled() bool {
	return c.enabled
}

// Check checks if a task has a valid cache entry.
// Returns (hit, cacheKey, error).
func (c *Cache) Check(ctx context.Context, t *task.Task) (bool, string, error) {
	if !c.enabled {
		return false, "", nil
	}

	// Tasks without inputs or outputs are not cached
	if !t.HasInputs() && !t.HasOutputs() {
		return false, "", nil
	}

	key, err := HashTask(t, c.projectRoot())
	if err != nil {
		return false, "", fmt.Errorf("failed to compute cache key: %w", err)
	}

	hit, err := c.backend.Check(ctx, t.Name, key)
	if err != nil {
		return false, key, err
	}

	return hit, key, nil
}

// Restore restores cached outputs for a task.
func (c *Cache) Restore(ctx context.Context, t *task.Task, key string) error {
	if !c.enabled {
		return nil
	}

	return c.backend.Restore(ctx, t.Name, key)
}

// Save saves task outputs to the cache.
func (c *Cache) Save(ctx context.Context, t *task.Task, key string, duration time.Duration) error {
	if !c.enabled {
		return nil
	}

	// Only cache tasks with outputs
	if !t.HasOutputs() {
		return nil
	}

	meta := Metadata{
		TaskName:  t.Name,
		CacheKey:  key,
		CreatedAt: time.Now(),
		Duration:  duration,
	}

	// Resolve output patterns relative to task workdir so that
	// "node_modules/**" with workdir="web" becomes "web/node_modules/**"
	outputs := t.Outputs
	if t.Workdir != "" {
		outputs = make([]string, len(t.Outputs))
		for i, p := range t.Outputs {
			outputs[i] = filepath.Join(t.Workdir, p)
		}
	}

	return c.backend.Save(ctx, t.Name, key, outputs, meta)
}

// Clear removes all cached data for a task.
func (c *Cache) Clear(ctx context.Context, taskName string) error {
	return c.backend.Clear(ctx, taskName)
}

// ClearAll removes all cached data.
func (c *Cache) ClearAll(ctx context.Context) error {
	return c.backend.ClearAll(ctx)
}

// GetStore returns the underlying store (available only for local backends).
func (c *Cache) GetStore() *Store {
	return c.store
}

// GetBackend returns the underlying backend.
func (c *Cache) GetBackend() Backend {
	return c.backend
}

// projectRoot returns the project root directory.
func (c *Cache) projectRoot() string {
	return c.root
}
