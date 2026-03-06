package worker

import (
	"time"
)

type TaskName string
type QueueName string

const (
	QueueNameDefault  QueueName = "DefaultQueue"
	QueueNamePriority QueueName = "PriorityQueue"
	ScheduleQueueName QueueName = "ScheduleQueue"

	WebhookTaskName                 TaskName = "Webhook"
	StaleRunsTaskName               TaskName = "stale_runs:check"
	ExecuteRunTaskName              TaskName = "execute_run"
	CacheGCTaskName                 TaskName = "cache_gc:run"
	ArtifactCleanupTaskName         TaskName = "artifact_cleanup:daily"
	PluginDownloadRecomputeTaskName TaskName = "plugin_downloads:recompute"
	AIAnalysisTaskName              TaskName = "ai_analysis:run"
	AIPublishTaskName               TaskName = "ai_publish:github"
	AISuggestRunTaskName            TaskName = "ai_suggest:run"
	AISuggestPublishTaskName        TaskName = "ai_suggest:publish"
	AIBlobCleanupTaskName           TaskName = "ai_blob_cleanup:run"
)

type ClientPayload struct {
	Data  []byte        `json:"data"`
	Delay time.Duration `json:"delay"`
}

// AIProjectConfig carries the resolved project-level AI config through job payloads.
type AIProjectConfig struct {
	BackendMode    string `json:"backend_mode"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	APIKey         string `json:"api_key,omitempty"`        // Resolved from env var (byok)
	AgentEndpoint  string `json:"agent_endpoint,omitempty"` // Agent mode
	AgentToken     string `json:"agent_token,omitempty"`    // Agent mode
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	MaxTokens      int    `json:"max_tokens,omitempty"`
	Mode           string `json:"mode,omitempty"` // "summarize" | "summarize_and_suggest"
	// Guardrails
	MinConfidence             float64  `json:"min_confidence,omitempty"`
	MaxSuggestionsPerAnalysis int      `json:"max_suggestions_per_analysis,omitempty"`
	BlockedPaths              []string `json:"blocked_paths,omitempty"`
	AllowedPaths              []string `json:"allowed_paths,omitempty"`
	// Rate limits
	MaxAnalysesPerHour    int `json:"max_analyses_per_hour,omitempty"`
	CooldownSeconds       int `json:"cooldown_seconds,omitempty"`
	MaxConcurrentAnalyses int `json:"max_concurrent_analyses,omitempty"`
}

// AIAnalysisPayload is the payload for the ai_analysis:run job.
type AIAnalysisPayload struct {
	RunID        string           `json:"run_id"`
	ProjectID    string           `json:"project_id"`
	GitBranch    string           `json:"git_branch,omitempty"`
	GitCommit    string           `json:"git_commit,omitempty"`
	WorkflowName string           `json:"workflow_name,omitempty"`
	Targets      string           `json:"targets,omitempty"` // sorted, joined by comma for dedup key
	AIConfig     *AIProjectConfig `json:"ai_config,omitempty"`
}

// AIPublishPayload is the payload for the ai_publish:github job.
type AIPublishPayload struct {
	AnalysisID string `json:"analysis_id"`
	RunID      string `json:"run_id"`
	ProjectID  string `json:"project_id"`
}

// AISuggestRunPayload is the payload for the ai_suggest:run job.
type AISuggestRunPayload struct {
	AnalysisID string           `json:"analysis_id"`
	RunID      string           `json:"run_id"`
	ProjectID  string           `json:"project_id"`
	AIConfig   *AIProjectConfig `json:"ai_config,omitempty"`
}

// AISuggestPublishPayload is the payload for the ai_suggest:publish job.
type AISuggestPublishPayload struct {
	AnalysisID string `json:"analysis_id"`
	RunID      string `json:"run_id"`
	ProjectID  string `json:"project_id"`
}

// ExecuteRunPayload is the payload for the execute_run job (project ID, run ID, targets, git ref).
type ExecuteRunPayload struct {
	ProjectID   string   `json:"project_id"`
	RunID       string   `json:"run_id"`
	Targets     []string `json:"targets"`
	GitBranch   string   `json:"git_branch,omitempty"`
	GitCommit   string   `json:"git_commit,omitempty"`
	RepoURL     string   `json:"repo_url,omitempty"`
	EventType   string   `json:"event_type,omitempty"`   // "push", "pull_request"
	EventAction string   `json:"event_action,omitempty"` // "opened", "synchronize", etc.
}
