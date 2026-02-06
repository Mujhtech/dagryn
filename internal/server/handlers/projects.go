package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/db/repo"
	serverctx "github.com/mujhtech/dagryn/internal/server/context"
	"github.com/mujhtech/dagryn/internal/server/response"
)

// --- Project Handlers ---

// ListProjects godoc
// @Summary List projects
// @Description Returns all projects the current user has access to
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20) maximum(100)
// @Success 200 {object} PaginatedResponse{data=[]ProjectResponse}
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/projects [get]
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projects, err := h.projects.ListByUser(ctx, user.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list projects"))
		return
	}

	// Convert to response format
	resp := make([]ProjectResponse, 0, len(projects))
	for _, p := range projects {
		resp = append(resp, projectWithMemberToResponse(&p))
	}

	// For now, return all projects (pagination can be added later)
	_ = response.Ok(w, r, "Success", PaginatedResponse{
		Data: resp,
		Meta: PaginationMeta{
			Page:       1,
			PerPage:    len(resp),
			Total:      int64(len(resp)),
			TotalPages: 1,
		},
	})
}

// CreateProject godoc
// @Summary Create a project
// @Description Creates a new project within a team
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param body body CreateProjectRequest true "Create project request"
// @Success 201 {object} ProjectResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/projects [post]
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req CreateProjectRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate required fields
	if req.Name == "" {
		_ = response.BadRequest(w, r, errors.New("project name is required"))
		return
	}

	// If team is specified, check user is a member with appropriate permissions
	var teamID *uuid.UUID
	if req.TeamID != uuid.Nil {
		teamID = &req.TeamID
		member, err := h.teams.GetMember(ctx, req.TeamID, user.ID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check team membership"))
			return
		}

		// Only admins and owners can create projects in a team
		if !member.Role.CanManageMembers() {
			_ = response.Forbidden(w, r, errors.New("you don't have permission to create projects in this team"))
			return
		}
	}

	// Generate slug if not provided
	slug := req.Slug
	if slug == "" {
		slug = generateSlug(req.Name)
	}

	// Check if slug exists within the team/personal scope
	exists, err := h.projects.SlugExists(ctx, teamID, slug)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to check slug"))
		return
	}
	if exists {
		_ = response.BadRequest(w, r, errors.New("a project with this slug already exists"))
		return
	}

	// Parse visibility
	visibility := models.VisibilityPrivate
	if req.Visibility != "" {
		visibility = models.Visibility(req.Visibility)
		if !models.IsValidVisibility(visibility) {
			_ = response.BadRequest(w, r, errors.New("invalid visibility. Must be private, team, or public"))
			return
		}
	}

	// When creating from GitHub (repo_url set), verify the repo contains dagryn.toml at the root
	if req.RepoURL != "" {
		var accessToken string

		// Prefer GitHub App installation token if provided
		if req.GitHubInstallationID != nil && req.GitHubRepoID != nil && h.githubApp != nil && h.githubInstallations != nil {
			instRecord, err := h.githubInstallations.GetByID(ctx, *req.GitHubInstallationID)
			if err == nil && instRecord != nil {
				token, err := h.githubApp.FetchInstallationToken(ctx, instRecord.InstallationID)
				if err == nil && token != nil {
					accessToken = token.Token
					// Validate that the repo belongs to this installation
					if err := h.validateGitHubRepoBelongsToInstallation(ctx, token.Token, *req.GitHubRepoID, req.RepoURL); err != nil {
						_ = response.BadRequest(w, r, errors.New("repository does not belong to the specified installation: "+err.Error()))
						return
					}
				}
			}
		}

		// Fallback to OAuth token if no installation token was obtained
		if accessToken == "" {
			if h.providerTokens == nil || h.providerEncrypt == nil {
				_ = response.BadRequest(w, r, errors.New("gitHub integration is not configured; cannot create project from repository"))
				return
			}
			tok, err := h.providerTokens.GetByUserAndProvider(ctx, user.ID, "github")
			if err != nil || tok == nil {
				_ = response.BadRequest(w, r, errors.New("no GitHub account linked. Log in with GitHub to create a project from a repository"))
				return
			}
			accessToken, err = h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
			if err != nil {
				_ = response.InternalServerError(w, r, errors.New("failed to use GitHub token"))
				return
			}
		}

		if err := h.checkGitHubRepoHasDagrynToml(ctx, accessToken, req.RepoURL); err != nil {
			_ = response.BadRequest(w, r, err)
			return
		}
	}

	// Create project
	project := &models.Project{
		TeamID:      teamID,
		Name:        req.Name,
		Slug:        slug,
		Description: stringPtr(req.Description),
		Visibility:  visibility,
		ConfigPath:  "dagryn.toml", // Default config path
	}
	if req.RepoURL != "" {
		project.RepoURL = &req.RepoURL
		project.RepoLinkedByUserID = &user.ID
		// Store GitHub App installation info if provided
		if req.GitHubInstallationID != nil {
			project.GitHubInstallationID = req.GitHubInstallationID
		}
		if req.GitHubRepoID != nil {
			project.GitHubRepoID = req.GitHubRepoID
		}
	}

	if err := h.projects.Create(ctx, project, user.ID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create project"))
		return
	}

	_ = response.Created(w, r, "Project created successfully", projectModelToResponse(project, models.RoleOwner))
}

