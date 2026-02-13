package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/internal/db/models"
)

// BillingRepo handles billing database operations.
type BillingRepo struct {
	pool *pgxpool.Pool
}

// NewBillingRepo creates a new billing repository.
func NewBillingRepo(pool *pgxpool.Pool) *BillingRepo {
	return &BillingRepo{pool: pool}
}

// --- Plans ---

// ListActivePlans returns all active billing plans ordered by sort_order.
func (r *BillingRepo) ListActivePlans(ctx context.Context) ([]models.BillingPlan, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stripe_price_id, name, slug, display_name, description,
		       price_cents, billing_period, is_per_seat,
		       max_projects, max_team_members, max_cache_bytes, max_storage_bytes, max_bandwidth_bytes,
		       max_concurrent_runs, cache_ttl_days, artifact_retention_days, log_retention_days,
		       container_execution, priority_queue, sso_enabled, audit_logs,
		       is_active, sort_order, created_at, updated_at
		FROM billing_plans
		WHERE is_active = TRUE
		ORDER BY sort_order ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []models.BillingPlan
	for rows.Next() {
		var p models.BillingPlan
		if err := rows.Scan(
			&p.ID, &p.StripePriceID, &p.Name, &p.Slug, &p.DisplayName, &p.Description,
			&p.PriceCents, &p.BillingPeriod, &p.IsPerSeat,
			&p.MaxProjects, &p.MaxTeamMembers, &p.MaxCacheBytes, &p.MaxStorageBytes, &p.MaxBandwidthBytes,
			&p.MaxConcurrentRuns, &p.CacheTTLDays, &p.ArtifactRetentionDays, &p.LogRetentionDays,
			&p.ContainerExecution, &p.PriorityQueue, &p.SSOEnabled, &p.AuditLogs,
			&p.IsActive, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

// GetPlanBySlug returns a plan by its URL slug.
func (r *BillingRepo) GetPlanBySlug(ctx context.Context, slug string) (*models.BillingPlan, error) {
	return r.scanPlan(ctx, `
		SELECT id, stripe_price_id, name, slug, display_name, description,
		       price_cents, billing_period, is_per_seat,
		       max_projects, max_team_members, max_cache_bytes, max_storage_bytes, max_bandwidth_bytes,
		       max_concurrent_runs, cache_ttl_days, artifact_retention_days, log_retention_days,
		       container_execution, priority_queue, sso_enabled, audit_logs,
		       is_active, sort_order, created_at, updated_at
		FROM billing_plans WHERE slug = $1
	`, slug)
}

// GetPlanByID returns a plan by its ID.
func (r *BillingRepo) GetPlanByID(ctx context.Context, id uuid.UUID) (*models.BillingPlan, error) {
	return r.scanPlan(ctx, `
		SELECT id, stripe_price_id, name, slug, display_name, description,
		       price_cents, billing_period, is_per_seat,
		       max_projects, max_team_members, max_cache_bytes, max_storage_bytes, max_bandwidth_bytes,
		       max_concurrent_runs, cache_ttl_days, artifact_retention_days, log_retention_days,
		       container_execution, priority_queue, sso_enabled, audit_logs,
		       is_active, sort_order, created_at, updated_at
		FROM billing_plans WHERE id = $1
	`, id)
}

// GetPlanByStripePriceID looks up a plan by its Stripe Price ID.
func (r *BillingRepo) GetPlanByStripePriceID(ctx context.Context, priceID string) (*models.BillingPlan, error) {
	return r.scanPlan(ctx, `
		SELECT id, stripe_price_id, name, slug, display_name, description,
		       price_cents, billing_period, is_per_seat,
		       max_projects, max_team_members, max_cache_bytes, max_storage_bytes, max_bandwidth_bytes,
		       max_concurrent_runs, cache_ttl_days, artifact_retention_days, log_retention_days,
		       container_execution, priority_queue, sso_enabled, audit_logs,
		       is_active, sort_order, created_at, updated_at
		FROM billing_plans WHERE stripe_price_id = $1
	`, priceID)
}

func (r *BillingRepo) scanPlan(ctx context.Context, query string, args ...any) (*models.BillingPlan, error) {
	var p models.BillingPlan
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.StripePriceID, &p.Name, &p.Slug, &p.DisplayName, &p.Description,
		&p.PriceCents, &p.BillingPeriod, &p.IsPerSeat,
		&p.MaxProjects, &p.MaxTeamMembers, &p.MaxCacheBytes, &p.MaxStorageBytes, &p.MaxBandwidthBytes,
		&p.MaxConcurrentRuns, &p.CacheTTLDays, &p.ArtifactRetentionDays, &p.LogRetentionDays,
		&p.ContainerExecution, &p.PriorityQueue, &p.SSOEnabled, &p.AuditLogs,
		&p.IsActive, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// --- Billing Accounts ---

// GetAccountByID returns a billing account by ID.
func (r *BillingRepo) GetAccountByID(ctx context.Context, id uuid.UUID) (*models.BillingAccount, error) {
	return r.scanAccount(ctx, `
		SELECT id, user_id, team_id, stripe_customer_id, email, name, created_at, updated_at
		FROM billing_accounts WHERE id = $1
	`, id)
}

// GetAccountByUserID returns the billing account for a user.
func (r *BillingRepo) GetAccountByUserID(ctx context.Context, userID uuid.UUID) (*models.BillingAccount, error) {
	return r.scanAccount(ctx, `
		SELECT id, user_id, team_id, stripe_customer_id, email, name, created_at, updated_at
		FROM billing_accounts WHERE user_id = $1
	`, userID)
}

// GetAccountByTeamID returns the billing account for a team.
func (r *BillingRepo) GetAccountByTeamID(ctx context.Context, teamID uuid.UUID) (*models.BillingAccount, error) {
	return r.scanAccount(ctx, `
		SELECT id, user_id, team_id, stripe_customer_id, email, name, created_at, updated_at
		FROM billing_accounts WHERE team_id = $1
	`, teamID)
}

// GetAccountByStripeCustomerID looks up an account by its Stripe Customer ID.
func (r *BillingRepo) GetAccountByStripeCustomerID(ctx context.Context, customerID string) (*models.BillingAccount, error) {
	return r.scanAccount(ctx, `
		SELECT id, user_id, team_id, stripe_customer_id, email, name, created_at, updated_at
		FROM billing_accounts WHERE stripe_customer_id = $1
	`, customerID)
}

// CreateAccount inserts a new billing account.
func (r *BillingRepo) CreateAccount(ctx context.Context, account *models.BillingAccount) error {
	if account.ID == uuid.Nil {
		account.ID = uuid.New()
	}
	account.CreatedAt = time.Now()
	account.UpdatedAt = account.CreatedAt

	_, err := r.pool.Exec(ctx, `
		INSERT INTO billing_accounts (id, user_id, team_id, stripe_customer_id, email, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, account.ID, account.UserID, account.TeamID, account.StripeCustomerID,
		account.Email, account.Name, account.CreatedAt, account.UpdatedAt)
	return err
}

// UpdateStripeCustomerID sets the Stripe Customer ID on a billing account.
func (r *BillingRepo) UpdateStripeCustomerID(ctx context.Context, accountID uuid.UUID, customerID string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE billing_accounts SET stripe_customer_id = $1, updated_at = NOW()
		WHERE id = $2
	`, customerID, accountID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BillingRepo) scanAccount(ctx context.Context, query string, args ...any) (*models.BillingAccount, error) {
	var a models.BillingAccount
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&a.ID, &a.UserID, &a.TeamID, &a.StripeCustomerID,
		&a.Email, &a.Name, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// --- Subscriptions ---

// GetActiveSubscription returns the active subscription for a billing account.
func (r *BillingRepo) GetActiveSubscription(ctx context.Context, accountID uuid.UUID) (*models.Subscription, error) {
	return r.scanSubscription(ctx, `
		SELECT id, billing_account_id, plan_id, stripe_subscription_id, status,
		       seat_count, current_period_start, current_period_end,
		       cancel_at_period_end, canceled_at, trial_end, created_at, updated_at
		FROM subscriptions
		WHERE billing_account_id = $1 AND status IN ('active', 'trialing')
		ORDER BY created_at DESC
		LIMIT 1
	`, accountID)
}

// GetSubscriptionByStatus returns the most recent subscription for an account matching any of the given statuses.
func (r *BillingRepo) GetSubscriptionByStatus(ctx context.Context, accountID uuid.UUID, statuses ...models.SubscriptionStatus) (*models.Subscription, error) {
	statusStrings := make([]string, len(statuses))
	for i, s := range statuses {
		statusStrings[i] = string(s)
	}
	return r.scanSubscription(ctx, `
		SELECT id, billing_account_id, plan_id, stripe_subscription_id, status,
		       seat_count, current_period_start, current_period_end,
		       cancel_at_period_end, canceled_at, trial_end, created_at, updated_at
		FROM subscriptions
		WHERE billing_account_id = $1 AND status = ANY($2)
		ORDER BY created_at DESC
		LIMIT 1
	`, accountID, statusStrings)
}

// GetSubscriptionByStripeID looks up a subscription by its Stripe Subscription ID.
func (r *BillingRepo) GetSubscriptionByStripeID(ctx context.Context, stripeSubID string) (*models.Subscription, error) {
	return r.scanSubscription(ctx, `
		SELECT id, billing_account_id, plan_id, stripe_subscription_id, status,
		       seat_count, current_period_start, current_period_end,
		       cancel_at_period_end, canceled_at, trial_end, created_at, updated_at
		FROM subscriptions
		WHERE stripe_subscription_id = $1
	`, stripeSubID)
}

// CreateSubscription inserts a new subscription.
func (r *BillingRepo) CreateSubscription(ctx context.Context, sub *models.Subscription) error {
	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = sub.CreatedAt

	_, err := r.pool.Exec(ctx, `
		INSERT INTO subscriptions (
			id, billing_account_id, plan_id, stripe_subscription_id, status,
			seat_count, current_period_start, current_period_end,
			cancel_at_period_end, canceled_at, trial_end, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, sub.ID, sub.BillingAccountID, sub.PlanID, sub.StripeSubscriptionID, sub.Status,
		sub.SeatCount, sub.CurrentPeriodStart, sub.CurrentPeriodEnd,
		sub.CancelAtPeriodEnd, sub.CanceledAt, sub.TrialEnd, sub.CreatedAt, sub.UpdatedAt)
	return err
}

// UpdateSubscription updates an existing subscription.
func (r *BillingRepo) UpdateSubscription(ctx context.Context, sub *models.Subscription) error {
	sub.UpdatedAt = time.Now()
	result, err := r.pool.Exec(ctx, `
		UPDATE subscriptions SET
			plan_id = $1, stripe_subscription_id = $2, status = $3,
			seat_count = $4, current_period_start = $5, current_period_end = $6,
			cancel_at_period_end = $7, canceled_at = $8, trial_end = $9, updated_at = $10
		WHERE id = $11
	`, sub.PlanID, sub.StripeSubscriptionID, sub.Status,
		sub.SeatCount, sub.CurrentPeriodStart, sub.CurrentPeriodEnd,
		sub.CancelAtPeriodEnd, sub.CanceledAt, sub.TrialEnd, sub.UpdatedAt, sub.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BillingRepo) scanSubscription(ctx context.Context, query string, args ...any) (*models.Subscription, error) {
	var s models.Subscription
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&s.ID, &s.BillingAccountID, &s.PlanID, &s.StripeSubscriptionID, &s.Status,
		&s.SeatCount, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.CancelAtPeriodEnd, &s.CanceledAt, &s.TrialEnd, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// --- Usage Events ---

// RecordUsageEvent inserts a new usage event.
func (r *BillingRepo) RecordUsageEvent(ctx context.Context, event *models.UsageEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	event.RecordedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO usage_events (id, billing_account_id, project_id, event_type, quantity, metadata, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, event.ID, event.BillingAccountID, event.ProjectID, event.EventType,
		event.Quantity, event.Metadata, event.RecordedAt)
	return err
}

// ListUnreportedEvents returns unreported usage events for a billing account.
func (r *BillingRepo) ListUnreportedEvents(ctx context.Context, accountID uuid.UUID, limit int) ([]models.UsageEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, billing_account_id, project_id, event_type, quantity, metadata,
		       recorded_at, reported_to_stripe, stripe_usage_id
		FROM usage_events
		WHERE billing_account_id = $1 AND reported_to_stripe = FALSE
		ORDER BY recorded_at ASC
		LIMIT $2
	`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.UsageEvent
	for rows.Next() {
		var e models.UsageEvent
		if err := rows.Scan(
			&e.ID, &e.BillingAccountID, &e.ProjectID, &e.EventType,
			&e.Quantity, &e.Metadata, &e.RecordedAt, &e.ReportedToStripe, &e.StripeUsageID,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// MarkEventsReported marks usage events as reported to Stripe.
func (r *BillingRepo) MarkEventsReported(ctx context.Context, eventIDs []uuid.UUID, stripeUsageID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE usage_events
		SET reported_to_stripe = TRUE, stripe_usage_id = $1
		WHERE id = ANY($2)
	`, stripeUsageID, eventIDs)
	return err
}

// GetUsageSummary returns aggregated usage by event type since a given time.
func (r *BillingRepo) GetUsageSummary(ctx context.Context, accountID uuid.UUID, since time.Time) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_type, COALESCE(SUM(quantity), 0) AS total
		FROM usage_events
		WHERE billing_account_id = $1 AND recorded_at >= $2
		GROUP BY event_type
	`, accountID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var eventType string
		var total int64
		if err := rows.Scan(&eventType, &total); err != nil {
			return nil, err
		}
		result[eventType] = total
	}
	return result, rows.Err()
}

// --- Invoices ---

// UpsertInvoice inserts or updates an invoice record.
func (r *BillingRepo) UpsertInvoice(ctx context.Context, invoice *models.Invoice) error {
	if invoice.ID == uuid.Nil {
		invoice.ID = uuid.New()
	}
	invoice.CreatedAt = time.Now()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO invoices (
			id, billing_account_id, stripe_invoice_id, amount_cents, currency,
			status, period_start, period_end, pdf_url, hosted_invoice_url, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (stripe_invoice_id) DO UPDATE SET
			amount_cents = EXCLUDED.amount_cents,
			status = EXCLUDED.status,
			pdf_url = EXCLUDED.pdf_url,
			hosted_invoice_url = EXCLUDED.hosted_invoice_url
	`, invoice.ID, invoice.BillingAccountID, invoice.StripeInvoiceID,
		invoice.AmountCents, invoice.Currency, invoice.Status,
		invoice.PeriodStart, invoice.PeriodEnd, invoice.PDFURL,
		invoice.HostedInvoiceURL, invoice.CreatedAt)
	return err
}

// ListInvoices returns paginated invoices for a billing account.
func (r *BillingRepo) ListInvoices(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]models.Invoice, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, billing_account_id, stripe_invoice_id, amount_cents, currency,
		       status, period_start, period_end, pdf_url, hosted_invoice_url, created_at
		FROM invoices
		WHERE billing_account_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		if err := rows.Scan(
			&inv.ID, &inv.BillingAccountID, &inv.StripeInvoiceID,
			&inv.AmountCents, &inv.Currency, &inv.Status,
			&inv.PeriodStart, &inv.PeriodEnd, &inv.PDFURL,
			&inv.HostedInvoiceURL, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

// --- Quota Helpers ---

// CountProjectsByAccount returns the number of projects linked to a billing account.
func (r *BillingRepo) CountProjectsByAccount(ctx context.Context, accountID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM projects WHERE billing_account_id = $1
	`, accountID).Scan(&count)
	return count, err
}

// CountTeamMembers returns the number of members in a team.
func (r *BillingRepo) CountTeamMembers(ctx context.Context, teamID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM team_members WHERE team_id = $1
	`, teamID).Scan(&count)
	return count, err
}

// CountActiveRunsByAccount returns the number of running runs across all projects for an account.
func (r *BillingRepo) CountActiveRunsByAccount(ctx context.Context, accountID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM runs r
		JOIN projects p ON r.project_id = p.id
		WHERE p.billing_account_id = $1 AND r.status = 'running'
	`, accountID).Scan(&count)
	return count, err
}

// GetCacheQuotaByAccount returns the aggregated cache usage across all projects for an account.
func (r *BillingRepo) GetCacheQuotaByAccount(ctx context.Context, accountID uuid.UUID) (currentBytes int64, bandwidthBytes int64, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cq.current_size_bytes), 0),
		       COALESCE(SUM(cq.current_bandwidth_bytes), 0)
		FROM cache_quotas cq
		WHERE cq.billing_account_id = $1
	`, accountID).Scan(&currentBytes, &bandwidthBytes)
	return
}

// ResetBandwidthForAccount resets bandwidth counters for all quotas linked to an account.
func (r *BillingRepo) ResetBandwidthForAccount(ctx context.Context, accountID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE cache_quotas
		SET current_bandwidth_bytes = 0, bandwidth_reset_at = NOW()
		WHERE billing_account_id = $1
	`, accountID)
	return err
}

// ListAccountsWithExpiredBandwidth returns billing account IDs where bandwidth needs resetting.
func (r *BillingRepo) ListAccountsWithExpiredBandwidth(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ba.id
		FROM billing_accounts ba
		JOIN subscriptions s ON s.billing_account_id = ba.id
		WHERE s.status IN ('active', 'trialing')
		  AND s.current_period_start IS NOT NULL
		  AND (
			NOT EXISTS (
				SELECT 1 FROM cache_quotas cq
				WHERE cq.billing_account_id = ba.id AND cq.bandwidth_reset_at >= s.current_period_start
			)
		  )
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListAllBillingAccountIDs returns all billing account IDs with active subscriptions.
func (r *BillingRepo) ListAllBillingAccountIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT billing_account_id FROM subscriptions
		WHERE status IN ('active', 'trialing')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListProjectIDsByAccount returns all project IDs linked to a billing account.
func (r *BillingRepo) ListProjectIDsByAccount(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM projects WHERE billing_account_id = $1
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IncrementBandwidth adds to the current_bandwidth_bytes for a project's cache quota.
// It ensures a quota row exists first so artifact-only projects are tracked correctly.
// Projects without a billing_account_id are silently skipped.
func (r *BillingRepo) IncrementBandwidth(ctx context.Context, projectID uuid.UUID, bytes int64) error {
	// Only proceed if project has a billing account linked
	var billingAccountID *uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT billing_account_id FROM projects WHERE id = $1`, projectID).Scan(&billingAccountID)
	if err != nil || billingAccountID == nil {
		return nil // no billing account = nothing to track
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO cache_quotas (project_id, billing_account_id)
		VALUES ($1, $2)
		ON CONFLICT (project_id) DO NOTHING
	`, projectID, *billingAccountID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE cache_quotas
		SET current_bandwidth_bytes = current_bandwidth_bytes + $1
		WHERE project_id = $2
	`, bytes, projectID)
	return err
}

// GetTotalStorageByAccount returns the total storage bytes (cache + artifacts) across all projects for an account.
func (r *BillingRepo) GetTotalStorageByAccount(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(cq.current_size_bytes), 0) +
			COALESCE((
				SELECT SUM(a.size_bytes) FROM artifacts a
				JOIN projects p2 ON a.project_id = p2.id
				WHERE p2.billing_account_id = $1
			), 0)
		FROM cache_quotas cq
		WHERE cq.billing_account_id = $1
	`, accountID).Scan(&total)
	return total, err
}

// GetTotalArtifactSizeByAccount returns the total artifact size across all projects for an account.
func (r *BillingRepo) GetTotalArtifactSizeByAccount(ctx context.Context, accountID uuid.UUID) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(a.size_bytes), 0)
		FROM artifacts a
		JOIN projects p ON a.project_id = p.id
		WHERE p.billing_account_id = $1
	`, accountID).Scan(&total)
	return total, err
}
