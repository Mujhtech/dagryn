package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/ai"
	"github.com/mujhtech/dagryn/pkg/ai/evidence"
	"github.com/mujhtech/dagryn/pkg/ai/provider"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AIAnalysisConfig holds configuration for the AI analysis job handler.
type AIAnalysisConfig struct {
	Enabled               bool
	BackendMode           string
	Provider              string
	APIKey                string
	TimeoutSeconds        int
	MaxTokens             int
	AgentEndpoint         string
	AgentToken            string
	MaxAnalysesPerHour    int
	CooldownSeconds       int
	MaxConcurrentAnalyses int
}

// aiProjectConfig mirrors job.AIProjectConfig to avoid an import cycle.
type aiProjectConfig struct {
	BackendMode               string   `json:"backend_mode"`
	Provider                  string   `json:"provider,omitempty"`
	Model                     string   `json:"model,omitempty"`
	APIKey                    string   `json:"api_key,omitempty"`
	AgentEndpoint             string   `json:"agent_endpoint,omitempty"`
	AgentToken                string   `json:"agent_token,omitempty"`
	TimeoutSeconds            int      `json:"timeout_seconds,omitempty"`
	MaxTokens                 int      `json:"max_tokens,omitempty"`
	Mode                      string   `json:"mode,omitempty"`
	MinConfidence             float64  `json:"min_confidence,omitempty"`
	MaxSuggestionsPerAnalysis int      `json:"max_suggestions_per_analysis,omitempty"`
	BlockedPaths              []string `json:"blocked_paths,omitempty"`
	AllowedPaths              []string `json:"allowed_paths,omitempty"`
	MaxAnalysesPerHour        int      `json:"max_analyses_per_hour,omitempty"`
	CooldownSeconds           int      `json:"cooldown_seconds,omitempty"`
	MaxConcurrentAnalyses     int      `json:"max_concurrent_analyses,omitempty"`
}

// aiAnalysisPayload mirrors job.AIAnalysisPayload to avoid an import cycle.
type aiAnalysisPayload struct {
	RunID        string           `json:"run_id"`
	ProjectID    string           `json:"project_id"`
	GitBranch    string           `json:"git_branch,omitempty"`
	GitCommit    string           `json:"git_commit,omitempty"`
	WorkflowName string           `json:"workflow_name,omitempty"`
	Targets      string           `json:"targets,omitempty"`
	AIConfig     *aiProjectConfig `json:"ai_config,omitempty"`
}

// aiRepoForHandler defines the AI repo operations needed by the handler.
type aiRepoForHandler interface {
	ai.AIDataStore
	FindPendingByDedupKey(ctx context.Context, dedupKey string) (*models.AIAnalysis, error)
	SupersedeByBranch(ctx context.Context, projectID uuid.UUID, branch string, excludeCommit string) error
	CountRecentAnalyses(ctx context.Context, projectID uuid.UUID, since time.Time) (int, error)
	UpdateAnalysisDedupKey(ctx context.Context, id uuid.UUID, dedupKey string) error
	CountInProgressAnalyses(ctx context.Context, projectID uuid.UUID) (int, error)
	GetMostRecentAnalysisByKey(ctx context.Context, projectID uuid.UUID, branch, commit string) (*models.AIAnalysis, error)
}

// AIAnalysisHandler processes ai_analysis:run jobs.
type AIAnalysisHandler struct {
	runs        evidence.RunDataSource
	workflows   evidence.WorkflowDataSource
	aiRepo      aiRepoForHandler
	encrypter   encrypt.Encrypt
	aiConfig    *AIAnalysisConfig
	jobEnqueuer JobEnqueuer
	logger      zerolog.Logger
	metrics     *telemetry.Metrics
}

// NewAIAnalysisHandler creates a new AI analysis job handler.
func NewAIAnalysisHandler(
	runs evidence.RunDataSource,
	workflows evidence.WorkflowDataSource,
	aiRepo aiRepoForHandler,
	encrypter encrypt.Encrypt,
	aiConfig *AIAnalysisConfig,
	jobEnqueuer JobEnqueuer,
	logger zerolog.Logger,
	metrics *telemetry.Metrics,
) *AIAnalysisHandler {
	return &AIAnalysisHandler{
		runs:        runs,
		workflows:   workflows,
		aiRepo:      aiRepo,
		encrypter:   encrypter,
		aiConfig:    aiConfig,
		jobEnqueuer: jobEnqueuer,
		logger:      logger.With().Str("handler", "ai_analysis").Logger(),
		metrics:     metrics,
	}
}

