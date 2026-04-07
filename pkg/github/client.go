package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the single entry point for all GitHub REST API interactions.
type Client struct {
	httpClient *http.Client
	token      string
}

// NewClient creates a Client that authenticates with the given token.
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		token:      token,
	}
}

// do executes an authenticated GitHub API request.
// If body is non-nil it is JSON-encoded. If dest is non-nil the response is
// JSON-decoded into it. Returns the HTTP status code and any error.
func (c *Client) do(ctx context.Context, method, url string, body, dest any) (int, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(respBody))
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// ---------------------------------------------------------------------------
// Repository operations
// ---------------------------------------------------------------------------

// GetRepo fetches repository metadata.
func (c *Client) GetRepo(ctx context.Context, owner, repo string) (*Repository, error) {
	var r Repository
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", baseURL, owner, repo), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListUserRepos lists repositories the authenticated user has access to.
func (c *Client) ListUserRepos(ctx context.Context) ([]Repository, error) {
	var repos []Repository
	_, err := c.do(ctx, http.MethodGet, baseURL+"/user/repos?affiliation=owner,collaborator,organization_member&sort=updated&per_page=100", nil, &repos)
	return repos, err
}

// ListInstallationRepos lists repositories accessible to a GitHub App installation.
func (c *Client) ListInstallationRepos(ctx context.Context) ([]Repository, error) {
	var resp InstallationReposResponse
	if _, err := c.do(ctx, http.MethodGet, baseURL+"/installation/repositories?per_page=100", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Repositories, nil
}

// GetContents fetches a file from the Contents API (base64-decoded).
func (c *Client) GetContents(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s", baseURL, owner, repo, path)
	if ref != "" {
		u += "?ref=" + ref
	}

	var file ContentFile
	if _, err := c.do(ctx, http.MethodGet, u, nil, &file); err != nil {
		return nil, err
	}
	if file.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported content encoding: %s", file.Encoding)
	}
	return base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
}

// GetRawContents fetches a file using the raw media type.
func (c *Client) GetRawContents(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s", baseURL, owner, repo, path)
	if ref != "" {
		u += "?ref=" + ref
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(respBody))
	}
	return io.ReadAll(resp.Body)
}

// ListContents lists directory entries from the Contents API.
func (c *Client) ListContents(ctx context.Context, owner, repo, path, ref string) ([]ContentItem, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s", baseURL, owner, repo, path)
	if ref != "" {
		u += "?ref=" + ref
	}
	var items []ContentItem
	_, err := c.do(ctx, http.MethodGet, u, nil, &items)
	return items, err
}

// FileExists returns nil if the file exists, or an error otherwise.
func (c *Client) FileExists(ctx context.Context, owner, repo, path string) error {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s", baseURL, owner, repo, path)
	_, err := c.do(ctx, http.MethodGet, u, nil, nil)
	return err
}

// ---------------------------------------------------------------------------
// Commit operations
// ---------------------------------------------------------------------------

// GetCommit fetches commit metadata for a given ref (SHA, branch, tag).
func (c *Client) GetCommit(ctx context.Context, owner, repo, ref string) (*Commit, error) {
	var commit Commit
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/commits/%s", baseURL, owner, repo, ref), nil, &commit); err != nil {
		return nil, err
	}
	return &commit, nil
}

// SetCommitStatus creates a commit status.
func (c *Client) SetCommitStatus(ctx context.Context, owner, repo, sha string, req CommitStatusRequest) error {
	u := fmt.Sprintf("%s/repos/%s/%s/statuses/%s", baseURL, owner, repo, sha)
	_, err := c.do(ctx, http.MethodPost, u, req, nil)
	return err
}

// ---------------------------------------------------------------------------
// Pull Request operations
// ---------------------------------------------------------------------------

// GetPRsByCommit lists pull requests associated with a commit SHA.
func (c *Client) GetPRsByCommit(ctx context.Context, owner, repo, sha string) ([]PullRequest, error) {
	var prs []PullRequest
	_, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/commits/%s/pulls", baseURL, owner, repo, sha), nil, &prs)
	return prs, err
}

