package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// UserStore defines the interface for user repository operations.
type UserStore interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByProvider(ctx context.Context, provider, providerID string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpsertByProvider(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// TokenStore defines the interface for token repository operations.
type TokenStore interface {
	Create(ctx context.Context, token *models.Token) error
	GetByJTI(ctx context.Context, jti string) (*models.Token, error)
	GetValidByJTI(ctx context.Context, jti string) (*models.Token, error)
	UpdateLastUsed(ctx context.Context, jti string) error
	Revoke(ctx context.Context, jti string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	RevokeByType(ctx context.Context, userID uuid.UUID, tokenType models.TokenType) error
	CleanupExpired(ctx context.Context, olderThan time.Duration) (int64, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Token, error)
}

// TeamStore defines the interface for team repository operations.
type TeamStore interface {
	Create(ctx context.Context, team *models.Team) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error)
	GetBySlug(ctx context.Context, slug string) (*models.Team, error)
	Update(ctx context.Context, team *models.Team) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]models.TeamWithMember, error)
	AddMember(ctx context.Context, teamID, userID uuid.UUID, role models.Role, invitedBy *uuid.UUID) error
	UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.Role) error
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
	GetMember(ctx context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error)
	ListMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMemberWithUser, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
}

// ProjectStore defines the interface for project repository operations.
type ProjectStore interface {
	Create(ctx context.Context, project *models.Project, ownerID uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
	GetByRepoURL(ctx context.Context, repoURL string) (*models.Project, error)
	GetByGitHubRepoID(ctx context.Context, installationID uuid.UUID, repoID int64) (*models.Project, error)
	GetBySlug(ctx context.Context, teamID *uuid.UUID, slug string) (*models.Project, error)
	GetByPathHash(ctx context.Context, userID uuid.UUID, pathHash string) (*models.Project, error)
	Update(ctx context.Context, project *models.Project) error
	UpdateLastRunAt(ctx context.Context, id uuid.UUID) error
	UpdateBillingAccountID(ctx context.Context, projectID, billingAccountID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]models.ProjectWithMember, error)
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]models.Project, error)
	AddMember(ctx context.Context, projectID, userID uuid.UUID, role models.Role, invitedBy *uuid.UUID) error
	UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.Role) error
	RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error
	GetMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error)
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]models.ProjectMemberWithUser, error)
	GetUserRole(ctx context.Context, projectID, userID uuid.UUID) (models.Role, error)
	SlugExists(ctx context.Context, teamID *uuid.UUID, slug string) (bool, error)
	ListPublic(ctx context.Context, limit, offset int) ([]models.Project, error)
	ListAll(ctx context.Context, limit, offset int) ([]models.Project, int, error)
}

// APIKeyStore defines the interface for API key repository operations.
type APIKeyStore interface {
	Create(ctx context.Context, key *models.APIKey) (string, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error)
	ValidateKey(ctx context.Context, rawKey string) (*models.APIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]models.APIKeyWithProject, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.APIKey, error)
	ListActive(ctx context.Context, userID uuid.UUID) ([]models.APIKeyWithProject, error)
}

// InvitationStore defines the interface for invitation repository operations.
type InvitationStore interface {
	Create(ctx context.Context, inv *models.Invitation) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Invitation, error)
	GetByToken(ctx context.Context, token string) (*models.Invitation, error)
	GetPendingByToken(ctx context.Context, token string) (*models.InvitationWithDetails, error)
	Accept(ctx context.Context, token string) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByToken(ctx context.Context, token string) error
	ListPendingByEmail(ctx context.Context, email string) ([]models.InvitationWithDetails, error)
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]models.InvitationWithDetails, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.InvitationWithDetails, error)
	CleanupExpired(ctx context.Context) (int64, error)
}

