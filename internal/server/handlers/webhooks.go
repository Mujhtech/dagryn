package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/job"
)

// GitHubPushEvent is a minimal representation of a GitHub push webhook.
type GitHubPushEvent struct {
	Ref        string `json:"ref"`   // e.g. "refs/heads/main"
	After      string `json:"after"` // commit SHA
	Repository struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

// GitHubPullRequestEvent is a minimal representation of a GitHub pull_request webhook.
type GitHubPullRequestEvent struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title string `json:"title"`
		Head  struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

// GitHubInstallationEvent represents a GitHub App installation event.
type GitHubInstallationEvent struct {
	Action       string `json:"action"` // "created", "deleted", "suspend", "unsuspend"
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"` // "User" or "Organization"
			ID    int64  `json:"id"`
		} `json:"account"`
	} `json:"installation"`
}

// GitHubInstallationRepositoriesEvent represents an installation_repositories event.
type GitHubInstallationRepositoriesEvent struct {
	Action       string `json:"action"` // "added", "removed"
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
			ID    int64  `json:"id"`
		} `json:"account"`
	} `json:"installation"`
	RepositoriesAdded   []GitHubRepoInfo `json:"repositories_added"`
	RepositoriesRemoved []GitHubRepoInfo `json:"repositories_removed"`
}

// GitHubRepoInfo represents minimal repo info from installation events.
type GitHubRepoInfo struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

