package executor

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mujhtech/dagryn/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeEnv(t *testing.T) {
	taskEnv := map[string]string{
		"CUSTOM_VAR": "custom_value",
		"PATH":       "/custom/path", // Override system PATH
	}

	merged := MergeEnv(taskEnv)

	// Check that task env is included
	found := false
	for _, e := range merged {
		if e == "CUSTOM_VAR=custom_value" {
			found = true
			break
		}
	}
	assert.True(t, found, "CUSTOM_VAR should be in merged env")

	// Check that PATH is overridden
	for _, e := range merged {
		if strings.HasPrefix(e, "PATH=") {
			assert.Equal(t, "PATH=/custom/path", e)
			break
		}
	}
}

func TestEnvToMap(t *testing.T) {
	env := []string{"KEY1=value1", "KEY2=value2", "KEY3=value=with=equals"}
	m := EnvToMap(env)

	assert.Equal(t, "value1", m["KEY1"])
	assert.Equal(t, "value2", m["KEY2"])
	assert.Equal(t, "value=with=equals", m["KEY3"])
}

func TestMapToEnv(t *testing.T) {
	m := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}
	env := MapToEnv(m)

	assert.Len(t, env, 2)
	assert.Contains(t, env, "KEY1=value1")
	assert.Contains(t, env, "KEY2=value2")
}

func TestOutputCapture(t *testing.T) {
	capture := NewOutputCapture()

	// Write to stdout
	_, _ = capture.StdoutWriter().Write([]byte("stdout output"))
	assert.Equal(t, "stdout output", capture.Stdout())

	// Write to stderr
	_, _ = capture.StderrWriter().Write([]byte("stderr output"))
	assert.Equal(t, "stderr output", capture.Stderr())

	// Combined output
	assert.Equal(t, "stdout outputstderr output", capture.Combined())

	// Reset
	capture.Reset()
	assert.Empty(t, capture.Stdout())
	assert.Empty(t, capture.Stderr())
}

func TestExecutor_Execute_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	executor := New(tmpDir)
	task := &task.Task{
		Name:    "echo-test",
		Command: "echo 'hello world'",
	}

	result := executor.Execute(context.Background(), task)

	assert.Equal(t, Success, result.Status)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Output, "hello world")
	assert.NoError(t, result.Error)
	assert.True(t, result.IsSuccess())
}

func TestExecutor_Execute_Failure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	executor := New(tmpDir)
	task := &task.Task{
		Name:    "fail-test",
		Command: "exit 1",
	}

	result := executor.Execute(context.Background(), task)

	assert.Equal(t, Failed, result.Status)
	assert.Equal(t, 1, result.ExitCode)
	assert.Error(t, result.Error)
	assert.False(t, result.IsSuccess())
}

func TestExecutor_Execute_Timeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	executor := New(tmpDir)
	task := &task.Task{
		Name:    "timeout-test",
		Command: "sleep 10",
		Timeout: 100 * time.Millisecond,
	}

	result := executor.Execute(context.Background(), task)

	assert.Equal(t, TimedOut, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "timed out")
}

func TestExecutor_Execute_Cancelled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	executor := New(tmpDir)
	task := &task.Task{
		Name:    "cancel-test",
		Command: "sleep 10",
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := executor.Execute(ctx, task)

	assert.Equal(t, Cancelled, result.Status)
}

func TestExecutor_Execute_WithWorkdir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create subdirectory
	subDir := "subdir"
	err = os.MkdirAll(tmpDir+"/"+subDir, 0755)
	require.NoError(t, err)

	executor := New(tmpDir)
	task := &task.Task{
		Name:    "workdir-test",
		Command: "pwd",
		Workdir: subDir,
	}

	result := executor.Execute(context.Background(), task)

	assert.Equal(t, Success, result.Status)
	assert.Contains(t, result.Output, subDir)
}

func TestExecutor_Execute_WithEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "executor-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	executor := New(tmpDir)
	task := &task.Task{
		Name:    "env-test",
		Command: "echo $MY_VAR",
		Env:     map[string]string{"MY_VAR": "test_value"},
	}

	result := executor.Execute(context.Background(), task)

	assert.Equal(t, Success, result.Status)
	assert.Contains(t, result.Output, "test_value")
}

func TestExecutor_DryRun(t *testing.T) {
	executor := New("/tmp")
	task := &task.Task{
		Name:    "dry-run-test",
		Command: "echo hello",
	}

	result := executor.DryRun(task)

	assert.Equal(t, Skipped, result.Status)
	assert.Contains(t, result.Output, "DRY RUN")
	assert.Contains(t, result.Output, "echo hello")
}

func TestStatus_String(t *testing.T) {
	assert.Equal(t, "SUCCESS", Success.String())
	assert.Equal(t, "FAILED", Failed.String())
	assert.Equal(t, "CACHED", Cached.String())
	assert.Equal(t, "SKIPPED", Skipped.String())
	assert.Equal(t, "TIMED_OUT", TimedOut.String())
	assert.Equal(t, "CANCELLED", Cancelled.String())
}

func TestResult_IsSuccess(t *testing.T) {
	assert.True(t, (&Result{Status: Success}).IsSuccess())
	assert.True(t, (&Result{Status: Cached}).IsSuccess())
	assert.False(t, (&Result{Status: Failed}).IsSuccess())
	assert.False(t, (&Result{Status: Skipped}).IsSuccess())
}