// RunStore defines the interface for run repository operations.
type RunStore interface {
	Create(ctx context.Context, run *models.Run) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Run, error)
	Update(ctx context.Context, run *models.Run) error
	UpdateGitHubCheckRunID(ctx context.Context, id uuid.UUID, checkRunID int64) error
	UpdateTargets(ctx context.Context, id uuid.UUID, targets []string) error
	Start(ctx context.Context, id uuid.UUID) error
	StartWithTotal(ctx context.Context, id uuid.UUID, totalTasks int) error
	Complete(ctx context.Context, id uuid.UUID, status models.RunStatus, errorMessage *string) error
	IncrementCompleted(ctx context.Context, id uuid.UUID, cacheHit bool) error
	IncrementFailed(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]models.Run, int, error)
	GetDashboardChartByProject(ctx context.Context, projectID uuid.UUID, days int) ([]RunDashboardChartPoint, error)
	GetDashboardFacetsByProject(ctx context.Context, projectID uuid.UUID) (*RunDashboardFacets, error)
	GetActiveByProject(ctx context.Context, projectID uuid.UUID) ([]models.Run, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CreateTaskResult(ctx context.Context, result *models.TaskResult) error
	UpdateTaskResult(ctx context.Context, result *models.TaskResult) error
	GetTaskResult(ctx context.Context, runID uuid.UUID, taskName string) (*models.TaskResult, error)
	ListTaskResults(ctx context.Context, runID uuid.UUID) ([]models.TaskResult, error)
	DeleteTaskResultsByRun(ctx context.Context, runID uuid.UUID) error
	GetRunWithTasks(ctx context.Context, id uuid.UUID) (*models.RunWithTasks, error)
	CleanupOldRuns(ctx context.Context, olderThan time.Duration, keepMinimum int) (int64, error)
	UpdateHeartbeat(ctx context.Context, id uuid.UUID) error
	ListStaleRuns(ctx context.Context, timeout time.Duration) ([]models.Run, error)
	MarkAsStale(ctx context.Context, id uuid.UUID) error
	AppendLog(ctx context.Context, log *models.RunLog) error
	AppendLogs(ctx context.Context, logs []models.RunLog) error
	GetLogs(ctx context.Context, runID uuid.UUID, limit, offset int) ([]models.RunLog, int, error)
	GetLogsByTask(ctx context.Context, runID uuid.UUID, taskName string, limit, offset int) ([]models.RunLog, int, error)
	GetLogsSince(ctx context.Context, runID uuid.UUID, afterID int64, limit int) ([]models.RunLog, error)
	DeleteLogs(ctx context.Context, runID uuid.UUID) error
	DeleteLogsOlderThanForProjects(ctx context.Context, projectIDs []uuid.UUID, before time.Time) (int64, error)
	GetRecentRunsAcrossProjects(ctx context.Context, projectIDs []uuid.UUID, limit int) ([]models.Run, error)
	GetProjectStats(ctx context.Context, projectIDs []uuid.UUID, days int) (map[uuid.UUID]*ProjectDashboardStats, error)
}

// ArtifactStore defines the interface for artifact repository operations.
type ArtifactStore interface {
	Create(ctx context.Context, artifact *models.Artifact) (*models.Artifact, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Artifact, error)
	ListByRun(ctx context.Context, runID uuid.UUID, limit, offset int) ([]*models.Artifact, error)
	ListByRunAndTask(ctx context.Context, runID uuid.UUID, taskName string, limit, offset int) ([]*models.Artifact, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListExpired(ctx context.Context, limit int) ([]*models.Artifact, error)
	DeleteExpired(ctx context.Context) (int64, error)
	DeleteOlderThanForProjects(ctx context.Context, projectIDs []uuid.UUID, before time.Time) (int64, []string, error)
	TotalSizeByProject(ctx context.Context, projectID uuid.UUID) (int64, error)
}

// ProviderTokenStore defines the interface for provider token repository operations.
type ProviderTokenStore interface {
	Upsert(ctx context.Context, userID uuid.UUID, provider, accessTokenEncrypted string) error
	GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*models.ProviderToken, error)
}

// GitHubInstallationStore defines the interface for GitHub installation repository operations.
type GitHubInstallationStore interface {
	UpsertByInstallationID(ctx context.Context, inst *models.GitHubInstallation) error
	GetByInstallationID(ctx context.Context, installationID int64) (*models.GitHubInstallation, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.GitHubInstallation, error)
	ListAll(ctx context.Context) ([]models.GitHubInstallation, error)
}

