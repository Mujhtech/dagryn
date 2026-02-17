package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// AIRepo handles AI analysis and publication database operations.
type AIRepo struct {
	pool *pgxpool.Pool
}

// NewAIRepo creates a new AI repository.
func NewAIRepo(pool *pgxpool.Pool) *AIRepo {
	return &AIRepo{pool: pool}
}

// --- Analysis CRUD ---

// CreateAnalysis inserts a new AI analysis record.
func (r *AIRepo) CreateAnalysis(ctx context.Context, a *models.AIAnalysis) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now

	// Ensure evidence_json is never NULL (column is NOT NULL DEFAULT '{}').
	if a.EvidenceJSON == nil {
		a.EvidenceJSON = []byte(`{}`)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_analyses (
			id, run_id, project_id, status, provider, provider_mode, model,
			prompt_version, prompt_hash, response_hash, summary, root_cause,
			confidence, evidence_json, raw_response_blob_key, error_message,
			dedup_key, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`, a.ID, a.RunID, a.ProjectID, a.Status, a.Provider, a.ProviderMode, a.Model,
		a.PromptVersion, a.PromptHash, a.ResponseHash, a.Summary, a.RootCause,
		a.Confidence, a.EvidenceJSON, a.RawResponseBlobKey, a.ErrorMessage,
		a.DedupKey, a.CreatedAt, a.UpdatedAt)
	return err
}

// GetAnalysisByID returns an analysis by its ID.
func (r *AIRepo) GetAnalysisByID(ctx context.Context, id uuid.UUID) (*models.AIAnalysis, error) {
	return r.scanAnalysis(ctx, `
		SELECT id, run_id, project_id, status, provider, provider_mode, model,
		       prompt_version, prompt_hash, response_hash, summary, root_cause,
		       confidence, evidence_json, raw_response_blob_key, error_message,
		       dedup_key, created_at, updated_at
		FROM ai_analyses WHERE id = $1
	`, id)
}

// GetAnalysisByRunID returns the latest analysis for a run.
func (r *AIRepo) GetAnalysisByRunID(ctx context.Context, runID uuid.UUID) (*models.AIAnalysis, error) {
	return r.scanAnalysis(ctx, `
		SELECT id, run_id, project_id, status, provider, provider_mode, model,
		       prompt_version, prompt_hash, response_hash, summary, root_cause,
		       confidence, evidence_json, raw_response_blob_key, error_message,
		       dedup_key, created_at, updated_at
		FROM ai_analyses WHERE run_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, runID)
}

// ListAnalysesByProject returns paginated analyses for a project.
func (r *AIRepo) ListAnalysesByProject(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]models.AIAnalysis, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_analyses WHERE project_id = $1`, projectID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, run_id, project_id, status, provider, provider_mode, model,
		       prompt_version, prompt_hash, response_hash, summary, root_cause,
		       confidence, evidence_json, raw_response_blob_key, error_message,
		       dedup_key, created_at, updated_at
		FROM ai_analyses WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, projectID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var analyses []models.AIAnalysis
	for rows.Next() {
		a, err := scanAnalysisRow(rows)
		if err != nil {
			return nil, 0, err
		}
		analyses = append(analyses, *a)
	}
	return analyses, total, rows.Err()
}

// UpdateAnalysisStatus updates the status (and optional error message) of an analysis.
func (r *AIRepo) UpdateAnalysisStatus(ctx context.Context, id uuid.UUID, status models.AIAnalysisStatus, errorMessage *string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE ai_analyses SET status = $1, error_message = $2, updated_at = NOW()
		WHERE id = $3
	`, status, errorMessage, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateAnalysisResults persists the full analysis output (summary, root_cause, etc.) along with the status.
