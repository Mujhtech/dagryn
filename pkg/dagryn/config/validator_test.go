package config

import (
	"path/filepath"
	"testing"

	"github.com/mujhtech/dagryn/pkg/dagryn/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg, err := Parse(filepath.Join("..", "..", "testdata", "valid.toml"))
	require.NoError(t, err)

	errors := Validate(cfg)
	assert.Empty(t, errors)
}

func TestValidate_MissingWorkflowName(t *testing.T) {
	cfg := &Config{
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Message == "workflow name is required" {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_NoTasks(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks:    map[string]TaskConfig{},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Message == "at least one task is required" {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_EmptyCommand(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: ""},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Task == "build" && assert.Contains(t, e.Message, "command is required") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_UsesWithoutCommand(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"setup": {
				Uses: pluginSpec("dagryn/setup-go@v1"),
				With: map[string]string{"go-version": "1.22"},
			},
			"build": {Command: "go build ./..."},
		},
	}

	errors := Validate(cfg)
	assert.Empty(t, errors)
}

func TestValidate_WithWithoutUses(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {
				Command: "go build ./...",
				With:    map[string]string{"key": "value"},
			},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Task == "build" && assert.Contains(t, e.Message, "'with' requires 'uses'") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

// pluginSpec creates a plugin.Spec from a single string for testing.
func pluginSpec(s string) pluginSpecType {
	var spec pluginSpecType
	spec.Plugins = []string{s}
	return spec
}

type pluginSpecType = plugin.Spec

func TestValidate_MissingDependency(t *testing.T) {
	cfg, err := Parse(filepath.Join("..", "..", "testdata", "missing_dep.toml"))
	require.NoError(t, err)

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Task == "build" && e.Message == `depends on unknown task "install"` {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_InvalidTimeout(t *testing.T) {
	cfg, err := Parse(filepath.Join("..", "..", "testdata", "invalid_timeout.toml"))
	require.NoError(t, err)

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Task == "build" && assert.Contains(t, e.Message, "invalid timeout") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_CyclicDependency(t *testing.T) {
	cfg, err := Parse(filepath.Join("..", "..", "testdata", "cycle.toml"))
	require.NoError(t, err)

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Task == "" && assert.Contains(t, e.Message, "cyclic dependency detected") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_RemoteCacheCloudMode(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		Cache: CacheConfig{
			Remote: RemoteCacheConfig{
				Enabled: true,
				Cloud:   true,
				// No provider, bucket, or base_path — cloud mode skips those
			},
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		// Should have no cache-related validation errors
		assert.NotContains(t, e.Message, "cache.remote.provider")
		assert.NotContains(t, e.Message, "cache.remote.bucket")
		assert.NotContains(t, e.Message, "cache.remote.base_path")
	}
}

func TestValidate_RemoteCacheCloudModeInvalidStrategy(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		Cache: CacheConfig{
			Remote: RemoteCacheConfig{
				Enabled:  true,
				Cloud:    true,
				Strategy: "invalid-strategy",
			},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, "cache.remote.strategy") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Task: "build", Message: "command is required"}
	assert.Equal(t, `task "build": command is required`, err.Error())

	err2 := &ValidationError{Message: "workflow name is required"}
	assert.Equal(t, "workflow name is required", err2.Error())
}

func TestValidationErrors_Error(t *testing.T) {
	var errors ValidationErrors
	assert.Equal(t, "", errors.Error())

	errors = append(errors, ValidationError{Task: "build", Message: "error1"})
	assert.Equal(t, `task "build": error1`, errors.Error())

	errors = append(errors, ValidationError{Task: "test", Message: "error2"})
	assert.Contains(t, errors.Error(), "2 validation errors")
}

// --- Group validation tests ---

func TestValidate_GroupNameCollision(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./...", Group: "test"}, // group name collides with task name "test"
			"test":  {Command: "go test ./..."},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Task == "build" && assert.Contains(t, e.Message, "collides with a task name") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_InvalidGroupName(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./...", Group: "123-invalid"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Task == "build" && assert.Contains(t, e.Message, "invalid name") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_ValidGroupName(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./...", Group: "backend"},
			"test":  {Command: "go test ./...", Group: "backend"},
		},
	}

	errors := Validate(cfg)
	// Should have no group-related errors
	for _, e := range errors {
		assert.NotContains(t, e.Message, "group")
	}
}

// --- Trigger validation tests ---

func TestValidate_ValidTriggers(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{
			Name: "ci",
			Trigger: &TriggerConfig{
				Push: &PushTriggerConfig{
					Branches: []string{"main", "develop"},
				},
				PullRequest: &PullRequestTriggerConfig{
					Branches: []string{"main"},
					Types:    []string{"opened", "synchronize", "reopened"},
				},
			},
		},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "trigger")
	}
}

func TestValidate_UnknownPRType(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{
			Name: "ci",
			Trigger: &TriggerConfig{
				PullRequest: &PullRequestTriggerConfig{
					Types: []string{"opened", "invalid_type"},
				},
			},
		},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, "unknown type") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_NilTrigger(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
	}

	errors := Validate(cfg)
	// No trigger-related errors
	for _, e := range errors {
		assert.NotContains(t, e.Message, "trigger")
	}
}

// --- AI validation tests ---

func boolPtr(b bool) *bool { return &b }

func TestValidate_AIDisabledByDefault(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		// AI section omitted — should default to disabled with no errors
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "ai.")
	}
}

