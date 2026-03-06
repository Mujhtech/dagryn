package handlers

import (
	"errors"
	"net/http"
	"time"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/service"
)

// ListTeams godoc
//
//	@Summary		List teams
//	@Description	Returns all teams the current user is a member of
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			page		query		int	false	"Page number"		default(1)
//	@Param			per_page	query		int	false	"Items per page"	default(20)	maximum(100)
//	@Success		200			{object}	PaginatedResponse{data=[]TeamResponse}
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/v1/teams [get]
func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	teams, err := h.store.Teams.ListByUser(ctx, user.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list teams"))
		return
	}

	// Convert to response format
	resp := make([]TeamResponse, 0, len(teams))
	for _, t := range teams {
		resp = append(resp, teamWithMemberToResponse(&t))
	}

	// For now, return all teams (pagination can be added later)
	_ = response.Ok(w, r, "Teams retrieved successfully", PaginatedResponse{
		Data: resp,
		Meta: PaginationMeta{
			Page:       1,
			PerPage:    len(resp),
			Total:      int64(len(resp)),
			TotalPages: 1,
		},
	})
}

// CreateTeam godoc
//
//	@Summary		Create a team
//	@Description	Creates a new team with the current user as owner
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateTeamRequest	true	"Create team request"
//	@Success		201		{object}	TeamResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/v1/teams [post]
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req CreateTeamRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate required fields
	if req.Name == "" {
		_ = response.BadRequest(w, r, errors.New("team name is required"))
		return
	}

	// Generate slug if not provided
	slug := req.Slug
	if slug == "" {
		slug = generateSlug(req.Name)
	}

	// Check if slug exists
	exists, err := h.store.Teams.SlugExists(ctx, slug)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to check slug"))
		return
	}
	if exists {
		_ = response.BadRequest(w, r, errors.New("a team with this slug already exists"))
		return
	}

	// Create team
	team := &models.Team{
		Name:        req.Name,
		Slug:        slug,
		OwnerID:     user.ID,
		Description: &req.Description,
	}

	if err := h.store.Teams.Create(ctx, team); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create team"))
		return
	}

	// Audit log: team created
	if h.auditService != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       team.ID,
			Action:       models.AuditActionTeamCreated,
			Category:     models.AuditCategoryTeam,
			ResourceType: "team",
			ResourceID:   team.ID.String(),
			Description:  "Team created: " + team.Name,
		})
	}

	_ = response.Created(w, r, "Team created successfully", teamModelToResponse(team, models.RoleOwner))
}

// GetTeam godoc
//
//	@Summary		Get a team
//	@Description	Returns a team by ID
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			teamId	path		string	true	"Team ID"	format(uuid)
//	@Success		200		{object}	TeamResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId} [get]
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
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

	// Check membership
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	team, err := h.store.Teams.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("team not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get team"))
		return
	}

	_ = response.Ok(w, r, "Team retrieved successfully", teamModelToResponse(team, member.Role))
}

// UpdateTeam godoc
//
//	@Summary		Update a team
//	@Description	Updates a team's details
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			teamId	path		string				true	"Team ID"	format(uuid)
//	@Param			body	body		UpdateTeamRequest	true	"Update team request"
//	@Success		200		{object}	TeamResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId} [patch]
func (h *Handler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
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

	// Check membership and permissions
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	if !member.Role.CanManageMembers() {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to update this team"))
		return
	}

	var req UpdateTeamRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	team, err := h.store.Teams.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("team not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get team"))
		return
	}

	// Update fields if provided
	if req.Name != nil {
		team.Name = *req.Name
	}
	if req.Description != nil {
		team.Description = req.Description
	}

	if err := h.store.Teams.Update(ctx, team); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update team"))
		return
	}

	// Audit log: team updated
	if h.auditService != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       teamID,
			Action:       models.AuditActionTeamUpdated,
			Category:     models.AuditCategoryTeam,
			ResourceType: "team",
			ResourceID:   teamID.String(),
			Description:  "Team updated: " + team.Name,
		})
	}

	_ = response.Ok(w, r, "Team updated successfully", teamModelToResponse(team, member.Role))
}

