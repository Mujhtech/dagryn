package handlers

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/rs/zerolog"
)

// AuditChainVerifyHandler runs periodic chain integrity verification.
type AuditChainVerifyHandler struct {
	auditService *service.AuditService
	auditRepo    repo.AuditLogStore
	metrics      *telemetry.Metrics
	logger       zerolog.Logger
}

// NewAuditChainVerifyHandler creates a new chain verify handler.
func NewAuditChainVerifyHandler(
	auditService *service.AuditService,
	auditRepo repo.AuditLogStore,
	metrics *telemetry.Metrics,
	logger zerolog.Logger,
) *AuditChainVerifyHandler {
	return &AuditChainVerifyHandler{
		auditService: auditService,
		auditRepo:    auditRepo,
		metrics:      metrics,
		logger:       logger.With().Str("handler", "audit_chain_verify").Logger(),
	}
}

// Handle processes the chain verification task.
func (h *AuditChainVerifyHandler) Handle(ctx context.Context, t *asynq.Task) error {
	h.logger.Info().Msg("Running audit chain verification...")

	policies, err := h.auditRepo.ListAllRetentionPolicies(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list retention policies for chain verification")
		return err
	}

	for _, policy := range policies {
		result, err := h.auditService.VerifyChain(ctx, policy.TeamID)
		if err != nil {
			h.logger.Error().Err(err).
				Str("team_id", policy.TeamID.String()).
				Msg("chain verification failed")
			continue
		}

		if !result.Valid {
			h.logger.Warn().
				Str("team_id", policy.TeamID.String()).
				Int("total_checked", result.TotalChecked).
				Str("message", result.Message).
				Msg("audit chain integrity broken")

			if h.metrics != nil && h.metrics.AuditChainBreaks != nil {
				h.metrics.AuditChainBreaks.Add(ctx, 1)
			}
		} else {
			h.logger.Debug().
				Str("team_id", policy.TeamID.String()).
				Int("total_checked", result.TotalChecked).
				Msg("audit chain verified OK")
		}
	}

	h.logger.Info().Msg("Audit chain verification completed")
	return nil
}
