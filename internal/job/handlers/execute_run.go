package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/internal/config"
	"github.com/mujhtech/dagryn/internal/dag"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/encrypt"
	"github.com/mujhtech/dagryn/internal/executor"
	"github.com/mujhtech/dagryn/internal/githubapp"
	"github.com/mujhtech/dagryn/internal/notification"
	"github.com/mujhtech/dagryn/internal/scheduler"
)

// githubAppClientAdapter adapts githubapp.Client to GitHubAppClient interface.
type githubAppClientAdapter struct {
	client *githubapp.Client
}

func (a *githubAppClientAdapter) FetchInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error) {
	if a.client == nil {
		return nil, fmt.Errorf("github App client not configured")
	}
	token, err := a.client.FetchInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	return &InstallationToken{
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

const (
	logFlushInterval = 2 * time.Second
	logBufferSize    = 50
)

// ExecuteRunHandler handles the execute_run job: clone repo, load config, run workflow, report status.
type ExecuteRunHandler struct {
	runs                *repo.RunRepo
	projects            *repo.ProjectRepo
	encrypter           encrypt.Encrypt
	providerTokens      *repo.ProviderTokenRepo
	providerEncrypt     encrypt.Encrypt
	githubApp           GitHubAppClient
	githubInstallations *repo.GitHubInstallationRepo
}

// GitHubAppClient is an interface for fetching installation tokens.
type GitHubAppClient interface {
	FetchInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error)
}

// InstallationToken represents a GitHub App installation access token.
type InstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

// NewGitHubAppClientAdapter creates an adapter from githubapp.Client to GitHubAppClient.
func NewGitHubAppClientAdapter(client *githubapp.Client) GitHubAppClient {
	if client == nil {
		return nil
	}
	return &githubAppClientAdapter{client: client}
}

// NewExecuteRunHandler creates an ExecuteRun handler.
func NewExecuteRunHandler(runs *repo.RunRepo, projects *repo.ProjectRepo, encrypter encrypt.Encrypt, providerTokens *repo.ProviderTokenRepo, providerEncrypt encrypt.Encrypt, githubApp GitHubAppClient, githubInstallations *repo.GitHubInstallationRepo) *ExecuteRunHandler {
	return &ExecuteRunHandler{
		runs:                runs,
		projects:            projects,
		encrypter:           encrypter,
		providerTokens:      providerTokens,
		providerEncrypt:     providerEncrypt,
		githubApp:           githubApp,
		githubInstallations: githubInstallations,
	}
}

// createSyntheticTask creates a task result for infrastructure operations like clone/cleanup.
func (h *ExecuteRunHandler) createSyntheticTask(ctx context.Context, runID uuid.UUID, taskName string) error {
	tr := &models.TaskResult{
		RunID:    runID,
		TaskName: taskName,
		Status:   models.TaskStatusRunning,
	}
	return h.runs.CreateTaskResult(ctx, tr)
}

// completeSyntheticTask completes a synthetic task with status, duration, and optional logs.
func (h *ExecuteRunHandler) completeSyntheticTask(ctx context.Context, runID uuid.UUID, taskName string, status models.TaskStatus, duration time.Duration, logs []string) error {
	tr, err := h.runs.GetTaskResult(ctx, runID, taskName)
	if err != nil {
		return err
	}
	tr.Status = status
	dur := duration.Milliseconds()
	tr.DurationMs = &dur
	exitCode := 0
	if status == models.TaskStatusFailed {
		exitCode = 1
	}
	tr.ExitCode = &exitCode
	now := time.Now()
	tr.FinishedAt = &now

	if err := h.runs.UpdateTaskResult(ctx, tr); err != nil {
		return err
	}

	// Append logs for the synthetic task
	if len(logs) > 0 {
		runLogs := make([]models.RunLog, len(logs))
		for i, line := range logs {
			runLogs[i] = models.RunLog{
				RunID:    runID,
				TaskName: taskName,
				Stream:   models.LogStreamStdout,
				LineNum:  i + 1,
				Content:  line,
			}
		}
		if err := h.runs.AppendLogs(ctx, runLogs); err != nil {
			slog.Warn("execute_run: append synthetic logs failed", "task", taskName, "error", err)
		}
	}

	return nil
}

// Handle processes the execute_run task.
func (h *ExecuteRunHandler) Handle(ctx context.Context, t *asynq.Task) error {
	rawPayload := string(t.Payload())
	var plaintext string
	if h.encrypter != nil {
		var err error
		plaintext, err = h.encrypter.Decrypt(rawPayload)
		if err != nil {
			slog.Error("execute_run: decrypt failed", "error", err)
			return err
		}
	} else {
		plaintext = rawPayload
	}

	var payload struct {
		ProjectID string   `json:"project_id"`
		RunID     string   `json:"run_id"`
		Targets   []string `json:"targets"`
		GitBranch string   `json:"git_branch,omitempty"`
		GitCommit string   `json:"git_commit,omitempty"`
		RepoURL   string   `json:"repo_url,omitempty"`
	}
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		slog.Error("execute_run: parse payload failed", "error", err)
		return err
	}

	runID, err := uuid.Parse(payload.RunID)
	if err != nil {
		return fmt.Errorf("invalid run_id: %w", err)
	}
	projectID, err := uuid.Parse(payload.ProjectID)
	if err != nil {
		return fmt.Errorf("invalid project_id: %w", err)
	}

	// Load run and project
	run, err := h.runs.GetByID(ctx, runID)
	if err != nil {
		if err == repo.ErrNotFound {
			slog.Warn("execute_run: run not found", "run_id", runID)
			return nil
		}
		return err
	}
	project, err := h.projects.GetByID(ctx, projectID)
	if err != nil {
		if err == repo.ErrNotFound {
			slog.Warn("execute_run: project not found", "project_id", projectID)
			return nil
		}
		return err
	}

	repoURL := payload.RepoURL
	if repoURL == "" && project.RepoURL != nil {
		repoURL = *project.RepoURL
	}
	if repoURL == "" {
		msg := "project has no repo_url; server execution requires a linked repository"
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		slog.Info("execute_run: skipped, no repo_url", "run_id", runID)
		return nil
	}

	// Create temp dir for clone
	workDir, err := os.MkdirTemp("", "dagryn-run-")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}

	// Track cleanup as a synthetic task
	cleanupStart := time.Now()
	defer func() {
		// Create and complete cleanup task
		if createErr := h.createSyntheticTask(ctx, runID, models.SyntheticTaskCleanup); createErr != nil {
			slog.Warn("execute_run: create cleanup task failed", "run_id", runID, "error", createErr)
		} else {
			removeErr := os.RemoveAll(workDir)
			cleanupDuration := time.Since(cleanupStart)
			cleanupStatus := models.TaskStatusSuccess
			cleanupLogs := []string{fmt.Sprintf("Removing temp directory: %s", workDir)}
			if removeErr != nil {
				cleanupStatus = models.TaskStatusFailed
				cleanupLogs = append(cleanupLogs, fmt.Sprintf("Error: %v", removeErr))
			} else {
				cleanupLogs = append(cleanupLogs, "Cleanup completed successfully")
			}
			if completeErr := h.completeSyntheticTask(ctx, runID, models.SyntheticTaskCleanup, cleanupStatus, cleanupDuration, cleanupLogs); completeErr != nil {
				slog.Warn("execute_run: complete cleanup task failed", "run_id", runID, "error", completeErr)
			}
		}
	}()

	// Start clone task
	cloneStart := time.Now()
	if err := h.createSyntheticTask(ctx, runID, models.SyntheticTaskClone); err != nil {
		slog.Warn("execute_run: create clone task failed", "run_id", runID, "error", err)
	}
	cloneLogs := []string{fmt.Sprintf("Cloning repository: %s", repoURL)}

	// Clone options: use repo-linked user's stored GitHub token, or fall back to env token
	cloneOpts := &git.CloneOptions{URL: repoURL}
	if auth := h.cloneAuth(ctx, repoURL, project); auth != nil {
		cloneOpts.Auth = auth
	}

	// Clone repository
	_, err = git.PlainClone(workDir, false, cloneOpts)
	if err != nil {
		msg := fmt.Sprintf("clone failed: %v", err)
		cloneLogs = append(cloneLogs, fmt.Sprintf("Error: %v", err))
		_ = h.completeSyntheticTask(ctx, runID, models.SyntheticTaskClone, models.TaskStatusFailed, time.Since(cloneStart), cloneLogs)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return fmt.Errorf("clone: %w", err)
	}
	gitRepo, err := git.PlainOpen(workDir)
	if err != nil {
		msg := fmt.Sprintf("open repo: %v", err)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return err
	}
	w, err := gitRepo.Worktree()
	if err != nil {
		msg := fmt.Sprintf("worktree: %v", err)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return err
	}
	if payload.GitCommit != "" {
		cloneLogs = append(cloneLogs, fmt.Sprintf("Checking out commit: %s", payload.GitCommit))
		err = w.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(payload.GitCommit)})
		if err != nil {
			msg := fmt.Sprintf("checkout commit %q: %v", payload.GitCommit, err)
			cloneLogs = append(cloneLogs, fmt.Sprintf("Error: %v", err))
			_ = h.completeSyntheticTask(ctx, runID, models.SyntheticTaskClone, models.TaskStatusFailed, time.Since(cloneStart), cloneLogs)
			_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
			return err
		}
	} else if payload.GitBranch != "" {
		cloneLogs = append(cloneLogs, fmt.Sprintf("Checking out branch: %s", payload.GitBranch))
		err = w.Checkout(&git.CheckoutOptions{Branch: plumbing.ReferenceName("refs/heads/" + payload.GitBranch)})
		if err != nil {
			msg := fmt.Sprintf("checkout branch %q: %v", payload.GitBranch, err)
			cloneLogs = append(cloneLogs, fmt.Sprintf("Error: %v", err))
			_ = h.completeSyntheticTask(ctx, runID, models.SyntheticTaskClone, models.TaskStatusFailed, time.Since(cloneStart), cloneLogs)
			_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
			return err
		}
	}

	// Clone completed successfully
	cloneLogs = append(cloneLogs, "Clone completed successfully")
	_ = h.completeSyntheticTask(ctx, runID, models.SyntheticTaskClone, models.TaskStatusSuccess, time.Since(cloneStart), cloneLogs)

	// Capture the actual commit hash we checked out
	head, err := gitRepo.Head()
	if err == nil {
		commitSHA := head.Hash().String()
		// Update run with the actual commit hash if it wasn't provided or differs
		if run.GitCommit == nil || *run.GitCommit == "" || *run.GitCommit != commitSHA {
			run.GitCommit = &commitSHA
			if err := h.runs.Update(ctx, run); err != nil {
				slog.Warn("execute_run: failed to update run with commit hash", "run_id", runID, "error", err)
			} else {
				slog.Info("execute_run: updated run with commit hash", "run_id", runID, "commit", commitSHA)
			}
		}
	} else {
		slog.Warn("execute_run: failed to get HEAD commit", "run_id", runID, "error", err)
	}

	// Load config
	configPath := project.ConfigPath
	if configPath == "" {
		configPath = config.DefaultConfigFile
	}
	configPath = filepath.Join(workDir, configPath)
	cfg, err := config.Parse(configPath)
	if err != nil {
		msg := fmt.Sprintf("load config: %v", err)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return fmt.Errorf("config: %w", err)
	}

	workflow, err := cfg.ToWorkflow()
	if err != nil {
		msg := fmt.Sprintf("workflow: %v", err)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return fmt.Errorf("workflow: %w", err)
	}

	targets := payload.Targets
	if len(targets) == 0 {
		targets = run.Targets
	}

	// For CI-triggered runs (push/PR), if no targets specified, use all tasks from dagryn.toml
	if len(targets) == 0 && run.TriggeredBy == models.TriggerSourceCI {
		targets = workflow.TaskNames()
		// Save all task names as targets for CI runs
		if err := h.runs.UpdateTargets(ctx, runID, targets); err != nil {
			slog.Warn("execute_run: failed to update targets", "run_id", runID, "error", err)
		}
	} else if len(targets) == 0 && workflow.Default {
		// For non-CI runs, use default workflow if available
		targets = workflow.TaskNames()
	}

	if len(targets) == 0 {
		msg := "no targets specified and no default workflow"
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return nil
	}

	for _, name := range targets {
		if _, ok := workflow.GetTask(name); !ok {
			msg := fmt.Sprintf("task %q not found", name)
			_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
			return nil
		}
	}

	// Build DAG and get total task count
	taskDeps := make(map[string][]string)
	for _, t := range workflow.ListTasks() {
		taskDeps[t.Name] = t.Needs
	}
	g, err := dag.BuildFromWorkflow(taskDeps)
	if err != nil {
		msg := fmt.Sprintf("build DAG: %v", err)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return err
	}
	if err := dag.DetectCycle(g); err != nil {
		msg := err.Error()
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return err
	}
	plan, err := dag.TopoSortFrom(g, targets)
	if err != nil {
		msg := fmt.Sprintf("toposort: %v", err)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return err
	}
	totalTasks := plan.TotalTasks()
	if totalTasks == 0 {
		_ = h.runs.Complete(ctx, runID, models.RunStatusSuccess, nil)
		return nil
	}

	if err := h.runs.StartWithTotal(ctx, runID, totalTasks); err != nil {
		return fmt.Errorf("start run: %w", err)
	}

	// Notify GitHub that run has started
	h.notifyGitHub(ctx, run, project, models.RunStatusRunning)

	opts := scheduler.DefaultOptions()
	opts.NoPlugins = true
	opts.NoCache = false
	sched, err := scheduler.New(workflow, workDir, opts)
	if err != nil {
		msg := fmt.Sprintf("scheduler: %v", err)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		return err
	}

	// Log buffer for batch append
	var logMu sync.Mutex
	logBuffer := make([]models.RunLog, 0, logBufferSize)
	lineNums := make(map[string]int)

	flushLogs := func() {
		logMu.Lock()
		if len(logBuffer) == 0 {
			logMu.Unlock()
			return
		}
		toSend := make([]models.RunLog, len(logBuffer))
		copy(toSend, logBuffer)
		logBuffer = logBuffer[:0]
		logMu.Unlock()
		if err := h.runs.AppendLogs(ctx, toSend); err != nil {
			slog.Warn("execute_run: append logs failed", "error", err)
		}
	}

	sched.OnTaskStart(func(name string, _ *executor.Result, cacheHit bool) {
		tr := &models.TaskResult{
			RunID:    runID,
			TaskName: name,
			Status:   models.TaskStatusRunning,
		}
		if err := h.runs.CreateTaskResult(ctx, tr); err != nil {
			slog.Warn("execute_run: create task failed", "task", name, "error", err)
			return
		}
		logMu.Lock()
		lineNums[name] = 0
		logMu.Unlock()
	})

	sched.OnTaskComplete(func(name string, result *executor.Result, cacheHit bool) {
		tr, err := h.runs.GetTaskResult(ctx, runID, name)
		if err != nil {
			slog.Warn("execute_run: get task result failed", "task", name, "error", err)
			return
		}
		status := executorStatusToTaskStatus(result.Status)
		tr.Status = status
		dur := result.Duration.Milliseconds()
		tr.DurationMs = &dur
		tr.ExitCode = &result.ExitCode
		now := time.Now()
		tr.FinishedAt = &now
		tr.CacheHit = cacheHit
		if err := h.runs.UpdateTaskResult(ctx, tr); err != nil {
			slog.Warn("execute_run: update task failed", "task", name, "error", err)
		}
		if result.Status == executor.Failed || result.Status == executor.TimedOut || result.Status == executor.Cancelled {
			_ = h.runs.IncrementFailed(ctx, runID)
		} else {
			_ = h.runs.IncrementCompleted(ctx, runID, cacheHit)
		}
	})

	sched.OnLogLine(func(taskName, stream, line string) {
		logMu.Lock()
		lineNums[taskName]++
		ln := lineNums[taskName]
		logBuffer = append(logBuffer, models.RunLog{
			RunID:    runID,
			TaskName: taskName,
			Stream:   models.LogStream(stream),
			LineNum:  ln,
			Content:  line,
		})
		if len(logBuffer) >= logBufferSize {
			toSend := make([]models.RunLog, len(logBuffer))
			copy(toSend, logBuffer)
			logBuffer = logBuffer[:0]
			logMu.Unlock()
			_ = h.runs.AppendLogs(ctx, toSend)
			return
		}
		logMu.Unlock()
	})

	// Periodic log flush
	stopFlush := make(chan struct{})
	defer close(stopFlush)
	go func() {
		ticker := time.NewTicker(logFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopFlush:
				return
			case <-ticker.C:
				flushLogs()
			}
		}
	}()

	summary, err := sched.Run(ctx, targets)
	flushLogs()

	if err != nil {
		msg := err.Error()
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		h.notifyGitHub(ctx, run, project, models.RunStatusFailed)
		return err
	}

	// The scheduler returns a summary even when tasks fail; failures are tracked in summary.Failures.
	// Treat any failure as a run failure.
	if summary != nil && summary.Failures > 0 {
		msg := fmt.Sprintf("%d task(s) failed", summary.Failures)
		_ = h.runs.Complete(ctx, runID, models.RunStatusFailed, &msg)
		h.notifyGitHub(ctx, run, project, models.RunStatusFailed)
		return nil
	}

	_ = h.runs.Complete(ctx, runID, models.RunStatusSuccess, nil)
	h.notifyGitHub(ctx, run, project, models.RunStatusSuccess)
	return nil
}

