package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ── Cluster handlers ──

// ListClusters godoc
//
//	@Summary		List clusters
//	@Description	Returns all clusters the current user has access to, optionally filtered by team
//	@Tags			clusters
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			team_id	query		string	false	"Filter by team ID"	format(uuid)
//	@Success		200		{array}		ClusterResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/clusters [get]
func (h *Handler) ListClusters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var teamID *uuid.UUID
	if tid := r.URL.Query().Get("team_id"); tid != "" {
		id, err := uuid.Parse(tid)
		if err != nil {
			_ = response.BadRequest(w, r, fmt.Errorf("invalid team_id"))
			return
		}
		if _, err := h.store.Teams.GetMember(ctx, id, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}
		teamID = &id
	}

	clusters, err := h.listAccessibleClusters(ctx, user.ID, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})
	_ = response.Ok(w, r, "clusters", clusters)
}

// CreateCluster godoc
//
//	@Summary		Create a cluster
//	@Description	Creates a new cluster for the current user or team
//	@Tags			clusters
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateClusterRequest	true	"Create cluster request"
//	@Success		201		{object}	ClusterResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/clusters [post]
func (h *Handler) CreateCluster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Labels      map[string]string `json:"labels"`
		TeamID      *uuid.UUID        `json:"team_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}
	if req.Name == "" {
		_ = response.BadRequest(w, r, errors.New("name is required"))
		return
	}

	labelsJSON, _ := json.Marshal(req.Labels)
	scopeType := "personal"
	var teamID *uuid.UUID
	var ownerUserID *uuid.UUID = &user.ID
	if req.TeamID != nil {
		if _, err := h.store.Teams.GetMember(ctx, *req.TeamID, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}
		scopeType = "team"
		teamID = req.TeamID
		ownerUserID = nil
	}

	cluster := &models.Cluster{
		Name:          req.Name,
		Slug:          generateSlug(req.Name),
		Description:   req.Description,
		Labels:        labelsJSON,
		ScopeType:     scopeType,
		TeamID:        teamID,
		OwnerUserID:   ownerUserID,
		SystemDefault: false,
	}

	if err := h.store.Clusters.CreateCluster(ctx, cluster); err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			_ = response.Conflict(w, r, errors.New("cluster name already exists in this scope"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Created(w, r, "cluster created", cluster)
}

// GetCluster godoc
//
//	@Summary		Get a cluster
//	@Description	Returns a cluster by its ID
//	@Tags			clusters
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			clusterId	path		string	true	"Cluster ID"	format(uuid)
//	@Success		200			{object}	ClusterResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/clusters/{clusterId} [get]
func (h *Handler) GetCluster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	id, err := getClusterIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	cluster, err := h.store.Clusters.GetCluster(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("cluster not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get cluster"))
		return
	}

	if cluster.TeamID != nil {
		if _, err := h.store.Teams.GetMember(ctx, *cluster.TeamID, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}
	} else if cluster.OwnerUserID != nil && *cluster.OwnerUserID != user.ID {
		_ = response.Forbidden(w, r, errors.New("you do not own this personal cluster"))
		return
	}

	_ = response.Ok(w, r, "cluster", cluster)
}

// UpdateCluster godoc
//
//	@Summary		Update a cluster
//	@Description	Updates an existing cluster's name, description, or labels
//	@Tags			clusters
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			clusterId	path		string					true	"Cluster ID"	format(uuid)
//	@Param			body		body		UpdateClusterRequest	true	"Update cluster request"
//	@Success		200			{object}	ClusterResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/clusters/{clusterId} [put]
func (h *Handler) UpdateCluster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	id, err := getClusterIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	cluster, err := h.store.Clusters.GetCluster(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("cluster not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get cluster"))
		return
	}

	if cluster.TeamID != nil {
		if _, err := h.store.Teams.GetMember(ctx, *cluster.TeamID, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}
	} else if cluster.OwnerUserID != nil && *cluster.OwnerUserID != user.ID {
		_ = response.Forbidden(w, r, errors.New("you do not own this personal cluster"))
		return
	}
	if cluster.SystemDefault {
		_ = response.BadRequest(w, r, errors.New("cannot modify default system cluster"))
		return
	}

	var req struct {
		Name        *string           `json:"name"`
		Description *string           `json:"description"`
		Labels      map[string]string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if req.Name != nil {
		cluster.Name = *req.Name
		cluster.Slug = generateSlug(*req.Name)
	}
	if req.Description != nil {
		cluster.Description = *req.Description
	}
	if req.Labels != nil {
		labelsJSON, _ := json.Marshal(req.Labels)
		cluster.Labels = labelsJSON
	}

	if err := h.store.Clusters.UpdateCluster(ctx, cluster); err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			_ = response.Conflict(w, r, errors.New("cluster name already exists in this scope"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "cluster updated", cluster)
}

// DeleteCluster godoc
//
//	@Summary		Delete a cluster
//	@Description	Deletes a cluster by its ID
//	@Tags			clusters
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			clusterId	path	string	true	"Cluster ID"	format(uuid)
//	@Success		204			"No Content"
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/clusters/{clusterId} [delete]
func (h *Handler) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	id, err := getClusterIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	cluster, err := h.store.Clusters.GetCluster(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("cluster not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get cluster"))
		return
	}
	if cluster.TeamID != nil {
		if _, err := h.store.Teams.GetMember(ctx, *cluster.TeamID, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}
	} else if cluster.OwnerUserID != nil && *cluster.OwnerUserID != user.ID {
		_ = response.Forbidden(w, r, errors.New("you do not own this personal cluster"))
		return
	}

	if err := h.store.Clusters.DeleteCluster(ctx, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("cluster not found"))
			return
		}
		if strings.Contains(err.Error(), "cannot delete") {
			_ = response.Conflict(w, r, err)
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.NoContent(w, r)
}

// ── Worker handlers ──

// ListWorkers godoc
//
//	@Summary		List workers
//	@Description	Returns all workers the current user has access to, optionally filtered by cluster, team, and status
//	@Tags			workers
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			cluster_id	query		string	false	"Filter by cluster ID"	format(uuid)
//	@Param			team_id		query		string	false	"Filter by team ID"		format(uuid)
//	@Param			status		query		string	false	"Filter by status"		Enums(online, draining, offline)
//	@Success		200			{array}		WorkerResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/workers [get]
func (h *Handler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var clusterID *uuid.UUID
	var workerStatus *models.WorkerStatus

	if cid := r.URL.Query().Get("cluster_id"); cid != "" {
		id, err := uuid.Parse(cid)
		if err != nil {
			_ = response.BadRequest(w, r, fmt.Errorf("invalid cluster_id"))
			return
		}
		clusterID = &id
	}
	if s := r.URL.Query().Get("status"); s != "" {
		status := models.WorkerStatus(s)
		workerStatus = &status
	}

	if clusterID != nil {
		if err := h.authorizeClusterAccess(ctx, user.ID, *clusterID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.NotFound(w, r, errors.New("cluster not found"))
				return
			}
			_ = response.Forbidden(w, r, err)
			return
		}
		workers, err := h.store.Clusters.ListWorkers(ctx, clusterID, workerStatus)
		if err != nil {
			_ = response.InternalServerError(w, r, err)
			return
		}
		_ = response.Ok(w, r, "workers", workers)
		return
	}

	var teamID *uuid.UUID
	if tid := r.URL.Query().Get("team_id"); tid != "" {
		id, err := uuid.Parse(tid)
		if err != nil {
			_ = response.BadRequest(w, r, fmt.Errorf("invalid team_id"))
			return
		}
		if _, err := h.store.Teams.GetMember(ctx, id, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}
		teamID = &id
	}

	clusters, err := h.listAccessibleClusters(ctx, user.ID, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	combined := make([]models.Worker, 0)
	seen := make(map[uuid.UUID]struct{})
	for _, c := range clusters {
		cid := c.ID
		workers, err := h.store.Clusters.ListWorkers(ctx, &cid, workerStatus)
		if err != nil {
			_ = response.InternalServerError(w, r, err)
			return
		}
		for _, wkr := range workers {
			if _, ok := seen[wkr.ID]; ok {
				continue
			}
			seen[wkr.ID] = struct{}{}
			combined = append(combined, wkr)
		}
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Hostname < combined[j].Hostname
	})

	_ = response.Ok(w, r, "workers", combined)
}

// GetWorker godoc
//
//	@Summary		Get a worker
//	@Description	Returns a worker by its ID
//	@Tags			workers
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			workerId	path		string	true	"Worker ID"	format(uuid)
//	@Success		200			{object}	WorkerResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/workers/{workerId} [get]
func (h *Handler) GetWorker(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	id, err := getWorkerIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	worker, err := h.store.Clusters.GetWorker(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("worker not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get worker"))
		return
	}
	if worker.ClusterID != nil {
		if err := h.authorizeClusterAccess(ctx, user.ID, *worker.ClusterID); err != nil {
			_ = response.Forbidden(w, r, err)
			return
		}
	}
	_ = response.Ok(w, r, "worker", worker)
}

// DeleteWorker godoc
//
//	@Summary		Delete a worker
//	@Description	Deregisters a worker by its ID
//	@Tags			workers
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			workerId	path	string	true	"Worker ID"	format(uuid)
//	@Success		204			"No Content"
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/workers/{workerId} [delete]
func (h *Handler) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	id, err := getWorkerIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	worker, err := h.store.Clusters.GetWorker(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("worker not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get worker"))
		return
	}
	if worker.ClusterID != nil {
		if err := h.authorizeClusterAccess(ctx, user.ID, *worker.ClusterID); err != nil {
			_ = response.Forbidden(w, r, err)
			return
		}
	}

	if err := h.store.Clusters.DeleteWorker(ctx, id); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.NoContent(w, r)
}

// DrainWorker godoc
//
//	@Summary		Drain a worker
//	@Description	Sets a worker to draining status, preventing new task assignments while allowing in-progress tasks to complete
//	@Tags			workers
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			workerId	path		string	true	"Worker ID"	format(uuid)
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/workers/{workerId}/drain [post]
func (h *Handler) DrainWorker(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	id, err := getWorkerIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	worker, err := h.store.Clusters.GetWorker(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("worker not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get worker"))
		return
	}
	if worker.ClusterID != nil {
		if err := h.authorizeClusterAccess(ctx, user.ID, *worker.ClusterID); err != nil {
			_ = response.Forbidden(w, r, err)
			return
		}
	}

	if err := h.store.Clusters.UpdateWorkerStatus(ctx, id, models.WorkerStatusDraining); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "worker draining", nil)
}

// ── Task Assignment handlers ──

// ListRunAssignments godoc
//
//	@Summary		List run task assignments
//	@Description	Returns all task assignments for a specific run, showing which workers executed which tasks
//	@Tags			task-assignments
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			runId	path		string	true	"Run ID"	format(uuid)
//	@Success		200		{array}		TaskAssignmentResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/runs/{runId}/assignments [get]
func (h *Handler) ListRunAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	runID, err := getRunIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	run, err := h.store.Runs.GetByID(ctx, runID)
	if err != nil {
		_ = response.NotFound(w, r, errors.New("run not found"))
		return
	}
	project, err := h.store.Projects.GetByID(ctx, run.ProjectID)
	if err != nil {
		_ = response.NotFound(w, r, errors.New("project not found"))
		return
	}
	if project.TeamID != nil {
		if _, err := h.store.Teams.GetMember(ctx, *project.TeamID, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}
	} else {
		if _, err := h.store.Projects.GetMember(ctx, run.ProjectID, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you do not have access to this personal project"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check project membership"))
			return
		}
	}

	assignments, err := h.store.Clusters.ListTaskAssignmentsByRun(ctx, runID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "task assignments", assignments)
}

func (h *Handler) authorizeClusterAccess(ctx context.Context, userID uuid.UUID, clusterID uuid.UUID) error {
	cluster, err := h.store.Clusters.GetCluster(ctx, clusterID)
	if err != nil {
		return err
	}
	if cluster.TeamID != nil {
		if _, err := h.store.Teams.GetMember(ctx, *cluster.TeamID, userID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				return errors.New("you are not a member of this team")
			}
			return errors.New("failed to check team membership")
		}
		return nil
	}
	if cluster.OwnerUserID != nil && *cluster.OwnerUserID != userID {
		return errors.New("you do not own this personal cluster")
	}
	return nil
}

func (h *Handler) listAccessibleClusters(ctx context.Context, userID uuid.UUID, teamID *uuid.UUID) ([]models.Cluster, error) {
	if teamID != nil {
		return h.store.Clusters.ListClustersForScope(ctx, teamID, &userID)
	}

	merged := make(map[uuid.UUID]models.Cluster)

	personal, err := h.store.Clusters.ListClustersForScope(ctx, nil, &userID)
	if err != nil {
		return nil, err
	}
	for _, c := range personal {
		merged[c.ID] = c
	}

	teams, err := h.store.Teams.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, tm := range teams {
		teamClusters, listErr := h.store.Clusters.ListClustersForScope(ctx, &tm.ID, &userID)
		if listErr != nil {
			return nil, listErr
		}
		for _, c := range teamClusters {
			merged[c.ID] = c
		}
	}

	all, err := h.store.Clusters.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range all {
		if c.ScopeType == "global" {
			merged[c.ID] = c
		}
	}

	out := make([]models.Cluster, 0, len(merged))
	for _, c := range merged {
		out = append(out, c)
	}
	return out, nil
}

// ── Helpers ──

func getClusterIDFromPath(r *http.Request) (uuid.UUID, error) {
	idStr, err := pathParamOrError(r, "clusterId")
	if err != nil {
		return uuid.Nil, fmt.Errorf("cluster ID is required")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid cluster ID")
	}
	return id, nil
}

func getWorkerIDFromPath(r *http.Request) (uuid.UUID, error) {
	idStr, err := pathParamOrError(r, "workerId")
	if err != nil {
		return uuid.Nil, fmt.Errorf("worker ID is required")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid worker ID")
	}
	return id, nil
}
