// Package client provides an HTTP client for the Dagryn API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Client is the Dagryn API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	creds      *Credentials
	credStore  *CredentialsStore
	refreshMu  sync.Mutex
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
		Timeout: 5 * time.Minute,
	}
}

// New creates a new API client.
//
// Per-request timeouts should be controlled via context deadlines (which callers
// already provide). The http.Client.Timeout is intentionally NOT set so that
// long-running uploads (cache, artifacts) aren't killed by a blanket timeout.
// Connection-level timeouts on the Transport protect against hung dials and idle
// connection reuse issues.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:9000"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.Timeout,      // Dial timeout (connect phase only)
			KeepAlive: 30 * time.Second, // TCP keep-alive probes
		}).DialContext,
		IdleConnTimeout:       60 * time.Second, // Close idle connections before server does
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: cfg.Timeout, // Time to wait for response headers
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}

	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Transport: transport,
			// No blanket Timeout — per-request context deadlines control timeouts.
			// This prevents long uploads from being killed prematurely.
		},
	}
}

// SetCredentials sets the authentication credentials.
func (c *Client) SetCredentials(creds *Credentials) {
	c.creds = creds
}

// SetCredentialsStore sets the credentials store for persisting refreshed tokens.
func (c *Client) SetCredentialsStore(store *CredentialsStore) {
	c.credStore = store
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
	// SyncOnly when true creates a run record for status tracking without triggering remote execution.
	SyncOnly bool   `json:"sync_only,omitempty"`
	HostOS   string `json:"host_os,omitempty"`
	HostArch string `json:"host_arch,omitempty"`
	HostName string `json:"host_name,omitempty"`
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
// Uses doRequestInternal to avoid triggering the auto-refresh interceptor.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	body := map[string]string{"refresh_token": refreshToken}
	resp, err := c.doRequestInternal(ctx, "POST", "/api/v1/auth/refresh", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// --- Heartbeat ---

// HeartbeatResponse represents the response from a heartbeat request.
type HeartbeatResponse struct {
	RunID           uuid.UUID `json:"run_id"`
	Status          string    `json:"status"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at"`
}

// Heartbeat sends a heartbeat for a run and returns the current run status.
func (c *Client) Heartbeat(ctx context.Context, projectID, runID uuid.UUID) (*HeartbeatResponse, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/heartbeat", projectID, runID)
	resp, err := c.doRequest(ctx, "POST", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Data HeartbeatResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse heartbeat response: %w", err)
	}

	return &result.Data, nil
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}

	return nil
}

// UpdateTaskStatusRequest represents a request to update task status.
type UpdateTaskStatusRequest struct {
	Status     string     `json:"status"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	DurationMs *int64     `json:"duration_ms,omitempty"`
	CacheHit   bool       `json:"cache_hit,omitempty"`
	CacheKey   string     `json:"cache_key,omitempty"`
	Output     string     `json:"output,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// UpdateTaskStatus updates the status of a task.
func (c *Client) UpdateTaskStatus(ctx context.Context, projectID, runID uuid.UUID, taskName, status string, exitCode *int, durationMs *int64, cacheHit bool, cacheKey string, startedAt, finishedAt *time.Time) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/tasks/%s", projectID, runID, taskName)
	req := UpdateTaskStatusRequest{
		Status:     status,
		ExitCode:   exitCode,
		DurationMs: durationMs,
		CacheHit:   cacheHit,
		CacheKey:   cacheKey,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}
	resp, err := c.doRequest(ctx, "PATCH", path, req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	Group          string            `json:"group,omitempty"`
	Condition      string            `json:"condition,omitempty"`
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result SyncWorkflowResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// --- Cache API Methods ---

// CheckCache checks if a cache entry exists for the given task/key.
// Returns true on 200, false on 404.
func (c *Client) CheckCache(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string) (bool, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/cache/%s/%s", projectID, taskName, cacheKey)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, c.parseError(resp)
	}
}

// UploadCache stores cache content for the given task/key.
// The body is sent as raw bytes (tar.gz archive).
func (c *Client) UploadCache(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string, body io.Reader, size int64) error {
	// Proactively refresh token before building the request — body is consumed once.
	if err := c.ensureValidToken(ctx); err != nil {
		_ = err // non-fatal, proceed with current token
	}

	path := fmt.Sprintf("/api/v1/projects/%s/cache/%s/%s", projectID, taskName, cacheKey)
	fullURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	if size > 0 {
		req.ContentLength = size
	}
	if c.creds != nil && c.creds.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return WrapNetworkError("upload cache", fullURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}
	return nil
}

// DownloadCache retrieves cache content for the given task/key.
// Returns the response body as a stream (caller must close).
func (c *Client) DownloadCache(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/cache/%s/%s/download", projectID, taskName, cacheKey)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	default:
		defer func() { _ = resp.Body.Close() }()
		return nil, c.parseError(resp)
	}
}

// ArtifactUploadOption configures optional fields for artifact uploads.
type ArtifactUploadOption func(*artifactUploadOpts)

type artifactUploadOpts struct {
	contentType string
	metadata    string // JSON string
}

// WithArtifactContentType sets an explicit content type for the artifact.
func WithArtifactContentType(ct string) ArtifactUploadOption {
	return func(o *artifactUploadOpts) { o.contentType = ct }
}

// WithArtifactMetadata sets JSON metadata for the artifact.
func WithArtifactMetadata(meta string) ArtifactUploadOption {
	return func(o *artifactUploadOpts) { o.metadata = meta }
}

// UploadArtifact uploads an artifact file for a run via multipart form.
func (c *Client) UploadArtifact(ctx context.Context, projectID, runID uuid.UUID, taskName, name, fileName string, reader io.Reader, opts ...ArtifactUploadOption) error {
	// Proactively refresh token before building the multipart body — reader is consumed once.
	if err := c.ensureValidToken(ctx); err != nil {
		_ = err // non-fatal, proceed with current token
	}

	var options artifactUploadOpts
	for _, o := range opts {
		o(&options)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("task_name", taskName); err != nil {
		return fmt.Errorf("failed to write task_name field: %w", err)
	}
	if err := writer.WriteField("name", name); err != nil {
		return fmt.Errorf("failed to write name field: %w", err)
	}
	if options.contentType != "" {
		if err := writer.WriteField("content_type", options.contentType); err != nil {
			return fmt.Errorf("failed to write content_type field: %w", err)
		}
	}
	if options.metadata != "" {
		if err := writer.WriteField("metadata", options.metadata); err != nil {
			return fmt.Errorf("failed to write metadata field: %w", err)
		}
	}

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, reader); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/artifacts", projectID, runID)
	fullURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.creds != nil && c.creds.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return WrapNetworkError("upload artifact", fullURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return c.parseError(resp)
	}
	return nil
}

// --- Billing ---

// BillingPlanResponse represents a billing plan.
type BillingPlanResponse struct {
	ID                    string `json:"id"`
	Slug                  string `json:"slug"`
	DisplayName           string `json:"display_name"`
	Description           string `json:"description"`
	PriceCents            int    `json:"price_cents"`
	BillingPeriod         string `json:"billing_period"`
	MaxCacheBytes         *int64 `json:"max_cache_bytes,omitempty"`
	MaxStorageBytes       *int64 `json:"max_storage_bytes,omitempty"`
	MaxBandwidth          *int64 `json:"max_bandwidth_bytes,omitempty"`
	MaxProjects           *int   `json:"max_projects,omitempty"`
	MaxTeamMembers        *int   `json:"max_team_members,omitempty"`
	MaxConcurrent         *int   `json:"max_concurrent_runs,omitempty"`
	MaxAIAnalysesPerMonth *int   `json:"max_ai_analyses_per_month,omitempty"`
	AIEnabled             bool   `json:"ai_enabled"`
	AISuggestionsEnabled  bool   `json:"ai_suggestions_enabled"`
}

// BillingResourceUsage holds live resource consumption for a billing account.
type BillingResourceUsage struct {
	CacheBytesUsed        int64 `json:"cache_bytes_used"`
	ArtifactBytesUsed     int64 `json:"artifact_bytes_used"`
	TotalStorageBytesUsed int64 `json:"total_storage_bytes_used"`
	BandwidthBytesUsed    int64 `json:"bandwidth_bytes_used"`
	ProjectsUsed          int   `json:"projects_used"`
	TeamMembersUsed       int   `json:"team_members_used"`
	ConcurrentRuns        int   `json:"concurrent_runs"`
	AIAnalysesUsed        int   `json:"ai_analyses_used"`
}

// BillingOverviewResponse represents the billing overview.
type BillingOverviewResponse struct {
	BaseResponse
	Data struct {
		Account       json.RawMessage       `json:"account"`
		Subscription  *json.RawMessage      `json:"subscription,omitempty"`
		Plan          *BillingPlanResponse  `json:"plan,omitempty"`
		Usage         map[string]int64      `json:"usage,omitempty"`
		ResourceUsage *BillingResourceUsage `json:"resource_usage,omitempty"`
	} `json:"data"`
}

// GetBillingOverview returns the billing overview for the current user.
func (c *Client) GetBillingOverview(ctx context.Context) (*BillingOverviewResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/billing/overview", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result BillingOverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode billing overview: %w", err)
	}
	return &result, nil
}

