package handlers

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
)

// StaleRunTimeout is the duration after which a run is considered stale.
const StaleRunTimeout = 5 * time.Minute

// StaleRunsHandler checks for and marks stale runs.
type StaleRunsHandler struct {
	runs repo.RunStore
}

// NewStaleRunsHandler creates a new stale runs handler.
func NewStaleRunsHandler(runs repo.RunStore) *StaleRunsHandler {
	return &StaleRunsHandler{runs: runs}
}

// Handle processes the stale runs check task.
func (h *StaleRunsHandler) Handle(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("Checking for stale runs...")

	// Find runs that are in "running" status with stale heartbeats
	staleRuns, err := h.runs.ListStaleRuns(ctx, StaleRunTimeout)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list stale runs")
		return err
	}

	if len(staleRuns) == 0 {
		log.Debug().Msg("No stale runs found")
		return nil
	}

	log.Info().Int("count", len(staleRuns)).Msg("Found stale runs")

	// Mark each run as stale (client disconnected)
	for _, run := range staleRuns {
		if err := h.runs.MarkAsStale(ctx, run.ID); err != nil {
			log.Error().
				Err(err).
				Str("run_id", run.ID.String()).
				Msg("Failed to mark run as stale")
			continue
		}

		log.Info().
			Str("run_id", run.ID.String()).
			Str("project_id", run.ProjectID.String()).
			Time("last_heartbeat", *run.LastHeartbeatAt).
			Msg("Marked run as stale (client disconnected)")
	}

	return nil
}
