package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	// CacheDir is the directory name for cache storage.
	CacheDir = ".dagryn"
	// CacheSubDir is the subdirectory for cached outputs.
	CacheSubDir = "cache"
)

// Metadata contains information about a cached task.
type Metadata struct {
	TaskName  string        `json:"task_name"`
	CacheKey  string        `json:"cache_key"`
	CreatedAt time.Time     `json:"created_at"`
	Duration  time.Duration `json:"duration"`
	InputHash string        `json:"input_hash"`
	Outputs   []string      `json:"outputs"`
}

// Store manages the cache storage.
type Store struct {
	root string // Project root directory
}

// NewStore creates a new cache store.
func NewStore(projectRoot string) *Store {
	return &Store{
		root: projectRoot,
	}
}

// CachePath returns the path to the cache directory.
func (s *Store) CachePath() string {
	return filepath.Join(s.root, CacheDir, CacheSubDir)
}

// TaskCachePath returns the path to a task's cache directory.
func (s *Store) TaskCachePath(taskName, key string) string {
	return filepath.Join(s.CachePath(), taskName, key)
}

// MetadataPath returns the path to the metadata file for a cached task.
func (s *Store) MetadataPath(taskName, key string) string {
	return filepath.Join(s.TaskCachePath(taskName, key), "metadata.json")
}

// OutputsPath returns the path to the outputs directory for a cached task.
func (s *Store) OutputsPath(taskName, key string) string {
	return filepath.Join(s.TaskCachePath(taskName, key), "outputs")
}

// Exists checks if a cache entry exists.
func (s *Store) Exists(taskName, key string) bool {
	metaPath := s.MetadataPath(taskName, key)
	_, err := os.Stat(metaPath)
	return err == nil
}

// Save saves task outputs to the cache.
func (s *Store) Save(taskName, key string, outputPatterns []string, meta Metadata) error {
	cachePath := s.TaskCachePath(taskName, key)
	outputsPath := s.OutputsPath(taskName, key)

	// Create cache directory
	if err := os.MkdirAll(outputsPath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Copy output files
	var savedOutputs []string
	for _, pattern := range outputPatterns {
		matches, err := filepath.Glob(filepath.Join(s.root, pattern))
		if err != nil {
			continue
		}
		for _, src := range matches {
			relPath, err := filepath.Rel(s.root, src)
			if err != nil {
				continue
			}
			dest := filepath.Join(outputsPath, relPath)
			if err := copyFile(src, dest); err != nil {
				// Log but don't fail - some outputs might not exist
				continue
			}
			savedOutputs = append(savedOutputs, relPath)
		}
	}

	// Save metadata
	meta.Outputs = savedOutputs
	metaPath := filepath.Join(cachePath, "metadata.json")
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// Restore restores cached outputs to the project directory.
func (s *Store) Restore(taskName, key string) error {
	outputsPath := s.OutputsPath(taskName, key)

	// Check if cache exists
	if _, err := os.Stat(outputsPath); os.IsNotExist(err) {
		return fmt.Errorf("cache not found for task %q with key %q", taskName, key)
	}

	// Walk and copy all cached files
	err := filepath.Walk(outputsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(outputsPath, path)
		if err != nil {
			return err
		}

		dest := filepath.Join(s.root, relPath)
		return copyFile(path, dest)
	})

	if err != nil {
		return fmt.Errorf("failed to restore cache: %w", err)
	}

	return nil
}

// GetMetadata retrieves metadata for a cached task.
func (s *Store) GetMetadata(taskName, key string) (*Metadata, error) {
	metaPath := s.MetadataPath(taskName, key)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// Clear removes all cached data for a task.
func (s *Store) Clear(taskName string) error {
	taskPath := filepath.Join(s.CachePath(), taskName)
	return os.RemoveAll(taskPath)
}

// ClearAll removes all cached data.
func (s *Store) ClearAll() error {
	return os.RemoveAll(s.CachePath())
}

// CacheEntry represents a discovered local cache entry.
type CacheEntry struct {
	TaskName string
	CacheKey string
}

// ListEntries returns all cache entries, optionally filtered to a single task.
// It walks the .dagryn/cache/ directory looking for metadata.json files.
func (s *Store) ListEntries(taskFilter string) ([]CacheEntry, error) {
	cachePath := s.CachePath()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, nil
	}

	var entries []CacheEntry

	taskDirs, err := os.ReadDir(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, taskDir := range taskDirs {
		if !taskDir.IsDir() {
			continue
		}
		taskName := taskDir.Name()
		if taskFilter != "" && taskName != taskFilter {
			continue
		}

		keyDirs, err := os.ReadDir(filepath.Join(cachePath, taskName))
		if err != nil {
			continue
		}
		for _, keyDir := range keyDirs {
			if !keyDir.IsDir() {
				continue
			}
			// Verify metadata.json exists
			metaPath := filepath.Join(cachePath, taskName, keyDir.Name(), "metadata.json")
			if _, err := os.Stat(metaPath); err == nil {
				entries = append(entries, CacheEntry{
					TaskName: taskName,
					CacheKey: keyDir.Name(),
				})
			}
		}
	}
	return entries, nil
}

// Root returns the project root.
func (s *Store) Root() string {
	return s.root
}

// copyFile copies a file from src to dest, creating directories as needed.
func copyFile(src, dest string) error {
	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, srcFile)
	return err
}
