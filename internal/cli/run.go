package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache/cloud"
	grpccache "github.com/mujhtech/dagryn/pkg/dagryn/cache/grpc"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache/remote"
	"github.com/mujhtech/dagryn/pkg/dagryn/condition"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/plugin"
	"github.com/mujhtech/dagryn/pkg/dagryn/scheduler"
	"github.com/mujhtech/dagryn/pkg/dagryn/task"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [task...]",
	Short: "Run one or more tasks",
	Long: `Run one or more tasks and their dependencies.

If no tasks are specified and a default workflow is defined, all tasks in the workflow will be run.

Examples:
  dagryn run build          # Run the 'build' task and its dependencies
  dagryn run test lint      # Run 'test' and 'lint' tasks
  dagryn run                # Run the default workflow
  dagryn run --sync build   # Run locally and sync status to remote server`,
	RunE: runRun,
}

var (
	parallel   int
	dryRun     bool
	syncRemote bool
	projectID  string
)

func init() {
	runCmd.Flags().IntVarP(&parallel, "parallel", "p", 0, "max parallel tasks (default: number of CPUs)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show execution plan without running")
	runCmd.Flags().BoolVar(&syncRemote, "sync", false, "sync run status to remote server")
	runCmd.Flags().StringVar(&projectID, "project", "", "project ID for remote sync (required with --sync)")
}

