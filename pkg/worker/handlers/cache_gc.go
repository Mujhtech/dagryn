package handlers

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
)

// CacheGCHandler runs garbage collection across all projects.
type CacheGCHandler struct {
	cacheService *service.CacheService
	projects     *repo.ProjectRepo
}

// NewCacheGCHandler creates a new cache GC handler.
func NewCacheGCHandler(cs *service.CacheService, projects *repo.ProjectRepo) *CacheGCHandler {
	return &CacheGCHandler{
		cacheService: cs,
		projects:     projects,
	}
}

// Handle processes the cache GC task.
func (h *CacheGCHandler) Handle(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("Running cache garbage collection...")

	projects, _, err := h.projects.ListAll(ctx, 1000, 0)
	if err != nil {
		log.Error().Err(err).Msg("Cache GC: failed to list projects")
		return err
	}

	var totalRemoved int
	var totalFreed int64
	var totalBlobs int

	for _, project := range projects {
		result, err := h.cacheService.RunGC(ctx, project.ID)
		if err != nil {
			log.Warn().Err(err).
				Str("project_id", project.ID.String()).
				Msg("Cache GC: failed for project")
			continue
		}
		totalRemoved += result.EntriesRemoved
		totalFreed += result.BytesFreed
		totalBlobs += result.BlobsRemoved
	}

	log.Info().
		Int("projects", len(projects)).
		Int("entries_removed", totalRemoved).
		Int64("bytes_freed", totalFreed).
		Int("blobs_removed", totalBlobs).
		Msg("Cache GC completed")

	return nil
}
