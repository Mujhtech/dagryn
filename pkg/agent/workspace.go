package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// WorkspaceManager handles workspace preparation for remote task execution.
type WorkspaceManager struct {
	baseDir string
}

// NewWorkspaceManager creates a new workspace manager.
func NewWorkspaceManager(baseDir string) *WorkspaceManager {
	return &WorkspaceManager{baseDir: baseDir}
}

// PrepareGit clones a git repository and checks out the specified ref.
func (w *WorkspaceManager) PrepareGit(ctx context.Context, repoURL, ref, commit, token string) (string, error) {
	suffix := commit
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	if suffix == "" {
		suffix = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	workdir := filepath.Join(w.baseDir, "workspace-"+suffix)

	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return "", fmt.Errorf("create workspace dir: %w", err)
	}

	// Build clone command
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}

	cloneURL := repoURL
	if token != "" {
		// Inject token into URL for HTTPS auth
		cloneURL = injectToken(repoURL, token)
	}
	args = append(args, cloneURL, workdir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = w.baseDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

	// Checkout specific commit if different from branch HEAD
	if commit != "" {
		cmd := exec.CommandContext(ctx, "git", "checkout", commit)
		cmd.Dir = workdir
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git checkout %s failed: %s: %w", commit, string(output), err)
		}
	}

	return workdir, nil
}

// PrepareArtifact downloads and extracts a workspace tarball from a signed URL.
func (w *WorkspaceManager) PrepareArtifact(ctx context.Context, signedURL string) (string, error) {
	workdir := filepath.Join(w.baseDir, "workspace-artifact")

	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return "", fmt.Errorf("create workspace dir: %w", err)
	}

	// Download tarball
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download workspace: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Write to temp file
	tmpFile := filepath.Join(w.baseDir, "workspace.tar.gz")
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	_ = f.Close()
	defer func() {
		_ = os.Remove(tmpFile)
	}()

	// Extract
	cmd := exec.CommandContext(ctx, "tar", "xzf", tmpFile, "-C", workdir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("extract workspace: %s: %w", string(output), err)
	}

	return workdir, nil
}

// Cleanup removes a workspace directory.
func (w *WorkspaceManager) Cleanup(workdir string) error {
	return os.RemoveAll(workdir)
}

func injectToken(repoURL, token string) string {
	// For HTTPS URLs, inject token: https://x-access-token:TOKEN@github.com/...
	if len(repoURL) > 8 && repoURL[:8] == "https://" {
		return "https://x-access-token:" + token + "@" + repoURL[8:]
	}
	return repoURL
}
