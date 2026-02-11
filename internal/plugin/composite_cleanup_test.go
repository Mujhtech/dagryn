package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger captures log messages for testing
type testLogger struct {
	infoLogs  []string
	debugLogs []string
	warnLogs  []string
	errorLogs []string
}

func (l *testLogger) Info(msg string, args ...interface{}) {
	l.infoLogs = append(l.infoLogs, msg)
}

func (l *testLogger) Debug(msg string, args ...interface{}) {
	l.debugLogs = append(l.debugLogs, msg)
}

func (l *testLogger) Warn(msg string, args ...interface{}) {
	l.warnLogs = append(l.warnLogs, msg)
}

func (l *testLogger) Error(msg string, args ...interface{}) {
	l.errorLogs = append(l.errorLogs, msg)
}

func TestCompositeExecutor_CleanupOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}
	e := NewCompositeExecutor(tmpDir, logger)
	ctx := context.Background()

	// Create test files to track execution order
	trackerFile := filepath.Join(tmpDir, "tracker.txt")

	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Steps: []CompositeStep{
			{Name: "step1", Command: "echo 'step1' >> " + trackerFile},
			{Name: "step2", Command: "echo 'step2' >> " + trackerFile},
		},
		Cleanup: []CompositeStep{
			{Name: "cleanup1", Command: "echo 'cleanup1' >> " + trackerFile},
			{Name: "cleanup2", Command: "echo 'cleanup2' >> " + trackerFile},
		},
	}

	err := e.Execute(ctx, manifest, nil, nil, "")
	require.NoError(t, err)

	// Verify cleanup ran
	assert.Contains(t, strings.Join(logger.infoLogs, " "), "running cleanup steps")
	assert.Contains(t, strings.Join(logger.infoLogs, " "), "cleanup completed")

	// Verify execution order: step1, step2, cleanup2 (reversed), cleanup1 (reversed)
	content, err := os.ReadFile(trackerFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	assert.Equal(t, []string{"step1", "step2", "cleanup2", "cleanup1"}, lines, "cleanup should run in reverse order")
}

func TestCompositeExecutor_CleanupOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}
	e := NewCompositeExecutor(tmpDir, logger)
	ctx := context.Background()

	trackerFile := filepath.Join(tmpDir, "tracker.txt")

	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Steps: []CompositeStep{
			{Name: "step1", Command: "echo 'step1' >> " + trackerFile},
			{Name: "failing-step", Command: "echo 'step2' >> " + trackerFile + " && exit 1"},
		},
		Cleanup: []CompositeStep{
			{Name: "cleanup1", Command: "echo 'cleanup1' >> " + trackerFile},
		},
	}

	err := e.Execute(ctx, manifest, nil, nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing-step")

	// Verify cleanup still ran despite failure
	assert.Contains(t, strings.Join(logger.infoLogs, " "), "running cleanup steps")
	assert.Contains(t, strings.Join(logger.infoLogs, " "), "cleanup completed")

	// Verify cleanup executed
	content, err := os.ReadFile(trackerFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "cleanup1")
}

func TestCompositeExecutor_CleanupErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}
	e := NewCompositeExecutor(tmpDir, logger)
	ctx := context.Background()

	trackerFile := filepath.Join(tmpDir, "tracker.txt")

	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Steps: []CompositeStep{
			{Name: "step1", Command: "echo 'step1' >> " + trackerFile},
		},
		Cleanup: []CompositeStep{
			{Name: "cleanup1", Command: "echo 'cleanup1' >> " + trackerFile},
			{Name: "failing-cleanup", Command: "exit 1"}, // This should fail but not stop other cleanup
			{Name: "cleanup3", Command: "echo 'cleanup3' >> " + trackerFile},
		},
	}

	err := e.Execute(ctx, manifest, nil, nil, "")
	require.NoError(t, err, "main steps should succeed")

	// Verify cleanup errors were logged as warnings
	assert.NotEmpty(t, logger.warnLogs, "should have warning logs for failed cleanup")
	warnMsg := strings.Join(logger.warnLogs, " ")
	assert.True(t, strings.Contains(warnMsg, "cleanup step") || strings.Contains(warnMsg, "failed"), "warning should mention cleanup step or failure")

	// Verify all cleanup steps ran despite one failing (reverse order: cleanup3, failing-cleanup, cleanup1)
	content, err := os.ReadFile(trackerFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	assert.Equal(t, []string{"step1", "cleanup3", "cleanup1"}, lines, "all cleanup steps should run even if one fails")
}

