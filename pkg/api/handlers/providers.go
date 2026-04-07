package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	gh "github.com/mujhtech/dagryn/pkg/github"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

const (
	githubContentsPath = "dagryn.toml"
	errNoDagrynToml    = "repository must contain dagryn.toml at the root to be used as a project"
)

// parseGitHubOwnerRepo delegates to the centralized github package.
func parseGitHubOwnerRepo(repoURL string) (owner, repo string, err error) {
	return gh.ParseOwnerRepo(repoURL)
}

// checkGitHubRepoHasDagrynToml verifies that the GitHub repo contains dagryn.toml at the root (default branch).
func (h *Handler) checkGitHubRepoHasDagrynToml(ctx context.Context, accessToken, repoURL string) error {
	owner, repoName, err := parseGitHubOwnerRepo(repoURL)
	if err != nil {
		return err
	}
	client := gh.NewClient(accessToken)
	if err := client.FileExists(ctx, owner, repoName, githubContentsPath); err != nil {
		return errors.New(errNoDagrynToml)
	}
	return nil
}

// validateGitHubRepoBelongsToInstallation verifies that a repo belongs to a GitHub App installation.
func (h *Handler) validateGitHubRepoBelongsToInstallation(ctx context.Context, accessToken string, repoID int64, repoURL string) error {
	owner, repoName, err := parseGitHubOwnerRepo(repoURL)
	if err != nil {
		return err
	}

	client := gh.NewClient(accessToken)
	r, err := client.GetRepo(ctx, owner, repoName)
	if err != nil {
		return fmt.Errorf("repository not accessible with installation token: %w", err)
	}
	if r.ID != repoID {
		return fmt.Errorf("repository ID mismatch: expected %d, got %d", repoID, r.ID)
	}
	return nil
}

// GitHubRepo is a minimal repo representation for the Import from GitHub UI.
type GitHubRepo = gh.Repository

// ListGitHubRepos godoc
//
//	@Summary		List GitHub repositories
//	@Description	Lists repositories the current user has access to (requires GitHub login with repo scope)
//	@Tags			providers
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{array}		GitHubRepo
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse	"GitHub token not linked; log in with GitHub to import repos"
//	@Router			/api/v1/providers/github/repos [get]
func (h *Handler) ListGitHubRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	if h.providerEncrypt == nil {
		_ = response.Forbidden(w, r, errors.New("gitHub integration is not configured"))
		return
	}

	tok, err := h.store.ProviderTokens.GetByUserAndProvider(ctx, user.ID, "github")
	if err != nil || tok == nil {
		_ = response.Forbidden(w, r, errors.New("no GitHub account linked. Log in with GitHub to import repositories"))
		return
	}

	accessToken, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to use GitHub token"))
		return
	}

	client := gh.NewClient(accessToken)
	repos, err := client.ListUserRepos(ctx)
	if err != nil {
		_ = response.InternalServerError(w, r, fmt.Errorf("gitHub API request failed: %w", err))
		return
	}

	_ = response.Ok(w, r, "Success", repos)
}

// GitHubAppInstallation represents a GitHub App installation.
type GitHubAppInstallation struct {
	ID             uuid.UUID `json:"id"`
	InstallationID int64     `json:"installation_id"`
	AccountLogin   string    `json:"account_login"`
	AccountType    string    `json:"account_type"`
}

// ListGitHubAppInstallations godoc
//
//	@Summary		List GitHub App installations
//	@Description	Lists all GitHub App installations accessible to the current user
//	@Tags			providers
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{array}		GitHubAppInstallation
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/v1/providers/github/app/installations [get]
func (h *Handler) ListGitHubAppInstallations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	// For now, return all installations. In the future, we can filter by user/team.
	// This requires tracking which users/teams have access to which installations.
	instRecords, err := h.store.GitHubInstallations.ListAll(ctx)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	installations := make([]GitHubAppInstallation, 0, len(instRecords))
	for _, inst := range instRecords {
		installations = append(installations, GitHubAppInstallation{
			ID:             inst.ID,
			InstallationID: inst.InstallationID,
			AccountLogin:   inst.AccountLogin,
			AccountType:    inst.AccountType,
		})
	}

	_ = response.Ok(w, r, "Success", installations)
}

