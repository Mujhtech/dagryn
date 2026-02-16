package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/internal/ai/aitypes"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/encrypt"
	"github.com/mujhtech/dagryn/internal/notification"
	"github.com/mujhtech/dagryn/internal/telemetry"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// aiPublishRepo defines AI repo operations needed by the publish handler.
type aiPublishRepo interface {
	GetAnalysisByID(ctx context.Context, id uuid.UUID) (*models.AIAnalysis, error)
	CreatePublication(ctx context.Context, p *models.AIPublication) error
	GetPublicationByRunAndDestination(ctx context.Context, runID uuid.UUID, dest models.AIPublicationDestination) (*models.AIPublication, error)
	UpdatePublication(ctx context.Context, id uuid.UUID, status models.AIPublicationStatus, externalID *string, errorMessage *string) error
}

// runRepo defines run repo ops needed by the publish handler.
type runRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Run, error)
}

// projectRepo defines project repo ops needed by the publish handler.
type projectRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
}

// providerTokenStore defines provider token repo ops needed by the publish handler.
type providerTokenStore interface {
	GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*models.ProviderToken, error)
}

// githubInstallationStore defines GitHub installation repo ops needed by the publish handler.
type githubInstallationStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.GitHubInstallation, error)
}

// aiPublishPayload mirrors job.AIPublishPayload to avoid an import cycle.
type aiPublishPayload struct {
	AnalysisID string `json:"analysis_id"`
	RunID      string `json:"run_id"`
	ProjectID  string `json:"project_id"`
}

// AIPublishHandler processes ai_publish:github jobs.
type AIPublishHandler struct {
	aiRepo              aiPublishRepo
	runs                runRepo
	projects            projectRepo
	providerTokens      providerTokenStore
	providerEncrypt     encrypt.Encrypt
	githubApp           GitHubAppClient
	githubInstallations githubInstallationStore
	encrypter           encrypt.Encrypt
	baseURL             string
	logger              zerolog.Logger
	metrics             *telemetry.Metrics
}

// NewAIPublishHandler creates a new AI publish job handler.
func NewAIPublishHandler(
	aiRepo aiPublishRepo,
	runs runRepo,
	projects projectRepo,
	providerTokens providerTokenStore,
	providerEncrypt encrypt.Encrypt,
	githubApp GitHubAppClient,
	githubInstallations githubInstallationStore,
	encrypter encrypt.Encrypt,
	baseURL string,
	logger zerolog.Logger,
	metrics ...*telemetry.Metrics,
) *AIPublishHandler {
	var m *telemetry.Metrics
	if len(metrics) > 0 {
		m = metrics[0]
	}
	return &AIPublishHandler{
		aiRepo:              aiRepo,
		runs:                runs,
		projects:            projects,
		providerTokens:      providerTokens,
		providerEncrypt:     providerEncrypt,
		githubApp:           githubApp,
		githubInstallations: githubInstallations,
		encrypter:           encrypter,
		baseURL:             baseURL,
		logger:              logger.With().Str("handler", "ai_publish").Logger(),
		metrics:             m,
	}
}