func runRun(cmd *cobra.Command, args []string) error {
	log := logger.New(verbose)

	// Get project root
	projectRoot, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	// Load config
	cfg, err := config.Parse(cfgFile)
	if err != nil {
		return err
	}

	// Validate config
	if errors := config.Validate(cfg); len(errors) > 0 {
		for _, e := range errors {
			log.Error("validation error", fmt.Errorf("%s", e.Error()))
		}
		return fmt.Errorf("configuration validation failed")
	}

	// Convert to workflow
	workflow, err := cfg.ToWorkflow()
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	// Determine targets
	targets := args
	if len(targets) == 0 {
		if workflow.Default {
			// Run all leaf tasks (tasks that are not dependencies of other tasks)
			targets = workflow.TaskNames()
		} else {
			return fmt.Errorf("no tasks specified and no default workflow. Use 'dagryn run <task>' or set default = true in [workflow]")
		}
	}

	// Resolve group names to task names
	if len(targets) > 0 {
		targets = workflow.ResolveTargets(targets)
	}

	// Verify targets exist
	for _, target := range targets {
		if _, ok := workflow.GetTask(target); !ok {
			return fmt.Errorf("task %q not found", target)
		}
	}

	// Set up remote sync if enabled
	var remoteSync *RemoteSync
	if syncRemote {
		sync, err := setupRemoteSync(projectRoot, targets)
		if err != nil {
			return err
		}
		remoteSync = sync
		log.Info(fmt.Sprintf("Remote sync enabled (run ID: %s)", remoteSync.RunID))
	}

	// Create scheduler options
	opts := scheduler.DefaultOptions()
	opts.NoCache = noCache || !cfg.Cache.IsEnabled()
	opts.NoPlugins = noPlugins
	opts.DryRun = dryRun
	if parallel > 0 {
		opts.Parallelism = parallel
	}

	// Build cache backend from config
	opts.CacheBackend = buildCacheBackend(cfg.Cache, projectRoot, log)

	// Pass global plugins to scheduler for integration hook dispatch
	if len(cfg.Plugins) > 0 {
		opts.GlobalPlugins = cfg.Plugins
	}

	// Build condition context for task conditions
	opts.ConditionContext = &condition.Context{
		Branch:  getGitBranch(),
		Event:   "cli",
		Trigger: "cli",
	}

	// Create scheduler
	sched, err := scheduler.New(workflow, projectRoot, opts)
	if err != nil {
		return err
	}

	// Set up output
	sched.SetOutput(os.Stdout, os.Stderr)

	// Set up callbacks
	sched.OnTaskStart(func(name string, result *executor.Result, cacheHit bool) {
		log.TaskStart(name)
		if remoteSync != nil {
			remoteSync.OnTaskStart(name, cacheHit)
		}
	})
	sched.OnTaskComplete(func(name string, result *executor.Result, cacheHit bool) {
		log.TaskEnd(name, result, cacheHit)
		if remoteSync != nil {
			remoteSync.OnTaskComplete(name, result, cacheHit)
		}
	})
	sched.OnPluginStart(func(spec string, result *plugin.InstallResult) {
		log.PluginStart(spec)
	})
	sched.OnPluginDone(func(spec string, result *plugin.InstallResult) {
		log.PluginDone(spec, result)
	})

	// Set up log streaming callback for real-time logs
	if remoteSync != nil {
		sched.OnLogLine(func(taskName, stream, line string) {
			remoteSync.AppendLog(taskName, stream, line)
		})
	}

	// Show execution plan in dry-run mode
	if dryRun {
		plan, err := sched.GetExecutionPlan(targets)
		if err != nil {
			return err
		}
		log.PrintPlan(plan.Levels)
		log.Info("\n[DRY RUN] No tasks were executed.")
		return nil
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Allow remote cancellation to stop the local scheduler
	if remoteSync != nil {
		remoteSync.SetCancelFunc(cancel)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Info("\nInterrupted. Cancelling tasks...")
		cancel()
		if remoteSync != nil {
			remoteSync.OnCancelled()
		}
	}()

	// Notify remote sync that run is starting
	if remoteSync != nil {
		remoteSync.OnRunStart()
	}

	// Run tasks — returns as soon as all tasks finish; cache saves may still
	// be running in the background.
	summary, err := sched.Run(ctx, targets)

	// Report run status first so the user sees the result immediately.
	// Skip if remotely cancelled — the server already has the correct status,
	// and the heartbeat goroutine updated the local record.
	if remoteSync != nil && !remoteSync.IsRemoteCancelled() {
		if err != nil {
			remoteSync.OnRunFailed(err)
		} else if summary != nil && summary.Failures > 0 {
			remoteSync.OnRunFailed(fmt.Errorf("%d task(s) failed", summary.Failures))
		} else {
			remoteSync.OnRunComplete(summary)
		}
	}

	if err != nil {
		// Wait for cache saves before cancelling the context.
		sched.WaitCacheSaves()
		if remoteSync != nil {
			remoteSync.Stop()
		}
		return err
	}

	// Print summary immediately so the user sees the result.
	log.Summary(summary.Results, summary.Total, summary.CacheHits)

	// Wait for background cache saves to complete (context must stay alive).
	sched.WaitCacheSaves()

	// Collect artifacts after status reporting. Stays synchronous so workDir
	// and remote sync resources are still available.
	if remoteSync != nil && summary != nil && len(summary.Results) > 0 {
		remoteSync.CollectArtifacts(workflow, summary, projectRoot)
	}

	// Stop periodic flusher and close resources
	if remoteSync != nil {
		remoteSync.Stop()
	}

	// Exit with error if any tasks failed
	if summary.Failures > 0 {
		return fmt.Errorf("%d task(s) failed", summary.Failures)
	}

	return nil
}

// RemoteSync handles syncing run status to the remote server.
type RemoteSync struct {
	client      *client.Client
	projectID   uuid.UUID
	RunID       uuid.UUID
	creds       *client.Credentials
	totalTasks  int
	logBuffer   []client.LogEntry
	logMu       sync.Mutex
	offline     bool      // true if we've detected we're offline
	offlineOnce sync.Once // ensures we only print offline message once
	errorCount  int       // count of consecutive errors
	maxErrors   int       // max errors before giving up on sync
	maxLogBuf   int       // max log entries to buffer before discarding oldest

	// Local run storage for offline fallback
	runStore     *client.RunStore
	projectRoot  string
	taskLineNums map[string]int // per-task line counters

	// Cancellation from remote
	cancelFunc      context.CancelFunc
	remoteCancelled atomic.Bool

	// Periodic flush
	flushTicker    *time.Ticker
	done           chan struct{}
	lastFlushError time.Time // tracks last flush failure for backoff
}

// setupRemoteSync creates a remote sync handler.
func setupRemoteSync(projectRoot string, targets []string) (*RemoteSync, error) {
	// Load credentials
	store, err := client.NewCredentialsStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	if creds == nil {
		return nil, fmt.Errorf("not logged in. Run 'dagryn auth login' first")
	}

	// Determine project ID - from flag or from .dagryn/project.json
	var projID uuid.UUID
	if projectID != "" {
		// Use explicit flag
		id, err := uuid.Parse(projectID)
		if err != nil {
			return nil, fmt.Errorf("invalid project ID: %w", err)
		}
		projID = id
	} else {
		// Try to load from .dagryn/project.json
		projectStore := client.NewProjectConfigStore(projectRoot)
		projectConfig, err := projectStore.Load()
		if err != nil {
			return nil, fmt.Errorf("failed to load project config: %w", err)
		}
		if projectConfig == nil {
			return nil, fmt.Errorf("no project linked. Run 'dagryn init --remote' first, or use --project flag")
		}
		projID = projectConfig.ProjectID
		fmt.Printf("Using linked project: %s (%s)\n", projectConfig.ProjectName, projectConfig.ProjectID)
	}

	// Create client
	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 30 * time.Second,
	})

	// Refresh token if expired
	if creds.IsExpired() {
		tokens, err := apiClient.RefreshToken(context.Background(), creds.RefreshToken)
		if err != nil {
			// Check if it's a network error
			if client.IsNetworkError(err) {
				return nil, fmt.Errorf("cannot connect to server at %s: %w", creds.ServerURL, err)
			}
			return nil, fmt.Errorf("session expired, please login again: %w", err)
		}
		creds.AccessToken = tokens.Data.AccessToken
		creds.RefreshToken = tokens.Data.RefreshToken
		creds.ExpiresAt = tokens.Data.ExpiresAt
		if err := store.Save(creds); err != nil {
			// Non-fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: Failed to save refreshed credentials: %v\n", err)
		}
	}

	apiClient.SetCredentials(creds)
	apiClient.SetCredentialsStore(store)

	// Sync workflow to remote before triggering the run
	// This ensures the remote has the latest workflow definition
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := syncWorkflowToRemote(syncCtx, apiClient, projectRoot, projID); err != nil {
		// Non-fatal warning - continue with run even if workflow sync fails
		fmt.Fprintf(os.Stderr, "Warning: Failed to sync workflow: %v\n", err)
	}
	syncCancel()

	// Get git info
	gitBranch := getGitBranch()
	gitCommit := getGitCommit()

	// Trigger run on server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hostname, _ := os.Hostname()

	resp, err := apiClient.TriggerRun(ctx, projID, client.TriggerRunRequest{
		Targets:   targets,
		GitBranch: gitBranch,
		GitCommit: gitCommit,
		SyncOnly:  true, // CLI executes locally, only sync status to server
		HostOS:    runtime.GOOS,
		HostArch:  runtime.GOARCH,
		HostName:  hostname,
	})
	if err != nil {
		// Provide helpful error messages based on error type
		if client.IsNetworkError(err) {
			return nil, fmt.Errorf("cannot connect to server at %s\n\nPlease check:\n  - Your network connection\n  - The server is running\n  - The server URL is correct\n\nTo run without sync, omit the --sync flag", creds.ServerURL)
		}
		if client.IsAuthError(err) {
			return nil, fmt.Errorf("authentication failed: %s\n\nPlease run 'dagryn auth login' to re-authenticate", client.UserFriendlyError(err))
		}
		return nil, fmt.Errorf("failed to create remote run: %w", err)
	}

	// Create local run store for offline fallback
	runStore := client.NewRunStore(projectRoot)

	// Create local run record
	localRun := &client.LocalRun{
		RunID:       resp.Data.RunID,
		ProjectID:   projID,
		ServerURL:   creds.ServerURL,
		Targets:     targets,
		GitBranch:   gitBranch,
		GitCommit:   gitCommit,
		StartedAt:   time.Now(),
		Status:      "running",
		PendingSync: false,
	}

	if err := runStore.CreateRun(localRun); err != nil {
		// Non-fatal, just warn - we can still sync to remote
		fmt.Fprintf(os.Stderr, "Warning: Failed to create local run record: %v\n", err)
	}

	rs := &RemoteSync{
		client:       apiClient,
		projectID:    projID,
		RunID:        resp.Data.RunID,
		creds:        creds,
		logBuffer:    make([]client.LogEntry, 0, 100),
		maxErrors:    5,     // Give up on sync after 5 consecutive failures
		maxLogBuf:    10000, // Cap buffer at 10k entries to prevent unbounded growth
		runStore:     runStore,
		projectRoot:  projectRoot,
		taskLineNums: make(map[string]int),
		done:         make(chan struct{}),
	}

	// Start periodic log flusher and heartbeat poller
	rs.startPeriodicFlush(2 * time.Second)
	rs.startHeartbeat(5 * time.Second)

	return rs, nil
}

