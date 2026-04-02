package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog/log"
)

// ConnectedWorker represents an in-memory view of a live worker connection.
type ConnectedWorker struct {
	ID          string
	Info        *WorkerInfo
	ActiveTasks int32
	LastSeen    time.Time
	TaskCh      chan *TaskDispatch  // Channel to send task assignments
	ControlCh   chan *ControlAction // Channel to send control messages
}

// WorkerInfo holds worker metadata (mirrors protobuf WorkerInfo).
type WorkerInfo struct {
	Hostname           string
	OS                 string
	Arch               string
	Environment        string
	Labels             map[string]string
	MaxConcurrentTasks int32
	Capabilities       []string
	Version            string
}

// TaskDispatch represents a task to be sent to a worker.
type TaskDispatch struct {
	AssignmentID string
	RunID        string
	ProjectID    string
	TaskName     string
	Command      string
	Workdir      string
	Env          map[string]string
	TimeoutMs    int64
	GitRepoURL   string
	GitRef       string
	GitCommit    string
	GitToken     string
}

// ControlAction represents a control message to send to a worker.
type ControlAction struct {
	Type        string // "drain", "shutdown"
	Reason      string
	GracePeriod time.Duration
}

// WorkerRegistry tracks connected workers in-memory and in the database.
type WorkerRegistry struct {
	store    repo.ClusterStore
	mu       sync.RWMutex
	workers  map[string]*ConnectedWorker // workerID -> live connection
	interval time.Duration               // heartbeat interval
	timeout  time.Duration               // stale detection threshold
}

// NewWorkerRegistry creates a new worker registry.
func NewWorkerRegistry(store repo.ClusterStore, heartbeatSec, staleTimeoutSec int) *WorkerRegistry {
	interval := 10 * time.Second
	timeout := 30 * time.Second
	if heartbeatSec > 0 {
		interval = time.Duration(heartbeatSec) * time.Second
	}
	if staleTimeoutSec > 0 {
		timeout = time.Duration(staleTimeoutSec) * time.Second
	}

	return &WorkerRegistry{
		store:    store,
		workers:  make(map[string]*ConnectedWorker),
		interval: interval,
		timeout:  timeout,
	}
}

// Register registers a new worker and returns its ID.
func (r *WorkerRegistry) Register(ctx context.Context, info *WorkerInfo, tokenHash string, clusterID *uuid.UUID) (*ConnectedWorker, error) {
	worker := &models.Worker{
		Hostname:           info.Hostname,
		OS:                 info.OS,
		Arch:               info.Arch,
		Environment:        info.Environment,
		Capabilities:       info.Capabilities,
		MaxConcurrentTasks: int(info.MaxConcurrentTasks),
		Version:            info.Version,
		Status:             models.WorkerStatusOnline,
		AuthTokenHash:      tokenHash,
		ClusterID:          clusterID,
	}

	// Marshal labels to JSON
	if info.Labels != nil {
		labelsJSON, err := marshalLabels(info.Labels)
		if err != nil {
			return nil, fmt.Errorf("marshal labels: %w", err)
		}
		worker.Labels = labelsJSON
	}

	if err := r.store.RegisterWorker(ctx, worker); err != nil {
		return nil, fmt.Errorf("register worker in db: %w", err)
	}

	cw := &ConnectedWorker{
		ID:        worker.ID.String(),
		Info:      info,
		LastSeen:  time.Now(),
		TaskCh:    make(chan *TaskDispatch, 16),
		ControlCh: make(chan *ControlAction, 4),
	}

	r.mu.Lock()
	r.workers[cw.ID] = cw
	r.mu.Unlock()

	log.Info().Str("worker_id", cw.ID).Str("hostname", info.Hostname).Msg("Worker registered")
	return cw, nil
}

// Heartbeat updates a worker's status and resource snapshot.
func (r *WorkerRegistry) Heartbeat(workerID string, activeTasks int32, resources *repo.WorkerResourceUpdate) error {
	r.mu.Lock()
	cw, ok := r.workers[workerID]
	if ok {
		cw.ActiveTasks = activeTasks
		cw.LastSeen = time.Now()
	}
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("worker %s not connected", workerID)
	}

	id, err := uuid.Parse(workerID)
	if err != nil {
		return fmt.Errorf("invalid worker ID: %w", err)
	}
	if resources == nil {
		resources = &repo.WorkerResourceUpdate{}
	}
	return r.store.UpdateWorkerHeartbeat(context.Background(), id, resources)
}

// Deregister removes a worker from the registry.
func (r *WorkerRegistry) Deregister(workerID string) {
	r.mu.Lock()
	cw, ok := r.workers[workerID]
	if ok {
		close(cw.TaskCh)
		close(cw.ControlCh)
		delete(r.workers, workerID)
	}
	r.mu.Unlock()

	if ok {
		id, err := uuid.Parse(workerID)
		if err == nil {
			_ = r.store.UpdateWorkerStatus(context.Background(), id, models.WorkerStatusOffline)
		}
		log.Info().Str("worker_id", workerID).Msg("Worker deregistered")
	}
}

// ListOnline returns all connected workers.
func (r *WorkerRegistry) ListOnline() []*ConnectedWorker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ConnectedWorker, 0, len(r.workers))
	for _, w := range r.workers {
		result = append(result, w)
	}
	return result
}

// GetWorker returns a connected worker by ID.
func (r *WorkerRegistry) GetWorker(id string) (*ConnectedWorker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.workers[id]
	return w, ok
}

// FindWorkers returns workers matching the given label requirements and capabilities.
func (r *WorkerRegistry) FindWorkers(labels map[string]string, capabilities []string) []*ConnectedWorker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ConnectedWorker
	for _, w := range r.workers {
		if matchLabels(w.Info.Labels, labels) && matchCapabilities(w.Info.Capabilities, capabilities) {
			result = append(result, w)
		}
	}
	return result
}

// StartStaleDetector runs a background goroutine that detects and marks stale workers.
func (r *WorkerRegistry) StartStaleDetector(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.detectStaleWorkers(ctx)
		}
	}
}

func (r *WorkerRegistry) detectStaleWorkers(ctx context.Context) {
	threshold := time.Now().Add(-r.timeout)

	r.mu.Lock()
	var staleIDs []string
	for id, w := range r.workers {
		if w.LastSeen.Before(threshold) {
			staleIDs = append(staleIDs, id)
		}
	}
	for _, id := range staleIDs {
		cw := r.workers[id]
		close(cw.TaskCh)
		close(cw.ControlCh)
		delete(r.workers, id)
	}
	r.mu.Unlock()

	for _, id := range staleIDs {
		uid, err := uuid.Parse(id)
		if err == nil {
			_ = r.store.UpdateWorkerStatus(ctx, uid, models.WorkerStatusOffline)
		}
		log.Warn().Str("worker_id", id).Msg("Worker marked offline (stale heartbeat)")
	}
}

// HeartbeatInterval returns the configured heartbeat interval.
func (r *WorkerRegistry) HeartbeatInterval() time.Duration {
	return r.interval
}

func matchLabels(workerLabels, required map[string]string) bool {
	for k, v := range required {
		if workerLabels[k] != v {
			return false
		}
	}
	return true
}

func matchCapabilities(workerCaps []string, required []string) bool {
	capSet := make(map[string]struct{}, len(workerCaps))
	for _, c := range workerCaps {
		capSet[c] = struct{}{}
	}
	for _, req := range required {
		if _, ok := capSet[req]; !ok {
			return false
		}
	}
	return true
}

func marshalLabels(labels map[string]string) ([]byte, error) {
	return json.Marshal(labels)
}
