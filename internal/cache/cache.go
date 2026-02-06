package cache

import (
	"fmt"
	"time"

	"github.com/mujhtech/dagryn/internal/task"
)

// Cache provides high-level caching operations.
type Cache struct {
	store   *Store
	enabled bool
}

// New creates a new cache instance.
func New(projectRoot string, enabled bool) *Cache {
	return &Cache{
		store:   NewStore(projectRoot),
		enabled: enabled,
	}
}

// IsEnabled returns whether caching is enabled.
func (c *Cache) IsEnabled() bool {
	return c.enabled
}

// Check checks if a task has a valid cache entry.
// Returns (hit, cacheKey, error).
func (c *Cache) Check(t *task.Task) (bool, string, error) {
	if !c.enabled {
		return false, "", nil
	}

	// Tasks without inputs or outputs are not cached
	if !t.HasInputs() && !t.HasOutputs() {
		return false, "", nil
	}

	key, err := HashTask(t, c.store.root)
	if err != nil {
		return false, "", fmt.Errorf("failed to compute cache key: %w", err)
	}

	if c.store.Exists(t.Name, key) {
		return true, key, nil
	}

	return false, key, nil
}

// Restore restores cached outputs for a task.
func (c *Cache) Restore(t *task.Task, key string) error {
	if !c.enabled {
		return nil
	}

	return c.store.Restore(t.Name, key)
}

// Save saves task outputs to the cache.
func (c *Cache) Save(t *task.Task, key string, duration time.Duration) error {
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

	return c.store.Save(t.Name, key, t.Outputs, meta)
}

// Clear removes all cached data for a task.
func (c *Cache) Clear(taskName string) error {
	return c.store.Clear(taskName)
}

// ClearAll removes all cached data.
func (c *Cache) ClearAll() error {
	return c.store.ClearAll()
}

// GetStore returns the underlying store.
func (c *Cache) GetStore() *Store {
	return c.store
}