// GetBillingPortalURL creates a Stripe portal session and returns the URL.
func (c *Client) GetBillingPortalURL(ctx context.Context, returnURL string) (string, error) {
	body := map[string]string{"return_url": returnURL}
	resp, err := c.doRequest(ctx, "POST", "/api/v1/billing/portal", body)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", c.parseError(resp)
	}

	var result struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode portal URL: %w", err)
	}
	return result.Data.URL, nil
}

// ListBillingPlans returns all available billing plans.
func (c *Client) ListBillingPlans(ctx context.Context) ([]BillingPlanResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/billing/plans", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result struct {
		Data []BillingPlanResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode billing plans: %w", err)
	}
	return result.Data, nil
}

// CreateCheckoutSession creates a Stripe Checkout session and returns the URL.
func (c *Client) CreateCheckoutSession(ctx context.Context, planSlug, successURL, cancelURL string) (string, error) {
	body := map[string]string{
		"plan_slug":   planSlug,
		"success_url": successURL,
		"cancel_url":  cancelURL,
	}
	resp, err := c.doRequest(ctx, "POST", "/api/v1/billing/checkout", body)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", c.parseError(resp)
	}

	var result struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode checkout URL: %w", err)
	}
	return result.Data.URL, nil
}

// --- Internal Methods ---

