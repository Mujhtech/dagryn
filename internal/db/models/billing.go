package models

import (
	"time"

	"github.com/google/uuid"
)

// BillingPlan represents a pricing tier.
type BillingPlan struct {
	ID            uuid.UUID `json:"id" db:"id"`
	StripePriceID string    `json:"stripe_price_id" db:"stripe_price_id"`
	Name          string    `json:"name" db:"name"`
	Slug          string    `json:"slug" db:"slug"`
	DisplayName   string    `json:"display_name" db:"display_name"`
	Description   *string   `json:"description,omitempty" db:"description"`
	PriceCents    int       `json:"price_cents" db:"price_cents"`
	BillingPeriod string    `json:"billing_period" db:"billing_period"`
	IsPerSeat     bool      `json:"is_per_seat" db:"is_per_seat"`

	// Quota limits (nil = unlimited)
	MaxProjects           *int   `json:"max_projects,omitempty" db:"max_projects"`
	MaxTeamMembers        *int   `json:"max_team_members,omitempty" db:"max_team_members"`
	MaxCacheBytes         *int64 `json:"max_cache_bytes,omitempty" db:"max_cache_bytes"`
	MaxStorageBytes       *int64 `json:"max_storage_bytes,omitempty" db:"max_storage_bytes"`
	MaxBandwidthBytes     *int64 `json:"max_bandwidth_bytes,omitempty" db:"max_bandwidth_bytes"`
	MaxConcurrentRuns     *int   `json:"max_concurrent_runs,omitempty" db:"max_concurrent_runs"`
	CacheTTLDays          *int   `json:"cache_ttl_days,omitempty" db:"cache_ttl_days"`
	ArtifactRetentionDays *int   `json:"artifact_retention_days,omitempty" db:"artifact_retention_days"`
	LogRetentionDays      *int   `json:"log_retention_days,omitempty" db:"log_retention_days"`

	// Feature flags
	ContainerExecution bool `json:"container_execution" db:"container_execution"`
	PriorityQueue      bool `json:"priority_queue" db:"priority_queue"`
	SSOEnabled         bool `json:"sso_enabled" db:"sso_enabled"`
	AuditLogs          bool `json:"audit_logs" db:"audit_logs"`

	IsActive  bool      `json:"is_active" db:"is_active"`
	SortOrder int       `json:"sort_order" db:"sort_order"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// BillingAccount ties a user or team to a Stripe customer.
type BillingAccount struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	UserID           *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	TeamID           *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	StripeCustomerID *string    `json:"stripe_customer_id,omitempty" db:"stripe_customer_id"`
	Email            string     `json:"email" db:"email"`
	Name             *string    `json:"name,omitempty" db:"name"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// SubscriptionStatus represents the state of a subscription.
type SubscriptionStatus string

const (
	SubscriptionActive     SubscriptionStatus = "active"
	SubscriptionTrialing   SubscriptionStatus = "trialing"
	SubscriptionPastDue    SubscriptionStatus = "past_due"
	SubscriptionCanceled   SubscriptionStatus = "canceled"
	SubscriptionUnpaid     SubscriptionStatus = "unpaid"
	SubscriptionIncomplete SubscriptionStatus = "incomplete"
)

// Subscription represents a billing account's plan subscription.
type Subscription struct {
	ID                   uuid.UUID          `json:"id" db:"id"`
	BillingAccountID     uuid.UUID          `json:"billing_account_id" db:"billing_account_id"`
	PlanID               uuid.UUID          `json:"plan_id" db:"plan_id"`
	StripeSubscriptionID *string            `json:"stripe_subscription_id,omitempty" db:"stripe_subscription_id"`
	Status               SubscriptionStatus `json:"status" db:"status"`
	SeatCount            int                `json:"seat_count" db:"seat_count"`
	CurrentPeriodStart   *time.Time         `json:"current_period_start,omitempty" db:"current_period_start"`
	CurrentPeriodEnd     *time.Time         `json:"current_period_end,omitempty" db:"current_period_end"`
	CancelAtPeriodEnd    bool               `json:"cancel_at_period_end" db:"cancel_at_period_end"`
	CanceledAt           *time.Time         `json:"canceled_at,omitempty" db:"canceled_at"`
	TrialEnd             *time.Time         `json:"trial_end,omitempty" db:"trial_end"`
	CreatedAt            time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at" db:"updated_at"`
}

// UsageEvent records a metered usage event.
type UsageEvent struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	BillingAccountID uuid.UUID  `json:"billing_account_id" db:"billing_account_id"`
	ProjectID        *uuid.UUID `json:"project_id,omitempty" db:"project_id"`
	EventType        string     `json:"event_type" db:"event_type"`
	Quantity         int64      `json:"quantity" db:"quantity"`
	Metadata         any        `json:"metadata,omitempty" db:"metadata"`
	RecordedAt       time.Time  `json:"recorded_at" db:"recorded_at"`
	ReportedToStripe bool       `json:"reported_to_stripe" db:"reported_to_stripe"`
	StripeUsageID    *string    `json:"stripe_usage_id,omitempty" db:"stripe_usage_id"`
}

// Invoice is a local cache of a Stripe invoice.
type Invoice struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	BillingAccountID uuid.UUID  `json:"billing_account_id" db:"billing_account_id"`
	StripeInvoiceID  string     `json:"stripe_invoice_id" db:"stripe_invoice_id"`
	AmountCents      int        `json:"amount_cents" db:"amount_cents"`
	Currency         string     `json:"currency" db:"currency"`
	Status           string     `json:"status" db:"status"`
	PeriodStart      *time.Time `json:"period_start,omitempty" db:"period_start"`
	PeriodEnd        *time.Time `json:"period_end,omitempty" db:"period_end"`
	PDFURL           *string    `json:"pdf_url,omitempty" db:"pdf_url"`
	HostedInvoiceURL *string    `json:"hosted_invoice_url,omitempty" db:"hosted_invoice_url"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}

// Usage event type constants.
const (
	UsageEventCacheUpload   = "cache_upload"
	UsageEventCacheDownload = "cache_download"
	UsageEventRunMinute     = "run_minute"
	UsageEventBandwidth     = "bandwidth"
)
