package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ── Cluster handlers ──

// ListClusters returns all clusters.
func (h *Handler) ListClusters(w http.ResponseWriter, r *http.Request) {
	clusters, err := h.store.Clusters.ListClusters(r.Context())
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "clusters", clusters)
}

// CreateCluster creates a new cluster.
func (h *Handler) CreateCluster(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Labels      map[string]string `json:"labels"`
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
	cluster := &models.Cluster{
		Name:        req.Name,
		Description: req.Description,
		Labels:      labelsJSON,
	}

	if err := h.store.Clusters.CreateCluster(r.Context(), cluster); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Created(w, r, "cluster created", cluster)
}

// GetCluster returns a cluster by ID.
func (h *Handler) GetCluster(w http.ResponseWriter, r *http.Request) {
	id, err := getClusterIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	cluster, err := h.store.Clusters.GetCluster(r.Context(), id)
	if err != nil {
		_ = response.NotFound(w, r, err)
		return
	}
	_ = response.Ok(w, r, "cluster", cluster)
}

// UpdateCluster updates a cluster.
func (h *Handler) UpdateCluster(w http.ResponseWriter, r *http.Request) {
	id, err := getClusterIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	cluster, err := h.store.Clusters.GetCluster(r.Context(), id)
	if err != nil {
		_ = response.NotFound(w, r, err)
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
	}
	if req.Description != nil {
		cluster.Description = *req.Description
	}
	if req.Labels != nil {
		labelsJSON, _ := json.Marshal(req.Labels)
		cluster.Labels = labelsJSON
	}

	if err := h.store.Clusters.UpdateCluster(r.Context(), cluster); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "cluster updated", cluster)
}

// DeleteCluster deletes a cluster.
func (h *Handler) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	id, err := getClusterIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if err := h.store.Clusters.DeleteCluster(r.Context(), id); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.NoContent(w, r)
}

// ── Worker handlers ──

// ListWorkers returns all workers, optionally filtered by cluster and status.
func (h *Handler) ListWorkers(w http.ResponseWriter, r *http.Request) {
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

	workers, err := h.store.Clusters.ListWorkers(r.Context(), clusterID, workerStatus)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "workers", workers)
}

// GetWorker returns a worker by ID.
func (h *Handler) GetWorker(w http.ResponseWriter, r *http.Request) {
	id, err := getWorkerIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	worker, err := h.store.Clusters.GetWorker(r.Context(), id)
	if err != nil {
		_ = response.NotFound(w, r, err)
		return
	}
	_ = response.Ok(w, r, "worker", worker)
}

// DeleteWorker deregisters a worker.
func (h *Handler) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	id, err := getWorkerIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if err := h.store.Clusters.DeleteWorker(r.Context(), id); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.NoContent(w, r)
}

// DrainWorker sets a worker to draining status.
func (h *Handler) DrainWorker(w http.ResponseWriter, r *http.Request) {
	id, err := getWorkerIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if err := h.store.Clusters.UpdateWorkerStatus(r.Context(), id, models.WorkerStatusDraining); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "worker draining", nil)
}

// ── Task Assignment handlers ──

// ListRunAssignments returns task assignments for a specific run.
func (h *Handler) ListRunAssignments(w http.ResponseWriter, r *http.Request) {
	runID, err := getRunIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	assignments, err := h.store.Clusters.ListTaskAssignmentsByRun(r.Context(), runID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "task assignments", assignments)
}

// ── Helpers ──

func getClusterIDFromPath(r *http.Request) (uuid.UUID, error) {
	idStr, err := pathParamOrError(r, ClusterIDParam)
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
	idStr, err := pathParamOrError(r, WorkerIDParam)
	if err != nil {
		return uuid.Nil, fmt.Errorf("worker ID is required")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid worker ID")
	}
	return id, nil
}