func TestValidate_AIEnabledRequiresMode(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled: boolPtr(true),
			// Mode and Backend.Mode missing
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	var hasMode, hasBackend bool
	for _, e := range errors {
		if e.Message == `ai.mode is required when AI is enabled (use "summarize" or "summarize_and_suggest")` {
			hasMode = true
		}
		if e.Message == `ai.backend.mode is required when AI is enabled (use "managed", "byok", or "agent")` {
			hasBackend = true
		}
	}
	assert.True(t, hasMode, "expected ai.mode required error")
	assert.True(t, hasBackend, "expected ai.backend.mode required error")
}

func TestValidate_AIInvalidMode(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled: boolPtr(true),
			Mode:    "invalid",
			Backend: AIBackendConfig{Mode: "managed"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, `ai.mode "invalid" is not supported`) {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_AIAgentRequiresEndpoint(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled: boolPtr(true),
			Mode:    "summarize",
			Backend: AIBackendConfig{
				Mode:  "agent",
				Agent: AIAgentConfig{
					// Endpoint missing, TimeoutSeconds missing
				},
			},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	var hasEndpoint, hasTimeout bool
	for _, e := range errors {
		if e.Message == `ai.backend.agent.endpoint is required when backend mode is "agent"` {
			hasEndpoint = true
		}
		if e.Message == `ai.backend.agent.timeout_seconds must be > 0 when backend mode is "agent"` {
			hasTimeout = true
		}
	}
	assert.True(t, hasEndpoint)
	assert.True(t, hasTimeout)
}

func TestValidate_AIBYOKRequiresAPIKeyEnv(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled: boolPtr(true),
			Mode:    "summarize",
			Backend: AIBackendConfig{Mode: "byok"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if e.Message == `ai.backend.byok.api_key_env should be set when backend mode is "byok"` {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_AIGuardrailsConfidence(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled: boolPtr(true),
			Mode:    "summarize",
			Backend: AIBackendConfig{Mode: "managed"},
			Guardrails: AIGuardrailConfig{
				MinConfidence: 1.5, // invalid
			},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, "min_confidence must be between 0.0 and 1.0") {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_AIUnsupportedProvider(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize",
			Provider: "unsupported",
			Backend:  AIBackendConfig{Mode: "managed"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, `ai.provider "unsupported" is not supported`) {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_AIGoogleProvider(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize",
			Provider: "google",
			Backend:  AIBackendConfig{Mode: "managed"},
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "ai.provider")
	}
}

func TestValidate_AIValidConfig(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize_and_suggest",
			Provider: "openai",
			Model:    "gpt-4o",
			Backend:  AIBackendConfig{Mode: "managed"},
			Guardrails: AIGuardrailConfig{
				MinConfidence:             0.7,
				MaxSuggestionLines:        50,
				MaxSuggestionsPerAnalysis: 5,
			},
			RateLimit: AIRateLimitConfig{
				MaxAnalysesPerHour:    10,
				CooldownSeconds:       30,
				MaxConcurrentAnalyses: 2,
			},
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "ai.")
	}
}

// --- AI model validation tests ---

func TestValidate_AIManagedModeUnsupportedModel(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize",
			Provider: "openai",
			Model:    "gpt-3.5-turbo",
			Backend:  AIBackendConfig{Mode: "managed"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, `ai.model "gpt-3.5-turbo" is not supported for managed mode`) {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_AIManagedModeValidModel(t *testing.T) {
	for _, model := range []string{"gpt-4o", "gpt-4o-mini", "o4-mini"} {
		t.Run(model, func(t *testing.T) {
			cfg := &Config{
				Workflow: WorkflowConfig{Name: "ci"},
				Tasks: map[string]TaskConfig{
					"build": {Command: "go build ./..."},
				},
				AI: AIConfig{
					Enabled:  boolPtr(true),
					Mode:     "summarize",
					Provider: "openai",
					Model:    model,
					Backend:  AIBackendConfig{Mode: "managed"},
				},
			}

			errors := Validate(cfg)
			for _, e := range errors {
				assert.NotContains(t, e.Message, "ai.model")
			}
		})
	}
}

func TestValidate_AIManagedModeGeminiModel(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize",
			Provider: "gemini",
			Model:    "gemini-3-pro-preview",
			Backend:  AIBackendConfig{Mode: "managed"},
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "ai.model")
	}
}

func TestValidate_AIManagedModeGeminiUnsupportedModel(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize",
			Provider: "google",
			Model:    "gemini-1.0-pro",
			Backend:  AIBackendConfig{Mode: "managed"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, `ai.model "gemini-1.0-pro" is not supported for managed mode`) {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

func TestValidate_AIBYOKModeSkipsModelValidation(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize",
			Provider: "openai",
			Model:    "my-custom-fine-tuned-model",
			Backend: AIBackendConfig{
				Mode: "byok",
				BYOK: AIBYOKConfig{APIKeyEnv: "MY_API_KEY"},
			},
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "ai.model")
	}
}

func TestValidate_AIAgentModeSkipsModelValidation(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled:  boolPtr(true),
			Mode:     "summarize",
			Provider: "openai",
			Model:    "anything-goes",
			Backend: AIBackendConfig{
				Mode: "agent",
				Agent: AIAgentConfig{
					Endpoint:       "https://my-agent.example.com",
					TimeoutSeconds: 30,
				},
			},
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "ai.model")
	}
}

func TestValidate_AIManagedModeEmptyModelAllowed(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled: boolPtr(true),
			Mode:    "summarize",
			Backend: AIBackendConfig{Mode: "managed"},
			// Model omitted — defaults applied at runtime
		},
	}

	errors := Validate(cfg)
	for _, e := range errors {
		assert.NotContains(t, e.Message, "ai.model")
	}
}

func TestValidate_AIManagedModeDefaultProviderModel(t *testing.T) {
	// When provider is empty (defaults to openai), model should be validated against openai list
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {Command: "go build ./..."},
		},
		AI: AIConfig{
			Enabled: boolPtr(true),
			Mode:    "summarize",
			Model:   "gemini-3-pro-preview", // google model with default (openai) provider
			Backend: AIBackendConfig{Mode: "managed"},
		},
	}

	errors := Validate(cfg)
	require.NotEmpty(t, errors)

	hasError := false
	for _, e := range errors {
		if assert.Contains(t, e.Message, `ai.model "gemini-3-pro-preview" is not supported for managed mode with provider "openai"`) {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}
