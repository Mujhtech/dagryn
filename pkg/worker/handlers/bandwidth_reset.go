package handlers

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
)

// BandwidthResetHandler resets bandwidth counters for accounts at the start of new billing periods.
type BandwidthResetHandler struct {
	billing *repo.BillingRepo
}

// NewBandwidthResetHandler creates a new bandwidth reset handler.
func NewBandwidthResetHandler(billing *repo.BillingRepo) *BandwidthResetHandler {
	return &BandwidthResetHandler{
		billing: billing,
	}
}

// Handle processes the bandwidth reset task.
func (h *BandwidthResetHandler) Handle(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("Running bandwidth reset check...")

	accountIDs, err := h.billing.ListAccountsWithExpiredBandwidth(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Bandwidth reset: failed to list accounts with expired bandwidth")
		return err
	}

	if len(accountIDs) == 0 {
		log.Info().Msg("Bandwidth reset: no accounts need resetting")
		return nil
	}

	var resetCount int
	for _, accountID := range accountIDs {
		if err := h.billing.ResetBandwidthForAccount(ctx, accountID); err != nil {
			log.Warn().Err(err).
				Str("account_id", accountID.String()).
				Msg("Bandwidth reset: failed for account")
			continue
		}
		resetCount++
	}

	log.Info().
		Int("accounts_checked", len(accountIDs)).
		Int("accounts_reset", resetCount).
		Msg("Bandwidth reset completed")

	return nil
}
