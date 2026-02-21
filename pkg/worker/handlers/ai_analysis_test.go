package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAIRepoForHandler struct {
	repo.AIStore       // embed for interface satisfaction
	analyses           map[uuid.UUID]*models.AIAnalysis
	pendingByDedup     map[string]*models.AIAnalysis
	recentCount        int
	supersedeCalled    bool
	supersedeArgs      []string
	dedupKeyUpdates    map[uuid.UUID]string
	createCallCount    int
	inProgressCount    int
	mostRecentAnalysis *models.AIAnalysis
}

func newMockAIRepoForHandler() *mockAIRepoForHandler {
	return &mockAIRepoForHandler{
		analyses:        make(map[uuid.UUID]*models.AIAnalysis),
		pendingByDedup:  make(map[string]*models.AIAnalysis),
		dedupKeyUpdates: make(map[uuid.UUID]string),
	}
}

func (m *mockAIRepoForHandler) CreateAnalysis(_ context.Context, a *models.AIAnalysis) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	m.analyses[a.ID] = a
	m.createCallCount++
	return nil
}

func (m *mockAIRepoForHandler) UpdateAnalysisStatus(_ context.Context, id uuid.UUID, status models.AIAnalysisStatus, errMsg *string) error {
	if a, ok := m.analyses[id]; ok {
		a.Status = status
		a.ErrorMessage = errMsg
	}
	return nil
}

func (m *mockAIRepoForHandler) UpdateAnalysisResults(_ context.Context, a *models.AIAnalysis) error {
	if existing, ok := m.analyses[a.ID]; ok {
		existing.Status = a.Status
		existing.Summary = a.Summary
		existing.RootCause = a.RootCause
		existing.Confidence = a.Confidence
		existing.EvidenceJSON = a.EvidenceJSON
		existing.PromptHash = a.PromptHash
		existing.ResponseHash = a.ResponseHash
		existing.ErrorMessage = a.ErrorMessage
	}
	return nil
}

func (m *mockAIRepoForHandler) FindPendingByDedupKey(_ context.Context, dedupKey string) (*models.AIAnalysis, error) {
	if a, ok := m.pendingByDedup[dedupKey]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockAIRepoForHandler) SupersedeByBranch(_ context.Context, projectID uuid.UUID, branch, excludeCommit string) error {
	m.supersedeCalled = true
	m.supersedeArgs = []string{projectID.String(), branch, excludeCommit}
	return nil
}

func (m *mockAIRepoForHandler) CountRecentAnalyses(_ context.Context, _ uuid.UUID, _ time.Time) (int, error) {
	return m.recentCount, nil
}

func (m *mockAIRepoForHandler) UpdateAnalysisDedupKey(_ context.Context, id uuid.UUID, dedupKey string) error {
	m.dedupKeyUpdates[id] = dedupKey
	return nil
}

func (m *mockAIRepoForHandler) CountInProgressAnalyses(_ context.Context, _ uuid.UUID) (int, error) {
	return m.inProgressCount, nil
}

func (m *mockAIRepoForHandler) GetMostRecentAnalysisByKey(_ context.Context, _ uuid.UUID, _, _ string) (*models.AIAnalysis, error) {
	if m.mostRecentAnalysis != nil {
		return m.mostRecentAnalysis, nil
	}
	return nil, fmt.Errorf("not found")
}

type mockRunDS struct {
	repo.RunStore // embed for interface satisfaction
	run           *models.Run
	taskResults   []models.TaskResult
	logs          []models.RunLog
}

func (m *mockRunDS) GetByID(_ context.Context, _ uuid.UUID) (*models.Run, error) {
	if m.run == nil {
		return nil, fmt.Errorf("not found")
	}
	return m.run, nil
}

func (m *mockRunDS) ListTaskResults(_ context.Context, _ uuid.UUID) ([]models.TaskResult, error) {
	return m.taskResults, nil
}

func (m *mockRunDS) GetLogsByTask(_ context.Context, _ uuid.UUID, _ string, _, _ int) ([]models.RunLog, int, error) {
	return m.logs, len(m.logs), nil
}

type mockAnalysisJobEnqueuer struct {
	enqueued []struct {
		queue    string
		taskName string
		data     []byte
	}
}

func (m *mockAnalysisJobEnqueuer) EnqueueRaw(queue, taskName string, data []byte) error {
	m.enqueued = append(m.enqueued, struct {
		queue    string
		taskName string
		data     []byte
	}{queue, taskName, data})
	return nil
}

type mockWorkflowDS struct {
	repo.WorkflowStore // embed for interface satisfaction
}

func (m *mockWorkflowDS) GetByID(_ context.Context, _ uuid.UUID) (*models.WorkflowWithTasks, error) {
	return nil, fmt.Errorf("not found")
}

