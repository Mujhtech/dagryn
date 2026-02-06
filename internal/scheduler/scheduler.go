package scheduler

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/mujhtech/dagryn/internal/cache"
	"github.com/mujhtech/dagryn/internal/dag"
	"github.com/mujhtech/dagryn/internal/executor"
	"github.com/mujhtech/dagryn/internal/plugin"
	"github.com/mujhtech/dagryn/internal/task"
)

// Options configures the scheduler behavior.
type Options struct {
	Parallelism int  // Max concurrent tasks (default: NumCPU)
	NoCache     bool // Disable caching
	NoPlugins   bool // Disable plugin installation
	FailFast    bool // Stop on first failure
	DryRun      bool // Show plan without executing
}

// DefaultOptions returns the default scheduler options.
func DefaultOptions() Options {
	return Options{
		Parallelism: runtime.NumCPU(),
		NoCache:     false,
		NoPlugins:   false,
		FailFast:    true,
		DryRun:      false,
	}
}

// RunSummary contains the results of a scheduler run.
type RunSummary struct {
	Results   []*executor.Result
	Total     time.Duration
	CacheHits int
	Failures  int
	StartTime time.Time
	EndTime   time.Time
}

// TaskCallback is called when a task starts or completes.
type TaskCallback func(taskName string, result *executor.Result, cacheHit bool)

// LogCallback is called for each line of task output.
// taskName is the name of the task producing the output.
// stream is either "stdout" or "stderr".
// line is the output line (without trailing newline).
type LogCallback func(taskName, stream, line string)

// taskState tracks the state of a task during execution.
type taskState struct {
	result   *executor.Result
	cacheKey string
	cacheHit bool
}

// Scheduler orchestrates the execution of tasks in a DAG.
type Scheduler struct {
	workflow      *task.Workflow
	graph         *dag.Graph
	executor      *executor.Executor
	cache         *cache.Cache
	pluginManager *plugin.Manager
	opts          Options
	projectRoot   string

	// Callbacks
	onTaskStart    TaskCallback
	onTaskComplete TaskCallback
	onPluginStart  PluginCallback
	onPluginDone   PluginCallback
	onLogLine      LogCallback

	// Output writers
	stdout io.Writer
	stderr io.Writer
}

// PluginCallback is called when a plugin is being installed.
type PluginCallback func(spec string, result *plugin.InstallResult)

// New creates a new scheduler.
func New(workflow *task.Workflow, projectRoot string, opts Options) (*Scheduler, error) {
	// Build the DAG from workflow
	taskDeps := make(map[string][]string)
	for _, t := range workflow.ListTasks() {
		taskDeps[t.Name] = t.Needs
	}

	g, err := dag.BuildFromWorkflow(taskDeps)
	if err != nil {
		return nil, fmt.Errorf("failed to build DAG: %w", err)
	}

	// Check for cycles
	if cycleErr := dag.DetectCycle(g); cycleErr != nil {
		return nil, cycleErr
	}

	return &Scheduler{
		workflow:      workflow,
		graph:         g,
		executor:      executor.New(projectRoot),
		cache:         cache.New(projectRoot, !opts.NoCache),
		pluginManager: plugin.NewManager(projectRoot),
		opts:          opts,
		projectRoot:   projectRoot,
	}, nil
}

// SetOutput sets the output writers for task execution.
func (s *Scheduler) SetOutput(stdout, stderr io.Writer) {
	s.stdout = stdout
	s.stderr = stderr
	s.executor = executor.New(s.projectRoot,
		executor.WithStdout(stdout),
		executor.WithStderr(stderr),
	)
}

// OnTaskStart sets the callback for when a task starts.
func (s *Scheduler) OnTaskStart(cb TaskCallback) {
	s.onTaskStart = cb
}

// OnTaskComplete sets the callback for when a task completes.
func (s *Scheduler) OnTaskComplete(cb TaskCallback) {
	s.onTaskComplete = cb
}

// OnPluginStart sets the callback for when a plugin starts installing.
func (s *Scheduler) OnPluginStart(cb PluginCallback) {
	s.onPluginStart = cb
}

// OnPluginDone sets the callback for when a plugin finishes installing.
func (s *Scheduler) OnPluginDone(cb PluginCallback) {
	s.onPluginDone = cb
}

