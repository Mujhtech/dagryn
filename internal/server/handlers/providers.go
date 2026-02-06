package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/repo"
	serverctx "github.com/mujhtech/dagryn/internal/server/context"
	"github.com/mujhtech/dagryn/internal/server/response"
)

const (
	githubReposURL      = "https://api.github.com/user/repos?affiliation=owner,collaborator,organization_member&sort=updated&per_page=100"
	githubContentsPath  = "dagryn.toml"
	errNoDagrynToml     = "repository must contain dagryn.toml at the root to be used as a project"
	errInvalidGitHubURL = "invalid GitHub repository URL"
)

// parseGitHubOwnerRepo extracts owner and repo name from a GitHub URL.
// Supports https://github.com/owner/repo, https://github.com/owner/repo.git, git@github.com:owner/repo.git.
func parseGitHubOwnerRepo(repoURL string) (owner, repo string, err error) {
	u := strings.TrimSpace(repoURL)
	u = strings.TrimSuffix(u, ".git")
	var parts []string
	if strings.HasPrefix(u, "git@github.com:") {
		u = strings.TrimPrefix(u, "git@github.com:")
		parts = strings.Split(u, "/")
	} else if strings.Contains(u, "github.com/") {
		i := strings.Index(u, "github.com/")
		u = u[i+len("github.com/"):]
		parts = strings.Split(u, "/")
	} else {
		return "", "", errors.New(errInvalidGitHubURL)
	}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New(errInvalidGitHubURL)
	}
	return parts[0], parts[1], nil
}

// checkGitHubRepoHasDagrynToml verifies that the GitHub repo contains dagryn.toml at the root (default branch).
// Returns an error if the file is missing or the request fails.
func (h *Handler) checkGitHubRepoHasDagrynToml(ctx context.Context, accessToken, repoURL string) error {
	owner, repo, err := parseGitHubOwnerRepo(repoURL)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, githubContentsPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check repository: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return errors.New(errNoDagrynToml)
	case http.StatusForbidden:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("access denied (403): ensure the GitHub App has 'Contents: Read' permission and the repository is included in the installation. If using GitHub App, verify the repo is selected when installing the app. GitHub response: %s", string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("repository must contain dagryn.toml at the root: GitHub returned %d: %s", resp.StatusCode, string(body))
	}
}

// validateGitHubRepoBelongsToInstallation verifies that a repo belongs to a GitHub App installation.
// It checks that the repo ID matches and the repo is accessible with the installation token.
func (h *Handler) validateGitHubRepoBelongsToInstallation(ctx context.Context, accessToken string, repoID int64, repoURL string) error {
	owner, repoName, err := parseGitHubOwnerRepo(repoURL)
	if err != nil {
		return err
	}

	// Fetch repo details using installation token
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate repository: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return fmt.Errorf("repository not accessible with installation token: GitHub returned %d", resp.StatusCode)
	}

	var repoResp struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoResp); err != nil {
		return fmt.Errorf("failed to parse repository response: %w", err)
	}

	if repoResp.ID != repoID {
		return fmt.Errorf("repository ID mismatch: expected %d, got %d", repoID, repoResp.ID)
	}

	return nil
}

// GitHubRepo is a minimal repo representation for the Import from GitHub UI.
type GitHubRepo struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// ListGitHubRepos godoc
// @Summary List GitHub repositories
// @Description Lists repositories the current user has access to (requires GitHub login with repo scope)
// @Tags providers
// @Security BearerAuth
// @Produce json
// @Success 200 {array} GitHubRepo
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "GitHub token not linked; log in with GitHub to import repos"
// @Router /api/v1/providers/github/repos [get]
func (h *Handler) ListGitHubRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	if h.providerTokens == nil || h.providerEncrypt == nil {
		_ = response.Forbidden(w, r, errors.New("gitHub integration is not configured"))
		return
	}

	tok, err := h.providerTokens.GetByUserAndProvider(ctx, user.ID, "github")
	if err != nil || tok == nil {
		_ = response.Forbidden(w, r, errors.New("no GitHub account linked. Log in with GitHub to import repositories"))
		return
	}

	accessToken, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to use GitHub token"))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReposURL, nil)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		_ = response.InternalServerError(w, r, fmt.Errorf("gitHub API request failed: %w", err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = response.InternalServerError(w, r, fmt.Errorf("gitHub API returned %d: %s", resp.StatusCode, string(body)))
		return
	}

	var raw []struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	list := make([]GitHubRepo, 0, len(raw))
	for _, r := range raw {
		list = append(list, GitHubRepo{
			ID:            r.ID,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}

	_ = response.Ok(w, r, "Success", list)
}

// GitHubAppInstallation represents a GitHub App installation.
type GitHubAppInstallation struct {
	ID             uuid.UUID `json:"id"`
	InstallationID int64     `json:"installation_id"`
	AccountLogin   string    `json:"account_login"`
	AccountType    string    `json:"account_type"`
}

// ListGitHubAppInstallations godoc
// @Summary List GitHub App installations
// @Description Lists all GitHub App installations accessible to the current user
// @Tags providers
// @Security BearerAuth
// @Produce json
// @Success 200 {array} GitHubAppInstallation
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/providers/github/app/installations [get]
func (h *Handler) ListGitHubAppInstallations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	if h.githubInstallations == nil {
		_ = response.Forbidden(w, r, errors.New("github App integration is not configured"))
		return
	}

	// For now, return all installations. In the future, we can filter by user/team.
	// This requires tracking which users/teams have access to which installations.
	instRecords, err := h.githubInstallations.ListAll(ctx)
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
// @Summary List repositories for a GitHub App installation
// @Description Lists repositories accessible via a GitHub App installation
// @Tags providers
// @Security BearerAuth
// @Produce json
// @Param installationID path string true "Installation ID (UUID)" format(uuid)
// @Success 200 {array} GitHubRepo
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/providers/github/app/installations/{installationID}/repos [get]
func (h *Handler) ListGitHubAppRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	if h.githubApp == nil || h.githubInstallations == nil {
		_ = response.Forbidden(w, r, errors.New("github App integration is not configured"))
		return
	}

	installationIDStr := chi.URLParam(r, "installationID")
	installationID, err := uuid.Parse(installationIDStr)
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid installation ID"))
		return
	}

	inst, err := h.githubInstallations.GetByID(ctx, installationID)
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

	// List repos accessible to this installation
	url := "https://api.github.com/installation/repositories?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		_ = response.InternalServerError(w, r, fmt.Errorf("github API request failed: %w", err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = response.InternalServerError(w, r, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body)))
		return
	}

	var raw struct {
		Repositories []struct {
			ID            int64  `json:"id"`
			FullName      string `json:"full_name"`
			CloneURL      string `json:"clone_url"`
			DefaultBranch string `json:"default_branch"`
			Private       bool   `json:"private"`
		} `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	list := make([]GitHubRepo, 0, len(raw.Repositories))
	for _, r := range raw.Repositories {
		list = append(list, GitHubRepo{
			ID:            r.ID,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}

	_ = response.Ok(w, r, "Success", list)
}
