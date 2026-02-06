package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/server/response"
)

// ListProjectWorkflows lists all workflows for a project.
// @Summary List project workflows
// @Description Get all workflows synced to a project
// @Tags workflows
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Success 200 {array} WorkflowResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /projects/{projectID}/workflows [get]
func (h *Handler) ListProjectWorkflows(w http.ResponseWriter, r *http.Request) {
	projectID, err := ParseUUID(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Get workflows with tasks
	workflows, err := h.workflows.ListByProjectWithTasks(r.Context(), projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to fetch workflows"))
		return
	}

	// Convert to response format
	resp := make([]WorkflowResponse, len(workflows))
	for i, wf := range workflows {
		resp[i] = toWorkflowResponse(wf)
	}

	_ = response.Ok(w, r, "Success", resp)
}

// SyncProjectWorkflow syncs a workflow from the CLI.
// @Summary Sync workflow
// @Description Sync workflow configuration from CLI to server
// @Tags workflows
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param request body SyncWorkflowRequest true "Workflow to sync"
// @Success 200 {object} SyncWorkflowResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /projects/{projectID}/workflows/sync [post]
func (h *Handler) SyncProjectWorkflow(w http.ResponseWriter, r *http.Request) {
	projectID, err := ParseUUID(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Verify project exists
	project, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to fetch project"))
		return
	}
	if project == nil {
		_ = response.NotFound(w, r, errors.New("project not found"))
		return
	}

	// Parse request
	var req SyncWorkflowRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.Name == "" {
		req.Name = "default"
	}

	// Create workflow model
	workflow := &models.ProjectWorkflow{
		ProjectID: projectID,
		Name:      req.Name,
		IsDefault: req.IsDefault,
	}
	if req.ConfigHash != "" {
		workflow.ConfigHash = &req.ConfigHash
	}
	if req.RawConfig != "" {
		workflow.RawConfig = &req.RawConfig
	}

	// Upsert workflow
	changed, err := h.workflows.Upsert(r.Context(), workflow)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to sync workflow"))
		return
	}

	// Convert tasks
	tasks := make([]models.WorkflowTask, len(req.Tasks))
	for i, t := range req.Tasks {
		tasks[i] = models.WorkflowTask{
			WorkflowID:     workflow.ID,
			Name:           t.Name,
			Command:        t.Command,
			Needs:          t.Needs,
			Inputs:         t.Inputs,
			Outputs:        t.Outputs,
			Plugins:        t.Plugins,
			TimeoutSeconds: t.TimeoutSeconds,
			Workdir:        t.Workdir,
			Env:            t.Env,
		}
	}

	// Upsert tasks
	if err := h.workflows.UpsertTasks(r.Context(), workflow.ID, tasks); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to sync workflow tasks"))
		return
	}

	message := "Workflow synced successfully"
	if changed {
		message = "Workflow updated successfully"
	}

	_ = response.Ok(w, r, message, SyncWorkflowResponse{
		WorkflowID: workflow.ID,
		Name:       workflow.Name,
		TaskCount:  len(tasks),
		Changed:    changed,
		Message:    message,
	})
}

// GetRunWorkflow gets the workflow snapshot for a specific run.
// @Summary Get run workflow
// @Description Get the workflow snapshot used for a specific run
// @Tags workflows
// @Produce json
// @Param projectID path string true "Project ID"
// @Param runID path string true "Run ID"
// @Success 200 {object} WorkflowResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /projects/{projectID}/runs/{runID}/workflow [get]
func (h *Handler) GetRunWorkflow(w http.ResponseWriter, r *http.Request) {
	runID, err := ParseUUID(chi.URLParam(r, "runID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid run ID"))
		return
	}

	// Get the run to find its workflow ID
	run, err := h.runs.GetByID(r.Context(), runID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to fetch run"))
		return
	}
	if run == nil {
		_ = response.NotFound(w, r, errors.New("run not found"))
		return
	}

	// Check if run has a workflow linked
	if run.WorkflowID == nil {
		_ = response.NotFound(w, r, errors.New("run has no workflow snapshot"))
		return
	}

	// Get the workflow
	workflow, err := h.workflows.GetByID(r.Context(), *run.WorkflowID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to fetch workflow"))
		return
	}
	if workflow == nil {
		_ = response.NotFound(w, r, errors.New("workflow not found"))
		return
	}

	_ = response.Ok(w, r, "Success", toWorkflowResponse(*workflow))
}

// toWorkflowResponse converts a WorkflowWithTasks to WorkflowResponse.
func toWorkflowResponse(wf models.WorkflowWithTasks) WorkflowResponse {
	tasks := make([]WorkflowTaskResponse, len(wf.Tasks))
	for i, t := range wf.Tasks {
		tasks[i] = WorkflowTaskResponse{
			Name:           t.Name,
			Command:        t.Command,
			Needs:          t.Needs,
			Inputs:         t.Inputs,
			Outputs:        t.Outputs,
			Plugins:        t.Plugins,
			TimeoutSeconds: t.TimeoutSeconds,
			Workdir:        t.Workdir,
			Env:            t.Env,
		}
	}

	return WorkflowResponse{
		ID:        wf.ID,
		Name:      wf.Name,
		Version:   wf.Version,
		IsDefault: wf.IsDefault,
		SyncedAt:  wf.SyncedAt,
		Tasks:     tasks,
	}
}