// buildProviderConfig builds a provider.ProviderConfig from the project config in the
// job payload, falling back to the server-level config for managed mode or old payloads.
func (h *AIAnalysisHandler) buildProviderConfig(projCfg *aiProjectConfig) provider.ProviderConfig {
	if projCfg == nil {
		// Backward compat: old payloads without project config — use server config.
		if h.aiConfig == nil {
			return provider.ProviderConfig{}
		}
		return provider.ProviderConfig{
			BackendMode:    h.aiConfig.BackendMode,
			Provider:       h.aiConfig.Provider,
			APIKey:         h.aiConfig.APIKey,
			MaxTokens:      h.aiConfig.MaxTokens,
			TimeoutSeconds: h.aiConfig.TimeoutSeconds,
			AgentEndpoint:  h.aiConfig.AgentEndpoint,
			AgentToken:     h.aiConfig.AgentToken,
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
		// Fall back to server config for managed mode.
		if h.aiConfig != nil {
			cfg.APIKey = h.aiConfig.APIKey
			cfg.AgentEndpoint = h.aiConfig.AgentEndpoint
			cfg.AgentToken = h.aiConfig.AgentToken
			if cfg.Provider == "" {
				cfg.Provider = h.aiConfig.Provider
			}
			if cfg.TimeoutSeconds == 0 {
				cfg.TimeoutSeconds = h.aiConfig.TimeoutSeconds
			}
			if cfg.MaxTokens == 0 {
				cfg.MaxTokens = h.aiConfig.MaxTokens
			}
		}
		cfg.BackendMode = "managed"
	}

	return cfg
}

// resolveRateLimits returns effective rate limit values from the project config,
// falling back to the server config defaults.
//
// For managed mode, the project cannot set limits *looser* than the server defaults:
//   - maxPerHour / maxConcurrent: project value is capped at the server value (lower wins)
//   - cooldownSec: project value is floored at the server value (higher wins)
//
// For byok/agent mode the project controls its own limits freely.
func (h *AIAnalysisHandler) resolveRateLimits(projCfg *aiProjectConfig) (maxPerHour, cooldownSec, maxConcurrent int) {
	// Start with server config defaults.
	if h.aiConfig != nil {
		maxPerHour = h.aiConfig.MaxAnalysesPerHour
		cooldownSec = h.aiConfig.CooldownSeconds
		maxConcurrent = h.aiConfig.MaxConcurrentAnalyses
	}

	if projCfg == nil {
		return
	}

	isManaged := projCfg.BackendMode == "managed" || projCfg.BackendMode == ""

	// MaxAnalysesPerHour: project can only make this stricter (lower) in managed mode.
	if projCfg.MaxAnalysesPerHour > 0 {
		if !isManaged || maxPerHour == 0 || projCfg.MaxAnalysesPerHour <= maxPerHour {
			maxPerHour = projCfg.MaxAnalysesPerHour
		}
		// else: managed mode and project value exceeds server cap — keep server value
	}

	// CooldownSeconds: project can only make this stricter (higher) in managed mode.
	if projCfg.CooldownSeconds > 0 {
		if !isManaged || projCfg.CooldownSeconds >= cooldownSec {
			cooldownSec = projCfg.CooldownSeconds
		}
	}

	// MaxConcurrentAnalyses: project can only make this stricter (lower) in managed mode.
	if projCfg.MaxConcurrentAnalyses > 0 {
		if !isManaged || maxConcurrent == 0 || projCfg.MaxConcurrentAnalyses <= maxConcurrent {
			maxConcurrent = projCfg.MaxConcurrentAnalyses
		}
	}

	return
}

// Handle processes the ai_analysis:run task.
func (h *AIAnalysisHandler) Handle(ctx context.Context, t *asynq.Task) error {
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

	var payload aiAnalysisPayload
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		h.logger.Error().Err(err).Msg("unmarshal payload failed")
		return err
	}

	projectID, err := uuid.Parse(payload.ProjectID)
	if err != nil {
		return fmt.Errorf("invalid project_id: %w", err)
	}
	runID, err := uuid.Parse(payload.RunID)
	if err != nil {
		return fmt.Errorf("invalid run_id: %w", err)
	}

	// Build dedup key: projectID:branch:commit:workflowName:targetsHash
	dedupKey := buildDedupKey(projectID, payload.GitBranch, payload.GitCommit, payload.WorkflowName, payload.Targets)

	// Dedup check: skip if a pending analysis already exists for this key.
	existing, err := h.aiRepo.FindPendingByDedupKey(ctx, dedupKey)
	if err == nil && existing != nil {
		h.logger.Info().
			Str("run_id", payload.RunID).
			Str("dedup_key", dedupKey).
			Msg("skipping duplicate analysis")
		return nil
	}

	// Force-push supersede: mark old pending analyses on same branch as superseded.
	if payload.GitBranch != "" && payload.GitCommit != "" {
		if err := h.aiRepo.SupersedeByBranch(ctx, projectID, payload.GitBranch, payload.GitCommit); err != nil {
			h.logger.Warn().Err(err).Msg("supersede by branch failed")
		}
	}

	// Resolve rate limits from project config, falling back to server config.
	maxPerHour, cooldownSec, maxConcurrent := h.resolveRateLimits(payload.AIConfig)
	if maxPerHour <= 0 {
		maxPerHour = 10 // sensible default
	}

	// Rate limit: count recent analyses for this project.
	count, err := h.aiRepo.CountRecentAnalyses(ctx, projectID, time.Now().Add(-1*time.Hour))
	if err != nil {
		h.logger.Warn().Err(err).Msg("count recent analyses failed")
		// Don't block on rate limit check failure; continue.
	} else if count >= maxPerHour {
		h.logger.Warn().
			Int("count", count).
			Int("max", maxPerHour).
			Str("project_id", payload.ProjectID).
			Msg("rate limit exceeded, skipping analysis")
		return nil
	}

	// Cooldown check: skip if a recent analysis exists for the same project+branch+commit.
	if cooldownSec > 0 && payload.GitBranch != "" && payload.GitCommit != "" {
		recent, err := h.aiRepo.GetMostRecentAnalysisByKey(ctx, projectID, payload.GitBranch, payload.GitCommit)
		if err == nil && recent != nil {
			if time.Since(recent.CreatedAt) < time.Duration(cooldownSec)*time.Second {
				h.logger.Info().
					Str("project_id", payload.ProjectID).
					Str("branch", payload.GitBranch).
					Str("commit", payload.GitCommit).
					Int("cooldown_seconds", cooldownSec).
					Msg("cooldown active, skipping analysis")
				return nil
			}
		}
	}

	// Concurrent analysis check: skip if too many are already in progress.
	if maxConcurrent > 0 {
		inProgress, err := h.aiRepo.CountInProgressAnalyses(ctx, projectID)
		if err != nil {
			h.logger.Warn().Err(err).Msg("count in-progress analyses failed")
		} else if inProgress >= maxConcurrent {
			h.logger.Warn().
				Int("in_progress", inProgress).
				Int("max", maxConcurrent).
				Str("project_id", payload.ProjectID).
				Msg("concurrent analysis limit reached, skipping")
			return nil
		}
	}

	// Build provider config from project config, falling back to server config for managed mode.
	providerCfg := h.buildProviderConfig(payload.AIConfig)

	analysisStart := time.Now()

	aiProvider, err := provider.NewProvider(providerCfg, h.logger)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create AI provider")
		if h.metrics != nil {
			h.metrics.AIProviderErrorsTotal.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("provider", providerCfg.Provider),
					attribute.String("error_type", "config"),
				))
		}
		return nil // Don't retry — misconfiguration won't self-heal.
	}

	// Build evidence builder.
	evidenceBuilder := evidence.NewEvidenceBuilder(h.runs, h.workflows, h.logger)

	// Build policy checker from project guardrails when available.
	guardrails := config.AIGuardrailConfig{}
	rateLimits := config.AIRateLimitConfig{}
	if payload.AIConfig != nil {
		guardrails = config.AIGuardrailConfig{
			MinConfidence:             payload.AIConfig.MinConfidence,
			MaxSuggestionsPerAnalysis: payload.AIConfig.MaxSuggestionsPerAnalysis,
			BlockedPaths:              payload.AIConfig.BlockedPaths,
			AllowedPaths:              payload.AIConfig.AllowedPaths,
		}
		rateLimits = config.AIRateLimitConfig{
			MaxAnalysesPerHour:    payload.AIConfig.MaxAnalysesPerHour,
			CooldownSeconds:       payload.AIConfig.CooldownSeconds,
			MaxConcurrentAnalyses: payload.AIConfig.MaxConcurrentAnalyses,
		}
	}
	policyChecker := ai.NewPolicyChecker(guardrails, rateLimits, h.logger)

	// Create orchestrator and run analysis.
	orchestrator := ai.NewOrchestrator(evidenceBuilder, aiProvider, policyChecker, h.aiRepo, h.logger)

	// Resolve the model name so it's recorded in the analysis record.
	model := providerCfg.Model
	if model == "" {
		switch providerCfg.Provider {
		case "google", "gemini":
			model = provider.ManagedModels["google"][1] // default to gemini-3-flash-preview
		default:
			model = "gpt-4o"
		}
	}
	analysis, err := orchestrator.RunAnalysis(ctx, runID, projectID, providerCfg.BackendMode, providerCfg.Provider, model)
	analysisDuration := time.Since(analysisStart).Seconds()

	if err != nil {
		h.logger.Error().Err(err).
			Str("run_id", payload.RunID).
			Msg("analysis failed")
		if h.metrics != nil {
			h.metrics.AIAnalysesTotal.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("status", "failed"),
					attribute.String("provider", providerCfg.Provider),
					attribute.String("mode", providerCfg.BackendMode),
				))
			h.metrics.AIAnalysisDuration.Record(ctx, analysisDuration,
				metric.WithAttributes(attribute.String("provider", providerCfg.Provider)))
			h.metrics.AIProviderErrorsTotal.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("provider", providerCfg.Provider),
					attribute.String("error_type", "analysis"),
				))
		}
		return nil
	}

	// Record metrics for successful analysis.
	if h.metrics != nil {
		h.metrics.AIAnalysesTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("status", "success"),
				attribute.String("provider", providerCfg.Provider),
				attribute.String("mode", providerCfg.BackendMode),
			))
		h.metrics.AIAnalysisDuration.Record(ctx, analysisDuration,
			metric.WithAttributes(attribute.String("provider", providerCfg.Provider)))
	}

	// Set dedup key on the analysis record (best-effort).
	if analysis != nil {
		analysis.DedupKey = &dedupKey
		_ = h.aiRepo.UpdateAnalysisDedupKey(ctx, analysis.ID, dedupKey)
	}

	// Enqueue publish job to post results to GitHub.
	if analysis != nil {
		h.enqueuePublishJob(analysis, payload)
		// Only enqueue suggestions when the project's AI mode opts in.
		if payload.AIConfig != nil && payload.AIConfig.Mode == "summarize_and_suggest" {
			h.enqueueSuggestJob(analysis, payload)
		}
	}

	h.logger.Info().
		Str("run_id", payload.RunID).
		Str("analysis_id", analysis.ID.String()).
		Msg("analysis completed")

	return nil
}

