package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/repo"
	serverctx "github.com/mujhtech/dagryn/internal/server/context"
	"github.com/mujhtech/dagryn/internal/server/response"
	"github.com/mujhtech/dagryn/internal/workflow/ghactions"
)

// TranslateGitHubWorkflows godoc
// @Summary Translate GitHub Actions workflows into Dagryn tasks
// @Description Fetches .github/workflows from a GitHub repo and returns a Dagryn TOML snippet
// @Tags providers
// @Security BearerAuth
// @Produce json
// @Param request body GitHubWorkflowTranslateRequest true "Repository details"
// @Success 200 {object} GitHubWorkflowTranslateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/providers/github/workflows/translate [post]
func (h *Handler) TranslateGitHubWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := serverctx.GetUser(ctx)
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

	files, err := fetchGitHubWorkflowFiles(ctx, accessToken, owner, repoName)
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
		if h.githubApp == nil || h.githubInstallations == nil {
			return "", errors.New("github App integration is not configured")
		}
		inst, err := h.githubInstallations.GetByID(ctx, *installationID)
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

	if h.providerTokens == nil || h.providerEncrypt == nil {
		return "", errors.New("github OAuth integration is not configured")
	}
	tok, err := h.providerTokens.GetByUserAndProvider(ctx, userID, "github")
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
	trimmed := strings.TrimSuffix(strings.TrimSpace(fullName), ".git")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", "", errors.New("repo_full_name must be in owner/repo format")
	}
	return parts[0], parts[1], nil
}

type githubContentItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type githubContentFile struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func fetchGitHubWorkflowFiles(ctx context.Context, token, owner, repoName string) (map[string][]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/.github/workflows", owner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return map[string][]byte{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var items []githubContentItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	files := make(map[string][]byte)
	for _, item := range items {
		if item.Type != "file" {
			continue
		}
		name := item.Name
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		content, err := fetchGitHubFile(ctx, token, owner, repoName, item.Path)
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return files, nil
}

func fetchGitHubFile(ctx context.Context, token, owner, repoName, path string) ([]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repoName, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var file githubContentFile
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, err
	}
	if file.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported content encoding: %s", file.Encoding)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