func TestCompositeExecutor_CleanupWithConditionals(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}
	e := NewCompositeExecutor(tmpDir, logger)
	ctx := context.Background()

	trackerFile := filepath.Join(tmpDir, "tracker.txt")

	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Inputs: map[string]InputDef{
			"cleanup-enabled": {Default: "true"},
		},
		Steps: []CompositeStep{
			{Name: "step1", Command: "echo 'step1' >> " + trackerFile},
		},
		Cleanup: []CompositeStep{
			{Name: "conditional-cleanup", Command: "echo 'conditional' >> " + trackerFile, If: "${inputs.cleanup-enabled}"},
			{Name: "always-cleanup", Command: "echo 'always' >> " + trackerFile},
		},
	}

	t.Run("cleanup enabled", func(t *testing.T) {
		inputs := map[string]string{"cleanup-enabled": "true"}
		err := e.Execute(ctx, manifest, inputs, nil, "")
		require.NoError(t, err)

		content, err := os.ReadFile(trackerFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "conditional")
		assert.Contains(t, string(content), "always")

		// Clean up for next test
		os.Remove(trackerFile)
	})

	t.Run("cleanup disabled", func(t *testing.T) {
		inputs := map[string]string{"cleanup-enabled": "false"}
		err := e.Execute(ctx, manifest, inputs, nil, "")
		require.NoError(t, err)

		content, err := os.ReadFile(trackerFile)
		require.NoError(t, err)
		assert.NotContains(t, string(content), "conditional", "conditional cleanup should be skipped")
		assert.Contains(t, string(content), "always", "always cleanup should run")
	})
}

func TestCompositeExecutor_CleanupEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}
	e := NewCompositeExecutor(tmpDir, logger)
	ctx := context.Background()

	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Steps: []CompositeStep{
			{
				Name:    "set-env",
				Command: "echo 'step ran'",
				Env:     map[string]string{"CONTAINER_ID": "abc123"},
			},
		},
		Cleanup: []CompositeStep{
			{
				Name:    "use-env",
				Command: "test \"$CONTAINER_ID\" = \"abc123\"", // Should have access to env from steps
			},
		},
	}

	err := e.Execute(ctx, manifest, nil, nil, "")
	require.NoError(t, err, "cleanup should have access to environment variables set during steps")
}

func TestCompositeExecutor_NoCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}
	e := NewCompositeExecutor(tmpDir, logger)
	ctx := context.Background()

	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Steps: []CompositeStep{
			{Name: "step1", Command: "echo hello"},
		},
		// No cleanup steps
	}

	err := e.Execute(ctx, manifest, nil, nil, "")
	require.NoError(t, err)

	// Verify no cleanup logs
	assert.NotContains(t, strings.Join(logger.infoLogs, " "), "running cleanup steps")
}

func TestCompositeExecutor_CleanupWithInputSubstitution(t *testing.T) {
	tmpDir := t.TempDir()
	logger := &testLogger{}
	e := NewCompositeExecutor(tmpDir, logger)
	ctx := context.Background()

	trackerFile := filepath.Join(tmpDir, "tracker.txt")

	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Inputs: map[string]InputDef{
			"resource-id": {Required: true},
		},
		Steps: []CompositeStep{
			{Name: "create", Command: "echo 'created ${inputs.resource-id}' >> " + trackerFile},
		},
		Cleanup: []CompositeStep{
			{Name: "destroy", Command: "echo 'destroyed ${inputs.resource-id}' >> " + trackerFile},
		},
	}

	inputs := map[string]string{"resource-id": "resource-123"}
	err := e.Execute(ctx, manifest, inputs, nil, "")
	require.NoError(t, err)

	content, err := os.ReadFile(trackerFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "created resource-123")
	assert.Contains(t, string(content), "destroyed resource-123")
}
