package container

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/mujhtech/dagryn/internal/executor"
	"github.com/mujhtech/dagryn/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRuntime is a test double for the Runtime interface.
type mockRuntime struct {
	mu sync.Mutex

	availableResult bool
	pullErr         error
	createID        string
	createErr       error
	startErr        error
	waitExitCode    int
	waitErr         error
	logsOutput      string // stdout content to write as Docker-multiplexed stream
	logsErr         error
	stopErr         error
	removeErr       error

	// Tracking
	pullCalls   []string
	createCalls []ContainerConfig
	startCalls  []string
	waitCalls   []string
	stopCalls   []string
	removeCalls []string
}

func newMockRuntime() *mockRuntime {
	return &mockRuntime{
		availableResult: true,
		createID:        "mock-container-123",
	}
}

func (m *mockRuntime) Available(_ context.Context) bool {
	return m.availableResult
}

func (m *mockRuntime) Pull(_ context.Context, image string) error {
	m.mu.Lock()
	m.pullCalls = append(m.pullCalls, image)
	m.mu.Unlock()
	return m.pullErr
}

func (m *mockRuntime) Create(_ context.Context, cfg ContainerConfig) (string, error) {
	m.mu.Lock()
	m.createCalls = append(m.createCalls, cfg)
	m.mu.Unlock()
	return m.createID, m.createErr
}

func (m *mockRuntime) Start(_ context.Context, containerID string) error {
	m.mu.Lock()
	m.startCalls = append(m.startCalls, containerID)
	m.mu.Unlock()
	return m.startErr
}

func (m *mockRuntime) Wait(_ context.Context, containerID string) (int, error) {
	m.mu.Lock()
	m.waitCalls = append(m.waitCalls, containerID)
	m.mu.Unlock()
	return m.waitExitCode, m.waitErr
}

func (m *mockRuntime) Logs(_ context.Context, containerID string, stdout, stderr io.Writer) error {
	if m.logsErr != nil {
		return m.logsErr
	}
	if m.logsOutput != "" && stdout != nil {
		// Write as Docker-multiplexed stream (stdout stream type = 1)
		data := []byte(m.logsOutput)
		header := make([]byte, 8)
		header[0] = 1 // stdout stream
		binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
		_, _ = stdout.Write(header)
		_, _ = stdout.Write(data)
	}
	return nil
}

func (m *mockRuntime) Stop(_ context.Context, containerID string, _ time.Duration) error {
	m.mu.Lock()
	m.stopCalls = append(m.stopCalls, containerID)
	m.mu.Unlock()
	return m.stopErr
}

func (m *mockRuntime) Remove(_ context.Context, containerID string) error {
	m.mu.Lock()
	m.removeCalls = append(m.removeCalls, containerID)
	m.mu.Unlock()
	return m.removeErr
}

func TestContainerExecutorSuccess(t *testing.T) {
	mock := newMockRuntime()
	mock.logsOutput = "hello world\n"

	cfg := &Config{
		Enabled: true,
		Image:   "golang:1.25",
	}

	var stdout bytes.Buffer
	exec := NewContainerExecutor(mock, "/workspace", cfg,
		WithContainerStdout(&stdout),
		WithContainerStderr(io.Discard),
	)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Success, result.Status)
	assert.Equal(t, 0, result.ExitCode)
	assert.Nil(t, result.Error)
	assert.Equal(t, "build", result.Task)
	assert.True(t, result.Duration > 0)

	// Verify mock was called correctly
	require.Len(t, mock.pullCalls, 1)
	assert.Equal(t, "golang:1.25", mock.pullCalls[0])

	require.Len(t, mock.createCalls, 1)
	assert.Equal(t, "golang:1.25", mock.createCalls[0].Image)
	assert.Equal(t, []string{"sh", "-c", "go build ./..."}, mock.createCalls[0].Command)
	assert.Equal(t, "/workspace", mock.createCalls[0].WorkDir)
	assert.Equal(t, "build", mock.createCalls[0].Env["DAGRYN_TASK"])

	require.Len(t, mock.startCalls, 1)
	assert.Equal(t, "mock-container-123", mock.startCalls[0])

	// Verify cleanup was called
	require.Len(t, mock.stopCalls, 1)
	require.Len(t, mock.removeCalls, 1)
}

