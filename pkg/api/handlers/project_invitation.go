package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ListProjectInvitations godoc
//
//	@Summary		List project invitations
//	@Description	Returns all pending invitations for a project
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Success		200			{object}	[]InvitationResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/invitations [get]
func (h *Handler) ListProjectInvitations(w http.ResponseWriter, r *http.Request) {
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
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view invitations"))
		return
	}

	invitations, err := h.store.Invitations.ListByProject(ctx, projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list invitations"))
		return
	}

	resp := make([]InvitationResponse, 0, len(invitations))
	for _, inv := range invitations {
		resp = append(resp, invitationWithDetailsToResponse(&inv))
	}

	_ = response.Ok(w, r, "Success", resp)
}

// CreateProjectInvitation godoc
//
//	@Summary		Create project invitation
//	@Description	Creates an invitation to join a project
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			body		body		CreateInvitationRequest	true	"Create invitation request"
//	@Success		201			{object}	InvitationResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/invitations [post]
func (h *Handler) CreateProjectInvitation(w http.ResponseWriter, r *http.Request) {
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
		_ = response.Forbidden(w, r, errors.New("you don't have permission to create invitations"))
		return
	}

	var req CreateInvitationRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate email
	if req.Email == "" {
		_ = response.BadRequest(w, r, errors.New("email is required"))
		return
	}

	invRole := models.Role(req.Role)
	if !models.IsValidProjectRole(invRole) {
		_ = response.BadRequest(w, r, errors.New("invalid role"))
		return
	}

	if invRole == models.RoleOwner {
		_ = response.BadRequest(w, r, errors.New("cannot invite with owner role"))
		return
	}

	// Create the invitation
	invitation := &models.Invitation{
		Email:     req.Email,
		ProjectID: &projectID,
		Role:      invRole,
		InvitedBy: user.ID,
	}

	if err := h.store.Invitations.Create(ctx, invitation); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create invitation"))
		return
	}

	_ = response.Created(w, r, "Created successfully", invitationModelToResponse(invitation))
}

// RevokeProjectInvitation godoc
//
//	@Summary		Revoke project invitation
//	@Description	Revokes a pending project invitation
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			projectId		path	string	true	"Project ID"	format(uuid)
//	@Param			invitationID	path	string	true	"Invitation ID"	format(uuid)
//	@Success		204				"No Content"
//	@Failure		401				{object}	ErrorResponse
//	@Failure		403				{object}	ErrorResponse
//	@Failure		404				{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/invitations/{invitationID} [delete]
func (h *Handler) RevokeProjectInvitation(w http.ResponseWriter, r *http.Request) {
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

	invitationID, err := uuid.Parse(chi.URLParam(r, "invitationID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid invitation ID"))
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
		_ = response.Forbidden(w, r, errors.New("you don't have permission to revoke invitations"))
		return
	}

	if err := h.store.Invitations.Delete(ctx, invitationID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("invitation not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to revoke invitation"))
		return
	}

	_ = response.NoContent(w, r)
}