// OnRunStart is called when the run starts.
func (s *RemoteSync) OnRunStart() {
	if s.isOffline() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.client.UpdateRunStatus(ctx, s.projectID, s.RunID, "running", nil, nil); err != nil {
		s.handleSyncError("update run status", err)
	} else {
		s.resetErrorCount()
	}
}

// SetTotalTasks sets the total number of tasks for the run.
func (s *RemoteSync) SetTotalTasks(total int) {
	s.totalTasks = total

	if s.isOffline() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.client.UpdateRunStatus(ctx, s.projectID, s.RunID, "running", &total, nil); err != nil {
		s.handleSyncError("update total tasks", err)
	} else {
		s.resetErrorCount()
	}
}

// OnTaskStart is called when a task starts.
func (s *RemoteSync) OnTaskStart(name string, cacheHit bool) {
	if s.isOffline() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create task result on server
	if err := s.client.CreateTask(ctx, s.projectID, s.RunID, name); err != nil {
		s.handleSyncError(fmt.Sprintf("create task %s", name), err)
		return
	}

	// Update status to running
	status := "running"
	if cacheHit {
		status = "cached"
	}
	now := time.Now()
	if err := s.client.UpdateTaskStatus(ctx, s.projectID, s.RunID, name, status, nil, nil, cacheHit, "", &now, nil); err != nil {
		s.handleSyncError(fmt.Sprintf("update task %s status", name), err)
	} else {
		s.resetErrorCount()
	}
}

