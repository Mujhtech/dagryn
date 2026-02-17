package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	dagrynstripe "github.com/mujhtech/dagryn/internal/stripe"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog"
)

// BillingService coordinates billing operations between the database and Stripe.
type BillingService struct {
	repo   *repo.BillingRepo
	stripe *dagrynstripe.Client
	logger zerolog.Logger
}

// NewBillingService creates a new billing service.
func NewBillingService(billingRepo *repo.BillingRepo, stripeClient *dagrynstripe.Client, logger zerolog.Logger) *BillingService {
	return &BillingService{
		repo:   billingRepo,
		stripe: stripeClient,
		logger: logger.With().Str("service", "billing").Logger(),
	}
}

// --- Plans ---

// ListPlans returns all active billing plans.
func (s *BillingService) ListPlans(ctx context.Context) ([]models.BillingPlan, error) {
	return s.repo.ListActivePlans(ctx)
}

// GetPlan returns a billing plan by slug.
func (s *BillingService) GetPlan(ctx context.Context, slug string) (*models.BillingPlan, error) {
	return s.repo.GetPlanBySlug(ctx, slug)
}

// --- Accounts ---

// GetOrCreateAccount returns the billing account for a user, creating one if needed.
func (s *BillingService) GetOrCreateAccount(ctx context.Context, userID uuid.UUID, email, name string) (*models.BillingAccount, error) {
	account, err := s.repo.GetAccountByUserID(ctx, userID)
	if err == nil {
		return account, nil
	}
	if err != repo.ErrNotFound {
		return nil, fmt.Errorf("billing: get account: %w", err)
	}

	// Create billing account
	account = &models.BillingAccount{
		UserID: &userID,
		Email:  email,
		Name:   &name,
	}
	if err := s.repo.CreateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("billing: create account: %w", err)
	}

	// Create Stripe customer
	if s.stripe != nil {
		metadata := map[string]string{
			"billing_account_id": account.ID.String(),
			"user_id":            userID.String(),
		}
		customer, err := s.stripe.CreateCustomer(ctx, email, name, metadata)
		if err != nil {
			s.logger.Warn().Err(err).Str("account_id", account.ID.String()).Msg("failed to create Stripe customer")
		} else {
			if err := s.repo.UpdateStripeCustomerID(ctx, account.ID, customer.ID); err != nil {
				s.logger.Warn().Err(err).Msg("failed to save Stripe customer ID")
			}
			account.StripeCustomerID = &customer.ID
		}
	}

	// Create free subscription
	freePlan, err := s.repo.GetPlanBySlug(ctx, "free")
	if err != nil {
		s.logger.Warn().Err(err).Msg("free plan not found, skipping default subscription")
		return account, nil
	}

	sub := &models.Subscription{
		BillingAccountID: account.ID,
		PlanID:           freePlan.ID,
		Status:           models.SubscriptionActive,
		SeatCount:        1,
	}
	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		s.logger.Warn().Err(err).Msg("failed to create free subscription")
	}

	s.logger.Info().
		Str("account_id", account.ID.String()).
		Str("user_id", userID.String()).
		Msg("billing account created with free plan")

	return account, nil
}

