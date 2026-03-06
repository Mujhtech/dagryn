package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/service"
)

// ListProjects godoc
//
//	@Summary		List projects
//	@Description	Returns all projects the current user has access to
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			page		query		int	false	"Page number"		default(1)
//	@Param			per_page	query		int	false	"Items per page"	default(20)	maximum(100)
//	@Success		200			{object}	PaginatedResponse{data=[]ProjectResponse}
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/v1/projects [get]
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	projects, err := h.store.Projects.ListByUser(ctx, user.ID)
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
//
//	@Summary		Create a project
//	@Description	Creates a new project within a team
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateProjectRequest	true	"Create project request"
//	@Success		201		{object}	ProjectResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Router			/api/v1/projects [post]
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
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
		member, err := h.store.Teams.GetMember(ctx, req.TeamID, user.ID)
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
	exists, err := h.store.Projects.SlugExists(ctx, teamID, slug)
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

	// Validate provided dagryn config TOML if supplied
	if strings.TrimSpace(req.DagrynConfig) != "" {
		if _, err := config.ParseBytes([]byte(req.DagrynConfig)); err != nil {
			_ = response.BadRequest(w, r, errors.New("invalid dagryn_config: "+err.Error()))
			return
		}
	}

	// When creating from GitHub (repo_url set), obtain an access token and optionally
	// verify the repo contains dagryn.toml (skipped when the client provides DagrynConfig).
	var accessToken string
	if req.RepoURL != "" {
		// Prefer GitHub App installation token if provided
		if req.GitHubInstallationID != nil && req.GitHubRepoID != nil && h.githubApp != nil {
			instRecord, err := h.store.GitHubInstallations.GetByID(ctx, *req.GitHubInstallationID)
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
			if h.providerEncrypt == nil {
				_ = response.BadRequest(w, r, errors.New("gitHub integration is not configured; cannot create project from repository"))
				return
			}
			tok, err := h.store.ProviderTokens.GetByUserAndProvider(ctx, user.ID, "github")
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

		// Only check for dagryn.toml in the repo when the client didn't provide config inline
		if strings.TrimSpace(req.DagrynConfig) == "" {
			if err := h.checkGitHubRepoHasDagrynToml(ctx, accessToken, req.RepoURL); err != nil {
				_ = response.BadRequest(w, r, err)
				return
			}
		}
	}

	// Check project creation quota
	if h.entitlements != nil {
		if err := h.entitlements.CheckQuota(ctx, "projects", uuid.Nil, 0); err != nil {
			if entitlement.IsQuotaError(err) {
				_ = response.PaymentRequired(w, r, err)
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check quota"))
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
	if req.DefaultBranch != "" {
		project.DefaultBranch = &req.DefaultBranch
	}

	if err := h.store.Projects.Create(ctx, project, user.ID); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create project"))
		return
	}

	// Fire lifecycle hook for billing account linkage (cloud) or no-op (self-hosted).
	if h.entitlements != nil {
		var ownerName string
		if user.Name != nil {
			ownerName = *user.Name
		}
		_ = h.entitlements.OnProjectCreated(ctx, entitlement.ProjectCreatedEvent{
			ProjectID:  project.ID,
			OwnerID:    user.ID,
			OwnerEmail: user.Email,
			OwnerName:  ownerName,
			TeamID:     teamID,
		})
	}

	// Audit log: project created
	if h.auditService != nil && teamID != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       *teamID,
			ProjectID:    &project.ID,
			Action:       models.AuditActionProjectCreated,
			Category:     models.AuditCategoryProject,
			ResourceType: "project",
			ResourceID:   project.ID.String(),
			Description:  "Project created: " + project.Name,
		})
	}

	// If a dagryn config was provided, sync the workflow to project_workflows
	// and commit dagryn.toml to the GitHub repo in the background.
	if rawCfg := strings.TrimSpace(req.DagrynConfig); rawCfg != "" {
		if err := h.syncWorkflowInternally(ctx, project.ID, rawCfg); err != nil {
			slog.Warn("create_project: failed to sync provided workflow", "project_id", project.ID, "error", err)
		}

		// Best-effort: create a branch + PR with dagryn.toml so users can merge it
		if accessToken != "" && req.RepoURL != "" {
			repoURL := req.RepoURL
			cfgContent := rawCfg
			branch := req.DefaultBranch
			go func() {
				if err := commitDagrynTomlToGitHub(accessToken, repoURL, cfgContent, branch); err != nil {
					slog.Warn("create_project: failed to create dagryn.toml PR on GitHub", "repo", repoURL, "error", err)
				}
			}()
		}
	}

	_ = response.Created(w, r, "Project created successfully", projectModelToResponse(project, models.RoleOwner))
}