func TestContainerExecutorFailedExitCode(t *testing.T) {
	mock := newMockRuntime()
	mock.waitExitCode = 1

	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/workspace", cfg)

	tsk := &task.Task{
		Name:    "test",
		Command: "go test ./...",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Failed, result.Status)
	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Error.Error(), "exited with code 1")
}

func TestContainerExecutorNoImage(t *testing.T) {
	mock := newMockRuntime()
	cfg := &Config{} // No image

	exec := NewContainerExecutor(mock, "/workspace", cfg)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Failed, result.Status)
	assert.Contains(t, result.Error.Error(), "no container image")
}

func TestContainerExecutorPullError(t *testing.T) {
	mock := newMockRuntime()
	mock.pullErr = fmt.Errorf("network error")

	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/workspace", cfg)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Failed, result.Status)
	assert.Contains(t, result.Error.Error(), "pull image")
}

func TestContainerExecutorCreateError(t *testing.T) {
	mock := newMockRuntime()
	mock.createErr = fmt.Errorf("insufficient resources")

	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/workspace", cfg)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Failed, result.Status)
	assert.Contains(t, result.Error.Error(), "create container")
}

func TestContainerExecutorCancellation(t *testing.T) {
	mock := newMockRuntime()
	mock.waitErr = context.Canceled

	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/workspace", cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tsk := &task.Task{
		Name:    "build",
		Command: "sleep 60",
	}

	result := exec.Execute(ctx, tsk)

	// Should be Cancelled or Failed depending on when cancellation hits
	assert.True(t, result.Status == executor.Cancelled || result.Status == executor.Failed)
}

func TestContainerExecutorWithTaskOverrides(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{
		Image:       "golang:1.25",
		MemoryLimit: "2g",
		CPULimit:    "2.0",
		Network:     "bridge",
	}

	exec := NewContainerExecutor(mock, "/workspace", cfg)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
		Container: &task.TaskContainerConfig{
			Image:       "golang:1.25-alpine",
			MemoryLimit: "4g",
		},
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Success, result.Status)

	require.Len(t, mock.createCalls, 1)
	assert.Equal(t, "golang:1.25-alpine", mock.createCalls[0].Image)
	assert.Equal(t, int64(4*1024*1024*1024), mock.createCalls[0].MemoryLimit)
	assert.Equal(t, int64(2_000_000_000), mock.createCalls[0].CPULimit) // inherited
	assert.Equal(t, "bridge", mock.createCalls[0].Network)              // inherited
}

func TestContainerExecutorWithWorkdir(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/workspace", cfg)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
		Workdir: "cmd/app",
	}

	result := exec.Execute(context.Background(), tsk)

	assert.Equal(t, executor.Success, result.Status)
	require.Len(t, mock.createCalls, 1)
	assert.Equal(t, "/workspace/cmd/app", mock.createCalls[0].WorkDir)
}

func TestContainerExecutorMountsProjectRoot(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/my/project", cfg)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
	}

	exec.Execute(context.Background(), tsk)

	require.Len(t, mock.createCalls, 1)
	require.Len(t, mock.createCalls[0].Mounts, 1)
	assert.Equal(t, "/my/project", mock.createCalls[0].Mounts[0].Source)
	assert.Equal(t, "/workspace", mock.createCalls[0].Mounts[0].Target)
	assert.False(t, mock.createCalls[0].Mounts[0].ReadOnly)
}

func TestContainerExecutorDryRun(t *testing.T) {
	mock := newMockRuntime()

	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/workspace", cfg)

	tsk := &task.Task{
		Name:    "build",
		Command: "go build ./...",
	}

	result := exec.DryRun(tsk)

	assert.Equal(t, executor.Skipped, result.Status)
	assert.Contains(t, result.Output, "DRY RUN")
	assert.Contains(t, result.Output, "golang:1.25")
	assert.Contains(t, result.Output, "go build ./...")
}

func TestContainerExecutorImplementsTaskExecutor(t *testing.T) {
	mock := newMockRuntime()
	cfg := &Config{Image: "golang:1.25"}
	exec := NewContainerExecutor(mock, "/workspace", cfg)

	// Verify it implements the interface at compile time
	var _ executor.TaskExecutor = exec
}
