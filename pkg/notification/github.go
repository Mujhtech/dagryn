package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func CommitStatus(ctx context.Context, token, owner, repoName, sha, state, desc, targetURL string) error {
	statusURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/statuses/%s", owner, repoName, sha)
	statusBody := map[string]string{
		"state":       state,
		"description": desc,
		"context":     "Dagryn / workflow",
	}

	if targetURL != "" {
		statusBody["target_url"] = targetURL
	}

	if err := SendGitHubJSON(ctx, token, http.MethodPost, statusURL, statusBody, nil); err != nil {
		return err
	}

	return nil
}

func SendGitHubJSON(ctx context.Context, token, method, url string, body interface{}, v any) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(resp.Body)
		return fmt.Errorf("github request failed with status %d", resp.StatusCode)
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return err
		}
	}

	return nil
}

// CheckRunOutput represents the output section of a GitHub check run.
type CheckRunOutput struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Text    string `json:"text,omitempty"`
}

// CheckRunRequest is used to create or update a GitHub check run.
type CheckRunRequest struct {
	Name        string          `json:"name,omitempty"`
	HeadSHA     string          `json:"head_sha,omitempty"`
	Status      string          `json:"status,omitempty"`       // queued, in_progress, completed
	Conclusion  string          `json:"conclusion,omitempty"`   // success, failure, cancelled, neutral, timed_out, action_required, stale, skipped
	DetailsURL  string          `json:"details_url,omitempty"`  // link back to Dagryn
	StartedAt   *time.Time      `json:"started_at,omitempty"`   // RFC3339
	CompletedAt *time.Time      `json:"completed_at,omitempty"` // RFC3339
	Output      *CheckRunOutput `json:"output,omitempty"`
}

// CreateCheckRun creates a new GitHub check run and returns its ID.
func CreateCheckRun(ctx context.Context, token, owner, repoName string, req CheckRunRequest) (int64, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/check-runs", owner, repoName)
	var respBody struct {
		ID int64 `json:"id"`
	}
	if err := SendGitHubJSON(ctx, token, http.MethodPost, url, req, &respBody); err != nil {
		return 0, err
	}
	return respBody.ID, nil
}

// UpdateCheckRun updates an existing GitHub check run.
func UpdateCheckRun(ctx context.Context, token, owner, repoName string, checkRunID int64, req CheckRunRequest) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/check-runs/%d", owner, repoName, checkRunID)
	return SendGitHubJSON(ctx, token, http.MethodPatch, url, req, nil)
}
