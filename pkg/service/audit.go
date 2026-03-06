package service

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/metric"
)

// AuditEntry is the handler-facing input for creating an audit log entry.
type AuditEntry struct {
	TeamID       uuid.UUID
	ProjectID    *uuid.UUID
	Action       string
	Category     string
	ResourceType string
	ResourceID   string
	Description  string
	Metadata     map[string]interface{}
}

// ChainVerifyResult holds the result of a hash chain verification.
type ChainVerifyResult struct {
	Valid        bool   `json:"valid"`
	TotalChecked int    `json:"total_checked"`
	FirstBreakAt *int64 `json:"first_break_at,omitempty"`
	Message      string `json:"message"`
}

// RetentionValidator is a callback that validates retention days against billing limits.
// Used by the cloud binary to enforce plan-level retention caps.
type RetentionValidator func(ctx context.Context, teamID uuid.UUID, days int) error

// ExportTracker is a callback invoked after a successful export, allowing the cloud
// binary to record export history for compliance without modifying OSS code.
type ExportTracker func(ctx context.Context, teamID uuid.UUID, filterJSON []byte, format string, rowCount int)

// AuditJobEnqueuer is the minimal interface needed to enqueue webhook forward jobs.
// Satisfied by *worker.Client via EnqueueRaw.
type AuditJobEnqueuer interface {
	EnqueueRaw(queue, taskName string, data []byte) error
}

// AuditService coordinates audit log operations.
type AuditService struct {
	repo               repo.AuditLogStore
	entitlements       entitlement.Checker
	logger             zerolog.Logger
	retentionValidator RetentionValidator
	exportTracker      ExportTracker
	jobEnqueuer        AuditJobEnqueuer
	metrics            *AuditMetrics
}

// AuditMetrics holds optional metric instruments for the audit service.
type AuditMetrics struct {
	LogsTotal        metric.Int64Counter
	WriteDuration    metric.Float64Histogram
	RetentionDeleted metric.Int64Counter
}

// NewAuditService creates a new audit service.
func NewAuditService(auditRepo repo.AuditLogStore, logger zerolog.Logger) *AuditService {
	return &AuditService{
		repo:   auditRepo,
		logger: logger.With().Str("service", "audit").Logger(),
	}
}

// SetEntitlements sets the entitlement checker for feature gating.
func (s *AuditService) SetEntitlements(c entitlement.Checker) {
	s.entitlements = c
}

// SetRetentionValidator sets a callback to validate retention days against billing limits.
func (s *AuditService) SetRetentionValidator(v RetentionValidator) {
	s.retentionValidator = v
}

// SetExportTracker sets a callback to record export events for compliance tracking.
func (s *AuditService) SetExportTracker(t ExportTracker) {
	s.exportTracker = t
}

// SetJobEnqueuer sets the job enqueuer for webhook forwarding.
func (s *AuditService) SetJobEnqueuer(e AuditJobEnqueuer) {
	s.jobEnqueuer = e
}

// SetMetrics sets the metric instruments for the audit service.
func (s *AuditService) SetMetrics(m *AuditMetrics) {
	s.metrics = m
}

// Repo returns the underlying audit log store (used by worker handlers).
func (s *AuditService) Repo() repo.AuditLogStore {
	return s.repo
}

// Log creates an audit log entry. It extracts actor info from context,
// computes the hash chain entry, and persists the record.
// This is a no-op if the audit_logs feature is not enabled.
func (s *AuditService) Log(ctx context.Context, entry AuditEntry) {
	// Feature gate: skip if audit logs are not enabled.
	if s.entitlements != nil && !s.entitlements.HasFeature(ctx, string(licensing.FeatureAuditLogs)) {
		return
	}

	s.logEntry(ctx, entry, nil)
}

// LogWithActor creates an audit log entry using the provided user as the actor
// instead of extracting from context. This is useful for login events where
// the user isn't yet in the request context.
func (s *AuditService) LogWithActor(ctx context.Context, entry AuditEntry, user *models.User) {
	// Feature gate: skip if audit logs are not enabled.
	if s.entitlements != nil && !s.entitlements.HasFeature(ctx, string(licensing.FeatureAuditLogs)) {
		return
	}

	s.logEntry(ctx, entry, user)
}

