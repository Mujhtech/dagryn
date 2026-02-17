package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/ai/aitypes"
	"github.com/mujhtech/dagryn/internal/ai/evidence"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/rs/zerolog"
)

// AIDataStore persists analysis records.
type AIDataStore interface {
	CreateAnalysis(ctx context.Context, a *models.AIAnalysis) error
	UpdateAnalysisStatus(ctx context.Context, id uuid.UUID, status models.AIAnalysisStatus, errorMessage *string) error
	UpdateAnalysisResults(ctx context.Context, a *models.AIAnalysis) error
}

// Orchestrator coordinates evidence building, provider invocation, and persistence.
type Orchestrator struct {
	evidenceBuilder *evidence.EvidenceBuilder
	provider        aitypes.Provider
	policy          *PolicyChecker
	aiRepo          AIDataStore
	logger          zerolog.Logger
}

// NewOrchestrator creates a new analysis orchestrator.
func NewOrchestrator(
	evidenceBuilder *evidence.EvidenceBuilder,
	provider aitypes.Provider,
	policy *PolicyChecker,
	aiRepo AIDataStore,
	logger zerolog.Logger,
) *Orchestrator {
	return &Orchestrator{
		evidenceBuilder: evidenceBuilder,
		provider:        provider,
		policy:          policy,
		aiRepo:          aiRepo,
		logger:          logger.With().Str("component", "orchestrator").Logger(),
	}
}

// RunAnalysis executes the full analysis pipeline for a run.
func (o *Orchestrator) RunAnalysis(ctx context.Context, runID, projectID uuid.UUID, providerMode, providerName, model string) (*models.AIAnalysis, error) {
	promptVersion := aitypes.PromptVersion

	// Create initial analysis record.
	analysis := &models.AIAnalysis{
		RunID:         runID,
		ProjectID:     projectID,
		Status:        models.AIAnalysisStatusInProgress,
		ProviderMode:  strPtr(providerMode),
		Provider:      strPtr(providerName),
		Model:         strPtr(model),
		PromptVersion: &promptVersion,
	}
	if err := o.aiRepo.CreateAnalysis(ctx, analysis); err != nil {
		return nil, fmt.Errorf("orchestrator: create analysis: %w", err)
	}

	// Run the pipeline, updating status on failure.
	if err := o.runPipeline(ctx, analysis); err != nil {
		errMsg := err.Error()
		_ = o.aiRepo.UpdateAnalysisStatus(ctx, analysis.ID, models.AIAnalysisStatusFailed, &errMsg)
		analysis.Status = models.AIAnalysisStatusFailed
		analysis.ErrorMessage = &errMsg
		return analysis, err
	}

	return analysis, nil
}

func (o *Orchestrator) runPipeline(ctx context.Context, analysis *models.AIAnalysis) error {
	// Build evidence.
	input, err := o.evidenceBuilder.Build(ctx, analysis.RunID)
	if err != nil {
		return fmt.Errorf("build evidence: %w", err)
	}

	// Validate pre-analysis.
	if err := o.policy.ValidatePreAnalysis(input); err != nil {
		return err
	}

	// Compute prompt hash.
	promptData, _ := json.Marshal(input)
	promptHash := fmt.Sprintf("%x", sha256.Sum256(promptData))

	// Invoke provider.
	output, err := o.provider.AnalyzeFailure(ctx, *input)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	// Post-analysis policy checks.
	if err := o.policy.CheckConfidence(output); err != nil {
		return err
	}

	// Filter file paths per policy.
	output.LikelyFiles = o.policy.CheckFilePaths(output.LikelyFiles)

	// Compute response hash.
	responseData, _ := json.Marshal(output)
	responseHash := fmt.Sprintf("%x", sha256.Sum256(responseData))

	// Marshal full output for storage (includes evidence, likely_files, recommended_actions).
	evidenceJSON, _ := json.Marshal(output)

	// Update analysis record with results.
	analysis.Summary = &output.Summary
	analysis.RootCause = &output.RootCause
	analysis.Confidence = &output.Confidence
	analysis.EvidenceJSON = evidenceJSON
	analysis.PromptHash = &promptHash
	analysis.ResponseHash = &responseHash
	analysis.Status = models.AIAnalysisStatusSuccess

	if err := o.aiRepo.UpdateAnalysisResults(ctx, analysis); err != nil {
		return fmt.Errorf("update analysis: %w", err)
	}

	o.logger.Info().
		Str("run_id", analysis.RunID.String()).
		Float64("confidence", output.Confidence).
		Msg("analysis completed successfully")

	return nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
