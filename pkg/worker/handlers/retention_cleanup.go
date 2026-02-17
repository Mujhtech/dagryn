package handlers

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
)

// RetentionCleanupHandler enforces plan-driven retention limits for artifacts, logs, and cache entries.
type RetentionCleanupHandler struct {
	billingRepo  *repo.BillingRepo
	artifactRepo *repo.ArtifactRepo
	runRepo      *repo.RunRepo
	cacheRepo    *repo.CacheRepo
	quotaService *service.QuotaService
}

// NewRetentionCleanupHandler creates a new retention cleanup handler.
func NewRetentionCleanupHandler(
	billingRepo *repo.BillingRepo,
	artifactRepo *repo.ArtifactRepo,
	runRepo *repo.RunRepo,
	cacheRepo *repo.CacheRepo,
	quotaService *service.QuotaService,
) *RetentionCleanupHandler {
	return &RetentionCleanupHandler{
		billingRepo:  billingRepo,
		artifactRepo: artifactRepo,
		runRepo:      runRepo,
		cacheRepo:    cacheRepo,
		quotaService: quotaService,
	}
}

// Handle processes the retention cleanup task.
func (h *RetentionCleanupHandler) Handle(ctx context.Context, _ *asynq.Task) error {
	log.Info().Msg("Running plan-driven retention cleanup...")

	accountIDs, err := h.billingRepo.ListAllBillingAccountIDs(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Retention cleanup: failed to list billing accounts")
		return err
	}

	var totalArtifacts, totalLogs, totalCache int64

	for _, accountID := range accountIDs {
		cacheTTL, artifactRetention, logRetention, err := h.quotaService.GetRetentionLimits(ctx, accountID)
		if err != nil {
			log.Warn().Err(err).Str("account_id", accountID.String()).Msg("Retention cleanup: failed to get limits")
			continue
		}

		// Skip if no retention limits are set
		if cacheTTL == nil && artifactRetention == nil && logRetention == nil {
			continue
		}

		// Get projects for this account
		projectIDs, err := h.billingRepo.ListProjectIDsByAccount(ctx, accountID)
		if err != nil {
			log.Warn().Err(err).Str("account_id", accountID.String()).Msg("Retention cleanup: failed to list projects")
			continue
		}
		if len(projectIDs) == 0 {
			continue
		}

		now := time.Now()

		// Enforce artifact_retention_days
		if artifactRetention != nil && *artifactRetention > 0 {
			before := now.AddDate(0, 0, -*artifactRetention)
			deleted, _, err := h.artifactRepo.DeleteOlderThanForProjects(ctx, projectIDs, before)
			if err != nil {
				log.Warn().Err(err).Str("account_id", accountID.String()).Msg("Retention cleanup: artifact deletion failed")
			} else {
				totalArtifacts += deleted
			}
		}

		// Enforce log_retention_days
		if logRetention != nil && *logRetention > 0 {
			before := now.AddDate(0, 0, -*logRetention)
			deleted, err := h.runRepo.DeleteLogsOlderThanForProjects(ctx, projectIDs, before)
			if err != nil {
				log.Warn().Err(err).Str("account_id", accountID.String()).Msg("Retention cleanup: log deletion failed")
			} else {
				totalLogs += deleted
			}
		}

		// Enforce cache_ttl_days
		if cacheTTL != nil && *cacheTTL > 0 {
			before := now.AddDate(0, 0, -*cacheTTL)
			deleted, err := h.cacheRepo.DeleteEntriesOlderThanForProjects(ctx, projectIDs, before)
			if err != nil {
				log.Warn().Err(err).Str("account_id", accountID.String()).Msg("Retention cleanup: cache deletion failed")
			} else {
				totalCache += deleted
			}
		}
	}

	log.Info().
		Int("accounts_processed", len(accountIDs)).
		Int64("artifacts_removed", totalArtifacts).
		Int64("log_rows_removed", totalLogs).
		Int64("cache_entries_removed", totalCache).
		Msg("Retention cleanup completed")

	return nil
}
