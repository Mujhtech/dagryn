package container

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/task"
)

// ContainerExecutor executes tasks inside containers.
// It implements executor.TaskExecutor.
type ContainerExecutor struct {
	runtime     Runtime
	projectRoot string
	config      *Config
	stdout      io.Writer
	stderr      io.Writer
	pluginPaths []string          // Plugin binary directories to mount
	extraEnv    map[string]string // Extra environment variables (e.g., from composite plugins)
	setupScript string            // Shell script prepended to the task command (runs inside the container)
}

// ContainerExecutorOption configures the ContainerExecutor.
type ContainerExecutorOption func(*ContainerExecutor)

// WithContainerStdout sets the stdout writer.
func WithContainerStdout(w io.Writer) ContainerExecutorOption {
	return func(e *ContainerExecutor) { e.stdout = w }
}

// WithContainerStderr sets the stderr writer.
func WithContainerStderr(w io.Writer) ContainerExecutorOption {
	return func(e *ContainerExecutor) { e.stderr = w }
}

// WithPluginPaths adds plugin binary directories to mount into the container.
func WithPluginPaths(paths []string) ContainerExecutorOption {
	return func(e *ContainerExecutor) { e.pluginPaths = paths }
}

// WithExtraEnv adds extra environment variables to the container.
func WithExtraEnv(env map[string]string) ContainerExecutorOption {
	return func(e *ContainerExecutor) { e.extraEnv = env }
}

// WithSetupScript sets a shell script that is prepended to the task command.
// This is used to run composite plugin setup steps inside the container.
func WithSetupScript(script string) ContainerExecutorOption {
	return func(e *ContainerExecutor) { e.setupScript = script }
}

// NewContainerExecutor creates a ContainerExecutor.
func NewContainerExecutor(runtime Runtime, projectRoot string, cfg *Config, opts ...ContainerExecutorOption) *ContainerExecutor {
	e := &ContainerExecutor{
		runtime:     runtime,
		projectRoot: projectRoot,
		config:      cfg,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs a task inside a container and returns the result.
func (e *ContainerExecutor) Execute(ctx context.Context, t *task.Task) *executor.Result {
	result := &executor.Result{
		Task:      t.Name,
		StartTime: time.Now(),
	}

	// Set up timeout context if specified
	if t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	}

	// Merge project-level config with task-level overrides
	containerCfg := e.config.Merge(t.Container)

	if containerCfg.Image == "" {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Status = executor.Failed
		result.Error = fmt.Errorf("no container image specified for task %q", t.Name)
		return result
	}

	// Pull image
	if err := e.runtime.Pull(ctx, containerCfg.Image); err != nil {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Status = executor.Failed
		result.Error = fmt.Errorf("pull image: %w", err)
		return result
	}

	// Resolve working directory inside container
	workDir := "/workspace"
	if t.Workdir != "" {
		workDir = "/workspace/" + t.Workdir
	}

	// Build environment
	// Priority: task env > extra env (composite) > system env
	env := make(map[string]string, len(t.Env)+len(e.extraEnv)+2)
	env["DAGRYN_TASK"] = t.Name
	env["CI"] = "true" // Standard CI indicator; tools like pnpm require it in non-TTY environments

	// Merge extra environment (from composite plugins)
	for k, v := range e.extraEnv {
		env[k] = v
	}

	// Task-specific environment takes precedence
	for k, v := range t.Env {
		env[k] = v
	}

	// Prepend the setup script (composite plugin steps) to the task command
	// so that tool installation runs inside the container. The setup script
	// uses explicit error handling (exit 1), then set -e applies to the task.
	command := t.Command
	if e.setupScript != "" {
		command = e.setupScript + "set -e\n" + command
	}

	// Add plugin paths to PATH via the shell command rather than as an env
	// var override. Setting PATH in the container env would replace the
	// image's PATH (e.g. golang image sets /usr/local/go/bin in its ENV).
	// By exporting inside the shell, $PATH inherits the image's value.
	if len(e.pluginPaths) > 0 {
		pluginPathsInContainer := make([]string, len(e.pluginPaths))
		for i := range e.pluginPaths {
			pluginPathsInContainer[i] = fmt.Sprintf("/dagryn-plugins/%d/bin", i)
		}
		command = fmt.Sprintf("export PATH=\"%s:$PATH\"\n%s",
			joinPaths(pluginPathsInContainer), command)
	}

	containerCfg.Command = []string{"sh", "-c", command}
	containerCfg.WorkDir = workDir
	containerCfg.Env = env
	containerCfg.Mounts = []Mount{
		{
			Source: e.projectRoot,
			Target: "/workspace",
		},
	}

	// Mount plugin directories (read-only)
	for i, pluginPath := range e.pluginPaths {
		containerCfg.Mounts = append(containerCfg.Mounts, Mount{
			Source:   pluginPath,
			Target:   fmt.Sprintf("/dagryn-plugins/%d/bin", i),
			ReadOnly: true,
		})
	}

	// Create container
	containerID, err := e.runtime.Create(ctx, containerCfg)
	if err != nil {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Status = executor.Failed
		result.Error = fmt.Errorf("create container: %w", err)
		return result
	}

	// Ensure cleanup on completion or cancellation.
	// Use context.Background() so cleanup completes even if the main ctx is cancelled.
	defer e.cleanup(containerID)

	// Start container
	if err := e.runtime.Start(ctx, containerID); err != nil {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Status = executor.Failed
		result.Error = fmt.Errorf("start container: %w", err)
		return result
	}

	// Stream logs in background
	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		if err := e.runtime.Logs(ctx, containerID, e.stdout, e.stderr); err != nil && ctx.Err() == nil {
			slog.Warn("container log streaming error", "container", containerID, "error", err)
		}
	}()

	// Wait for container to exit
	exitCode, waitErr := e.runtime.Wait(ctx, containerID)

	// Wait for logs to finish streaming, but don't block forever on cancellation
	select {
	case <-logsDone:
	case <-time.After(5 * time.Second):
		slog.Warn("timed out waiting for container log stream to close", "container", containerID)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.ExitCode = exitCode

	if waitErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = executor.TimedOut
			result.Error = fmt.Errorf("task timed out after %s", t.Timeout)
		} else if ctx.Err() == context.Canceled {
			result.Status = executor.Cancelled
			result.Error = fmt.Errorf("task was cancelled")
		} else {
			result.Status = executor.Failed
			result.Error = waitErr
		}
		return result
	}

	if exitCode != 0 {
		result.Status = executor.Failed
		result.Error = fmt.Errorf("container exited with code %d", exitCode)
	} else {
		result.Status = executor.Success
	}

	return result
}

// DryRun simulates container-based task execution.
func (e *ContainerExecutor) DryRun(t *task.Task) *executor.Result {
	containerCfg := e.config.Merge(t.Container)
	img := containerCfg.Image
	if img == "" {
		img = "(no image)"
	}
	return &executor.Result{
		Task:      t.Name,
		Status:    executor.Skipped,
		Output:    fmt.Sprintf("[DRY RUN] Would execute in container %s: %s", img, t.Command),
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}
}

// cleanup stops and removes a container using a background context to prevent orphans.
func (e *ContainerExecutor) cleanup(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = e.runtime.Stop(ctx, containerID, 10*time.Second)
	_ = e.runtime.Remove(ctx, containerID)
}

// joinPaths joins multiple paths with a colon separator for PATH environment variable.
func joinPaths(paths []string) string {
	result := ""
	for i, p := range paths {
		if i > 0 {
			result += ":"
		}
		result += p
	}
	return result
}