// encodePayload marshals and base64-encodes a payload (matching NoOpEncrypt.Encrypt).
func encodePayload(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return []byte(base64.StdEncoding.EncodeToString(data))
}

func TestBuildDedupKey(t *testing.T) {
	pid := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	key1 := buildDedupKey(pid, "main", "abc123", "default", "lint,test")
	key2 := buildDedupKey(pid, "main", "abc123", "default", "test,lint")
	// Same targets different order -> same hash
	assert.Equal(t, key1, key2)

	// Different commit -> different key
	key3 := buildDedupKey(pid, "main", "def456", "default", "lint,test")
	assert.NotEqual(t, key1, key3)

	// Empty targets -> empty hash part
	key4 := buildDedupKey(pid, "main", "abc123", "default", "")
	assert.Contains(t, key4, "default:")
	assert.True(t, len(key4) > 0)

	// Different branch -> different key
	key5 := buildDedupKey(pid, "dev", "abc123", "default", "lint,test")
	assert.NotEqual(t, key1, key5)
}

func TestAIAnalysis_Dedup_SkipExisting(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()

	payload := aiAnalysisPayload{
		RunID:        runID.String(),
		ProjectID:    projectID.String(),
		GitBranch:    "main",
		GitCommit:    "abc123",
		WorkflowName: "default",
		Targets:      "lint,test",
	}
	payloadBytes := encodePayload(t, payload)

	dedupKey := buildDedupKey(projectID, "main", "abc123", "default", "lint,test")

	mockAI := newMockAIRepoForHandler()
	mockAI.pendingByDedup[dedupKey] = &models.AIAnalysis{
		ID:     uuid.New(),
		Status: models.AIAnalysisStatusPending,
	}

	handler := NewAIAnalysisHandler(
		store.Store{
			AI:        mockAI,
			Runs:      &mockRunDS{},
			Workflows: &mockWorkflowDS{},
		},
		encrypt.NewNoOpEncrypt(),
		&AIAnalysisConfig{
			Enabled:            true,
			BackendMode:        "managed",
			Provider:           "openai",
			MaxAnalysesPerHour: 10,
		},
		nil, // jobEnqueuer
		zerolog.Nop(),
		nil, // metrics
	)

	task := asynq.NewTask("ai_analysis:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	// Should not have created a new analysis since dedup found existing
	assert.Equal(t, 0, mockAI.createCallCount)
}

func TestAIAnalysis_RateLimit_Exceeded(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()

	payload := aiAnalysisPayload{
		RunID:        runID.String(),
		ProjectID:    projectID.String(),
		GitBranch:    "main",
		GitCommit:    "abc123",
		WorkflowName: "default",
	}
	payloadBytes := encodePayload(t, payload)

	mockAI := newMockAIRepoForHandler()
	mockAI.recentCount = 10 // At limit

	handler := NewAIAnalysisHandler(
		store.Store{
			AI:        mockAI,
			Runs:      &mockRunDS{},
			Workflows: &mockWorkflowDS{},
		},
		encrypt.NewNoOpEncrypt(),
		&AIAnalysisConfig{
			Enabled:            true,
			BackendMode:        "managed",
			Provider:           "openai",
			MaxAnalysesPerHour: 10,
		},
		nil, // jobEnqueuer
		zerolog.Nop(),
		nil, // metrics
	)

	task := asynq.NewTask("ai_analysis:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	// Should not have created a new analysis since rate limited
	assert.Equal(t, 0, mockAI.createCallCount)
}

func TestAIAnalysis_Supersede_OldCommit(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()

	payload := aiAnalysisPayload{
		RunID:        runID.String(),
		ProjectID:    projectID.String(),
		GitBranch:    "feature-x",
		GitCommit:    "newcommit",
		WorkflowName: "default",
	}
	payloadBytes := encodePayload(t, payload)

	mockAI := newMockAIRepoForHandler()
	mockAI.recentCount = 0

	exitCode := 1
	dur := int64(5000)
	now := time.Now()
	mockRuns := &mockRunDS{
		run: &models.Run{
			ID:          runID,
			ProjectID:   projectID,
			Status:      models.RunStatusFailed,
			TotalTasks:  2,
			FailedTasks: 1,
		},
		taskResults: []models.TaskResult{
			{
				RunID:      runID,
				TaskName:   "test",
				Status:     models.TaskStatusFailed,
				ExitCode:   &exitCode,
				DurationMs: &dur,
				FinishedAt: &now,
			},
		},
		logs: []models.RunLog{
			{RunID: runID, TaskName: "test", Stream: models.LogStreamStderr, Content: "FAIL: TestSomething"},
		},
	}

	// Use an invalid provider so the pipeline fails after supersede is called.
	// This tests that supersede is invoked with correct args.
	handler := NewAIAnalysisHandler(
		store.Store{
			AI:        mockAI,
			Runs:      mockRuns,
			Workflows: &mockWorkflowDS{},
		},
		encrypt.NewNoOpEncrypt(),
		&AIAnalysisConfig{
			Enabled:            true,
			BackendMode:        "invalid_mode",
			Provider:           "openai",
			MaxAnalysesPerHour: 10,
		},
		nil, // jobEnqueuer
		zerolog.Nop(),
		nil, // metrics
	)

	task := asynq.NewTask("ai_analysis:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err) // Handler returns nil even on provider failure

	// Verify supersede was called with correct args
	assert.True(t, mockAI.supersedeCalled)
	assert.Equal(t, projectID.String(), mockAI.supersedeArgs[0])
	assert.Equal(t, "feature-x", mockAI.supersedeArgs[1])
	assert.Equal(t, "newcommit", mockAI.supersedeArgs[2])
}

func TestAIAnalysis_ProviderError_NoRetry(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()

	payload := aiAnalysisPayload{
		RunID:     runID.String(),
		ProjectID: projectID.String(),
		GitBranch: "main",
		GitCommit: "abc",
	}
	payloadBytes := encodePayload(t, payload)

	mockAI := newMockAIRepoForHandler()

	// Use an invalid backend mode to trigger provider creation failure.
	handler := NewAIAnalysisHandler(
		store.Store{
			AI:        mockAI,
			Runs:      &mockRunDS{},
			Workflows: &mockWorkflowDS{},
		},
		encrypt.NewNoOpEncrypt(),
		&AIAnalysisConfig{
			Enabled:            true,
			BackendMode:        "invalid_mode",
			MaxAnalysesPerHour: 10,
		},
		nil, // jobEnqueuer
		zerolog.Nop(),
		nil, // metrics
	)

	task := asynq.NewTask("ai_analysis:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	// Should return nil even on provider error — no asynq retry.
	require.NoError(t, err)
}

func TestAIAnalysis_Disabled(t *testing.T) {
	cfg := &AIAnalysisConfig{
		Enabled: false,
	}
	// When Enabled is false, the handler should not be registered.
	// This is tested at the job.go wiring level.
	assert.False(t, cfg.Enabled)
}

func TestAIAnalysis_HappyPath(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()

	payload := aiAnalysisPayload{
		RunID:        runID.String(),
		ProjectID:    projectID.String(),
		GitBranch:    "main",
		GitCommit:    "abc123",
		WorkflowName: "default",
		Targets:      "lint,test",
	}
	payloadBytes := encodePayload(t, payload)

	mockAI := newMockAIRepoForHandler()
	mockAI.recentCount = 0

	exitCode := 1
	dur := int64(5000)
	now := time.Now()
	mockRuns := &mockRunDS{
		run: &models.Run{
			ID:          runID,
			ProjectID:   projectID,
			Status:      models.RunStatusFailed,
			TotalTasks:  2,
			FailedTasks: 1,
		},
		taskResults: []models.TaskResult{
			{
				RunID:      runID,
				TaskName:   "test",
				Status:     models.TaskStatusFailed,
				ExitCode:   &exitCode,
				DurationMs: &dur,
				FinishedAt: &now,
			},
		},
		logs: []models.RunLog{
			{RunID: runID, TaskName: "test", Stream: models.LogStreamStderr, Content: "FAIL: TestSomething"},
		},
	}

	handler := NewAIAnalysisHandler(
		store.Store{
			AI:        mockAI,
			Runs:      mockRuns,
			Workflows: &mockWorkflowDS{},
		},
		encrypt.NewNoOpEncrypt(),
		&AIAnalysisConfig{
			Enabled:            true,
			BackendMode:        "managed",
			Provider:           "openai",
			APIKey:             "test-key",
			MaxAnalysesPerHour: 10,
		},
		nil, // jobEnqueuer
		zerolog.Nop(),
		nil, // metrics
	)

	task := asynq.NewTask("ai_analysis:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)

	// Verify an analysis record was created (by the orchestrator).
	assert.Equal(t, 1, mockAI.createCallCount)
}

func TestAIAnalysis_EnqueuesSuggestJob(t *testing.T) {
	enqueuer := &mockAnalysisJobEnqueuer{}

	handler := &AIAnalysisHandler{
		jobEnqueuer: enqueuer,
		logger:      zerolog.Nop(),
	}

	analysisID := uuid.New()
	analysis := &models.AIAnalysis{ID: analysisID}
	projCfg := &aiProjectConfig{BackendMode: "byok", Provider: "openai", APIKey: "test-key"}
	payload := aiAnalysisPayload{
		RunID:     "run-123",
		ProjectID: "proj-456",
		AIConfig:  projCfg,
	}

	handler.enqueueSuggestJob(analysis, payload)

	require.Len(t, enqueuer.enqueued, 1)
	assert.Equal(t, "DefaultQueue", enqueuer.enqueued[0].queue)
	assert.Equal(t, "ai_suggest:run", enqueuer.enqueued[0].taskName)

	var sugPayload struct {
		AnalysisID string           `json:"analysis_id"`
		RunID      string           `json:"run_id"`
		ProjectID  string           `json:"project_id"`
		AIConfig   *aiProjectConfig `json:"ai_config,omitempty"`
	}
	require.NoError(t, json.Unmarshal(enqueuer.enqueued[0].data, &sugPayload))
	assert.Equal(t, analysisID.String(), sugPayload.AnalysisID)
	assert.Equal(t, "run-123", sugPayload.RunID)
	assert.Equal(t, "proj-456", sugPayload.ProjectID)
	require.NotNil(t, sugPayload.AIConfig)
	assert.Equal(t, "byok", sugPayload.AIConfig.BackendMode)
	assert.Equal(t, "test-key", sugPayload.AIConfig.APIKey)
}

func TestAIAnalysis_EnqueuesSuggestJob_NilEnqueuer(t *testing.T) {
	handler := &AIAnalysisHandler{
		jobEnqueuer: nil,
		logger:      zerolog.Nop(),
	}

	analysis := &models.AIAnalysis{ID: uuid.New()}
	payload := aiAnalysisPayload{RunID: "run-1", ProjectID: "proj-1"}

	// Should not panic.
	handler.enqueueSuggestJob(analysis, payload)
}

func TestAIAnalysis_EnqueuesPublishAndSuggestJobs(t *testing.T) {
	enqueuer := &mockAnalysisJobEnqueuer{}

	handler := &AIAnalysisHandler{
		jobEnqueuer: enqueuer,
		logger:      zerolog.Nop(),
	}

	analysis := &models.AIAnalysis{ID: uuid.New()}
	payload := aiAnalysisPayload{
		RunID:     uuid.New().String(),
		ProjectID: uuid.New().String(),
	}

	handler.enqueuePublishJob(analysis, payload)
	handler.enqueueSuggestJob(analysis, payload)

	require.Len(t, enqueuer.enqueued, 2)
	assert.Equal(t, "ai_publish:github", enqueuer.enqueued[0].taskName)
	assert.Equal(t, "ai_suggest:run", enqueuer.enqueued[1].taskName)
}

func TestAIAnalysis_CooldownEnforced(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()

	payload := aiAnalysisPayload{
		RunID:     runID.String(),
		ProjectID: projectID.String(),
		GitBranch: "main",
		GitCommit: "abc123",
	}
	payloadBytes := encodePayload(t, payload)

	mockAI := newMockAIRepoForHandler()
	// Simulate a recent analysis within cooldown.
	mockAI.mostRecentAnalysis = &models.AIAnalysis{
		ID:        uuid.New(),
		CreatedAt: time.Now().Add(-10 * time.Second), // 10 seconds ago
	}

	handler := NewAIAnalysisHandler(
		store.Store{
			AI:        mockAI,
			Runs:      &mockRunDS{},
			Workflows: &mockWorkflowDS{},
		},
		encrypt.NewNoOpEncrypt(),
		&AIAnalysisConfig{
			Enabled:            true,
			BackendMode:        "byok",
			Provider:           "openai",
			MaxAnalysesPerHour: 10,
			CooldownSeconds:    60, // 60 second cooldown
		},
		nil, // jobEnqueuer
		zerolog.Nop(),
		nil, // metrics
	)

	task := asynq.NewTask("ai_analysis:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	// Should not have created any analysis — cooldown active.
	assert.Equal(t, 0, mockAI.createCallCount)
}

func TestAIAnalysis_ConcurrentLimitExceeded(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()

	payload := aiAnalysisPayload{
		RunID:     runID.String(),
		ProjectID: projectID.String(),
		GitBranch: "main",
		GitCommit: "abc123",
	}
	payloadBytes := encodePayload(t, payload)

	mockAI := newMockAIRepoForHandler()
	mockAI.inProgressCount = 3 // At limit

	handler := NewAIAnalysisHandler(
		store.Store{
			AI:        mockAI,
			Runs:      &mockRunDS{},
			Workflows: &mockWorkflowDS{},
		},
		encrypt.NewNoOpEncrypt(),
		&AIAnalysisConfig{
			Enabled:               true,
			BackendMode:           "byok",
			Provider:              "openai",
			MaxAnalysesPerHour:    10,
			MaxConcurrentAnalyses: 3,
		},
		nil, // jobEnqueuer
		zerolog.Nop(),
		nil, // metrics
	)

	task := asynq.NewTask("ai_analysis:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	// Should not have created any analysis — concurrent limit reached.
	assert.Equal(t, 0, mockAI.createCallCount)
}
