package handlers

import (
	"context"

	"github.com/mujhtech/dagryn/internal/db"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/encrypt"
	"github.com/mujhtech/dagryn/internal/githubapp"
	"github.com/mujhtech/dagryn/internal/job"
	"github.com/mujhtech/dagryn/internal/server/sse"
	"github.com/mujhtech/dagryn/internal/service"
	dagrynstripe "github.com/mujhtech/dagryn/internal/stripe"
)

// ReadyChecker can optionally be implemented by dependencies to participate in /ready.
type ReadyChecker interface {
	Ready(ctx context.Context) error
}

// Handler holds all HTTP handlers and their dependencies.
type Handler struct {
	db          *db.DB
	users       *repo.UserRepo
	tokens      *repo.TokenRepo
	teams       *repo.TeamRepo
	projects    *repo.ProjectRepo
	apikeys     *repo.APIKeyRepo
	invitations *repo.InvitationRepo
	runs        *repo.RunRepo
	sseHub      *sse.Hub
	jobClient   *job.Client

	providerTokens  *repo.ProviderTokenRepo
	providerEncrypt encrypt.Encrypt

	// Config-driven ready checks
	readyCheckDatabase bool
	readyCheckRedis    bool
	redisForReady      ReadyChecker

	// GitHub App integration (optional)
	githubApp           *githubapp.Client
	githubInstallations *repo.GitHubInstallationRepo

	// Workflow management
	workflows *repo.WorkflowRepo

	// Cache service (optional; nil when storage is not configured)
	cacheService *service.CacheService

	// Artifact service (optional; nil when storage is not configured)
	artifactService *service.ArtifactService

	// Cancel manager (optional; nil when Redis is not configured)
	cancelManager *job.CancelManager

	// Plugin registry service (optional; nil when DB is not configured for registry)
	registryService *service.PluginRegistryService

	// Billing service (optional; nil when Stripe is not configured)
	billingService *service.BillingService

	// Stripe client (optional; nil when Stripe is not configured)
	stripeClient *dagrynstripe.Client

	// Quota service (optional; nil when billing is not configured)
	quotaService *service.QuotaService
}

// New creates a new Handler with all dependencies.
// jobClient is optional; when set, TriggerRun will enqueue ExecuteRun for projects with repo_url.
// readyCheckDatabase and readyCheckRedis enable DB/Redis checks in /ready; redisForReady is used when readyCheckRedis is true.
// providerTokens and providerEncrypt are used for ListGitHubRepos (Import from GitHub).
func New(
	database *db.DB,
	users *repo.UserRepo,
	tokens *repo.TokenRepo,
	teams *repo.TeamRepo,
	projects *repo.ProjectRepo,
	apikeys *repo.APIKeyRepo,
	invitations *repo.InvitationRepo,
	runs *repo.RunRepo,
	sseHub *sse.Hub,
	jobClient *job.Client,
	providerTokens *repo.ProviderTokenRepo,
	providerEncrypt encrypt.Encrypt,
	readyCheckDatabase bool,
	readyCheckRedis bool,
	redisForReady ReadyChecker,
	githubApp *githubapp.Client,
	githubInstallations *repo.GitHubInstallationRepo,
	workflows *repo.WorkflowRepo,
	cacheService *service.CacheService,
	artifactService *service.ArtifactService,
	cancelManager *job.CancelManager,
	registryService *service.PluginRegistryService,
	billingService *service.BillingService,
	stripeClient *dagrynstripe.Client,
	quotaService *service.QuotaService,
) *Handler {
	return &Handler{
		db:                  database,
		users:               users,
		tokens:              tokens,
		teams:               teams,
		projects:            projects,
		apikeys:             apikeys,
		invitations:         invitations,
		runs:                runs,
		sseHub:              sseHub,
		jobClient:           jobClient,
		providerTokens:      providerTokens,
		providerEncrypt:     providerEncrypt,
		readyCheckDatabase:  readyCheckDatabase,
		readyCheckRedis:     readyCheckRedis,
		redisForReady:       redisForReady,
		githubApp:           githubApp,
		githubInstallations: githubInstallations,
		workflows:           workflows,
		cacheService:        cacheService,
		artifactService:     artifactService,
		cancelManager:       cancelManager,
		registryService:     registryService,
		billingService:      billingService,
		stripeClient:        stripeClient,
		quotaService:        quotaService,
	}
}
