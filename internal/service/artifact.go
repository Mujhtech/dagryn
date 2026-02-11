package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/rs/zerolog"
)

// ArtifactService coordinates artifact storage and database operations.
type ArtifactService struct {
	repo   *repo.ArtifactRepo
	bucket storage.Bucket
	signer storage.SignedURLer
	logger zerolog.Logger
}

// NewArtifactService creates a new artifact service.
func NewArtifactService(artifactRepo *repo.ArtifactRepo, bucket storage.Bucket, logger zerolog.Logger) *ArtifactService {
	var signer storage.SignedURLer
	if s, ok := bucket.(storage.SignedURLer); ok {
		signer = s
	}
	return &ArtifactService{
		repo:   artifactRepo,
		bucket: bucket,
		signer: signer,
		logger: logger.With().Str("service", "artifact").Logger(),
	}
}

// Upload stores artifact content and creates the DB record.
func (s *ArtifactService) Upload(ctx context.Context, projectID, runID uuid.UUID, taskName, name, fileName string, reader io.Reader, size int64, ttl time.Duration, contentType string) (*models.Artifact, error) {
	if s.repo == nil || s.bucket == nil {
		return nil, fmt.Errorf("artifact service not configured")
	}
	if name == "" {
		name = fileName
	}
	if fileName == "" {
		fileName = name
	}

	artifactID := uuid.New()
	storageKey := artifactStorageKey(projectID, runID, taskName, artifactID, fileName)

	head := make([]byte, 512)
	n, _ := io.ReadFull(reader, head)
	head = head[:n]
	if contentType == "" {
		contentType = http.DetectContentType(head)
	}

	hasher := sha256.New()
	combined := io.MultiReader(bytes.NewReader(head), reader)
	tee := io.TeeReader(combined, hasher)

	putOpts := &storage.PutOptions{ContentType: contentType}
	if size > 0 {
		putOpts.ContentLength = size
	}
	if err := s.bucket.Put(ctx, storageKey, tee, putOpts); err != nil {
		return nil, fmt.Errorf("artifact: upload blob: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}

	var taskNamePtr *string
	if taskName != "" {
		taskNamePtr = &taskName
	}

	metadata := json.RawMessage(`{}`)
	if name != fileName {
		if encoded, err := json.Marshal(map[string]string{"path": name}); err == nil {
			metadata = encoded
		}
	}

	artifact := &models.Artifact{
		ID:           artifactID,
		ProjectID:    projectID,
		RunID:        runID,
		TaskName:     taskNamePtr,
		Name:         name,
		FileName:     fileName,
		ContentType:  contentType,
		SizeBytes:    size,
		StorageKey:   storageKey,
		DigestSHA256: &digest,
		ExpiresAt:    expiresAt,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
	}

	if _, err := s.repo.Create(ctx, artifact); err != nil {
		_ = s.bucket.Delete(ctx, storageKey)
		return nil, fmt.Errorf("artifact: create record: %w", err)
	}

	s.logger.Debug().
		Str("project", projectID.String()).
		Str("run", runID.String()).
		Str("task", taskName).
		Str("name", name).
		Msg("artifact uploaded")

	return artifact, nil
}

// Download retrieves artifact content.
func (s *ArtifactService) Download(ctx context.Context, artifactID string) (io.ReadCloser, error) {
	id, err := uuid.Parse(artifactID)
	if err != nil {
		return nil, fmt.Errorf("artifact: invalid id: %w", err)
	}
	artifact, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rc, err := s.bucket.Get(ctx, artifact.StorageKey)
	if err != nil {
		if storage.IsNotFound(err) {
			return nil, repo.ErrNotFound
		}
		return nil, err
	}
	return rc, nil
}

// Get retrieves artifact metadata.
func (s *ArtifactService) Get(ctx context.Context, artifactID string) (*models.Artifact, error) {
	id, err := uuid.Parse(artifactID)
	if err != nil {
		return nil, fmt.Errorf("artifact: invalid id: %w", err)
	}
	return s.repo.GetByID(ctx, id)
}

// DownloadURL returns a pre-signed URL if supported.
func (s *ArtifactService) DownloadURL(ctx context.Context, artifactID string, expiry time.Duration) (string, error) {
	if s.signer == nil {
		return "", fmt.Errorf("artifact: signed URLs not supported")
	}
	id, err := uuid.Parse(artifactID)
	if err != nil {
		return "", fmt.Errorf("artifact: invalid id: %w", err)
	}
	artifact, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return s.signer.SignedGetURL(ctx, artifact.StorageKey, expiry)
}

// List returns artifacts for a run, optionally filtered by task.
func (s *ArtifactService) List(ctx context.Context, runID, taskName string, limit, offset int) ([]*models.Artifact, error) {
	id, err := uuid.Parse(runID)
	if err != nil {
		return nil, fmt.Errorf("artifact: invalid run id: %w", err)
	}
	if taskName != "" {
		return s.repo.ListByRunAndTask(ctx, id, taskName, limit, offset)
	}
	return s.repo.ListByRun(ctx, id, limit, offset)
}

// Delete removes an artifact and its storage object.
func (s *ArtifactService) Delete(ctx context.Context, artifactID string) error {
	id, err := uuid.Parse(artifactID)
	if err != nil {
		return fmt.Errorf("artifact: invalid id: %w", err)
	}
	artifact, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.bucket.Delete(ctx, artifact.StorageKey); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// CleanupExpired removes expired artifacts from storage and DB.
func (s *ArtifactService) CleanupExpired(ctx context.Context) (int64, error) {
	if s.repo == nil || s.bucket == nil {
		return 0, nil
	}

	expired, err := s.repo.ListExpired(ctx, 500)
	if err != nil {
		return 0, err
	}

	var removed int64
	for _, art := range expired {
		if err := s.bucket.Delete(ctx, art.StorageKey); err != nil {
			s.logger.Warn().Err(err).Str("artifact_id", art.ID.String()).Msg("artifact delete failed")
			continue
		}
		if err := s.repo.Delete(ctx, art.ID); err != nil && err != repo.ErrNotFound {
			s.logger.Warn().Err(err).Str("artifact_id", art.ID.String()).Msg("artifact db delete failed")
			continue
		}
		removed++
	}

	return removed, nil
}

func artifactStorageKey(projectID, runID uuid.UUID, taskName string, artifactID uuid.UUID, fileName string) string {
	if taskName == "" {
		taskName = "run"
	}
	return path.Join(
		"artifacts",
		projectID.String(),
		runID.String(),
		taskName,
		artifactID.String(),
		path.Base(fileName),
	)
}
