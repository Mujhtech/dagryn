package cache

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

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

		if strings.Contains(pattern, "**") {
			files, err = resolveDoublestar(root, pattern)
		} else {
			files, err = resolveStandard(root, pattern)
		}
		if err != nil {
			return nil, err
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

// resolveDoublestar handles patterns with ** using doublestar.Glob with os.DirFS.
func resolveDoublestar(root, pattern string) ([]string, error) {
	fsys := os.DirFS(root)
	matches, err := doublestar.Glob(fsys, pattern, doublestar.WithFilesOnly())
	if err != nil {
		return nil, err
	}

	abs := make([]string, len(matches))
	for i, m := range matches {
		abs[i] = filepath.Join(root, m)
	}
	return abs, nil
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
