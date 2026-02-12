package config

import (
	"path/filepath"
	"testing"

	"github.com/mujhtech/dagryn/internal/plugin"
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