// OnTaskComplete is called when a task completes.
func (s *RemoteSync) OnTaskComplete(name string, result *executor.Result, cacheHit bool) {
	if s.isOffline() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Determine status based on result
	var status string
	switch result.Status {
	case executor.Success:
		status = "success"
	case executor.Failed:
		status = "failed"
	case executor.Cached:
		status = "cached"
	case executor.Skipped:
		status = "skipped"
	case executor.Cancelled:
		status = "cancelled"
	default:
		status = "failed"
	}

	// Convert duration to milliseconds
	durationMs := result.Duration.Milliseconds()
	exitCode := result.ExitCode

	// Send accurate client-side timestamps
	var startedAt, finishedAt *time.Time
	if !result.StartTime.IsZero() {
		startedAt = &result.StartTime
	}
	if !result.EndTime.IsZero() {
		finishedAt = &result.EndTime
	}

	if err := s.client.UpdateTaskStatus(ctx, s.projectID, s.RunID, name, status, &exitCode, &durationMs, cacheHit, "", startedAt, finishedAt); err != nil {
		s.handleSyncError(fmt.Sprintf("update task %s completion", name), err)
	} else {
		s.resetErrorCount()
	}
}

// OnRunComplete is called when the run completes successfully.
func (s *RemoteSync) OnRunComplete(summary *scheduler.RunSummary) {
	// Update local run record
	s.updateLocalRunStatus("success", "")

	// Flush any remaining logs
	s.flushLogs()

	if s.isOffline() {
		fmt.Fprintf(os.Stderr, "\nNote: Run completed locally but could not sync final status to server.\n")
		fmt.Fprintf(os.Stderr, "Logs stored in: .dagryn/runs/%s/\n", s.RunID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.client.UpdateRunStatus(ctx, s.projectID, s.RunID, "success", nil, nil); err != nil {
		s.handleSyncError("update run completion status", err)
	}
}

// OnRunFailed is called when the run fails.
func (s *RemoteSync) OnRunFailed(err error) {
	// Update local run record
	s.updateLocalRunStatus("failed", err.Error())

	// Flush any remaining logs
	s.flushLogs()

	if s.isOffline() {
		fmt.Fprintf(os.Stderr, "\nNote: Run failed locally but could not sync failure status to server.\n")
		fmt.Fprintf(os.Stderr, "Logs stored in: .dagryn/runs/%s/\n", s.RunID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errorMsg := err.Error()
	if updateErr := s.client.UpdateRunStatus(ctx, s.projectID, s.RunID, "failed", nil, &errorMsg); updateErr != nil {
		s.handleSyncError("update run failure status", updateErr)
	}
}

// OnCancelled is called when the run is cancelled.
func (s *RemoteSync) OnCancelled() {
	// Update local run record
	s.updateLocalRunStatus("cancelled", "Run was cancelled by user")

	if s.isOffline() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.client.CancelRun(ctx, s.projectID, s.RunID); err != nil {
		s.handleSyncError("cancel remote run", err)
	}
}

// updateLocalRunStatus updates the local run record with final status.
func (s *RemoteSync) updateLocalRunStatus(status, errorMsg string) {
	if s.runStore == nil {
		return
	}

	run, err := s.runStore.GetRun(s.RunID)
	if err != nil || run == nil {
		return
	}

	now := time.Now()
	run.Status = status
	run.FinishedAt = &now
	run.ErrorMsg = errorMsg

	if err := s.runStore.UpdateRun(run); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to update local run status: %v\n", err)
	}
}

// AppendLog adds a log line to the buffer and flushes when full.
// It also stores logs locally for offline fallback.
func (s *RemoteSync) AppendLog(taskName, stream, line string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	// Increment per-task line number
	s.taskLineNums[taskName]++
	lineNum := s.taskLineNums[taskName]

	// Always store locally for offline fallback/history
	if s.runStore != nil {
		entry := &client.RunLogEntry{
			Timestamp: time.Now(),
			TaskName:  taskName,
			Stream:    stream,
			Line:      line,
		}
		if err := s.runStore.AppendLog(s.RunID, entry); err != nil {
			// Non-fatal, just warn once
			fmt.Fprintf(os.Stderr, "Warning: Failed to store log locally: %v\n", err)
		}
	}

	// Buffer for remote send
	s.logBuffer = append(s.logBuffer, client.LogEntry{
		TaskName: taskName,
		Stream:   stream,
		Line:     line,
		LineNum:  lineNum,
	})

	// Flush when buffer is full, but skip if we recently had a flush failure
	// to avoid a retry storm. The periodic flusher (every 2s) handles retries.
	if len(s.logBuffer) >= 100 && time.Since(s.lastFlushError) > 2*time.Second {
		s.flushLogsLocked()
	}
}

// flushLogs sends buffered logs to the server.
func (s *RemoteSync) flushLogs() {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.flushLogsLocked()
}

// flushLogsLocked sends buffered logs (must be called with lock held).
// On success the buffer is cleared. On error the logs are retained for retry
// on the next flush, subject to the maxLogBuf cap.
func (s *RemoteSync) flushLogsLocked() {
	if len(s.logBuffer) == 0 {
		return
	}

	if s.isOffline() {
		s.logBuffer = s.logBuffer[:0]
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.client.AppendLogs(ctx, s.projectID, s.RunID, s.logBuffer); err != nil {
		s.handleSyncError("send logs", err)
		s.lastFlushError = time.Now()
		// Keep logs in buffer for retry on next flush.
		// Trim oldest entries if the buffer exceeds the cap.
		if s.maxLogBuf > 0 && len(s.logBuffer) > s.maxLogBuf {
			s.logBuffer = s.logBuffer[len(s.logBuffer)-s.maxLogBuf:]
		}
		return
	}

	s.resetErrorCount()
	s.logBuffer = s.logBuffer[:0]
}

// isOffline returns true if we've detected we're offline and should skip sync operations.
func (s *RemoteSync) isOffline() bool {
	return s.offline
}

// markOffline marks the sync as offline and prints a message once.
// It also marks the local run as pending sync.
func (s *RemoteSync) markOffline() {
	s.offline = true
	s.offlineOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "\nWarning: Lost connection to server. Remote sync disabled.\n")
		fmt.Fprintf(os.Stderr, "Logs are being stored locally in .dagryn/runs/%s/\n", s.RunID)
		fmt.Fprintf(os.Stderr, "The local run will continue. You can sync later when online.\n\n")

		// Mark local run as pending sync
		if s.runStore != nil {
			run, err := s.runStore.GetRun(s.RunID)
			if err == nil && run != nil {
				run.PendingSync = true
				if err := s.runStore.UpdateRun(run); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to mark run as pending sync: %v\n", err)
				}
			}
		}
	})
}

