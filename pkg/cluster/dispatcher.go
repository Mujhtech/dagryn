package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/task"
	"github.com/rs/zerolog/log"
)

// Dispatcher decides which worker executes a task and manages the dispatch lifecycle.
type Dispatcher struct {
	registry  *WorkerRegistry
	store     repo.ClusterStore
	router    TaskRouter
}

// NewDispatcher creates a new task dispatcher.
func NewDispatcher(registry *WorkerRegistry, store repo.ClusterStore, routerStrategy string) *Dispatcher {
	return &Dispatcher{
		registry: registry,
		store:    store,
		router:   NewRouter(routerStrategy),
	}
}

// RemoteTaskExecutor implements executor.TaskExecutor for distributed dispatch.
type RemoteTaskExecutor struct {
	dispatcher *Dispatcher
	runID      string
	projectID  string
	gitSource  *GitSourceConfig
	maxRetries int
}

// GitSourceConfig holds git clone information for remote workers.
type GitSourceConfig struct {
	RepoURL string
	Ref     string
	Commit  string
	Token   string
}

// NewRemoteTaskExecutor creates a new remote task executor.
func NewRemoteTaskExecutor(dispatcher *Dispatcher, runID, projectID string, gitSource *GitSourceConfig, maxRetries int) *RemoteTaskExecutor {
	if maxRetries <= 0 {
		maxRetries = 2
	}
	return &RemoteTaskExecutor{
		dispatcher: dispatcher,
		runID:      runID,
		projectID:  projectID,
		gitSource:  gitSource,
		maxRetries: maxRetries,
	}
}