// DeleteTeam godoc
//
//	@Summary		Delete a team
//	@Description	Deletes a team (requires owner role)
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			teamId	path	string	true	"Team ID"	format(uuid)
//	@Success		204		"No Content"
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId} [delete]
func (h *Handler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
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

	// Check membership and permissions (only owner can delete)
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	if member.Role != models.RoleOwner {
		_ = response.Forbidden(w, r, errors.New("only the owner can delete this team"))
		return
	}

	// Get team name before deletion for audit.
	team, _ := h.store.Teams.GetByID(ctx, teamID)

	// Audit log: team deleted (logged before deletion so the team_id still exists).
	if h.auditService != nil {
		desc := "Team deleted"
		if team != nil {
			desc = "Team deleted: " + team.Name
		}
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       teamID,
			Action:       models.AuditActionTeamDeleted,
			Category:     models.AuditCategoryTeam,
			ResourceType: "team",
			ResourceID:   teamID.String(),
			Description:  desc,
		})
	}

	if err := h.store.Teams.Delete(ctx, teamID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("team not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to delete team"))
		return
	}

	_ = response.NoContent(w, r)
}

// ListTeamMembers godoc
//
//	@Summary		List team members
//	@Description	Returns all members of a team
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			teamId	path		string	true	"Team ID"	format(uuid)
//	@Success		200		{object}	[]TeamMemberResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/members [get]
func (h *Handler) ListTeamMembers(w http.ResponseWriter, r *http.Request) {
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

	// Check membership
	_, err = h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	members, err := h.store.Teams.ListMembers(ctx, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list members"))
		return
	}

	resp := make([]TeamMemberResponse, 0, len(members))
	for _, m := range members {
		resp = append(resp, teamMemberWithUserToResponse(&m))
	}

	_ = response.Ok(w, r, "Team members retrieved successfully", resp)
}

// AddTeamMember godoc
//
//	@Summary		Add team member
//	@Description	Adds a user to a team
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			teamId	path		string					true	"Team ID"	format(uuid)
//	@Param			body	body		AddTeamMemberRequest	true	"Add member request"
//	@Success		201		{object}	TeamMemberResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/members [post]
func (h *Handler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
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

	// Check membership and permissions
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	if !member.Role.CanManageMembers() {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage members"))
		return
	}

	var req AddTeamMemberRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate role
	role := models.Role(req.Role)
	if !models.IsValidTeamRole(role) {
		_ = response.BadRequest(w, r, errors.New("invalid role: must be owner, admin, or member"))
		return
	}

	// Cannot assign owner role
	if role == models.RoleOwner {
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
	_, err = h.store.Teams.GetMember(ctx, teamID, req.UserID)
	if err == nil {
		_ = response.BadRequest(w, r, errors.New("user is already a member of this team"))
		return
	}
	if !errors.Is(err, repo.ErrNotFound) {
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	// Add member
	if err := h.store.Teams.AddMember(ctx, teamID, req.UserID, role, &user.ID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to add member"))
		return
	}

	// Audit log: member added
	if h.auditService != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       teamID,
			Action:       models.AuditActionMemberAdded,
			Category:     models.AuditCategoryMember,
			ResourceType: "team_member",
			ResourceID:   req.UserID.String(),
			Description:  "Member added to team",
			Metadata:     map[string]interface{}{"role": string(role), "target_email": targetUser.Email},
		})
	}

	_ = response.Created(w, r, "Member added successfully", TeamMemberResponse{
		User:     userModelToResponse(targetUser),
		Role:     string(role),
		JoinedAt: time.Now(),
	})
}

// RemoveTeamMember godoc
//
//	@Summary		Remove team member
//	@Description	Removes a user from a team
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			teamId	path	string	true	"Team ID"	format(uuid)
//	@Param			userID	path	string	true	"User ID"	format(uuid)
//	@Success		204		"No Content"
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/members/{userID} [delete]
func (h *Handler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
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

	targetUserID, err := getUserIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Check membership and permissions
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	// Allow users to remove themselves, or admins/owners to remove others
	if user.ID != targetUserID && !member.Role.CanManageMembers() {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to remove members"))
		return
	}

	// Cannot remove the owner
	targetMember, err := h.store.Teams.GetMember(ctx, teamID, targetUserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("member not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get member"))
		return
	}

	if targetMember.Role == models.RoleOwner {
		_ = response.BadRequest(w, r, errors.New("cannot remove the team owner"))
		return
	}

	if err := h.store.Teams.RemoveMember(ctx, teamID, targetUserID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to remove member"))
		return
	}

	// Audit log: member removed
	if h.auditService != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       teamID,
			Action:       models.AuditActionMemberRemoved,
			Category:     models.AuditCategoryMember,
			ResourceType: "team_member",
			ResourceID:   targetUserID.String(),
			Description:  "Member removed from team",
		})
	}

	_ = response.NoContent(w, r)
}

