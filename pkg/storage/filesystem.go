package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemBucket implements Bucket using the local filesystem.
type FilesystemBucket struct {
	root   string
	prefix string
}

// NewFilesystemBucket creates a filesystem-backed Bucket rooted at basePath.
func NewFilesystemBucket(basePath, prefix string) (*FilesystemBucket, error) {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("storage/filesystem: invalid base path: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("storage/filesystem: failed to create base directory: %w", err)
	}
	return &FilesystemBucket{root: abs, prefix: prefix}, nil
}

func (b *FilesystemBucket) resolve(key string) string {
	return filepath.Join(b.root, filepath.FromSlash(b.prefix+key))
}

func (b *FilesystemBucket) Put(_ context.Context, key string, r io.Reader, _ *PutOptions) error {
	p := b.resolve(key)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("storage/filesystem: mkdir: %w", err)
	}
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("storage/filesystem: create: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage/filesystem: write: %w", err)
	}
	return f.Close()
}

func (b *FilesystemBucket) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(b.resolve(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage/filesystem: open: %w", err)
	}
	return f, nil
}

func (b *FilesystemBucket) Delete(_ context.Context, key string) error {
	err := os.Remove(b.resolve(key))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage/filesystem: remove: %w", err)
	}
	return nil
}

func (b *FilesystemBucket) Exists(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(b.resolve(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("storage/filesystem: stat: %w", err)
}

func (b *FilesystemBucket) List(_ context.Context, prefix string, opts *ListOptions) (*ListResult, error) {
	fullPrefix := b.prefix + prefix
	searchRoot := filepath.Join(b.root, filepath.FromSlash(fullPrefix))

	// If the search root is a file, its parent dir should be walked
	info, err := os.Stat(searchRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("storage/filesystem: stat: %w", err)
	}
	if err != nil || !info.IsDir() {
		searchRoot = filepath.Dir(searchRoot)
	}

	maxKeys := 1000
	if opts != nil && opts.MaxKeys > 0 {
		maxKeys = opts.MaxKeys
	}

	result := &ListResult{}
	err = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(b.root, path)
		if err != nil {
			return nil
		}
		key := filepath.ToSlash(rel)
		// Strip the bucket prefix to return the logical key
		if b.prefix != "" {
			key = strings.TrimPrefix(key, b.prefix)
		}
		if !strings.HasPrefix(b.prefix+key, fullPrefix) {
			return nil
		}
		if len(result.Keys) >= maxKeys {
			result.IsTruncated = true
			return filepath.SkipAll
		}
		result.Keys = append(result.Keys, key)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("storage/filesystem: walk: %w", err)
	}
	return result, nil
}
