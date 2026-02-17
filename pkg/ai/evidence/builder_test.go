package evidence

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunDataSource implements RunDataSource for tests.
type mockRunDataSource struct {
	run         *models.Run
	runErr      error
	taskResults []models.TaskResult
	taskErr     error
	logs        map[string][]models.RunLog
	logErr      error
}

func (m *mockRunDataSource) GetByID(_ context.Context, _ uuid.UUID) (*models.Run, error) {
	return m.run, m.runErr
}

func (m *mockRunDataSource) ListTaskResults(_ context.Context, _ uuid.UUID) ([]models.TaskResult, error) {
	return m.taskResults, m.taskErr
}

func (m *mockRunDataSource) GetLogsByTask(_ context.Context, _ uuid.UUID, taskName string, _, _ int) ([]models.RunLog, int, error) {
	if m.logErr != nil {
		return nil, 0, m.logErr
	}
	logs := m.logs[taskName]
	return logs, len(logs), nil
}

// mockWorkflowDataSource implements WorkflowDataSource for tests.
type mockWorkflowDataSource struct {
	workflow *models.WorkflowWithTasks
	err      error
}

func (m *mockWorkflowDataSource) GetByID(_ context.Context, _ uuid.UUID) (*models.WorkflowWithTasks, error) {
	return m.workflow, m.err
}

func ptrStr(s string) *string        { return &s }
func ptrInt(i int) *int              { return &i }
func ptrInt64(i int64) *int64        { return &i }
func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }

func newTestRun() *models.Run {
	return &models.Run{
		ID:               uuid.New(),
		ProjectID:        uuid.New(),
		WorkflowName:     ptrStr("ci"),
		Status:           models.RunStatusFailed,
		TotalTasks:       3,
		CompletedTasks:   1,
		FailedTasks:      1,
		CacheHits:        0,
		DurationMs:       ptrInt64(5000),
		GitBranch:        ptrStr("main"),
		GitCommit:        ptrStr("abc123"),
		CommitMessage:    ptrStr("fix tests"),
		CommitAuthorName: ptrStr("dev"),
		CreatedAt:        time.Now(),
	}
}

func TestBuild_SingleFailedTask(t *testing.T) {
	run := newTestRun()
	runID := run.ID

	runDS := &mockRunDataSource{
		run: run,
		taskResults: []models.TaskResult{
			{ID: uuid.New(), RunID: runID, TaskName: "lint", Status: models.TaskStatusSuccess},
			{ID: uuid.New(), RunID: runID, TaskName: "test", Status: models.TaskStatusFailed, ExitCode: ptrInt(1), ErrorMessage: ptrStr("tests failed"), DurationMs: ptrInt64(3000)},
		},
		logs: map[string][]models.RunLog{
			"test": {
				{RunID: runID, TaskName: "test", Stream: models.LogStreamStdout, Content: "running tests..."},
				{RunID: runID, TaskName: "test", Stream: models.LogStreamStderr, Content: "FAIL: TestFoo"},
			},
		},
	}
	wfDS := &mockWorkflowDataSource{err: assert.AnError}

	builder := NewEvidenceBuilder(runDS, wfDS, zerolog.Nop())
	input, err := builder.Build(context.Background(), runID)

	require.NoError(t, err)
	assert.Equal(t, runID.String(), input.RunID)
	assert.Equal(t, "ci", input.WorkflowName)
	assert.Equal(t, "main", input.GitBranch)
	assert.Equal(t, 3, input.TotalTasks)
	assert.Len(t, input.FailedTasks, 1)
	assert.Equal(t, "test", input.FailedTasks[0].TaskName)
	assert.Equal(t, 1, input.FailedTasks[0].ExitCode)
	assert.Contains(t, input.FailedTasks[0].StdoutTail, "running tests...")
	assert.Contains(t, input.FailedTasks[0].StderrTail, "FAIL: TestFoo")
}

