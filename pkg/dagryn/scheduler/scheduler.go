package scheduler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mujhtech/dagryn/pkg/dagryn/cache"
	"github.com/mujhtech/dagryn/pkg/dagryn/condition"
	"github.com/mujhtech/dagryn/pkg/dagryn/container"
	"github.com/mujhtech/dagryn/pkg/dagryn/dag"
	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/plugin"
	"github.com/mujhtech/dagryn/pkg/dagryn/task"
)

// Options configures the scheduler behavior.
type Options struct {
	Parallelism      int                // Max concurrent tasks (default: NumCPU)
	NoCache          bool               // Disable caching
	NoPlugins        bool               // Disable plugin installation
	FailFast         bool               // Stop on first failure
	DryRun           bool               // Show plan without executing
	CacheBackend     cache.Backend      // Optional custom cache backend
	ContainerConfig  *container.Config  // Optional container isolation config
	ConditionContext *condition.Context // Optional context for evaluating task conditions
	GlobalPlugins    map[string]string  // Global plugins (name -> spec) from config [plugins] section
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
	result           *executor.Result
	cacheKey         string
	cacheHit         bool
	conditionSkipped bool // true when task was skipped due to unmet condition
}

// Scheduler orchestrates the execution of tasks in a DAG.
type Scheduler struct {
	workflow          *task.Workflow
	graph             *dag.Graph
	executor          executor.TaskExecutor
	cache             *cache.Cache
	pluginManager     *plugin.Manager
	compositeExecutor *plugin.CompositeExecutor
	opts              Options
	projectRoot       string

	// Background cache saves
	cacheSaveWg sync.WaitGroup

	// Integration plugin hooks
	integrationRegistry *plugin.IntegrationRegistry

	// Container isolation
	containerRuntime container.Runtime
	containerConfig  *container.Config

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

type compositeCleanupTask struct {
	manifest *plugin.Manifest
	setup    *plugin.CompositeSetupResult
	workdir  string
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

	var c *cache.Cache
	if opts.CacheBackend != nil {
		c = cache.NewWithBackend(projectRoot, !opts.NoCache, opts.CacheBackend)
	} else {
		c = cache.New(projectRoot, !opts.NoCache)
	}

	hookExecutor := plugin.NewHookExecutor(nil)
	integrationRegistry := plugin.NewIntegrationRegistry(hookExecutor)

	s := &Scheduler{
		workflow:            workflow,
		graph:               g,
		executor:            executor.New(projectRoot),
		cache:               c,
		pluginManager:       plugin.NewManager(projectRoot),
		compositeExecutor:   plugin.NewCompositeExecutor(projectRoot, nil),
		integrationRegistry: integrationRegistry,
		opts:                opts,
		projectRoot:         projectRoot,
	}

	// Initialize container runtime if configured
	if opts.ContainerConfig != nil && opts.ContainerConfig.Enabled {
		rt, err := container.NewDockerRuntime()
		if err != nil {
			slog.Warn("container runtime not available, falling back to host execution", "error", err)
		} else if !rt.Available(context.Background()) {
			slog.Warn("container runtime not reachable, falling back to host execution")
			_ = rt.Close()
		} else {
			s.containerRuntime = rt
			s.containerConfig = opts.ContainerConfig
			slog.Info("container isolation enabled", "image", opts.ContainerConfig.Image)
		}
	}

	return s, nil
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

	// Register integration plugins from global config
	if !s.opts.DryRun {
		s.registerIntegrationPlugins(ctx)
	}

	// Track task states
	states := make(map[string]*taskState)
	statesMu := sync.Mutex{}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Dispatch on_run_start hook
	hookCtx := &plugin.HookContext{
		ProjectRoot: s.projectRoot,
	}
	s.integrationRegistry.DispatchHook(ctx, plugin.HookOnRunStart, hookCtx)

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

	// Dispatch on_run_success or on_run_failure, then on_run_end
	endHookCtx := &plugin.HookContext{
		ProjectRoot: s.projectRoot,
	}
	if summary.Failures > 0 {
		endHookCtx.RunStatus = "failed"
		s.integrationRegistry.DispatchHook(ctx, plugin.HookOnRunFailure, endHookCtx)
	} else {
		endHookCtx.RunStatus = "success"
		s.integrationRegistry.DispatchHook(ctx, plugin.HookOnRunSuccess, endHookCtx)
	}
	s.integrationRegistry.DispatchHook(ctx, plugin.HookOnRunEnd, endHookCtx)

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

// WaitCacheSaves blocks until all background cache save goroutines complete.
// Callers should invoke this before cancelling the context that was passed to
// Run, since saves need a live context to upload to remote backends.
func (s *Scheduler) WaitCacheSaves() {
	s.cacheSaveWg.Wait()
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

	// Check if any dependency failed (condition-skipped deps are not failures)
	statesMu.Lock()
	for _, dep := range t.Needs {
		if depState, exists := states[dep]; exists {
			if depState.result != nil && !depState.result.IsSuccess() && !depState.conditionSkipped {
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

	// Evaluate task condition before running
	if t.If != "" && s.opts.ConditionContext != nil {
		matched, err := condition.Evaluate(t.If, s.opts.ConditionContext)
		if err != nil {
			slog.Warn("condition eval failed, running task anyway", "task", taskName, "error", err)
		} else if !matched {
			result := &executor.Result{
				Task:      taskName,
				Status:    executor.Skipped,
				StartTime: time.Now(),
				EndTime:   time.Now(),
			}
			if s.onTaskStart != nil {
				s.onTaskStart(taskName, nil, false)
			}
			if s.onTaskComplete != nil {
				s.onTaskComplete(taskName, result, false)
			}
			return &taskState{result: result, conditionSkipped: true}
		}
	}

	// Notify task start
	if s.onTaskStart != nil {
		s.onTaskStart(taskName, nil, false)
	}

	// Dispatch on_task_start hook
	taskHookCtx := &plugin.HookContext{
		TaskName:    taskName,
		ProjectRoot: s.projectRoot,
	}
	s.integrationRegistry.DispatchHook(ctx, plugin.HookOnTaskStart, taskHookCtx)

	// Check cache
	cacheHit, cacheKey, cacheErr := s.cache.Check(ctx, t)
	if cacheErr != nil {
		slog.Warn("cache check failed, proceeding without cache",
			"task", taskName, "key", cacheKey, "error", cacheErr)
	}

	if cacheHit {
		// Restore from cache
		if err := s.cache.Restore(ctx, t, cacheKey); err != nil {
			slog.Warn("cache restore failed, re-executing task",
				"task", taskName, "key", cacheKey, "error", err)
			// Fall through to execution instead of returning Cached
		} else {
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
	}

	// Dry run mode
	if s.opts.DryRun {
		result := s.executor.DryRun(t)
		if s.onTaskComplete != nil {
			s.onTaskComplete(taskName, result, false)
		}
		return &taskState{result: result, cacheKey: cacheKey}
	}

	// Check if this is a composite task
	if t.IsComposite() {
		return s.executeCompositeTask(ctx, t, cacheKey)
	}

	// Get plugin paths for this task
	pluginPaths := s.getPluginPathsForTask(t)

	// Run composite plugin setup steps before the task command.
	// This handles tasks like web-install that have both uses=["setup-node"] and a command.
	compositeEnv, compositeCleanupTasks := s.runCompositeSetup(ctx, t)

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

	// Choose executor: container (if available and configured) or host
	var taskExec executor.TaskExecutor
	if s.containerRuntime != nil && s.containerConfig != nil {
		taskExec = container.NewContainerExecutor(
			s.containerRuntime, s.projectRoot, s.containerConfig,
			container.WithContainerStdout(stdoutWriter),
			container.WithContainerStderr(stderrWriter),
			container.WithPluginPaths(pluginPaths),
			container.WithExtraEnv(compositeEnv),
		)
	} else {
		taskExec = executor.New(s.projectRoot,
			executor.WithStdout(stdoutWriter),
			executor.WithStderr(stderrWriter),
			executor.WithPluginPaths(pluginPaths),
			executor.WithExtraEnv(compositeEnv),
		)
	}

	// Execute task
	result := taskExec.Execute(ctx, t)

	// Run composite cleanup after the task command (post behavior).
	s.runCompositeCleanup(compositeCleanupTasks)

	// Flush any partial lines from LineWriters
	if stdoutLW != nil {
		stdoutLW.Flush()
	}
	if stderrLW != nil {
		stderrLW.Flush()
	}

	// Save to cache on success (non-blocking — downstream tasks share data
	// via the filesystem, not via cache, so the next DAG level can start
	// immediately). Uses a detached context because Run()'s defer cancel()
	// fires before WaitCacheSaves() is called by the caller.
	if result.IsSuccess() && cacheKey != "" {
		s.cacheSaveWg.Add(1)
		go func() {
			defer s.cacheSaveWg.Done()
			saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer saveCancel()
			if err := s.cache.Save(saveCtx, t, cacheKey, result.Duration); err != nil {
				slog.Warn("cache save failed", "task", taskName, "key", cacheKey, "error", err)
			}
		}()
	}

	if s.onTaskComplete != nil {
		s.onTaskComplete(taskName, result, false)
	}

	// Dispatch on_task_end hook
	taskEndHookCtx := &plugin.HookContext{
		TaskName:       taskName,
		TaskDurationMs: result.Duration.Milliseconds(),
		ProjectRoot:    s.projectRoot,
	}
	if result.IsSuccess() {
		taskEndHookCtx.TaskStatus = "success"
	} else {
		taskEndHookCtx.TaskStatus = "failed"
	}
	s.integrationRegistry.DispatchHook(ctx, plugin.HookOnTaskEnd, taskEndHookCtx)

	return &taskState{result: result, cacheKey: cacheKey}
}

// executeCompositeTask handles execution of composite plugin tasks.
func (s *Scheduler) executeCompositeTask(ctx context.Context, t *task.Task, cacheKey string) *taskState {
	startTime := time.Now()

	// Resolve the plugin to get its manifest
	spec := t.Uses[0]
	resolved, err := s.pluginManager.Resolve(ctx, spec)
	if err != nil {
		result := &executor.Result{
			Task:      t.Name,
			Status:    executor.Failed,
			Error:     fmt.Errorf("failed to resolve composite plugin %s: %w", spec, err),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}
		if s.onTaskComplete != nil {
			s.onTaskComplete(t.Name, result, false)
		}
		return &taskState{result: result, cacheKey: cacheKey}
	}

	if resolved.Manifest == nil || !resolved.Manifest.IsComposite() {
		result := &executor.Result{
			Task:      t.Name,
			Status:    executor.Failed,
			Error:     fmt.Errorf("plugin %s is not a composite plugin", spec),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}
		if s.onTaskComplete != nil {
			s.onTaskComplete(t.Name, result, false)
		}
		return &taskState{result: result, cacheKey: cacheKey}
	}

	// Determine working directory
	workdir := s.projectRoot
	if t.Workdir != "" {
		workdir = filepath.Join(s.projectRoot, t.Workdir)
	}

	// Execute the composite steps
	err = s.compositeExecutor.Execute(ctx, resolved.Manifest, t.With, t.Env, workdir)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	var result *executor.Result
	if err != nil {
		result = &executor.Result{
			Task:      t.Name,
			Status:    executor.Failed,
			Error:     err,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
		}
	} else {
		result = &executor.Result{
			Task:      t.Name,
			Status:    executor.Success,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
		}

		// Save to cache on success (non-blocking). Uses a detached context
		// because Run()'s defer cancel() fires before WaitCacheSaves().
		if cacheKey != "" {
			s.cacheSaveWg.Add(1)
			go func() {
				defer s.cacheSaveWg.Done()
				saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer saveCancel()
				if err := s.cache.Save(saveCtx, t, cacheKey, duration); err != nil {
					slog.Warn("cache save failed", "task", t.Name, "key", cacheKey, "error", err)
				}
			}()
		}
	}

	if s.onTaskComplete != nil {
		s.onTaskComplete(t.Name, result, false)
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

		// Resolve the plugin first to check if it's composite
		resolved, err := s.pluginManager.Resolve(ctx, spec)
		if err != nil {
			if s.onPluginDone != nil {
				s.onPluginDone(spec, nil)
			}
			return fmt.Errorf("failed to resolve plugin %s: %w", spec, err)
		}

		// Integration plugins are handled separately via registerIntegrationPlugins
		if resolved.Manifest != nil && resolved.Manifest.IsIntegration() {
			s.pluginManager.Register(spec, resolved)
			if s.onPluginDone != nil {
				s.onPluginDone(spec, &plugin.InstallResult{
					Plugin:  resolved,
					Status:  plugin.StatusInstalled,
					Message: fmt.Sprintf("Resolved integration plugin %s", resolved.Name),
				})
			}
			continue
		}

		// Composite plugins don't need binary installation but still need registration
		if resolved.Manifest != nil && resolved.Manifest.IsComposite() {
			s.pluginManager.Register(spec, resolved)
			if s.onPluginDone != nil {
				s.onPluginDone(spec, &plugin.InstallResult{
					Plugin:  resolved,
					Status:  plugin.StatusInstalled,
					Message: fmt.Sprintf("Resolved composite plugin %s", resolved.Name),
				})
			}
			continue
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

// runCompositeSetup runs composite plugin setup steps for a task that has both
// a command and composite plugin uses. Returns collected environment variables
// from the composite steps (e.g., PATH modifications).
func (s *Scheduler) runCompositeSetup(ctx context.Context, t *task.Task) (map[string]string, []compositeCleanupTask) {
	if len(t.Uses) == 0 {
		return nil, nil
	}

	env := make(map[string]string)
	cleanupTasks := make([]compositeCleanupTask, 0)

	workdir := s.projectRoot
	if t.Workdir != "" {
		workdir = filepath.Join(s.projectRoot, t.Workdir)
	}

	for _, spec := range t.Uses {
		resolved, err := s.pluginManager.Resolve(ctx, spec)
		if err != nil || resolved.Manifest == nil || !resolved.Manifest.IsComposite() {
			continue
		}

		// Execute only setup steps now; run cleanup after task command.
		setup, err := s.compositeExecutor.ExecuteSetup(ctx, resolved.Manifest, t.With, t.Env, workdir)
		if err != nil {
			// Log but don't fail — the task command will likely fail with a clear error
			continue
		}
		cleanupTasks = append(cleanupTasks, compositeCleanupTask{
			manifest: resolved.Manifest,
			setup:    setup,
			workdir:  workdir,
		})

		// Collect environment variables from composite steps
		stepEnv := s.compositeExecutor.CollectStepEnv(resolved.Manifest, t.With)
		for k, v := range stepEnv {
			env[k] = v
		}
	}

	if len(env) == 0 {
		return nil, cleanupTasks
	}
	return env, cleanupTasks
}

func (s *Scheduler) runCompositeCleanup(tasks []compositeCleanupTask) {
	for _, t := range tasks {
		s.compositeExecutor.RunCleanup(t.manifest, t.setup, t.workdir)
	}
}

// registerIntegrationPlugins resolves global plugins and registers those with
// type "integration" into the integration registry for hook dispatch.
func (s *Scheduler) registerIntegrationPlugins(ctx context.Context) {
	if s.opts.GlobalPlugins == nil || s.opts.NoPlugins {
		return
	}

	for name, spec := range s.opts.GlobalPlugins {
		resolved, err := s.pluginManager.Resolve(ctx, spec)
		if err != nil {
			slog.Warn("failed to resolve global plugin, skipping", "plugin", name, "spec", spec, "error", err)
			continue
		}
		if resolved.Manifest == nil || !resolved.Manifest.IsIntegration() {
			continue
		}

		// Collect inputs from the manifest defaults
		inputs := make(map[string]string)
		for k, v := range resolved.Manifest.Inputs {
			if v.Default != "" {
				inputs[k] = v.Default
			}
		}

		s.integrationRegistry.Register(plugin.IntegrationPlugin{
			Name:     name,
			Manifest: resolved.Manifest,
			Inputs:   inputs,
		})
		slog.Info("registered integration plugin", "name", name)
	}
}

// GetPluginManager returns the plugin manager.
func (s *Scheduler) GetPluginManager() *plugin.Manager {
	return s.pluginManager
}
