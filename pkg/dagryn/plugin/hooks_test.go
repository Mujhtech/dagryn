package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookContext_Env(t *testing.T) {
	hctx := &HookContext{
		RunID:          "run-123",
		TaskName:       "build",
		TaskStatus:     "success",
		TaskDurationMs: 1500,
		RunStatus:      "success",
		ProjectRoot:    "/tmp/project",
	}

	env := hctx.Env()
	assert.Equal(t, "run-123", env["DAGRYN_RUN_ID"])
	assert.Equal(t, "build", env["DAGRYN_TASK_NAME"])
	assert.Equal(t, "success", env["DAGRYN_TASK_STATUS"])
	assert.Equal(t, "1500", env["DAGRYN_TASK_DURATION_MS"])
	assert.Equal(t, "success", env["DAGRYN_RUN_STATUS"])
	assert.Equal(t, "/tmp/project", env["DAGRYN_PROJECT_ROOT"])
}

func TestHookContext_Env_EmptyFields(t *testing.T) {
	hctx := &HookContext{
		ProjectRoot: "/tmp",
	}

	env := hctx.Env()
	assert.Equal(t, "/tmp", env["DAGRYN_PROJECT_ROOT"])
	_, hasRunID := env["DAGRYN_RUN_ID"]
	assert.False(t, hasRunID, "empty RunID should not produce env var")
}

func TestHookExecutor_RunHook_Success(t *testing.T) {
	executor := NewHookExecutor(nil)

	hook := HookDef{
		Command: "echo hello",
	}

	hctx := &HookContext{
		ProjectRoot: t.TempDir(),
	}

	err := executor.RunHook(context.Background(), "test-plugin", "on_run_start", hook, nil, hctx)
	require.NoError(t, err)
}

func TestHookExecutor_RunHook_ConditionSkip(t *testing.T) {
	executor := NewHookExecutor(nil)

	hook := HookDef{
		Command: "echo should-not-run",
		If:      "false",
	}

	hctx := &HookContext{
		ProjectRoot: t.TempDir(),
	}

	err := executor.RunHook(context.Background(), "test-plugin", "on_run_start", hook, nil, hctx)
	require.NoError(t, err) // skipped, no error
}

func TestHookExecutor_RunHook_ConditionWithInput(t *testing.T) {
	executor := NewHookExecutor(nil)

	hook := HookDef{
		Command: "echo ${inputs.msg}",
		If:      "${inputs.enabled}",
	}

	inputs := map[string]string{
		"enabled": "true",
		"msg":     "hello-world",
	}

	hctx := &HookContext{
		ProjectRoot: t.TempDir(),
	}

	err := executor.RunHook(context.Background(), "test-plugin", "on_run_start", hook, inputs, hctx)
	require.NoError(t, err)
}

func TestHookExecutor_RunHook_Failure(t *testing.T) {
	executor := NewHookExecutor(nil)

	hook := HookDef{
		Command: "exit 1",
	}

	hctx := &HookContext{
		ProjectRoot: t.TempDir(),
	}

	err := executor.RunHook(context.Background(), "test-plugin", "on_run_start", hook, nil, hctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook test-plugin/on_run_start failed")
}

func TestHookExecutor_RunHook_WithEnv(t *testing.T) {
	executor := NewHookExecutor(nil)

	hook := HookDef{
		Command: "test \"$MY_VAR\" = \"hello\"",
		Env: map[string]string{
			"MY_VAR": "${inputs.greeting}",
		},
	}

	inputs := map[string]string{
		"greeting": "hello",
	}

	hctx := &HookContext{
		ProjectRoot: t.TempDir(),
	}

	err := executor.RunHook(context.Background(), "test-plugin", "on_run_start", hook, inputs, hctx)
	require.NoError(t, err)
}