// doRequest wraps doRequestInternal with automatic token refresh.
// It proactively refreshes expired tokens before the request and retries once on 401.
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	// Proactively refresh token if expired
	if err := c.ensureValidToken(ctx); err != nil {
		// Non-fatal: proceed with existing token, server will reject if truly expired
		_ = err
	}

	resp, err := c.doRequestInternal(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	// If 401 and we have a refresh token, try refreshing and retry once
	if resp.StatusCode == http.StatusUnauthorized && c.creds != nil && c.creds.RefreshToken != "" {
		_ = resp.Body.Close()

		if refreshErr := c.ensureValidToken(ctx); refreshErr != nil {
			return nil, refreshErr
		}
		return c.doRequestInternal(ctx, method, path, body)
	}

	return resp, nil
}

// doRequestInternal performs the raw HTTP request without token refresh logic.
func (c *Client) doRequestInternal(ctx context.Context, method, path string, body any) (*http.Response, error) {
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

// ensureValidToken refreshes the access token if it has expired.
// It serializes concurrent refresh attempts so only one refresh occurs.
func (c *Client) ensureValidToken(ctx context.Context) error {
	if c.creds == nil || c.creds.RefreshToken == "" {
		return nil
	}

	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	// Double-check after acquiring lock — another goroutine may have refreshed
	if !c.creds.IsExpired() {
		return nil
	}

	body := map[string]string{"refresh_token": c.creds.RefreshToken}
	resp, err := c.doRequestInternal(ctx, "POST", "/api/v1/auth/refresh", body)
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed: %w", c.parseError(resp))
	}

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse refresh response: %w", err)
	}

	c.creds.AccessToken = result.Data.AccessToken
	c.creds.RefreshToken = result.Data.RefreshToken
	c.creds.ExpiresAt = result.Data.ExpiresAt

	// Persist refreshed credentials to disk if store is available
	if c.credStore != nil {
		if err := c.credStore.Save(c.creds); err != nil {
			// Non-fatal — token is refreshed in memory even if disk save fails
			_ = err
		}
	}

	return nil
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