// Handle processes the ai_publish:github task.
func (h *AIPublishHandler) Handle(ctx context.Context, t *asynq.Task) error {
	// Decrypt payload.
	rawPayload := string(t.Payload())
	var plaintext string
	if h.encrypter != nil {
		var err error
		plaintext, err = h.encrypter.Decrypt(rawPayload)
		if err != nil {
			h.logger.Error().Err(err).Msg("decrypt failed")
			return err
		}
	} else {
		plaintext = rawPayload
	}

	var payload aiPublishPayload
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		h.logger.Error().Err(err).Msg("unmarshal payload failed")
		return err
	}

	analysisID, err := uuid.Parse(payload.AnalysisID)
	if err != nil {
		return fmt.Errorf("invalid analysis_id: %w", err)
	}
	runID, err := uuid.Parse(payload.RunID)
	if err != nil {
		return fmt.Errorf("invalid run_id: %w", err)
	}
	projectID, err := uuid.Parse(payload.ProjectID)
	if err != nil {
		return fmt.Errorf("invalid project_id: %w", err)
	}

	// Fetch analysis — skip if not found or not successful.
	analysis, err := h.aiRepo.GetAnalysisByID(ctx, analysisID)
	if err != nil {
		h.logger.Warn().Err(err).Str("analysis_id", payload.AnalysisID).Msg("analysis not found, skipping publish")
		return nil
	}
	if analysis.Status != models.AIAnalysisStatusSuccess {
		h.logger.Debug().
			Str("analysis_id", payload.AnalysisID).
			Str("status", string(analysis.Status)).
			Msg("analysis not successful, skipping publish")
		return nil
	}

	// Fetch run.
	run, err := h.runs.GetByID(ctx, runID)
	if err != nil {
		h.logger.Warn().Err(err).Str("run_id", payload.RunID).Msg("run not found, skipping publish")
		return nil
	}

	// Fetch project.
	project, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		h.logger.Warn().Err(err).Str("project_id", payload.ProjectID).Msg("project not found, skipping publish")
		return nil
	}

	// Guard: need repo URL and git commit for GitHub publishing.
	if project.RepoURL == nil || *project.RepoURL == "" {
		h.logger.Debug().Msg("no repo URL on project, skipping publish")
		return nil
	}
	if run.GitCommit == nil || *run.GitCommit == "" {
		h.logger.Debug().Msg("no git commit on run, skipping publish")
		return nil
	}

	// Get GitHub token.
	accessToken, err := h.getGitHubToken(ctx, project)
	if err != nil || accessToken == "" {
		h.logger.Debug().Err(err).Msg("no GitHub token available, skipping publish")
		return nil
	}

	// Parse owner/repo from URL.
	owner, repoName, err := parseGitHubOwnerRepoFromURL(*project.RepoURL)
	if err != nil {
		h.logger.Debug().Err(err).Str("url", *project.RepoURL).Msg("failed to parse GitHub URL, skipping publish")
		return nil
	}

	// Build target URL.
	targetURL := fmt.Sprintf("%s/projects/%s/runs/%s", strings.TrimRight(h.baseURL, "/"), project.ID, run.ID)

	// Publish PR comment.
	if err := h.publishPRComment(ctx, analysis, run, owner, repoName, accessToken, targetURL); err != nil {
		// GitHub API errors should be retried.
		return err
	}

	// Publish check run.
	if err := h.publishCheckRun(ctx, analysis, run, owner, repoName, accessToken, targetURL); err != nil {
		return err
	}

	h.logger.Info().
		Str("analysis_id", payload.AnalysisID).
		Str("run_id", payload.RunID).
		Msg("AI analysis published to GitHub")

	if h.metrics != nil {
		h.metrics.AIPublicationsTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("destination", "github"),
				attribute.String("status", "success"),
			))
	}

	return nil
}

// getGitHubToken obtains a GitHub access token — prefers GitHub App, falls back to OAuth.
func (h *AIPublishHandler) getGitHubToken(ctx context.Context, project *models.Project) (string, error) {
	// Prefer GitHub App installation token.
	if project.GitHubInstallationID != nil && h.githubApp != nil && h.githubInstallations != nil {
		inst, err := h.githubInstallations.GetByID(ctx, *project.GitHubInstallationID)
		if err == nil && inst != nil {
			token, err := h.githubApp.FetchInstallationToken(ctx, inst.InstallationID)
			if err == nil && token != nil {
				return token.Token, nil
			}
		}
	}

	// Fallback to OAuth token.
	if h.providerTokens != nil && h.providerEncrypt != nil && project.RepoLinkedByUserID != nil {
		tok, err := h.providerTokens.GetByUserAndProvider(ctx, *project.RepoLinkedByUserID, "github")
		if err == nil && tok != nil {
			decrypted, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
			if err == nil {
				return decrypted, nil
			}
		}
	}

	return "", fmt.Errorf("no GitHub token available")
}

