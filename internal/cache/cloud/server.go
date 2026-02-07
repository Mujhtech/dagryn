package cloud

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/cache"
	"github.com/mujhtech/dagryn/internal/service"
)

// ServerBackend implements cache.Backend by calling CacheService directly
// (in-process). Used by the worker's ExecuteRun handler so server-side runs
// get cloud cache without an HTTP round-trip.
type ServerBackend struct {
	svc         *service.CacheService
	projectID   uuid.UUID
	projectRoot string
}

// NewServerBackend creates a server-side cloud cache backend.
func NewServerBackend(svc *service.CacheService, projectID uuid.UUID, projectRoot string) *ServerBackend {
	return &ServerBackend{
		svc:         svc,
		projectID:   projectID,
		projectRoot: projectRoot,
	}
}

func (b *ServerBackend) Check(ctx context.Context, taskName, key string) (bool, error) {
	return b.svc.Check(ctx, b.projectID, taskName, key)
}

func (b *ServerBackend) Restore(ctx context.Context, taskName, key string) error {
	rc, err := b.svc.Download(ctx, b.projectID, taskName, key)
	if err != nil {
		return fmt.Errorf("server cache download: %w", err)
	}
	defer func() { _ = rc.Close() }()

	if err := extractArchive(b.projectRoot, rc); err != nil {
		return fmt.Errorf("server cache extract: %w", err)
	}
	return nil
}

func (b *ServerBackend) Save(ctx context.Context, taskName, key string, outputPatterns []string, _ cache.Metadata) error {
	if len(outputPatterns) == 0 {
		return nil
	}

	archive, err := createArchive(b.projectRoot, outputPatterns)
	if err != nil {
		return fmt.Errorf("server cache archive: %w", err)
	}
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archive.Name())
	}()

	info, err := archive.Stat()
	if err != nil {
		return fmt.Errorf("server cache stat archive: %w", err)
	}

	// Read into memory so the service can tee-read for hashing.
	// The archive is already a temp file, so we just pass it as io.Reader.
	data, err := io.ReadAll(archive)
	if err != nil {
		return fmt.Errorf("server cache read archive: %w", err)
	}

	if err := b.svc.Upload(ctx, b.projectID, taskName, key, bytes.NewReader(data), info.Size()); err != nil {
		return fmt.Errorf("server cache upload: %w", err)
	}
	return nil
}

func (b *ServerBackend) Clear(_ context.Context, _ string) error {
	return nil
}

func (b *ServerBackend) ClearAll(_ context.Context) error {
	return nil
}

var _ cache.Backend = (*ServerBackend)(nil)
