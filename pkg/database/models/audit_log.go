package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog represents a single audit log entry.
type AuditLog struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	SequenceNum  int64      `json:"sequence_num" db:"sequence_num"`
	TeamID       uuid.UUID  `json:"team_id" db:"team_id"`
	ProjectID    *uuid.UUID `json:"project_id,omitempty" db:"project_id"`
	ActorType    string     `json:"actor_type" db:"actor_type"`
	ActorID      *uuid.UUID `json:"actor_id,omitempty" db:"actor_id"`
	ActorEmail   string     `json:"actor_email" db:"actor_email"`
	Action       string     `json:"action" db:"action"`
	Category     string     `json:"category" db:"category"`
	ResourceType string     `json:"resource_type" db:"resource_type"`
	ResourceID   string     `json:"resource_id" db:"resource_id"`
	Description  string     `json:"description" db:"description"`
	Metadata     []byte     `json:"metadata" db:"metadata"`
	IPAddress    string     `json:"ip_address" db:"ip_address"`
	UserAgent    string     `json:"user_agent" db:"user_agent"`
	RequestID    string     `json:"request_id" db:"request_id"`
	PrevHash     string     `json:"prev_hash" db:"prev_hash"`
	EntryHash    string     `json:"entry_hash" db:"entry_hash"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// AuditWebhook represents a configured webhook for forwarding audit log entries.
type AuditWebhook struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	TeamID          uuid.UUID  `json:"team_id" db:"team_id"`
	URL             string     `json:"url" db:"url"`
	SecretEncrypted string     `json:"-" db:"secret_encrypted"`
	Description     string     `json:"description" db:"description"`
	EventFilter     []string   `json:"event_filter" db:"event_filter"`
	IsActive        bool       `json:"is_active" db:"is_active"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy       *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
}

// AuditLogRetentionPolicy defines how long audit logs are kept per team.
type AuditLogRetentionPolicy struct {
	TeamID        uuid.UUID  `json:"team_id" db:"team_id"`
	RetentionDays int        `json:"retention_days" db:"retention_days"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	UpdatedBy     *uuid.UUID `json:"updated_by,omitempty" db:"updated_by"`
}

// Actor types
const (
	AuditActorUser   = "user"
	AuditActorAPIKey = "api_key"
	AuditActorSystem = "system"
)

// Audit log categories
const (
	AuditCategoryAuth    = "auth"
	AuditCategoryProject = "project"
	AuditCategoryMember  = "member"
	AuditCategoryTeam    = "team"
	AuditCategoryAPIKey  = "api_key"
	AuditCategoryCache   = "cache"
	AuditCategoryRun     = "run"
	AuditCategoryAudit   = "audit_access"
	AuditCategorySystem  = "system"
)

// Audit log actions
const (
	// Auth actions
	AuditActionAuthLogin  = "auth.login"
	AuditActionAuthLogout = "auth.logout"

	// Project actions
	AuditActionProjectCreated = "project.created"
	AuditActionProjectUpdated = "project.updated"
	AuditActionProjectDeleted = "project.deleted"

	// Member actions
	AuditActionMemberAdded       = "member.added"
	AuditActionMemberRemoved     = "member.removed"
	AuditActionMemberRoleChanged = "member.role_changed"

	// Team actions
	AuditActionTeamCreated = "team.created"
	AuditActionTeamUpdated = "team.updated"
	AuditActionTeamDeleted = "team.deleted"

	// API key actions
	AuditActionAPIKeyCreated = "api_key.created"
	AuditActionAPIKeyRevoked = "api_key.revoked"

	// Cache actions
	AuditActionCacheCleared = "cache.cleared"

	// Audit access actions (meta-audit)
	AuditActionAuditViewed   = "audit.viewed"
	AuditActionAuditExported = "audit.exported"

	// System actions
	AuditActionRetentionGC    = "system.retention_gc"
	AuditActionRetentionEpoch = "system.retention_epoch"

	// Billing actions (used by cloud billing webhooks)
	AuditActionBillingSubscriptionCreated  = "billing.subscription_created"
	AuditActionBillingSubscriptionUpdated  = "billing.subscription_updated"
	AuditActionBillingSubscriptionCanceled = "billing.subscription_canceled"
	AuditActionBillingPaymentFailed        = "billing.payment_failed"
	AuditActionBillingPlanChanged          = "billing.plan_changed"
	AuditActionBillingInvoiceGenerated     = "billing.invoice_generated"

	// SSO actions (used by cloud SSO integration)
	AuditActionSSOProviderConfigured = "sso.provider_configured"
	AuditActionSSOProviderRemoved    = "sso.provider_removed"

	// Team billing actions (used by cloud billing linkage)
	AuditActionTeamBillingLinked   = "team.billing_linked"
	AuditActionTeamBillingUnlinked = "team.billing_unlinked"
)

// Audit log categories (continued)
const (
	AuditCategoryBilling = "billing"
	AuditCategorySSO     = "sso"
)
