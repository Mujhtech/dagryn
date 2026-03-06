package cluster

import (
	"context"
	"time"

	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
)

// Reassigner periodically checks for orphaned task assignments (from offline workers)
// and re-queues them for dispatch to healthy workers.
type Reassigner struct {
	store    repo.ClusterStore
	interval time.Duration
}

// NewReassigner creates a new task reassigner.
func NewReassigner(store repo.ClusterStore, interval time.Duration) *Reassigner {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &Reassigner{store: store, interval: interval}
}

// Start runs the reassigner loop until the context is cancelled.
func (r *Reassigner) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reassignOrphaned(ctx)
		}
	}
}

func (r *Reassigner) reassignOrphaned(ctx context.Context) {
	assignments, err := r.store.ListOrphanedAssignments(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list orphaned task assignments")
		return
	}

	for _, a := range assignments {
		if a.RetryCount >= a.MaxRetries {
			// Exhausted retries, mark as failed
			_ = r.store.UpdateTaskAssignmentStatus(ctx, a.ID, models.TaskAssignmentFailed, nil)
			log.Warn().
				Str("assignment_id", a.ID.String()).
				Str("task", a.TaskName).
				Int("retries", a.RetryCount).
				Msg("Task assignment failed (retries exhausted)")
			continue
		}

		// Re-queue for dispatch
		if err := r.store.IncrementAssignmentRetry(ctx, a.ID); err != nil {
			log.Warn().Err(err).Str("assignment_id", a.ID.String()).Msg("Failed to re-queue task assignment")
			continue
		}

		log.Info().
			Str("assignment_id", a.ID.String()).
			Str("task", a.TaskName).
			Int("retry", a.RetryCount+1).
			Msg("Task assignment re-queued")
	}
}
