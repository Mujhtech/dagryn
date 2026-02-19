package handlers

import (
	"errors"
	"net/http"
	"time"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ListProjectMembers godoc
//
//	@Summary		List project members
//	@Description	Returns all members of a project
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Success		200			{object}	[]ProjectMemberResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/members [get]
func (h *Handler) ListProjectMembers(w http.ResponseWriter, r *http.Request) {
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

	// Check user has access to project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to view members
	if !role.HasPermission(models.PermissionMembersView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view members"))
		return
	}

	members, err := h.store.Projects.ListMembers(ctx, projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list members"))
		return
	}

	resp := make([]ProjectMemberResponse, 0, len(members))
	for _, m := range members {
		resp = append(resp, projectMemberWithUserToResponse(&m))
	}

	_ = response.Ok(w, r, "Success", resp)
}

// AddProjectMember godoc
//
//	@Summary		Add project member
//	@Description	Adds a user to a project
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			body		body		AddProjectMemberRequest	true	"Add member request"
//	@Success		201			{object}	ProjectMemberResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/members [post]
func (h *Handler) AddProjectMember(w http.ResponseWriter, r *http.Request) {
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

	// Check user has access to project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to manage members
	if !role.HasPermission(models.PermissionMembersManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage members"))
		return
	}

	var req AddProjectMemberRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate role
	newRole := models.Role(req.Role)
	if !models.IsValidProjectRole(newRole) {
		_ = response.BadRequest(w, r, errors.New("invalid role: must be owner, admin, member, or viewer"))
		return
	}

	// Cannot assign owner role
	if newRole == models.RoleOwner {
		_ = response.BadRequest(w, r, errors.New("cannot assign owner role"))
		return
	}

	// Check if user exists
	targetUser, err := h.store.Users.GetByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("user not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get user"))
		return
	}

	// Check if already a member
	_, err = h.store.Projects.GetMember(ctx, projectID, req.UserID)
	if err == nil {
		_ = response.BadRequest(w, r, errors.New("user is already a member of this project"))
		return
	}
	if !errors.Is(err, repo.ErrNotFound) {
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	// Check team members quota
	if h.entitlements != nil {
		if err := h.entitlements.CheckQuota(ctx, "team_members", projectID, 0); err != nil {
			if entitlement.IsQuotaError(err) {
				_ = response.PaymentRequired(w, r, err)
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check quota"))
			return
		}
	}

	// Add member
	if err := h.store.Projects.AddMember(ctx, projectID, req.UserID, newRole, &user.ID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to add member"))
		return
	}

	_ = response.Created(w, r, "Created successfully", ProjectMemberResponse{
		User:     userModelToResponse(targetUser),
		Role:     string(newRole),
		JoinedAt: time.Now(),
	})
}

// RemoveProjectMember godoc
//
//	@Summary		Remove project member
//	@Description	Removes a user from a project
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			projectId	path	string	true	"Project ID"	format(uuid)
//	@Param			userID		path	string	true	"User ID"		format(uuid)
//	@Success		204			"No Content"
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/members/{userID} [delete]
func (h *Handler) RemoveProjectMember(w http.ResponseWriter, r *http.Request) {
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

	targetUserID, err := getUserIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Check user has access to project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Allow users to remove themselves, or admins/owners to remove others
	if user.ID != targetUserID && !role.HasPermission(models.PermissionMembersManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to remove members"))
		return
	}

	// Cannot remove the owner
	targetMember, err := h.store.Projects.GetMember(ctx, projectID, targetUserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("member not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get member"))
		return
	}

	if targetMember.Role == models.RoleOwner {
		_ = response.BadRequest(w, r, errors.New("cannot remove the project owner"))
		return
	}

	if err := h.store.Projects.RemoveMember(ctx, projectID, targetUserID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to remove member"))
		return
	}

	_ = response.NoContent(w, r)
}

// UpdateProjectMemberRole godoc
//
//	@Summary		Update project member role
//	@Description	Updates a project member's role
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			userID		path		string					true	"User ID"		format(uuid)
//	@Param			body		body		UpdateMemberRoleRequest	true	"Update role request"
//	@Success		200			{object}	ProjectMemberResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/members/{userID}/role [patch]
func (h *Handler) UpdateProjectMemberRole(w http.ResponseWriter, r *http.Request) {
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

	targetUserID, err := getUserIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Check user has access to project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to manage members
	if !role.HasPermission(models.PermissionMembersManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage members"))
		return
	}

	var req UpdateMemberRoleRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	newRole := models.Role(req.Role)
	if !models.IsValidProjectRole(newRole) {
		_ = response.BadRequest(w, r, errors.New("invalid role: must be owner, admin, member, or viewer"))
		return
	}

	// Cannot change owner role
	targetMember, err := h.store.Projects.GetMember(ctx, projectID, targetUserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("member not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get member"))
		return
	}

	if targetMember.Role == models.RoleOwner || newRole == models.RoleOwner {
		_ = response.BadRequest(w, r, errors.New("cannot change owner role"))
		return
	}

	if err := h.store.Projects.UpdateMemberRole(ctx, projectID, targetUserID, newRole); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update role"))
		return
	}

	targetUser, _ := h.store.Users.GetByID(ctx, targetUserID)
	_ = response.Ok(w, r, "Success", ProjectMemberResponse{
		User:     userModelToResponse(targetUser),
		Role:     string(newRole),
		JoinedAt: targetMember.JoinedAt,
	})
}
