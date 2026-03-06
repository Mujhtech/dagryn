package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/service"
)

// ListTeamAuditLogs godoc
//
//	@Summary		List team audit logs
//	@Description	Returns audit logs for a team with cursor-based pagination and optional filters
//	@Tags			audit-logs
//	@Security		BearerAuth
//	@Produce		json
//	@Param			teamID		path		string	true	"Team ID"
//	@Param			actor_id	query		string	false	"Filter by actor ID"
//	@Param			actor_email	query		string	false	"Filter by actor email"
//	@Param			action		query		string	false	"Filter by action"
//	@Param			category	query		string	false	"Filter by category"
//	@Param			since		query		string	false	"Filter entries after this time (RFC3339)"
//	@Param			until		query		string	false	"Filter entries before this time (RFC3339)"
//	@Param			cursor		query		string	false	"Cursor for pagination"
//	@Param			limit		query		int		false	"Items per page"	default(50)
//	@Success		200			{object}	AuditLogListResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamID}/audit-logs [get]
func (h *Handler) ListTeamAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid team ID"))
		return
	}

	// Check team membership and audit log permission.
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view audit logs"))
		return
	}

	if h.auditService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("audit log service not configured"))
		return
	}

	filter := parseAuditLogFilter(r, teamID)
	result, err := h.auditService.List(ctx, filter)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list audit logs"))
		return
	}

	// Emit meta-audit entry synchronously (avoids using cancelled HTTP context in a goroutine).
	h.auditService.Log(ctx, service.AuditEntry{
		TeamID:       teamID,
		Action:       models.AuditActionAuditViewed,
		Category:     models.AuditCategoryAudit,
		ResourceType: "audit_log",
		Description:  "Audit logs viewed",
	})

	_ = response.Ok(w, r, "Success", result)
}

// ListProjectAuditLogs godoc
//
//	@Summary		List project audit logs
//	@Description	Returns audit logs scoped to a specific project
//	@Tags			audit-logs
//	@Security		BearerAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			actor_id	query		string	false	"Filter by actor ID"
//	@Param			actor_email	query		string	false	"Filter by actor email"
//	@Param			action		query		string	false	"Filter by action"
//	@Param			category	query		string	false	"Filter by category"
//	@Param			since		query		string	false	"Filter entries after this time (RFC3339)"
//	@Param			until		query		string	false	"Filter entries before this time (RFC3339)"
//	@Param			cursor		query		string	false	"Cursor for pagination"
//	@Param			limit		query		int		false	"Items per page"	default(50)
//	@Success		200			{object}	AuditLogListResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/audit-logs [get]
func (h *Handler) ListProjectAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := getProjectIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Get project to find the team.
	project, err := h.store.Projects.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("project not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get project"))
		return
	}

	if project.TeamID == nil {
		_ = response.BadRequest(w, r, errors.New("project is not part of a team"))
		return
	}

	// Check team membership and permission.
	member, err := h.store.Teams.GetMember(ctx, *project.TeamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project's team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view audit logs"))
		return
	}

	if h.auditService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("audit log service not configured"))
		return
	}

	filter := parseAuditLogFilter(r, *project.TeamID)
	filter.ProjectID = &projectID
	result, err := h.auditService.List(ctx, filter)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list audit logs"))
		return
	}

	// Emit meta-audit entry.
	h.auditService.Log(ctx, service.AuditEntry{
		TeamID:       *project.TeamID,
		ProjectID:    &projectID,
		Action:       models.AuditActionAuditViewed,
		Category:     models.AuditCategoryAudit,
		ResourceType: "audit_log",
		Description:  "Project audit logs viewed",
	})

	_ = response.Ok(w, r, "Success", result)
}

// GetAuditLog godoc
//
//	@Summary		Get audit log entry
//	@Description	Returns a single audit log entry by ID
//	@Tags			audit-logs
//	@Security		BearerAuth
//	@Produce		json
//	@Param			auditLogId	path		string	true	"Audit Log ID"
//	@Success		200			{object}	AuditLogResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/audit-logs/{auditLogId} [get]
func (h *Handler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	logID, err := getAuditLogIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	if h.auditService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("audit log service not configured"))
		return
	}

	entry, err := h.auditService.GetByID(ctx, logID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("audit log entry not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get audit log entry"))
		return
	}

	// Verify the user has access to the team this entry belongs to.
	member, err := h.store.Teams.GetMember(ctx, entry.TeamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this audit log"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view audit logs"))
		return
	}

	_ = response.Ok(w, r, "Success", entry)
}