// publishPRComment publishes or updates a PR comment with the analysis results.
func (h *AIPublishHandler) publishPRComment(ctx context.Context, analysis *models.AIAnalysis, run *models.Run, owner, repoName, accessToken, targetURL string) error {
	// Skip if no PR number.
	if run.PRNumber == nil {
		h.logger.Debug().Str("run_id", run.ID.String()).Msg("no PR number, skipping PR comment")
		return nil
	}

	// Check idempotency.
	existing, err := h.aiRepo.GetPublicationByRunAndDestination(ctx, run.ID, models.AIPublicationDestGitHubPRComment)
	if err == nil && existing != nil {
		if existing.Status == models.AIPublicationStatusSent && existing.ExternalID != nil {
			// Update existing comment.
			commentBody := map[string]string{
				"body": buildAICommentBody(analysis, run, targetURL),
			}
			commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%s", owner, repoName, *existing.ExternalID)
			if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPatch, commentURL, commentBody, nil); err != nil {
				h.logger.Warn().Err(err).Msg("failed to update PR comment")
				errMsg := err.Error()
				_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusFailed, nil, &errMsg)
				return err
			}
			_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusUpdated, existing.ExternalID, nil)
			return nil
		}
		// Previous attempt failed — retry by posting a new comment, then update the existing record.
		commentBody := map[string]string{
			"body": buildAICommentBody(analysis, run, targetURL),
		}
		commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repoName, *run.PRNumber)
		var respBody struct {
			ID int64 `json:"id"`
		}
		if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPost, commentURL, commentBody, &respBody); err != nil {
			h.logger.Warn().Err(err).Msg("failed to create PR comment")
			errMsg := err.Error()
			_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusFailed, nil, &errMsg)
			return err
		}
		externalID := fmt.Sprintf("%d", respBody.ID)
		_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusSent, &externalID, nil)
		return nil
	}

	// Create new comment.
	commentBody := map[string]string{
		"body": buildAICommentBody(analysis, run, targetURL),
	}
	commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repoName, *run.PRNumber)

	var respBody struct {
		ID int64 `json:"id"`
	}
	if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPost, commentURL, commentBody, &respBody); err != nil {
		h.logger.Warn().Err(err).Msg("failed to create PR comment")
		pub := &models.AIPublication{
			AnalysisID:   analysis.ID,
			RunID:        run.ID,
			Destination:  models.AIPublicationDestGitHubPRComment,
			Status:       models.AIPublicationStatusFailed,
			ErrorMessage: strPtr(err.Error()),
		}
		_ = h.aiRepo.CreatePublication(ctx, pub)
		return err
	}

	// Store publication.
	externalID := fmt.Sprintf("%d", respBody.ID)
	pub := &models.AIPublication{
		AnalysisID:  analysis.ID,
		RunID:       run.ID,
		Destination: models.AIPublicationDestGitHubPRComment,
		ExternalID:  &externalID,
		Status:      models.AIPublicationStatusSent,
	}
	if err := h.aiRepo.CreatePublication(ctx, pub); err != nil {
		h.logger.Warn().Err(err).Msg("failed to persist PR comment publication")
	}

	return nil
}