// GetAccountForTeam returns the billing account for a team, creating one if needed.
func (s *BillingService) GetAccountForTeam(ctx context.Context, teamID uuid.UUID, email, teamName string) (*models.BillingAccount, error) {
	account, err := s.repo.GetAccountByTeamID(ctx, teamID)
	if err == nil {
		return account, nil
	}
	if err != repo.ErrNotFound {
		return nil, fmt.Errorf("billing: get team account: %w", err)
	}

	account = &models.BillingAccount{
		TeamID: &teamID,
		Email:  email,
		Name:   &teamName,
	}
	if err := s.repo.CreateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("billing: create team account: %w", err)
	}

	// Create Stripe customer for team
	if s.stripe != nil {
		metadata := map[string]string{
			"billing_account_id": account.ID.String(),
			"team_id":            teamID.String(),
		}
		customer, err := s.stripe.CreateCustomer(ctx, email, teamName, metadata)
		if err != nil {
			s.logger.Warn().Err(err).Str("account_id", account.ID.String()).Msg("failed to create Stripe customer for team")
		} else {
			if err := s.repo.UpdateStripeCustomerID(ctx, account.ID, customer.ID); err != nil {
				s.logger.Warn().Err(err).Msg("failed to save Stripe customer ID")
			}
			account.StripeCustomerID = &customer.ID
		}
	}

	// Create free subscription (same as GetOrCreateAccount)
	freePlan, err := s.repo.GetPlanBySlug(ctx, "free")
	if err != nil {
		s.logger.Warn().Err(err).Msg("free plan not found, skipping default subscription for team")
		return account, nil
	}

	sub := &models.Subscription{
		BillingAccountID: account.ID,
		PlanID:           freePlan.ID,
		Status:           models.SubscriptionActive,
		SeatCount:        1,
	}
	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		s.logger.Warn().Err(err).Msg("failed to create free subscription for team")
	}

	s.logger.Info().
		Str("account_id", account.ID.String()).
		Str("team_id", teamID.String()).
		Msg("team billing account created with free plan")

	return account, nil
}

// GetAccount returns a billing account by ID.
func (s *BillingService) GetAccount(ctx context.Context, accountID uuid.UUID) (*models.BillingAccount, error) {
	return s.repo.GetAccountByID(ctx, accountID)
}

// --- Subscriptions ---

// ResourceUsage holds actual resource consumption for a billing account.
type ResourceUsage struct {
	CacheBytesUsed        int64 `json:"cache_bytes_used"`
	ArtifactBytesUsed     int64 `json:"artifact_bytes_used"`
	TotalStorageBytesUsed int64 `json:"total_storage_bytes_used"`
	BandwidthBytesUsed    int64 `json:"bandwidth_bytes_used"`
	ProjectsUsed          int   `json:"projects_used"`
	TeamMembersUsed       int   `json:"team_members_used"`
	ConcurrentRuns        int   `json:"concurrent_runs"`
	AIAnalysesUsed        int   `json:"ai_analyses_used"`
}

// BillingOverview holds the account, subscription, plan, and usage data for display.
type BillingOverview struct {
	Account       *models.BillingAccount `json:"account"`
	Subscription  *models.Subscription   `json:"subscription,omitempty"`
	Plan          *models.BillingPlan    `json:"plan,omitempty"`
	Usage         map[string]int64       `json:"usage,omitempty"`
	ResourceUsage *ResourceUsage         `json:"resource_usage,omitempty"`
}

// GetOverview returns a billing overview for a billing account.
func (s *BillingService) GetOverview(ctx context.Context, accountID uuid.UUID) (*BillingOverview, error) {
	account, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("billing: get account: %w", err)
	}

	overview := &BillingOverview{Account: account}

	sub, err := s.repo.GetActiveSubscription(ctx, accountID)
	if err != nil && err != repo.ErrNotFound {
		return nil, fmt.Errorf("billing: get subscription: %w", err)
	}
	// Fall back to past_due/unpaid so the billing UI still shows the plan.
	if sub == nil {
		sub, err = s.repo.GetSubscriptionByStatus(ctx, accountID, models.SubscriptionPastDue, models.SubscriptionUnpaid)
		if err != nil && err != repo.ErrNotFound {
			return nil, fmt.Errorf("billing: get overdue subscription: %w", err)
		}
	}
	if sub != nil {
		overview.Subscription = sub
		plan, err := s.repo.GetPlanByID(ctx, sub.PlanID)
		if err == nil {
			overview.Plan = plan
		}
	}

	// Current period usage events (for billing detail / Stripe reconciliation)
	since := time.Now().AddDate(0, -1, 0)
	if sub != nil && sub.CurrentPeriodStart != nil {
		since = *sub.CurrentPeriodStart
	}
	usage, err := s.repo.GetUsageSummary(ctx, accountID, since)
	if err == nil {
		overview.Usage = usage
	}

	// Actual resource consumption from live queries
	resUsage := &ResourceUsage{}

	// Cache storage + bandwidth from cache_quotas (current state, not cumulative events)
	cacheBytes, bandwidthBytes, err := s.repo.GetCacheQuotaByAccount(ctx, accountID)
	if err == nil {
		resUsage.CacheBytesUsed = cacheBytes
		resUsage.BandwidthBytesUsed = bandwidthBytes
	}

	// Artifact storage
	artifactBytes, err := s.repo.GetTotalArtifactSizeByAccount(ctx, accountID)
	if err == nil {
		resUsage.ArtifactBytesUsed = artifactBytes
	}

	// Total unified storage (cache + artifacts)
	totalStorage, err := s.repo.GetTotalStorageByAccount(ctx, accountID)
	if err == nil {
		resUsage.TotalStorageBytesUsed = totalStorage
	}

	// Project count
	projectCount, err := s.repo.CountProjectsByAccount(ctx, accountID)
	if err == nil {
		resUsage.ProjectsUsed = projectCount
	}

	// Team member count (only relevant for team accounts)
	if account.TeamID != nil {
		memberCount, err := s.repo.CountTeamMembers(ctx, *account.TeamID)
		if err == nil {
			resUsage.TeamMembersUsed = memberCount
		}
	}

	// Active concurrent runs
	runCount, err := s.repo.CountActiveRunsByAccount(ctx, accountID)
	if err == nil {
		resUsage.ConcurrentRuns = runCount
	}

	// AI analyses this month
	startOfMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC)
	aiCount, err := s.repo.CountAIAnalysesByAccount(ctx, accountID, startOfMonth)
	if err == nil {
		resUsage.AIAnalysesUsed = aiCount
	}

	overview.ResourceUsage = resUsage

	return overview, nil
}

