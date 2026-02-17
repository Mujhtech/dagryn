package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/rs/zerolog"
)

// RunDataSource provides run, task result, and log data.
type RunDataSource interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Run, error)
	ListTaskResults(ctx context.Context, runID uuid.UUID) ([]models.TaskResult, error)
	GetLogsByTask(ctx context.Context, runID uuid.UUID, taskName string, limit, offset int) ([]models.RunLog, int, error)
}

// WorkflowDataSource provides workflow task graph data.
type WorkflowDataSource interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.WorkflowWithTasks, error)
}

// EvidenceBuilder extracts and assembles analysis evidence from run data.
type EvidenceBuilder struct {
	runs      RunDataSource
	workflows WorkflowDataSource
	logger    zerolog.Logger
}

// NewEvidenceBuilder creates a new EvidenceBuilder.
func NewEvidenceBuilder(runs RunDataSource, workflows WorkflowDataSource, logger zerolog.Logger) *EvidenceBuilder {
	return &EvidenceBuilder{
		runs:      runs,
		workflows: workflows,
		logger:    logger.With().Str("component", "evidence_builder").Logger(),
	}
}

// Build assembles an AnalysisInput for the given run.
func (b *EvidenceBuilder) Build(ctx context.Context, runID uuid.UUID) (*aitypes.AnalysisInput, error) {
	run, err := b.runs.GetByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("evidence: get run: %w", err)
	}

	taskResults, err := b.runs.ListTaskResults(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("evidence: list task results: %w", err)
	}

	// Build task graph from workflow if available, otherwise from task results.
	var taskGraph []aitypes.TaskNode
	if run.WorkflowID != nil {
		wf, err := b.workflows.GetByID(ctx, *run.WorkflowID)
		if err != nil {
			b.logger.Warn().Err(err).Msg("failed to fetch workflow, building graph from task results")
			taskGraph = b.buildTaskGraphFromResults(taskResults)
		} else {
			taskGraph = b.buildTaskGraph(taskResults, wf.Tasks)
		}
	} else {
		taskGraph = b.buildTaskGraphFromResults(taskResults)
	}

	// Collect failed task evidence (capped at MaxFailedTasks).
	var failedTasks []aitypes.FailedTaskEvidence
	for _, tr := range taskResults {
		if tr.Status != models.TaskStatusFailed {
			continue
		}
		if len(failedTasks) >= aitypes.MaxFailedTasks {
			b.logger.Debug().Int("cap", aitypes.MaxFailedTasks).Msg("capping failed task evidence")
			break
		}

		stdout, stderr, err := b.extractLogTails(ctx, runID, tr.TaskName)
		if err != nil {
			b.logger.Warn().Err(err).Str("task", tr.TaskName).Msg("failed to extract logs")
		}

		fte := aitypes.FailedTaskEvidence{
			TaskName:   tr.TaskName,
			StdoutTail: stdout,
			StderrTail: stderr,
		}
		if tr.ExitCode != nil {
			fte.ExitCode = *tr.ExitCode
		}
		if tr.ErrorMessage != nil {
			fte.ErrorMessage = *tr.ErrorMessage
		}
		if tr.DurationMs != nil {
			fte.DurationMs = *tr.DurationMs
		}
		failedTasks = append(failedTasks, fte)
	}

	input := &aitypes.AnalysisInput{
		RunID:           runID.String(),
		ProjectID:       run.ProjectID.String(),
		TaskGraph:       taskGraph,
		FailedTasks:     failedTasks,
		TotalTasks:      run.TotalTasks,
		CompletedTasks:  run.CompletedTasks,
		FailedTaskCount: run.FailedTasks,
		CacheHits:       run.CacheHits,
	}

	if run.WorkflowName != nil {
		input.WorkflowName = *run.WorkflowName
	}
	if run.GitBranch != nil {
		input.GitBranch = *run.GitBranch
	}
	if run.GitCommit != nil {
		input.GitCommit = *run.GitCommit
	}
	if run.CommitMessage != nil {
		input.CommitMessage = *run.CommitMessage
	}
	if run.CommitAuthorName != nil {
		input.CommitAuthor = *run.CommitAuthorName
	}
	if run.PRTitle != nil {
		input.PRTitle = *run.PRTitle
	}
	if run.PRNumber != nil {
		input.PRNumber = *run.PRNumber
	}
	if run.ErrorMessage != nil {
		input.RunErrorMessage = *run.ErrorMessage
	}
	if run.DurationMs != nil {
		input.DurationMs = *run.DurationMs
	}

	// Enforce total evidence size limit.
	b.truncateEvidence(input)

	return input, nil
}

