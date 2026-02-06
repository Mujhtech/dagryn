// Package client provides an HTTP client for the Dagryn API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Client is the Dagryn API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	creds      *Credentials
}

// Config holds client configuration.
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() Config {
	return Config{
		BaseURL: "http://localhost:9000",
		Timeout: 30 * time.Second,
	}
}

// New creates a new API client.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:9000"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// SetCredentials sets the authentication credentials.
func (c *Client) SetCredentials(creds *Credentials) {
	c.creds = creds
}

// --- Request/Response Types ---

type BaseResponse struct {
	Message string `json:"message"`
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error string `json:"error"`
	BaseResponse
}

// UserResponse represents a user.
type UserResponse struct {
	BaseResponse
	Data UserData `json:"data"`
}

type UserData struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"created_at"`
}

// TokenResponse represents authentication tokens.
type TokenResponse struct {
	BaseResponse
	Data struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		TokenType    string    `json:"token_type"`
		ExpiresIn    int64     `json:"expires_in"`
		ExpiresAt    time.Time `json:"expires_at"`
		User         UserData  `json:"user"`
	}
}

// DeviceCodeResponse represents a device code response.
type DeviceCodeResponse struct {
	BaseResponse
	Data struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
}

// DeviceCodePendingResponse represents a pending device code.
type DeviceCodePendingResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// TriggerRunRequest represents a request to trigger a run.
type TriggerRunRequest struct {
	Targets   []string `json:"targets,omitempty"`
	GitBranch string   `json:"git_branch,omitempty"`
	GitCommit string   `json:"git_commit,omitempty"`
}

// TriggerRunResponse represents a triggered run.
type TriggerRunResponse struct {
	Data struct {
		RunID     uuid.UUID `json:"run_id"`
		Status    string    `json:"status"`
		Message   string    `json:"message"`
		StreamURL string    `json:"stream_url,omitempty"`
		LogsURL   string    `json:"logs_url,omitempty"`
	} `json:"data"`
	BaseResponse
}

// RunResponse represents a run.
type RunResponse struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	WorkflowName  string     `json:"workflow_name"`
	Status        string     `json:"status"`
	TriggerSource string     `json:"trigger_source"`
	TriggerRef    string     `json:"trigger_ref,omitempty"`
	CommitSHA     string     `json:"commit_sha,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Duration      *int64     `json:"duration_ms,omitempty"`
	TaskCount     int        `json:"task_count"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ProjectResponse represents a project.
type ProjectResponse struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	RepoURL     string    `json:"repo_url,omitempty"`
	Visibility  string    `json:"visibility"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// --- API Methods ---

// RequestDeviceCode initiates the device code flow.
func (c *Client) RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/auth/device", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// PollDeviceCode polls for device code authorization.
// Returns (tokens, pending, error) where pending is true if still waiting.
func (c *Client) PollDeviceCode(ctx context.Context, deviceCode string) (*TokenResponse, bool, error) {
	body := map[string]string{"device_code": deviceCode}
	resp, err := c.doRequest(ctx, "POST", "/api/v1/auth/device/poll", body)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	// 428 Precondition Required = authorization pending
	if resp.StatusCode == http.StatusPreconditionRequired {
		return nil, true, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false, c.parseError(resp)
	}

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, false, nil
}

