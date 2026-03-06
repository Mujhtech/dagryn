package repo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// AuditLogFilter configures audit log listing and filtering.
type AuditLogFilter struct {
	TeamID     uuid.UUID
	ProjectID  *uuid.UUID
	ActorID    *uuid.UUID
	ActorEmail string
	Action     string
	Category   string
	Since      *time.Time
	Until      *time.Time
	Cursor     string // sequence_num cursor for pagination
	Limit      int
}

// AuditLogListResult holds a page of audit logs with cursor info.
type AuditLogListResult struct {
	Data       []models.AuditLog `json:"data"`
	NextCursor string            `json:"next_cursor,omitempty"`
	HasNext    bool              `json:"has_next"`
}

// AuditLogRepo handles audit log database operations.
type AuditLogRepo struct {
	pool *pgxpool.Pool
}

// NewAuditLogRepo creates a new audit log repository.
func NewAuditLogRepo(pool *pgxpool.Pool) AuditLogStore {
	return &AuditLogRepo{pool: pool}
}

// Create inserts a new audit log entry.
func (r *AuditLogRepo) Create(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			team_id, project_id, actor_type, actor_id, actor_email,
			action, category, resource_type, resource_id, description,
			metadata, ip_address, user_agent, request_id,
			prev_hash, entry_hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, sequence_num, created_at`

	return r.pool.QueryRow(ctx, query,
		log.TeamID, log.ProjectID, log.ActorType, log.ActorID, log.ActorEmail,
		log.Action, log.Category, log.ResourceType, log.ResourceID, log.Description,
		log.Metadata, log.IPAddress, log.UserAgent, log.RequestID,
		log.PrevHash, log.EntryHash,
	).Scan(&log.ID, &log.SequenceNum, &log.CreatedAt)
}

// GetByID retrieves a single audit log entry by ID.
func (r *AuditLogRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	var log models.AuditLog
	err := r.pool.QueryRow(ctx, `
		SELECT id, sequence_num, team_id, project_id, actor_type, actor_id, actor_email,
			action, category, resource_type, resource_id, description,
			metadata, ip_address, user_agent, request_id,
			prev_hash, entry_hash, created_at
		FROM audit_logs WHERE id = $1`, id,
	).Scan(
		&log.ID, &log.SequenceNum, &log.TeamID, &log.ProjectID,
		&log.ActorType, &log.ActorID, &log.ActorEmail,
		&log.Action, &log.Category, &log.ResourceType, &log.ResourceID, &log.Description,
		&log.Metadata, &log.IPAddress, &log.UserAgent, &log.RequestID,
		&log.PrevHash, &log.EntryHash, &log.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &log, nil
}

// List retrieves audit logs matching the filter with cursor-based pagination.
func (r *AuditLogRepo) List(ctx context.Context, filter AuditLogFilter) (*AuditLogListResult, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("team_id = $%d", argIdx))
	args = append(args, filter.TeamID)
	argIdx++

	if filter.ProjectID != nil {
		conditions = append(conditions, fmt.Sprintf("project_id = $%d", argIdx))
		args = append(args, *filter.ProjectID)
		argIdx++
	}
	if filter.ActorID != nil {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argIdx))
		args = append(args, *filter.ActorID)
		argIdx++
	}
	if filter.ActorEmail != "" {
		conditions = append(conditions, fmt.Sprintf("actor_email = $%d", argIdx))
		args = append(args, filter.ActorEmail)
		argIdx++
	}
	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, filter.Category)
		argIdx++
	}
	if filter.Since != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.Since)
		argIdx++
	}
	if filter.Until != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filter.Until)
		argIdx++
	}
	if filter.Cursor != "" {
		cursorSeq, err := strconv.ParseInt(filter.Cursor, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("audit_log: invalid cursor: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("sequence_num < $%d", argIdx))
		args = append(args, cursorSeq)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Fetch limit+1 to determine if there are more results.
	query := fmt.Sprintf(`
		SELECT id, sequence_num, team_id, project_id, actor_type, actor_id, actor_email,
			action, category, resource_type, resource_id, description,
			metadata, ip_address, user_agent, request_id,
			prev_hash, entry_hash, created_at
		FROM audit_logs
		WHERE %s
		ORDER BY sequence_num DESC
		LIMIT $%d`, where, argIdx)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("audit_log: list: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(
			&log.ID, &log.SequenceNum, &log.TeamID, &log.ProjectID,
			&log.ActorType, &log.ActorID, &log.ActorEmail,
			&log.Action, &log.Category, &log.ResourceType, &log.ResourceID, &log.Description,
			&log.Metadata, &log.IPAddress, &log.UserAgent, &log.RequestID,
			&log.PrevHash, &log.EntryHash, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("audit_log: scan: %w", err)
		}
		logs = append(logs, log)
	}

	result := &AuditLogListResult{}
	if len(logs) > limit {
		result.HasNext = true
		result.NextCursor = fmt.Sprintf("%d", logs[limit-1].SequenceNum)
		result.Data = logs[:limit]
	} else {
		result.Data = logs
	}
	if result.Data == nil {
		result.Data = []models.AuditLog{}
	}

	return result, nil
}

// DeleteBefore deletes audit log entries older than the given time for a team.
// Returns the number of deleted rows.
func (r *AuditLogRepo) DeleteBefore(ctx context.Context, teamID uuid.UUID, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM audit_logs WHERE team_id = $1 AND created_at < $2`,
		teamID, before,
	)
	if err != nil {
		return 0, fmt.Errorf("audit_log: delete before: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetLastHash returns the entry_hash and sequence_num of the most recent entry for a team.
// Returns empty strings if no entries exist.
func (r *AuditLogRepo) GetLastHash(ctx context.Context, teamID uuid.UUID) (string, int64, error) {
	var hash string
	var seq int64
	err := r.pool.QueryRow(ctx,
		`SELECT entry_hash, sequence_num FROM audit_logs
		 WHERE team_id = $1 ORDER BY sequence_num DESC LIMIT 1`,
		teamID,
	).Scan(&hash, &seq)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", 0, nil
		}
		return "", 0, err
	}
	return hash, seq, nil
}

