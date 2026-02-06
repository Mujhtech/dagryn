package scheduler

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mujhtech/dagryn/internal/executor"
	"github.com/mujhtech/dagryn/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "build", Command: "echo build"})
	_ = workflow.AddTask(&task.Task{Name: "test", Command: "echo test", Needs: []string{"build"}})

	scheduler, err := New(workflow, tmpDir, DefaultOptions())
	require.NoError(t, err)
	assert.NotNil(t, scheduler)
}

func TestNew_CyclicDependency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "a", Command: "echo a", Needs: []string{"b"}})
	_ = workflow.AddTask(&task.Task{Name: "b", Command: "echo b", Needs: []string{"a"}})

	_, err = New(workflow, tmpDir, DefaultOptions())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cyclic")
}

func TestScheduler_Run_SimpleChain(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "install", Command: "echo install"})
	_ = workflow.AddTask(&task.Task{Name: "build", Command: "echo build", Needs: []string{"install"}})
	_ = workflow.AddTask(&task.Task{Name: "test", Command: "echo test", Needs: []string{"build"}})

	opts := DefaultOptions()
	opts.NoCache = true
	scheduler, err := New(workflow, tmpDir, opts)
	require.NoError(t, err)

	summary, err := scheduler.Run(context.Background(), []string{"test"})
	require.NoError(t, err)

	assert.Len(t, summary.Results, 3)
	assert.Equal(t, 0, summary.Failures)

	// Verify all tasks succeeded
	for _, result := range summary.Results {
		assert.True(t, result.IsSuccess(), "task %s should succeed", result.Task)
	}
}

func TestScheduler_Run_ParallelTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "install", Command: "echo install"})
	_ = workflow.AddTask(&task.Task{Name: "build", Command: "echo build", Needs: []string{"install"}})
	_ = workflow.AddTask(&task.Task{Name: "lint", Command: "echo lint", Needs: []string{"install"}})

	opts := DefaultOptions()
	opts.NoCache = true
	scheduler, err := New(workflow, tmpDir, opts)
	require.NoError(t, err)

	summary, err := scheduler.Run(context.Background(), []string{"build", "lint"})
	require.NoError(t, err)

	assert.Len(t, summary.Results, 3)
	assert.Equal(t, 0, summary.Failures)
}

func TestScheduler_Run_FailFast(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "fail", Command: "exit 1"})
	_ = workflow.AddTask(&task.Task{Name: "after", Command: "echo after", Needs: []string{"fail"}})

	opts := DefaultOptions()
	opts.NoCache = true
	opts.FailFast = true
	scheduler, err := New(workflow, tmpDir, opts)
	require.NoError(t, err)

	summary, err := scheduler.Run(context.Background(), []string{"after"})
	require.NoError(t, err)

	assert.True(t, summary.Failures > 0)
}

func TestScheduler_Run_DryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "build", Command: "echo should not run"})

	opts := DefaultOptions()
	opts.DryRun = true
	scheduler, err := New(workflow, tmpDir, opts)
	require.NoError(t, err)

	summary, err := scheduler.Run(context.Background(), []string{"build"})
	require.NoError(t, err)

	assert.Len(t, summary.Results, 1)
	assert.Contains(t, summary.Results[0].Output, "DRY RUN")
}

func TestScheduler_RunAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "a", Command: "echo a"})
	_ = workflow.AddTask(&task.Task{Name: "b", Command: "echo b"})

	opts := DefaultOptions()
	opts.NoCache = true
	scheduler, err := New(workflow, tmpDir, opts)
	require.NoError(t, err)

	summary, err := scheduler.RunAll(context.Background())
	require.NoError(t, err)

	assert.Len(t, summary.Results, 2)
}

func TestScheduler_Callbacks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "build", Command: "echo build"})

	opts := DefaultOptions()
	opts.NoCache = true
	scheduler, err := New(workflow, tmpDir, opts)
	require.NoError(t, err)

	var startCalled, completeCalled bool
	scheduler.OnTaskStart(func(name string, result *executor.Result, cacheHit bool) {
		startCalled = true
		assert.Equal(t, "build", name)
	})
	scheduler.OnTaskComplete(func(name string, result *executor.Result, cacheHit bool) {
		completeCalled = true
		assert.Equal(t, "build", name)
		assert.True(t, result.IsSuccess())
	})

	_, err = scheduler.Run(context.Background(), []string{"build"})
	require.NoError(t, err)

	assert.True(t, startCalled)
	assert.True(t, completeCalled)
}

func TestScheduler_GetExecutionPlan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "install", Command: "echo install"})
	_ = workflow.AddTask(&task.Task{Name: "build", Command: "echo build", Needs: []string{"install"}})
	_ = workflow.AddTask(&task.Task{Name: "test", Command: "echo test", Needs: []string{"build"}})

	scheduler, err := New(workflow, tmpDir, DefaultOptions())
	require.NoError(t, err)

	plan, err := scheduler.GetExecutionPlan([]string{"test"})
	require.NoError(t, err)

	assert.Equal(t, 3, plan.TotalTasks())
}

func TestScheduler_Cancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := task.NewWorkflow("ci", nil)
	_ = workflow.AddTask(&task.Task{Name: "slow", Command: "sleep 10"})

	opts := DefaultOptions()
	opts.NoCache = true
	scheduler, err := New(workflow, tmpDir, opts)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	summary, err := scheduler.Run(ctx, []string{"slow"})
	require.NoError(t, err)

	// Task should be cancelled
	if len(summary.Results) > 0 {
		// It might be cancelled or still running
		assert.True(t, summary.Results[0].Status == executor.Cancelled ||
			summary.Results[0].Error != nil)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.True(t, opts.Parallelism > 0)
	assert.False(t, opts.NoCache)
	assert.True(t, opts.FailFast)
	assert.False(t, opts.DryRun)
}
