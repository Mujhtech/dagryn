package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache/cloud"
	grpccache "github.com/mujhtech/dagryn/pkg/dagryn/cache/grpc"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache/remote"
	"github.com/mujhtech/dagryn/pkg/dagryn/condition"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/dagryn/container"
	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/plugin"
	"github.com/mujhtech/dagryn/pkg/dagryn/scheduler"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	parallel   int
	dryRun     bool
	syncRemote bool
	projectID  string
	flags      *cli.Flags
)

// NewCmd creates the run command.
func NewCmd(f *cli.Flags) *cobra.Command {
	flags = f

	cmd := &cobra.Command{
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

	cmd.Flags().IntVarP(&parallel, "parallel", "p", 0, "max parallel tasks (default: number of CPUs)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show execution plan without running")
	cmd.Flags().BoolVar(&syncRemote, "sync", false, "sync run status to remote server")
	cmd.Flags().StringVar(&projectID, "project", "", "project ID for remote sync (required with --sync)")

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	log := logger.New(flags.Verbose)

	// Get project root
	projectRoot, err := cli.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	// Load config
	cfg, err := config.Parse(flags.CfgFile)
	if err != nil {
		return cli.WrapError(err, nil)
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
			return cli.WrapError(fmt.Errorf("task %q not found", target), cfg)
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
	opts.NoCache = flags.NoCache || !cfg.Cache.IsEnabled()
	opts.NoPlugins = flags.NoPlugins
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

	// Wire container config from project TOML
	if cfg.Container.Enabled {
		opts.ContainerConfig = &container.Config{
			Enabled:     true,
			Image:       cfg.Container.Image,
			MemoryLimit: cfg.Container.MemoryLimit,
			CPULimit:    cfg.Container.CPULimit,
			Network:     cfg.Container.Network,
		}
	}

	// Build condition context for task conditions
	opts.ConditionContext = &condition.Context{
		Branch:  cli.GetGitBranch(),
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

		// Get execution plan to send total task count to the server
		plan, planErr := sched.GetExecutionPlan(targets)
		if planErr == nil {
			remoteSync.SetTotalTasks(plan.TotalTasks())
		}
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
		waitCacheSaves(sched)
		if remoteSync != nil {
			remoteSync.Stop()
		}
		return err
	}

	// Print summary immediately so the user sees the result.
	log.Summary(summary.Results, summary.Total, summary.CacheHits)

	// Wait for background cache saves to complete (context must stay alive).
	waitCacheSaves(sched)

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

// waitCacheSaves shows a progress bar while background cache saves complete.
func waitCacheSaves(sched *scheduler.Scheduler) {
	done := make(chan struct{})
	go func() {
		sched.WaitCacheSaves()
		close(done)
	}()

	// If saves finish quickly (< 150ms), skip the progress bar entirely.
	select {
	case <-done:
		return
	case <-time.After(150 * time.Millisecond):
	}

	total, completed := sched.CacheSaveProgress()
	if total == 0 {
		<-done
		return
	}

	w := cliui.NewWriter()
	bar := cliui.NewProgressBar(os.Stderr, "Saving cache", total)
	bar.Set(completed)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			bar.Complete("")
			w.Successf("cache saved (%d tasks)", total)
			return
		case <-ticker.C:
			_, completed = sched.CacheSaveProgress()
			bar.Set(completed)
		}
	}
}

// buildCacheBackend creates a cache.Backend from the configuration.
// Returns nil to use the default local backend.
func buildCacheBackend(cfg config.CacheConfig, projectRoot string, log *logger.Logger) cache.Backend {
	cacheRoot := projectRoot
	if cfg.Dir != "" {
		cacheRoot = cfg.Dir
	}

	local := cache.NewLocalBackend(cacheRoot)

	if !cfg.Remote.Enabled || flags.NoRemoteCache || !syncRemote {
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

	bucket, err := cli.BuildBucket(cfg.Remote)
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
