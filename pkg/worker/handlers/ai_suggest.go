package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/mujhtech/dagryn/pkg/ai/provider"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AISuggestConfig holds configuration for suggestion generation guardrails.
type AISuggestConfig struct {
	MaxSuggestionLines        int
	MaxSuggestionsPerAnalysis int
	MaxFilesChanged           int
	MinConfidence             float64
	AllowedPaths              []string
	BlockedPaths              []string
}

// DefaultAISuggestConfig returns sensible defaults for suggestion guardrails.
func DefaultAISuggestConfig() AISuggestConfig {
	return AISuggestConfig{
		MaxSuggestionLines:        20,
		MaxSuggestionsPerAnalysis: 5,
		MaxFilesChanged:           5,
		MinConfidence:             0.70,
	}
}

// aiSuggestRepo defines AI repo operations needed by the suggest handler.
type aiSuggestRepo interface {
	GetAnalysisByID(ctx context.Context, id uuid.UUID) (*models.AIAnalysis, error)
	CreateSuggestion(ctx context.Context, s *models.AISuggestion) error
	UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status models.AISuggestionStatus, githubCommentID *string, failureReason *string) error
}

// aiSuggestPayload mirrors job.AISuggestRunPayload to avoid import cycle.
type aiSuggestPayload struct {
	AnalysisID string           `json:"analysis_id"`
	RunID      string           `json:"run_id"`
	ProjectID  string           `json:"project_id"`
	AIConfig   *aiProjectConfig `json:"ai_config,omitempty"`
}

// AISuggestHandler processes ai_suggest:run jobs.
type AISuggestHandler struct {
	aiRepo       aiSuggestRepo
	runs         runRepo
	encrypter    encrypt.Encrypt
	config       AISuggestConfig
	serverConfig *AIAnalysisConfig // Managed-mode fallback
	logger       zerolog.Logger
	metrics      *telemetry.Metrics
}

// NewAISuggestHandler creates a new suggestion generation handler.
func NewAISuggestHandler(
	aiRepo aiSuggestRepo,
	runs runRepo,
	encrypter encrypt.Encrypt,
	config AISuggestConfig,
	serverConfig *AIAnalysisConfig,
	logger zerolog.Logger,
	metrics ...*telemetry.Metrics,
) *AISuggestHandler {
	var m *telemetry.Metrics
	if len(metrics) > 0 {
		m = metrics[0]
	}
	return &AISuggestHandler{
		aiRepo:       aiRepo,
		runs:         runs,
		encrypter:    encrypter,
		config:       config,
		serverConfig: serverConfig,
		logger:       logger.With().Str("handler", "ai_suggest").Logger(),
		metrics:      m,
	}
}

// buildSuggestProviderConfig builds a provider.ProviderConfig from the project config,
// falling back to server config for managed mode.
func (h *AISuggestHandler) buildSuggestProviderConfig(projCfg *aiProjectConfig) provider.ProviderConfig {
	if projCfg == nil {
		// Backward compat: old payloads — use server config.
		if h.serverConfig == nil {
			return provider.ProviderConfig{}
		}
		return provider.ProviderConfig{
			BackendMode:    h.serverConfig.BackendMode,
			Provider:       h.serverConfig.Provider,
			APIKey:         h.serverConfig.APIKey,
			MaxTokens:      h.serverConfig.MaxTokens,
			TimeoutSeconds: h.serverConfig.TimeoutSeconds,
			AgentEndpoint:  h.serverConfig.AgentEndpoint,
			AgentToken:     h.serverConfig.AgentToken,
		}
	}

	cfg := provider.ProviderConfig{
		BackendMode:    projCfg.BackendMode,
		Provider:       projCfg.Provider,
		Model:          projCfg.Model,
		TimeoutSeconds: projCfg.TimeoutSeconds,
		MaxTokens:      projCfg.MaxTokens,
	}

	switch projCfg.BackendMode {
	case "byok":
		cfg.APIKey = projCfg.APIKey
	case "agent":
		cfg.AgentEndpoint = projCfg.AgentEndpoint
		cfg.AgentToken = projCfg.AgentToken
	case "managed", "":
		if h.serverConfig != nil {
			cfg.APIKey = h.serverConfig.APIKey
			cfg.AgentEndpoint = h.serverConfig.AgentEndpoint
			cfg.AgentToken = h.serverConfig.AgentToken
			if cfg.Provider == "" {
				cfg.Provider = h.serverConfig.Provider
			}
			if cfg.TimeoutSeconds == 0 {
				cfg.TimeoutSeconds = h.serverConfig.TimeoutSeconds
			}
			if cfg.MaxTokens == 0 {
				cfg.MaxTokens = h.serverConfig.MaxTokens
			}
		}
		cfg.BackendMode = "managed"
	}

	return cfg
}