// CreateCheckoutSession creates a Stripe Checkout session for subscribing to a plan.
func (s *BillingService) CreateCheckoutSession(ctx context.Context, accountID uuid.UUID, planSlug, successURL, cancelURL string) (string, error) {
	if s.stripe == nil {
		return "", fmt.Errorf("billing: stripe not configured")
	}

	account, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("billing: get account: %w", err)
	}
	if account.StripeCustomerID == nil {
		if err := s.ensureStripeCustomer(ctx, account); err != nil {
			return "", err
		}
	}

	plan, err := s.repo.GetPlanBySlug(ctx, planSlug)
	if err != nil {
		return "", fmt.Errorf("billing: get plan: %w", err)
	}
	if plan.StripePriceID == "" {
		return "", fmt.Errorf("billing: plan %s has no Stripe price", planSlug)
	}

	session, err := s.stripe.CreateCheckoutSession(ctx, *account.StripeCustomerID, plan.StripePriceID, successURL, cancelURL)
	if err != nil {
		return "", fmt.Errorf("billing: create checkout: %w", err)
	}

	return session.URL, nil
}

// CreatePortalSession creates a Stripe Billing Portal session for self-service management.
func (s *BillingService) CreatePortalSession(ctx context.Context, accountID uuid.UUID, returnURL string) (string, error) {
	if s.stripe == nil {
		return "", fmt.Errorf("billing: stripe not configured")
	}

	account, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("billing: get account: %w", err)
	}
	if account.StripeCustomerID == nil {
		if err := s.ensureStripeCustomer(ctx, account); err != nil {
			return "", err
		}
	}

	session, err := s.stripe.CreatePortalSession(ctx, *account.StripeCustomerID, returnURL)
	if err != nil {
		return "", fmt.Errorf("billing: create portal session: %w", err)
	}

	return session.URL, nil
}

// CancelSubscription cancels the active subscription.
func (s *BillingService) CancelSubscription(ctx context.Context, accountID uuid.UUID, atPeriodEnd bool) error {
	sub, err := s.repo.GetActiveSubscription(ctx, accountID)
	if err != nil {
		return fmt.Errorf("billing: get subscription: %w", err)
	}

	if s.stripe != nil && sub.StripeSubscriptionID != nil {
		_, err := s.stripe.CancelSubscription(ctx, *sub.StripeSubscriptionID, atPeriodEnd)
		if err != nil {
			return fmt.Errorf("billing: cancel stripe subscription: %w", err)
		}
	}

	if atPeriodEnd {
		sub.CancelAtPeriodEnd = true
		now := time.Now()
		sub.CanceledAt = &now
	} else {
		sub.Status = models.SubscriptionCanceled
		now := time.Now()
		sub.CanceledAt = &now
	}

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("billing: update subscription: %w", err)
	}

	s.logger.Info().
		Str("account_id", accountID.String()).
		Bool("at_period_end", atPeriodEnd).
		Msg("subscription canceled")

	return nil
}

