// API client for Dagryn dashboard

// Use environment variable or default to current origin for production
const API_BASE = import.meta.env.VITE_API_URL || "/api/v1";

// Auth types
export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  created_at: string;
}

export interface AuthProvider {
  id: string;
  name: string;
  display_name?: string;
  auth_url: string;
  enabled?: boolean;
  icon?: string;
}

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

export interface DeviceCodeResponse {
  device_code: string;
  user_code: string;
  verification_uri: string;
  expires_in: number;
  interval: number;
}

// Team types
export interface Team {
  id: string;
  name: string;
  slug: string;
  description?: string;
  avatar_url?: string;
  member_count: number;
  created_at: string;
  updated_at: string;
}

export interface TeamMember {
  user: User;
  role: string;
  joined_at: string;
}

export interface Invitation {
  id: string;
  email: string;
  role: string;
  team_id?: string;
  team_name?: string;
  project_id?: string;
  project_name?: string;
  invited_by?: User;
  status: string;
  expires_at: string;
  created_at: string;
  /** Set only when listing pending invitations for current user; use for accept/decline. */
  accept_token?: string;
}

// GitHub provider (Import from GitHub)
export interface GitHubRepo {
  id: number;
  full_name: string;
  clone_url: string;
  default_branch: string;
  private: boolean;
}

export interface GitHubAppInstallation {
  id: string;
  installation_id: number;
  account_login: string;
  account_type: string;
}

// Project types
export interface Project {
  id: string;
  team_id: string | null;
  name: string;
  slug: string;
  description?: string;
  visibility: string;
  repo_url?: string;
  github_installation_id?: string;
  github_repo_id?: number;
  member_count: number;
  created_at: string;
  updated_at: string;
}

// API key types
export interface APIKey {
  id: string;
  name: string;
  prefix: string;
  scope: string;
  project_id?: string;
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
}

export interface APIKeyCreated extends APIKey {
  key: string;
}

// Run types
export interface Run {
  id: string;
  project_id: string;
  workflow_name: string;
  status: RunStatus;
  trigger_source: string;
  trigger_ref?: string;
  commit_sha?: string;
  pr_title?: string;
  pr_number?: number;
  commit_message?: string;
  commit_author_name?: string;
  commit_author_email?: string;
  triggered_by_user?: {
    id: string;
    email: string;
    name: string;
    avatar_url?: string;
  };
  started_at?: string;
  finished_at?: string;
  duration_ms?: number;
  task_count: number;
  created_at: string;
}

export interface RunDetail extends Run {
  tasks: TaskResult[];
  completed_tasks: number;
  failed_tasks: number;
  cache_hits: number;
  error_message?: string;
  client_disconnected?: boolean;
  last_heartbeat_at?: string;
}

export interface TaskResult {
  id: string;
  run_id: string;
  task_name: string;
  status: TaskStatus;
  exit_code?: number;
  started_at?: string;
  finished_at?: string;
  duration_ms?: number;
  cache_hit: boolean;
  cache_key?: string;
}

export type RunStatus =
  | "pending"
  | "running"
  | "success"
  | "failed"
  | "cancelled";
export type TaskStatus =
  | "pending"
  | "running"
  | "success"
  | "failed"
  | "cached"
  | "skipped"
  | "cancelled";

// Trigger run types
export interface TriggerRunRequest {
  targets?: string[];
  git_branch?: string;
  git_commit?: string;
  force?: boolean;
}

export interface TriggerRunResponse {
  run_id: string;
  status: string;
  message: string;
  stream_url?: string;
  logs_url?: string;
}

// Log types
export interface LogEntry {
  id: number;
  task_name: string;
  stream: "stdout" | "stderr";
  line_num: number;
  content: string;
  created_at: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  meta: {
    page: number;
    per_page: number;
    total: number;
    total_pages: number;
  };
}

// Workflow types
export interface Workflow {
  id: string;
  name: string;
  version: number;
  is_default: boolean;
  synced_at: string;
  tasks: WorkflowTask[];
}

