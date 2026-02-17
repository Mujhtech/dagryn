package handlers

import (
	"context"

	"github.com/mujhtech/dagryn/internal/githubapp"
	"github.com/mujhtech/dagryn/internal/server/sse"
	"github.com/mujhtech/dagryn/internal/service"
	dagrynstripe "github.com/mujhtech/dagryn/internal/stripe"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/mujhtech/dagryn/pkg/worker"
)

// ReadyChecker can optionally be implemented by dependencies to participate in /ready.
type ReadyChecker interface {
	Ready(ctx context.Context) error
}

// Handler holds all HTTP handlers and their dependencies.
type Handler struct {
	db        *database.DB
	store     store.Store
	sseHub    *sse.Hub
	jobClient *worker.Client

	providerTokens  *repo.ProviderTokenRepo
	providerEncrypt encrypt.Encrypt

	// Config-driven ready checks
	readyCheckDatabase bool
	readyCheckRedis    bool
	redisForReady      ReadyChecker

	// GitHub App integration (optional)
	githubApp *githubapp.Client

	// Cache service (optional; nil when storage is not configured)
	cacheService *service.CacheService

	// Artifact service (optional; nil when storage is not configured)
	artifactService *service.ArtifactService

	// Cancel manager (optional; nil when Redis is not configured)
	cancelManager *worker.CancelManager

	// Plugin registry service (optional; nil when DB is not configured for registry)
	registryService *service.PluginRegistryService

	// Billing service (optional; nil when Stripe is not configured)
	billingService *service.BillingService

	// Stripe client (optional; nil when Stripe is not configured)
	stripeClient *dagrynstripe.Client

	// Quota service (optional; nil when billing is not configured)
	quotaService *service.QuotaService

	// License feature gate (optional; nil = Community edition or cloud mode)
	featureGate *licensing.FeatureGate

	// cloudMode is true for the managed cloud deployment.
	// When true, the license system is bypassed and billing handles everything.
	cloudMode bool

	// baseURL is the public-facing dashboard URL for links in GitHub check runs.
	baseURL string
}

// New creates a new Handler with all dependencies.
// jobClient is optional; when set, TriggerRun will enqueue ExecuteRun for projects with repo_url.
// readyCheckDatabase and readyCheckRedis enable DB/Redis checks in /ready; redisForReady is used when readyCheckRedis is true.
// providerTokens and providerEncrypt are used for ListGitHubRepos (Import from GitHub).
func New(
	database *database.DB,
	store store.Store,
	sseHub *sse.Hub,
	jobClient *worker.Client,
	providerEncrypt encrypt.Encrypt,
	readyCheckDatabase bool,
	readyCheckRedis bool,
	redisForReady ReadyChecker,
	githubApp *githubapp.Client,
	cacheService *service.CacheService,
	artifactService *service.ArtifactService,
	cancelManager *worker.CancelManager,
	registryService *service.PluginRegistryService,
	billingService *service.BillingService,
	stripeClient *dagrynstripe.Client,
	quotaService *service.QuotaService,
	baseURL string,
) (*Handler, error) {
	return &Handler{
		db:                 database,
		store:              store,
		sseHub:             sseHub,
		jobClient:          jobClient,
		providerEncrypt:    providerEncrypt,
		readyCheckDatabase: readyCheckDatabase,
		readyCheckRedis:    readyCheckRedis,
		redisForReady:      redisForReady,
		githubApp:          githubApp,
		cacheService:       cacheService,
		artifactService:    artifactService,
		cancelManager:      cancelManager,
		registryService:    registryService,
		billingService:     billingService,
		stripeClient:       stripeClient,
		quotaService:       quotaService,
		baseURL:            baseURL,
	}, nil
}

// SetFeatureGate sets the license feature gate for edition/feature checks.
func (h *Handler) SetFeatureGate(gate *licensing.FeatureGate) {
	h.featureGate = gate
}

// FeatureGate returns the license feature gate (may be nil).
func (h *Handler) FeatureGate() *licensing.FeatureGate {
	return h.featureGate
}

// SetCloudMode marks this handler as running in cloud (managed SaaS) mode.
func (h *Handler) SetCloudMode(enabled bool) {
	h.cloudMode = enabled
}

// IsCloudMode returns true when the server is running as the managed cloud deployment.
// In cloud mode, the license system is not used — quota enforcement is handled by the billing system.
func (h *Handler) IsCloudMode() bool {
	return h.cloudMode
}