// GetProject godoc
// @Summary Get a project
// @Description Returns a project by ID
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Success 200 {object} ProjectResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID} [get]
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Get user's role for this project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	project, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("project not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get project"))
		return
	}

	_ = response.Ok(w, r, "Success", projectModelToResponse(project, role))
}

// UpdateProject godoc
// @Summary Update a project
// @Description Updates a project's details
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Param body body UpdateProjectRequest true "Update project request"
// @Success 200 {object} ProjectResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID} [patch]
func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Get user's role for this project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to edit
	if !role.HasPermission(models.PermissionProjectEdit) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to edit this project"))
		return
	}

	var req UpdateProjectRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	project, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("project not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get project"))
		return
	}

	// Update fields if provided
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = req.Description
	}
	if req.Visibility != nil {
		visibility := models.Visibility(*req.Visibility)
		if !models.IsValidVisibility(visibility) {
			_ = response.BadRequest(w, r, errors.New("invalid visibility"))
			return
		}
		project.Visibility = visibility
	}
	if req.RepoURL != nil {
		project.RepoURL = req.RepoURL
		project.RepoLinkedByUserID = &user.ID
	}

	if err := h.projects.Update(ctx, project); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update project"))
		return
	}

	_ = response.Ok(w, r, "Success", projectModelToResponse(project, role))
}