// ListGitHubAppRepos godoc
//
//	@Summary		List repositories for a GitHub App installation
//	@Description	Lists repositories accessible via a GitHub App installation
//	@Tags			providers
//	@Security		BearerAuth
//	@Produce		json
//	@Param			installationId	path		string	true	"Installation ID (UUID)"	format(uuid)
//	@Success		200				{array}		GitHubRepo
//	@Failure		401				{object}	ErrorResponse
//	@Failure		404				{object}	ErrorResponse
//	@Router			/api/v1/providers/github/app/installations/{installationId}/repos [get]
func (h *Handler) ListGitHubAppRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	if h.githubApp == nil {
		_ = response.Forbidden(w, r, errors.New("github App integration is not configured"))
		return
	}

	installationID, err := getInstallationIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	inst, err := h.store.GitHubInstallations.GetByID(ctx, installationID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("installation not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	token, err := h.githubApp.FetchInstallationToken(ctx, inst.InstallationID)
	if err != nil {
		_ = response.InternalServerError(w, r, fmt.Errorf("failed to fetch installation token: %w", err))
		return
	}

	client := gh.NewClient(token.Token)
	repos, err := client.ListInstallationRepos(ctx)
	if err != nil {
		_ = response.InternalServerError(w, r, fmt.Errorf("github API request failed: %w", err))
		return
	}

	_ = response.Ok(w, r, "Success", repos)
}

// commitDagrynTomlToGitHub creates a new branch with dagryn.toml and opens a
// pull request for the user to merge.
func commitDagrynTomlToGitHub(accessToken, repoURL, content, defaultBranch string) error {
	owner, repoName, err := parseGitHubOwnerRepo(repoURL)
	if err != nil {
		return err
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	ctx := context.Background()
	client := gh.NewClient(accessToken)
	branchName := "dagryn/add-config"

	// 1. Get the SHA of the default branch HEAD
	baseSHA, err := client.GetRef(ctx, owner, repoName, "heads/"+defaultBranch)
	if err != nil {
		return fmt.Errorf("get default branch ref: %w", err)
	}

	// 2. Create a new branch
	status, err := client.CreateRef(ctx, owner, repoName, "refs/heads/"+branchName, baseSHA)
	if err != nil {
		// 422 = branch already exists (perhaps from a previous attempt) — continue
		if status != http.StatusUnprocessableEntity {
			return fmt.Errorf("create branch: %w", err)
		}
		slog.Info("commitDagrynToml: branch already exists, reusing", "branch", branchName)
	}

	// 3. Commit dagryn.toml to the new branch
	fileReq := gh.CreateFileRequest{
		Message: "chore: add dagryn.toml configuration\n\nGenerated by Dagryn during project import.",
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
		Branch:  branchName,
	}
	status, err = client.CreateOrUpdateFile(ctx, owner, repoName, githubContentsPath, fileReq)
	if err != nil {
		// 422 = file already exists on this branch — continue to PR creation
		if status != http.StatusUnprocessableEntity {
			return fmt.Errorf("create file: %w", err)
		}
		slog.Info("commitDagrynToml: file already exists on branch", "branch", branchName)
	}

	// 4. Open a pull request
	prReq := gh.CreatePRRequest{
		Title: "chore: add dagryn.toml configuration",
		Head:  branchName,
		Base:  defaultBranch,
		Body:  "This PR adds a `dagryn.toml` workflow configuration file generated during project import.\n\nYou can edit and merge when ready — Dagryn will use the stored configuration in the meantime.",
	}
	status, err = client.CreatePR(ctx, owner, repoName, prReq)
	if err != nil {
		// 422 = PR already exists for this head/base pair
		if status == http.StatusUnprocessableEntity {
			slog.Info("commitDagrynToml: PR already exists", "branch", branchName)
			return nil
		}
		return fmt.Errorf("create PR: %w", err)
	}

	slog.Info("commitDagrynToml: PR created", "repo", repoURL, "branch", branchName)
	return nil
}