// OnLogLine sets the callback for streaming task output line-by-line.
// The callback is invoked for each line of stdout/stderr output from tasks.
func (s *Scheduler) OnLogLine(cb LogCallback) {
	s.onLogLine = cb
}

// Run executes the specified tasks and their dependencies.
func (s *Scheduler) Run(ctx context.Context, targets []string) (*RunSummary, error) {
	summary := &RunSummary{
		Results:   make([]*executor.Result, 0),
		StartTime: time.Now(),
	}

	// Get execution plan
	plan, err := dag.TopoSortFrom(s.graph, targets)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution plan: %w", err)
	}

	if plan.TotalTasks() == 0 {
		summary.EndTime = time.Now()
		summary.Total = summary.EndTime.Sub(summary.StartTime)
		return summary, nil
	}

	// Install plugins for all tasks in the plan (if not disabled)
	if !s.opts.NoPlugins && !s.opts.DryRun {
		if err := s.installPluginsForPlan(ctx, plan); err != nil {
			return nil, fmt.Errorf("failed to install plugins: %w", err)
		}
	}

	// Track task states
	states := make(map[string]*taskState)
	statesMu := sync.Mutex{}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Process each level
	for _, level := range plan.Levels {
		// Check if context is cancelled
		if ctx.Err() != nil {
			break
		}

		// Execute tasks in this level in parallel
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, s.opts.Parallelism)
		levelResults := make(chan *taskState, len(level))

		for _, taskName := range level {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()

				// Acquire semaphore
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-ctx.Done():
					return
				}

				state := s.executeTask(ctx, name, states, &statesMu)
				levelResults <- state

				// Check for failure in fail-fast mode
				if s.opts.FailFast && state.result != nil && !state.result.IsSuccess() && state.result.Status != executor.Skipped {
					cancel()
				}
			}(taskName)
		}

		// Wait for all tasks in level to complete
		go func() {
			wg.Wait()
			close(levelResults)
		}()

		// Collect results
		for state := range levelResults {
			if state != nil && state.result != nil {
				statesMu.Lock()
				states[state.result.Task] = state
				summary.Results = append(summary.Results, state.result)
				if state.cacheHit {
					summary.CacheHits++
				}
				if !state.result.IsSuccess() && state.result.Status != executor.Skipped {
					summary.Failures++
				}
				statesMu.Unlock()
			}
		}
	}

	summary.EndTime = time.Now()
	summary.Total = summary.EndTime.Sub(summary.StartTime)

	return summary, nil
}

// RunAll executes all tasks in the workflow.
func (s *Scheduler) RunAll(ctx context.Context) (*RunSummary, error) {
	// Get all leaf tasks (tasks that nothing depends on)
	leaves := s.graph.LeafNodes()
	if len(leaves) == 0 {
		// If no leaves, run all tasks
		leaves = s.graph.NodeNames()
	}
	return s.Run(ctx, leaves)
}

