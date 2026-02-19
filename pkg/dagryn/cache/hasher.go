package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/mujhtech/dagryn/pkg/dagryn/task"
)

// hashCache memoizes per-file hash computations within a single CLI run.
// Since dagryn is a short-lived CLI process, there is no stale-data concern.
// The mod-time check ensures correctness when a task modifies files between
// cache check and cache save.
var hashCache = struct {
	mu sync.RWMutex

	// files maps absolute path → (hex hash, mod-time).
	// Re-hashing is skipped when mod-time hasn't changed.
	files map[string]fileHashEntry
}{
	files: make(map[string]fileHashEntry, 512),
}

type fileHashEntry struct {
	hash    string
	modTime int64 // UnixNano
}

// HashTask computes a cache key for a task based on its inputs.
func HashTask(t *task.Task, projectRoot string) (string, error) {
	h := sha256.New()

	// Hash task name
	h.Write([]byte(t.Name))
	h.Write([]byte{0})

	// Hash command
	h.Write([]byte(t.Command))
	h.Write([]byte{0})

	// Hash environment variables (sorted for determinism)
	if len(t.Env) > 0 {
		envKeys := make([]string, 0, len(t.Env))
		for k := range t.Env {
			envKeys = append(envKeys, k)
		}
		sort.Strings(envKeys)
		for _, k := range envKeys {
			h.Write([]byte(k))
			h.Write([]byte("="))
			h.Write([]byte(t.Env[k]))
			h.Write([]byte{0})
		}
	}

	// Hash workdir
	h.Write([]byte(t.Workdir))
	h.Write([]byte{0})

	// Hash input files — resolve patterns relative to workdir when set
	if len(t.Inputs) > 0 {
		inputRoot := projectRoot
		if t.Workdir != "" {
			inputRoot = filepath.Join(projectRoot, t.Workdir)
		}
		inputHash, err := HashFiles(t.Inputs, inputRoot)
		if err != nil {
			return "", fmt.Errorf("failed to hash input files: %w", err)
		}
		h.Write([]byte(inputHash))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFiles computes a hash of files matching the given glob patterns.
// Individual file hashes are memoized (keyed by path + mod-time) so that
// tasks sharing the same input files (e.g. five Go tasks with "**/*.go")
// only read and SHA256 each file once.
func HashFiles(patterns []string, root string) (string, error) {
	h := sha256.New()

	// Collect all matching files (already deduplicated and sorted)
	files, err := ResolveFilePatterns(root, patterns)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file patterns: %w", err)
	}

	// Hash each file (per-file results are memoized)
	for _, file := range files {
		// Hash relative path
		relPath, _ := filepath.Rel(root, file)
		h.Write([]byte(relPath))
		h.Write([]byte{0})

		// Hash file contents (memoized per file + mod-time)
		fileHash, err := hashFileCached(file)
		if err != nil {
			return "", fmt.Errorf("failed to hash file %q: %w", file, err)
		}
		h.Write([]byte(fileHash))
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFileCached returns the SHA256 of a file, using a memoized result
// when the file's modification time hasn't changed.
func hashFileCached(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	modTime := info.ModTime().UnixNano()

	hashCache.mu.RLock()
	if entry, ok := hashCache.files[path]; ok && entry.modTime == modTime {
		hashCache.mu.RUnlock()
		return entry.hash, nil
	}
	hashCache.mu.RUnlock()

	hash, err := hashFile(path)
	if err != nil {
		return "", err
	}

	hashCache.mu.Lock()
	hashCache.files[path] = fileHashEntry{hash: hash, modTime: modTime}
	hashCache.mu.Unlock()

	return hash, nil
}

// hashFile computes the SHA256 hash of a file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashString computes the SHA256 hash of a string.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
