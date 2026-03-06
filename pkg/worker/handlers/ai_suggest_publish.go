package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/notification"
	"github.com/rs/zerolog"
)

// aiSuggestPublishRepo defines repo operations needed by the suggest publish handler.
type aiSuggestPublishRepo interface {
	GetAnalysisByID(ctx context.Context, id uuid.UUID) (*models.AIAnalysis, error)
	ListPendingSuggestionsByAnalysis(ctx context.Context, analysisID uuid.UUID) ([]models.AISuggestion, error)
	UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status models.AISuggestionStatus, githubCommentID *string, failureReason *string) error
	GetPublicationByRunAndDestination(ctx context.Context, runID uuid.UUID, dest models.AIPublicationDestination) (*models.AIPublication, error)
	CreatePublication(ctx context.Context, p *models.AIPublication) error
	UpdatePublication(ctx context.Context, id uuid.UUID, status models.AIPublicationStatus, externalID *string, errorMessage *string) error
}

// aiSuggestPublishPayload mirrors job.AISuggestPublishPayload to avoid import cycle.
type aiSuggestPublishPayload struct {
	AnalysisID string `json:"analysis_id"`
	RunID      string `json:"run_id"`
	ProjectID  string `json:"project_id"`
}

// AISuggestPublishHandler processes ai_suggest:publish jobs.
type AISuggestPublishHandler struct {
	aiRepo              aiSuggestPublishRepo
	runs                runRepo
	projects            projectRepo
	providerTokens      providerTokenStore
	providerEncrypt     encrypt.Encrypt
	githubApp           GitHubAppClient
	githubInstallations githubInstallationStore
	encrypter           encrypt.Encrypt
	logger              zerolog.Logger
}

// NewAISuggestPublishHandler creates a new suggestion publish handler.
func NewAISuggestPublishHandler(
	aiRepo aiSuggestPublishRepo,
	runs runRepo,
	projects projectRepo,
	providerTokens providerTokenStore,
	providerEncrypt encrypt.Encrypt,
	githubApp GitHubAppClient,
	githubInstallations githubInstallationStore,
	encrypter encrypt.Encrypt,
	logger zerolog.Logger,
) *AISuggestPublishHandler {
	return &AISuggestPublishHandler{
		aiRepo:              aiRepo,
		runs:                runs,
		projects:            projects,
		providerTokens:      providerTokens,
		providerEncrypt:     providerEncrypt,
		githubApp:           githubApp,
		githubInstallations: githubInstallations,
		encrypter:           encrypter,
		logger:              logger.With().Str("handler", "ai_suggest_publish").Logger(),
	}
}