func (r *AIRepo) UpdateAnalysisResults(ctx context.Context, a *models.AIAnalysis) error {
	if a.EvidenceJSON == nil {
		a.EvidenceJSON = []byte(`{}`)
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE ai_analyses SET
			status = $1, summary = $2, root_cause = $3, confidence = $4,
			evidence_json = $5, prompt_hash = $6, response_hash = $7,
			error_message = $8, model = $9, updated_at = NOW()
		WHERE id = $10
	`, a.Status, a.Summary, a.RootCause, a.Confidence,
		a.EvidenceJSON, a.PromptHash, a.ResponseHash,
		a.ErrorMessage, a.Model, a.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FindPendingByDedupKey returns a pending analysis matching the dedup key.
func (r *AIRepo) FindPendingByDedupKey(ctx context.Context, dedupKey string) (*models.AIAnalysis, error) {
	return r.scanAnalysis(ctx, `
		SELECT id, run_id, project_id, status, provider, provider_mode, model,
		       prompt_version, prompt_hash, response_hash, summary, root_cause,
		       confidence, evidence_json, raw_response_blob_key, error_message,
		       dedup_key, created_at, updated_at
		FROM ai_analyses WHERE dedup_key = $1 AND status = 'pending'
		LIMIT 1
	`, dedupKey)
}

// SupersedeByBranch marks old pending analyses as superseded for a project/branch combo,
// excluding a specific commit (the current one).
func (r *AIRepo) SupersedeByBranch(ctx context.Context, projectID uuid.UUID, branch string, excludeCommit string) error {
	dedupPattern := projectID.String() + ":" + branch + ":%"
	_, err := r.pool.Exec(ctx, `
		UPDATE ai_analyses SET status = 'superseded', updated_at = NOW()
		WHERE project_id = $1
		  AND status = 'pending'
		  AND dedup_key LIKE $2
		  AND dedup_key != $3
	`, projectID, dedupPattern, projectID.String()+":"+branch+":"+excludeCommit)
	return err
}

// UpdateAnalysisDedupKey sets the dedup_key on an analysis record.
func (r *AIRepo) UpdateAnalysisDedupKey(ctx context.Context, id uuid.UUID, dedupKey string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE ai_analyses SET dedup_key = $1, updated_at = NOW() WHERE id = $2
	`, dedupKey, id)
	return err
}