// GetPRsByBranch lists open pull requests for a head branch.
func (c *Client) GetPRsByBranch(ctx context.Context, owner, repo, branch string) ([]PullRequest, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&head=%s:%s&per_page=1", baseURL, owner, repo, owner, branch)
	var prs []PullRequest
	_, err := c.do(ctx, http.MethodGet, u, nil, &prs)
	return prs, err
}

// GetPRFiles lists the files changed in a pull request.
func (c *Client) GetPRFiles(ctx context.Context, owner, repo string, prNumber int) ([]PRFile, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?per_page=100", baseURL, owner, repo, prNumber)
	var files []PRFile
	_, err := c.do(ctx, http.MethodGet, u, nil, &files)
	return files, err
}

// CreateIssueComment posts a comment on an issue or pull request and returns
// the new comment's ID.
func (c *Client) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (int64, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", baseURL, owner, repo, number)
	var resp struct {
		ID int64 `json:"id"`
	}
	if _, err := c.do(ctx, http.MethodPost, u, map[string]string{"body": body}, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// UpdateIssueComment updates an existing issue/PR comment.
func (c *Client) UpdateIssueComment(ctx context.Context, owner, repo string, commentID int64, body string) error {
	u := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", baseURL, owner, repo, commentID)
	_, err := c.do(ctx, http.MethodPatch, u, map[string]string{"body": body}, nil)
	return err
}

// CreatePRReview posts a pull request review and returns the review ID.
func (c *Client) CreatePRReview(ctx context.Context, owner, repo string, prNumber int, review any) (int64, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", baseURL, owner, repo, prNumber)
	var resp struct {
		ID int64 `json:"id"`
	}
	if _, err := c.do(ctx, http.MethodPost, u, review, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// CreatePR creates a pull request.
func (c *Client) CreatePR(ctx context.Context, owner, repo string, req CreatePRRequest) (int, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/pulls", baseURL, owner, repo)
	return c.do(ctx, http.MethodPost, u, req, nil)
}

// ---------------------------------------------------------------------------
// Check Run operations
// ---------------------------------------------------------------------------

// CreateCheckRun creates a new GitHub check run and returns its ID.
func (c *Client) CreateCheckRun(ctx context.Context, owner, repo string, req CheckRunRequest) (int64, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/check-runs", baseURL, owner, repo)
	var resp struct {
		ID int64 `json:"id"`
	}
	if _, err := c.do(ctx, http.MethodPost, u, req, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// UpdateCheckRun updates an existing GitHub check run.
func (c *Client) UpdateCheckRun(ctx context.Context, owner, repo string, checkRunID int64, req CheckRunRequest) error {
	u := fmt.Sprintf("%s/repos/%s/%s/check-runs/%d", baseURL, owner, repo, checkRunID)
	_, err := c.do(ctx, http.MethodPatch, u, req, nil)
	return err
}

// ---------------------------------------------------------------------------
// Git operations (refs, file creation)
// ---------------------------------------------------------------------------

// GetRef returns the SHA of a Git reference (e.g. "heads/main").
func (c *Client) GetRef(ctx context.Context, owner, repo, ref string) (string, error) {
	var r GitRef
	if _, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/git/ref/%s", baseURL, owner, repo, ref), nil, &r); err != nil {
		return "", err
	}
	return r.Object.SHA, nil
}

// CreateRef creates a new Git reference. ref should be a full ref like "refs/heads/branch-name".
func (c *Client) CreateRef(ctx context.Context, owner, repo, ref, sha string) (int, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/git/refs", baseURL, owner, repo)
	return c.do(ctx, http.MethodPost, u, map[string]string{"ref": ref, "sha": sha}, nil)
}

// CreateOrUpdateFile creates or updates a file via the Contents API.
func (c *Client) CreateOrUpdateFile(ctx context.Context, owner, repo, path string, req CreateFileRequest) (int, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s", baseURL, owner, repo, path)
	return c.do(ctx, http.MethodPut, u, req, nil)
}
