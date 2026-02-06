package config

import (
	"path/filepath"
	"testing"

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
		if e.Task == "build" && e.Message == "command is required" {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)
}

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
