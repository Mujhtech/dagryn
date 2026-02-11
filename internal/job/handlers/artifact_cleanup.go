package handlers

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/internal/service"
	"github.com/rs/zerolog/log"
)

// ArtifactCleanupHandler removes expired artifacts.
type ArtifactCleanupHandler struct {
	artifactService *service.ArtifactService
}

// NewArtifactCleanupHandler creates a new artifact cleanup handler.
func NewArtifactCleanupHandler(svc *service.ArtifactService) *ArtifactCleanupHandler {
	return &ArtifactCleanupHandler{artifactService: svc}
}

// Handle processes the artifact cleanup task.
func (h *ArtifactCleanupHandler) Handle(ctx context.Context, _ *asynq.Task) error {
	log.Info().Msg("Running artifact cleanup...")

	removed, err := h.artifactService.CleanupExpired(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Artifact cleanup failed")
		return err
	}

	log.Info().Int64("artifacts_removed", removed).Msg("Artifact cleanup completed")
	return nil
}
