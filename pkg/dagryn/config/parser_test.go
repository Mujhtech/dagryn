package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ValidConfig(t *testing.T) {
	cfg, err := Parse(filepath.Join("..", "..", "testdata", "valid.toml"))
	require.NoError(t, err)

	assert.Equal(t, "ci", cfg.Workflow.Name)
	assert.True(t, cfg.Workflow.Default)
	assert.Len(t, cfg.Tasks, 4)

	// Check install task
	install, ok := cfg.Tasks["install"]
	assert.True(t, ok)
	assert.Equal(t, "npm install", install.Command)
	assert.Equal(t, []string{"package.json", "package-lock.json"}, install.Inputs)
	assert.Equal(t, []string{"node_modules/**"}, install.Outputs)
	assert.Equal(t, "5m", install.Timeout)

	// Check build task
	build, ok := cfg.Tasks["build"]
	assert.True(t, ok)
	assert.Equal(t, "npm run build", build.Command)
	assert.Equal(t, []string{"install"}, build.Needs)
	assert.Equal(t, "./packages/app", build.Workdir)

	// Check lint task
	lint, ok := cfg.Tasks["lint"]
	assert.True(t, ok)
	assert.Equal(t, "true", lint.Env["CI"])
}

func TestParse_FileNotFound(t *testing.T) {
	_, err := Parse("nonexistent.toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestParseBytes_InvalidTOML(t *testing.T) {
	_, err := ParseBytes([]byte("invalid toml [[["))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}

func TestConfig_ToWorkflow(t *testing.T) {
	cfg, err := Parse(filepath.Join("..", "..", "testdata", "valid.toml"))
	require.NoError(t, err)

	w, err := cfg.ToWorkflow()
	require.NoError(t, err)

	assert.Equal(t, "ci", w.Name)
	assert.True(t, w.Default)
	assert.Equal(t, 4, w.Size())

	// Check task conversion
	install, ok := w.GetTask("install")
	assert.True(t, ok)
	assert.Equal(t, "npm install", install.Command)
	assert.Equal(t, 5*time.Minute, install.Timeout)

	build, ok := w.GetTask("build")
	assert.True(t, ok)
	assert.Equal(t, []string{"install"}, build.Needs)
	assert.Equal(t, "./packages/app", build.Workdir)
}

func TestConfig_ToWorkflow_InvalidTimeout(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build": {
				Command: "npm build",
				Timeout: "invalid",
			},
		},
	}

	_, err := cfg.ToWorkflow()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout")
}

func TestParseBytes_WithTriggers(t *testing.T) {
	toml := `
[workflow]
name = "ci"

[workflow.trigger.push]
branches = ["main", "develop"]

[workflow.trigger.pull_request]
branches = ["main"]
types = ["opened", "synchronize"]

[tasks.build]
command = "go build ./..."
`
	cfg, err := ParseBytes([]byte(toml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Workflow.Trigger)
	require.NotNil(t, cfg.Workflow.Trigger.Push)
	assert.Equal(t, []string{"main", "develop"}, cfg.Workflow.Trigger.Push.Branches)
	require.NotNil(t, cfg.Workflow.Trigger.PullRequest)
	assert.Equal(t, []string{"main"}, cfg.Workflow.Trigger.PullRequest.Branches)
	assert.Equal(t, []string{"opened", "synchronize"}, cfg.Workflow.Trigger.PullRequest.Types)
}

func TestParseBytes_WithoutTriggers(t *testing.T) {
	toml := `
[workflow]
name = "ci"

[tasks.build]
command = "go build ./..."
`
	cfg, err := ParseBytes([]byte(toml))
	require.NoError(t, err)
	assert.Nil(t, cfg.Workflow.Trigger)
}

func TestParseBytes_WithGroupAndIf(t *testing.T) {
	toml := `
[workflow]
name = "ci"

[tasks.build]
command = "go build ./..."
group = "backend"

[tasks.deploy]
command = "make deploy"
group = "backend"
if = "branch == 'main'"
needs = ["build"]
`
	cfg, err := ParseBytes([]byte(toml))
	require.NoError(t, err)

	build := cfg.Tasks["build"]
	assert.Equal(t, "backend", build.Group)
	assert.Empty(t, build.If)

	deploy := cfg.Tasks["deploy"]
	assert.Equal(t, "backend", deploy.Group)
	assert.Equal(t, "branch == 'main'", deploy.If)
}

func TestConfig_ToWorkflow_GroupAndIf(t *testing.T) {
	cfg := &Config{
		Workflow: WorkflowConfig{Name: "ci"},
		Tasks: map[string]TaskConfig{
			"build":  {Command: "go build ./...", Group: "backend"},
			"deploy": {Command: "make deploy", Group: "backend", If: "branch == 'main'"},
		},
	}

	w, err := cfg.ToWorkflow()
	require.NoError(t, err)

	build, ok := w.GetTask("build")
	require.True(t, ok)
	assert.Equal(t, "backend", build.Group)
	assert.Empty(t, build.If)

	deploy, ok := w.GetTask("deploy")
	require.True(t, ok)
	assert.Equal(t, "backend", deploy.Group)
	assert.Equal(t, "branch == 'main'", deploy.If)
}