// ensureStripeCustomer creates a Stripe customer for the billing account if one doesn't exist,
// then persists the customer ID back to the database. The account's StripeCustomerID field
// is updated in-place so callers can use it immediately.
func (s *BillingService) ensureStripeCustomer(ctx context.Context, account *models.BillingAccount) error {
	name := account.Email
	if account.Name != nil {
		name = *account.Name
	}

	cust, err := s.stripe.CreateCustomer(ctx, account.Email, name, map[string]string{
		"billing_account_id": account.ID.String(),
	})
	if err != nil {
		return fmt.Errorf("billing: create stripe customer: %w", err)
	}

	if err := s.repo.UpdateStripeCustomerID(ctx, account.ID, cust.ID); err != nil {
		return fmt.Errorf("billing: save stripe customer id: %w", err)
	}

	account.StripeCustomerID = &cust.ID

	s.logger.Info().
		Str("account_id", account.ID.String()).
		Str("stripe_customer_id", cust.ID).
		Msg("created Stripe customer for billing account")

	return nil
}

// --- Usage ---

// RecordUsage records a usage event for a billing account.
func (s *BillingService) RecordUsage(ctx context.Context, accountID uuid.UUID, projectID *uuid.UUID, eventType string, quantity int64) error {
	event := &models.UsageEvent{
		BillingAccountID: accountID,
		ProjectID:        projectID,
		EventType:        eventType,
		Quantity:         quantity,
	}
	return s.repo.RecordUsageEvent(ctx, event)
}

// GetUsageSummary returns aggregated usage since a given time.
func (s *BillingService) GetUsageSummary(ctx context.Context, accountID uuid.UUID, since time.Time) (map[string]int64, error) {
	return s.repo.GetUsageSummary(ctx, accountID, since)
}

// --- Invoices ---

// ListInvoices returns paginated invoices for a billing account.
func (s *BillingService) ListInvoices(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]models.Invoice, error) {
	return s.repo.ListInvoices(ctx, accountID, limit, offset)
}

// --- Webhook Processing ---

// HandleSubscriptionUpdated processes a Stripe subscription update event.
func (s *BillingService) HandleSubscriptionUpdated(ctx context.Context, stripeSubID string, status models.SubscriptionStatus, periodStart, periodEnd *time.Time, cancelAtPeriodEnd bool) error {
	sub, err := s.repo.GetSubscriptionByStripeID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("billing: find subscription %s: %w", stripeSubID, err)
	}

	sub.Status = status
	sub.CurrentPeriodStart = periodStart
	sub.CurrentPeriodEnd = periodEnd
	sub.CancelAtPeriodEnd = cancelAtPeriodEnd

	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("billing: update subscription: %w", err)
	}

	s.logger.Info().
		Str("stripe_sub_id", stripeSubID).
		Str("status", string(status)).
		Msg("subscription updated from webhook")

	return nil
}

// HandleInvoicePaid processes a paid invoice from Stripe webhook.
func (s *BillingService) HandleInvoicePaid(ctx context.Context, stripeCustomerID, stripeInvoiceID string, amountCents int, currency, pdfURL, hostedURL string, periodStart, periodEnd *time.Time) error {
	account, err := s.repo.GetAccountByStripeCustomerID(ctx, stripeCustomerID)
	if err != nil {
		return fmt.Errorf("billing: find account for customer %s: %w", stripeCustomerID, err)
	}

	invoice := &models.Invoice{
		BillingAccountID: account.ID,
		StripeInvoiceID:  stripeInvoiceID,
		AmountCents:      amountCents,
		Currency:         currency,
		Status:           "paid",
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		PDFURL:           &pdfURL,
		HostedInvoiceURL: &hostedURL,
	}

	if err := s.repo.UpsertInvoice(ctx, invoice); err != nil {
		return fmt.Errorf("billing: upsert invoice: %w", err)
	}

	s.logger.Info().
		Str("invoice_id", stripeInvoiceID).
		Int("amount_cents", amountCents).
		Msg("invoice recorded from webhook")

	return nil
}

