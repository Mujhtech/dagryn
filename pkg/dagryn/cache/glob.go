package cache

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

// defaultSkipDirs are directories that should be skipped when resolving
// broad recursive glob patterns (e.g. "**/*.go"). They contain metadata,
// dependencies, or VCS data that should not be hashed as source inputs.
//
// These are only applied when the pattern does NOT explicitly name the
// directory (e.g. "node_modules/**" will still resolve node_modules).
var defaultSkipDirs = map[string]struct{}{
	".dagryn":      {},
	".git":         {},
	"node_modules": {},
	"vendor":       {},
	".next":        {},
	"__pycache__":  {},
}

// globCache memoizes individual glob pattern resolution within a single run.
// Key: "root\x00pattern", Value: list of absolute file paths.
// This avoids walking the same directory tree multiple times when many tasks
// share the same input patterns (e.g. five Go tasks with "**/*.go").
var globCache = struct {
	mu    sync.RWMutex
	items map[string][]string
}{
	items: make(map[string][]string, 32),
}

// ResolveFilePatterns resolves glob patterns relative to root and returns
// deduplicated, sorted absolute file paths (never directories).
// Patterns containing "**" use doublestar for recursive matching.
// Patterns without "**" use filepath.Glob, walking any directory matches
// to collect individual files.
func ResolveFilePatterns(root string, patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	var result []string

	for _, pattern := range patterns {
		var files []string
		var err error

		// Check glob cache first
		cacheKey := root + "\x00" + pattern
		globCache.mu.RLock()
		cached, ok := globCache.items[cacheKey]
		globCache.mu.RUnlock()

		if ok {
			files = cached
		} else {
			if strings.Contains(pattern, "**") {
				files, err = resolveDoublestar(root, pattern)
			} else {
				files, err = resolveStandard(root, pattern)
			}
			if err != nil {
				return nil, err
			}

			// Cache the result for this pattern
			globCache.mu.Lock()
			globCache.items[cacheKey] = files
			globCache.mu.Unlock()
		}

		for _, f := range files {
			if _, ok := seen[f]; !ok {
				seen[f] = struct{}{}
				result = append(result, f)
			}
		}
	}

	sort.Strings(result)
	return result, nil
}

// patternExplicitRoot returns the first path component of a glob pattern,
// or "" if the pattern starts with "**" (i.e., matches from the root).
// For example:
//
//	"node_modules/**"  → "node_modules"
//	"src/**/*.go"      → "src"
//	"**/*.go"          → ""
func patternExplicitRoot(pattern string) string {
	parts := strings.SplitN(filepath.ToSlash(pattern), "/", 2)
	if len(parts) == 0 || parts[0] == "**" {
		return ""
	}
	// If the first segment contains wildcards, treat it as broad.
	if strings.ContainsAny(parts[0], "*?[") {
		return ""
	}
	return parts[0]
}

// resolveDoublestar handles patterns with ** using doublestar.Glob with os.DirFS.
// For broad patterns (rooted at **), matches inside default skip directories
// are filtered out to avoid hashing cached files, node_modules, etc.
// For targeted patterns (e.g. "node_modules/**"), no filtering is applied.
func resolveDoublestar(root, pattern string) ([]string, error) {
	fsys := os.DirFS(root)
	matches, err := doublestar.Glob(fsys, pattern, doublestar.WithFilesOnly())
	if err != nil {
		return nil, err
	}

	// Only filter when the pattern is a broad recursive scan.
	explicitRoot := patternExplicitRoot(pattern)
	shouldFilter := explicitRoot == "" || isSkipDir(explicitRoot)

	// If the pattern explicitly names a skip dir (e.g. "node_modules/**"),
	// do NOT filter — the user intentionally wants those files.
	if explicitRoot != "" {
		shouldFilter = false
	}

	if !shouldFilter {
		abs := make([]string, len(matches))
		for i, m := range matches {
			abs[i] = filepath.Join(root, m)
		}
		return abs, nil
	}

	filtered := make([]string, 0, len(matches))
	for _, m := range matches {
		if isInsideSkipDir(m) {
			continue
		}
		filtered = append(filtered, filepath.Join(root, m))
	}
	return filtered, nil
}

// resolveStandard handles patterns without ** using filepath.Glob,
// walking any matched directories to collect individual files.
func resolveStandard(root, pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return nil, err
	}

	var files []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.IsDir() {
			err = filepath.Walk(m, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !fi.IsDir() {
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				continue
			}
		} else {
			files = append(files, m)
		}
	}
	return files, nil
}

// isSkipDir returns true if the directory name is in the skip list.
func isSkipDir(name string) bool {
	_, ok := defaultSkipDirs[name]
	return ok
}

// isInsideSkipDir returns true if the relative path starts with or contains
// a directory that should be excluded from glob results.
func isInsideSkipDir(relPath string) bool {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		if _, ok := defaultSkipDirs[part]; ok {
			return true
		}
	}
	return false
}
