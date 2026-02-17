package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// --- User Handlers ---

// GetCurrentUser godoc
// @Summary Get current user
// @Description Returns the currently authenticated user's profile
// @Tags users
// @Security BearerAuth
// @Security APIKeyAuth
// @Produce json
// @Success 200 {object} UserResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/users/me [get]
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	_ = response.Ok(w, r, "User retrieved successfully", userModelToResponse(user))
}

// UpdateCurrentUser godoc
// @Summary Update current user
// @Description Updates the currently authenticated user's profile
// @Tags users
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param body body UpdateUserRequest true "Update user request"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/users/me [patch]
func (h *Handler) UpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req UpdateUserRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Update fields if provided
	if req.Name != nil {
		user.Name = req.Name
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}

	if err := h.store.Users.Update(ctx, user); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update user"))
		return
	}

	_ = response.Ok(w, r, "User updated successfully", userModelToResponse(user))
}

// --- Helper functions ---

func userModelToResponse(user *models.User) UserResponse {
	resp := UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Provider:  user.Provider,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	if user.Name != nil {
		resp.Name = *user.Name
	}
	if user.AvatarURL != nil {
		resp.AvatarURL = *user.AvatarURL
	}
	return resp
}

func teamModelToResponse(team *models.Team, role models.Role) TeamResponse {
	resp := TeamResponse{
		ID:        team.ID,
		Name:      team.Name,
		Slug:      team.Slug,
		CreatedAt: team.CreatedAt,
		UpdatedAt: team.UpdatedAt,
	}
	if team.Description != nil {
		resp.Description = *team.Description
	}
	return resp
}

func teamWithMemberToResponse(team *models.TeamWithMember) TeamResponse {
	resp := TeamResponse{
		ID:        team.ID,
		Name:      team.Name,
		Slug:      team.Slug,
		CreatedAt: team.CreatedAt,
		UpdatedAt: team.UpdatedAt,
	}
	if team.Description != nil {
		resp.Description = *team.Description
	}
	return resp
}

func teamMemberWithUserToResponse(member *models.TeamMemberWithUser) TeamMemberResponse {
	return TeamMemberResponse{
		User:     userModelToResponse(&member.User),
		Role:     string(member.Role),
		JoinedAt: member.JoinedAt,
	}
}

func invitationModelToResponse(inv *models.Invitation) InvitationResponse {
	// Compute status from invitation state
	status := "pending"
	if inv.IsAccepted() {
		status = "accepted"
	} else if inv.IsExpired() {
		status = "expired"
	}

	return InvitationResponse{
		ID:        inv.ID,
		Email:     inv.Email,
		Role:      string(inv.Role),
		TeamID:    inv.TeamID,
		ProjectID: inv.ProjectID,
		Status:    status,
		ExpiresAt: inv.ExpiresAt,
		CreatedAt: inv.CreatedAt,
	}
}

func invitationWithDetailsToResponse(inv *models.InvitationWithDetails) InvitationResponse {
	// Compute status from invitation state
	status := "pending"
	if inv.IsAccepted() {
		status = "accepted"
	} else if inv.IsExpired() {
		status = "expired"
	}

	resp := InvitationResponse{
		ID:        inv.ID,
		Email:     inv.Email,
		Role:      string(inv.Role),
		TeamID:    inv.TeamID,
		ProjectID: inv.ProjectID,
		Status:    status,
		ExpiresAt: inv.ExpiresAt,
		CreatedAt: inv.CreatedAt,
	}
	if inv.TeamName != nil {
		resp.TeamName = *inv.TeamName
	}
	if inv.ProjectName != nil {
		resp.ProjectName = *inv.ProjectName
	}
	if inv.InviterName != nil || inv.InviterEmail != "" {
		resp.InvitedBy = UserResponse{
			ID:    inv.InvitedBy,
			Email: inv.InviterEmail,
			Name:  "",
		}
		if inv.InviterName != nil {
			resp.InvitedBy.Name = *inv.InviterName
		}
	}
	return resp
}

func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	return slug
}
