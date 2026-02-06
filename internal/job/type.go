package job

import (
	"time"
)

type TaskName string
type QueueName string

const (
	QueueNameDefault  QueueName = "DefaultQueue"
	ScheduleQueueName QueueName = "ScheduleQueue"

	WebhookTaskName    TaskName = "Webhook"
	StaleRunsTaskName  TaskName = "stale_runs:check"
	ExecuteRunTaskName TaskName = "execute_run"
)

type ClientPayload struct {
	Data  []byte        `json:"data"`
	Delay time.Duration `json:"delay"`
}

// ExecuteRunPayload is the payload for the execute_run job (project ID, run ID, targets, git ref).
type ExecuteRunPayload struct {
	ProjectID string   `json:"project_id"`
	RunID     string   `json:"run_id"`
	Targets   []string `json:"targets"`
	GitBranch string   `json:"git_branch,omitempty"`
	GitCommit string   `json:"git_commit,omitempty"`
	RepoURL   string   `json:"repo_url,omitempty"`
}
