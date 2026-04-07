package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	gh "github.com/mujhtech/dagryn/pkg/github"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/workflow/ghactions"
)

// TranslateGitHubWorkflowYAML godoc
//
//	@Summary		Translate pasted GitHub Actions YAML into Dagryn tasks
//	@Description	Converts a GitHub Actions workflow YAML string into a Dagryn TOML snippet
//	@Tags			workflows
//	@Produce		json
//	@Param			request	body		GitHubWorkflowYAMLTranslateRequest	true	"Workflow YAML payload"
//	@Success		200		{object}	GitHubWorkflowTranslateResponse
//	@Failure		400		{object}	ErrorResponse
//	@Router			/api/v1/workflows/translate [post]
func (h *Handler) TranslateGitHubWorkflowYAML(w http.ResponseWriter, r *http.Request) {
	var req GitHubWorkflowYAMLTranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	workflowYAML := strings.TrimSpace(req.WorkflowYAML)
	if workflowYAML == "" {
		_ = response.BadRequest(w, r, errors.New("workflow_yaml is required"))
		return
	}

	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = "workflow.yml"
	}
	if !strings.HasSuffix(fileName, ".yml") && !strings.HasSuffix(fileName, ".yaml") {
		fileName += ".yml"
	}

	translated, err := ghactions.TranslateWorkflows(map[string][]byte{
		fileName: []byte(workflowYAML),
	})
	if err != nil {
		_ = response.BadRequest(w, r, fmt.Errorf("failed to translate workflow YAML: %w", err))
		return
	}

	resp := GitHubWorkflowTranslateResponse{
		Detected:  translated.TasksToml != "",
		Plugins:   translated.Plugins,
		TasksToml: translated.TasksToml,
	}
	for _, wf := range translated.Workflows {
		resp.Workflows = append(resp.Workflows, GitHubWorkflowSummary{
			File:      wf.File,
			Name:      wf.Name,
			TaskCount: wf.TaskCount,
		})
	}

	_ = response.Ok(w, r, "Success", resp)
}

// TranslateGitHubWorkflows godoc
//
//	@Summary		Translate GitHub Actions workflows into Dagryn tasks
//	@Description	Fetches .github/workflows from a GitHub repo and returns a Dagryn TOML snippet
//	@Tags			providers
//	@Security		BearerAuth
//	@Produce		json
//	@Param			request	body		GitHubWorkflowTranslateRequest	true	"Repository details"
//	@Success		200		{object}	GitHubWorkflowTranslateResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Router			/api/v1/providers/github/workflows/translate [post]
func (h *Handler) TranslateGitHubWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req GitHubWorkflowTranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	req.RepoFullName = strings.TrimSpace(req.RepoFullName)
	if req.RepoFullName == "" || !strings.Contains(req.RepoFullName, "/") {
		_ = response.BadRequest(w, r, errors.New("repo_full_name must be in owner/repo format"))
		return
	}

	accessToken, err := h.resolveGitHubAccessToken(ctx, user.ID, req.GitHubInstallationID)
	if err != nil {
		_ = response.Forbidden(w, r, err)
		return
	}
	if accessToken == "" {
		_ = response.Forbidden(w, r, errors.New("no GitHub access token available"))
		return
	}

	owner, repoName, err := splitGitHubFullName(req.RepoFullName)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	ref := strings.TrimSpace(req.Ref)

	// Check if dagryn.toml already exists in the repo — if so, return it
	// and skip workflow translation entirely.
	dagrynContent, err := fetchGitHubFile(ctx, accessToken, owner, repoName, githubContentsPath, ref)
	if err == nil && len(dagrynContent) > 0 {
		resp := GitHubWorkflowTranslateResponse{
			HasDagrynToml: true,
			DagrynToml:    string(dagrynContent),
		}
		_ = response.Ok(w, r, "Success", resp)
		return
	}

	files, err := fetchGitHubWorkflowFiles(ctx, accessToken, owner, repoName, ref)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	translated, err := ghactions.TranslateWorkflows(files)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	resp := GitHubWorkflowTranslateResponse{
		Detected:  len(files) > 0 && translated.TasksToml != "",
		Plugins:   translated.Plugins,
		TasksToml: translated.TasksToml,
	}
	for _, wf := range translated.Workflows {
		resp.Workflows = append(resp.Workflows, GitHubWorkflowSummary{
			File:      wf.File,
			Name:      wf.Name,
			TaskCount: wf.TaskCount,
		})
	}

	_ = response.Ok(w, r, "Success", resp)
}

func (h *Handler) resolveGitHubAccessToken(ctx context.Context, userID uuid.UUID, installationID *uuid.UUID) (string, error) {
	if installationID != nil {
		if h.githubApp == nil {
			return "", errors.New("github App integration is not configured")
		}
		inst, err := h.store.GitHubInstallations.GetByID(ctx, *installationID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				return "", errors.New("github installation not found")
			}
			return "", err
		}
		token, err := h.githubApp.FetchInstallationToken(ctx, inst.InstallationID)
		if err != nil {
			return "", fmt.Errorf("failed to fetch installation token: %w", err)
		}
		return token.Token, nil
	}

	if h.providerEncrypt == nil {
		return "", errors.New("github OAuth integration is not configured")
	}
	tok, err := h.store.ProviderTokens.GetByUserAndProvider(ctx, userID, "github")
	if err != nil {
		return "", err
	}
	accessToken, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
	if err != nil {
		return "", err
	}
	return accessToken, nil
}

func splitGitHubFullName(fullName string) (string, string, error) {
	return gh.SplitFullName(fullName)
}

func fetchGitHubWorkflowFiles(ctx context.Context, token, owner, repoName, ref string) (map[string][]byte, error) {
	client := gh.NewClient(token)
	items, err := client.ListContents(ctx, owner, repoName, ".github/workflows", ref)
	if err != nil {
		// Treat 404 (no workflows dir) as empty.
		return map[string][]byte{}, nil
	}

	files := make(map[string][]byte)
	for _, item := range items {
		if item.Type != "file" {
			continue
		}
		if !strings.HasSuffix(item.Name, ".yml") && !strings.HasSuffix(item.Name, ".yaml") {
			continue
		}
		content, err := client.GetContents(ctx, owner, repoName, item.Path, ref)
		if err != nil {
			return nil, err
		}
		files[item.Name] = content
	}
	return files, nil
}

func fetchGitHubFile(ctx context.Context, token, owner, repoName, path, ref string) ([]byte, error) {
	return gh.NewClient(token).GetContents(ctx, owner, repoName, path, ref)
}