func executorStatusToTaskStatus(s executor.Status) models.TaskStatus {
	switch s {
	case executor.Success:
		return models.TaskStatusSuccess
	case executor.Failed:
		return models.TaskStatusFailed
	case executor.Cached:
		return models.TaskStatusCached
	case executor.Skipped:
		return models.TaskStatusSkipped
	case executor.TimedOut:
		return models.TaskStatusFailed
	case executor.Cancelled:
		return models.TaskStatusCancelled
	default:
		return models.TaskStatusFailed
	}
}

// cloneAuth returns Auth for private Git clones: prefers the repo-linked user's stored GitHub token,
// then falls back to GITHUB_TOKEN or DAGRYN_CLONE_TOKEN from the environment.
func (h *ExecuteRunHandler) cloneAuth(ctx context.Context, repoURL string, project *models.Project) transport.AuthMethod {
	// Prefer GitHub App installation token if available
	if project.GitHubInstallationID != nil && h.githubApp != nil && h.githubInstallations != nil {
		inst, err := h.githubInstallations.GetByID(ctx, *project.GitHubInstallationID)
		if err == nil && inst != nil {
			token, err := h.githubApp.FetchInstallationToken(ctx, inst.InstallationID)
			if err == nil && token != nil && token.Token != "" {
				return &githttp.BasicAuth{Username: "x-access-token", Password: token.Token}
			}
		}
	}

	// Fallback to OAuth token: use the user who linked the repo (their stored GitHub token)
	if project.RepoLinkedByUserID != nil && h.providerTokens != nil && h.providerEncrypt != nil {
		tok, err := h.providerTokens.GetByUserAndProvider(ctx, *project.RepoLinkedByUserID, "github")
		if err == nil && tok != nil {
			accessToken, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
			if err == nil && accessToken != "" {
				return &githttp.BasicAuth{Username: "oauth2", Password: accessToken}
			}
		}
	}
	return cloneAuthFromEnv(repoURL)
}