// GetRetentionPolicy retrieves the retention policy for a team.
func (r *AuditLogRepo) GetRetentionPolicy(ctx context.Context, teamID uuid.UUID) (*models.AuditLogRetentionPolicy, error) {
	var p models.AuditLogRetentionPolicy
	err := r.pool.QueryRow(ctx,
		`SELECT team_id, retention_days, updated_at, updated_by
		 FROM audit_log_retention_policies WHERE team_id = $1`,
		teamID,
	).Scan(&p.TeamID, &p.RetentionDays, &p.UpdatedAt, &p.UpdatedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// UpsertRetentionPolicy creates or updates the retention policy for a team.
func (r *AuditLogRepo) UpsertRetentionPolicy(ctx context.Context, policy *models.AuditLogRetentionPolicy) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log_retention_policies (team_id, retention_days, updated_at, updated_by)
		VALUES ($1, $2, now(), $3)
		ON CONFLICT (team_id) DO UPDATE SET
			retention_days = EXCLUDED.retention_days,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by`,
		policy.TeamID, policy.RetentionDays, policy.UpdatedBy,
	)
	return err
}

// ListAllRetentionPolicies returns all retention policies (used by GC job).
func (r *AuditLogRepo) ListAllRetentionPolicies(ctx context.Context) ([]models.AuditLogRetentionPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT team_id, retention_days, updated_at, updated_by
		 FROM audit_log_retention_policies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []models.AuditLogRetentionPolicy
	for rows.Next() {
		var p models.AuditLogRetentionPolicy
		if err := rows.Scan(&p.TeamID, &p.RetentionDays, &p.UpdatedAt, &p.UpdatedBy); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

// CountByTeam returns the total number of audit log entries for a team.
func (r *AuditLogRepo) CountByTeam(ctx context.Context, teamID uuid.UUID) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE team_id = $1`, teamID,
	).Scan(&count)
	return count, err
}