// handleSyncError handles an error from a sync operation.
// It tracks consecutive errors and marks as offline if too many occur.
func (s *RemoteSync) handleSyncError(op string, err error) {
	if err == nil {
		return
	}

	s.errorCount++

	// Check if this is a network error
	if client.IsNetworkError(err) {
		if s.errorCount >= s.maxErrors {
			s.markOffline()
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Failed to %s (network error, attempt %d/%d)\n", op, s.errorCount, s.maxErrors)
		}
		return
	}

	// For non-network errors, just warn
	fmt.Fprintf(os.Stderr, "Warning: Failed to %s: %v\n", op, err)
}

// resetErrorCount resets the consecutive error counter on success.
func (s *RemoteSync) resetErrorCount() {
	s.errorCount = 0
}

// startPeriodicFlush starts a goroutine that flushes logs at regular intervals.
func (s *RemoteSync) startPeriodicFlush(interval time.Duration) {
	s.flushTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-s.flushTicker.C:
				s.flushLogs()
			case <-s.done:
				return
			}
		}
	}()
}

// SetCancelFunc sets the context cancel function so that a remote cancellation
// detected via heartbeat can stop the local scheduler.
func (s *RemoteSync) SetCancelFunc(cancel context.CancelFunc) {
	s.cancelFunc = cancel
}