func (s *AuditService) logEntry(ctx context.Context, entry AuditEntry, actorOverride *models.User) {
	// Extract actor info from context or use the override.
	actorType := models.AuditActorSystem
	var actorID *uuid.UUID
	var actorEmail string

	user := actorOverride
	if user == nil {
		user = apiCtx.GetUser(ctx)
	}
	if user != nil {
		actorType = models.AuditActorUser
		actorID = &user.ID
		actorEmail = user.Email
	}
	if apiCtx.GetAuthMethod(ctx) == apiCtx.AuthMethodAPIKey {
		apiKey := apiCtx.GetAPIKey(ctx)
		if apiKey != nil {
			actorType = models.AuditActorAPIKey
			actorID = &apiKey.ID
			if user != nil {
				actorEmail = user.Email
			}
		}
	}

	// Extract request context (IP, user-agent, request ID).
	ipAddress := apiCtx.GetIPAddress(ctx)
	userAgent := apiCtx.GetUserAgent(ctx)
	requestID := apiCtx.GetRequestID(ctx)

	// Serialize metadata.
	var metadataBytes []byte
	if entry.Metadata != nil {
		var err error
		metadataBytes, err = json.Marshal(entry.Metadata)
		if err != nil {
			s.logger.Warn().Err(err).Msg("failed to marshal audit metadata")
			metadataBytes = []byte("{}")
		}
	} else {
		metadataBytes = []byte("{}")
	}

	// Get previous hash for chain.
	prevHash, _, err := s.repo.GetLastHash(ctx, entry.TeamID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to get last hash for chain")
		prevHash = ""
	}

	now := time.Now().UTC()

	// Compute entry hash: SHA256(prev_hash || action || actor_email || resource_id || created_at)
	entryHash := computeEntryHash(prevHash, entry.Action, actorEmail, entry.ResourceID, now)

	auditLog := &models.AuditLog{
		TeamID:       entry.TeamID,
		ProjectID:    entry.ProjectID,
		ActorType:    actorType,
		ActorID:      actorID,
		ActorEmail:   actorEmail,
		Action:       entry.Action,
		Category:     entry.Category,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Description:  entry.Description,
		Metadata:     metadataBytes,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		RequestID:    requestID,
		PrevHash:     prevHash,
		EntryHash:    entryHash,
	}

	start := time.Now()
	if err := s.repo.Create(ctx, auditLog); err != nil {
		s.logger.Error().Err(err).
			Str("action", entry.Action).
			Str("team_id", entry.TeamID.String()).
			Msg("failed to create audit log entry")
		return
	}
	duration := time.Since(start).Seconds()

	// Record metrics.
	if s.metrics != nil {
		if s.metrics.LogsTotal != nil {
			s.metrics.LogsTotal.Add(ctx, 1)
		}
		if s.metrics.WriteDuration != nil {
			s.metrics.WriteDuration.Record(ctx, duration)
		}
	}

	// Enqueue webhook forward job.
	if s.jobEnqueuer != nil {
		entryJSON, err := json.Marshal(auditLog)
		if err == nil {
			payload, _ := json.Marshal(map[string]interface{}{
				"team_id":    entry.TeamID.String(),
				"entry_json": entryJSON,
			})
			if err := s.jobEnqueuer.EnqueueRaw("DefaultQueue", "audit_webhook:forward", payload); err != nil {
				s.logger.Warn().Err(err).Msg("failed to enqueue webhook forward job")
			}
		}
	}
}

// List retrieves audit logs matching the filter with cursor-based pagination.
func (s *AuditService) List(ctx context.Context, filter repo.AuditLogFilter) (*repo.AuditLogListResult, error) {
	return s.repo.List(ctx, filter)
}

// GetByID retrieves a single audit log entry by ID.
func (s *AuditService) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	return s.repo.GetByID(ctx, id)
}