// Execute dispatches a task to a remote worker and waits for the result.
func (e *RemoteTaskExecutor) Execute(ctx context.Context, t *task.Task) *executor.Result {
	startTime := time.Now()

	// Find eligible workers
	var available []*ConnectedWorker
	if t.Routing != nil && t.Routing.Cluster != "" {
		// Filter by cluster - in this implementation we filter by label matching
		available = e.dispatcher.registry.FindWorkers(t.Routing.Labels, nil)
	} else {
		available = e.dispatcher.registry.ListOnline()
	}

	// Filter workers that have capacity
	var eligible []*ConnectedWorker
	for _, w := range available {
		if w.ActiveTasks < w.Info.MaxConcurrentTasks {
			eligible = append(eligible, w)
		}
	}

	if len(eligible) == 0 {
		return &executor.Result{
			Task:      t.Name,
			Status:    executor.Failed,
			Error:     fmt.Errorf("no eligible workers available for task %s", t.Name),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}
	}

	// Select worker via routing strategy
	worker, err := e.dispatcher.router.SelectWorker(ctx, t, eligible)
	if err != nil {
		return &executor.Result{
			Task:      t.Name,
			Status:    executor.Failed,
			Error:     fmt.Errorf("worker selection failed: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}
	}

	// Create task assignment in DB
	runID, _ := uuid.Parse(e.runID)
	workerID, _ := uuid.Parse(worker.ID)
	now := time.Now()
	assignment := &models.TaskAssignment{
		RunID:      runID,
		TaskName:   t.Name,
		WorkerID:   &workerID,
		Status:     models.TaskAssignmentAssigned,
		AssignedAt: &now,
		MaxRetries: e.maxRetries,
	}

	if err := e.dispatcher.store.CreateTaskAssignment(ctx, assignment); err != nil {
		return &executor.Result{
			Task:      t.Name,
			Status:    executor.Failed,
			Error:     fmt.Errorf("create task assignment: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}
	}

	// Build dispatch message
	dispatch := &TaskDispatch{
		AssignmentID: assignment.ID.String(),
		RunID:        e.runID,
		ProjectID:    e.projectID,
		TaskName:     t.Name,
		Command:      t.Command,
		Workdir:      t.Workdir,
		Env:          t.Env,
		TimeoutMs:    t.Timeout.Milliseconds(),
	}
	if e.gitSource != nil {
		dispatch.GitRepoURL = e.gitSource.RepoURL
		dispatch.GitRef = e.gitSource.Ref
		dispatch.GitCommit = e.gitSource.Commit
		dispatch.GitToken = e.gitSource.Token
	}

	// Send to worker
	select {
	case worker.TaskCh <- dispatch:
		log.Debug().
			Str("task", t.Name).
			Str("worker", worker.ID).
			Str("assignment", assignment.ID.String()).
			Msg("Task dispatched to worker")
	case <-ctx.Done():
		return &executor.Result{
			Task:      t.Name,
			Status:    executor.Cancelled,
			Error:     ctx.Err(),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}
	}

	// Update assignment status to running
	_ = e.dispatcher.store.UpdateTaskAssignmentStatus(ctx, assignment.ID, models.TaskAssignmentRunning, nil)

	// Wait for result (the worker will send back via TaskEvent which gets
	// processed by the gRPC handler and updates the assignment in DB).
	// For now, we poll the assignment status.
	result := e.waitForResult(ctx, assignment.ID, t.Name, startTime)

	// Update assignment with final status
	status := models.TaskAssignmentCompleted
	if result.Status != executor.Success {
		status = models.TaskAssignmentFailed
	}
	resultJSON, _ := json.Marshal(map[string]interface{}{
		"status":    result.Status.String(),
		"exit_code": result.ExitCode,
		"error":     errorString(result.Error),
		"output":    result.Output,
	})
	_ = e.dispatcher.store.UpdateTaskAssignmentStatus(ctx, assignment.ID, status, resultJSON)

	return result
}

// DryRun simulates task dispatch without executing.
func (e *RemoteTaskExecutor) DryRun(t *task.Task) *executor.Result {
	available := e.dispatcher.registry.ListOnline()
	workerDesc := "no workers available"
	if len(available) > 0 {
		worker, err := e.dispatcher.router.SelectWorker(context.Background(), t, available)
		if err == nil {
			workerDesc = fmt.Sprintf("worker %s (%s)", worker.ID, worker.Info.Hostname)
		}
	}

	return &executor.Result{
		Task:      t.Name,
		Status:    executor.Skipped,
		Output:    fmt.Sprintf("[DRY RUN] Would dispatch to %s", workerDesc),
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}
}

// waitForResult polls the task assignment until it completes or times out.
func (e *RemoteTaskExecutor) waitForResult(ctx context.Context, assignmentID uuid.UUID, taskName string, startTime time.Time) *executor.Result {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return &executor.Result{
				Task:      taskName,
				Status:    executor.Cancelled,
				Error:     ctx.Err(),
				StartTime: startTime,
				EndTime:   time.Now(),
				Duration:  time.Since(startTime),
			}
		case <-ticker.C:
			a, err := e.dispatcher.store.GetTaskAssignment(ctx, assignmentID)
			if err != nil {
				continue
			}
			switch a.Status {
			case models.TaskAssignmentCompleted:
				return parseAssignmentResult(a, taskName, startTime)
			case models.TaskAssignmentFailed:
				return parseAssignmentResult(a, taskName, startTime)
			}
			// Still running, continue polling
		}
	}
}

func parseAssignmentResult(a *models.TaskAssignment, taskName string, startTime time.Time) *executor.Result {
	result := &executor.Result{
		Task:      taskName,
		StartTime: startTime,
		EndTime:   time.Now(),
		Duration:  time.Since(startTime),
	}

	if a.Status == models.TaskAssignmentCompleted {
		result.Status = executor.Success
	} else {
		result.Status = executor.Failed
	}

	if a.Result != nil {
		var parsed struct {
			ExitCode int    `json:"exit_code"`
			Error    string `json:"error"`
			Output   string `json:"output"`
		}
		if err := json.Unmarshal(a.Result, &parsed); err == nil {
			result.ExitCode = parsed.ExitCode
			result.Output = parsed.Output
			if parsed.Error != "" {
				result.Error = fmt.Errorf("%s", parsed.Error)
			}
		}
	}

	return result
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