// IsRemoteCancelled returns true if the run was cancelled via the dashboard.
func (s *RemoteSync) IsRemoteCancelled() bool {
	return s.remoteCancelled.Load()
}

// startHeartbeat starts a goroutine that periodically sends heartbeats to the
// server. If the server reports the run as "cancelled", it cancels the local
// context to stop running tasks.
func (s *RemoteSync) startHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if s.isOffline() {
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				resp, err := s.client.Heartbeat(ctx, s.projectID, s.RunID)
				cancel()

				if err != nil {
					// Heartbeat failure is non-critical — silently ignore.
					continue
				}

				if resp.Status == "cancelled" {
					fmt.Fprintf(os.Stderr, "\nRun cancelled remotely. Stopping tasks...\n")
					s.remoteCancelled.Store(true)
					s.updateLocalRunStatus("cancelled", "Cancelled remotely")
					if s.cancelFunc != nil {
						s.cancelFunc()
					}
					return
				}
			case <-s.done:
				return
			}
		}
	}()
}

// Stop stops the periodic flusher, flushes remaining logs, and closes resources.
// CollectArtifacts uploads artifacts from successful tasks to the remote server.
// When a task produces 2+ output files they are bundled into a single tar.gz archive.
func (s *RemoteSync) CollectArtifacts(workflow *task.Workflow, summary *scheduler.RunSummary, projectRoot string) {
	if s.isOffline() || workflow == nil || summary == nil {
		return
	}

	resultStatus := make(map[string]executor.Status, len(summary.Results))
	for _, res := range summary.Results {
		resultStatus[res.Task] = res.Status
	}

	for _, tsk := range workflow.ListTasks() {
		status, ok := resultStatus[tsk.Name]
		if !ok || (status != executor.Success && status != executor.Cached) {
			continue
		}
		if !tsk.HasOutputs() {
			continue
		}

		outputs := tsk.Outputs
		if tsk.Workdir != "" {
			outputs = make([]string, len(tsk.Outputs))
			for i, p := range tsk.Outputs {
				outputs[i] = filepath.Join(tsk.Workdir, p)
			}
		}

		resolved, err := cache.ResolveFilePatterns(projectRoot, outputs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to resolve artifact patterns for %s: %v\n", tsk.Name, err)
			continue
		}

		// Filter out skip paths.
		var filtered []string
		for _, path := range resolved {
			relPath, err := filepath.Rel(projectRoot, path)
			if err != nil {
				continue
			}
			if isArtifactSkipPath(relPath) {
				continue
			}
			filtered = append(filtered, path)
		}

		if len(filtered) == 0 {
			continue
		}

		if len(filtered) == 1 {
			// Single file — upload directly.
			path := filtered[0]
			relPath, _ := filepath.Rel(projectRoot, path)
			fileName := filepath.Base(relPath)

			f, err := os.Open(path)
			if err != nil {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			err = s.client.UploadArtifact(ctx, s.projectID, s.RunID, tsk.Name, relPath, fileName, f)
			cancel()
			_ = f.Close()

			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: artifact upload failed for %s: %v\n", relPath, err)
			}
			continue
		}

		// 2+ files — bundle into a single tar.gz archive.
		var relPatterns []string
		for _, path := range filtered {
			relPath, _ := filepath.Rel(projectRoot, path)
			relPatterns = append(relPatterns, relPath)
		}

		archive, err := cloud.CreateArchive(projectRoot, relPatterns, artifactSkipDirs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: artifact archive failed for %s: %v\n", tsk.Name, err)
			continue
		}

		artifactName := tsk.Name + " outputs"
		archiveFileName := tsk.Name + "-outputs.tar.gz"
		metaJSON := fmt.Sprintf(`{"archive":true,"file_count":%d}`, len(filtered))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err = s.client.UploadArtifact(ctx, s.projectID, s.RunID, tsk.Name, artifactName, archiveFileName, archive,
			client.WithArtifactContentType("application/gzip"),
			client.WithArtifactMetadata(metaJSON),
		)
		cancel()
		_ = archive.Close()
		_ = os.Remove(archive.Name())

		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: artifact archive upload failed for %s: %v\n", tsk.Name, err)
		}
	}
}

