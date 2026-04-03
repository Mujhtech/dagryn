package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/google/uuid"
	serverconfig "github.com/mujhtech/dagryn/pkg/config"
	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/mujhtech/dagryn/pkg/cluster/v1"
)

// GRPCServer wraps a gRPC server with cluster service handlers.
type GRPCServer struct {
	v1.UnimplementedClusterServiceServer

	server    *grpc.Server
	registry  *WorkerRegistry
	store     repo.ClusterStore
	tokens    repo.ClusterWorkerTokenStore
	logBridge *LogBridge
	cfg       serverconfig.ClusterConfig
}

// NewGRPCServer creates and configures a new gRPC server for cluster operations.
func NewGRPCServer(registry *WorkerRegistry, store repo.ClusterStore, tokenStore repo.ClusterWorkerTokenStore, logBridge *LogBridge, cfg serverconfig.ClusterConfig) *GRPCServer {
	var opts []grpc.ServerOption

	// mTLS if configured (moved earlier per review recommendation)
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		creds, err := LoadTLSCredentials(cfg.TLSCertFile, cfg.TLSKeyFile, cfg.TLSCAFile)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load TLS credentials, falling back to insecure")
		} else {
			opts = append(opts, grpc.Creds(creds))
		}
	}

	// Token auth interceptors
	if tokenStore != nil {
		opts = append(opts,
			grpc.ChainStreamInterceptor(TokenAuthStreamInterceptorWithStore(tokenStore)),
			grpc.ChainUnaryInterceptor(TokenAuthUnaryInterceptorWithStore(tokenStore)),
		)
	}

	// Keepalive
	opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle: 5 * time.Minute,
		Time:              30 * time.Second,
		Timeout:           10 * time.Second,
	}))

	srv := grpc.NewServer(opts...)

	gs := &GRPCServer{
		server:    srv,
		registry:  registry,
		store:     store,
		tokens:    tokenStore,
		logBridge: logBridge,
		cfg:       cfg,
	}

	v1.RegisterClusterServiceServer(srv, gs)
	return gs
}

// Serve starts the gRPC server on the given address.
func (s *GRPCServer) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	log.Info().Str("addr", addr).Msg("gRPC cluster server started")
	return s.server.Serve(lis)
}

// GracefulStop gracefully stops the gRPC server.
func (s *GRPCServer) GracefulStop() {
	s.server.GracefulStop()
}

// RegisterWorker implements the bidirectional heartbeat stream.
func (s *GRPCServer) RegisterWorker(stream v1.ClusterService_RegisterWorkerServer) error {
	var workerID string
	defer func() {
		if workerID != "" {
			s.registry.Deregister(workerID)
		}
	}()

	for {
		hb, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if workerID == "" && hb.Info != nil {
			// First heartbeat: register the worker
			md, ok := metadata.FromIncomingContext(stream.Context())
			if !ok {
				return fmt.Errorf("missing metadata")
			}
			scope, scopeErr := ResolveWorkerScope(stream.Context(), md, s.tokens)
			if scopeErr != nil {
				return scopeErr
			}

			tokens := md.Get(tokenMetadataKey)
			if len(tokens) == 0 {
				return fmt.Errorf("missing registration token")
			}
			tokenHash := HashToken(tokens[0])

			var clusterID *uuid.UUID
			cluster, clusterErr := s.resolveWorkerCluster(stream.Context(), hb.Info, scope)
			if clusterErr != nil {
				return clusterErr
			}
			if cluster != nil {
				clusterID = &cluster.ID
			}

			info := &WorkerInfo{
				Hostname:           hb.Info.Hostname,
				OS:                 hb.Info.Os,
				Arch:               hb.Info.Arch,
				Environment:        hb.Info.Environment,
				Labels:             hb.Info.Labels,
				MaxConcurrentTasks: hb.Info.MaxConcurrentTasks,
				Capabilities:       hb.Info.Capabilities,
				Version:            hb.Info.Version,
			}

			cw, err := s.registry.Register(stream.Context(), info, tokenHash, clusterID)
			if err != nil {
				return fmt.Errorf("register worker: %w", err)
			}
			workerID = cw.ID
			hb.WorkerId = workerID
		}

		if workerID == "" {
			return fmt.Errorf("first heartbeat must include WorkerInfo")
		}

		// Update heartbeat
		var resources *repo.WorkerResourceUpdate
		if hb.Resources != nil {
			resources = &repo.WorkerResourceUpdate{
				CPUMillicoresAvail: hb.Resources.CpuMillicoresAvailable,
				MemoryBytesAvail:   hb.Resources.MemoryBytesAvailable,
				DiskBytesAvail:     hb.Resources.DiskBytesAvailable,
				CPUUsagePercent:    hb.Resources.CpuUsagePercent,
				MemoryUsagePercent: hb.Resources.MemoryUsagePercent,
				ActiveTasks:        int(hb.ActiveTasks),
			}
		}
		_ = s.registry.Heartbeat(workerID, hb.ActiveTasks, resources)

		// Send ack (include worker_id on first ack so the agent knows its assigned ID)
		ack := &v1.HeartbeatAck{
			ServerTime:        timestamppb.Now(),
			HeartbeatInterval: durationpb.New(s.registry.HeartbeatInterval()),
			WorkerId:          workerID,
		}
		if err := stream.Send(&v1.ControlMessage{
			Message: &v1.ControlMessage_Ack{Ack: ack},
		}); err != nil {
			return err
		}
	}
}

