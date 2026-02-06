package models

import (
	"time"

	"github.com/google/uuid"
)

// Run represents a workflow execution.
type Run struct {
	ID                 uuid.UUID     `json:"id" db:"id"`
	ProjectID          uuid.UUID     `json:"project_id" db:"project_id"`
	WorkflowID         *uuid.UUID    `json:"workflow_id,omitempty" db:"workflow_id"`
	WorkflowName       *string       `json:"workflow_name,omitempty" db:"workflow_name"`
	Targets            []string      `json:"targets,omitempty" db:"targets"`
	Status             RunStatus     `json:"status" db:"status"`
	TotalTasks         int           `json:"total_tasks" db:"total_tasks"`
	CompletedTasks     int           `json:"completed_tasks" db:"completed_tasks"`
	FailedTasks        int           `json:"failed_tasks" db:"failed_tasks"`
	CacheHits          int           `json:"cache_hits" db:"cache_hits"`
	DurationMs         *int64        `json:"duration_ms,omitempty" db:"duration_ms"`
	ErrorMessage       *string       `json:"error_message,omitempty" db:"error_message"`
	TriggeredBy        TriggerSource `json:"triggered_by" db:"triggered_by"`
	TriggeredByUserID  *uuid.UUID    `json:"triggered_by_user_id,omitempty" db:"triggered_by_user_id"`
	GitBranch          *string       `json:"git_branch,omitempty" db:"git_branch"`
	GitCommit          *string       `json:"git_commit,omitempty" db:"git_commit"`
	PRTitle            *string       `json:"pr_title,omitempty" db:"pr_title"`             // For PR-triggered runs
	PRNumber           *int          `json:"pr_number,omitempty" db:"pr_number"`           // For PR-triggered runs
	CommitMessage      *string       `json:"commit_message,omitempty" db:"commit_message"` // Commit message
	CommitAuthorName   *string       `json:"commit_author_name,omitempty" db:"commit_author_name"`
	CommitAuthorEmail  *string       `json:"commit_author_email,omitempty" db:"commit_author_email"`
	GitHubPRCommentID  *int64        `json:"github_pr_comment_id,omitempty" db:"github_pr_comment_id"`
	StartedAt          *time.Time    `json:"started_at,omitempty" db:"started_at"`
	FinishedAt         *time.Time    `json:"finished_at,omitempty" db:"finished_at"`
	LastHeartbeatAt    *time.Time    `json:"last_heartbeat_at,omitempty" db:"last_heartbeat_at"`
	ClientDisconnected bool          `json:"client_disconnected" db:"client_disconnected"`
	CreatedAt          time.Time     `json:"created_at" db:"created_at"`
}

// RunStatus represents the status of a run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusSuccess   RunStatus = "success"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// Synthetic task names for infrastructure operations in remote runs.
const (
	SyntheticTaskClone   = "__clone__"
	SyntheticTaskCleanup = "__cleanup__"
)

// IsTerminal returns true if the run has finished.
func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusSuccess, RunStatusFailed, RunStatusCancelled:
		return true
	}
	return false
}

// TriggerSource represents how a run was triggered.
type TriggerSource string

const (
	TriggerSourceCLI       TriggerSource = "cli"
	TriggerSourceAPI       TriggerSource = "api"
	TriggerSourceDashboard TriggerSource = "dashboard"
	TriggerSourceCI        TriggerSource = "ci"
)

// TaskResult represents the result of a single task execution.
type TaskResult struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	RunID        uuid.UUID  `json:"run_id" db:"run_id"`
	TaskName     string     `json:"task_name" db:"task_name"`
	Status       TaskStatus `json:"status" db:"status"`
	DurationMs   *int64     `json:"duration_ms,omitempty" db:"duration_ms"`
	ExitCode     *int       `json:"exit_code,omitempty" db:"exit_code"`
	Output       *string    `json:"output,omitempty" db:"output"`
	ErrorMessage *string    `json:"error_message,omitempty" db:"error_message"`
	CacheHit     bool       `json:"cache_hit" db:"cache_hit"`
	CacheKey     *string    `json:"cache_key,omitempty" db:"cache_key"`
	StartedAt    *time.Time `json:"started_at,omitempty" db:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusSuccess   TaskStatus = "success"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCached    TaskStatus = "cached"
	TaskStatusSkipped   TaskStatus = "skipped"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// IsTerminal returns true if the task has finished.
func (s TaskStatus) IsTerminal() bool {
	switch s {
	case TaskStatusSuccess, TaskStatusFailed, TaskStatusCached, TaskStatusSkipped, TaskStatusCancelled:
		return true
	}
	return false
}

// RunWithProject combines run data with project info.
type RunWithProject struct {
	Run
	ProjectName string  `json:"project_name" db:"project_name"`
	ProjectSlug string  `json:"project_slug" db:"project_slug"`
	TeamName    *string `json:"team_name,omitempty" db:"team_name"`
}

// RunWithTasks combines run data with task results.
type RunWithTasks struct {
	Run
	Tasks []TaskResult `json:"tasks"`
}

// LogStream represents the output stream type.
type LogStream string

const (
	LogStreamStdout LogStream = "stdout"
	LogStreamStderr LogStream = "stderr"
)

// RunLog represents a single log line from a task execution.
type RunLog struct {
	ID        int64     `json:"id" db:"id"`
	RunID     uuid.UUID `json:"run_id" db:"run_id"`
	TaskName  string    `json:"task_name" db:"task_name"`
	Stream    LogStream `json:"stream" db:"stream"`
	LineNum   int       `json:"line_num" db:"line_num"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
