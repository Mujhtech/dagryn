package handlers

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/rs/zerolog/log"
)

// AuditRetentionGCHandler runs audit log retention garbage collection.
type AuditRetentionGCHandler struct {
	auditService *service.AuditService
}

// NewAuditRetentionGCHandler creates a new audit retention GC handler.
func NewAuditRetentionGCHandler(as *service.AuditService) *AuditRetentionGCHandler {
	return &AuditRetentionGCHandler{auditService: as}
}

// Handle processes the audit retention GC task.
func (h *AuditRetentionGCHandler) Handle(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("Running audit log retention GC...")

	if err := h.auditService.RunRetentionGC(ctx); err != nil {
		log.Error().Err(err).Msg("Audit retention GC: failed")
		return err
	}

	log.Info().Msg("Audit retention GC completed")
	return nil
}