// ConnectProjectToGitHub godoc
// @Summary Connect a project to GitHub
// @Description Connects a locally-created project to a GitHub repository
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Param request body ConnectGitHubRequest true "GitHub connection details"
// @Success 200 {object} ProjectResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/connect-github [post]
func (h *Handler) ConnectProjectToGitHub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Get user's role for this project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to edit
	if !role.HasPermission(models.PermissionProjectEdit) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to edit this project"))
		return
	}

	var req ConnectGitHubRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	// Validate required fields
	if req.GitHubInstallationID == uuid.Nil {
		_ = response.BadRequest(w, r, errors.New("github_installation_id is required"))
		return
	}
	if req.GitHubRepoID == 0 {
		_ = response.BadRequest(w, r, errors.New("github_repo_id is required"))
		return
	}
	if req.RepoURL == "" {
		_ = response.BadRequest(w, r, errors.New("repo_url is required"))
		return
	}

	// Check GitHub App is configured
	if h.githubApp == nil || h.githubInstallations == nil {
		_ = response.BadRequest(w, r, errors.New("github App integration is not configured"))
		return
	}

	// Verify installation exists and user has access
	instRecord, err := h.githubInstallations.GetByID(ctx, req.GitHubInstallationID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.BadRequest(w, r, errors.New("github installation not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to verify installation"))
		return
	}

	// Fetch installation token
	token, err := h.githubApp.FetchInstallationToken(ctx, instRecord.InstallationID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to get installation token"))
		return
	}

	// Validate repo belongs to installation
	if err := h.validateGitHubRepoBelongsToInstallation(ctx, token.Token, req.GitHubRepoID, req.RepoURL); err != nil {
		_ = response.BadRequest(w, r, errors.New("repository does not belong to the specified installation: "+err.Error()))
		return
	}

	// Check repo has dagryn.toml
	if err := h.checkGitHubRepoHasDagrynToml(ctx, token.Token, req.RepoURL); err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	// Get and update project
	project, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("project not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get project"))
		return
	}

	// Update GitHub fields
	project.GitHubInstallationID = &req.GitHubInstallationID
	project.GitHubRepoID = &req.GitHubRepoID
	project.RepoURL = &req.RepoURL
	project.RepoLinkedByUserID = &user.ID

	if err := h.projects.Update(ctx, project); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update project"))
		return
	}

	_ = response.Ok(w, r, "Project connected to GitHub successfully", projectModelToResponse(project, role))
}

// DeleteProject godoc
// @Summary Delete a project
// @Description Deletes a project (requires owner/admin role)
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Param projectID path string true "Project ID" format(uuid)
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID} [delete]
func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Get user's role for this project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Only owner can delete
	if !role.CanDeleteProject() {
		_ = response.Forbidden(w, r, errors.New("only the project owner can delete this project"))
		return
	}

	if err := h.projects.Delete(ctx, projectID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("project not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to delete project"))
		return
	}

	_ = response.NoContent(w, r)
}

// --- Project Member Handlers ---

// ListProjectMembers godoc
// @Summary List project members
// @Description Returns all members of a project
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Success 200 {object} []ProjectMemberResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/members [get]
func (h *Handler) ListProjectMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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

	members, err := h.projects.ListMembers(ctx, projectID)
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
// @Summary Add project member
// @Description Adds a user to a project
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Param body body AddProjectMemberRequest true "Add member request"
// @Success 201 {object} ProjectMemberResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/members [post]
func (h *Handler) AddProjectMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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
	targetUser, err := h.users.GetByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("user not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get user"))
		return
	}

	// Check if already a member
	_, err = h.projects.GetMember(ctx, projectID, req.UserID)
	if err == nil {
		_ = response.BadRequest(w, r, errors.New("user is already a member of this project"))
		return
	}
	if !errors.Is(err, repo.ErrNotFound) {
		_ = response.InternalServerError(w, r, errors.New("failed to check membership"))
		return
	}

	// Add member
	if err := h.projects.AddMember(ctx, projectID, req.UserID, newRole, &user.ID); err != nil {
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
// @Summary Remove project member
// @Description Removes a user from a project
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Param projectID path string true "Project ID" format(uuid)
// @Param userID path string true "User ID" format(uuid)
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/members/{userID} [delete]
func (h *Handler) RemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid user ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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
	targetMember, err := h.projects.GetMember(ctx, projectID, targetUserID)
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

	if err := h.projects.RemoveMember(ctx, projectID, targetUserID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to remove member"))
		return
	}

	_ = response.NoContent(w, r)
}

// UpdateProjectMemberRole godoc
// @Summary Update project member role
// @Description Updates a project member's role
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Param userID path string true "User ID" format(uuid)
// @Param body body UpdateMemberRoleRequest true "Update role request"
// @Success 200 {object} ProjectMemberResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/members/{userID}/role [patch]
func (h *Handler) UpdateProjectMemberRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid user ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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
	targetMember, err := h.projects.GetMember(ctx, projectID, targetUserID)
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

	if err := h.projects.UpdateMemberRole(ctx, projectID, targetUserID, newRole); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update role"))
		return
	}

	targetUser, _ := h.users.GetByID(ctx, targetUserID)
	_ = response.Ok(w, r, "Success", ProjectMemberResponse{
		User:     userModelToResponse(targetUser),
		Role:     string(newRole),
		JoinedAt: targetMember.JoinedAt,
	})
}