// publishCheckRun publishes or updates a GitHub check run with the analysis results.
func (h *AIPublishHandler) publishCheckRun(ctx context.Context, analysis *models.AIAnalysis, run *models.Run, owner, repoName, accessToken, targetURL string) error {
	sha := *run.GitCommit

	// Check idempotency.
	existing, err := h.aiRepo.GetPublicationByRunAndDestination(ctx, run.ID, models.AIPublicationDestGitHubCheck)
	if err == nil && existing != nil {
		if existing.Status == models.AIPublicationStatusSent && existing.ExternalID != nil {
			// Update existing check run.
			var checkRunID int64
			if _, err := fmt.Sscanf(*existing.ExternalID, "%d", &checkRunID); err == nil {
				checkOutput := buildAICheckRunOutput(analysis)
				now := time.Now()
				req := notification.CheckRunRequest{
					Status:      "completed",
					Conclusion:  "neutral",
					DetailsURL:  targetURL,
					Output:      checkOutput,
					CompletedAt: &now,
				}
				if err := notification.UpdateCheckRun(ctx, accessToken, owner, repoName, checkRunID, req); err != nil {
					h.logger.Warn().Err(err).Msg("failed to update check run")
					errMsg := err.Error()
					_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusFailed, nil, &errMsg)
					return err
				}
				_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusUpdated, existing.ExternalID, nil)
			}
			return nil
		}
		// Previous attempt failed — retry by creating a new check run, then update the existing record.
		checkOutput := buildAICheckRunOutput(analysis)
		now := time.Now()
		req := notification.CheckRunRequest{
			Name:        "Dagryn / AI Analysis",
			HeadSHA:     sha,
			Status:      "completed",
			Conclusion:  "neutral",
			DetailsURL:  targetURL,
			Output:      checkOutput,
			CompletedAt: &now,
		}
		newCheckRunID, err := notification.CreateCheckRun(ctx, accessToken, owner, repoName, req)
		if err != nil {
			h.logger.Warn().Err(err).Msg("failed to create check run")
			errMsg := err.Error()
			_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusFailed, nil, &errMsg)
			return err
		}
		externalID := fmt.Sprintf("%d", newCheckRunID)
		_ = h.aiRepo.UpdatePublication(ctx, existing.ID, models.AIPublicationStatusSent, &externalID, nil)
		return nil
	}

	// Create new check run.
	checkOutput := buildAICheckRunOutput(analysis)
	now := time.Now()
	req := notification.CheckRunRequest{
		Name:        "Dagryn / AI Analysis",
		HeadSHA:     sha,
		Status:      "completed",
		Conclusion:  "neutral",
		DetailsURL:  targetURL,
		Output:      checkOutput,
		CompletedAt: &now,
	}

	checkRunID, err := notification.CreateCheckRun(ctx, accessToken, owner, repoName, req)
	if err != nil {
		h.logger.Warn().Err(err).Msg("failed to create check run")
		pub := &models.AIPublication{
			AnalysisID:   analysis.ID,
			RunID:        run.ID,
			Destination:  models.AIPublicationDestGitHubCheck,
			Status:       models.AIPublicationStatusFailed,
			ErrorMessage: strPtr(err.Error()),
		}
		_ = h.aiRepo.CreatePublication(ctx, pub)
		return err
	}

	// Store publication.
	externalID := fmt.Sprintf("%d", checkRunID)
	pub := &models.AIPublication{
		AnalysisID:  analysis.ID,
		RunID:       run.ID,
		Destination: models.AIPublicationDestGitHubCheck,
		ExternalID:  &externalID,
		Status:      models.AIPublicationStatusSent,
	}
	if err := h.aiRepo.CreatePublication(ctx, pub); err != nil {
		h.logger.Warn().Err(err).Msg("failed to persist check run publication")
	}

	return nil
}

