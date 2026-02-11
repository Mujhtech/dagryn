// Package handlers provides HTTP request handlers for the Dagryn API.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ErrorResponse represents an error response.
// @Description Error response returned by the API
type ErrorResponse struct {
	Error   string `json:"error" example:"bad_request"`
	Message string `json:"message" example:"Invalid request body"`
	Details any    `json:"details,omitempty"`
}

// SuccessResponse represents a generic success response.
// @Description Generic success response
type SuccessResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// PaginationMeta contains pagination metadata.
// @Description Pagination metadata
type PaginationMeta struct {
	Page       int   `json:"page" example:"1"`
	PerPage    int   `json:"per_page" example:"20"`
	Total      int64 `json:"total" example:"100"`
	TotalPages int   `json:"total_pages" example:"5"`
}

// PaginatedResponse wraps paginated data.
// @Description Paginated response wrapper
type PaginatedResponse struct {
	Data interface{}    `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

// HealthResponse represents the health check response.
// @Description Health check response
type HealthResponse struct {
	Status    string    `json:"status" example:"healthy"`
	Version   string    `json:"version" example:"1.0.0"`
	Timestamp time.Time `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// ReadyResponse represents the readiness check response.
// @Description Readiness check response
type ReadyResponse struct {
	Status   string            `json:"status" example:"ready"`
	Database string            `json:"database" example:"connected"`
	Checks   map[string]string `json:"checks,omitempty"`
}

// --- User types ---

// UserResponse represents a user in API responses.
// @Description User information
type UserResponse struct {
	ID        uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string    `json:"email" example:"user@example.com"`
	Name      string    `json:"name" example:"John Doe"`
	AvatarURL string    `json:"avatar_url,omitempty" example:"https://avatars.githubusercontent.com/u/1234567"`
	Provider  string    `json:"provider" example:"github"`
	CreatedAt time.Time `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

// UpdateUserRequest represents a request to update a user.
// @Description Update user request
type UpdateUserRequest struct {
	Name      *string `json:"name,omitempty" example:"John Doe"`
	AvatarURL *string `json:"avatar_url,omitempty" example:"https://example.com/avatar.png"`
}

// --- Team types ---

// TeamResponse represents a team in API responses.
// @Description Team information
type TeamResponse struct {
	ID          uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name        string    `json:"name" example:"Engineering"`
	Slug        string    `json:"slug" example:"engineering"`
	Description string    `json:"description,omitempty" example:"Engineering team"`
	AvatarURL   string    `json:"avatar_url,omitempty" example:"https://example.com/team-avatar.png"`
	MemberCount int       `json:"member_count" example:"5"`
	CreatedAt   time.Time `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt   time.Time `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

// CreateTeamRequest represents a request to create a team.
// @Description Create team request
type CreateTeamRequest struct {
	Name        string `json:"name" binding:"required" example:"Engineering"`
	Slug        string `json:"slug,omitempty" example:"engineering"`
	Description string `json:"description,omitempty" example:"Engineering team"`
}

// UpdateTeamRequest represents a request to update a team.
// @Description Update team request
type UpdateTeamRequest struct {
	Name        *string `json:"name,omitempty" example:"Engineering"`
	Description *string `json:"description,omitempty" example:"Engineering team"`
	AvatarURL   *string `json:"avatar_url,omitempty" example:"https://example.com/team-avatar.png"`
}

// TeamMemberResponse represents a team member in API responses.
// @Description Team member information
type TeamMemberResponse struct {
	User     UserResponse `json:"user"`
	Role     string       `json:"role" example:"admin"`
	JoinedAt time.Time    `json:"joined_at" example:"2024-01-15T10:30:00Z"`
}

// AddTeamMemberRequest represents a request to add a team member.
// @Description Add team member request
type AddTeamMemberRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Role   string    `json:"role" binding:"required" example:"member"`
}

// UpdateMemberRoleRequest represents a request to update a member's role.
// @Description Update member role request
type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required" example:"admin"`
}

// --- Project types ---

// ProjectResponse represents a project in API responses.
// @Description Project information
type ProjectResponse struct {
	ID          uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	TeamID      uuid.UUID  `json:"team_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name        string     `json:"name" example:"api-service"`
	Slug        string     `json:"slug" example:"api-service"`
	Description string     `json:"description,omitempty" example:"Main API service"`
	RepoURL     string     `json:"repo_url,omitempty" example:"https://github.com/org/repo"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty" example:"2024-01-15T10:30:00Z"`
	ConfigPath  string     `json:"config_path" example:"dagryn.toml"`
	Visibility  string     `json:"visibility" example:"private"`
	MemberCount int        `json:"member_count" example:"3"`
	CreatedAt   time.Time  `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt   time.Time  `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

// CreateProjectRequest represents a request to create a project.
// @Description Create project request
type CreateProjectRequest struct {
	TeamID               uuid.UUID  `json:"team_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name                 string     `json:"name" binding:"required" example:"api-service"`
	Slug                 string     `json:"slug,omitempty" example:"api-service"`
	Description          string     `json:"description,omitempty" example:"Main API service"`
	RepoURL              string     `json:"repo_url,omitempty" example:"https://github.com/org/repo"`
	GitHubInstallationID *uuid.UUID `json:"github_installation_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	GitHubRepoID         *int64     `json:"github_repo_id,omitempty" example:"123456789"`
	Visibility           string     `json:"visibility,omitempty" example:"private"`
}

// UpdateProjectRequest represents a request to update a project.
// @Description Update project request
type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty" example:"api-service"`
	Description *string `json:"description,omitempty" example:"Main API service"`
	RepoURL     *string `json:"repo_url,omitempty" example:"https://github.com/org/repo"`
	Visibility  *string `json:"visibility,omitempty" example:"private"`
}

// ConnectGitHubRequest represents a request to connect a project to GitHub.
// @Description Connect project to GitHub request
type ConnectGitHubRequest struct {
	GitHubInstallationID uuid.UUID `json:"github_installation_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	GitHubRepoID         int64     `json:"github_repo_id" binding:"required" example:"123456789"`
	RepoURL              string    `json:"repo_url" binding:"required" example:"https://github.com/org/repo"`
}

// GitHubWorkflowTranslateRequest represents a request to translate GitHub workflows.
// @Description Request payload for GitHub Actions workflow translation
type GitHubWorkflowTranslateRequest struct {
	RepoFullName         string     `json:"repo_full_name" binding:"required" example:"owner/repo"`
	GitHubInstallationID *uuid.UUID `json:"github_installation_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// GitHubWorkflowSummary is a minimal summary of a translated workflow file.
// @Description Summary of a GitHub Actions workflow file
type GitHubWorkflowSummary struct {
	File      string `json:"file" example:"ci.yml"`
	Name      string `json:"name" example:"CI"`
	TaskCount int    `json:"task_count" example:"3"`
}

// GitHubWorkflowTranslateResponse contains the translated Dagryn TOML snippet.
// @Description Translation result for GitHub Actions workflows
type GitHubWorkflowTranslateResponse struct {
	Detected  bool                    `json:"detected" example:"true"`
	Workflows []GitHubWorkflowSummary `json:"workflows"`
	Plugins   map[string]string       `json:"plugins"`
	TasksToml string                  `json:"tasks_toml"`
}

// ProjectMemberResponse represents a project member in API responses.
// @Description Project member information
type ProjectMemberResponse struct {
	User     UserResponse `json:"user"`
	Role     string       `json:"role" example:"admin"`
	JoinedAt time.Time    `json:"joined_at" example:"2024-01-15T10:30:00Z"`
}

// AddProjectMemberRequest represents a request to add a project member.
// @Description Add project member request
type AddProjectMemberRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Role   string    `json:"role" binding:"required" example:"member"`
}

// --- API Key types ---

// APIKeyResponse represents an API key in API responses (without the secret).
// @Description API key information (secret only shown on creation)
type APIKeyResponse struct {
	ID         uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name       string     `json:"name" example:"CI/CD Key"`
	Prefix     string     `json:"prefix" example:"dg_live_abc123"`
	Scope      string     `json:"scope" example:"user"`
	ProjectID  *uuid.UUID `json:"project_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" example:"2024-01-15T10:30:00Z"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" example:"2024-04-15T10:30:00Z"`
	CreatedAt  time.Time  `json:"created_at" example:"2024-01-15T10:30:00Z"`
}

// APIKeyCreatedResponse represents a newly created API key (includes the secret).
// @Description API key creation response (includes secret, shown only once)
type APIKeyCreatedResponse struct {
	APIKeyResponse
	Key string `json:"key" example:"dg_live_abc123xyz789..."`
}

// CreateAPIKeyRequest represents a request to create an API key.
// @Description Create API key request
type CreateAPIKeyRequest struct {
	Name      string `json:"name" binding:"required" example:"CI/CD Key"`
	ExpiresIn string `json:"expires_in,omitempty" example:"90d"`
}

// --- Invitation types ---

// InvitationResponse represents an invitation in API responses.
// @Description Invitation information
type InvitationResponse struct {
	ID          uuid.UUID    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email       string       `json:"email" example:"newuser@example.com"`
	Role        string       `json:"role" example:"member"`
	TeamID      *uuid.UUID   `json:"team_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	TeamName    string       `json:"team_name,omitempty" example:"Engineering"`
	ProjectID   *uuid.UUID   `json:"project_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ProjectName string       `json:"project_name,omitempty" example:"api-service"`
	InvitedBy   UserResponse `json:"invited_by"`
	Status      string       `json:"status" example:"pending"`
	ExpiresAt   time.Time    `json:"expires_at" example:"2024-01-22T10:30:00Z"`
	CreatedAt   time.Time    `json:"created_at" example:"2024-01-15T10:30:00Z"`
	// AcceptToken is set only when listing pending invitations for the current user, so the client can call accept/decline.
	AcceptToken string `json:"accept_token,omitempty"`
}

// CreateInvitationRequest represents a request to create an invitation.
// @Description Create invitation request
type CreateInvitationRequest struct {
	Email string `json:"email" binding:"required" example:"newuser@example.com"`
	Role  string `json:"role" binding:"required" example:"member"`
}

// --- Run types ---

// RunResponse represents a workflow run in API responses.
// @Description Workflow run information
type RunResponse struct {
	ID                uuid.UUID     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	ProjectID         uuid.UUID     `json:"project_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	WorkflowName      string        `json:"workflow_name" example:"build"`
	Status            string        `json:"status" example:"success"`
	TriggerSource     string        `json:"trigger_source" example:"cli"`
	TriggerRef        string        `json:"trigger_ref,omitempty" example:"refs/heads/main"`
	CommitSHA         string        `json:"commit_sha,omitempty" example:"abc123def456"`
	PRTitle           string        `json:"pr_title,omitempty" example:"Fix bug in authentication"`
	PRNumber          *int          `json:"pr_number,omitempty" example:"123"`
	CommitMessage     string        `json:"commit_message,omitempty" example:"fix: resolve authentication issue"`
	CommitAuthorName  string        `json:"commit_author_name,omitempty" example:"John Doe"`
	CommitAuthorEmail string        `json:"commit_author_email,omitempty" example:"john@example.com"`
	TriggeredByUser   *UserResponse `json:"triggered_by_user,omitempty"` // User who triggered (for local/API runs)
	StartedAt         *time.Time    `json:"started_at,omitempty" example:"2024-01-15T10:30:00Z"`
	FinishedAt        *time.Time    `json:"finished_at,omitempty" example:"2024-01-15T10:35:00Z"`
	Duration          *int64        `json:"duration_ms,omitempty" example:"300000"`
	TaskCount         int           `json:"task_count" example:"5"`
	CreatedAt         time.Time     `json:"created_at" example:"2024-01-15T10:30:00Z"`
}

// TaskResultResponse represents a task execution result in API responses.
// @Description Task execution result
type TaskResultResponse struct {
	ID         uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RunID      uuid.UUID  `json:"run_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	TaskName   string     `json:"task_name" example:"build"`
	Status     string     `json:"status" example:"success"`
	ExitCode   *int       `json:"exit_code,omitempty" example:"0"`
	StartedAt  *time.Time `json:"started_at,omitempty" example:"2024-01-15T10:30:00Z"`
	FinishedAt *time.Time `json:"finished_at,omitempty" example:"2024-01-15T10:31:00Z"`
	Duration   *int64     `json:"duration_ms,omitempty" example:"60000"`
	CacheHit   bool       `json:"cache_hit" example:"false"`
	CacheKey   string     `json:"cache_key,omitempty" example:"build-abc123"`
}

// TriggerRunRequest represents a request to trigger a workflow run.
// @Description Trigger run request
type TriggerRunRequest struct {
	Targets   []string `json:"targets,omitempty" example:"[\"build\",\"test\"]"`
	GitBranch string   `json:"git_branch,omitempty" example:"main"`
	GitCommit string   `json:"git_commit,omitempty" example:"abc123def456"`
	Force     bool     `json:"force,omitempty" example:"false"`
	// SyncOnly when true creates a run record for status tracking without triggering remote execution.
	// Use this when the CLI is executing locally and only needs to sync status to the server.
	SyncOnly bool `json:"sync_only,omitempty" example:"false"`
}

// TriggerRunResponse represents the response after triggering a run.
// @Description Trigger run response
type TriggerRunResponse struct {
	RunID     uuid.UUID `json:"run_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status    string    `json:"status" example:"pending"`
	Message   string    `json:"message" example:"Run queued successfully"`
	StreamURL string    `json:"stream_url,omitempty" example:"/api/v1/projects/{projectID}/runs/{runID}/events"`
	LogsURL   string    `json:"logs_url,omitempty" example:"/api/v1/projects/{projectID}/runs/{runID}/logs"`
}

// CancelRunResponse represents the response after cancelling a run.
// @Description Cancel run response
type CancelRunResponse struct {
	RunID       uuid.UUID `json:"run_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status      string    `json:"status" example:"cancelled"`
	Message     string    `json:"message" example:"Run cancelled successfully"`
	CancelledAt time.Time `json:"cancelled_at" example:"2024-01-15T10:35:00Z"`
}

// RunDetailResponse represents detailed run information including tasks.
// @Description Detailed run information
type RunDetailResponse struct {
	RunResponse
	Tasks          []TaskResultResponse `json:"tasks"`
	CompletedTasks int                  `json:"completed_tasks" example:"3"`
	FailedTasks    int                  `json:"failed_tasks" example:"1"`
	CacheHits      int                  `json:"cache_hits" example:"2"`
	ErrorMessage   string               `json:"error_message,omitempty" example:"Task 'test' failed with exit code 1"`
}

// RunDashboardChartPointResponse is a daily aggregate for run dashboards.
// @Description Daily chart point for project runs
type RunDashboardChartPointResponse struct {
	Date       string `json:"date" example:"2026-02-11"`
	Success    int    `json:"success" example:"12"`
	Failed     int    `json:"failed" example:"2"`
	DurationMs int64  `json:"duration_ms" example:"95000"`
}

// RunDashboardUserFacetResponse is a user facet for run filters.
// @Description Filterable user value for run dashboards
type RunDashboardUserFacetResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string `json:"name" example:"Jane Doe"`
	AvatarURL string `json:"avatar_url,omitempty" example:"https://example.com/avatar.png"`
}

