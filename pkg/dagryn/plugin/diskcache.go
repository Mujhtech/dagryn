package plugin

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DiskCache provides TTL-based file caching for API responses.
type DiskCache struct {
	baseDir  string
	disabled bool
}

type cacheEntry struct {
	Data     json.RawMessage `json:"data"`
	CachedAt time.Time       `json:"cached_at"`
	TTLSec   int64           `json:"ttl_sec"`
}

// NewDiskCache creates a new disk cache at the given directory.
func NewDiskCache(baseDir string) *DiskCache {
	return &DiskCache{baseDir: baseDir}
}

// Disable turns off caching (reads always miss, writes are no-ops).
func (c *DiskCache) Disable() {
	c.disabled = true
}

// Get returns cached data for the key, or nil on miss/expired/disabled.
func (c *DiskCache) Get(key string) ([]byte, error) {
	if c.disabled {
		return nil, nil
	}

	path := c.keyPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // miss
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Corrupted cache file, remove it
		_ = os.Remove(path)
		return nil, nil
	}

	// Check TTL
	if time.Since(entry.CachedAt) > time.Duration(entry.TTLSec)*time.Second {
		_ = os.Remove(path)
		return nil, nil
	}

	return entry.Data, nil
}

// Set stores data in the cache with the given TTL.
func (c *DiskCache) Set(key string, data []byte, ttl time.Duration) error {
	if c.disabled {
		return nil
	}

	path := c.keyPath(key)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	entry := cacheEntry{
		Data:     data,
		CachedAt: time.Now(),
		TTLSec:   int64(ttl.Seconds()),
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(path, encoded, 0644)
}

// keyPath returns the file path for a cache key.
// Keys are hashed to avoid filesystem path issues.
func (c *DiskCache) keyPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	// Use the original key structure as subdirectory for readability,
	// but use hash as the filename to avoid path issues
	dir := filepath.Dir(key)
	name := fmt.Sprintf("%x.json", hash[:8])
	return filepath.Join(c.baseDir, dir, name)
}