func (s *GRPCServer) resolveWorkerCluster(ctx context.Context, info *v1.WorkerInfo, scope *WorkerScope) (*models.Cluster, error) {
	if scope == nil {
		return nil, fmt.Errorf("worker scope is required")
	}

	if scope.Type == "team" {
		if info != nil && info.Labels != nil {
			if clusterName, ok := info.Labels["cluster"]; ok && clusterName != "" {
				cluster, err := s.store.GetClusterByNameInScope(ctx, clusterName, scope.TeamID, nil)
				if err != nil {
					return nil, fmt.Errorf("team cluster %q not found", clusterName)
				}
				return cluster, nil
			}
		}
		return s.store.EnsureDefaultClusterForScope(ctx, scope.TeamID, nil)
	}

	if scope.Type == "personal" {
		if info != nil && info.Labels != nil {
			if clusterName, ok := info.Labels["cluster"]; ok && clusterName != "" {
				cluster, err := s.store.GetClusterByNameInScope(ctx, clusterName, nil, scope.OwnerUserID)
				if err != nil {
					return nil, fmt.Errorf("personal cluster %q not found", clusterName)
				}
				return cluster, nil
			}
		}
		return s.store.EnsureDefaultClusterForScope(ctx, nil, scope.OwnerUserID)
	}

	if info != nil && info.Labels != nil {
		if _, ok := info.Labels["team_id"]; ok {
			return nil, fmt.Errorf("team_id label is not allowed for worker registration")
		}
		if _, ok := info.Labels["owner_user_id"]; ok {
			return nil, fmt.Errorf("owner_user_id label is not allowed for worker registration")
		}
	}

	return nil, fmt.Errorf("unsupported worker scope: %q", scope.Type)
}

// TaskStream implements the bidirectional task dispatch/result stream.
func (s *GRPCServer) TaskStream(stream v1.ClusterService_TaskStreamServer) error {
	// Read the first event to identify the worker
	firstEvent, err := stream.Recv()
	if err != nil {
		return err
	}

	workerID := firstEvent.WorkerId
	cw, ok := s.registry.GetWorker(workerID)
	if !ok {
		return fmt.Errorf("worker %s not registered", workerID)
	}

	// Process initial event (only if it carries an actual task event, not just
	// the identification handshake with only WorkerId set)
	if firstEvent.AssignmentId != "" {
		s.handleTaskEvent(firstEvent)
	}

	// Start goroutine to send task assignments to this worker
	errCh := make(chan error, 1)
	go func() {
		for dispatch := range cw.TaskCh {
			assignment := &v1.TaskAssignment{
				AssignmentId: dispatch.AssignmentID,
				RunId:        dispatch.RunID,
				ProjectId:    dispatch.ProjectID,
				TaskName:     dispatch.TaskName,
				Command:      dispatch.Command,
				Workdir:      dispatch.Workdir,
				Env:          dispatch.Env,
				TimeoutMs:    dispatch.TimeoutMs,
			}
			if dispatch.GitRepoURL != "" {
				assignment.Source = &v1.TaskAssignment_Git{
					Git: &v1.GitSource{
						RepoUrl: dispatch.GitRepoURL,
						Ref:     dispatch.GitRef,
						Commit:  dispatch.GitCommit,
						Token:   dispatch.GitToken,
					},
				}
			}
			if err := stream.Send(assignment); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Read events from worker
	for {
		select {
		case err := <-errCh:
			return err
		default:
		}

		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		s.handleTaskEvent(event)
	}
}

func (s *GRPCServer) handleTaskEvent(event *v1.TaskEvent) {
	assignmentID, err := uuid.Parse(event.AssignmentId)
	if err != nil {
		log.Warn().Str("assignment_id", event.AssignmentId).Msg("Invalid assignment ID in task event")
		return
	}

	switch e := event.Event.(type) {
	case *v1.TaskEvent_Accepted:
		_ = s.store.UpdateTaskAssignmentStatus(
			context.Background(), assignmentID, models.TaskAssignmentRunning, nil,
		)
	case *v1.TaskEvent_Rejected:
		log.Warn().
			Str("assignment_id", event.AssignmentId).
			Str("reason", e.Rejected.Reason).
			Msg("Worker rejected task")
		_ = s.store.UpdateTaskAssignmentStatus(
			context.Background(), assignmentID, models.TaskAssignmentPending, nil,
		)
	case *v1.TaskEvent_Result:
		status := models.TaskAssignmentCompleted
		execStatus := executor.Status(e.Result.Status)
		if execStatus != executor.Success && execStatus != executor.Cached {
			status = models.TaskAssignmentFailed
		}
		resultJSON, _ := json.Marshal(map[string]any{
			"status":    execStatus.String(),
			"exit_code": e.Result.ExitCode,
			"error":     e.Result.ErrorMessage,
			"output":    e.Result.Output,
		})
		_ = s.store.UpdateTaskAssignmentStatus(
			context.Background(), assignmentID, status, resultJSON,
		)
	}
}

// StreamLogs receives log entries from workers and bridges them to SSE.
func (s *GRPCServer) StreamLogs(stream v1.ClusterService_StreamLogsServer) error {
	var count int64
	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&v1.LogAck{LinesReceived: count})
		}
		if err != nil {
			return err
		}
		count++

		if s.logBridge != nil {
			s.logBridge.HandleLogEntry(entry)
		}
	}
}