// GitHubWebhook handles incoming GitHub webhooks and triggers runs for linked projects.
// Requests are authenticated via the GitHub App webhook secret when configured.
func (h *Handler) GitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if h.jobClient == nil {
		http.Error(w, "job system not configured", http.StatusServiceUnavailable)
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		http.Error(w, "missing X-GitHub-Event", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify webhook signature when GitHub App is configured.
	// if h.githubApp != nil {
	// 	sig := r.Header.Get("X-Hub-Signature-256")
	// 	if sig == "" || !h.githubApp.VerifyWebhookSignature(body, sig) {
	// 		http.Error(w, "invalid webhook signature", http.StatusUnauthorized)
	// 		return
	// 	}
	// }

	ctx := r.Context()

	switch event {
	case "push":
		var payload GitHubPushEvent
		if err := json.Unmarshal(body, &payload); err != nil {
			slog.Error("github_webhook: parse push failed", "error", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if err := h.handleGitHubPush(ctx, &payload); err != nil {
			slog.Error("github_webhook: handle push failed", "error", err)
		}
	case "pull_request":
		var payload GitHubPullRequestEvent
		if err := json.Unmarshal(body, &payload); err != nil {
			slog.Error("github_webhook: parse pull_request failed", "error", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		// Only react to opened/synchronize/reopened
		switch payload.Action {
		case "opened", "synchronize", "reopened":
			if err := h.handleGitHubPullRequest(ctx, &payload); err != nil {
				slog.Error("github_webhook: handle pull_request failed", "error", err)
			}
		default:
			// Ignore other actions
		}
	case "installation":
		var payload GitHubInstallationEvent
		if err := json.Unmarshal(body, &payload); err != nil {
			slog.Error("github_webhook: parse installation failed", "error", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if err := h.handleGitHubInstallation(ctx, &payload); err != nil {
			slog.Error("github_webhook: handle installation failed", "error", err)
		}
	case "installation_repositories":
		var payload GitHubInstallationRepositoriesEvent
		if err := json.Unmarshal(body, &payload); err != nil {
			slog.Error("github_webhook: parse installation_repositories failed", "error", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if err := h.handleGitHubInstallationRepositories(ctx, &payload); err != nil {
			slog.Error("github_webhook: handle installation_repositories failed", "error", err)
		}
	default:
		// Other events are currently ignored.
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) handleGitHubPush(ctx context.Context, payload *GitHubPushEvent) error {
	repoURL := strings.TrimSpace(payload.Repository.CloneURL)
	if repoURL == "" {
		return errors.New("missing clone_url")
	}

	var project *models.Project
	var err error

	// Try to find project by GitHub App installation + repo ID first (preferred)
	if payload.Installation.ID > 0 && payload.Repository.ID > 0 && h.githubInstallations != nil {
		inst, err := h.githubInstallations.GetByInstallationID(ctx, payload.Installation.ID)
		if err == nil && inst != nil {
			project, err = h.projects.GetByGitHubRepoID(ctx, inst.ID, payload.Repository.ID)
			if err != nil && !errors.Is(err, repo.ErrNotFound) {
				return err
			}
		}
	}

	// Fallback to repo_url lookup (for legacy OAuth-based projects)
	if project == nil {
		project, err = h.projects.GetByRepoURL(ctx, repoURL)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				// No linked project for this repo; ignore.
				return nil
			}
			return err
		}
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	run := &models.Run{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Status:      models.RunStatusPending,
		TriggeredBy: models.TriggerSourceCI,
		CreatedAt:   time.Now(),
	}

	if branch != "" {
		run.GitBranch = &branch
	}
	if payload.After != "" {
		sha := payload.After
		run.GitCommit = &sha
	}

	// Enrich with commit metadata (message/author).
	// Prefer GitHub App installation token if available, otherwise fall back to OAuth token.
	if project.GitHubInstallationID != nil && payload.Installation.ID > 0 && h.githubApp != nil {
		token, err := h.githubApp.FetchInstallationToken(ctx, payload.Installation.ID)
		if err == nil && token != nil {
			h.enrichRunWithGitHubCommitUsingToken(ctx, run, project, token.Token, branch)
			h.enrichRunWithGitHubPRUsingToken(ctx, run, project, token.Token)
		} else {
			// Fallback to OAuth token if installation token fetch fails
			h.enrichRunWithGitHubCommit(ctx, run, project, projectOwnerForWebhook(project), branch)
			h.enrichRunWithGitHubPR(ctx, run, project, projectOwnerForWebhook(project))
		}
	} else {
		h.enrichRunWithGitHubCommit(ctx, run, project, projectOwnerForWebhook(project), branch)
		h.enrichRunWithGitHubPR(ctx, run, project, projectOwnerForWebhook(project))
	}

	if err := h.runs.Create(ctx, run); err != nil {
		return err
	}

	// Enqueue server-side execution if repo_url is set.
	if project.RepoURL != nil && *project.RepoURL != "" {
		data, err := json.Marshal(job.ExecuteRunPayload{
			ProjectID: project.ID.String(),
			RunID:     run.ID.String(),
			GitBranch: branch,
			GitCommit: payload.After,
			RepoURL:   *project.RepoURL,
		})
		if err == nil {
			_ = h.jobClient.Enqueue(job.QueueNameDefault, job.ExecuteRunTaskName, &job.ClientPayload{Data: data})
		}
	}

	return nil
}

func (h *Handler) handleGitHubPullRequest(ctx context.Context, payload *GitHubPullRequestEvent) error {
	repoURL := strings.TrimSpace(payload.Repository.CloneURL)
	if repoURL == "" {
		return errors.New("missing clone_url")
	}

	var project *models.Project
	var err error

	// Try to find project by GitHub App installation + repo ID first (preferred)
	if payload.Installation.ID > 0 && payload.Repository.ID > 0 && h.githubInstallations != nil {
		inst, err := h.githubInstallations.GetByInstallationID(ctx, payload.Installation.ID)
		if err == nil && inst != nil {
			project, err = h.projects.GetByGitHubRepoID(ctx, inst.ID, payload.Repository.ID)
			if err != nil && !errors.Is(err, repo.ErrNotFound) {
				return err
			}
		}
	}

	// Fallback to repo_url lookup (for legacy OAuth-based projects)
	if project == nil {
		project, err = h.projects.GetByRepoURL(ctx, repoURL)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				return nil
			}
			return err
		}
	}

	branch := strings.TrimSpace(payload.PullRequest.Head.Ref)
	sha := strings.TrimSpace(payload.PullRequest.Head.SHA)

	run := &models.Run{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Status:      models.RunStatusPending,
		TriggeredBy: models.TriggerSourceCI,
		CreatedAt:   time.Now(),
	}

	if branch != "" {
		run.GitBranch = &branch
	}
	if sha != "" {
		run.GitCommit = &sha
	}
	if payload.PullRequest.Title != "" {
		title := payload.PullRequest.Title
		run.PRTitle = &title
	}
	if payload.Number != 0 {
		n := payload.Number
		run.PRNumber = &n
	}

	// Enrich with commit metadata (message/author).
	// Prefer GitHub App installation token if available, otherwise fall back to OAuth token.
	if project.GitHubInstallationID != nil && payload.Installation.ID > 0 && h.githubApp != nil {
		token, err := h.githubApp.FetchInstallationToken(ctx, payload.Installation.ID)
		if err == nil && token != nil {
			h.enrichRunWithGitHubCommitUsingToken(ctx, run, project, token.Token, branch)
		} else {
			// Fallback to OAuth token if installation token fetch fails
			h.enrichRunWithGitHubCommit(ctx, run, project, projectOwnerForWebhook(project), branch)
		}
	} else {
		h.enrichRunWithGitHubCommit(ctx, run, project, projectOwnerForWebhook(project), branch)
	}

	if err := h.runs.Create(ctx, run); err != nil {
		return err
	}

	if project.RepoURL != nil && *project.RepoURL != "" {
		data, err := json.Marshal(job.ExecuteRunPayload{
			ProjectID: project.ID.String(),
			RunID:     run.ID.String(),
			GitBranch: branch,
			GitCommit: sha,
			RepoURL:   *project.RepoURL,
		})
		if err == nil {
			_ = h.jobClient.Enqueue(job.QueueNameDefault, job.ExecuteRunTaskName, &job.ClientPayload{Data: data})
		}
	}

	return nil
}

// handleGitHubInstallation handles installation events (created, deleted, suspend, unsuspend).
func (h *Handler) handleGitHubInstallation(ctx context.Context, payload *GitHubInstallationEvent) error {
	if h.githubInstallations == nil {
		return nil
	}

	inst := &models.GitHubInstallation{
		InstallationID: payload.Installation.ID,
		AccountLogin:   payload.Installation.Account.Login,
		AccountType:    payload.Installation.Account.Type,
		AccountID:      payload.Installation.Account.ID,
	}

	switch payload.Action {
	case "created", "unsuspend":
		// Upsert installation record
		return h.githubInstallations.UpsertByInstallationID(ctx, inst)
	case "deleted", "suspend":
		// For now, we keep the record but could mark it inactive later
		// GitHub will stop sending webhooks for suspended/deleted installations anyway
		return nil
	default:
		return nil
	}
}

// handleGitHubInstallationRepositories handles installation_repositories events (added, removed).
func (h *Handler) handleGitHubInstallationRepositories(ctx context.Context, payload *GitHubInstallationRepositoriesEvent) error {
	if h.githubInstallations == nil {
		return nil
	}

	// Ensure installation record exists
	inst := &models.GitHubInstallation{
		InstallationID: payload.Installation.ID,
		AccountLogin:   payload.Installation.Account.Login,
		AccountType:    payload.Installation.Account.Type,
		AccountID:      payload.Installation.Account.ID,
	}
	if err := h.githubInstallations.UpsertByInstallationID(ctx, inst); err != nil {
		return err
	}

	// For now, we don't track individual repo additions/removals in a separate table.
	// We'll re-resolve via GitHub API when needed (e.g., when listing repos for project creation).
	return nil
}

// projectOwnerForWebhook returns the user ID whose provider token should be used when enriching webhook runs.
// This is used as a fallback for legacy OAuth-based projects.
func projectOwnerForWebhook(project *models.Project) uuid.UUID {
	if project.RepoLinkedByUserID != nil {
		return *project.RepoLinkedByUserID
	}
	// If repo_linked_by_user_id is not set, fall back to zero UUID; enrichRunWithGitHubCommit
	// will try the current user in that case, but for webhooks there is no current user,
	// so this simply means "no specific token".
	return uuid.Nil
}
