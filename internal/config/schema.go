package config

import (
	"github.com/mujhtech/dagryn/internal/plugin"
	"github.com/mujhtech/dagryn/internal/task"
)

// ContainerConfig holds the project-level container isolation configuration.
type ContainerConfig struct {
	Enabled     bool   `toml:"enabled"`
	Image       string `toml:"image"`        // Default image, e.g. "golang:1.25"
	MemoryLimit string `toml:"memory_limit"` // e.g. "2g", "512m"
	CPULimit    string `toml:"cpu_limit"`    // e.g. "2.0", "0.5"
	Network     string `toml:"network"`      // e.g. "bridge", "none"
}

// Config represents the root configuration loaded from dagryn.toml.
type Config struct {
	Workflow  WorkflowConfig        `toml:"workflow"`
	Tasks     map[string]TaskConfig `toml:"tasks"`
	Plugins   map[string]string     `toml:"plugins"` // Global plugins available to all tasks
	Cache     CacheConfig           `toml:"cache"`
	Container ContainerConfig       `toml:"container"` // Project-level container isolation settings
	AI        AIConfig              `toml:"ai"`        // AI analysis configuration
}

// AIConfig controls AI-powered analysis of CI runs.
type AIConfig struct {
	Enabled    *bool             `toml:"enabled"`  // *bool, default false
	Mode       string            `toml:"mode"`     // "summarize" | "summarize_and_suggest"
	Provider   string            `toml:"provider"` // "openai", "google", "gemini"
	Model      string            `toml:"model"`
	Backend    AIBackendConfig   `toml:"backend"`
	Guardrails AIGuardrailConfig `toml:"guardrails"`
	RateLimit  AIRateLimitConfig `toml:"rate_limit"`
	Publish    AIPublishConfig   `toml:"publish"`
}

// IsEnabled returns whether AI analysis is enabled (defaults to false).
func (c AIConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return false
	}
	return *c.Enabled
}

// AIBackendConfig configures which AI backend to use.
type AIBackendConfig struct {
	Mode    string        `toml:"mode"` // "managed" | "byok" | "agent"
	Profile string        `toml:"profile"`
	BYOK    AIBYOKConfig  `toml:"byok"`
	Agent   AIAgentConfig `toml:"agent"`
}

// AIBYOKConfig holds "bring your own key" configuration.
type AIBYOKConfig struct {
	APIKeyEnv string `toml:"api_key_env"`
}

