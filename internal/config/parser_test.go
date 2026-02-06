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