// enqueuePublishJob enqueues an ai_publish:github job to post analysis results to GitHub.
func (h *AIAnalysisHandler) enqueuePublishJob(analysis *models.AIAnalysis, payload aiAnalysisPayload) {
	if h.jobEnqueuer == nil {
		return
	}
	pubPayload := struct {
		AnalysisID string `json:"analysis_id"`
		RunID      string `json:"run_id"`
		ProjectID  string `json:"project_id"`
	}{
		AnalysisID: analysis.ID.String(),
		RunID:      payload.RunID,
		ProjectID:  payload.ProjectID,
	}
	data, err := json.Marshal(pubPayload)
	if err != nil {
		h.logger.Warn().Err(err).Msg("failed to marshal publish payload")
		return
	}
	if err := h.jobEnqueuer.EnqueueRaw("DefaultQueue", "ai_publish:github", data); err != nil {
		h.logger.Warn().Err(err).Msg("failed to enqueue publish job")
	}
}

// enqueueSuggestJob enqueues an ai_suggest:run job to generate inline code suggestions.
func (h *AIAnalysisHandler) enqueueSuggestJob(analysis *models.AIAnalysis, payload aiAnalysisPayload) {
	if h.jobEnqueuer == nil {
		return
	}
	sugPayload := struct {
		AnalysisID string           `json:"analysis_id"`
		RunID      string           `json:"run_id"`
		ProjectID  string           `json:"project_id"`
		AIConfig   *aiProjectConfig `json:"ai_config,omitempty"`
	}{
		AnalysisID: analysis.ID.String(),
		RunID:      payload.RunID,
		ProjectID:  payload.ProjectID,
		AIConfig:   payload.AIConfig,
	}
	data, err := json.Marshal(sugPayload)
	if err != nil {
		h.logger.Warn().Err(err).Msg("failed to marshal suggest payload")
		return
	}
	if err := h.jobEnqueuer.EnqueueRaw("DefaultQueue", "ai_suggest:run", data); err != nil {
		h.logger.Warn().Err(err).Msg("failed to enqueue suggest job")
	}
}

// buildDedupKey constructs a dedup key from analysis parameters.
func buildDedupKey(projectID uuid.UUID, branch, commit, workflowName, targets string) string {
	targetsHash := ""
	if targets != "" {
		// Sort targets for consistent hashing.
		parts := strings.Split(targets, ",")
		sort.Strings(parts)
		sorted := strings.Join(parts, ",")
		hash := sha256.Sum256([]byte(sorted))
		targetsHash = fmt.Sprintf("%x", hash[:4]) // first 8 hex chars
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s", projectID.String(), branch, commit, workflowName, targetsHash)
}