func (s *RemoteSync) Stop() {
	// Signal flusher goroutine to stop
	if s.done != nil {
		close(s.done)
	}

	// Stop ticker
	if s.flushTicker != nil {
		s.flushTicker.Stop()
	}

	// Final flush with retries — this is the last chance to deliver logs.
	for attempt := 0; attempt < 3; attempt++ {
		s.flushLogs()
		s.logMu.Lock()
		empty := len(s.logBuffer) == 0
		s.logMu.Unlock()
		if empty || s.isOffline() {
			break
		}
		// Brief backoff before retry
		time.Sleep(500 * time.Millisecond)
	}

	// Close run store (closes log file descriptors)
	if s.runStore != nil {
		if err := s.runStore.CloseLogs(s.RunID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to close log file: %v\n", err)
		}
	}
}

// artifactSkipDirs are directories whose contents should never be uploaded as
// artifacts — they are dependencies or metadata, not build outputs.
var artifactSkipDirs = []string{
	"node_modules",
	".git",
	".dagryn",
}

// isArtifactSkipPath returns true if relPath falls inside a skip directory.
func isArtifactSkipPath(relPath string) bool {
	for _, dir := range artifactSkipDirs {
		if relPath == dir || strings.HasPrefix(relPath, dir+string(filepath.Separator)) {
			return true
		}
		// Also check nested occurrences (e.g. web/node_modules/...)
		nested := string(filepath.Separator) + dir + string(filepath.Separator)
		if strings.Contains(relPath, nested) {
			return true
		}
	}
	return false
}

