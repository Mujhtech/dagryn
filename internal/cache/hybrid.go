package cache

import (
	"context"
	"fmt"
)

// Strategy controls how the hybrid backend prioritizes local vs remote.
type Strategy string

const (
	// StrategyLocalFirst checks local cache first, falls back to remote.
	StrategyLocalFirst Strategy = "local-first"
	// StrategyRemoteFirst checks remote cache first, falls back to local.
	StrategyRemoteFirst Strategy = "remote-first"
	// StrategyWriteThrough writes to both local and remote on save,
	// and reads from local first (same as local-first for reads).
	StrategyWriteThrough Strategy = "write-through"
)

// HybridConfig configures the HybridBackend behavior.
type HybridConfig struct {
	Strategy        Strategy
	FallbackOnError bool // when true, remote errors are non-fatal
}

// DefaultHybridConfig returns sensible defaults.
func DefaultHybridConfig() HybridConfig {
	return HybridConfig{
		Strategy:        StrategyLocalFirst,
		FallbackOnError: true,
	}
}

// HybridBackend combines a local and remote Backend.
type HybridBackend struct {
	local  Backend
	remote Backend
	cfg    HybridConfig
}

// NewHybridBackend creates a backend that combines local and remote caches.
func NewHybridBackend(local, remote Backend, cfg HybridConfig) *HybridBackend {
	return &HybridBackend{
		local:  local,
		remote: remote,
		cfg:    cfg,
	}
}

func (h *HybridBackend) Check(ctx context.Context, taskName, key string) (bool, error) {
	switch h.cfg.Strategy {
	case StrategyRemoteFirst:
		return h.checkRemoteFirst(ctx, taskName, key)
	default: // local-first, write-through
		return h.checkLocalFirst(ctx, taskName, key)
	}
}

func (h *HybridBackend) checkLocalFirst(ctx context.Context, taskName, key string) (bool, error) {
	hit, err := h.local.Check(ctx, taskName, key)
	if err != nil {
		return false, err
	}
	if hit {
		return true, nil
	}

	hit, err = h.remote.Check(ctx, taskName, key)
	if err != nil {
		if h.cfg.FallbackOnError {
			return false, nil
		}
		return false, fmt.Errorf("hybrid cache: remote check: %w", err)
	}
	return hit, nil
}

func (h *HybridBackend) checkRemoteFirst(ctx context.Context, taskName, key string) (bool, error) {
	hit, err := h.remote.Check(ctx, taskName, key)
	if err != nil {
		if h.cfg.FallbackOnError {
			// Fall back to local
			return h.local.Check(ctx, taskName, key)
		}
		return false, fmt.Errorf("hybrid cache: remote check: %w", err)
	}
	if hit {
		return true, nil
	}

	return h.local.Check(ctx, taskName, key)
}

func (h *HybridBackend) Restore(ctx context.Context, taskName, key string) error {
	switch h.cfg.Strategy {
	case StrategyRemoteFirst:
		return h.restoreRemoteFirst(ctx, taskName, key)
	default: // local-first, write-through
		return h.restoreLocalFirst(ctx, taskName, key)
	}
}

func (h *HybridBackend) restoreLocalFirst(ctx context.Context, taskName, key string) error {
	localHit, _ := h.local.Check(ctx, taskName, key)
	if localHit {
		return h.local.Restore(ctx, taskName, key)
	}

	// Restore from remote
	if err := h.remote.Restore(ctx, taskName, key); err != nil {
		if h.cfg.FallbackOnError {
			return fmt.Errorf("hybrid cache: remote restore failed: %w", err)
		}
		return err
	}

	// Warm local cache: the files are now on disk after remote restore,
	// so we can save them into the local backend using the same key.
	// We pass nil output patterns — the local backend's Save will see
	// no files to copy and just write metadata, which is enough to make
	// local Check() return true. But we can do better: we have the
	// remote backend that already placed files, so we tell local to
	// "check" and it will miss, then on next Save (from scheduler) it
	// gets populated. For immediate warming we propagate the entry.
	_ = h.warmLocal(ctx, taskName, key)

	return nil
}