// --- Project Invitation Handlers ---

// ListProjectInvitations godoc
// @Summary List project invitations
// @Description Returns all pending invitations for a project
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Success 200 {object} []InvitationResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/invitations [get]
func (h *Handler) ListProjectInvitations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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

	invitations, err := h.invitations.ListByProject(ctx, projectID)
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
// @Summary Create project invitation
// @Description Creates an invitation to join a project
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Param body body CreateInvitationRequest true "Create invitation request"
// @Success 201 {object} InvitationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/invitations [post]
func (h *Handler) CreateProjectInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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

	if err := h.invitations.Create(ctx, invitation); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create invitation"))
		return
	}

	_ = response.Created(w, r, "Created successfully", invitationModelToResponse(invitation))
}

// RevokeProjectInvitation godoc
// @Summary Revoke project invitation
// @Description Revokes a pending project invitation
// @Tags projects
// @Security BearerAuth
// @Security APIKeyAuth
// @Param projectID path string true "Project ID" format(uuid)
// @Param invitationID path string true "Invitation ID" format(uuid)
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/invitations/{invitationID} [delete]
func (h *Handler) RevokeProjectInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	invitationID, err := uuid.Parse(chi.URLParam(r, "invitationID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid invitation ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
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

	if err := h.invitations.Delete(ctx, invitationID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("invitation not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to revoke invitation"))
		return
	}

	_ = response.NoContent(w, r)
}

// --- Project API Key Handlers ---

// ListProjectAPIKeys godoc
// @Summary List project API keys
// @Description Returns all API keys for a project
// @Tags projects,api-keys
// @Security BearerAuth
// @Security APIKeyAuth
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Success 200 {object} []APIKeyResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/api-keys [get]
func (h *Handler) ListProjectAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to view API keys
	if !role.HasPermission(models.PermissionAPIKeysView) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to view API keys"))
		return
	}

	keys, err := h.apikeys.ListByProject(ctx, projectID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list API keys"))
		return
	}

	resp := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, apiKeyModelToResponse(&k))
	}

	_ = response.Ok(w, r, "Success", resp)
}

// CreateProjectAPIKey godoc
// @Summary Create project API key
// @Description Creates a new API key for a project
// @Tags projects,api-keys
// @Security BearerAuth
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID" format(uuid)
// @Param body body CreateAPIKeyRequest true "Create API key request"
// @Success 201 {object} APIKeyCreatedResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/api-keys [post]
func (h *Handler) CreateProjectAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to manage API keys
	if !role.HasPermission(models.PermissionAPIKeysManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to create API keys"))
		return
	}

	var req CreateAPIKeyRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.Name == "" {
		_ = response.BadRequest(w, r, errors.New("API key name is required"))
		return
	}

	// Parse expiration
	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		duration, err := parseDuration(req.ExpiresIn)
		if err != nil {
			_ = response.BadRequest(w, r, errors.New("invalid expiration format: use format like '90d', '30d', '1y'"))
			return
		}
		exp := time.Now().Add(duration)
		expiresAt = &exp
	}

	// Create API key
	key := &models.APIKey{
		UserID:    user.ID,
		ProjectID: &projectID,
		Name:      req.Name,
		Scope:     models.APIKeyScopeProject,
		ExpiresAt: expiresAt,
	}

	rawKey, err := h.apikeys.Create(ctx, key)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create API key"))
		return
	}

	_ = response.Created(w, r, "Created successfully", APIKeyCreatedResponse{
		APIKeyResponse: apiKeyModelToResponse(key),
		Key:            rawKey,
	})
}