// cloneAuthFromEnv returns Auth for private Git clones when a token is set in the environment.
// Supports GITHUB_TOKEN (GitHub only) or DAGRYN_CLONE_TOKEN (any HTTPS host). For GitHub,
// use a personal access token with repo scope so the worker can clone private repositories.
func cloneAuthFromEnv(repoURL string) transport.AuthMethod {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return nil
	}
	lower := strings.ToLower(repoURL)
	isGitHub := strings.Contains(lower, "github.com")

	// Prefer GITHUB_TOKEN for GitHub; otherwise DAGRYN_CLONE_TOKEN for any host
	var token string
	if isGitHub {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token == "" {
		token = strings.TrimSpace(os.Getenv("DAGRYN_CLONE_TOKEN"))
	}
	if token == "" {
		return nil
	}

	// GITHUB_TOKEN is only used for GitHub URLs (already ensured above)
	// DAGRYN_CLONE_TOKEN is used for any URL
	return &githttp.BasicAuth{
		Username: "oauth2",
		Password: token,
	}
}

// notifyGitHub updates GitHub commit status and posts a PR summary comment for runs
// that originated from GitHub PR/push events.
func (h *ExecuteRunHandler) notifyGitHub(ctx context.Context, run *models.Run, project *models.Project, status models.RunStatus) {
	// Reload run to ensure we have the latest state (e.g. completion status)
	// and to avoid overwriting it when we save the comment ID later.
	latestRun, err := h.runs.GetByID(ctx, run.ID)
	if err == nil {
		run = latestRun
	} else {
		slog.Warn("notifyGitHub: failed to reload run", "run_id", run.ID, "error", err)
		// Proceed with the stale run object, but be aware of potential overwrite risks
	}

	// Only notify for PR runs with a commit SHA
	if run.PRNumber == nil && run.GitCommit == nil {
		return
	}
	if project.RepoURL == nil || *project.RepoURL == "" {
		return
	}
	if run.GitCommit == nil || *run.GitCommit == "" {
		return
	}

	// Obtain access token - prefer GitHub App installation token
	var accessToken string

	if project.GitHubInstallationID != nil && h.githubApp != nil && h.githubInstallations != nil {
		inst, err := h.githubInstallations.GetByID(ctx, *project.GitHubInstallationID)
		if err == nil && inst != nil {
			token, err := h.githubApp.FetchInstallationToken(ctx, inst.InstallationID)
			if err == nil && token != nil {
				accessToken = token.Token
			}
		}
	}

	// Fallback to OAuth token
	if accessToken == "" && h.providerTokens != nil && h.providerEncrypt != nil && project.RepoLinkedByUserID != nil {
		tok, err := h.providerTokens.GetByUserAndProvider(ctx, *project.RepoLinkedByUserID, "github")
		if err == nil && tok != nil {
			decrypted, err := h.providerEncrypt.Decrypt(tok.AccessTokenEncrypted)
			if err == nil {
				accessToken = decrypted
			}
		}
	}

	if accessToken == "" {
		slog.Debug("no_github_token_for_notification", "run_id", run.ID, "project_id", project.ID)
		return
	}

	// Parse owner/repo from URL
	owner, repoName, err := parseGitHubOwnerRepoFromURL(*project.RepoURL)
	if err != nil {
		slog.Debug("failed_to_parse_github_url", "url", *project.RepoURL, "error", err)
		return
	}

	sha := *run.GitCommit

	// Map status to GitHub state
	state := "pending"
	switch status {
	case models.RunStatusSuccess:
		state = "success"
	case models.RunStatusFailed:
		state = "failure"
	case models.RunStatusCancelled:
		state = "error"
	}

	// Build description
	desc := fmt.Sprintf("Dagryn run %s", status)
	if run.DurationMs != nil {
		desc = fmt.Sprintf("Dagryn run %s in %dms", status, *run.DurationMs)
	}

	// Build target URL (link back to Dagryn run detail)
	baseURL := "https://dagryn.mujhtech.xyz" // optional: derive from config later
	targetURL := ""
	if baseURL != "" {
		targetURL = fmt.Sprintf("%s/projects/%s/runs/%s", strings.TrimRight(baseURL, "/"), project.ID, run.ID)
	}

	// Post commit status
	if err := notification.CommitStatus(ctx, accessToken, owner, repoName, sha, state, desc, targetURL); err != nil {
		slog.Error("github_status_update_failed", "run_id", run.ID, "error", err)
	}

	// 2) PR summary comment (create once, then update same comment)
	commentBody := map[string]string{
		"body": buildGitHubPRComment(run, status, targetURL),
	}

	if run.GitHubPRCommentID != nil {
		// Update existing comment
		commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%d", owner, repoName, *run.GitHubPRCommentID)
		if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPatch, commentURL, commentBody, nil); err != nil {
			slog.Error("github_pr_comment_update_failed", "run_id", run.ID, "error", err)
		}
	} else {
		// Create new comment and persist its ID
		commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repoName, *run.PRNumber)
		var respBody struct {
			ID int64 `json:"id"`
		}

		if err := notification.SendGitHubJSON(ctx, accessToken, http.MethodPost, commentURL, commentBody, &respBody); err != nil {
			slog.Error("github_pr_comment_create_failed", "run_id", run.ID, "error", err)
		}
		slog.Info("github_pr_comment_created", "run_id", run.ID, "comment_id", respBody.ID)
		if respBody.ID != 0 {
			run.GitHubPRCommentID = &respBody.ID
			if err := h.runs.Update(ctx, run); err != nil {
				slog.Error("github_pr_comment_id_persist_failed", "run_id", run.ID, "error", err)
			}
			slog.Info("github_pr_comment_id_persisted", "run_id", run.ID, "comment_id", respBody.ID)
		}
	}

	slog.Info("github_notification_sent", "run_id", run.ID, "status", status, "sha", sha)
}

