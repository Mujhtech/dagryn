package cloud

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache"
)

// Backend implements cache.Backend using the Dagryn Cloud cache HTTP API.
type Backend struct {
	client      *client.Client
	projectID   uuid.UUID
	projectRoot string
}

// NewBackend creates a cloud cache backend.
func NewBackend(c *client.Client, projectID uuid.UUID, projectRoot string) *Backend {
	return &Backend{
		client:      c,
		projectID:   projectID,
		projectRoot: projectRoot,
	}
}

func (b *Backend) Check(ctx context.Context, taskName, key string) (bool, error) {
	return b.client.CheckCache(ctx, b.projectID, taskName, key)
}

func (b *Backend) Restore(ctx context.Context, taskName, key string) error {
	dl, err := b.client.DownloadCache(ctx, b.projectID, taskName, key)
	if err != nil {
		return fmt.Errorf("cloud cache download: %w", err)
	}
	defer func() { _ = dl.Body.Close() }()

	vr := newVerifyReader(dl.Body, dl.DigestHash, dl.SizeBytes)
	if err := ExtractArchive(b.projectRoot, vr); err != nil {
		return fmt.Errorf("cloud cache extract: %w", err)
	}
	if err := vr.Verify(); err != nil {
		return fmt.Errorf("cloud cache integrity: %w", err)
	}
	return nil
}

func (b *Backend) Save(ctx context.Context, taskName, key string, outputPatterns []string, _ cache.Metadata) error {
	if len(outputPatterns) == 0 {
		return nil
	}

	archive, err := CreateArchive(b.projectRoot, outputPatterns, nil)
	if err != nil {
		return fmt.Errorf("cloud cache archive: %w", err)
	}
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archive.Name())
	}()

	info, err := archive.Stat()
	if err != nil {
		return fmt.Errorf("cloud cache stat archive: %w", err)
	}

	if err := b.client.UploadCache(ctx, b.projectID, taskName, key, archive, info.Size()); err != nil {
		return fmt.Errorf("cloud cache upload: %w", err)
	}
	return nil
}

func (b *Backend) Clear(_ context.Context, _ string) error {
	// Cloud GC handles cleanup; no-op on client side.
	return nil
}

func (b *Backend) ClearAll(_ context.Context) error {
	// Cloud GC handles cleanup; no-op on client side.
	return nil
}

// Verify interface compliance at compile time.
var _ cache.Backend = (*Backend)(nil)