// Handle processes the ai_suggest:publish task.
func (h *AISuggestPublishHandler) Handle(ctx context.Context, t *asynq.Task) error {
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

	var payload aiSuggestPublishPayload
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

	// Fetch analysis.
	analysis, err := h.aiRepo.GetAnalysisByID(ctx, analysisID)
	if err != nil {
		h.logger.Warn().Err(err).Msg("analysis not found")
		return nil
	}
	if analysis.Status != models.AIAnalysisStatusSuccess {
		return nil
	}

	// Fetch pending suggestions.
	suggestions, err := h.aiRepo.ListPendingSuggestionsByAnalysis(ctx, analysisID)
	if err != nil || len(suggestions) == 0 {
		h.logger.Debug().Int("count", len(suggestions)).Msg("no pending suggestions to publish")
		return nil
	}

	// Fetch run and project.
	run, err := h.runs.GetByID(ctx, runID)
	if err != nil {
		h.logger.Warn().Err(err).Msg("run not found")
		return nil
	}
	project, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		h.logger.Warn().Err(err).Msg("project not found")
		return nil
	}

	// Guard: need PR and commit for review comments.
	if run.PRNumber == nil || run.GitCommit == nil {
		h.logger.Debug().Msg("no PR or commit, skipping suggestion publish")
		return nil
	}
	if project.RepoURL == nil || *project.RepoURL == "" {
		h.logger.Debug().Msg("no repo URL, skipping suggestion publish")
		return nil
	}

	// Get GitHub token.
	accessToken, err := h.getGitHubToken(ctx, project)
	if err != nil || accessToken == "" {
		h.logger.Debug().Err(err).Msg("no GitHub token, skipping suggestion publish")
		return nil
	}

	owner, repoName, err := parseGitHubOwnerRepoFromURL(*project.RepoURL)
	if err != nil {
		h.logger.Debug().Err(err).Msg("failed to parse GitHub URL")
		return nil
	}

	// Check idempotency for PR review publication.
	existing, err := h.aiRepo.GetPublicationByRunAndDestination(ctx, runID, models.AIPublicationDestGitHubPRReview)
	if err == nil && existing != nil && (existing.Status == models.AIPublicationStatusSent || existing.Status == models.AIPublicationStatusUpdated) {
		h.logger.Debug().Msg("PR review already published, skipping")
		return nil
	}

	// Build review comments from suggestions.
	comments := buildReviewComments(suggestions)

	if len(comments) == 0 {
		h.logger.Debug().Msg("no valid review comments to post")
		return nil
	}

	// Post as a single PR review.
	reviewBody := buildReviewBody(analysis, suggestions)
	reviewReq := map[string]interface{}{
		"body":      reviewBody,
		"event":     "COMMENT",
		"commit_id": *run.GitCommit,
		"comments":  comments,
	}

	reviewURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/reviews", owner, repoName, *run.PRNumber)
	var reviewResp struct {
		ID int64 `json:"id"`
	}
	if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPost, reviewURL, reviewReq, &reviewResp); err != nil {
		h.logger.Warn().Err(err).Msg("failed to create PR review")
		pub := &models.AIPublication{
			AnalysisID:   analysisID,
			RunID:        runID,
			Destination:  models.AIPublicationDestGitHubPRReview,
			Status:       models.AIPublicationStatusFailed,
			ErrorMessage: strPtr(err.Error()),
		}
		_ = h.aiRepo.CreatePublication(ctx, pub)
		return err // Retry on GitHub API error.
	}

	// Mark suggestions as posted.
	reviewIDStr := fmt.Sprintf("%d", reviewResp.ID)
	for i := range suggestions {
		_ = h.aiRepo.UpdateSuggestionStatus(ctx, suggestions[i].ID, models.AISuggestionStatusPosted, &reviewIDStr, nil)
	}

	// Create publication record.
	pub := &models.AIPublication{
		AnalysisID:  analysisID,
		RunID:       runID,
		Destination: models.AIPublicationDestGitHubPRReview,
		ExternalID:  &reviewIDStr,
		Status:      models.AIPublicationStatusSent,
	}
	if err := h.aiRepo.CreatePublication(ctx, pub); err != nil {
		h.logger.Warn().Err(err).Msg("failed to persist review publication")
	}

	h.logger.Info().
		Str("analysis_id", payload.AnalysisID).
		Int("suggestions_posted", len(suggestions)).
		Int64("review_id", reviewResp.ID).
		Msg("suggestions published as PR review")

	return nil
}

// getGitHubToken obtains a GitHub access token for the project.
func (h *AISuggestPublishHandler) getGitHubToken(ctx context.Context, project *models.Project) (string, error) {
	if project.GitHubInstallationID != nil && h.githubApp != nil && h.githubInstallations != nil {
		inst, err := h.githubInstallations.GetByID(ctx, *project.GitHubInstallationID)
		if err == nil && inst != nil {
			token, err := h.githubApp.FetchInstallationToken(ctx, inst.InstallationID)
			if err == nil && token != nil {
				return token.Token, nil
			}
		}
	}
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

// reviewComment represents a single review comment for the GitHub PR review API.
type reviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
}

// buildReviewComments builds GitHub PR review comments with suggestion blocks.
func buildReviewComments(suggestions []models.AISuggestion) []reviewComment {
	var comments []reviewComment
	for _, s := range suggestions {
		if s.FilePath == "" || s.SuggestedCode == "" {
			continue
		}
		body := buildSuggestionCommentBody(s)
		comment := reviewComment{
			Path: s.FilePath,
			Line: s.EndLine,
			Side: "RIGHT",
			Body: body,
		}
		comments = append(comments, comment)
	}
	return comments
}

// buildSuggestionCommentBody builds a GitHub suggestion block for a single suggestion.
func buildSuggestionCommentBody(s models.AISuggestion) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**Dagryn AI Suggestion** (%.0f%% confidence)\n\n", s.Confidence*100)
	fmt.Fprintf(&b, "%s\n\n", s.Explanation)
	fmt.Fprintf(&b, "```suggestion\n%s\n```\n", s.SuggestedCode)
	return b.String()
}

// buildReviewBody builds the top-level review body summarizing all suggestions.
func buildReviewBody(analysis *models.AIAnalysis, suggestions []models.AISuggestion) string {
	var b strings.Builder
	b.WriteString("**Dagryn AI Code Suggestions**\n\n")
	if analysis.Summary != nil {
		fmt.Fprintf(&b, "%s\n\n", *analysis.Summary)
	}
	fmt.Fprintf(&b, "Generated %d suggestion(s) based on failure analysis.\n\n", len(suggestions))
	b.WriteString("_Accept individual suggestions using GitHub's \"Apply suggestion\" button._\n")
	b.WriteString("_Powered by [Dagryn AI](https://dagryn.dev)_")
	return b.String()
}