// Export writes audit logs in CSV or JSON format to the given writer.
// Capped at 100,000 rows. Paginates through the repo to collect all results.
func (s *AuditService) Export(ctx context.Context, filter repo.AuditLogFilter, format string, w io.Writer) error {
	const maxExportRows = 100000
	const pageSize = 100 // repo.List clamps to 100 max per page

	filter.Limit = pageSize
	var allLogs []models.AuditLog

	for len(allLogs) < maxExportRows {
		result, err := s.repo.List(ctx, filter)
		if err != nil {
			return fmt.Errorf("audit export: %w", err)
		}
		allLogs = append(allLogs, result.Data...)
		if !result.HasNext || result.NextCursor == "" {
			break
		}
		filter.Cursor = result.NextCursor
	}

	if len(allLogs) > maxExportRows {
		allLogs = allLogs[:maxExportRows]
	}

	var exportErr error
	switch format {
	case "csv":
		exportErr = s.exportCSV(w, allLogs)
	default:
		exportErr = s.exportJSON(w, allLogs)
	}

	// Track the export for compliance if a tracker is configured.
	if exportErr == nil && s.exportTracker != nil {
		filterJSON, _ := json.Marshal(map[string]interface{}{
			"team_id":     filter.TeamID.String(),
			"project_id":  filter.ProjectID,
			"category":    filter.Category,
			"action":      filter.Action,
			"actor_email": filter.ActorEmail,
			"since":       filter.Since,
			"until":       filter.Until,
		})
		s.exportTracker(ctx, filter.TeamID, filterJSON, format, len(allLogs))
	}

	return exportErr
}