func (h *HybridBackend) restoreRemoteFirst(ctx context.Context, taskName, key string) error {
	remoteHit, remoteErr := h.remote.Check(ctx, taskName, key)
	if remoteErr != nil && !h.cfg.FallbackOnError {
		return fmt.Errorf("hybrid cache: remote check: %w", remoteErr)
	}

	if remoteHit {
		if err := h.remote.Restore(ctx, taskName, key); err != nil {
			if h.cfg.FallbackOnError {
				// Try local as fallback
				return h.local.Restore(ctx, taskName, key)
			}
			return err
		}
		_ = h.warmLocal(ctx, taskName, key)
		return nil
	}

	// Fall back to local
	return h.local.Restore(ctx, taskName, key)
}

// warmLocal copies the cache entry from remote into local so future lookups
// are fast. The remote restore already wrote files to the project root, so
// the local backend can re-save them with empty output patterns (just
// creates the metadata marker).
func (h *HybridBackend) warmLocal(ctx context.Context, taskName, key string) error {
	meta := Metadata{
		TaskName: taskName,
		CacheKey: key,
	}
	// Save with nil patterns — the LocalBackend.Save will create the
	// metadata marker which is enough for Check() to return true.
	return h.local.Save(ctx, taskName, key, nil, meta)
}

func (h *HybridBackend) Save(ctx context.Context, taskName, key string, outputPatterns []string, meta Metadata) error {
	switch h.cfg.Strategy {
	case StrategyRemoteFirst:
		return h.saveRemoteFirst(ctx, taskName, key, outputPatterns, meta)
	default: // local-first, write-through
		return h.saveLocalFirst(ctx, taskName, key, outputPatterns, meta)
	}
}

func (h *HybridBackend) saveLocalFirst(ctx context.Context, taskName, key string, outputPatterns []string, meta Metadata) error {
	// Save locally first
	if err := h.local.Save(ctx, taskName, key, outputPatterns, meta); err != nil {
		return err
	}

	// Then save to remote
	if err := h.remote.Save(ctx, taskName, key, outputPatterns, meta); err != nil {
		if h.cfg.FallbackOnError {
			return nil
		}
		return fmt.Errorf("hybrid cache: remote save: %w", err)
	}

	return nil
}

func (h *HybridBackend) saveRemoteFirst(ctx context.Context, taskName, key string, outputPatterns []string, meta Metadata) error {
	// Save to remote first
	if err := h.remote.Save(ctx, taskName, key, outputPatterns, meta); err != nil {
		if h.cfg.FallbackOnError {
			// Fall back to local-only save
			return h.local.Save(ctx, taskName, key, outputPatterns, meta)
		}
		return fmt.Errorf("hybrid cache: remote save: %w", err)
	}

	// Then save locally
	if err := h.local.Save(ctx, taskName, key, outputPatterns, meta); err != nil {
		return err
	}

	return nil
}

func (h *HybridBackend) Clear(ctx context.Context, taskName string) error {
	if err := h.local.Clear(ctx, taskName); err != nil {
		return err
	}
	if err := h.remote.Clear(ctx, taskName); err != nil {
		if h.cfg.FallbackOnError {
			return nil
		}
		return fmt.Errorf("hybrid cache: remote clear: %w", err)
	}
	return nil
}

func (h *HybridBackend) ClearAll(ctx context.Context) error {
	if err := h.local.ClearAll(ctx); err != nil {
		return err
	}
	if err := h.remote.ClearAll(ctx); err != nil {
		if h.cfg.FallbackOnError {
			return nil
		}
		return fmt.Errorf("hybrid cache: remote clear all: %w", err)
	}
	return nil
}

// Verify interface compliance at compile time.
var _ Backend = (*HybridBackend)(nil)