// CountRecentAnalyses counts non-superseded analyses for a project since a given time (for rate limiting).
func (r *AIRepo) CountRecentAnalyses(ctx context.Context, projectID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_analyses
		WHERE project_id = $1 AND created_at >= $2 AND status NOT IN ('superseded')
	`, projectID, since).Scan(&count)
	return count, err
}

// --- Publication CRUD ---

// CreatePublication inserts a new publication record.
func (r *AIRepo) CreatePublication(ctx context.Context, p *models.AIPublication) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_publications (
			id, analysis_id, run_id, destination, external_id, status,
			error_message, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, p.ID, p.AnalysisID, p.RunID, p.Destination, p.ExternalID,
		p.Status, p.ErrorMessage, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetPublicationByRunAndDestination returns the publication for a run+destination combo (idempotency).
func (r *AIRepo) GetPublicationByRunAndDestination(ctx context.Context, runID uuid.UUID, destination models.AIPublicationDestination) (*models.AIPublication, error) {
	return r.scanPublication(ctx, `
		SELECT id, analysis_id, run_id, destination, external_id, status,
		       error_message, created_at, updated_at
		FROM ai_publications WHERE run_id = $1 AND destination = $2
	`, runID, destination)
}

// UpdatePublication updates a publication's status, external ID, and error message.
func (r *AIRepo) UpdatePublication(ctx context.Context, id uuid.UUID, status models.AIPublicationStatus, externalID *string, errorMessage *string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE ai_publications SET status = $1, external_id = $2, error_message = $3, updated_at = NOW()
		WHERE id = $4
	`, status, externalID, errorMessage, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPublicationsByAnalysis returns all publications for a given analysis.
func (r *AIRepo) ListPublicationsByAnalysis(ctx context.Context, analysisID uuid.UUID) ([]models.AIPublication, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, analysis_id, run_id, destination, external_id, status,
		       error_message, created_at, updated_at
		FROM ai_publications WHERE analysis_id = $1
		ORDER BY created_at ASC
	`, analysisID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pubs []models.AIPublication
	for rows.Next() {
		p, err := scanPublicationRow(rows)
		if err != nil {
			return nil, err
		}
		pubs = append(pubs, *p)
	}
	return pubs, rows.Err()
}

// --- Suggestion CRUD ---

// CreateSuggestion inserts a new AI suggestion record.
func (r *AIRepo) CreateSuggestion(ctx context.Context, s *models.AISuggestion) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_suggestions (
			id, analysis_id, run_id, file_path, start_line, end_line,
			original_code, suggested_code, explanation, confidence, status,
			github_comment_id, risk_score, failure_reason, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, s.ID, s.AnalysisID, s.RunID, s.FilePath, s.StartLine, s.EndLine,
		s.OriginalCode, s.SuggestedCode, s.Explanation, s.Confidence, s.Status,
		s.GitHubCommentID, s.RiskScore, s.FailureReason, s.CreatedAt, s.UpdatedAt)
	return err
}

// ListSuggestionsByAnalysis returns all suggestions for a given analysis.
func (r *AIRepo) ListSuggestionsByAnalysis(ctx context.Context, analysisID uuid.UUID) ([]models.AISuggestion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, analysis_id, run_id, file_path, start_line, end_line,
		       original_code, suggested_code, explanation, confidence, status,
		       github_comment_id, risk_score, failure_reason, created_at, updated_at
		FROM ai_suggestions WHERE analysis_id = $1
		ORDER BY file_path ASC, start_line ASC
	`, analysisID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suggestions []models.AISuggestion
	for rows.Next() {
		s, err := scanSuggestionRow(rows)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, *s)
	}
	return suggestions, rows.Err()
}

// ListPendingSuggestionsByAnalysis returns pending suggestions for publishing.
func (r *AIRepo) ListPendingSuggestionsByAnalysis(ctx context.Context, analysisID uuid.UUID) ([]models.AISuggestion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, analysis_id, run_id, file_path, start_line, end_line,
		       original_code, suggested_code, explanation, confidence, status,
		       github_comment_id, risk_score, failure_reason, created_at, updated_at
		FROM ai_suggestions WHERE analysis_id = $1 AND status = 'pending'
		ORDER BY file_path ASC, start_line ASC
	`, analysisID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suggestions []models.AISuggestion
	for rows.Next() {
		s, err := scanSuggestionRow(rows)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, *s)
	}
	return suggestions, rows.Err()
}

// UpdateSuggestionStatus updates a suggestion's status and optional fields.
func (r *AIRepo) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status models.AISuggestionStatus, githubCommentID *string, failureReason *string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE ai_suggestions SET status = $1, github_comment_id = $2, failure_reason = $3, updated_at = NOW()
		WHERE id = $4
	`, status, githubCommentID, failureReason, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountInProgressAnalyses counts analyses with status 'in_progress' for a project (for concurrency limiting).
func (r *AIRepo) CountInProgressAnalyses(ctx context.Context, projectID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_analyses
		WHERE project_id = $1 AND status = 'in_progress'
	`, projectID).Scan(&count)
	return count, err
}

// GetMostRecentAnalysisByKey returns the most recent analysis matching project+branch+commit (for cooldown).
func (r *AIRepo) GetMostRecentAnalysisByKey(ctx context.Context, projectID uuid.UUID, branch, commit string) (*models.AIAnalysis, error) {
	dedupPrefix := projectID.String() + ":" + branch + ":" + commit + ":"
	return r.scanAnalysis(ctx, `
		SELECT id, run_id, project_id, status, provider, provider_mode, model,
		       prompt_version, prompt_hash, response_hash, summary, root_cause,
		       confidence, evidence_json, raw_response_blob_key, error_message,
		       dedup_key, created_at, updated_at
		FROM ai_analyses
		WHERE project_id = $1 AND dedup_key LIKE $2
		ORDER BY created_at DESC LIMIT 1
	`, projectID, dedupPrefix+"%")
}

// ListPostedSuggestionsByProjectAndBranch returns posted suggestions for a project+branch.
func (r *AIRepo) ListPostedSuggestionsByProjectAndBranch(ctx context.Context, projectID uuid.UUID, branch string) ([]models.AISuggestion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.analysis_id, s.run_id, s.file_path, s.start_line, s.end_line,
		       s.original_code, s.suggested_code, s.explanation, s.confidence, s.status,
		       s.github_comment_id, s.risk_score, s.failure_reason, s.created_at, s.updated_at
		FROM ai_suggestions s
		JOIN ai_analyses a ON s.analysis_id = a.id
		JOIN runs r ON a.run_id = r.id
		WHERE a.project_id = $1
		  AND r.git_branch = $2
		  AND s.status = 'posted'
		ORDER BY s.created_at DESC
	`, projectID, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suggestions []models.AISuggestion
	for rows.Next() {
		s, err := scanSuggestionRow(rows)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, *s)
	}
	return suggestions, rows.Err()
}

