package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
