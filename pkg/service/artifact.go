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
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/rs/zerolog"
)

// ArtifactService coordinates artifact storage and database operations.
type ArtifactService struct {
	repo         repo.ArtifactStore
	bucket       storage.Bucket
	signer       storage.SignedURLer
	logger       zerolog.Logger
	entitlements entitlement.Checker
}

// SetEntitlements sets the entitlement checker for quota enforcement.
func (s *ArtifactService) SetEntitlements(c entitlement.Checker) {
	s.entitlements = c
}

// NewArtifactService creates a new artifact service.
func NewArtifactService(artifactRepo repo.ArtifactStore, bucket storage.Bucket, logger zerolog.Logger) *ArtifactService {
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
func (s *ArtifactService) Upload(ctx context.Context, projectID, runID uuid.UUID, taskName, name, fileName string, reader io.Reader, size int64, ttl time.Duration, contentType string, extraMetadata json.RawMessage) (*models.Artifact, error) {
	if s.repo == nil || s.bucket == nil {
		return nil, fmt.Errorf("artifact service not configured")
	}
	// Entitlement-based storage quota check (unified path for OSS + cloud).
	if s.entitlements != nil {
		if err := s.entitlements.CheckQuota(ctx, "storage", projectID, size); err != nil {
			return nil, err
		}
	}

	if name == "" {
		name = fileName
	}
	if fileName == "" {
		fileName = name
	}

	artifactID := uuid.New()
	storageKey := artifactStorageKey(projectID, runID, taskName, artifactID, fileName)

	var digest string

	// When the reader is seekable (e.g. *os.File from server-side artifact
	// collection), stream the hash and upload directly without buffering the
	// entire body in memory. For non-seekable readers (e.g. HTTP request
	// bodies), fall back to io.ReadAll so S3 retries can re-read the body.
	if rs, ok := reader.(io.ReadSeeker); ok {
		if size <= 0 {
			// Try to determine size via Stat if available (e.g. *os.File).
			if f, ok := reader.(*os.File); ok {
				if info, err := f.Stat(); err == nil {
					size = info.Size()
				}
			}
		}

		// Detect content type from the first 512 bytes.
		if contentType == "" {
			sniff := make([]byte, 512)
			n, _ := io.ReadFull(rs, sniff)
			contentType = http.DetectContentType(sniff[:n])
			if _, err := rs.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("artifact: seek after sniff: %w", err)
			}
		}

		// Hash the file by streaming from disk.
		hasher := sha256.New()
		if _, err := io.Copy(hasher, rs); err != nil {
			return nil, fmt.Errorf("artifact: hash data: %w", err)
		}
		digest = hex.EncodeToString(hasher.Sum(nil))

		// Seek back for upload.
		if _, err := rs.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("artifact: seek for upload: %w", err)
		}

		putOpts := &storage.PutOptions{
			ContentType:   contentType,
			ContentLength: size,
		}
		if err := s.bucket.Put(ctx, storageKey, rs, putOpts); err != nil {
			return nil, fmt.Errorf("artifact: upload blob: %w", err)
		}
	} else {
		// Non-seekable reader: buffer into memory for S3 retries.
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("artifact: read upload data: %w", err)
		}
		if size <= 0 {
			size = int64(len(data))
		}

		if contentType == "" {
			contentType = http.DetectContentType(data)
		}

		hasher := sha256.New()
		hasher.Write(data)
		digest = hex.EncodeToString(hasher.Sum(nil))

		putOpts := &storage.PutOptions{
			ContentType:   contentType,
			ContentLength: size,
		}
		if err := s.bucket.Put(ctx, storageKey, bytes.NewReader(data), putOpts); err != nil {
			return nil, fmt.Errorf("artifact: upload blob: %w", err)
		}
	}
	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}

	var taskNamePtr *string
	if taskName != "" {
		taskNamePtr = &taskName
	}

	var metadata json.RawMessage
	if extraMetadata != nil {
		// Start with caller-provided metadata and merge path if needed.
		merged := make(map[string]interface{})
		_ = json.Unmarshal(extraMetadata, &merged)
		if name != fileName {
			merged["path"] = name
		}
		metadata, _ = json.Marshal(merged)
	} else if name != fileName {
		if encoded, err := json.Marshal(map[string]string{"path": name}); err == nil {
			metadata = encoded
		}
	}
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
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

	// Record bandwidth usage for upload (fire-and-forget)
	if s.entitlements != nil {
		go s.entitlements.RecordUsage(context.Background(), "bandwidth", projectID, size)
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

	// Bandwidth quota check.
	if s.entitlements != nil {
		if err := s.entitlements.CheckQuota(ctx, "bandwidth", artifact.ProjectID, artifact.SizeBytes); err != nil {
			return nil, err
		}
	}

	rc, err := s.bucket.Get(ctx, artifact.StorageKey)
	if err != nil {
		if storage.IsNotFound(err) {
			return nil, repo.ErrNotFound
		}
		return nil, err
	}

	// Record bandwidth usage (fire-and-forget)
	if s.entitlements != nil {
		go s.entitlements.RecordUsage(context.Background(), "bandwidth", artifact.ProjectID, artifact.SizeBytes)
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
