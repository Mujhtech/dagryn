package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/rs/zerolog"
)

// AuditWebhookForwardPayload is the payload for the audit_webhook:forward job.
type AuditWebhookForwardPayload struct {
	TeamID    string `json:"team_id"`
	EntryJSON []byte `json:"entry_json"`
}

// AuditWebhookForwardHandler delivers audit log entries to configured webhooks.
type AuditWebhookForwardHandler struct {
	auditRepo repo.AuditLogStore
	encrypter encrypt.Encrypt
	logger    zerolog.Logger
	client    *http.Client
	metrics   *telemetry.Metrics
}

// NewAuditWebhookForwardHandler creates a new webhook forward handler.
func NewAuditWebhookForwardHandler(
	auditRepo repo.AuditLogStore,
	enc encrypt.Encrypt,
	logger zerolog.Logger,
	metrics *telemetry.Metrics,
) *AuditWebhookForwardHandler {
	return &AuditWebhookForwardHandler{
		auditRepo: auditRepo,
		encrypter: enc,
		logger:    logger.With().Str("handler", "audit_webhook_forward").Logger(),
		client:    &http.Client{Timeout: 10 * time.Second},
		metrics:   metrics,
	}
}

// Handle processes the webhook forward task.
func (h *AuditWebhookForwardHandler) Handle(ctx context.Context, t *asynq.Task) error {
	// Decrypt payload.
	rawPayload := string(t.Payload())
	var plaintext string
	if h.encrypter != nil {
		var err error
		plaintext, err = h.encrypter.Decrypt(rawPayload)
		if err != nil {
			return fmt.Errorf("audit webhook: decrypt payload: %w", err)
		}
	} else {
		plaintext = rawPayload
	}

	var payload AuditWebhookForwardPayload
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		return fmt.Errorf("audit webhook: unmarshal payload: %w", err)
	}

	teamID, err := uuid.Parse(payload.TeamID)
	if err != nil {
		return fmt.Errorf("audit webhook: invalid team_id: %w", err)
	}

	webhooks, err := h.auditRepo.ListActiveWebhooksByTeam(ctx, teamID)
	if err != nil {
		return fmt.Errorf("audit webhook: list webhooks: %w", err)
	}

	for _, webhook := range webhooks {
		// Check event filter.
		if len(webhook.EventFilter) > 0 {
			var entry map[string]interface{}
			if err := json.Unmarshal(payload.EntryJSON, &entry); err == nil {
				action, _ := entry["action"].(string)
				if !contains(webhook.EventFilter, action) {
					continue
				}
			}
		}

		start := time.Now()

		// Decrypt the HMAC secret.
		secret, err := h.encrypter.Decrypt(webhook.SecretEncrypted)
		if err != nil {
			h.logger.Warn().Err(err).
				Str("webhook_id", webhook.ID.String()).
				Msg("failed to decrypt webhook secret")
			continue
		}

		// Compute HMAC-SHA256 signature.
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload.EntryJSON)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// POST to webhook URL.
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(payload.EntryJSON))
		if err != nil {
			h.logger.Warn().Err(err).
				Str("webhook_id", webhook.ID.String()).
				Msg("failed to create webhook request")
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Dagryn-Signature", signature)
		req.Header.Set("X-Dagryn-Event", "audit_log")

		resp, err := h.client.Do(req)
		duration := time.Since(start)

		if err != nil {
			h.logger.Warn().Err(err).
				Str("webhook_id", webhook.ID.String()).
				Str("url", webhook.URL).
				Dur("duration", duration).
				Msg("webhook delivery failed")

			if h.metrics != nil && h.metrics.AuditWebhookTotal != nil {
				h.metrics.AuditWebhookTotal.Add(ctx, 1)
			}
			continue
		}
		_ = resp.Body.Close()

		if h.metrics != nil {
			if h.metrics.AuditWebhookTotal != nil {
				h.metrics.AuditWebhookTotal.Add(ctx, 1)
			}
			if h.metrics.AuditWebhookDuration != nil {
				h.metrics.AuditWebhookDuration.Record(ctx, duration.Seconds())
			}
		}

		if resp.StatusCode >= 400 {
			h.logger.Warn().
				Str("webhook_id", webhook.ID.String()).
				Int("status", resp.StatusCode).
				Dur("duration", duration).
				Msg("webhook returned error status")
		} else {
			h.logger.Debug().
				Str("webhook_id", webhook.ID.String()).
				Int("status", resp.StatusCode).
				Dur("duration", duration).
				Msg("webhook delivered successfully")
		}
	}

	return nil
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
