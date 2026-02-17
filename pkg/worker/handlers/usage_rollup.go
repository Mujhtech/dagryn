package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	dagrynstripe "github.com/mujhtech/dagryn/internal/stripe"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
)

// UsageRollupHandler aggregates unreported usage events and reports them to Stripe.
type UsageRollupHandler struct {
	billing *repo.BillingRepo
	stripe  *dagrynstripe.Client
}

// NewUsageRollupHandler creates a new usage rollup handler.
func NewUsageRollupHandler(billing *repo.BillingRepo, stripe *dagrynstripe.Client) *UsageRollupHandler {
	return &UsageRollupHandler{
		billing: billing,
		stripe:  stripe,
	}
}

// Handle processes the usage rollup task.
func (h *UsageRollupHandler) Handle(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("Running billing usage rollup...")

	accountIDs, err := h.billing.ListAllBillingAccountIDs(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Usage rollup: failed to list billing accounts")
		return err
	}

	var totalReported int
	for _, accountID := range accountIDs {
		reported, err := h.rollupForAccount(ctx, accountID)
		if err != nil {
			log.Warn().Err(err).
				Str("account_id", accountID.String()).
				Msg("Usage rollup: failed for account")
			continue
		}
		totalReported += reported
	}

	log.Info().
		Int("accounts", len(accountIDs)).
		Int("events_reported", totalReported).
		Msg("Usage rollup completed")

	return nil
}

func (h *UsageRollupHandler) rollupForAccount(ctx context.Context, accountID uuid.UUID) (int, error) {
	events, err := h.billing.ListUnreportedEvents(ctx, accountID, 500)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}

	// Aggregate by event type
	type aggregate struct {
		quantity int64
		ids      []uuid.UUID
	}
	aggregated := make(map[string]*aggregate)
	for _, e := range events {
		agg, ok := aggregated[e.EventType]
		if !ok {
			agg = &aggregate{}
			aggregated[e.EventType] = agg
		}
		agg.quantity += e.Quantity
		agg.ids = append(agg.ids, e.ID)
	}

	// Get the Stripe customer ID for this account
	account, err := h.billing.GetAccountByID(ctx, accountID)
	if err != nil {
		return 0, err
	}

	var reported int
	for eventType, agg := range aggregated {
		// Report to Stripe if we have a customer and a stripe client
		stripeUsageID := ""
		if h.stripe != nil && account.StripeCustomerID != nil {
			meterEvent, err := h.stripe.ReportUsage(ctx, eventType, *account.StripeCustomerID, agg.quantity, 0)
			if err != nil {
				log.Warn().Err(err).
					Str("event_type", eventType).
					Int64("quantity", agg.quantity).
					Msg("Usage rollup: failed to report to Stripe")
				continue
			}
			if meterEvent != nil {
				stripeUsageID = meterEvent.Identifier
			}
		}

		if err := h.billing.MarkEventsReported(ctx, agg.ids, stripeUsageID); err != nil {
			log.Warn().Err(err).
				Str("event_type", eventType).
				Msg("Usage rollup: failed to mark events as reported")
			continue
		}
		reported += len(agg.ids)
	}

	return reported, nil
}