export interface WorkflowTask {
  name: string;
  command: string;
  needs?: string[];
  inputs?: string[];
  outputs?: string[];
  plugins?: string[];
  timeout_seconds?: number;
  workdir?: string;
  env?: Record<string, string>;
}

// Cache types
export interface CacheStats {
  total_entries: number;
  total_size_bytes: number;
  hit_count: number;
  quota_used_pct: number;
  top_tasks: TaskCacheStats[];
}

export interface TaskCacheStats {
  task_name: string;
  entries: number;
  size_bytes: number;
  total_hits: number;
}

export interface CacheAnalytics {
  days: DailyUsage[];
  total_hits: number;
  total_misses: number;
  hit_rate: number;
  total_bytes_uploaded: number;
  total_bytes_downloaded: number;
}

export interface DailyUsage {
  date: string;
  bytes_uploaded: number;
  bytes_downloaded: number;
  cache_hits: number;
  cache_misses: number;
  hit_rate: number;
}

// API Error
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

class ApiClient {
  private token: string | null = null;

  constructor() {
    // Load token from localStorage on init
    if (typeof window !== "undefined") {
      this.token = localStorage.getItem("access_token");
    }
  }

  setToken(token: string | null) {
    this.token = token;
    if (typeof window !== "undefined") {
      if (token) {
        localStorage.setItem("access_token", token);
      } else {
        localStorage.removeItem("access_token");
      }
    }
  }

  getToken(): string | null {
    return this.token;
  }

  clearToken() {
    this.setToken(null);
    if (typeof window !== "undefined") {
      localStorage.removeItem("refresh_token");
    }
  }