// RefreshToken refreshes the access token.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	body := map[string]string{"refresh_token": refreshToken}
	resp, err := c.doRequest(ctx, "POST", "/api/v1/auth/refresh", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// Logout logs out the current user.
func (c *Client) Logout(ctx context.Context, revokeAll bool) error {
	body := map[string]bool{"revoke_all": revokeAll}
	resp, err := c.doRequest(ctx, "POST", "/api/v1/auth/logout", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// GetCurrentUser returns the current authenticated user.
func (c *Client) GetCurrentUser(ctx context.Context) (*UserResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/users/me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// ListProjects returns all projects the user has access to.
func (c *Client) ListProjects(ctx context.Context) ([]ProjectResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/projects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Data struct {
			Data []ProjectResponse `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data.Data, nil
}

// TriggerRun triggers a new run for a project.
func (c *Client) TriggerRun(ctx context.Context, projectID uuid.UUID, req TriggerRunRequest) (*TriggerRunResponse, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/runs", projectID)
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result TriggerRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// GetRun returns a run by ID.
func (c *Client) GetRun(ctx context.Context, projectID, runID uuid.UUID) (*RunResponse, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s", projectID, runID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result RunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// CancelRun cancels a running or pending run.
func (c *Client) CancelRun(ctx context.Context, projectID, runID uuid.UUID) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/cancel", projectID, runID)
	resp, err := c.doRequest(ctx, "POST", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// --- Run Status Update Methods ---

// UpdateRunStatusRequest represents a request to update run status.
type UpdateRunStatusRequest struct {
	Status       string  `json:"status"`
	TotalTasks   *int    `json:"total_tasks,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
}

// UpdateRunStatus updates the status of a run.
func (c *Client) UpdateRunStatus(ctx context.Context, projectID, runID uuid.UUID, status string, totalTasks *int, errorMsg *string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/status", projectID, runID)
	req := UpdateRunStatusRequest{
		Status:       status,
		TotalTasks:   totalTasks,
		ErrorMessage: errorMsg,
	}
	resp, err := c.doRequest(ctx, "PATCH", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// CreateTaskRequest represents a request to create a task.
type CreateTaskRequest struct {
	TaskName string `json:"task_name"`
}

// CreateTask creates a new task result for a run.
func (c *Client) CreateTask(ctx context.Context, projectID, runID uuid.UUID, taskName string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/tasks", projectID, runID)
	req := CreateTaskRequest{TaskName: taskName}
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}

	return nil
}

// UpdateTaskStatusRequest represents a request to update task status.
type UpdateTaskStatusRequest struct {
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMs *int64 `json:"duration_ms,omitempty"`
	CacheHit   bool   `json:"cache_hit,omitempty"`
	CacheKey   string `json:"cache_key,omitempty"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// UpdateTaskStatus updates the status of a task.
func (c *Client) UpdateTaskStatus(ctx context.Context, projectID, runID uuid.UUID, taskName, status string, exitCode *int, durationMs *int64, cacheHit bool, cacheKey string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/tasks/%s", projectID, runID, taskName)
	req := UpdateTaskStatusRequest{
		Status:     status,
		ExitCode:   exitCode,
		DurationMs: durationMs,
		CacheHit:   cacheHit,
		CacheKey:   cacheKey,
	}
	resp, err := c.doRequest(ctx, "PATCH", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// LogEntry represents a single log line.
type LogEntry struct {
	TaskName string `json:"task_name,omitempty"`
	Stream   string `json:"stream"`
	Line     string `json:"line"`
	LineNum  int    `json:"line_num,omitempty"`
}

// BatchLogRequest represents a batch of log entries.
type BatchLogRequest struct {
	Logs []LogEntry `json:"logs"`
}

// AppendLogs appends log lines to a run.
func (c *Client) AppendLogs(ctx context.Context, projectID, runID uuid.UUID, logs []LogEntry) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/logs", projectID, runID)
	req := BatchLogRequest{Logs: logs}
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// --- Team Types ---

// TeamResponse represents a team.
type TeamResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	MemberCount int       `json:"member_count"`
	Role        string    `json:"role,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// --- Project Creation Types ---

// CreateProjectRequest represents a request to create a project.
type CreateProjectRequest struct {
	Name        string    `json:"name"`
	Slug        string    `json:"slug,omitempty"`
	Description string    `json:"description,omitempty"`
	TeamID      uuid.UUID `json:"team_id,omitempty"`
	Visibility  string    `json:"visibility,omitempty"` // private, team, public
	RepoURL     string    `json:"repo_url,omitempty"`
}

// CreateProjectResponse represents a created project.
type CreateProjectResponse struct {
	BaseResponse
	Data ProjectResponse `json:"data"`
}

// --- Team API Methods ---

// ListTeams returns all teams the user belongs to.
func (c *Client) ListTeams(ctx context.Context) ([]TeamResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/teams", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Data struct {
			Data []TeamResponse `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data.Data, nil
}

// --- Project API Methods ---

// GetProject returns a project by ID.
func (c *Client) GetProject(ctx context.Context, projectID uuid.UUID) (*ProjectResponse, error) {
	path := fmt.Sprintf("/api/v1/projects/%s", projectID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Data ProjectResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Data, nil
}

// CreateProject creates a new project.
func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (*ProjectResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/projects", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result CreateProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Data, nil
}

// --- Workflow Sync Types ---

// SyncWorkflowRequest represents a request to sync a workflow.
type SyncWorkflowRequest struct {
	Name       string                 `json:"name"`
	IsDefault  bool                   `json:"is_default"`
	ConfigHash string                 `json:"config_hash,omitempty"`
	RawConfig  string                 `json:"raw_config,omitempty"`
	Tasks      []SyncWorkflowTaskData `json:"tasks"`
}

// SyncWorkflowTaskData represents task data in a sync request.
type SyncWorkflowTaskData struct {
	Name           string            `json:"name"`
	Command        string            `json:"command"`
	Needs          []string          `json:"needs,omitempty"`
	Inputs         []string          `json:"inputs,omitempty"`
	Outputs        []string          `json:"outputs,omitempty"`
	Plugins        []string          `json:"plugins,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	Workdir        *string           `json:"workdir,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
}

// SyncWorkflowResponse represents the response after syncing a workflow.
type SyncWorkflowResponse struct {
	BaseResponse
	Data struct {
		WorkflowID uuid.UUID `json:"workflow_id"`
		Name       string    `json:"name"`
		TaskCount  int       `json:"task_count"`
		Changed    bool      `json:"changed"`
		Message    string    `json:"message"`
	} `json:"data"`
}

// SyncWorkflow syncs a workflow to the server.
func (c *Client) SyncWorkflow(ctx context.Context, projectID uuid.UUID, req SyncWorkflowRequest) (*SyncWorkflowResponse, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/workflows/sync", projectID)
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result SyncWorkflowResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// --- Internal Methods ---

func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Add authentication if available
	if c.creds != nil && c.creds.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Wrap network errors with better context
		return nil, WrapNetworkError("request", fullURL, err)
	}

	return resp, nil
}

func (c *Client) parseError(resp *http.Response) error {
	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		// Could not parse error response, return generic API error
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("request failed with status %d", resp.StatusCode),
		}
	}

	// Return structured API error
	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    errResp.Message,
		ErrorCode:  errResp.Error,
	}
}
