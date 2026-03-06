package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mujhtech/dagryn/pkg/dagryn/cache"
	"github.com/mujhtech/dagryn/pkg/storage"
)

// StorageBackend implements cache.Backend using a storage.Bucket.
type StorageBackend struct {
	bucket      storage.Bucket
	projectRoot string
}

// NewStorageBackend creates a remote cache backend backed by the given bucket.
func NewStorageBackend(bucket storage.Bucket, projectRoot string) *StorageBackend {
	return &StorageBackend{bucket: bucket, projectRoot: projectRoot}
}

func (b *StorageBackend) Check(ctx context.Context, taskName, key string) (bool, error) {
	return b.bucket.Exists(ctx, ActionKey(taskName, key))
}

func (b *StorageBackend) Restore(ctx context.Context, taskName, key string) error {
	// Download manifest
	rc, err := b.bucket.Get(ctx, ActionKey(taskName, key))
	if err != nil {
		if storage.IsNotFound(err) {
			return fmt.Errorf("remote cache not found for task %q key %q", taskName, key)
		}
		return fmt.Errorf("remote cache: get manifest: %w", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("remote cache: read manifest: %w", err)
	}

	manifest, err := UnmarshalManifest(data)
	if err != nil {
		return fmt.Errorf("remote cache: parse manifest: %w", err)
	}

	// Restore each file
	for relPath, digest := range manifest.Files {
		if err := b.restoreFile(ctx, relPath, digest); err != nil {
			return fmt.Errorf("remote cache: restore %q: %w", relPath, err)
		}
	}
	return nil
}

func (b *StorageBackend) restoreFile(ctx context.Context, relPath string, digest *Digest) error {
	rc, err := b.bucket.Get(ctx, digest.Key())
	if err != nil {
		return fmt.Errorf("get blob: %w", err)
	}
	defer func() { _ = rc.Close() }()

	dest := filepath.Join(b.projectRoot, relPath)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return f.Close()
}

func (b *StorageBackend) Save(ctx context.Context, taskName, key string, outputPatterns []string, _ cache.Metadata) error {
	manifest := &Manifest{Files: make(map[string]*Digest)}

	// Resolve output patterns and upload each file
	files, err := cache.ResolveFilePatterns(b.projectRoot, outputPatterns)
	if err != nil {
		return fmt.Errorf("remote cache: resolve patterns: %w", err)
	}
	for _, src := range files {
		relPath, err := filepath.Rel(b.projectRoot, src)
		if err != nil {
			continue
		}

		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}

		digest := DigestBytes(data)

		// Upload to CAS if not already present
		exists, err := b.bucket.Exists(ctx, digest.Key())
		if err != nil {
			return fmt.Errorf("remote cache: check CAS %q: %w", digest.Key(), err)
		}
		if !exists {
			if err := b.bucket.Put(ctx, digest.Key(), bytes.NewReader(data), nil); err != nil {
				return fmt.Errorf("remote cache: upload CAS %q: %w", digest.Key(), err)
			}
		}

		manifest.Files[relPath] = &digest
	}

	// Upload manifest
	manifestData, err := MarshalManifest(manifest)
	if err != nil {
		return fmt.Errorf("remote cache: marshal manifest: %w", err)
	}
	if err := b.bucket.Put(ctx, ActionKey(taskName, key), bytes.NewReader(manifestData), nil); err != nil {
		return fmt.Errorf("remote cache: upload manifest: %w", err)
	}

	return nil
}

func (b *StorageBackend) Clear(ctx context.Context, taskName string) error {
	prefix := fmt.Sprintf("ac/%s/", taskName)
	result, err := b.bucket.List(ctx, prefix, nil)
	if err != nil {
		return fmt.Errorf("remote cache: list %q: %w", prefix, err)
	}
	for _, key := range result.Keys {
		if err := b.bucket.Delete(ctx, key); err != nil {
			return fmt.Errorf("remote cache: delete %q: %w", key, err)
		}
	}
	return nil
}

func (b *StorageBackend) ClearAll(ctx context.Context) error {
	// Clear all action cache entries
	result, err := b.bucket.List(ctx, "ac/", nil)
	if err != nil {
		return fmt.Errorf("remote cache: list ac/: %w", err)
	}
	for _, key := range result.Keys {
		if err := b.bucket.Delete(ctx, key); err != nil {
			return fmt.Errorf("remote cache: delete %q: %w", key, err)
		}
	}
	return nil
}

// Verify interface compliance at compile time.
var _ cache.Backend = (*StorageBackend)(nil)
