package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	v1 "github.com/mujhtech/dagryn/pkg/cluster/v1"
	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/task"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Config holds the agent configuration.
type Config struct {
	ServerAddr    string
	Token         string
	Labels        map[string]string
	MaxConcurrent int
	HeartbeatSec  int
	WorkDir       string
	ClusterName   string
	TLSCertFile   string
	TLSKeyFile    string
	TLSCAFile     string
}

// Agent is a worker agent that connects to a Dagryn control plane.
type Agent struct {
	config     Config
	grpcConn   *grpc.ClientConn
	client     v1.ClusterServiceClient
	workerID   string
	env        *DetectedEnvironment
	workspace  *WorkspaceManager
	cancelFunc context.CancelFunc
	draining   atomic.Bool
	activeTasks atomic.Int32
	wg         sync.WaitGroup
}

// New creates a new agent with the given configuration.
func New(cfg Config) *Agent {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 4
	}
	if cfg.HeartbeatSec <= 0 {
		cfg.HeartbeatSec = 10
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = os.TempDir()
	}

	return &Agent{
		config:    cfg,
		env:       DetectEnvironment(),
		workspace: NewWorkspaceManager(cfg.WorkDir),
	}
}

// Start connects to the control plane and enters the task execution loop.
func (a *Agent) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancelFunc = cancel

	// Connect to gRPC server
	var dialOpts []grpc.DialOption
	if a.config.TLSCertFile != "" {
		// mTLS not yet wired - will use LoadClientTLSCredentials from cluster package
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(a.config.ServerAddr, dialOpts...)
	if err != nil {
		cancel()
		return fmt.Errorf("connect to server: %w", err)
	}
	a.grpcConn = conn
	a.client = v1.NewClusterServiceClient(conn)

	// Merge auto-detected labels with config labels
	labels := make(map[string]string)
	for k, v := range a.env.Labels {
		labels[k] = v
	}
	for k, v := range a.config.Labels {
		labels[k] = v
	}

	log.Info().
		Str("server", a.config.ServerAddr).
		Str("hostname", a.env.Hostname).
		Str("environment", a.env.Type).
		Msg("Agent starting")

	// Add token to context metadata
	md := metadata.New(map[string]string{
		"x-registration-token": a.config.Token,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Start heartbeat stream (must complete registration before task stream starts)
	errCh := make(chan error, 2)
	registered := make(chan struct{})
	go func() {
		errCh <- a.runHeartbeatLoop(ctx, labels, registered)
	}()

	// Wait for registration to complete before starting task stream
	go func() {
		select {
		case <-registered:
			errCh <- a.runTaskLoop(ctx)
		case <-ctx.Done():
			errCh <- ctx.Err()
		}
	}()

	// Handle graceful shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		log.Info().Str("signal", sig.String()).Msg("Shutdown signal received")
		return a.Shutdown(ctx)
	case err := <-errCh:
		cancel()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runHeartbeatLoop maintains the registration/heartbeat stream.
// It closes the registered channel once the worker ID has been assigned.
func (a *Agent) runHeartbeatLoop(ctx context.Context, labels map[string]string, registered chan<- struct{}) error {
	stream, err := a.client.RegisterWorker(ctx)
	if err != nil {
		return fmt.Errorf("register worker stream: %w", err)
	}

	// Send initial heartbeat with WorkerInfo
	if err := stream.Send(&v1.WorkerHeartbeat{
		Info: &v1.WorkerInfo{
			Hostname:           a.env.Hostname,
			Os:                 a.env.OS,
			Arch:               a.env.Arch,
			Environment:        a.env.Type,
			Labels:             labels,
			MaxConcurrentTasks: int32(a.config.MaxConcurrent),
			Capabilities:       a.env.Capabilities,
			Version:            "1.0.0", // TODO: use build version
		},
		Resources:   a.collectResources(),
		ActiveTasks: a.activeTasks.Load(),
		Timestamp:   timestamppb.Now(),
	}); err != nil {
		return fmt.Errorf("send initial heartbeat: %w", err)
	}

	// Read ack to get worker ID
	ack, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive heartbeat ack: %w", err)
	}
	if ackMsg := ack.GetAck(); ackMsg != nil && ackMsg.WorkerId != "" {
		a.workerID = ackMsg.WorkerId
	}

	log.Info().Str("worker_id", a.workerID).Msg("Worker registered with control plane")
	close(registered)

	// Periodic heartbeat loop
	ticker := time.NewTicker(time.Duration(a.config.HeartbeatSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := stream.Send(&v1.WorkerHeartbeat{
				WorkerId:    a.workerID,
				Resources:   a.collectResources(),
				ActiveTasks: a.activeTasks.Load(),
				Timestamp:   timestamppb.Now(),
			}); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
			}

			if _, err := stream.Recv(); err != nil {
				return fmt.Errorf("heartbeat ack: %w", err)
			}
		}
	}
}

// runTaskLoop receives and executes task assignments.
func (a *Agent) runTaskLoop(ctx context.Context) error {
	stream, err := a.client.TaskStream(ctx)
	if err != nil {
		return fmt.Errorf("task stream: %w", err)
	}

	// Send an initial empty event to identify ourselves
	if err := stream.Send(&v1.TaskEvent{
		WorkerId: a.workerID,
	}); err != nil {
		return fmt.Errorf("send worker ID: %w", err)
	}

	// Use a channel to serialize all stream.Send calls (gRPC streams are not
	// safe for concurrent Send). We never close sendCh from the receive side
	// because in-flight task goroutines may still write to it. Instead, the
	// sender goroutine exits when ctx is cancelled or a send error occurs.
	sendCh := make(chan *v1.TaskEvent, a.config.MaxConcurrent*2)
	sendErrCh := make(chan error, 1)
	go func() {
		for {
			select {
			case ev, ok := <-sendCh:
				if !ok {
					return
				}
				if err := stream.Send(ev); err != nil {
					sendErrCh <- err
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	sem := make(chan struct{}, a.config.MaxConcurrent)

	for {
		select {
		case err := <-sendErrCh:
			return fmt.Errorf("stream send: %w", err)
		default:
		}

		assignment, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("receive task: %w", err)
		}

		if a.draining.Load() {
			// Reject task during drain — use select to avoid blocking if
			// the sender goroutine has already exited.
			select {
			case sendCh <- &v1.TaskEvent{
				AssignmentId: assignment.AssignmentId,
				WorkerId:     a.workerID,
				Event: &v1.TaskEvent_Rejected{
					Rejected: &v1.TaskRejected{Reason: "agent is draining"},
				},
			}:
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		// Accept task
		select {
		case sendCh <- &v1.TaskEvent{
			AssignmentId: assignment.AssignmentId,
			WorkerId:     a.workerID,
			Event:        &v1.TaskEvent_Accepted{Accepted: &v1.TaskAccepted{}},
		}:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Execute in goroutine with semaphore
		a.wg.Add(1)
		go func(assign *v1.TaskAssignment) {
			defer a.wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			a.activeTasks.Add(1)
			defer a.activeTasks.Add(-1)

			result := a.executeTask(ctx, assign)

			// Use select to avoid blocking forever if the sender goroutine
			// has exited due to error or context cancellation.
			select {
			case sendCh <- &v1.TaskEvent{
				AssignmentId: assign.AssignmentId,
				WorkerId:     a.workerID,
				Event:        &v1.TaskEvent_Result{Result: result},
			}:
			case <-ctx.Done():
			}
		}(assignment)
	}
}

// executeTask executes a single task assignment.
func (a *Agent) executeTask(ctx context.Context, assignment *v1.TaskAssignment) *v1.TaskResult {
	startTime := time.Now()

	// Prepare workspace
	var workdir string
	var err error
	switch src := assignment.Source.(type) {
	case *v1.TaskAssignment_Git:
		workdir, err = a.workspace.PrepareGit(ctx, src.Git.RepoUrl, src.Git.Ref, src.Git.Commit, src.Git.Token)
	case *v1.TaskAssignment_Artifact:
		workdir, err = a.workspace.PrepareArtifact(ctx, src.Artifact.SignedUrl)
	default:
		workdir = a.config.WorkDir
	}

	if err != nil {
		return &v1.TaskResult{
			TaskName:     assignment.TaskName,
			Status:       int32(executor.Failed),
			ErrorMessage: fmt.Sprintf("workspace preparation failed: %v", err),
			StartTime:    timestamppb.New(startTime),
			EndTime:      timestamppb.Now(),
			DurationMs:   time.Since(startTime).Milliseconds(),
		}
	}
	if workdir != a.config.WorkDir {
		defer a.workspace.Cleanup(workdir)
	}

	// Build task
	t := &task.Task{
		Name:    assignment.TaskName,
		Command: assignment.Command,
		Workdir: assignment.Workdir,
		Env:     assignment.Env,
	}
	if assignment.TimeoutMs > 0 {
		t.Timeout = time.Duration(assignment.TimeoutMs) * time.Millisecond
	}

	// Execute
	exec := executor.New(workdir)
	result := exec.Execute(ctx, t)

	errMsg := ""
	if result.Error != nil {
		errMsg = result.Error.Error()
	}

	return &v1.TaskResult{
		TaskName:     result.Task,
		Status:       int32(result.Status),
		DurationMs:   result.Duration.Milliseconds(),
		Output:       result.Output,
		ExitCode:     int32(result.ExitCode),
		ErrorMessage: errMsg,
		StartTime:    timestamppb.New(result.StartTime),
		EndTime:      timestamppb.New(result.EndTime),
	}
}

// Shutdown gracefully drains and disconnects the agent.
func (a *Agent) Shutdown(_ context.Context) error {
	a.draining.Store(true)
	log.Info().Msg("Agent draining, waiting for in-flight tasks...")

	// Wait for in-flight tasks
	a.wg.Wait()

	if a.cancelFunc != nil {
		a.cancelFunc()
	}

	if a.grpcConn != nil {
		_ = a.grpcConn.Close()
	}

	log.Info().Msg("Agent shutdown complete")
	return nil
}

func (a *Agent) collectResources() *v1.ResourceSnapshot {
	// Basic resource collection; a production implementation would use
	// cgroups or system APIs for accurate values.
	return &v1.ResourceSnapshot{
		CpuMillicoresAvailable: 0,
		MemoryBytesAvailable:   0,
		DiskBytesAvailable:     0,
		CpuUsagePercent:        0,
		MemoryUsagePercent:     0,
	}
}