// Handle processes the ai_suggest:run task.
func (h *AISuggestHandler) Handle(ctx context.Context, t *asynq.Task) error {
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

	var payload aiSuggestPayload
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

	// Fetch analysis — must be successful.
	analysis, err := h.aiRepo.GetAnalysisByID(ctx, analysisID)
	if err != nil {
		h.logger.Warn().Err(err).Str("analysis_id", payload.AnalysisID).Msg("analysis not found")
		return nil
	}
	if analysis.Status != models.AIAnalysisStatusSuccess {
		h.logger.Debug().Str("status", string(analysis.Status)).Msg("analysis not successful, skipping suggestions")
		return nil
	}

	// Fetch run for metadata.
	run, err := h.runs.GetByID(ctx, runID)
	if err != nil {
		h.logger.Warn().Err(err).Str("run_id", payload.RunID).Msg("run not found")
		return nil
	}

	// Build suggestion provider per-job from project config.
	suggestProviderCfg := h.buildSuggestProviderConfig(payload.AIConfig)
	suggestProvider, spErr := provider.NewSuggestionProvider(suggestProviderCfg, h.logger)
	if spErr != nil {
		h.logger.Debug().Err(spErr).Msg("no suggestion provider available, skipping")
		return nil
	}
	if suggestProvider == nil {
		h.logger.Debug().Msg("no suggestion provider configured, skipping")
		return nil
	}

	// Build analysis output from stored data.
	var analysisOutput aitypes.AnalysisOutput
	if analysis.Summary != nil {
		analysisOutput.Summary = *analysis.Summary
	}
	if analysis.RootCause != nil {
		analysisOutput.RootCause = *analysis.RootCause
	}
	if analysis.Confidence != nil {
		analysisOutput.Confidence = *analysis.Confidence
	}
	if len(analysis.EvidenceJSON) > 0 {
		_ = json.Unmarshal(analysis.EvidenceJSON, &analysisOutput.Evidence)
	}

	// Build suggestion input.
	input := aitypes.SuggestionInput{
		RunID:     payload.RunID,
		ProjectID: payload.ProjectID,
		Analysis:  analysisOutput,
	}
	if run.GitBranch != nil {
		input.GitBranch = *run.GitBranch
	}
	if run.GitCommit != nil {
		input.GitCommit = *run.GitCommit
	}

	// Call provider.
	suggestions, err := suggestProvider.ProposeSuggestions(ctx, input)
	if err != nil {
		h.logger.Error().Err(err).Msg("suggestion provider failed")
		return nil // Don't retry — provider failures shouldn't block.
	}

	// Apply guardrails from project config when available, falling back to defaults.
	suggestCfg := h.config
	if payload.AIConfig != nil {
		if payload.AIConfig.MinConfidence > 0 {
			suggestCfg.MinConfidence = payload.AIConfig.MinConfidence
		}
		if payload.AIConfig.MaxSuggestionsPerAnalysis > 0 {
			suggestCfg.MaxSuggestionsPerAnalysis = payload.AIConfig.MaxSuggestionsPerAnalysis
		}
		if len(payload.AIConfig.BlockedPaths) > 0 {
			suggestCfg.BlockedPaths = payload.AIConfig.BlockedPaths
		}
		if len(payload.AIConfig.AllowedPaths) > 0 {
			suggestCfg.AllowedPaths = payload.AIConfig.AllowedPaths
		}
	}

	// Apply guardrails and persist.
	stored := 0
	for _, s := range suggestions {
		if stored >= suggestCfg.MaxSuggestionsPerAnalysis {
			break
		}

		// Validate guardrails.
		reason := validateSuggestion(s, suggestCfg)
		status := models.AISuggestionStatusPending
		var failureReason *string
		if reason != "" {
			status = models.AISuggestionStatusFailedValidation
			failureReason = &reason
		}

		suggestion := &models.AISuggestion{
			AnalysisID:    analysisID,
			RunID:         runID,
			FilePath:      s.FilePath,
			StartLine:     s.StartLine,
			EndLine:       s.EndLine,
			OriginalCode:  s.OriginalCode,
			SuggestedCode: s.SuggestedCode,
			Explanation:   s.Explanation,
			Confidence:    s.Confidence,
			Status:        status,
			FailureReason: failureReason,
		}

		if err := h.aiRepo.CreateSuggestion(ctx, suggestion); err != nil {
			h.logger.Warn().Err(err).Str("file", s.FilePath).Msg("failed to persist suggestion")
			continue
		}
		stored++
	}

	h.logger.Info().
		Str("analysis_id", payload.AnalysisID).
		Int("generated", len(suggestions)).
		Int("stored", stored).
		Msg("suggestions generated")

	if h.metrics != nil && stored > 0 {
		h.metrics.AISuggestionsTotal.Add(ctx, int64(stored),
			metric.WithAttributes(attribute.String("status", "stored")))
	}

	return nil
}

// validateSuggestion checks a suggestion against guardrails. Returns empty string if valid.
func validateSuggestion(s aitypes.SuggestionOutput, cfg AISuggestConfig) string {
	// Check confidence threshold.
	if s.Confidence < cfg.MinConfidence {
		return fmt.Sprintf("confidence %.2f below threshold %.2f", s.Confidence, cfg.MinConfidence)
	}

	// Check line count.
	lineCount := s.EndLine - s.StartLine + 1
	if cfg.MaxSuggestionLines > 0 && lineCount > cfg.MaxSuggestionLines {
		return fmt.Sprintf("suggestion spans %d lines, max is %d", lineCount, cfg.MaxSuggestionLines)
	}

	// Check blocked paths.
	for _, pattern := range cfg.BlockedPaths {
		if matched, _ := filepath.Match(pattern, s.FilePath); matched {
			return fmt.Sprintf("file %s matches blocked path %s", s.FilePath, pattern)
		}
	}

	// Check allowed paths (if set, file must match at least one).
	if len(cfg.AllowedPaths) > 0 {
		matched := false
		for _, pattern := range cfg.AllowedPaths {
			if m, _ := filepath.Match(pattern, s.FilePath); m {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Sprintf("file %s not in allowed paths", s.FilePath)
		}
	}

	// Check empty suggestion.
	if s.SuggestedCode == "" {
		return "empty suggested code"
	}
	if s.FilePath == "" {
		return "empty file path"
	}

	return ""
}