// buildGitHubPRComment constructs a human-friendly summary comment for a run.
func buildGitHubPRComment(run *models.Run, status models.RunStatus, targetURL string) string {
	var b strings.Builder
	icon := "🟡"
	switch status {
	case models.RunStatusSuccess:
		icon = "✅"
	case models.RunStatusFailed:
		icon = "❌"
	case models.RunStatusCancelled:
		icon = "⚪️"
	}

	fmt.Fprintf(&b, "%s **Dagryn workflow %s**\n\n", icon, strings.ToUpper(string(status)))

	if run.PRTitle != nil && *run.PRTitle != "" {
		fmt.Fprintf(&b, "- **PR**: %s\n", *run.PRTitle)
	}
	if run.CommitMessage != nil && *run.CommitMessage != "" {
		fmt.Fprintf(&b, "- **Commit**: %s\n", *run.CommitMessage)
	}
	if run.GitBranch != nil && *run.GitBranch != "" {
		fmt.Fprintf(&b, "- **Branch**: `%s`\n", *run.GitBranch)
	}
	if run.GitCommit != nil && *run.GitCommit != "" {
		sha := *run.GitCommit
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fmt.Fprintf(&b, "- **SHA**: `%s`\n", sha)
	}
	if run.DurationMs != nil {
		fmt.Fprintf(&b, "- **Duration**: %s\n", formatDurationMs(*run.DurationMs))
	}

	if targetURL != "" {
		fmt.Fprintf(&b, "\n[View run in Dagryn](%s)\n", targetURL)
	}

	return b.String()
}

// formatDurationMs formats a millisecond duration into a human-readable string.
func formatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	sec := ms / 1000
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	min := sec / 60
	sec = sec % 60
	return fmt.Sprintf("%dm %ds", min, sec)
}

// parseGitHubOwnerRepoFromURL extracts owner and repo from a GitHub URL.
func parseGitHubOwnerRepoFromURL(repoURL string) (owner, repo string, err error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimSuffix(repoURL, "/")

	// Handle HTTPS URLs: https://github.com/owner/repo
	if strings.HasPrefix(repoURL, "https://github.com/") {
		parts := strings.Split(strings.TrimPrefix(repoURL, "https://github.com/"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	// Handle SSH URLs: git@github.com:owner/repo
	if strings.HasPrefix(repoURL, "git@github.com:") {
		parts := strings.Split(strings.TrimPrefix(repoURL, "git@github.com:"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("cannot parse GitHub URL: %s", repoURL)
}
