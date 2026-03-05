package container

import (
	"context"
	"testing"

	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerExecutorWithPluginPaths(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{Image: "ubuntu:22.04"}
	exec := NewContainerExecutor(mock, "/workspace", cfg,
		WithPluginPaths([]string{"/home/user/.dagryn/plugins/node/bin", "/home/user/.dagryn/plugins/go/bin"}),
	)

	tsk := &task.Task{
		Name:    "test",
		Command: "node --version",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Success, result.Status)

	// Verify plugin paths were mounted
	require.Len(t, mock.createCalls, 1)
	mounts := mock.createCalls[0].Mounts
	require.Len(t, mounts, 3) // workspace + 2 plugin paths

	// Check workspace mount
	assert.Equal(t, "/workspace", mounts[0].Source)
	assert.Equal(t, "/workspace", mounts[0].Target)
	assert.False(t, mounts[0].ReadOnly)

	// Check plugin mounts
	assert.Equal(t, "/home/user/.dagryn/plugins/node/bin", mounts[1].Source)
	assert.Equal(t, "/dagryn-plugins/0/bin", mounts[1].Target)
	assert.True(t, mounts[1].ReadOnly)

	assert.Equal(t, "/home/user/.dagryn/plugins/go/bin", mounts[2].Source)
	assert.Equal(t, "/dagryn-plugins/1/bin", mounts[2].Target)
	assert.True(t, mounts[2].ReadOnly)

	// Verify plugin paths are exported in the command (not env) so the
	// image's PATH is preserved via $PATH shell expansion.
	cmd := mock.createCalls[0].Command
	require.True(t, len(cmd) >= 3, "expected sh -c <script>")
	script := cmd[2]
	assert.Contains(t, script, "/dagryn-plugins/0/bin")
	assert.Contains(t, script, "/dagryn-plugins/1/bin")
}

func TestContainerExecutorWithExtraEnv(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{Image: "ubuntu:22.04"}
	exec := NewContainerExecutor(mock, "/workspace", cfg,
		WithExtraEnv(map[string]string{
			"NODE_VERSION": "20.0.0",
			"CUSTOM_VAR":   "custom_value",
		}),
	)

	tsk := &task.Task{
		Name:    "test",
		Command: "echo $NODE_VERSION",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Success, result.Status)

	// Verify extra environment was passed
	require.Len(t, mock.createCalls, 1)
	env := mock.createCalls[0].Env
	assert.Equal(t, "20.0.0", env["NODE_VERSION"])
	assert.Equal(t, "custom_value", env["CUSTOM_VAR"])
	assert.Equal(t, "test", env["DAGRYN_TASK"])
}

func TestContainerExecutorWithPluginsAndEnv(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{Image: "ubuntu:22.04"}
	exec := NewContainerExecutor(mock, "/workspace", cfg,
		WithPluginPaths([]string{"/home/user/.dagryn/plugins/node/bin"}),
		WithExtraEnv(map[string]string{
			"NODE_VERSION": "20.0.0",
		}),
	)

	tsk := &task.Task{
		Name:    "test",
		Command: "node --version",
		Env: map[string]string{
			"CUSTOM_TASK_VAR": "task_value",
		},
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Success, result.Status)

	// Verify all environment sources are merged correctly
	require.Len(t, mock.createCalls, 1)
	env := mock.createCalls[0].Env

	// System env
	assert.Equal(t, "test", env["DAGRYN_TASK"])

	// Extra env (from composite plugins)
	assert.Equal(t, "20.0.0", env["NODE_VERSION"])

	// Task env (highest priority)
	assert.Equal(t, "task_value", env["CUSTOM_TASK_VAR"])

	// Plugin paths are exported in the command (not env) to preserve image PATH
	cmd := mock.createCalls[0].Command
	require.True(t, len(cmd) >= 3, "expected sh -c <script>")
	assert.Contains(t, cmd[2], "/dagryn-plugins/0/bin")
}

func TestContainerExecutorEnvPriority(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{Image: "ubuntu:22.04"}
	exec := NewContainerExecutor(mock, "/workspace", cfg,
		WithExtraEnv(map[string]string{
			"SHARED_VAR": "from_extra",
		}),
	)

	tsk := &task.Task{
		Name:    "test",
		Command: "echo $SHARED_VAR",
		Env: map[string]string{
			"SHARED_VAR": "from_task", // Should override extra env
		},
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Success, result.Status)

	// Task env should take precedence over extra env
	require.Len(t, mock.createCalls, 1)
	env := mock.createCalls[0].Env
	assert.Equal(t, "from_task", env["SHARED_VAR"])
}

func TestJoinPaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "empty",
			paths:    []string{},
			expected: "",
		},
		{
			name:     "single path",
			paths:    []string{"/usr/bin"},
			expected: "/usr/bin",
		},
		{
			name:     "multiple paths",
			paths:    []string{"/usr/local/bin", "/usr/bin", "/bin"},
			expected: "/usr/local/bin:/usr/bin:/bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinPaths(tt.paths)
			assert.Equal(t, tt.expected, result)
		})
	}
}
