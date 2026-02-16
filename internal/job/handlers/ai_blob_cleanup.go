package handlers

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// aiBlobCleanupRepo defines the AI repo operations needed by the blob cleanup handler.
type aiBlobCleanupRepo interface {
	DeleteExpiredBlobKeys(ctx context.Context, olderThan time.Time) (int64, error)
}

// AIBlobCleanupHandler processes ai_blob_cleanup:run jobs.
// It nullifies raw_response_blob_key on analyses older than the configured TTL
// and optionally deletes the corresponding blobs from object storage.
type AIBlobCleanupHandler struct {
	aiRepo   aiBlobCleanupRepo
	ttlHours int
	logger   zerolog.Logger
}

// NewAIBlobCleanupHandler creates a new blob cleanup handler.
func NewAIBlobCleanupHandler(
	aiRepo aiBlobCleanupRepo,
	ttlHours int,
	logger zerolog.Logger,
) *AIBlobCleanupHandler {
	if ttlHours <= 0 {
		ttlHours = 168 // Default: 7 days
	}
	return &AIBlobCleanupHandler{
		aiRepo:   aiRepo,
		ttlHours: ttlHours,
		logger:   logger.With().Str("handler", "ai_blob_cleanup").Logger(),
	}
}

// Handle processes the ai_blob_cleanup:run task.
func (h *AIBlobCleanupHandler) Handle(ctx context.Context, _ *asynq.Task) error {
	cutoff := time.Now().Add(-time.Duration(h.ttlHours) * time.Hour)

	cleaned, err := h.aiRepo.DeleteExpiredBlobKeys(ctx, cutoff)
	if err != nil {
		h.logger.Error().Err(err).Msg("blob cleanup failed")
		return err
	}

	if cleaned > 0 {
		h.logger.Info().
			Int64("cleaned", cleaned).
			Int("ttl_hours", h.ttlHours).
			Msg("expired AI blob keys cleaned up")
	}

	return nil
}
