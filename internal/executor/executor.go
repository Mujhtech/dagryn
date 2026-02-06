package executor

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mujhtech/dagryn/internal/task"
)

// Status represents the result status of a task execution.
type Status int

const (
	// Success indicates the task completed successfully.
	Success Status = iota
	// Failed indicates the task failed.
	Failed
	// Cached indicates the task was skipped due to cache hit.
	Cached
	// Skipped indicates the task was skipped (e.g., dependency failed).
	Skipped
	// TimedOut indicates the task exceeded its timeout.
	TimedOut
	// Cancelled indicates the task was cancelled.
	Cancelled
)

// String returns the string representation of the status.
func (s Status) String() string {
	switch s {
	case Success:
		return "SUCCESS"
	case Failed:
		return "FAILED"
	case Cached:
		return "CACHED"
	case Skipped:
		return "SKIPPED"
	case TimedOut:
		return "TIMED_OUT"
	case Cancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

// Result represents the result of executing a task.
type Result struct {
	Task      string
	Status    Status
	Duration  time.Duration
	Output    string
	Error     error
	ExitCode  int
	StartTime time.Time
	EndTime   time.Time
}

// IsSuccess returns true if the task succeeded.
func (r *Result) IsSuccess() bool {
	return r.Status == Success || r.Status == Cached
}

// Executor executes tasks.
type Executor struct {
	projectRoot  string
	pluginPaths  []string // Additional paths to prepend to PATH for plugins
	stdoutWriter io.Writer
	stderrWriter io.Writer
}

// Option is a functional option for configuring the executor.
type Option func(*Executor)

// WithStdout sets the stdout writer for streaming output.
func WithStdout(w io.Writer) Option {
	return func(e *Executor) {
		e.stdoutWriter = w
	}
}

// WithStderr sets the stderr writer for streaming output.
func WithStderr(w io.Writer) Option {
	return func(e *Executor) {
		e.stderrWriter = w
	}
}

// WithPluginPaths sets additional paths to prepend to PATH for plugin binaries.
func WithPluginPaths(paths []string) Option {
	return func(e *Executor) {
		e.pluginPaths = paths
	}
}

// New creates a new executor.
func New(projectRoot string, opts ...Option) *Executor {
	e := &Executor{
		projectRoot: projectRoot,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs a task and returns the result.
func (e *Executor) Execute(ctx context.Context, t *task.Task) *Result {
	result := &Result{
		Task:      t.Name,
		StartTime: time.Now(),
	}

	// Set up timeout context if specified
	if t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	}

	// Resolve working directory
	workdir := e.projectRoot
	if t.Workdir != "" {
		workdir = filepath.Join(e.projectRoot, t.Workdir)
	}

	// Create output capture
	capture := NewOutputCaptureWithWriters(e.stdoutWriter, e.stderrWriter)

	// Create command
	cmd := exec.CommandContext(ctx, "sh", "-c", t.Command)
	cmd.Dir = workdir
	cmd.Env = MergeEnvWithPlugins(t.Env, e.pluginPaths)
	cmd.Stdout = capture.StdoutWriter()
	cmd.Stderr = capture.StderrWriter()

	// Execute command
	err := cmd.Run()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Output = capture.Combined()

	// Determine result status
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = TimedOut
			result.Error = fmt.Errorf("task timed out after %s", t.Timeout)
		} else if ctx.Err() == context.Canceled {
			result.Status = Cancelled
			result.Error = fmt.Errorf("task was cancelled")
		} else {
			result.Status = Failed
			result.Error = err
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			}
		}
	} else {
		result.Status = Success
		result.ExitCode = 0
	}

	return result
}

// DryRun simulates task execution without actually running the command.
func (e *Executor) DryRun(t *task.Task) *Result {
	return &Result{
		Task:      t.Name,
		Status:    Skipped,
		Duration:  0,
		Output:    fmt.Sprintf("[DRY RUN] Would execute: %s", t.Command),
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}
}