// WorkflowStore defines the interface for workflow repository operations.
type WorkflowStore interface {
	Upsert(ctx context.Context, workflow *models.ProjectWorkflow) (bool, error)
	UpsertTasks(ctx context.Context, workflowID uuid.UUID, tasks []models.WorkflowTask) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.WorkflowWithTasks, error)
	GetByProjectAndName(ctx context.Context, projectID uuid.UUID, name string) (*models.WorkflowWithTasks, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ProjectWorkflow, error)
	ListByProjectWithTasks(ctx context.Context, projectID uuid.UUID) ([]models.WorkflowWithTasks, error)
	GetDefaultByProject(ctx context.Context, projectID uuid.UUID) (*models.ProjectWorkflow, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// PluginRegistryStore defines the interface for plugin registry repository operations.
type PluginRegistryStore interface {
	GetPublisherByName(ctx context.Context, name string) (*models.PluginPublisher, error)
	GetPublisherByID(ctx context.Context, id uuid.UUID) (*models.PluginPublisher, error)
	CreatePublisher(ctx context.Context, p *models.PluginPublisher) (*models.PluginPublisher, error)
	ListPublishers(ctx context.Context) ([]*models.PluginPublisher, error)
	GetPluginByPublisherAndName(ctx context.Context, publisherName, pluginName string) (*models.RegistryPluginWithPublisher, error)
	GetPluginByID(ctx context.Context, id uuid.UUID) (*models.RegistryPlugin, error)
	CreatePlugin(ctx context.Context, p *models.RegistryPlugin) (*models.RegistryPlugin, error)
	UpdatePlugin(ctx context.Context, p *models.RegistryPlugin) error
	SearchPlugins(ctx context.Context, params PluginSearchParams) (*PluginSearchResult, error)
	ListFeatured(ctx context.Context, limit int) ([]*models.RegistryPluginWithPublisher, error)
	ListTrending(ctx context.Context, limit, days int) ([]*models.RegistryPluginWithPublisher, error)
	GetVersion(ctx context.Context, pluginID uuid.UUID, version string) (*models.PluginVersion, error)
	CreateVersion(ctx context.Context, v *models.PluginVersion) (*models.PluginVersion, error)
	ListVersions(ctx context.Context, pluginID uuid.UUID) ([]*models.PluginVersion, error)
	YankVersion(ctx context.Context, pluginID uuid.UUID, version string) error
	RecordDownload(ctx context.Context, d *models.PluginDownload) error
	IncrementDownloads(ctx context.Context, pluginID, versionID uuid.UUID) error
	GetDownloadStats(ctx context.Context, pluginID uuid.UUID, days int) ([]DownloadStat, error)
	RecomputeWeeklyDownloads(ctx context.Context) error
}

// AIStore defines the interface for AI repository operations.
type AIStore interface {
	CreateAnalysis(ctx context.Context, a *models.AIAnalysis) error
	GetAnalysisByID(ctx context.Context, id uuid.UUID) (*models.AIAnalysis, error)
	GetAnalysisByRunID(ctx context.Context, runID uuid.UUID) (*models.AIAnalysis, error)
	ListAnalysesByProject(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]models.AIAnalysis, int, error)
	UpdateAnalysisStatus(ctx context.Context, id uuid.UUID, status models.AIAnalysisStatus, errorMessage *string) error
	UpdateAnalysisResults(ctx context.Context, a *models.AIAnalysis) error
	FindPendingByDedupKey(ctx context.Context, dedupKey string) (*models.AIAnalysis, error)
	SupersedeByBranch(ctx context.Context, projectID uuid.UUID, branch string, excludeCommit string) error
	UpdateAnalysisDedupKey(ctx context.Context, id uuid.UUID, dedupKey string) error
	CountRecentAnalyses(ctx context.Context, projectID uuid.UUID, since time.Time) (int, error)
	CreatePublication(ctx context.Context, p *models.AIPublication) error
	GetPublicationByRunAndDestination(ctx context.Context, runID uuid.UUID, destination models.AIPublicationDestination) (*models.AIPublication, error)
	UpdatePublication(ctx context.Context, id uuid.UUID, status models.AIPublicationStatus, externalID *string, errorMessage *string) error
	ListPublicationsByAnalysis(ctx context.Context, analysisID uuid.UUID) ([]models.AIPublication, error)
	CreateSuggestion(ctx context.Context, s *models.AISuggestion) error
	ListSuggestionsByAnalysis(ctx context.Context, analysisID uuid.UUID) ([]models.AISuggestion, error)
	ListPendingSuggestionsByAnalysis(ctx context.Context, analysisID uuid.UUID) ([]models.AISuggestion, error)
	UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status models.AISuggestionStatus, githubCommentID *string, failureReason *string) error
	CountInProgressAnalyses(ctx context.Context, projectID uuid.UUID) (int, error)
	GetMostRecentAnalysisByKey(ctx context.Context, projectID uuid.UUID, branch, commit string) (*models.AIAnalysis, error)
	ListPostedSuggestionsByProjectAndBranch(ctx context.Context, projectID uuid.UUID, branch string) ([]models.AISuggestion, error)
	DeleteExpiredBlobKeys(ctx context.Context, olderThan time.Time) (int64, error)
}

