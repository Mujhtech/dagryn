package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/mujhtech/dagryn/internal/task"
)

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
func HashFiles(patterns []string, root string) (string, error) {
	h := sha256.New()

	// Collect all matching files
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return "", fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	// Sort for determinism
	sort.Strings(files)

	// Hash each file
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue // Skip files that don't exist
		}
		if info.IsDir() {
			continue // Skip directories
		}

		// Hash relative path
		relPath, _ := filepath.Rel(root, file)
		h.Write([]byte(relPath))
		h.Write([]byte{0})

		// Hash file contents
		fileHash, err := hashFile(file)
		if err != nil {
			return "", fmt.Errorf("failed to hash file %q: %w", file, err)
		}
		h.Write([]byte(fileHash))
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
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