func (s *AuditService) exportCSV(w io.Writer, logs []models.AuditLog) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header.
	if err := writer.Write([]string{
		"id", "timestamp", "actor_type", "actor_email", "action", "category",
		"resource_type", "resource_id", "description", "ip_address", "project_id", "team_id",
	}); err != nil {
		return err
	}

	for _, log := range logs {
		projectID := ""
		if log.ProjectID != nil {
			projectID = log.ProjectID.String()
		}
		if err := writer.Write([]string{
			log.ID.String(),
			log.CreatedAt.Format(time.RFC3339),
			log.ActorType,
			log.ActorEmail,
			log.Action,
			log.Category,
			log.ResourceType,
			log.ResourceID,
			log.Description,
			log.IPAddress,
			projectID,
			log.TeamID.String(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *AuditService) exportJSON(w io.Writer, logs []models.AuditLog) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(logs)
}

// RunRetentionGC deletes old audit logs based on each team's retention policy.
// Inserts an epoch marker when entries are deleted so chain verification
// starts from the post-GC boundary.
func (s *AuditService) RunRetentionGC(ctx context.Context) error {
	policies, err := s.repo.ListAllRetentionPolicies(ctx)
	if err != nil {
		return fmt.Errorf("audit gc: list policies: %w", err)
	}

	for _, policy := range policies {
		cutoff := time.Now().Add(-time.Duration(policy.RetentionDays) * 24 * time.Hour)
		deleted, err := s.repo.DeleteBefore(ctx, policy.TeamID, cutoff)
		if err != nil {
			s.logger.Error().Err(err).
				Str("team_id", policy.TeamID.String()).
				Msg("audit gc: failed to delete old entries")
			continue
		}

		if deleted > 0 {
			s.logger.Info().
				Str("team_id", policy.TeamID.String()).
				Int64("deleted", deleted).
				Int("retention_days", policy.RetentionDays).
				Msg("audit gc: deleted old entries")

			if s.metrics != nil && s.metrics.RetentionDeleted != nil {
				s.metrics.RetentionDeleted.Add(ctx, deleted)
			}

			// Insert epoch marker so chain verification knows there was a GC boundary.
			s.logEpochMarker(ctx, policy.TeamID, deleted)
		}
	}

	return nil
}

func (s *AuditService) logEpochMarker(ctx context.Context, teamID uuid.UUID, deletedCount int64) {
	prevHash, _, err := s.repo.GetLastHash(ctx, teamID)
	if err != nil {
		prevHash = ""
	}

	now := time.Now().UTC()
	entryHash := computeEntryHash(prevHash, models.AuditActionRetentionEpoch, "", "", now)

	metadata, _ := json.Marshal(map[string]interface{}{
		"deleted_count": deletedCount,
	})

	epoch := &models.AuditLog{
		TeamID:       teamID,
		ActorType:    models.AuditActorSystem,
		Action:       models.AuditActionRetentionEpoch,
		Category:     models.AuditCategorySystem,
		ResourceType: "audit_log",
		Description:  fmt.Sprintf("Retention GC deleted %d entries", deletedCount),
		Metadata:     metadata,
		PrevHash:     prevHash,
		EntryHash:    entryHash,
	}

	if err := s.repo.Create(ctx, epoch); err != nil {
		s.logger.Error().Err(err).Str("team_id", teamID.String()).Msg("audit gc: failed to insert epoch marker")
	}
}

// VerifyChain walks the hash chain for a team and checks for breaks.
// It verifies both hash recomputation and prev_hash linkage between entries.
func (s *AuditService) VerifyChain(ctx context.Context, teamID uuid.UUID) (*ChainVerifyResult, error) {
	result := &ChainVerifyResult{Valid: true}
	var afterSeq int64
	var prevEntryHash string // tracks the previous entry's hash for linkage verification

	for {
		batch, err := s.repo.ListChain(ctx, teamID, afterSeq, 1000)
		if err != nil {
			return nil, fmt.Errorf("audit verify: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		for _, entry := range batch {
			result.TotalChecked++

			// Epoch markers reset the chain after GC — update tracking and skip verification.
			if entry.Action == models.AuditActionRetentionEpoch {
				prevEntryHash = entry.EntryHash
				afterSeq = entry.SequenceNum
				continue
			}

			// Verify prev_hash linkage: this entry's PrevHash must match
			// the previous entry's EntryHash.
			if prevEntryHash != "" && entry.PrevHash != prevEntryHash {
				result.Valid = false
				seq := entry.SequenceNum
				result.FirstBreakAt = &seq
				result.Message = fmt.Sprintf("chain linkage broken at sequence %d: prev_hash does not match previous entry", entry.SequenceNum)
				return result, nil
			}

			// Recompute hash and compare.
			expected := computeEntryHash(
				entry.PrevHash, entry.Action, entry.ActorEmail,
				entry.ResourceID, entry.CreatedAt,
			)
			if expected != entry.EntryHash {
				result.Valid = false
				seq := entry.SequenceNum
				result.FirstBreakAt = &seq
				result.Message = fmt.Sprintf("hash mismatch at sequence %d", entry.SequenceNum)
				return result, nil
			}

			prevEntryHash = entry.EntryHash
			afterSeq = entry.SequenceNum
		}
	}

	if result.TotalChecked == 0 {
		result.Message = "no entries to verify"
	} else {
		result.Message = fmt.Sprintf("all %d entries verified successfully", result.TotalChecked)
	}

	return result, nil
}

// UpdateRetention updates the retention policy for a team.
// If a RetentionValidator is set, it is called first to enforce plan limits.
func (s *AuditService) UpdateRetention(ctx context.Context, teamID uuid.UUID, days int, updatedBy *uuid.UUID) error {
	if s.retentionValidator != nil {
		if err := s.retentionValidator(ctx, teamID, days); err != nil {
			return err
		}
	}
	return s.repo.UpsertRetentionPolicy(ctx, &models.AuditLogRetentionPolicy{
		TeamID:        teamID,
		RetentionDays: days,
		UpdatedBy:     updatedBy,
	})
}

// LogSystem creates an audit log entry for system-originated events (no HTTP context).
func (s *AuditService) LogSystem(ctx context.Context, teamID uuid.UUID, action, category, resourceType, resourceID, description string, metadata map[string]interface{}) {
	s.logEntry(ctx, AuditEntry{
		TeamID:       teamID,
		Action:       action,
		Category:     category,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Description:  description,
		Metadata:     metadata,
	}, nil)
}

// GetRetention returns the retention policy for a team.
// Returns a default 90-day policy if none is set.
func (s *AuditService) GetRetention(ctx context.Context, teamID uuid.UUID) (*models.AuditLogRetentionPolicy, error) {
	policy, err := s.repo.GetRetentionPolicy(ctx, teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			return &models.AuditLogRetentionPolicy{
				TeamID:        teamID,
				RetentionDays: 90,
			}, nil
		}
		return nil, err
	}
	return policy, nil
}

func computeEntryHash(prevHash, action, actorEmail, resourceID string, createdAt time.Time) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s",
		prevHash, action, actorEmail, resourceID, createdAt.Format(time.RFC3339Nano))
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}