// AIAgentConfig holds external agent endpoint configuration.
type AIAgentConfig struct {
	Endpoint       string `toml:"endpoint"`
	AuthTokenEnv   string `toml:"auth_token_env"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
}

// AIGuardrailConfig holds safety guardrails for AI suggestions.
type AIGuardrailConfig struct {
	MinConfidence             float64  `toml:"min_confidence"`
	MaxSuggestionLines        int      `toml:"max_suggestion_lines"`
	MaxSuggestionsPerAnalysis int      `toml:"max_suggestions_per_analysis"`
	MaxFilesChanged           int      `toml:"max_files_changed"`
	AllowedPaths              []string `toml:"allowed_paths"`
	BlockedPaths              []string `toml:"blocked_paths"`
	RequireHumanApproval      *bool    `toml:"require_human_approval"`
}

// AIRateLimitConfig holds rate limiting configuration for AI analyses.
type AIRateLimitConfig struct {
	MaxAnalysesPerHour    int `toml:"max_analyses_per_hour"`
	CooldownSeconds       int `toml:"cooldown_seconds"`
	MaxConcurrentAnalyses int `toml:"max_concurrent_analyses"`
}

// AIPublishConfig controls where AI results are published.
type AIPublishConfig struct {
	GitHubComment     *bool `toml:"github_comment"`     // default true
	GitHubCheck       *bool `toml:"github_check"`       // default true
	GitHubSuggestions bool  `toml:"github_suggestions"` // default false
}

// CacheConfig controls local and remote caching.
type CacheConfig struct {
	Enabled *bool             `toml:"enabled"` // default true when nil
	Dir     string            `toml:"dir"`     // override local cache directory
	Remote  RemoteCacheConfig `toml:"remote"`
}

// IsEnabled returns whether local caching is enabled (defaults to true).
func (c CacheConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// RemoteCacheConfig configures the remote cache backend.
type RemoteCacheConfig struct {
	Enabled         bool   `toml:"enabled"`
	Cloud           bool   `toml:"cloud"`    // Use Dagryn Cloud cache API
	Provider        string `toml:"provider"` // "s3", "filesystem", "grpc" (ignored when cloud=true)
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	UsePathStyle    bool   `toml:"use_path_style"`
	Prefix          string `toml:"prefix"`
	BasePath        string `toml:"base_path"`
	Strategy        string `toml:"strategy"`          // default "local-first"
	FallbackOnError *bool  `toml:"fallback_on_error"` // default true when nil

	// gRPC (REAPI) cache settings
	GRPCTarget   string `toml:"grpc_target"`   // e.g. "localhost:9092"
	InstanceName string `toml:"instance_name"` // REAPI instance name
	TLS          *bool  `toml:"tls"`           // enable TLS (default true when nil)
	TLSCACert    string `toml:"tls_ca_cert"`   // custom CA cert path
	AuthToken    string `toml:"auth_token"`    // bearer token
}

// IsTLS returns whether TLS is enabled (defaults to true when nil).
func (rc RemoteCacheConfig) IsTLS() bool {
	if rc.TLS == nil {
		return true
	}
	return *rc.TLS
}

// IsFallbackOnError returns whether remote errors are non-fatal (defaults to true).
func (rc RemoteCacheConfig) IsFallbackOnError() bool {
	if rc.FallbackOnError == nil {
		return true
	}
	return *rc.FallbackOnError
}

// WorkflowConfig represents the workflow section of the config.
type WorkflowConfig struct {
	Name    string         `toml:"name"`
	Default bool           `toml:"default"`
	Trigger *TriggerConfig `toml:"trigger"` // Optional workflow triggers
}

// TriggerConfig defines which webhook events trigger the workflow.
// A nil TriggerConfig means all events match (backward compatible).
type TriggerConfig struct {
	Push        *PushTriggerConfig        `toml:"push"`
	PullRequest *PullRequestTriggerConfig `toml:"pull_request"`
}

// PushTriggerConfig filters push events by branch.
type PushTriggerConfig struct {
	Branches []string `toml:"branches"`
}

// PullRequestTriggerConfig filters pull_request events by target branch and action type.
type PullRequestTriggerConfig struct {
	Branches []string `toml:"branches"` // target (base) branches
	Types    []string `toml:"types"`    // e.g. "opened", "synchronize", "reopened"
}

// MatchesPush returns true if a push to the given branch should trigger the workflow.
// Returns true if TriggerConfig is nil, Push is nil, or branches list is empty (all match).
func (tc *TriggerConfig) MatchesPush(branch string) bool {
	if tc == nil || tc.Push == nil || len(tc.Push.Branches) == 0 {
		return true
	}
	for _, b := range tc.Push.Branches {
		if b == branch {
			return true
		}
	}
	return false
}

// MatchesPullRequest returns true if a pull_request event with the given base branch
// and action should trigger the workflow.
// Returns true if TriggerConfig is nil, PullRequest is nil, or both lists are empty.
func (tc *TriggerConfig) MatchesPullRequest(baseBranch, action string) bool {
	if tc == nil || tc.PullRequest == nil {
		return true
	}
	pr := tc.PullRequest

	if len(pr.Branches) > 0 {
		matched := false
		for _, b := range pr.Branches {
			if b == baseBranch {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(pr.Types) > 0 {
		for _, t := range pr.Types {
			if t == action {
				return true
			}
		}
		return false
	}

	return true
}

// TaskConfig represents a task definition in the config file.
type TaskConfig struct {
	Command   string                    `toml:"command"`
	Uses      plugin.Spec               `toml:"uses"` // Plugin dependencies (single string or array)
	Inputs    []string                  `toml:"inputs"`
	Outputs   []string                  `toml:"outputs"`
	Needs     []string                  `toml:"needs"`
	Env       map[string]string         `toml:"env"`
	Timeout   string                    `toml:"timeout"` // e.g., "30s", "5m"
	Workdir   string                    `toml:"workdir"`
	With      map[string]string         `toml:"with"`      // Inputs for composite plugins
	Container *task.TaskContainerConfig `toml:"container"` // Per-task container overrides
	Group     string                    `toml:"group"`     // Logical group for target resolution
	If        string                    `toml:"if"`        // Condition expression for conditional execution
}

// HasPlugins returns true if the task has any plugin dependencies.
func (t *TaskConfig) HasPlugins() bool {
	return !t.Uses.IsEmpty()
}

// GetPlugins returns the list of plugin specs for this task.
func (t *TaskConfig) GetPlugins() []string {
	return t.Uses.Plugins
}
