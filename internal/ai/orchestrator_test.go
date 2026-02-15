package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/ai/aitypes"
	"github.com/mujhtech/dagryn/internal/ai/evidence"
	"github.com/mujhtech/dagryn/internal/config"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock data sources ---

type mockRunDS struct {
	run    *models.Run
	tasks  []models.TaskResult
	logs   map[string][]models.RunLog
	runErr error
}

func (m *mockRunDS) GetByID(_ context.Context, _ uuid.UUID) (*models.Run, error) {
	return m.run, m.runErr
}
func (m *mockRunDS) ListTaskResults(_ context.Context, _ uuid.UUID) ([]models.TaskResult, error) {
	return m.tasks, nil
}
func (m *mockRunDS) GetLogsByTask(_ context.Context, _ uuid.UUID, taskName string, _, _ int) ([]models.RunLog, int, error) {
	logs := m.logs[taskName]
	return logs, len(logs), nil
}

type mockWorkflowDS struct {
	err error
}

func (m *mockWorkflowDS) GetByID(_ context.Context, _ uuid.UUID) (*models.WorkflowWithTasks, error) {
	return nil, m.err
}

// --- Mock provider ---

type mockProvider struct {
	output *aitypes.AnalysisOutput
	err    error
}

func (m *mockProvider) AnalyzeFailure(_ context.Context, _ aitypes.AnalysisInput) (*aitypes.AnalysisOutput, error) {
	return m.output, m.err
}

// --- Mock AI data store ---

type mockAIDataStore struct {
	created  *models.AIAnalysis
	statuses []models.AIAnalysisStatus
	errMsg   *string
}

func (m *mockAIDataStore) CreateAnalysis(_ context.Context, a *models.AIAnalysis) error {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	m.created = a
	return nil
}

func (m *mockAIDataStore) UpdateAnalysisStatus(_ context.Context, _ uuid.UUID, status models.AIAnalysisStatus, errMsg *string) error {
	m.statuses = append(m.statuses, status)
	m.errMsg = errMsg
	return nil
}

func (m *mockAIDataStore) UpdateAnalysisResults(_ context.Context, a *models.AIAnalysis) error {
	m.statuses = append(m.statuses, a.Status)
	return nil
}

// --- Test helpers ---

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }
func ptrInt64(i int64) *int64 { return &i }

func setupOrchestrator(runDS *mockRunDS, prov *mockProvider, guardrails config.AIGuardrailConfig) (*Orchestrator, *mockAIDataStore) {
	logger := zerolog.Nop()
	wfDS := &mockWorkflowDS{err: errors.New("no workflow")}
	eb := evidence.NewEvidenceBuilder(runDS, wfDS, logger)
	policy := NewPolicyChecker(guardrails, config.AIRateLimitConfig{}, logger)
	aiRepo := &mockAIDataStore{}
	orch := NewOrchestrator(eb, prov, policy, aiRepo, logger)
	return orch, aiRepo
}

func defaultRunDS() *mockRunDS {
	runID := uuid.New()
	return &mockRunDS{
		run: &models.Run{
			ID:           runID,
			ProjectID:    uuid.New(),
			Status:       models.RunStatusFailed,
			TotalTasks:   2,
			FailedTasks:  1,
			WorkflowName: ptrStr("ci"),
			DurationMs:   ptrInt64(5000),
			CreatedAt:    time.Now(),
		},
		tasks: []models.TaskResult{
			{RunID: runID, TaskName: "build", Status: models.TaskStatusSuccess},
			{RunID: runID, TaskName: "test", Status: models.TaskStatusFailed, ExitCode: ptrInt(1), ErrorMessage: ptrStr("exit 1")},
		},
		logs: map[string][]models.RunLog{
			"test": {
				{RunID: runID, TaskName: "test", Stream: models.LogStreamStderr, Content: "FAIL: TestFoo"},
			},
		},
	}
}

func defaultOutput() *aitypes.AnalysisOutput {
	return &aitypes.AnalysisOutput{
		Summary:            "Tests failed due to nil pointer",
		RootCause:          "Missing nil check",
		Confidence:         0.85,
		Evidence:           []aitypes.EvidenceItem{{Task: "test", Reason: "panic"}},
		LikelyFiles:        []string{"handler.go"},
		RecommendedActions: []string{"Add nil check"},
	}
}

// --- Tests ---