// RunDashboardSummaryResponse contains non-paginated data for charts/facets.
// @Description Stable run dashboard data independent of pagination
type RunDashboardSummaryResponse struct {
	Chart        []RunDashboardChartPointResponse `json:"chart"`
	Users        []RunDashboardUserFacetResponse  `json:"users"`
	Workflows    []string                         `json:"workflows"`
	Branches     []string                         `json:"branches"`
	StatusCounts map[string]int                   `json:"status_counts"`
}

// --- Artifact types ---

// ArtifactResponse represents a stored artifact.
// @Description Artifact metadata
type ArtifactResponse struct {
	ID           uuid.UUID       `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	ProjectID    uuid.UUID       `json:"project_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RunID        uuid.UUID       `json:"run_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	TaskName     string          `json:"task_name,omitempty" example:"build"`
	Name         string          `json:"name" example:"dist/app"`
	FileName     string          `json:"file_name" example:"app"`
	ContentType  string          `json:"content_type" example:"application/octet-stream"`
	SizeBytes    int64           `json:"size_bytes" example:"1024"`
	StorageKey   string          `json:"storage_key,omitempty" example:"artifacts/.../app"`
	DigestSHA256 string          `json:"digest_sha256,omitempty" example:"deadbeef"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// --- Run Status Update Types ---

// UpdateRunStatusRequest represents a request to update run status.
// @Description Update run status request
type UpdateRunStatusRequest struct {
	Status       string  `json:"status" example:"running"`
	TotalTasks   *int    `json:"total_tasks,omitempty" example:"5"`
	ErrorMessage *string `json:"error_message,omitempty" example:"Build failed"`
}

// UpdateTaskStatusRequest represents a request to update task status.
// @Description Update task status request
type UpdateTaskStatusRequest struct {
	Status     string `json:"status" example:"running"`
	ExitCode   *int   `json:"exit_code,omitempty" example:"0"`
	DurationMs *int64 `json:"duration_ms,omitempty" example:"5000"`
	CacheHit   bool   `json:"cache_hit,omitempty" example:"false"`
	CacheKey   string `json:"cache_key,omitempty" example:"task-abc123"`
	Output     string `json:"output,omitempty" example:"Build successful"`
	Error      string `json:"error,omitempty" example:"Command failed"`
}

// CreateTaskRequest represents a request to create a task result.
// @Description Create task request
type CreateTaskRequest struct {
	TaskName string `json:"task_name" example:"build"`
}

// AppendLogRequest represents a request to append a log line.
// @Description Append log request
type AppendLogRequest struct {
	TaskName string `json:"task_name,omitempty" example:"build"`
	Stream   string `json:"stream" example:"stdout"`
	Line     string `json:"line" example:"Compiling main.go..."`
	LineNum  int    `json:"line_num,omitempty" example:"1"`
}

// BatchLogRequest represents a request to append multiple log lines.
// @Description Batch log request
type BatchLogRequest struct {
	Logs []AppendLogRequest `json:"logs"`
}

// HeartbeatResponse represents the response from a heartbeat request.
// @Description Heartbeat response
type HeartbeatResponse struct {
	RunID           uuid.UUID `json:"run_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status          string    `json:"status" example:"running"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at" example:"2024-01-15T10:30:00Z"`
}

// LogResponse represents a single log entry.
// @Description Log entry response
type LogResponse struct {
	ID        int64     `json:"id" example:"1"`
	TaskName  string    `json:"task_name" example:"build"`
	Stream    string    `json:"stream" example:"stdout"`
	LineNum   int       `json:"line_num" example:"1"`
	Content   string    `json:"content" example:"Compiling main.go..."`
	CreatedAt time.Time `json:"created_at" example:"2024-01-15T10:30:00Z"`
}

// --- Workflow types ---

// SyncWorkflowRequest represents a request to sync workflow from CLI.
// @Description Sync workflow request
type SyncWorkflowRequest struct {
	Name       string                 `json:"name" binding:"required" example:"default"`
	IsDefault  bool                   `json:"is_default" example:"true"`
	ConfigHash string                 `json:"config_hash" example:"sha256:abc123..."`
	RawConfig  string                 `json:"raw_config,omitempty"`
	Tasks      []SyncWorkflowTaskData `json:"tasks" binding:"required"`
}

// SyncWorkflowFromTomlRequest represents a request to sync workflow from a TOML snippet.
// @Description Sync workflow request from raw TOML
type SyncWorkflowFromTomlRequest struct {
	RawConfig string `json:"raw_config" binding:"required"`
}

// SyncWorkflowTaskData represents task data in a sync request.
// @Description Workflow task data
type SyncWorkflowTaskData struct {
	Name           string            `json:"name" binding:"required" example:"build"`
	Command        string            `json:"command" binding:"required" example:"go build ./..."`
	Needs          []string          `json:"needs,omitempty" example:"[\"lint\"]"`
	Inputs         []string          `json:"inputs,omitempty" example:"[\"**/*.go\"]"`
	Outputs        []string          `json:"outputs,omitempty" example:"[\"bin/app\"]"`
	Plugins        []string          `json:"plugins,omitempty" example:"[\"setup-go@v4\"]"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty" example:"300"`
	Workdir        *string           `json:"workdir,omitempty" example:"./cmd"`
	Env            map[string]string `json:"env,omitempty"`
}