  private async fetch<T>(
    path: string,
    options: RequestInit = {},
  ): Promise<{
    data: T;
    message?: string;
  }> {
    const headers: HeadersInit = {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    };

    if (this.token) {
      (headers as Record<string, string>)["Authorization"] =
        `Bearer ${this.token}`;
    }

    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ error: "unknown", message: "Request failed" }));
      const message =
        response.status >= 400 && response.status < 500 && error.error
          ? error.error
          : error.message || `HTTP ${response.status}`;
      throw new ApiError(response.status, error.error || "unknown", message);
    }

    // Handle empty responses
    const text = await response.text();
    if (!text) return { data: {} as T };
    return JSON.parse(text) as {
      data: T;
      message?: string;
    };
  }

  // Auth
  async getAuthProviders(): Promise<{ data: AuthProvider[] }> {
    return this.fetch("/auth/providers");
  }

  async startOAuth(provider: string) {
    return this.fetch<{ url: string }>(`/auth/${provider}`);
  }

  async oauthCallback(
    provider: string,
    code: string,
    state?: string,
  ): Promise<TokenResponse> {
    const response = await this.fetch<TokenResponse>(
      `/auth/${provider}/callback`,
      {
        method: "POST",
        body: JSON.stringify({ code, state }),
      },
    );
    const { data } = response;
    this.setToken(data.access_token);
    if (typeof window !== "undefined" && data.refresh_token) {
      localStorage.setItem("refresh_token", data.refresh_token);
    }
    return data;
  }

  async refreshToken(): Promise<TokenResponse> {
    const refreshToken =
      typeof window !== "undefined"
        ? localStorage.getItem("refresh_token")
        : null;
    if (!refreshToken) {
      throw new ApiError(401, "no_refresh_token", "No refresh token available");
    }
    const response = await this.fetch<TokenResponse>("/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    this.setToken(response.data.access_token);
    if (typeof window !== "undefined" && response.data.refresh_token) {
      localStorage.setItem("refresh_token", response.data.refresh_token);
    }
    return response.data;
  }

  async logout(): Promise<void> {
    try {
      await this.fetch("/auth/logout", { method: "POST" });
    } finally {
      this.clearToken();
    }
  }

  // Device code flow (for CLI auth in browser)
  async requestDeviceCode() {
    return this.fetch<DeviceCodeResponse>("/auth/device", { method: "POST" });
  }

  async authorizeDevice(userCode: string) {
    return this.fetch("/auth/device/authorize", {
      method: "POST",
      body: JSON.stringify({ user_code: userCode }),
    });
  }

  async denyDevice(userCode: string) {
    return this.fetch("/auth/device/deny", {
      method: "POST",
      body: JSON.stringify({ user_code: userCode }),
    });
  }

  // User
  async getCurrentUser() {
    return this.fetch<User>("/users/me");
  }

  async updateCurrentUser(data: Partial<Pick<User, "name">>) {
    return this.fetch("/users/me", {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  // Teams
  async listTeams() {
    return this.fetch<PaginatedResponse<Team>>("/teams");
  }

  async getTeam(id: string) {
    return this.fetch<Team>(`/teams/${id}`);
  }

  async createTeam(data: {
    name: string;
    slug?: string;
    description?: string;
  }) {
    return this.fetch<Team>("/teams", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateTeam(id: string, data: { name?: string; description?: string }) {
    return this.fetch<Team>(`/teams/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  async deleteTeam(id: string) {
    return this.fetch(`/teams/${id}`, { method: "DELETE" });
  }

  async listTeamMembers(teamId: string) {
    return this.fetch<TeamMember[]>(`/teams/${teamId}/members`);
  }

  async addTeamMember(teamId: string, data: { user_id: string; role: string }) {
    return this.fetch<TeamMember>(`/teams/${teamId}/members`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async removeTeamMember(teamId: string, userId: string) {
    return this.fetch(`/teams/${teamId}/members/${userId}`, {
      method: "DELETE",
    });
  }

  async updateTeamMemberRole(teamId: string, userId: string, role: string) {
    return this.fetch<TeamMember>(`/teams/${teamId}/members/${userId}/role`, {
      method: "PATCH",
      body: JSON.stringify({ role }),
    });
  }

  async listTeamInvitations(teamId: string) {
    return this.fetch<Invitation[]>(`/teams/${teamId}/invitations`);
  }

  async createTeamInvitation(
    teamId: string,
    data: { email: string; role: string },
  ) {
    return this.fetch<Invitation>(`/teams/${teamId}/invitations`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async revokeTeamInvitation(teamId: string, invitationId: string) {
    return this.fetch(`/teams/${teamId}/invitations/${invitationId}`, {
      method: "DELETE",
    });
  }

  // Invitations (current user)
  async listPendingInvitations() {
    return this.fetch<Invitation[]>("/invitations");
  }

  async acceptInvitation(token: string) {
    return this.fetch(`/invitations/${token}/accept`, {
      method: "POST",
    });
  }

  async declineInvitation(token: string) {
    return this.fetch(`/invitations/${token}/decline`, {
      method: "POST",
    });
  }

  // Projects
  async listProjects() {
    return this.fetch<PaginatedResponse<Project>>("/projects");
  }

  async getProject(id: string) {
    return this.fetch<Project>(`/projects/${id}`);
  }

  /** List repos the current user has access to (requires GitHub login with repo scope). */
  async listGitHubRepos() {
    return this.fetch<GitHubRepo[]>("/providers/github/repos");
  }

  // GitHub App (installations + repos via app)
  async listGitHubAppInstallations() {
    return this.fetch<GitHubAppInstallation[]>(
      "/providers/github/app/installations",
    );
  }

  async listGitHubAppRepos(installationId: string) {
    return this.fetch<GitHubRepo[]>(
      `/providers/github/app/installations/${installationId}/repos`,
    );
  }

  async createProject(data: {
    name: string;
    slug: string;
    description?: string;
    visibility?: string;
    team_id?: string | null;
    repo_url?: string;
    github_installation_id?: string;
    github_repo_id?: number;
  }) {
    const body: Record<string, unknown> = {
      name: data.name,
      slug: data.slug,
      description: data.description ?? "",
      visibility: data.visibility ?? "private",
    };
    if (data.team_id != null && data.team_id !== "")
      body.team_id = data.team_id;
    if (data.repo_url != null && data.repo_url !== "") {
      body.repo_url = data.repo_url;
    }
    if (
      data.github_installation_id != null &&
      data.github_installation_id !== ""
    ) {
      body.github_installation_id = data.github_installation_id;
    }
    if (data.github_repo_id != null) {
      body.github_repo_id = data.github_repo_id;
    }
    return this.fetch<Project>("/projects", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  // Runs
  async listRuns(projectId: string, page = 1, perPage = 20) {
    return this.fetch<PaginatedResponse<Run>>(
      `/projects/${projectId}/runs?page=${page}&per_page=${perPage}`,
    );
  }

  async getRun(projectId: string, runId: string) {
    return this.fetch<Run>(`/projects/${projectId}/runs/${runId}`);
  }

  async getRunDetail(projectId: string, runId: string) {
    return this.fetch<RunDetail>(`/projects/${projectId}/runs/${runId}/detail`);
  }

  async getRunTasks(projectId: string, runId: string) {
    return this.fetch<TaskResult[]>(
      `/projects/${projectId}/runs/${runId}/tasks`,
    );
  }

  async cancelRun(projectId: string, runId: string) {
    await this.fetch(`/projects/${projectId}/runs/${runId}/cancel`, {
      method: "POST",
    });
  }

  async triggerRun(projectId: string, request?: TriggerRunRequest) {
    return this.fetch<TriggerRunResponse>(`/projects/${projectId}/runs`, {
      method: "POST",
      body: JSON.stringify(request || {}),
    });
  }

  // Logs
  async getRunLogs(
    projectId: string,
    runId: string,
    options?: {
      page?: number;
      perPage?: number;
      task?: string;
      afterId?: number;
    },
  ) {
    const params = new URLSearchParams();
    if (options?.page) params.set("page", String(options.page));
    if (options?.perPage) params.set("per_page", String(options.perPage));
    if (options?.task) params.set("task", options.task);
    if (options?.afterId) params.set("after_id", String(options.afterId));

    const query = params.toString() ? `?${params.toString()}` : "";
    return this.fetch<PaginatedResponse<LogEntry>>(
      `/projects/${projectId}/runs/${runId}/logs/history${query}`,
    );
  }

  async getRunLogsSince(
    projectId: string,
    runId: string,
    afterId: number,
    limit = 1000,
  ) {
    return this.fetch<LogEntry[]>(
      `/projects/${projectId}/runs/${runId}/logs/history?after_id=${afterId}&per_page=${limit}`,
    );
  }

  // Projects management
  async updateProject(
    id: string,
    data: {
      name?: string;
      description?: string;
      visibility?: string;
    },
  ) {
    return this.fetch<Project>(`/projects/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  async deleteProject(id: string) {
    return this.fetch(`/projects/${id}`, { method: "DELETE" });
  }

  async connectProjectToGitHub(
    projectId: string,
    data: {
      github_installation_id: string;
      github_repo_id: number;
      repo_url: string;
    },
  ) {
    return this.fetch<Project>(`/projects/${projectId}/connect-github`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Project API keys
  async listProjectAPIKeys(projectId: string) {
    return this.fetch<APIKey[]>(`/projects/${projectId}/api-keys`);
  }

  async createProjectAPIKey(
    projectId: string,
    data: { name: string; expires_in?: string },
  ) {
    return this.fetch<APIKeyCreated>(`/projects/${projectId}/api-keys`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async revokeProjectAPIKey(projectId: string, keyId: string) {
    await this.fetch(`/projects/${projectId}/api-keys/${keyId}`, {
      method: "DELETE",
    });
  }

  // Workflows
  async listProjectWorkflows(projectId: string) {
    return this.fetch<Workflow[]>(`/projects/${projectId}/workflows`);
  }

  async getRunWorkflow(projectId: string, runId: string) {
    return this.fetch<Workflow>(
      `/projects/${projectId}/runs/${runId}/workflow`,
    );
  }

  // Cache
  async getCacheStats(projectId: string) {
    return this.fetch<CacheStats>(`/projects/${projectId}/cache/stats`);
  }

  async getCacheAnalytics(projectId: string, days = 30) {
    return this.fetch<CacheAnalytics>(
      `/projects/${projectId}/cache/analytics?days=${days}`,
    );
  }
}

export const api = new ApiClient();