// ListChain returns audit logs ordered by sequence_num ascending for chain verification.
func (r *AuditLogRepo) ListChain(ctx context.Context, teamID uuid.UUID, afterSeq int64, limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, sequence_num, team_id, project_id, actor_type, actor_id, actor_email,
			action, category, resource_type, resource_id, description,
			metadata, ip_address, user_agent, request_id,
			prev_hash, entry_hash, created_at
		FROM audit_logs
		WHERE team_id = $1 AND sequence_num > $2
		ORDER BY sequence_num ASC
		LIMIT $3`, teamID, afterSeq, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(
			&log.ID, &log.SequenceNum, &log.TeamID, &log.ProjectID,
			&log.ActorType, &log.ActorID, &log.ActorEmail,
			&log.Action, &log.Category, &log.ResourceType, &log.ResourceID, &log.Description,
			&log.Metadata, &log.IPAddress, &log.UserAgent, &log.RequestID,
			&log.PrevHash, &log.EntryHash, &log.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// --- Webhook CRUD ---

// CreateWebhook inserts a new audit webhook.
func (r *AuditLogRepo) CreateWebhook(ctx context.Context, w *models.AuditWebhook) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO audit_webhooks (team_id, url, secret_encrypted, description, event_filter, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		w.TeamID, w.URL, w.SecretEncrypted, w.Description,
		pgtype.FlatArray[string](w.EventFilter), w.IsActive, w.CreatedBy,
	).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
}

// GetWebhookByID retrieves a webhook by ID.
func (r *AuditLogRepo) GetWebhookByID(ctx context.Context, id uuid.UUID) (*models.AuditWebhook, error) {
	var w models.AuditWebhook
	var eventFilter pgtype.FlatArray[string]
	err := r.pool.QueryRow(ctx, `
		SELECT id, team_id, url, secret_encrypted, description, event_filter,
			is_active, created_at, updated_at, created_by
		FROM audit_webhooks WHERE id = $1`, id,
	).Scan(
		&w.ID, &w.TeamID, &w.URL, &w.SecretEncrypted, &w.Description,
		&eventFilter, &w.IsActive, &w.CreatedAt, &w.UpdatedAt, &w.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	w.EventFilter = eventFilter
	return &w, nil
}

// ListWebhooksByTeam lists all webhooks for a team.
func (r *AuditLogRepo) ListWebhooksByTeam(ctx context.Context, teamID uuid.UUID) ([]models.AuditWebhook, error) {
	return r.listWebhooks(ctx, teamID, false)
}

// ListActiveWebhooksByTeam lists only active webhooks for a team.
func (r *AuditLogRepo) ListActiveWebhooksByTeam(ctx context.Context, teamID uuid.UUID) ([]models.AuditWebhook, error) {
	return r.listWebhooks(ctx, teamID, true)
}

func (r *AuditLogRepo) listWebhooks(ctx context.Context, teamID uuid.UUID, activeOnly bool) ([]models.AuditWebhook, error) {
	query := `
		SELECT id, team_id, url, secret_encrypted, description, event_filter,
			is_active, created_at, updated_at, created_by
		FROM audit_webhooks WHERE team_id = $1`
	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Query(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []models.AuditWebhook
	for rows.Next() {
		var w models.AuditWebhook
		var eventFilter pgtype.FlatArray[string]
		if err := rows.Scan(
			&w.ID, &w.TeamID, &w.URL, &w.SecretEncrypted, &w.Description,
			&eventFilter, &w.IsActive, &w.CreatedAt, &w.UpdatedAt, &w.CreatedBy,
		); err != nil {
			return nil, err
		}
		w.EventFilter = eventFilter
		webhooks = append(webhooks, w)
	}
	if webhooks == nil {
		webhooks = []models.AuditWebhook{}
	}
	return webhooks, nil
}

// UpdateWebhook updates an existing webhook.
func (r *AuditLogRepo) UpdateWebhook(ctx context.Context, w *models.AuditWebhook) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE audit_webhooks SET
			url = $1, description = $2, event_filter = $3, is_active = $4, updated_at = now()
		WHERE id = $5`,
		w.URL, w.Description, pgtype.FlatArray[string](w.EventFilter), w.IsActive, w.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteWebhook deletes a webhook by ID.
func (r *AuditLogRepo) DeleteWebhook(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM audit_webhooks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