// ExportTeamAuditLogs godoc
//
//	@Summary		Export team audit logs
//	@Description	Exports audit logs as CSV or JSON file download
//	@Tags			audit-logs
//	@Security		BearerAuth
//	@Produce		json
//	@Produce		text/csv
//	@Param			teamId		path		string	true	"Team ID"
//	@Param			format		query		string	false	"Export format (csv or json)"	default(json)	Enums(csv, json)
//	@Param			actor_id	query		string	false	"Filter by actor ID"
//	@Param			actor_email	query		string	false	"Filter by actor email"
//	@Param			action		query		string	false	"Filter by action"
//	@Param			category	query		string	false	"Filter by category"
//	@Param			since		query		string	false	"Filter entries after this time (RFC3339)"
//	@Param			until		query		string	false	"Filter entries before this time (RFC3339)"
//	@Success		200			{file}		file
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/export [get]
func (h *Handler) ExportTeamAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsExport) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to export audit logs"))
		return
	}

	if h.auditService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("audit log service not configured"))
		return
	}

	filter := parseAuditLogFilter(r, teamID)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=audit-logs.csv")
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=audit-logs.json")
		format = "json"
	}

	if err := h.auditService.Export(ctx, filter, format, w); err != nil {
		// Headers already sent; log and return.
		return
	}

	// Emit meta-audit entry synchronously (avoids using cancelled HTTP context in a goroutine).
	h.auditService.Log(ctx, service.AuditEntry{
		TeamID:       teamID,
		Action:       models.AuditActionAuditExported,
		Category:     models.AuditCategoryAudit,
		ResourceType: "audit_log",
		Description:  "Audit logs exported as " + format,
		Metadata:     map[string]interface{}{"format": format},
	})
}

// GetAuditRetentionPolicy godoc
//
//	@Summary		Get retention policy
//	@Description	Returns the audit log retention policy for a team
//	@Tags			audit-logs
//	@Security		BearerAuth
//	@Produce		json
//	@Param			teamId	path		string	true	"Team ID"
//	@Success		200		{object}	AuditRetentionPolicyResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		503		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/retention [get]
func (h *Handler) GetAuditRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view audit log settings"))
		return
	}

	if h.auditService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("audit log service not configured"))
		return
	}

	policy, err := h.auditService.GetRetention(ctx, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to get retention policy"))
		return
	}

	_ = response.Ok(w, r, "Success", policy)
}

// UpdateAuditRetentionPolicy godoc
//
//	@Summary		Update retention policy
//	@Description	Updates the audit log retention policy for a team
//	@Tags			audit-logs
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			teamId	path		string						true	"Team ID"
//	@Param			body	body		UpdateRetentionPolicyRequest	true	"Retention policy update"
//	@Success		200		{object}	AuditRetentionPolicyResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		503		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/retention [put]
func (h *Handler) UpdateAuditRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage audit log settings"))
		return
	}

	if h.auditService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("audit log service not configured"))
		return
	}

	var req struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.RetentionDays < 1 || req.RetentionDays > 3650 {
		_ = response.BadRequest(w, r, errors.New("retention_days must be between 1 and 3650"))
		return
	}

	if err := h.auditService.UpdateRetention(ctx, teamID, req.RetentionDays, &user.ID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update retention policy"))
		return
	}

	_ = response.Ok(w, r, "Retention policy updated", map[string]interface{}{
		"team_id":        teamID,
		"retention_days": req.RetentionDays,
	})
}

// VerifyAuditChain godoc
//
//	@Summary		Verify audit chain integrity
//	@Description	Verifies the hash chain integrity of audit log entries for a team
//	@Tags			audit-logs
//	@Security		BearerAuth
//	@Produce		json
//	@Param			teamId	path		string	true	"Team ID"
//	@Success		200		{object}	AuditChainVerifyResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		503		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/audit-logs/verify [get]
func (h *Handler) VerifyAuditChain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}
	if !member.Role.HasPermission(models.PermissionAuditLogsManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to verify audit logs"))
		return
	}

	if h.auditService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("audit log service not configured"))
		return
	}

	result, err := h.auditService.VerifyChain(ctx, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to verify audit chain"))
		return
	}

	_ = response.Ok(w, r, "Chain verification complete", result)
}

// parseAuditLogFilter extracts filter parameters from the query string.
func parseAuditLogFilter(r *http.Request, teamID uuid.UUID) repo.AuditLogFilter {
	filter := repo.AuditLogFilter{
		TeamID: teamID,
	}

	q := r.URL.Query()

	if v := q.Get("actor_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.ActorID = &id
		}
	}
	if v := q.Get("actor_email"); v != "" {
		filter.ActorEmail = v
	}
	if v := q.Get("action"); v != "" {
		filter.Action = v
	}
	if v := q.Get("category"); v != "" {
		filter.Category = v
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = &t
		}
	}
	if v := q.Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Until = &t
		}
	}
	if v := q.Get("cursor"); v != "" {
		filter.Cursor = v
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}

	return filter
}
