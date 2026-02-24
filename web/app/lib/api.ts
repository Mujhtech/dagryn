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
  language: string;
}

export interface GitHubAppInstallation {
  id: string;
  installation_id: number;
  account_login: string;
  account_type: string;
}

export interface GitHubWorkflowSummary {
  file: string;
  name: string;
  task_count: number;
}

export interface GitHubWorkflowTranslateResponse {
  detected: boolean;
  workflows: GitHubWorkflowSummary[];
  plugins: Record<string, string>;
  tasks_toml: string;
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
  default_branch?: string;
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
  host_os?: string;
  host_arch?: string;
  host_name?: string;
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

export interface RunDashboardChartPoint {
  date: string;
  success: number;
  failed: number;
  duration_ms: number;
}

export interface RunDashboardUserFacet {
  id: string;
  name: string;
  avatar_url?: string;
}

export interface RunDashboardSummary {
  chart: RunDashboardChartPoint[];
  users: RunDashboardUserFacet[];
  workflows: string[];
  branches: string[];
  status_counts: Record<string, number>;
}

export interface Artifact {
  id: string;
  project_id: string;
  run_id: string;
  task_name?: string;
  name: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  storage_key?: string;
  digest_sha256?: string;
  expires_at?: string;
  created_at: string;
  metadata?: Record<string, unknown>;
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
export interface WorkflowTrigger {
  push?: { branches: string[] };
  pull_request?: { branches?: string[]; types?: string[] };
}

export interface WorkflowCacheConfig {
  enabled: boolean;
  dir?: string;
  remote_enabled: boolean;
  remote_cloud: boolean;
}

export interface WorkflowAIConfig {
  enabled: boolean;
  mode?: string;
  provider?: string;
  model?: string;
  backend_mode?: string;
}

export interface WorkflowContainerConfig {
  enabled: boolean;
  image?: string;
  memory_limit?: string;
  cpu_limit?: string;
  network?: string;
}

export interface Workflow {
  id: string;
  name: string;
  version: number;
  is_default: boolean;
  synced_at: string;
  tasks: WorkflowTask[];
  trigger?: WorkflowTrigger;
  cache?: WorkflowCacheConfig;
  ai?: WorkflowAIConfig;
  container?: WorkflowContainerConfig;
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
  group?: string;
  condition?: string;
}

export interface SyncWorkflowResponse {
  workflow_id: string;
  name: string;
  task_count: number;
  changed: boolean;
  message: string;
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

// Plugin types
export interface PluginInputDef {
  description: string;
  required: boolean;
  default?: string;
}

export interface PluginOutputDef {
  description: string;
}

export interface PluginStep {
  name: string;
  command: string;
  if?: string;
}

export interface PluginInfo {
  name: string;
  source: string;
  version: string;
  description: string;
  type: string;
  author?: string;
  license?: string;
  installed: boolean;
  homepage?: string;
  readme?: string;
  license_text?: string;
  inputs?: Record<string, PluginInputDef>;
  outputs?: Record<string, PluginOutputDef>;
  steps?: PluginStep[];
  cleanup?: PluginStep[];
}

// Plugin Registry types
export interface PluginPublisher {
  id: string;
  name: string;
  display_name: string;
  avatar_url?: string;
  website?: string;
  verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface RegistryPluginSummary {
  id: string;
  publisher_id: string;
  name: string;
  description: string;
  type: string;
  license?: string;
  homepage?: string;
  repository_url?: string;
  readme?: string;
  total_downloads: number;
  weekly_downloads: number;
  stars: number;
  featured: boolean;
  deprecated: boolean;
  latest_version: string;
  publisher_name: string;
  publisher_verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface PluginVersionInfo {
  id: string;
  plugin_id: string;
  version: string;
  manifest_json: Record<string, unknown>;
  checksum_sha256?: string;
  downloads: number;
  yanked: boolean;
  release_notes?: string;
  published_at: string;
}

export interface RegistryPluginDetail {
  plugin: RegistryPluginSummary;
  versions: PluginVersionInfo[];
}

export interface PluginSearchResult {
  plugins: RegistryPluginSummary[];
  total: number;
  page: number;
  per_page: number;
}

export interface PluginAnalytics {
  total_downloads: number;
  weekly_downloads: number;
  daily_stats: PluginDailyDownload[];
}

export interface PluginDailyDownload {
  date: string;
  downloads: number;
}

// AI Analysis types
export type AIAnalysisStatus =
  | "pending"
  | "in_progress"
  | "success"
  | "failed"
  | "quota_exceeded"
  | "superseded";

export interface AIAnalysis {
  id: string;
  run_id: string;
  project_id: string;
  status: AIAnalysisStatus;
  provider?: string;
  model?: string;
  summary?: string;
  root_cause?: string;
  confidence?: number;
  evidence_json?: string;
  error_message?: string;
  provider_mode?: string;
  prompt_version?: string;
  prompt_hash?: string;
  response_hash?: string;
  raw_response_blob_key?: string;
  dedup_key?: string;
  created_at: string;
  updated_at: string;
}

export type AIPublicationStatus = "pending" | "sent" | "updated" | "failed";
export type AIPublicationDestination =
  | "github_pr_comment"
  | "github_check"
  | "github_pr_review";

export interface AIPublication {
  id: string;
  analysis_id: string;
  run_id: string;
  destination: AIPublicationDestination;
  external_id?: string;
  status: AIPublicationStatus;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export type AISuggestionStatus =
  | "pending"
  | "posted"
  | "accepted"
  | "dismissed"
  | "failed_validation";

export interface AISuggestion {
  id: string;
  analysis_id: string;
  run_id: string;
  file_path: string;
  start_line: number;
  end_line: number;
  original_code: string;
  suggested_code: string;
  explanation: string;
  confidence: number;
  status: AISuggestionStatus;
  github_comment_id?: string;
  risk_score?: number;
  failure_reason?: string;
  created_at: string;
  updated_at: string;
}

export interface AIAnalysisDetail {
  analysis: AIAnalysis;
  publications: AIPublication[] | null;
  suggestions: AISuggestion[] | null;
}

export interface AIAnalysesList {
  analyses: AIAnalysis[];
  total: number;
  limit: number;
  offset: number;
}

export interface AISuggestionsResponse {
  suggestions: AISuggestion[];
  analysis_id: string;
}

// Dashboard Overview types
export interface DashboardOverview {
  projects: DashboardProject[];
  recent_runs: DashboardRun[];
}

export interface DashboardProject {
  id: string;
  name: string;
  slug: string;
  visibility: string;
  repo_url?: string;
  member_count: number;
  updated_at: string;
  created_at: string;
  chart: RunDashboardChartPoint[];
  latest_run?: DashboardRun;
  total_runs_7d: number;
  success_runs_7d: number;
  failed_runs_7d: number;
  avg_duration_ms: number;
  top_branch?: string;
}

export interface DashboardRun {
  id: string;
  project_id: string;
  project_name: string;
  workflow_name: string;
  status: RunStatus;
  trigger_ref?: string;
  commit_sha?: string;
  commit_author_name?: string;
  triggered_by_user?: {
    id: string;
    email: string;
    name: string;
    avatar_url?: string;
  };
  duration_ms?: number;
  created_at: string;
}

// Feature entry returned by the API
export interface FeatureEntry {
  feature: string;
  label: string;
  enabled: boolean;
}

// License types
export interface LicenseStatus {
  mode: "cloud" | "self_hosted";
  edition: "community" | "pro" | "enterprise" | "cloud";
  licensed: boolean;
  customer?: string;
  seats: number;
  features: FeatureEntry[];
  limits: {
    projects: { current: number; limit: number | null };
    team_members: { current: number; limit: number | null };
    concurrent_runs: { current: number; limit: number | null };
  };
  expires_at?: string;
  days_remaining?: number;
  grace_period: boolean;
  expiring: boolean;
}

// Capabilities types
export interface CapabilitiesNavItem {
  key: string;
  label: string;
  enabled: boolean;
}

export interface CapabilitiesResponse {
  mode: "cloud" | "self_hosted";
  edition: "community" | "pro" | "enterprise" | "cloud";
  features: FeatureEntry[];
  nav: CapabilitiesNavItem[];
}

// Audit log types
export interface AuditLogEntry {
  id: string;
  team_id: string;
  actor_id?: string;
  actor_email: string;
  actor_type: string;
  action: string;
  category: string;
  resource_type: string;
  resource_id?: string;
  project_id?: string;
  description: string;
  metadata?: Record<string, unknown>;
  ip_address?: string;
  user_agent?: string;
  entry_hash: string;
  prev_hash?: string;
  sequence_num: number;
  created_at: string;
}

export interface AuditLogListResult {
  data: AuditLogEntry[];
  next_cursor?: string;
  has_next: boolean;
}

export interface AuditRetentionPolicy {
  team_id: string;
  retention_days: number;
  updated_at?: string;
}

export interface AuditChainVerifyResult {
  valid: boolean;
  total_entries: number;
  broken_entries?: string[];
}

export interface AuditWebhook {
  id: string;
  team_id: string;
  url: string;
  description: string;
  event_filter: string[];
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuditWebhookCreated extends AuditWebhook {
  secret: string;
}

// Analytics types
export interface AnalyticsOverview {
  runs: RunAnalyticsOverview;
  cache: CacheAnalyticsOverview;
  artifacts: ArtifactAnalyticsOverview;
  bandwidth: BandwidthAnalyticsOverview;
  ai: AIAnalyticsOverview;
  audit_log: AuditLogAnalyticsOverview;
  projects: ProjectActivitySummary[];
}

export interface RunAnalyticsOverview {
  total_runs: number;
  success_runs: number;
  failed_runs: number;
  cancelled_runs: number;
  success_rate: number;
  avg_duration_ms: number;
  chart: {
    date: string;
    success: number;
    failed: number;
    cancelled: number;
    avg_duration_ms: number;
  }[];
}

export interface CacheAnalyticsOverview {
  total_entries: number;
  total_size_bytes: number;
  total_hits: number;
  total_misses: number;
  hit_rate: number;
  total_bytes_uploaded: number;
  total_bytes_downloaded: number;
  chart: {
    date: string;
    cache_hits: number;
    cache_misses: number;
    bytes_uploaded: number;
    bytes_downloaded: number;
  }[];
}

export interface ArtifactAnalyticsOverview {
  total_artifacts: number;
  total_size_bytes: number;
  chart: { date: string; count: number; size_bytes: number }[];
}

export interface BandwidthAnalyticsOverview {
  total_bytes: number;
  upload_bytes: number;
  download_bytes: number;
  chart: { date: string; upload_bytes: number; download_bytes: number }[];
}

export interface AIAnalyticsOverview {
  total_analyses: number;
  success_analyses: number;
  failed_analyses: number;
  total_suggestions: number;
  applied_suggestions: number;
  chart: { date: string; analyses: number; suggestions: number }[];
}

export interface AuditLogAnalyticsOverview {
  total_events: number;
  top_actions: { action: string; count: number }[];
  top_actors: { actor_email: string; count: number }[];
  chart: { date: string; events: number }[];
}

export interface ProjectActivitySummary {
  project_id: string;
  project_name: string;
  total_runs: number;
  success_rate: number;
  cache_size_bytes: number;
  artifact_size_bytes: number;
  bandwidth_bytes: number;
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
  private refreshPromise: Promise<TokenResponse> | null = null;

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

  private async fetchInternal<T>(
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

  private async fetch<T>(
    path: string,
    options: RequestInit = {},
  ): Promise<{
    data: T;
    message?: string;
  }> {
    try {
      return await this.fetchInternal<T>(path, options);
    } catch (error) {
      if (
        error instanceof ApiError &&
        error.status === 401 &&
        this.hasRefreshToken()
      ) {
        await this.refreshTokenInternal();
        return this.fetchInternal<T>(path, options);
      }
      throw error;
    }
  }

  private async fetchBlobInternal(
    path: string,
  ): Promise<{ blob: Blob; filename?: string; contentType?: string }> {
    const headers: HeadersInit = {};
    if (this.token) {
      (headers as Record<string, string>)["Authorization"] =
        `Bearer ${this.token}`;
    }

    const response = await fetch(`${API_BASE}${path}`, { headers });
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

    const contentType =
      response.headers.get("Content-Type") || "application/octet-stream";
    const contentDisposition =
      response.headers.get("Content-Disposition") || "";
    const filenameMatch = contentDisposition.match(/filename=\"?([^\";]+)\"?/i);
    const filename = filenameMatch ? filenameMatch[1] : undefined;
    const blob = await response.blob();
    return { blob, filename, contentType };
  }

  private async fetchBlob(
    path: string,
  ): Promise<{ blob: Blob; filename?: string; contentType?: string }> {
    try {
      return await this.fetchBlobInternal(path);
    } catch (error) {
      if (
        error instanceof ApiError &&
        error.status === 401 &&
        this.hasRefreshToken()
      ) {
        await this.refreshTokenInternal();
        return this.fetchBlobInternal(path);
      }
      throw error;
    }
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

  private hasRefreshToken(): boolean {
    return (
      typeof window !== "undefined" &&
      !!localStorage.getItem("refresh_token")
    );
  }

  // Deduplicates concurrent refresh attempts — all callers share one in-flight request.
  private async refreshTokenInternal(): Promise<TokenResponse> {
    if (!this.refreshPromise) {
      this.refreshPromise = this.doRefresh().finally(() => {
        this.refreshPromise = null;
      });
    }
    return this.refreshPromise;
  }

  // Performs the actual token refresh. Uses fetchInternal to avoid triggering the 401 interceptor.
  private async doRefresh(): Promise<TokenResponse> {
    const refreshToken =
      typeof window !== "undefined"
        ? localStorage.getItem("refresh_token")
        : null;
    if (!refreshToken) {
      throw new ApiError(401, "no_refresh_token", "No refresh token available");
    }
    const response = await this.fetchInternal<TokenResponse>("/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    this.setToken(response.data.access_token);
    if (typeof window !== "undefined" && response.data.refresh_token) {
      localStorage.setItem("refresh_token", response.data.refresh_token);
    }
    return response.data;
  }

  async refreshToken(): Promise<TokenResponse> {
    return this.refreshTokenInternal();
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

  async translateGitHubWorkflows(data: {
    repo_full_name: string;
    github_installation_id?: string;
    ref?: string;
  }) {
    return this.fetch<GitHubWorkflowTranslateResponse>(
      "/providers/github/workflows/translate",
      {
        method: "POST",
        body: JSON.stringify(data),
      },
    );
  }

  async translateGitHubWorkflowYAML(data: {
    workflow_yaml: string;
    file_name?: string;
  }) {
    return this.fetch<GitHubWorkflowTranslateResponse>("/workflows/translate", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async getSampleTemplate(language: string) {
    return this.fetch<{ language: string; project_type: string; template: string }>(
      `/templates/sample?language=${encodeURIComponent(language)}`,
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
    default_branch?: string;
    dagryn_config?: string;
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
    if (data.default_branch != null && data.default_branch !== "") {
      body.default_branch = data.default_branch;
    }
    if (data.dagryn_config != null && data.dagryn_config !== "") {
      body.dagryn_config = data.dagryn_config;
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

  async getRunDashboardSummary(projectId: string, days = 30) {
    return this.fetch<RunDashboardSummary>(
      `/projects/${projectId}/runs/summary?days=${days}`,
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

  // Artifacts
  async listRunArtifacts(
    projectId: string,
    runId: string,
    options?: { task?: string; limit?: number; offset?: number },
  ) {
    const params = new URLSearchParams();
    if (options?.task) params.set("task", options.task);
    if (options?.limit) params.set("limit", String(options.limit));
    if (options?.offset) params.set("offset", String(options.offset));
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.fetch<Artifact[]>(
      `/projects/${projectId}/runs/${runId}/artifacts${query}`,
    );
  }

  async deleteArtifact(projectId: string, runId: string, artifactId: string) {
    await this.fetch(
      `/projects/${projectId}/runs/${runId}/artifacts/${artifactId}`,
      { method: "DELETE" },
    );
  }

  async downloadArtifact(projectId: string, runId: string, artifactId: string) {
    return this.fetchBlob(
      `/projects/${projectId}/runs/${runId}/artifacts/${artifactId}/download`,
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
      default_branch?: string;
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

  async syncProjectWorkflowFromToml(projectId: string, rawConfig: string) {
    return this.fetch<SyncWorkflowResponse>(
      `/projects/${projectId}/workflows/sync-from-toml`,
      {
        method: "POST",
        body: JSON.stringify({ raw_config: rawConfig }),
      },
    );
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

  // AI Analysis
  async getRunAIAnalysis(projectId: string, runId: string) {
    return this.fetch<AIAnalysisDetail>(
      `/projects/${projectId}/runs/${runId}/ai-analysis`,
    );
  }

  async listProjectAIAnalyses(
    projectId: string,
    options?: { limit?: number; offset?: number },
  ) {
    const params = new URLSearchParams();
    if (options?.limit) params.set("limit", String(options.limit));
    if (options?.offset) params.set("offset", String(options.offset));
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.fetch<AIAnalysesList>(
      `/projects/${projectId}/ai-analyses${query}`,
    );
  }

  async retryAIAnalysis(projectId: string, runId: string) {
    return this.fetch(
      `/projects/${projectId}/runs/${runId}/ai-analysis/retry`,
      {
        method: "POST",
      },
    );
  }

  async getAISuggestions(projectId: string, runId: string) {
    return this.fetch<AISuggestionsResponse>(
      `/projects/${projectId}/runs/${runId}/ai-suggestions`,
    );
  }

  async postAISuggestions(projectId: string, runId: string) {
    return this.fetch(
      `/projects/${projectId}/runs/${runId}/ai-suggestions/post`,
      { method: "POST" },
    );
  }

  // Plugins
  async listOfficialPlugins(params?: {
    q?: string;
    type?: string;
    sort?: string;
  }) {
    const query = new URLSearchParams();
    if (params?.q) query.set("q", params.q);
    if (params?.type) query.set("type", params.type);
    if (params?.sort) query.set("sort", params.sort);
    const suffix = query.toString() ? `?${query.toString()}` : "";
    return this.fetch<{ plugins: PluginInfo[] }>(`/plugins/official${suffix}`);
  }

  async getPlugin(pluginName: string) {
    return this.fetch<PluginInfo>(`/plugins/${pluginName}`);
  }

  async listProjectPlugins(projectId: string) {
    return this.fetch<{ plugins: PluginInfo[] }>(
      `/projects/${projectId}/plugins`,
    );
  }

  // Plugin Registry
  async searchRegistryPlugins(params?: {
    q?: string;
    type?: string;
    publisher?: string;
    sort?: string;
    page?: number;
    per_page?: number;
  }) {
    const query = new URLSearchParams();
    if (params?.q) query.set("q", params.q);
    if (params?.type) query.set("type", params.type);
    if (params?.publisher) query.set("publisher", params.publisher);
    if (params?.sort) query.set("sort", params.sort);
    if (params?.page) query.set("page", String(params.page));
    if (params?.per_page) query.set("per_page", String(params.per_page));
    const suffix = query.toString() ? `?${query.toString()}` : "";
    return this.fetch<PluginSearchResult>(`/registry/plugins${suffix}`);
  }

  async getRegistryPlugin(publisher: string, name: string) {
    return this.fetch<RegistryPluginDetail>(
      `/registry/plugins/${publisher}/${name}`,
    );
  }

  async getRegistryPluginVersions(publisher: string, name: string) {
    return this.fetch<PluginVersionInfo[]>(
      `/registry/plugins/${publisher}/${name}/versions`,
    );
  }

  async getRegistryPluginAnalytics(publisher: string, name: string, days = 30) {
    return this.fetch<PluginAnalytics>(
      `/registry/plugins/${publisher}/${name}/analytics?days=${days}`,
    );
  }

  async listFeaturedPlugins(limit = 10) {
    return this.fetch<RegistryPluginSummary[]>(
      `/registry/featured?limit=${limit}`,
    );
  }

  async listTrendingPlugins(limit = 10) {
    return this.fetch<RegistryPluginSummary[]>(
      `/registry/trending?limit=${limit}`,
    );
  }

  async recordPluginDownload(publisher: string, name: string, version: string) {
    return this.fetch(`/registry/plugins/${publisher}/${name}/download`, {
      method: "POST",
      body: JSON.stringify({ version }),
    });
  }

  async publishPluginVersion(
    publisher: string,
    name: string,
    data: {
      version: string;
      manifest: Record<string, unknown>;
      release_notes?: string;
    },
  ) {
    return this.fetch(`/registry/plugins/${publisher}/${name}/versions`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async installProjectPlugin(projectId: string, spec: string) {
    return this.fetch(`/projects/${projectId}/plugins`, {
      method: "POST",
      body: JSON.stringify({ spec }),
    });
  }

  async uninstallProjectPlugin(projectId: string, pluginName: string) {
    return this.fetch(`/projects/${projectId}/plugins/${pluginName}`, {
      method: "DELETE",
    });
  }

  // Dashboard Overview
  async getDashboardOverview() {
    return this.fetch<DashboardOverview>("/dashboard/overview");
  }

  // Capabilities
  async getCapabilities() {
    return this.fetch<CapabilitiesResponse>("/capabilities");
  }

  // License
  async getLicenseStatus() {
    return this.fetch<LicenseStatus>("/license");
  }

  async activateLicense(licenseKey: string) {
    return this.fetch<LicenseStatus>("/license/activate", {
      method: "POST",
      body: JSON.stringify({ license_key: licenseKey }),
    });
  }

  // Audit Logs
  async listTeamAuditLogs(
    teamId: string,
    params?: {
      actor_id?: string;
      actor_email?: string;
      action?: string;
      category?: string;
      since?: string;
      until?: string;
      cursor?: string;
      limit?: number;
    },
  ) {
    const qs = new URLSearchParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined) qs.set(k, String(v));
      });
    }
    const q = qs.toString();
    return this.fetch<AuditLogListResult>(
      `/teams/${teamId}/audit-logs${q ? `?${q}` : ""}`,
    );
  }

  async listProjectAuditLogs(
    projectId: string,
    params?: {
      actor_id?: string;
      actor_email?: string;
      action?: string;
      category?: string;
      since?: string;
      until?: string;
      cursor?: string;
      limit?: number;
    },
  ) {
    const qs = new URLSearchParams();
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined) qs.set(k, String(v));
      });
    }
    const q = qs.toString();
    return this.fetch<AuditLogListResult>(
      `/projects/${projectId}/audit-logs${q ? `?${q}` : ""}`,
    );
  }

  async exportTeamAuditLogs(
    teamId: string,
    format: "csv" | "json" = "json",
    params?: {
      actor_email?: string;
      action?: string;
      category?: string;
      since?: string;
      until?: string;
    },
  ) {
    const qs = new URLSearchParams();
    qs.set("format", format);
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined) qs.set(k, String(v));
      });
    }
    return this.fetchBlob(`/teams/${teamId}/audit-logs/export?${qs.toString()}`);
  }

  async getAuditRetentionPolicy(teamId: string) {
    return this.fetch<AuditRetentionPolicy>(
      `/teams/${teamId}/audit-logs/retention`,
    );
  }

  async updateAuditRetentionPolicy(teamId: string, retentionDays: number) {
    return this.fetch<AuditRetentionPolicy>(
      `/teams/${teamId}/audit-logs/retention`,
      {
        method: "PUT",
        body: JSON.stringify({ retention_days: retentionDays }),
      },
    );
  }

  async verifyAuditChain(teamId: string) {
    return this.fetch<AuditChainVerifyResult>(
      `/teams/${teamId}/audit-logs/verify`,
    );
  }

  // Audit Webhooks
  async listAuditWebhooks(teamId: string) {
    return this.fetch<AuditWebhook[]>(
      `/teams/${teamId}/audit-logs/webhooks`,
    );
  }

  async createAuditWebhook(
    teamId: string,
    data: { url: string; description?: string; event_filter?: string[] },
  ) {
    return this.fetch<AuditWebhookCreated>(
      `/teams/${teamId}/audit-logs/webhooks`,
      {
        method: "POST",
        body: JSON.stringify(data),
      },
    );
  }

  async updateAuditWebhook(
    teamId: string,
    webhookId: string,
    data: Partial<{ url: string; description: string; event_filter: string[]; is_active: boolean }>,
  ) {
    return this.fetch<AuditWebhook>(
      `/teams/${teamId}/audit-logs/webhooks/${webhookId}`,
      {
        method: "PUT",
        body: JSON.stringify(data),
      },
    );
  }

  async deleteAuditWebhook(teamId: string, webhookId: string) {
    return this.fetch<void>(
      `/teams/${teamId}/audit-logs/webhooks/${webhookId}`,
      {
        method: "DELETE",
      },
    );
  }

  async testAuditWebhook(teamId: string, webhookId: string) {
    return this.fetch<{ success: boolean; status_code?: number; error?: string; duration_ms: number }>(
      `/teams/${teamId}/audit-logs/webhooks/${webhookId}/test`,
      { method: "POST" },
    );
  }

  // Analytics
  async getTeamAnalytics(teamId: string, days = 30) {
    return this.fetch<AnalyticsOverview>(
      `/teams/${teamId}/analytics?days=${days}`,
    );
  }

  async getUserAnalytics(days = 30) {
    return this.fetch<AnalyticsOverview>(`/analytics?days=${days}`);
  }

  // Public wrapper around the private fetch method.
  // Used by the cloud overlay to make authenticated API calls
  // (e.g. billing) without duplicating auth/refresh logic.
  async request<T>(path: string, options: RequestInit = {}): Promise<{ data: T; message?: string }> {
    return this.fetch<T>(path, options);
  }
}

export const api = new ApiClient();
