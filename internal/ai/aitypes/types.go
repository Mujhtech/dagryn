package aitypes

import (
	"context"
	"errors"
	"fmt"
)

// Version constants for prompt and contract schema.
const (
	PromptVersion   = "1"
	ContractVersion = "1"
)

// Limits for evidence and output.
const (
	MaxSummaryLength    = 500
	MaxLogTailLines     = 100
	MaxLogTailBytes     = 8192
	MaxTotalEvidenceLen = 32768
	MaxFailedTasks      = 5
)

// AnalysisInput is the evidence bundle sent to an AI provider.
type AnalysisInput struct {
	RunID           string               `json:"run_id"`
	ProjectID       string               `json:"project_id"`
	WorkflowName    string               `json:"workflow_name,omitempty"`
	GitBranch       string               `json:"git_branch,omitempty"`
	GitCommit       string               `json:"git_commit,omitempty"`
	CommitMessage   string               `json:"commit_message,omitempty"`
	CommitAuthor    string               `json:"commit_author,omitempty"`
	PRTitle         string               `json:"pr_title,omitempty"`
	PRNumber        int                  `json:"pr_number,omitempty"`
	TaskGraph       []TaskNode           `json:"task_graph"`
	FailedTasks     []FailedTaskEvidence `json:"failed_tasks"`
	RunErrorMessage string               `json:"run_error_message,omitempty"`
	TotalTasks      int                  `json:"total_tasks"`
	CompletedTasks  int                  `json:"completed_tasks"`
	FailedTaskCount int                  `json:"failed_task_count"`
	CacheHits       int                  `json:"cache_hits"`
	DurationMs      int64                `json:"duration_ms,omitempty"`
}

// TaskNode describes a task in the workflow graph.
type TaskNode struct {
	Name    string   `json:"name"`
	Command string   `json:"command,omitempty"`
	Needs   []string `json:"needs,omitempty"`
	Status  string   `json:"status"`
}

// FailedTaskEvidence holds evidence for a single failed task.
type FailedTaskEvidence struct {
	TaskName     string `json:"task_name"`
	ExitCode     int    `json:"exit_code"`
	ErrorMessage string `json:"error_message,omitempty"`
	StdoutTail   string `json:"stdout_tail,omitempty"`
	StderrTail   string `json:"stderr_tail,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	Command      string `json:"command,omitempty"`
}

// AnalysisOutput is the structured response from an AI provider.
type AnalysisOutput struct {
	Summary            string         `json:"summary"`
	RootCause          string         `json:"root_cause"`
	Confidence         float64        `json:"confidence"`
	Evidence           []EvidenceItem `json:"evidence"`
	LikelyFiles        []string       `json:"likely_files"`
	RecommendedActions []string       `json:"recommended_actions"`
}

// EvidenceItem is a single piece of supporting evidence.
type EvidenceItem struct {
	Task       string `json:"task"`
	LogExcerpt string `json:"log_excerpt,omitempty"`
	Reason     string `json:"reason"`
}

// Provider can analyze a failed run.
type Provider interface {
	AnalyzeFailure(ctx context.Context, input AnalysisInput) (*AnalysisOutput, error)
}

// AgentClient is a remote BYOA agent that also supports health checks.
type AgentClient interface {
	Provider
	HealthCheck(ctx context.Context) error
}

// ProviderError represents an error from an AI provider.
type ProviderError struct {
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider error (status %d, retryable=%v): %s", e.StatusCode, e.Retryable, e.Message)
}

// SuggestionInput is the context sent to an AI provider for generating suggestions.
type SuggestionInput struct {
	RunID       string               `json:"run_id"`
	ProjectID   string               `json:"project_id"`
	Analysis    AnalysisOutput       `json:"analysis"`
	GitBranch   string               `json:"git_branch,omitempty"`
	GitCommit   string               `json:"git_commit,omitempty"`
	FailedTasks []FailedTaskEvidence `json:"failed_tasks"`
}

// SuggestionOutput is a single inline code suggestion from the AI provider.
type SuggestionOutput struct {
	FilePath      string  `json:"file_path"`
	StartLine     int     `json:"start_line"`
	EndLine       int     `json:"end_line"`
	OriginalCode  string  `json:"original_code"`
	SuggestedCode string  `json:"suggested_code"`
	Explanation   string  `json:"explanation"`
	Confidence    float64 `json:"confidence"`
}

// SuggestionProvider can generate inline code suggestions for a failed run.
type SuggestionProvider interface {
	ProposeSuggestions(ctx context.Context, input SuggestionInput) ([]SuggestionOutput, error)
}

// Sentinel errors.
var (
	ErrProviderTimeout     = errors.New("ai: provider request timed out")
	ErrProviderUnavailable = errors.New("ai: provider unavailable")
	ErrInvalidResponse     = errors.New("ai: invalid response from provider")
	ErrConfidenceTooLow    = errors.New("ai: confidence below threshold")
	ErrPolicyViolation     = errors.New("ai: policy violation")
)