// HandleInvoicePaymentFailed marks the subscription as past_due when a Stripe invoice payment fails.
func (s *BillingService) HandleInvoicePaymentFailed(ctx context.Context, stripeCustomerID, stripeInvoiceID string) error {
	account, err := s.repo.GetAccountByStripeCustomerID(ctx, stripeCustomerID)
	if err != nil {
		return fmt.Errorf("billing: find account for customer %s: %w", stripeCustomerID, err)
	}

	sub, err := s.repo.GetActiveSubscription(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("billing: get active subscription for account %s: %w", account.ID, err)
	}

	sub.Status = models.SubscriptionPastDue
	if err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("billing: update subscription to past_due: %w", err)
	}

	s.logger.Warn().
		Str("account_id", account.ID.String()).
		Str("invoice_id", stripeInvoiceID).
		Msg("invoice payment failed, subscription marked past_due")

	return nil
}

// HandleCheckoutCompleted processes a completed Stripe Checkout session.
func (s *BillingService) HandleCheckoutCompleted(ctx context.Context, stripeCustomerID, stripeSubID string) error {
	account, err := s.repo.GetAccountByStripeCustomerID(ctx, stripeCustomerID)
	if err != nil {
		return fmt.Errorf("billing: find account for customer %s: %w", stripeCustomerID, err)
	}

	// Check if subscription already exists
	_, err = s.repo.GetSubscriptionByStripeID(ctx, stripeSubID)
	if err == nil {
		return nil // Already tracked
	}

	// Retrieve the Stripe subscription to get the price/plan details
	stripeSub, err := s.stripe.GetSubscription(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("billing: retrieve stripe subscription %s: %w", stripeSubID, err)
	}

	// Extract the price ID from the first subscription item
	if len(stripeSub.Items.Data) == 0 {
		return fmt.Errorf("billing: stripe subscription %s has no items", stripeSubID)
	}
	item := stripeSub.Items.Data[0]
	priceID := ""
	if item.Price != nil {
		priceID = item.Price.ID
	}
	if priceID == "" {
		return fmt.Errorf("billing: stripe subscription %s item has no price", stripeSubID)
	}

	// Find the matching local plan by Stripe price ID
	plan, err := s.repo.GetPlanByStripePriceID(ctx, priceID)
	if err != nil {
		return fmt.Errorf("billing: no local plan for stripe price %s: %w", priceID, err)
	}

	// Deactivate any existing active subscription for this account
	existingSub, err := s.repo.GetActiveSubscription(ctx, account.ID)
	if err == nil && existingSub != nil {
		existingSub.Status = models.SubscriptionCanceled
		now := time.Now()
		existingSub.CanceledAt = &now
		if updateErr := s.repo.UpdateSubscription(ctx, existingSub); updateErr != nil {
			return fmt.Errorf("billing: deactivate old subscription %s: %w", existingSub.ID, updateErr)
		}
	}

	// Extract period dates from subscription item
	var periodStart, periodEnd *time.Time
	if item.CurrentPeriodStart > 0 {
		t := time.Unix(item.CurrentPeriodStart, 0)
		periodStart = &t
	}
	if item.CurrentPeriodEnd > 0 {
		t := time.Unix(item.CurrentPeriodEnd, 0)
		periodEnd = &t
	}

	// Create the local subscription record
	sub := &models.Subscription{
		BillingAccountID:     account.ID,
		PlanID:               plan.ID,
		StripeSubscriptionID: &stripeSubID,
		Status:               models.SubscriptionActive,
		SeatCount:            int(item.Quantity),
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
	}
	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("billing: create subscription record: %w", err)
	}

	s.logger.Info().
		Str("account_id", account.ID.String()).
		Str("stripe_sub_id", stripeSubID).
		Str("plan", plan.Slug).
		Msg("checkout completed, subscription created")

	return nil
}