// buildTaskGraph builds the task graph from workflow tasks, using task results for status.
func (b *EvidenceBuilder) buildTaskGraph(taskResults []models.TaskResult, workflowTasks []models.WorkflowTask) []aitypes.TaskNode {
	statusMap := make(map[string]string, len(taskResults))
	for _, tr := range taskResults {
		statusMap[tr.TaskName] = string(tr.Status)
	}

	nodes := make([]aitypes.TaskNode, 0, len(workflowTasks))
	for _, wt := range workflowTasks {
		status := statusMap[wt.Name]
		if status == "" {
			status = "unknown"
		}
		nodes = append(nodes, aitypes.TaskNode{
			Name:    wt.Name,
			Command: wt.Command,
			Needs:   wt.Needs,
			Status:  status,
		})
	}
	return nodes
}

// buildTaskGraphFromResults builds a minimal graph from task results alone.
func (b *EvidenceBuilder) buildTaskGraphFromResults(taskResults []models.TaskResult) []aitypes.TaskNode {
	nodes := make([]aitypes.TaskNode, 0, len(taskResults))
	for _, tr := range taskResults {
		nodes = append(nodes, aitypes.TaskNode{
			Name:   tr.TaskName,
			Status: string(tr.Status),
		})
	}
	return nodes
}

// extractLogTails fetches logs for a task and separates stdout/stderr tails.
func (b *EvidenceBuilder) extractLogTails(ctx context.Context, runID uuid.UUID, taskName string) (stdout, stderr string, err error) {
	logs, _, err := b.runs.GetLogsByTask(ctx, runID, taskName, aitypes.MaxLogTailLines, 0)
	if err != nil {
		return "", "", err
	}

	var stdoutLines, stderrLines []string
	for _, l := range logs {
		content := RedactAll(l.Content)
		switch l.Stream {
		case models.LogStreamStdout:
			stdoutLines = append(stdoutLines, content)
		case models.LogStreamStderr:
			stderrLines = append(stderrLines, content)
		}
	}

	stdout = joinAndTruncate(stdoutLines, aitypes.MaxLogTailBytes)
	stderr = joinAndTruncate(stderrLines, aitypes.MaxLogTailBytes)
	return stdout, stderr, nil
}

// joinAndTruncate joins lines and keeps only the tail within maxBytes.
func joinAndTruncate(lines []string, maxBytes int) string {
	if len(lines) == 0 {
		return ""
	}
	joined := strings.Join(lines, "\n")
	if len(joined) <= maxBytes {
		return joined
	}
	// Keep the tail (most recent output).
	return joined[len(joined)-maxBytes:]
}

// truncateEvidence trims log tails proportionally if total evidence exceeds MaxTotalEvidenceLen.
func (b *EvidenceBuilder) truncateEvidence(input *aitypes.AnalysisInput) {
	data, err := json.Marshal(input)
	if err != nil {
		return
	}
	if len(data) <= aitypes.MaxTotalEvidenceLen {
		return
	}

	// Calculate total log bytes across failed tasks.
	totalLogBytes := 0
	for _, ft := range input.FailedTasks {
		totalLogBytes += len(ft.StdoutTail) + len(ft.StderrTail)
	}
	if totalLogBytes == 0 {
		return
	}

	// Calculate how much we need to trim.
	excess := len(data) - aitypes.MaxTotalEvidenceLen
	ratio := float64(totalLogBytes-excess) / float64(totalLogBytes)
	if ratio < 0.1 {
		ratio = 0.1
	}

	for i := range input.FailedTasks {
		ft := &input.FailedTasks[i]
		if newLen := int(float64(len(ft.StdoutTail)) * ratio); newLen < len(ft.StdoutTail) {
			ft.StdoutTail = ft.StdoutTail[len(ft.StdoutTail)-newLen:]
		}
		if newLen := int(float64(len(ft.StderrTail)) * ratio); newLen < len(ft.StderrTail) {
			ft.StderrTail = ft.StderrTail[len(ft.StderrTail)-newLen:]
		}
	}
}
