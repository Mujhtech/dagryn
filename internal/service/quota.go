package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/rs/zerolog"
)

// PlanLimits holds the effective limits for a billing account based on its plan.
type PlanLimits struct {
	PlanSlug              string `json:"plan_slug"`
	PlanDisplayName       string `json:"plan_display_name"`
	MaxProjects           *int   `json:"max_projects,omitempty"`
	MaxTeamMembers        *int   `json:"max_team_members,omitempty"`
	MaxCacheBytes         *int64 `json:"max_cache_bytes,omitempty"`
	MaxStorageBytes       *int64 `json:"max_storage_bytes,omitempty"`
	MaxBandwidthBytes     *int64 `json:"max_bandwidth_bytes,omitempty"`
	MaxConcurrentRuns     *int   `json:"max_concurrent_runs,omitempty"`
	CacheTTLDays          *int   `json:"cache_ttl_days,omitempty"`
	ArtifactRetentionDays *int   `json:"artifact_retention_days,omitempty"`
	LogRetentionDays      *int   `json:"log_retention_days,omitempty"`
	ContainerExecution    bool   `json:"container_execution"`
	PriorityQueue         bool   `json:"priority_queue"`
	SSOEnabled            bool   `json:"sso_enabled"`
	AuditLogs             bool   `json:"audit_logs"`
}

// QuotaService checks and enforces plan limits.
type QuotaService struct {
	billing  *repo.BillingRepo
	projects *repo.ProjectRepo
	logger   zerolog.Logger
}

// NewQuotaService creates a new quota enforcement service.
func NewQuotaService(billing *repo.BillingRepo, projects *repo.ProjectRepo, logger zerolog.Logger) *QuotaService {
	return &QuotaService{
		billing:  billing,
		projects: projects,
		logger:   logger.With().Str("service", "quota").Logger(),
	}
}

// GetLimits returns the effective plan limits for a billing account.
func (s *QuotaService) GetLimits(ctx context.Context, accountID uuid.UUID) (*PlanLimits, error) {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	return &PlanLimits{
		PlanSlug:              plan.Slug,
		PlanDisplayName:       plan.DisplayName,
		MaxProjects:           plan.MaxProjects,
		MaxTeamMembers:        plan.MaxTeamMembers,
		MaxCacheBytes:         plan.MaxCacheBytes,
		MaxStorageBytes:       plan.MaxStorageBytes,
		MaxBandwidthBytes:     plan.MaxBandwidthBytes,
		MaxConcurrentRuns:     plan.MaxConcurrentRuns,
		CacheTTLDays:          plan.CacheTTLDays,
		ArtifactRetentionDays: plan.ArtifactRetentionDays,
		LogRetentionDays:      plan.LogRetentionDays,
		ContainerExecution:    plan.ContainerExecution,
		PriorityQueue:         plan.PriorityQueue,
		SSOEnabled:            plan.SSOEnabled,
		AuditLogs:             plan.AuditLogs,
	}, nil
}