// DeleteExpiredBlobKeys nulls raw_response_blob_key for analyses older than the given duration.
func (r *AIRepo) DeleteExpiredBlobKeys(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := r.pool.Exec(ctx, `
		UPDATE ai_analyses SET raw_response_blob_key = NULL, updated_at = NOW()
		WHERE raw_response_blob_key IS NOT NULL AND created_at < $1
	`, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// --- Scan helpers ---

func (r *AIRepo) scanAnalysis(ctx context.Context, query string, args ...any) (*models.AIAnalysis, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	var a models.AIAnalysis
	err := row.Scan(
		&a.ID, &a.RunID, &a.ProjectID, &a.Status, &a.Provider, &a.ProviderMode, &a.Model,
		&a.PromptVersion, &a.PromptHash, &a.ResponseHash, &a.Summary, &a.RootCause,
		&a.Confidence, &a.EvidenceJSON, &a.RawResponseBlobKey, &a.ErrorMessage,
		&a.DedupKey, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func scanAnalysisRow(rows pgx.Rows) (*models.AIAnalysis, error) {
	var a models.AIAnalysis
	err := rows.Scan(
		&a.ID, &a.RunID, &a.ProjectID, &a.Status, &a.Provider, &a.ProviderMode, &a.Model,
		&a.PromptVersion, &a.PromptHash, &a.ResponseHash, &a.Summary, &a.RootCause,
		&a.Confidence, &a.EvidenceJSON, &a.RawResponseBlobKey, &a.ErrorMessage,
		&a.DedupKey, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AIRepo) scanPublication(ctx context.Context, query string, args ...any) (*models.AIPublication, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	var p models.AIPublication
	err := row.Scan(
		&p.ID, &p.AnalysisID, &p.RunID, &p.Destination, &p.ExternalID,
		&p.Status, &p.ErrorMessage, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func scanPublicationRow(rows pgx.Rows) (*models.AIPublication, error) {
	var p models.AIPublication
	err := rows.Scan(
		&p.ID, &p.AnalysisID, &p.RunID, &p.Destination, &p.ExternalID,
		&p.Status, &p.ErrorMessage, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanSuggestionRow(rows pgx.Rows) (*models.AISuggestion, error) {
	var s models.AISuggestion
	err := rows.Scan(
		&s.ID, &s.AnalysisID, &s.RunID, &s.FilePath, &s.StartLine, &s.EndLine,
		&s.OriginalCode, &s.SuggestedCode, &s.Explanation, &s.Confidence, &s.Status,
		&s.GitHubCommentID, &s.RiskScore, &s.FailureReason, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