// RevokeProjectAPIKey godoc
// @Summary Revoke project API key
// @Description Revokes an API key for a project
// @Tags projects,api-keys
// @Security BearerAuth
// @Security APIKeyAuth
// @Param projectID path string true "Project ID" format(uuid)
// @Param keyID path string true "API Key ID" format(uuid)
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/projects/{projectID}/api-keys/{keyID} [delete]
func (h *Handler) RevokeProjectAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid project ID"))
		return
	}

	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid API key ID"))
		return
	}

	// Check user has access to project
	role, err := h.projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	// Check permission to manage API keys
	if !role.HasPermission(models.PermissionAPIKeysManage) {
		_ = response.Forbidden(w, r, errors.New("you don't have permission to revoke API keys"))
		return
	}

	// Verify the key belongs to this project
	key, err := h.apikeys.GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("API key not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get API key"))
		return
	}

	if key.ProjectID == nil || *key.ProjectID != projectID {
		_ = response.NotFound(w, r, errors.New("API key not found in this project"))
		return
	}

	if err := h.apikeys.Revoke(ctx, keyID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to revoke API key"))
		return
	}

	_ = response.NoContent(w, r)
}

// --- Helper functions ---

func projectModelToResponse(project *models.Project, role models.Role) ProjectResponse {
	resp := ProjectResponse{
		ID:         project.ID,
		Name:       project.Name,
		Slug:       project.Slug,
		Visibility: string(project.Visibility),
		CreatedAt:  project.CreatedAt,
		UpdatedAt:  project.UpdatedAt,
		ConfigPath: project.ConfigPath,
		// RepoLinkedByUserID: project.RepoLinkedByUserID,
		LastRunAt: project.LastRunAt,
	}

	if project.RepoURL != nil {
		resp.RepoURL = *project.RepoURL
	}

	if project.TeamID != nil {
		resp.TeamID = *project.TeamID
	}
	if project.Description != nil {
		resp.Description = *project.Description
	}
	return resp
}

func projectWithMemberToResponse(project *models.ProjectWithMember) ProjectResponse {
	resp := ProjectResponse{
		ID:         project.ID,
		Name:       project.Name,
		Slug:       project.Slug,
		Visibility: string(project.Visibility),
		CreatedAt:  project.CreatedAt,
		UpdatedAt:  project.UpdatedAt,
		// RepoLinkedByUserID: project.RepoLinkedByUserID,
		LastRunAt:  project.LastRunAt,
		ConfigPath: project.ConfigPath,
	}
	if project.RepoURL != nil {
		resp.RepoURL = *project.RepoURL
	}
	if project.TeamID != nil {
		resp.TeamID = *project.TeamID
	}
	if project.Description != nil {
		resp.Description = *project.Description
	}
	return resp
}

func projectMemberWithUserToResponse(member *models.ProjectMemberWithUser) ProjectMemberResponse {
	return ProjectMemberResponse{
		User:     userModelToResponse(&member.User),
		Role:     string(member.Role),
		JoinedAt: member.JoinedAt,
	}
}

func apiKeyModelToResponse(key *models.APIKey) APIKeyResponse {
	return APIKeyResponse{
		ID:         key.ID,
		Name:       key.Name,
		Prefix:     key.KeyPrefix,
		Scope:      string(key.Scope),
		ProjectID:  key.ProjectID,
		LastUsedAt: key.LastUsedAt,
		ExpiresAt:  key.ExpiresAt,
		CreatedAt:  key.CreatedAt,
	}
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parseDuration parses a duration string like "90d", "30d", "1y"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, errors.New("invalid duration")
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]

	var value int
	for _, c := range valueStr {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid duration value")
		}
		value = value*10 + int(c-'0')
	}

	switch unit {
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(value) * 365 * 24 * time.Hour, nil
	case 'h':
		return time.Duration(value) * time.Hour, nil
	default:
		return 0, errors.New("invalid duration unit")
	}
}