// CheckCacheUpload returns nil if the upload is allowed, or a QuotaExceededError.
func (s *QuotaService) CheckCacheUpload(ctx context.Context, accountID uuid.UUID, sizeBytes int64) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil // no plan = no limits
	}
	if plan.MaxCacheBytes == nil {
		return nil // unlimited
	}

	currentBytes, _, err := s.billing.GetCacheQuotaByAccount(ctx, accountID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to get cache usage for quota check")
		return nil // fail open
	}

	if currentBytes+sizeBytes > *plan.MaxCacheBytes {
		return &QuotaExceededError{
			Resource:   "cache_storage",
			Current:    currentBytes,
			Limit:      *plan.MaxCacheBytes,
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// CheckStorageUpload returns nil if the unified storage quota allows the upload, or a QuotaExceededError.
// This checks the account-level max_storage_bytes limit covering all storage (cache + artifacts).
func (s *QuotaService) CheckStorageUpload(ctx context.Context, accountID uuid.UUID, sizeBytes int64) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil // no plan = no limits
	}
	if plan.MaxStorageBytes == nil {
		return nil // unlimited
	}

	totalBytes, err := s.billing.GetTotalStorageByAccount(ctx, accountID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to get total storage for quota check")
		return nil // fail open
	}

	if totalBytes+sizeBytes > *plan.MaxStorageBytes {
		return &QuotaExceededError{
			Resource:   "storage",
			Current:    totalBytes,
			Limit:      *plan.MaxStorageBytes,
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// RecordBandwidthUsage increments the bandwidth counter for a project.
// This should be called after any download or upload that counts toward bandwidth.
func (s *QuotaService) RecordBandwidthUsage(ctx context.Context, projectID uuid.UUID, sizeBytes int64) {
	if err := s.billing.IncrementBandwidth(ctx, projectID, sizeBytes); err != nil {
		s.logger.Warn().Err(err).Str("project_id", projectID.String()).Msg("failed to record bandwidth usage")
	}
}

// CheckBandwidthUsage returns nil if the account's bandwidth quota allows the transfer, or a QuotaExceededError.
// This is the unified bandwidth check covering all transfer types (cache, artifacts, etc.).
func (s *QuotaService) CheckBandwidthUsage(ctx context.Context, accountID uuid.UUID, sizeBytes int64) error {
	return s.checkBandwidth(ctx, accountID, sizeBytes)
}

// CheckCacheDownload returns nil if bandwidth allows, or a QuotaExceededError.
// Deprecated: Use CheckBandwidthUsage for new code.
func (s *QuotaService) CheckCacheDownload(ctx context.Context, accountID uuid.UUID, sizeBytes int64) error {
	return s.checkBandwidth(ctx, accountID, sizeBytes)
}

func (s *QuotaService) checkBandwidth(ctx context.Context, accountID uuid.UUID, sizeBytes int64) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil
	}
	if plan.MaxBandwidthBytes == nil {
		return nil
	}

	_, bandwidthBytes, err := s.billing.GetCacheQuotaByAccount(ctx, accountID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to get bandwidth usage for quota check")
		return nil
	}

	if bandwidthBytes+sizeBytes > *plan.MaxBandwidthBytes {
		return &QuotaExceededError{
			Resource:   "bandwidth",
			Current:    bandwidthBytes,
			Limit:      *plan.MaxBandwidthBytes,
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// CheckProjectCreation returns nil if the account can create another project.
func (s *QuotaService) CheckProjectCreation(ctx context.Context, accountID uuid.UUID) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil
	}
	if plan.MaxProjects == nil {
		return nil
	}

	count, err := s.billing.CountProjectsByAccount(ctx, accountID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to count projects for quota check")
		return nil
	}

	if count >= *plan.MaxProjects {
		return &QuotaExceededError{
			Resource:   "projects",
			Current:    int64(count),
			Limit:      int64(*plan.MaxProjects),
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// CheckTeamMember returns nil if the team can add another member.
func (s *QuotaService) CheckTeamMember(ctx context.Context, accountID uuid.UUID, teamID uuid.UUID) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil
	}
	if plan.MaxTeamMembers == nil {
		return nil
	}

	count, err := s.billing.CountTeamMembers(ctx, teamID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to count team members for quota check")
		return nil
	}

	if count >= *plan.MaxTeamMembers {
		return &QuotaExceededError{
			Resource:   "team_members",
			Current:    int64(count),
			Limit:      int64(*plan.MaxTeamMembers),
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// CheckConcurrentRuns returns nil if the account can start another run.
func (s *QuotaService) CheckConcurrentRuns(ctx context.Context, accountID uuid.UUID) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil
	}
	if plan.MaxConcurrentRuns == nil {
		return nil
	}

	count, err := s.billing.CountActiveRunsByAccount(ctx, accountID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to count active runs for quota check")
		return nil
	}

	if count >= *plan.MaxConcurrentRuns {
		return &QuotaExceededError{
			Resource:   "concurrent_runs",
			Current:    int64(count),
			Limit:      int64(*plan.MaxConcurrentRuns),
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// GetAccountForProject resolves the billing account ID for a project.
// Returns uuid.Nil if the project has no billing account linked.
func (s *QuotaService) GetAccountForProject(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("quota: get project: %w", err)
	}
	if project.BillingAccountID == nil {
		return uuid.Nil, nil
	}
	return *project.BillingAccountID, nil
}

// ResolveAccountByTeamID resolves the billing account ID for a team.
// Returns uuid.Nil if no billing account is linked to the team.
func (s *QuotaService) ResolveAccountByTeamID(ctx context.Context, teamID uuid.UUID) (uuid.UUID, error) {
	account, err := s.billing.GetAccountByTeamID(ctx, teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	return account.ID, nil
}

// CheckTeamMemberByTeamID resolves the billing account for a team and checks the team member quota.
// Returns nil if no billing account is linked to the team (fail open).
func (s *QuotaService) CheckTeamMemberByTeamID(ctx context.Context, teamID uuid.UUID) error {
	account, err := s.billing.GetAccountByTeamID(ctx, teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil // no billing account = no limits
		}
		s.logger.Warn().Err(err).Msg("failed to resolve billing account for team")
		return nil // fail open
	}
	return s.CheckTeamMember(ctx, account.ID, teamID)
}

// CheckContainerExecution returns nil if the account's plan allows container execution.
func (s *QuotaService) CheckContainerExecution(ctx context.Context, accountID uuid.UUID) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil // no plan = no limits
	}
	if !plan.ContainerExecution {
		return &QuotaExceededError{
			Resource:   "container_execution",
			Current:    0,
			Limit:      0,
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// CheckSSOAllowed returns nil if the account's plan allows SSO.
func (s *QuotaService) CheckSSOAllowed(ctx context.Context, accountID uuid.UUID) error {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil
	}
	if !plan.SSOEnabled {
		return &QuotaExceededError{
			Resource:   "sso",
			Current:    0,
			Limit:      0,
			PlanSlug:   plan.Slug,
			UpgradeURL: "/billing/plans",
		}
	}
	return nil
}

// CheckAuditLogsAllowed returns true if the account's plan allows audit logs.
func (s *QuotaService) CheckAuditLogsAllowed(ctx context.Context, accountID uuid.UUID) bool {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return false
	}
	return plan.AuditLogs
}

// GetCacheTTLForProject resolves the plan's cache_ttl_days for a project.
// Returns nil if no TTL is configured or the project has no billing account.
func (s *QuotaService) GetCacheTTLForProject(ctx context.Context, projectID uuid.UUID) *int {
	accountID, err := s.GetAccountForProject(ctx, projectID)
	if err != nil || accountID == uuid.Nil {
		return nil
	}
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil
	}
	return plan.CacheTTLDays
}

// GetRetentionLimits returns the retention limits for a billing account's plan.
// Returns nil values for limits that are not set (unlimited).
func (s *QuotaService) GetRetentionLimits(ctx context.Context, accountID uuid.UUID) (cacheTTLDays, artifactRetentionDays, logRetentionDays *int, err error) {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return nil, nil, nil, err
	}
	return plan.CacheTTLDays, plan.ArtifactRetentionDays, plan.LogRetentionDays, nil
}

// GetPriorityQueue returns true if the account's plan includes priority queue access.
func (s *QuotaService) GetPriorityQueue(ctx context.Context, accountID uuid.UUID) bool {
	plan, err := s.getPlanForAccount(ctx, accountID)
	if err != nil || plan == nil {
		return false
	}
	return plan.PriorityQueue
}

// getPlanForAccount resolves the effective plan for a billing account.
// It checks active/trialing subs first, then falls back to past_due/unpaid subs
// so that quota limits are still enforced (not unlimited) when payment is overdue.
func (s *QuotaService) getPlanForAccount(ctx context.Context, accountID uuid.UUID) (*models.BillingPlan, error) {
	sub, err := s.billing.GetActiveSubscription(ctx, accountID)
	if err != nil {
		if err == repo.ErrNotFound {
			// No active/trialing sub — check for past_due/unpaid so limits still apply
			sub, err = s.billing.GetSubscriptionByStatus(ctx, accountID, models.SubscriptionPastDue, models.SubscriptionUnpaid)
			if err != nil {
				if err == repo.ErrNotFound {
					return nil, nil // truly no subscription
				}
				return nil, fmt.Errorf("quota: get overdue subscription: %w", err)
			}
		} else {
			return nil, fmt.Errorf("quota: get subscription: %w", err)
		}
	}

	plan, err := s.billing.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, fmt.Errorf("quota: get plan: %w", err)
	}
	return plan, nil
}