// buildCacheBackend creates a cache.Backend from the configuration.
// Returns nil to use the default local backend.
func buildCacheBackend(cfg config.CacheConfig, projectRoot string, log *logger.Logger) cache.Backend {
	cacheRoot := projectRoot
	if cfg.Dir != "" {
		cacheRoot = cfg.Dir
	}

	local := cache.NewLocalBackend(cacheRoot)

	if !cfg.Remote.Enabled || noRemoteCache {
		return local
	}

	strategy := cache.StrategyLocalFirst
	switch cfg.Remote.Strategy {
	case "remote-first":
		strategy = cache.StrategyRemoteFirst
	case "write-through":
		strategy = cache.StrategyWriteThrough
	}

	onRemoteErr := func(op string, err error) {
		log.Error(fmt.Sprintf("remote cache %s (non-fatal, using local)", op), err)
	}

	if cfg.Remote.Cloud {
		cloudBackend, err := buildCloudBackend(projectRoot, log)
		if err != nil {
			log.Error("cloud cache not available, falling back to local", err)
			return local
		}
		return cache.NewHybridBackend(local, cloudBackend, cache.HybridConfig{
			Strategy:        strategy,
			FallbackOnError: cfg.Remote.IsFallbackOnError(),
			OnError:         onRemoteErr,
		})
	}

	if cfg.Remote.Provider == "grpc" {
		grpcBackend, err := buildGRPCBackend(cfg.Remote, projectRoot, log)
		if err != nil {
			log.Error("grpc cache not available, falling back to local", err)
			return local
		}
		return cache.NewHybridBackend(local, grpcBackend, cache.HybridConfig{
			Strategy:        strategy,
			FallbackOnError: cfg.Remote.IsFallbackOnError(),
			OnError:         onRemoteErr,
		})
	}

	bucket, err := buildBucket(cfg.Remote)
	if err != nil {
		log.Error("failed to create remote cache, falling back to local", err)
		return local
	}

	remoteBackend := remote.NewStorageBackend(bucket, projectRoot)

	return cache.NewHybridBackend(local, remoteBackend, cache.HybridConfig{
		Strategy:        strategy,
		FallbackOnError: cfg.Remote.IsFallbackOnError(),
		OnError:         onRemoteErr,
	})
}

// buildCloudBackend creates a cloud cache backend using the Dagryn server API.
func buildCloudBackend(projectRoot string, log *logger.Logger) (cache.Backend, error) {
	credStore, err := client.NewCredentialsStore()
	if err != nil {
		return nil, fmt.Errorf("credentials store: %w", err)
	}

	creds, err := credStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}
	if creds == nil {
		return nil, fmt.Errorf("not logged in — run 'dagryn auth login' first")
	}

	projectStore := client.NewProjectConfigStore(projectRoot)
	projectConfig, err := projectStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}
	if projectConfig == nil {
		return nil, fmt.Errorf("no project linked — run 'dagryn init --remote' first")
	}

	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 60 * time.Second,
	})
	apiClient.SetCredentials(creds)

	log.Info(fmt.Sprintf("Cloud cache enabled (project: %s)", projectConfig.ProjectName))

	return cloud.NewBackend(apiClient, projectConfig.ProjectID, projectRoot), nil
}

// buildGRPCBackend creates a gRPC cache backend using the REAPI v2 protocol.
func buildGRPCBackend(rc config.RemoteCacheConfig, projectRoot string, log *logger.Logger) (cache.Backend, error) {
	connCfg := grpccache.ConnConfig{
		Target:       rc.GRPCTarget,
		InstanceName: rc.InstanceName,
		TLS:          rc.IsTLS(),
		TLSCACert:    rc.TLSCACert,
		AuthToken:    rc.AuthToken,
		DialTimeout:  10 * time.Second,
	}
	if rc.TLS != nil && !*rc.TLS {
		connCfg.Insecure = true
		connCfg.TLS = false
	}
	conn, err := grpccache.Dial(context.Background(), connCfg)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	log.Info(fmt.Sprintf("gRPC cache enabled (target: %s)", rc.GRPCTarget))
	return grpccache.NewBackend(conn, rc.InstanceName, projectRoot), nil
}

// getGitBranch returns the current git branch.
func getGitBranch() string {
	// Simple implementation - read from .git/HEAD
	data, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	return ""
}

// getGitCommit returns the current git commit hash.
func getGitCommit() string {
	// First check if HEAD is a direct reference
	data, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))

	// If it's a direct commit hash (detached HEAD)
	if !strings.HasPrefix(content, "ref:") {
		if len(content) >= 7 {
			return content[:7]
		}
		return content
	}

	// Otherwise, read the ref file
	refPath := strings.TrimPrefix(content, "ref: ")
	refData, err := os.ReadFile(".git/" + refPath)
	if err != nil {
		return ""
	}
	commit := strings.TrimSpace(string(refData))
	if len(commit) >= 7 {
		return commit[:7]
	}
	return commit
}