// GetProject godoc
//
//	@Summary		Get a project
//	@Description	Returns a project by ID
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			projectId	path		string	true	"Project ID"	format(uuid)
//	@Success		200			{object}	ProjectResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectID} [get]
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
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

	// Get user's role for this project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Forbidden(w, r, errors.New("you don't have access to this project"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to check access"))
		return
	}

	project, err := h.store.Projects.GetByID(ctx, projectID)
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
//
//	@Summary		Update a project
//	@Description	Updates a project's details
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			body		body		UpdateProjectRequest	true	"Update project request"
//	@Success		200			{object}	ProjectResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectID} [patch]
func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
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

	// Get user's role for this project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
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

	project, err := h.store.Projects.GetByID(ctx, projectID)
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

	if err := h.store.Projects.Update(ctx, project); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update project"))
		return
	}

	// Audit log: project updated
	if h.auditService != nil && project.TeamID != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       *project.TeamID,
			ProjectID:    &project.ID,
			Action:       models.AuditActionProjectUpdated,
			Category:     models.AuditCategoryProject,
			ResourceType: "project",
			ResourceID:   project.ID.String(),
			Description:  "Project updated: " + project.Name,
		})
	}

	_ = response.Ok(w, r, "Success", projectModelToResponse(project, role))
}

// ConnectProjectToGitHub godoc
//
//	@Summary		Connect a project to GitHub
//	@Description	Connects a locally-created project to a GitHub repository
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			projectId	path		string					true	"Project ID"	format(uuid)
//	@Param			request		body		ConnectGitHubRequest	true	"GitHub connection details"
//	@Success		200			{object}	ProjectResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectId}/connect-github [post]
func (h *Handler) ConnectProjectToGitHub(w http.ResponseWriter, r *http.Request) {
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

	// Get user's role for this project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
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
	if h.githubApp == nil {
		_ = response.BadRequest(w, r, errors.New("github App integration is not configured"))
		return
	}

	// Verify installation exists and user has access
	instRecord, err := h.store.GitHubInstallations.GetByID(ctx, req.GitHubInstallationID)
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
	project, err := h.store.Projects.GetByID(ctx, projectID)
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
	if req.DefaultBranch != "" {
		project.DefaultBranch = &req.DefaultBranch
	}

	if err := h.store.Projects.Update(ctx, project); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to update project"))
		return
	}

	_ = response.Ok(w, r, "Project connected to GitHub successfully", projectModelToResponse(project, role))
}

// DeleteProject godoc
//
//	@Summary		Delete a project
//	@Description	Deletes a project (requires owner/admin role)
//	@Tags			projects
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Param			projectId	path	string	true	"Project ID"	format(uuid)
//	@Success		204			"No Content"
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/projects/{projectID} [delete]
func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
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

	// Get user's role for this project
	role, err := h.store.Projects.GetUserRole(ctx, projectID, user.ID)
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

	// Get project before deletion for audit details.
	project, _ := h.store.Projects.GetByID(ctx, projectID)

	// Audit log: project deleted (logged before deletion so the team/project context still exists).
	if h.auditService != nil && project != nil && project.TeamID != nil {
		h.auditService.Log(ctx, service.AuditEntry{
			TeamID:       *project.TeamID,
			ProjectID:    &projectID,
			Action:       models.AuditActionProjectDeleted,
			Category:     models.AuditCategoryProject,
			ResourceType: "project",
			ResourceID:   projectID.String(),
			Description:  "Project deleted: " + project.Name,
		})
	}

	if err := h.store.Projects.Delete(ctx, projectID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("project not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to delete project"))
		return
	}

	_ = response.NoContent(w, r)
}