// UpdateTeamMemberRole godoc
//
//	@Summary		Update team member role
//	@Description	Updates a team member's role
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			teamId	path		string					true	"Team ID"	format(uuid)
//	@Param			userID	path		string					true	"User ID"	format(uuid)
//	@Param			body	body		UpdateMemberRoleRequest	true	"Update role request"
//	@Success		200		{object}	TeamMemberResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/members/{userID}/role [patch]
func (h *Handler) UpdateTeamMemberRole(w http.ResponseWriter, r *http.Request) {
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

	targetUserID, err := getUserIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Check membership and permissions
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	if !member.Role.CanManageMembers() {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to manage members"))
		return
	}

	var req UpdateMemberRoleRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	role := models.Role(req.Role)
	if !models.IsValidTeamRole(role) {
		_ = response.BadRequest(w, r, errors.New("invalid role: must be owner, admin, or member"))
		return
	}

	// Cannot change owner role
	targetMember, err := h.store.Teams.GetMember(ctx, teamID, targetUserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("member not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get member"))
		return
	}

	if targetMember.Role == models.RoleOwner || role == models.RoleOwner {
		_ = response.BadRequest(w, r, errors.New("cannot change owner role"))
		return
	}

	if err := h.store.Teams.UpdateMemberRole(ctx, teamID, targetUserID, role); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update role"))
		return
	}

	// Audit log: member role changed
	if h.auditService != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       teamID,
			Action:       models.AuditActionMemberRoleChanged,
			Category:     models.AuditCategoryMember,
			ResourceType: "team_member",
			ResourceID:   targetUserID.String(),
			Description:  "Member role changed",
			Metadata: map[string]interface{}{
				"old_role": string(targetMember.Role),
				"new_role": string(role),
			},
		})
	}

	targetUser, _ := h.store.Users.GetByID(ctx, targetUserID)
	_ = response.Ok(w, r, "Member role updated successfully", TeamMemberResponse{
		User:     userModelToResponse(targetUser),
		Role:     string(role),
		JoinedAt: targetMember.JoinedAt,
	})
}

// ListTeamInvitations godoc
//
//	@Summary		List team invitations
//	@Description	Returns all pending invitations for a team
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			teamId	path		string	true	"Team ID"	format(uuid)
//	@Success		200		{object}	[]InvitationResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/invitations [get]
func (h *Handler) ListTeamInvitations(w http.ResponseWriter, r *http.Request) {
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

	// Check membership
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	if !member.Role.CanManageMembers() {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view invitations"))
		return
	}

	invitations, err := h.store.Invitations.ListByTeam(ctx, teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list invitations"))
		return
	}

	resp := make([]InvitationResponse, 0, len(invitations))
	for _, inv := range invitations {
		resp = append(resp, invitationWithDetailsToResponse(&inv))
	}

	_ = response.Ok(w, r, "Invitations retrieved successfully", resp)
}

// CreateTeamInvitation godoc
//
//	@Summary		Create team invitation
//	@Description	Creates an invitation to join a team
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			teamId	path		string					true	"Team ID"	format(uuid)
//	@Param			body	body		CreateInvitationRequest	true	"Create invitation request"
//	@Success		201		{object}	InvitationResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/invitations [post]
func (h *Handler) CreateTeamInvitation(w http.ResponseWriter, r *http.Request) {
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

	// Check membership and permissions
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	if !member.Role.CanManageMembers() {
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

	role := models.Role(req.Role)
	if !models.IsValidTeamRole(role) {
		_ = response.BadRequest(w, r, errors.New("invalid role"))
		return
	}

	if role == models.RoleOwner {
		_ = response.BadRequest(w, r, errors.New("cannot invite with owner role"))
		return
	}

	// Create the invitation
	invitation := &models.Invitation{
		Email:     req.Email,
		TeamID:    &teamID,
		Role:      role,
		InvitedBy: user.ID,
	}

	if err := h.store.Invitations.Create(ctx, invitation); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create invitation"))
		return
	}

	_ = response.Created(w, r, "Invitation created successfully", invitationModelToResponse(invitation))
}

// RevokeTeamInvitation godoc
//
//	@Summary		Revoke team invitation
//	@Description	Revokes a pending team invitation
//	@Tags			teams
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			teamId			path	string	true	"Team ID"		format(uuid)
//	@Param			invitationId	path	string	true	"Invitation ID"	format(uuid)
//	@Success		204				"No Content"
//	@Failure		401				{object}	ErrorResponse
//	@Failure		403				{object}	ErrorResponse
//	@Failure		404				{object}	ErrorResponse
//	@Router			/api/v1/teams/{teamId}/invitations/{invitationId} [delete]
func (h *Handler) RevokeTeamInvitation(w http.ResponseWriter, r *http.Request) {
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

	invitationID, err := getInvitationIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Check membership and permissions
	member, err := h.store.Teams.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	if !member.Role.CanManageMembers() {
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