// WorkflowResponse represents a workflow in API responses.
// @Description Workflow information
type WorkflowResponse struct {
	ID        uuid.UUID              `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string                 `json:"name" example:"default"`
	Version   int                    `json:"version" example:"1"`
	IsDefault bool                   `json:"is_default" example:"true"`
	SyncedAt  time.Time              `json:"synced_at" example:"2024-01-15T10:30:00Z"`
	Tasks     []WorkflowTaskResponse `json:"tasks"`
}

// WorkflowTaskResponse represents a task in a workflow.
// @Description Workflow task information
type WorkflowTaskResponse struct {
	Name           string            `json:"name" example:"build"`
	Command        string            `json:"command" example:"go build ./..."`
	Needs          []string          `json:"needs,omitempty"`
	Inputs         []string          `json:"inputs,omitempty"`
	Outputs        []string          `json:"outputs,omitempty"`
	Plugins        []string          `json:"plugins,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	Workdir        *string           `json:"workdir,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
}

// SyncWorkflowResponse represents the response after syncing a workflow.
// @Description Sync workflow response
type SyncWorkflowResponse struct {
	WorkflowID uuid.UUID `json:"workflow_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name       string    `json:"name" example:"default"`
	TaskCount  int       `json:"task_count" example:"5"`
	Changed    bool      `json:"changed" example:"true"`
	Message    string    `json:"message" example:"Workflow synced successfully"`
}

// --- Helper functions ---

// ParseJSON parses a JSON request body.
func ParseJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// ParseUUID parses a UUID from a string.
func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// GetPageParams extracts pagination parameters from the request.
func GetPageParams(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := parseInt(p); err == nil && n > 0 {
			page = n
		}
	}

	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if n, err := parseInt(pp); err == nil && n > 0 && n <= 100 {
			perPage = n
		}
	}

	return page, perPage
}

func parseInt(s string) (int, error) {
	var n int
	err := json.Unmarshal([]byte(s), &n)
	return n, err
}

// CalculateTotalPages calculates the total number of pages.
func CalculateTotalPages(total int64, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	pages := int(total) / perPage
	if int(total)%perPage > 0 {
		pages++
	}
	return pages
}