func TestOrchestrator_HappyPath(t *testing.T) {
	runDS := defaultRunDS()
	prov := &mockProvider{output: defaultOutput()}
	orch, aiRepo := setupOrchestrator(runDS, prov, config.AIGuardrailConfig{})

	analysis, err := orch.RunAnalysis(context.Background(), runDS.run.ID, runDS.run.ProjectID, "byok", "openai", "gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, models.AIAnalysisStatusSuccess, analysis.Status)
	assert.NotNil(t, analysis.Summary)
	assert.Equal(t, "Tests failed due to nil pointer", *analysis.Summary)
	assert.NotNil(t, analysis.PromptHash)
	assert.NotNil(t, analysis.ResponseHash)
	assert.Contains(t, aiRepo.statuses, models.AIAnalysisStatusSuccess)
}

func TestOrchestrator_EvidenceBuildError(t *testing.T) {
	runDS := &mockRunDS{runErr: errors.New("db error")}
	prov := &mockProvider{output: defaultOutput()}
	orch, aiRepo := setupOrchestrator(runDS, prov, config.AIGuardrailConfig{})

	analysis, err := orch.RunAnalysis(context.Background(), uuid.New(), uuid.New(), "byok", "openai", "gpt-4o")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build evidence")
	assert.Equal(t, models.AIAnalysisStatusFailed, analysis.Status)
	assert.Contains(t, aiRepo.statuses, models.AIAnalysisStatusFailed)
}

func TestOrchestrator_ProviderError(t *testing.T) {
	runDS := defaultRunDS()
	prov := &mockProvider{err: &aitypes.ProviderError{StatusCode: 500, Message: "server error", Retryable: true}}
	orch, aiRepo := setupOrchestrator(runDS, prov, config.AIGuardrailConfig{})

	analysis, err := orch.RunAnalysis(context.Background(), runDS.run.ID, runDS.run.ProjectID, "byok", "openai", "gpt-4o")
	require.Error(t, err)
	assert.Equal(t, models.AIAnalysisStatusFailed, analysis.Status)
	assert.Contains(t, aiRepo.statuses, models.AIAnalysisStatusFailed)
}

func TestOrchestrator_LowConfidence(t *testing.T) {
	runDS := defaultRunDS()
	out := defaultOutput()
	out.Confidence = 0.2
	prov := &mockProvider{output: out}
	orch, aiRepo := setupOrchestrator(runDS, prov, config.AIGuardrailConfig{MinConfidence: 0.5})

	analysis, err := orch.RunAnalysis(context.Background(), runDS.run.ID, runDS.run.ProjectID, "byok", "openai", "gpt-4o")
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrConfidenceTooLow)
	assert.Equal(t, models.AIAnalysisStatusFailed, analysis.Status)
	assert.Contains(t, aiRepo.statuses, models.AIAnalysisStatusFailed)
}

func TestOrchestrator_PolicyViolation_NoFailedTasks(t *testing.T) {
	runDS := defaultRunDS()
	// Override: no failed tasks
	runDS.tasks = []models.TaskResult{
		{RunID: runDS.run.ID, TaskName: "build", Status: models.TaskStatusSuccess},
	}
	prov := &mockProvider{output: defaultOutput()}
	orch, aiRepo := setupOrchestrator(runDS, prov, config.AIGuardrailConfig{})

	analysis, err := orch.RunAnalysis(context.Background(), runDS.run.ID, runDS.run.ProjectID, "byok", "openai", "gpt-4o")
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrPolicyViolation)
	assert.Equal(t, models.AIAnalysisStatusFailed, analysis.Status)
	assert.Contains(t, aiRepo.statuses, models.AIAnalysisStatusFailed)
}

func TestOrchestrator_FilePathFiltering(t *testing.T) {
	runDS := defaultRunDS()
	out := defaultOutput()
	out.LikelyFiles = []string{"handler.go", "secrets.env", "main.go"}
	prov := &mockProvider{output: out}
	orch, _ := setupOrchestrator(runDS, prov, config.AIGuardrailConfig{
		BlockedPaths: []string{"*.env"},
	})

	analysis, err := orch.RunAnalysis(context.Background(), runDS.run.ID, runDS.run.ProjectID, "byok", "openai", "gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, models.AIAnalysisStatusSuccess, analysis.Status)
	// The blocked file should have been filtered from the output by policy
}