// AuditLogStore defines the interface for audit log repository operations.
type AuditLogStore interface {
	Create(ctx context.Context, log *models.AuditLog) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
	List(ctx context.Context, filter AuditLogFilter) (*AuditLogListResult, error)
	DeleteBefore(ctx context.Context, teamID uuid.UUID, before time.Time) (int64, error)
	GetLastHash(ctx context.Context, teamID uuid.UUID) (string, int64, error)
	GetRetentionPolicy(ctx context.Context, teamID uuid.UUID) (*models.AuditLogRetentionPolicy, error)
	UpsertRetentionPolicy(ctx context.Context, policy *models.AuditLogRetentionPolicy) error
	ListAllRetentionPolicies(ctx context.Context) ([]models.AuditLogRetentionPolicy, error)
	CountByTeam(ctx context.Context, teamID uuid.UUID) (int64, error)
	ListChain(ctx context.Context, teamID uuid.UUID, afterSeq int64, limit int) ([]models.AuditLog, error)

	// Webhook CRUD
	CreateWebhook(ctx context.Context, w *models.AuditWebhook) error
	GetWebhookByID(ctx context.Context, id uuid.UUID) (*models.AuditWebhook, error)
	ListWebhooksByTeam(ctx context.Context, teamID uuid.UUID) ([]models.AuditWebhook, error)
	UpdateWebhook(ctx context.Context, w *models.AuditWebhook) error
	DeleteWebhook(ctx context.Context, id uuid.UUID) error
	ListActiveWebhooksByTeam(ctx context.Context, teamID uuid.UUID) ([]models.AuditWebhook, error)
}

// AnalyticsStore defines the interface for cross-project analytics aggregation.
type AnalyticsStore interface {
	GetTeamAnalytics(ctx context.Context, projectIDs []uuid.UUID, teamID *uuid.UUID, days int) (*TeamAnalytics, error)
}

// CacheStore defines the interface for cache repository operations.
type CacheStore interface {
	FindEntry(ctx context.Context, projectID uuid.UUID, taskName, cacheKey string) (*models.CacheEntry, error)
	UpsertEntry(ctx context.Context, entry *models.CacheEntry) error
	DeleteEntriesOlderThanForProjects(ctx context.Context, projectIDs []uuid.UUID, before time.Time) (int64, error)
	DeleteEntry(ctx context.Context, id uuid.UUID) error
	ListEntries(ctx context.Context, projectID uuid.UUID, opts ListEntriesOpts) ([]models.CacheEntry, error)
	IncrementHitCount(ctx context.Context, id uuid.UUID) error
	UpsertBlob(ctx context.Context, blob *models.CacheBlob) error
	DecrementBlobRef(ctx context.Context, digestHash string) error
	ListOrphanedBlobs(ctx context.Context) ([]models.CacheBlob, error)
	DeleteBlob(ctx context.Context, digestHash string) error
	GetQuota(ctx context.Context, projectID uuid.UUID) (*models.CacheQuota, error)
	EnsureQuota(ctx context.Context, projectID uuid.UUID) error
	UpdateQuotaUsage(ctx context.Context, projectID uuid.UUID, sizeDelta int64, entryDelta int) error
	IncrementBandwidthUsage(ctx context.Context, projectID uuid.UUID, bytes int64) error
	GetStats(ctx context.Context, projectID uuid.UUID) (*CacheStats, error)
	ListExpired(ctx context.Context, before time.Time) ([]models.CacheEntry, error)
	ListLRU(ctx context.Context, projectID uuid.UUID, limit int) ([]models.CacheEntry, error)
	IncrementUsage(ctx context.Context, projectID uuid.UUID, bytesUploaded, bytesDownloaded int64, hits, misses int) error
	GetUsageAnalytics(ctx context.Context, projectID uuid.UUID, days int) ([]UsageAnalytics, error)
}
