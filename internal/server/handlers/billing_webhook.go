package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/server/response"
	"github.com/rs/zerolog/log"
	gostripe "github.com/stripe/stripe-go/v84"
)

// StripeWebhook handles Stripe webhook events.
// POST /api/v1/webhooks/stripe
func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil || h.stripeClient == nil {
		_ = response.ServiceUnavailable(w, r, nil)
		return
	}

	payload, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		log.Warn().Err(err).Msg("stripe webhook: failed to read body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("Stripe-Signature")
	if signature == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	event, err := h.stripeClient.VerifyWebhookSignature(payload, signature)
	if err != nil {
		log.Warn().Err(err).Msg("stripe webhook: signature verification failed")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		var sub gostripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			log.Warn().Err(err).Msg("stripe webhook: failed to parse subscription")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		status := mapStripeSubStatus(sub.Status)
		var periodStart, periodEnd *time.Time
		// In stripe-go v82, current period is on subscription items
		if len(sub.Items.Data) > 0 {
			item := sub.Items.Data[0]
			if item.CurrentPeriodStart > 0 {
				t := time.Unix(item.CurrentPeriodStart, 0)
				periodStart = &t
			}
			if item.CurrentPeriodEnd > 0 {
				t := time.Unix(item.CurrentPeriodEnd, 0)
				periodEnd = &t
			}
		}

		if err := h.billingService.HandleSubscriptionUpdated(ctx, sub.ID, status, periodStart, periodEnd, sub.CancelAtPeriodEnd); err != nil {
			log.Warn().Err(err).Str("sub_id", sub.ID).Msg("stripe webhook: failed to handle subscription update")
			// Return 200 anyway to prevent Stripe retries for known-bad data
		}

	case "customer.subscription.deleted":
		var sub gostripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			log.Warn().Err(err).Msg("stripe webhook: failed to parse subscription deletion")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := h.billingService.HandleSubscriptionUpdated(ctx, sub.ID, models.SubscriptionCanceled, nil, nil, false); err != nil {
			log.Warn().Err(err).Str("sub_id", sub.ID).Msg("stripe webhook: failed to handle subscription deletion")
		}

	case "invoice.paid":
		var inv gostripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			log.Warn().Err(err).Msg("stripe webhook: failed to parse invoice")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Extract the billing period from the first line item, which holds the
		// actual subscription period (start/end of the billing cycle).
		// Invoice.PeriodStart/PeriodEnd is just the invoice generation date.
		var periodStart, periodEnd *time.Time
		if inv.Lines != nil && len(inv.Lines.Data) > 0 && inv.Lines.Data[0].Period != nil {
			period := inv.Lines.Data[0].Period
			if period.Start > 0 {
				t := time.Unix(period.Start, 0)
				periodStart = &t
			}
			if period.End > 0 {
				t := time.Unix(period.End, 0)
				periodEnd = &t
			}
		}

		pdfURL := ""
		if inv.InvoicePDF != "" {
			pdfURL = inv.InvoicePDF
		}
		hostedURL := ""
		if inv.HostedInvoiceURL != "" {
			hostedURL = inv.HostedInvoiceURL
		}

		customerID := ""
		if inv.Customer != nil {
			customerID = inv.Customer.ID
		}

		if err := h.billingService.HandleInvoicePaid(ctx, customerID, inv.ID,
			int(inv.AmountPaid), string(inv.Currency),
			pdfURL, hostedURL, periodStart, periodEnd); err != nil {
			log.Warn().Err(err).Str("invoice_id", inv.ID).Msg("stripe webhook: failed to handle invoice")
		}

	case "invoice.payment_failed":
		var inv gostripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			log.Warn().Err(err).Msg("stripe webhook: failed to parse failed invoice")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		customerID := ""
		if inv.Customer != nil {
			customerID = inv.Customer.ID
		}

		if customerID != "" {
			if err := h.billingService.HandleInvoicePaymentFailed(ctx, customerID, inv.ID); err != nil {
				log.Warn().Err(err).Str("invoice_id", inv.ID).Msg("stripe webhook: failed to handle payment failure")
			}
		}

	case "checkout.session.completed":
		var session gostripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			log.Warn().Err(err).Msg("stripe webhook: failed to parse checkout session")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		customerID := ""
		if session.Customer != nil {
			customerID = session.Customer.ID
		}
		subID := ""
		if session.Subscription != nil {
			subID = session.Subscription.ID
		}

		if customerID != "" && subID != "" {
			if err := h.billingService.HandleCheckoutCompleted(ctx, customerID, subID); err != nil {
				log.Warn().Err(err).Msg("stripe webhook: failed to handle checkout completion")
			}
		}

	default:
		log.Debug().Str("type", string(event.Type)).Msg("stripe webhook: unhandled event type")
	}

	w.WriteHeader(http.StatusOK)
}

// mapStripeSubStatus maps a Stripe subscription status to our internal model.
func mapStripeSubStatus(status gostripe.SubscriptionStatus) models.SubscriptionStatus {
	switch status {
	case gostripe.SubscriptionStatusActive:
		return models.SubscriptionActive
	case gostripe.SubscriptionStatusTrialing:
		return models.SubscriptionTrialing
	case gostripe.SubscriptionStatusPastDue:
		return models.SubscriptionPastDue
	case gostripe.SubscriptionStatusCanceled:
		return models.SubscriptionCanceled
	case gostripe.SubscriptionStatusUnpaid:
		return models.SubscriptionUnpaid
	case gostripe.SubscriptionStatusIncomplete:
		return models.SubscriptionIncomplete
	default:
		return models.SubscriptionIncomplete
	}
}