func TestBuild_MultipleFailedTasks(t *testing.T) {
	run := newTestRun()
	run.FailedTasks = 3

	var results []models.TaskResult
	logs := map[string][]models.RunLog{}
	for i := 0; i < 3; i++ {
		name := "task" + string(rune('a'+i))
		results = append(results, models.TaskResult{
			ID: uuid.New(), RunID: run.ID, TaskName: name, Status: models.TaskStatusFailed, ExitCode: ptrInt(1),
		})
		logs[name] = []models.RunLog{{RunID: run.ID, TaskName: name, Stream: models.LogStreamStderr, Content: "error in " + name}}
	}

	builder := NewEvidenceBuilder(
		&mockRunDataSource{run: run, taskResults: results, logs: logs},
		&mockWorkflowDataSource{err: assert.AnError},
		zerolog.Nop(),
	)
	input, err := builder.Build(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Len(t, input.FailedTasks, 3)
}

func TestBuild_NoFailures(t *testing.T) {
	run := newTestRun()
	run.Status = models.RunStatusSuccess
	run.FailedTasks = 0

	builder := NewEvidenceBuilder(
		&mockRunDataSource{
			run: run,
			taskResults: []models.TaskResult{
				{ID: uuid.New(), RunID: run.ID, TaskName: "build", Status: models.TaskStatusSuccess},
			},
		},
		&mockWorkflowDataSource{err: assert.AnError},
		zerolog.Nop(),
	)
	input, err := builder.Build(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Empty(t, input.FailedTasks)
}

func TestBuild_WithWorkflowGraph(t *testing.T) {
	run := newTestRun()
	wfID := uuid.New()
	run.WorkflowID = ptrUUID(wfID)

	wf := &models.WorkflowWithTasks{
		ProjectWorkflow: models.ProjectWorkflow{ID: wfID},
		Tasks: []models.WorkflowTask{
			{Name: "lint", Command: "golint ./...", Needs: nil},
			{Name: "test", Command: "go test ./...", Needs: []string{"lint"}},
			{Name: "build", Command: "go build", Needs: []string{"test"}},
		},
	}

	builder := NewEvidenceBuilder(
		&mockRunDataSource{
			run: run,
			taskResults: []models.TaskResult{
				{RunID: run.ID, TaskName: "lint", Status: models.TaskStatusSuccess},
				{RunID: run.ID, TaskName: "test", Status: models.TaskStatusFailed, ExitCode: ptrInt(1)},
				{RunID: run.ID, TaskName: "build", Status: models.TaskStatusSkipped},
			},
			logs: map[string][]models.RunLog{
				"test": {{RunID: run.ID, TaskName: "test", Stream: models.LogStreamStderr, Content: "FAIL"}},
			},
		},
		&mockWorkflowDataSource{workflow: wf},
		zerolog.Nop(),
	)

	input, err := builder.Build(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Len(t, input.TaskGraph, 3)
	assert.Equal(t, "lint", input.TaskGraph[0].Name)
	assert.Equal(t, "golint ./...", input.TaskGraph[0].Command)
	assert.Equal(t, []string{"lint"}, input.TaskGraph[1].Needs)
	assert.Equal(t, "failed", input.TaskGraph[1].Status)
}

func TestBuild_RedactionApplied(t *testing.T) {
	run := newTestRun()
	builder := NewEvidenceBuilder(
		&mockRunDataSource{
			run: run,
			taskResults: []models.TaskResult{
				{RunID: run.ID, TaskName: "deploy", Status: models.TaskStatusFailed, ExitCode: ptrInt(1)},
			},
			logs: map[string][]models.RunLog{
				"deploy": {
					{RunID: run.ID, TaskName: "deploy", Stream: models.LogStreamStderr, Content: "api_key=super_secret_value_xyz"},
				},
			},
		},
		&mockWorkflowDataSource{err: assert.AnError},
		zerolog.Nop(),
	)

	input, err := builder.Build(context.Background(), run.ID)
	require.NoError(t, err)
	require.Len(t, input.FailedTasks, 1)
	assert.NotContains(t, input.FailedTasks[0].StderrTail, "super_secret_value_xyz")
	assert.Contains(t, input.FailedTasks[0].StderrTail, "[REDACTED]")
}

func TestBuild_EvidenceSizeCap(t *testing.T) {
	run := newTestRun()

	// Create a large log that exceeds MaxTotalEvidenceLen
	bigLog := strings.Repeat("x", aitypes.MaxTotalEvidenceLen)

	builder := NewEvidenceBuilder(
		&mockRunDataSource{
			run: run,
			taskResults: []models.TaskResult{
				{RunID: run.ID, TaskName: "big", Status: models.TaskStatusFailed, ExitCode: ptrInt(1)},
			},
			logs: map[string][]models.RunLog{
				"big": {{RunID: run.ID, TaskName: "big", Stream: models.LogStreamStdout, Content: bigLog}},
			},
		},
		&mockWorkflowDataSource{err: assert.AnError},
		zerolog.Nop(),
	)

	input, err := builder.Build(context.Background(), run.ID)
	require.NoError(t, err)
	// The total stdout should have been trimmed
	assert.Less(t, len(input.FailedTasks[0].StdoutTail), len(bigLog))
}

func TestBuild_FailedTasksCapped(t *testing.T) {
	run := newTestRun()
	run.FailedTasks = 10

	var results []models.TaskResult
	for i := 0; i < 10; i++ {
		results = append(results, models.TaskResult{
			RunID: run.ID, TaskName: "task" + string(rune('0'+i)), Status: models.TaskStatusFailed, ExitCode: ptrInt(1),
		})
	}

	builder := NewEvidenceBuilder(
		&mockRunDataSource{run: run, taskResults: results, logs: map[string][]models.RunLog{}},
		&mockWorkflowDataSource{err: assert.AnError},
		zerolog.Nop(),
	)

	input, err := builder.Build(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Len(t, input.FailedTasks, aitypes.MaxFailedTasks)
}

func TestBuild_RunNotFound(t *testing.T) {
	builder := NewEvidenceBuilder(
		&mockRunDataSource{runErr: assert.AnError},
		&mockWorkflowDataSource{},
		zerolog.Nop(),
	)
	_, err := builder.Build(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get run")
}

func TestBuild_NilOptionalFields(t *testing.T) {
	run := &models.Run{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Status:    models.RunStatusFailed,
		CreatedAt: time.Now(),
	}

	builder := NewEvidenceBuilder(
		&mockRunDataSource{run: run, taskResults: nil},
		&mockWorkflowDataSource{err: assert.AnError},
		zerolog.Nop(),
	)

	input, err := builder.Build(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Empty(t, input.WorkflowName)
	assert.Empty(t, input.GitBranch)
	assert.Empty(t, input.FailedTasks)
}