// executeTask executes a single task, checking dependencies and cache.
func (s *Scheduler) executeTask(ctx context.Context, taskName string, states map[string]*taskState, statesMu *sync.Mutex) *taskState {
	t, ok := s.workflow.GetTask(taskName)
	if !ok {
		return &taskState{
			result: &executor.Result{
				Task:   taskName,
				Status: executor.Failed,
				Error:  fmt.Errorf("task %q not found", taskName),
			},
		}
	}

	// Check if any dependency failed
	statesMu.Lock()
	for _, dep := range t.Needs {
		if depState, exists := states[dep]; exists {
			if depState.result != nil && !depState.result.IsSuccess() {
				statesMu.Unlock()
				result := &executor.Result{
					Task:      taskName,
					Status:    executor.Skipped,
					Error:     fmt.Errorf("dependency %q failed", dep),
					StartTime: time.Now(),
					EndTime:   time.Now(),
				}
				if s.onTaskComplete != nil {
					s.onTaskComplete(taskName, result, false)
				}
				return &taskState{result: result}
			}
		}
	}
	statesMu.Unlock()

	// Notify task start
	if s.onTaskStart != nil {
		s.onTaskStart(taskName, nil, false)
	}

	// Check cache
	cacheHit, cacheKey, _ := s.cache.Check(t)

	if cacheHit {
		// Restore from cache
		_ = s.cache.Restore(t, cacheKey)
		result := &executor.Result{
			Task:      taskName,
			Status:    executor.Cached,
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
		if s.onTaskComplete != nil {
			s.onTaskComplete(taskName, result, true)
		}
		return &taskState{result: result, cacheKey: cacheKey, cacheHit: true}
	}

	// Dry run mode
	if s.opts.DryRun {
		result := s.executor.DryRun(t)
		if s.onTaskComplete != nil {
			s.onTaskComplete(taskName, result, false)
		}
		return &taskState{result: result, cacheKey: cacheKey}
	}

	// Get plugin paths for this task
	pluginPaths := s.getPluginPathsForTask(t)

	// Set up output writers, wrapping with LineWriter if log callback is set
	stdoutWriter, stderrWriter := s.stdout, s.stderr
	var stdoutLW, stderrLW *executor.LineWriter

	if s.onLogLine != nil {
		name := taskName // Capture for closure
		stdoutLW = executor.NewLineWriter(s.stdout, func(line string) {
			s.onLogLine(name, "stdout", line)
		})
		stderrLW = executor.NewLineWriter(s.stderr, func(line string) {
			s.onLogLine(name, "stderr", line)
		})
		stdoutWriter = stdoutLW
		stderrWriter = stderrLW
	}

	// Create executor with appropriate options
	taskExecutor := executor.New(s.projectRoot,
		executor.WithStdout(stdoutWriter),
		executor.WithStderr(stderrWriter),
		executor.WithPluginPaths(pluginPaths),
	)

	// Execute task
	result := taskExecutor.Execute(ctx, t)

	// Flush any partial lines from LineWriters
	if stdoutLW != nil {
		stdoutLW.Flush()
	}
	if stderrLW != nil {
		stderrLW.Flush()
	}

	// Save to cache on success
	if result.IsSuccess() && cacheKey != "" {
		_ = s.cache.Save(t, cacheKey, result.Duration)
	}

	if s.onTaskComplete != nil {
		s.onTaskComplete(taskName, result, false)
	}

	return &taskState{result: result, cacheKey: cacheKey}
}

// GetExecutionPlan returns the execution plan without running tasks.
func (s *Scheduler) GetExecutionPlan(targets []string) (*dag.ExecutionPlan, error) {
	if len(targets) == 0 {
		return dag.TopoSort(s.graph)
	}
	return dag.TopoSortFrom(s.graph, targets)
}

// GetGraph returns the DAG.
func (s *Scheduler) GetGraph() *dag.Graph {
	return s.graph
}

// installPluginsForPlan installs all plugins required by tasks in the execution plan.
func (s *Scheduler) installPluginsForPlan(ctx context.Context, plan *dag.ExecutionPlan) error {
	// Collect all unique plugins from tasks in the plan
	pluginSpecs := make(map[string]bool)
	for _, level := range plan.Levels {
		for _, taskName := range level {
			t, ok := s.workflow.GetTask(taskName)
			if !ok {
				continue
			}
			for _, spec := range t.Uses {
				pluginSpecs[spec] = true
			}
		}
	}

	if len(pluginSpecs) == 0 {
		return nil
	}

	// Install all plugins
	specs := make([]string, 0, len(pluginSpecs))
	for spec := range pluginSpecs {
		specs = append(specs, spec)
	}

	for _, spec := range specs {
		if s.onPluginStart != nil {
			s.onPluginStart(spec, nil)
		}

		result, err := s.pluginManager.Install(ctx, spec)
		if err != nil {
			if s.onPluginDone != nil {
				s.onPluginDone(spec, result)
			}
			return fmt.Errorf("failed to install plugin %s: %w", spec, err)
		}

		if s.onPluginDone != nil {
			s.onPluginDone(spec, result)
		}
	}

	return nil
}

// getPluginPathsForTask returns the binary directories for a task's plugins.
func (s *Scheduler) getPluginPathsForTask(t *task.Task) []string {
	if len(t.Uses) == 0 {
		return nil
	}
	return s.pluginManager.GetBinPaths(t.Uses)
}

// GetPluginManager returns the plugin manager.
func (s *Scheduler) GetPluginManager() *plugin.Manager {
	return s.pluginManager
}