// buildAICommentBody formats the analysis as a GitHub PR comment in markdown.
func buildAICommentBody(analysis *models.AIAnalysis, run *models.Run, targetURL string) string {
	var b strings.Builder

	b.WriteString("**Dagryn AI Failure Analysis**\n\n")

	if analysis.Summary != nil && *analysis.Summary != "" {
		fmt.Fprintf(&b, "**Summary:** %s\n\n", *analysis.Summary)
	}

	if analysis.RootCause != nil && *analysis.RootCause != "" {
		fmt.Fprintf(&b, "**Root Cause:** %s\n\n", *analysis.RootCause)
	}

	if analysis.Confidence != nil {
		pct := int(*analysis.Confidence * 100)
		fmt.Fprintf(&b, "**Confidence:** %d%% %s\n\n", pct, confidenceBar(pct))
	}

	// Unmarshal evidence_json — new format stores full AnalysisOutput (object),
	// old format stores just []EvidenceItem (array). Handle both.
	var fullOutput aitypes.AnalysisOutput
	var evidence []aitypes.EvidenceItem
	if len(analysis.EvidenceJSON) > 0 {
		if err := json.Unmarshal(analysis.EvidenceJSON, &fullOutput); err == nil && fullOutput.Summary != "" {
			// New format: full output object.
			evidence = fullOutput.Evidence
		} else {
			// Old format: bare evidence array.
			_ = json.Unmarshal(analysis.EvidenceJSON, &evidence)
		}
	}

	if len(fullOutput.LikelyFiles) > 0 {
		b.WriteString("**Likely Files:**\n")
		for _, f := range fullOutput.LikelyFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	if len(fullOutput.RecommendedActions) > 0 {
		b.WriteString("**Recommended Actions:**\n")
		for i, a := range fullOutput.RecommendedActions {
			fmt.Fprintf(&b, "%d. %s\n", i+1, a)
		}
		b.WriteString("\n")
	}

	if len(evidence) > 0 {
		b.WriteString("<details>\n<summary>Evidence</summary>\n\n")
		b.WriteString("| Task | Reason |\n")
		b.WriteString("|------|--------|\n")
		for _, e := range evidence {
			fmt.Fprintf(&b, "| %s | %s |\n", e.Task, e.Reason)
		}
		b.WriteString("\n</details>\n\n")
	}

	b.WriteString("---\n")
	fmt.Fprintf(&b, "_[View full analysis](%s)_ · _Powered by [Dagryn AI](https://dagryn.dev)_\n", targetURL)

	return b.String()
}

// buildAICheckRunOutput creates a GitHub check run output from the analysis.
func buildAICheckRunOutput(analysis *models.AIAnalysis) *notification.CheckRunOutput {
	title := "AI Failure Analysis"
	if analysis.Confidence != nil {
		title = fmt.Sprintf("AI Analysis: %d%% confidence", int(*analysis.Confidence*100))
	}

	summary := ""
	if analysis.Summary != nil {
		summary = *analysis.Summary
	}

	// Build text body (markdown).
	var text strings.Builder
	if analysis.RootCause != nil && *analysis.RootCause != "" {
		fmt.Fprintf(&text, "**Root Cause:** %s\n\n", *analysis.RootCause)
	}

	// Parse evidence from evidence_json (handles both full output and bare array formats).
	var evidence []aitypes.EvidenceItem
	if len(analysis.EvidenceJSON) > 0 {
		var fullOutput aitypes.AnalysisOutput
		if err := json.Unmarshal(analysis.EvidenceJSON, &fullOutput); err == nil && fullOutput.Summary != "" {
			evidence = fullOutput.Evidence
		} else {
			_ = json.Unmarshal(analysis.EvidenceJSON, &evidence)
		}
	}

	if len(evidence) > 0 {
		text.WriteString("**Evidence:**\n\n")
		text.WriteString("| Task | Reason |\n")
		text.WriteString("|------|--------|\n")
		for _, e := range evidence {
			fmt.Fprintf(&text, "| %s | %s |\n", e.Task, e.Reason)
		}
	}

	return &notification.CheckRunOutput{
		Title:   title,
		Summary: summary,
		Text:    text.String(),
	}
}

// confidenceBar returns a simple visual bar for confidence percentage.
func confidenceBar(pct int) string {
	filled := pct / 10
	empty := 10 - filled
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", empty)
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}
