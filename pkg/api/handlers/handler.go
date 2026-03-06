package handlers

import (
	"context"

	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/githubapp"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/mujhtech/dagryn/pkg/server/sse"
	"github.com/mujhtech/dagryn/pkg/service"
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

	// Audit service (optional; nil when audit logs are not configured)
	auditService *service.AuditService

	// Encryption for webhook secrets (optional; uses providerEncrypt if not set)
	encrypter encrypt.Encrypt

	// Unified entitlement checker (license-backed for OSS, billing-backed for cloud).
	entitlements entitlement.Checker

	// License feature gate (optional; nil = Community edition).
	// Used by the /license endpoint for detailed claims info.
	featureGate *licensing.FeatureGate

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
		baseURL:            baseURL,
	}, nil
}

// SetEntitlementChecker sets the unified entitlement checker.
// In the OSS binary this is a LicenseChecker; in the cloud binary
// it will be a BillingChecker from the private repo.
func (h *Handler) SetEntitlementChecker(c entitlement.Checker) {
	h.entitlements = c
}

// Entitlements returns the entitlement checker (may be nil during startup).
func (h *Handler) Entitlements() entitlement.Checker {
	return h.entitlements
}

// SetFeatureGate sets the license feature gate for detailed license info.
func (h *Handler) SetFeatureGate(gate *licensing.FeatureGate) {
	h.featureGate = gate
}

// FeatureGate returns the license feature gate (may be nil).
func (h *Handler) FeatureGate() *licensing.FeatureGate {
	return h.featureGate
}

// SetAuditService sets the audit service for audit logging.
func (h *Handler) SetAuditService(s *service.AuditService) {
	h.auditService = s
}

// AuditService returns the audit service (may be nil).
func (h *Handler) AuditService() *service.AuditService {
	return h.auditService
}

// SetEncrypter sets the encrypter for webhook secret encryption.
func (h *Handler) SetEncrypter(e encrypt.Encrypt) {
	h.encrypter = e
}

// Encrypter returns the encrypter (falls back to providerEncrypt).
func (h *Handler) Encrypter() encrypt.Encrypt {
	if h.encrypter != nil {
		return h.encrypter
	}
	return h.providerEncrypt
}
