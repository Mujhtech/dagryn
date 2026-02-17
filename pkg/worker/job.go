package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/server/sse"
	"github.com/mujhtech/dagryn/pkg/service"
	dagrynstripe "github.com/mujhtech/dagryn/pkg/stripe"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/redis"
	"github.com/mujhtech/dagryn/pkg/worker/handlers"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("dagryn.job")

// Job manages the background job system including client, executor, and scheduler.
type Job struct {
	Client          *Client
	Executor        *Executor
	Scheduler       *Scheduler
	CancelManager   *CancelManager
	encrypter       encrypt.Encrypt
	store           store.Store
	providerEncrypt encrypt.Encrypt
	githubApp       interface {
		FetchInstallationToken(ctx context.Context, installationID int64) (*handlers.InstallationToken, error)
	}
	cacheService      *service.CacheService
	artifactService   *service.ArtifactService
	containerDefaults *handlers.ContainerDefaults
	eventPublisher    sse.EventPublisher
	stripeClient      *dagrynstripe.Client
	quotaService      *service.QuotaService
	aiConfig          *handlers.AIAnalysisConfig
	metrics           *telemetry.Metrics
	baseURL           string
}

// Config holds the configuration for the job system.
type Config struct {
	// Concurrency is the number of concurrent workers.
	Concurrency int
	// EncryptionKey is used to encrypt job payloads.
	EncryptionKey string
	Store         store.Store

	// ProviderTokenEncrypt is used to decrypt provider tokens (same key as server: JWT secret truncated).
	ProviderTokenEncrypt encrypt.Encrypt
	// GitHubAppClient is used to fetch installation tokens for GitHub App-based repos.
	GitHubAppClient interface {
		FetchInstallationToken(ctx context.Context, installationID int64) (*handlers.InstallationToken, error)
	}
	// CacheService is the cache service for GC jobs (optional).
	CacheService *service.CacheService
	// ArtifactService is the artifact service for uploads/cleanup (optional).
	ArtifactService *service.ArtifactService
	// CancelManager is optional; if nil and Redis is available, a default will be created.
	CancelManager *CancelManager
	// ContainerDefaults holds server-level container isolation defaults (optional).
	ContainerDefaults *handlers.ContainerDefaults
	// EventPublisher publishes SSE events to Redis for real-time browser updates (optional).
	EventPublisher sse.EventPublisher

	// StripeClient is the Stripe client for reporting usage (optional).
	StripeClient *dagrynstripe.Client
	// QuotaService is the quota enforcement service (optional).
	QuotaService *service.QuotaService
	// AIConfig holds AI analysis job configuration (optional).
	AIConfig *handlers.AIAnalysisConfig
	// Metrics holds OTel metric instruments (optional).
	Metrics *telemetry.Metrics
	// BaseURL is the public-facing dashboard URL used in GitHub check runs and AI comments.
	BaseURL string
}

// DefaultConfig returns sensible defaults for job configuration.
func DefaultConfig() Config {
	return Config{
		Concurrency: 10,
	}
}

// New creates a new Job instance.
func New(cfg Config, appCtx context.Context, rds *redis.Redis) (*Job, error) {
	// Create encrypter
	var enc encrypt.Encrypt
	if cfg.EncryptionKey != "" {
		var err error
		enc, err = encrypt.NewAESEncrypt(cfg.EncryptionKey)
		if err != nil {
			return nil, err
		}
	} else {
		// Use no-op encryption for development (not recommended for production)
		enc = encrypt.NewNoOpEncrypt()
	}

	cancelMgr := cfg.CancelManager
	if cancelMgr == nil && rds != nil {
		cancelMgr = NewCancelManager(rds)
	}

	eventPub := cfg.EventPublisher
	if eventPub == nil {
		eventPub = sse.NoOpEventPublisher{}
	}

	return &Job{
		encrypter:     enc,
		Client:        NewClient(rds, enc),
		Executor:      NewExecutor(appCtx, rds, cfg.Concurrency),
		Scheduler:     NewScheduler(rds),
		CancelManager: cancelMgr,

		providerEncrypt:   cfg.ProviderTokenEncrypt,
		githubApp:         cfg.GitHubAppClient,
		cacheService:      cfg.CacheService,
		artifactService:   cfg.ArtifactService,
		containerDefaults: cfg.ContainerDefaults,
		eventPublisher:    eventPub,
		stripeClient:      cfg.StripeClient,
		quotaService:      cfg.QuotaService,
		aiConfig:          cfg.AIConfig,
		metrics:           cfg.Metrics,
		baseURL:           cfg.BaseURL,
	}, nil
}

// RegisterAndStart registers all job handlers and starts the executor and scheduler.
func (j *Job) RegisterAndStart() error {
	// Register job handlers
	j.Executor.RegisterJobHandler(WebhookTaskName, asynq.HandlerFunc(handlers.HandleWebhook(j.encrypter)))

	staleRunsHandler := handlers.NewStaleRunsHandler(j.store.Runs)
	j.Executor.RegisterJobHandler(StaleRunsTaskName, asynq.HandlerFunc(staleRunsHandler.Handle))

	// Schedule stale runs check to run every minute
	j.Scheduler.RegisterTask("*/1 * * * *", ScheduleQueueName, StaleRunsTaskName)

	execHandler := handlers.NewExecuteRunHandler(
		j.store.Runs,
		j.store.Projects,
		j.store.Workflows,
		j.encrypter,
		j.store.ProviderTokens,
		j.providerEncrypt,
		j.githubApp,
		j.store.GitHubInstallations,
		j.cacheService,
		j.artifactService,
		j.CancelManager,
		j.containerDefaults,
		j.eventPublisher,
		j.quotaService,
		j.Client,
		j.baseURL,
	)
	j.Executor.RegisterJobHandler(ExecuteRunTaskName, asynq.HandlerFunc(execHandler.Handle))

	// Register cache GC handler if cache service is available
	if j.cacheService != nil {
		cacheGCHandler := handlers.NewCacheGCHandler(
			j.cacheService,
			j.store.Projects,
		)
		j.Executor.RegisterJobHandler(CacheGCTaskName, asynq.HandlerFunc(cacheGCHandler.Handle))
		j.Scheduler.RegisterTask("0 * * * *", ScheduleQueueName, CacheGCTaskName) // every hour
	}

	// Register artifact cleanup handler if artifact service is available
	if j.artifactService != nil {
		artifactCleanupHandler := handlers.NewArtifactCleanupHandler(j.artifactService)
		j.Executor.RegisterJobHandler(ArtifactCleanupTaskName, asynq.HandlerFunc(artifactCleanupHandler.Handle))
		j.Scheduler.RegisterTask("0 2 * * *", ScheduleQueueName, ArtifactCleanupTaskName) // daily at 02:00
	}

	// Register billing usage rollup handler if billing repo is available

	usageRollupHandler := handlers.NewUsageRollupHandler(j.store.Billing, j.stripeClient)
	j.Executor.RegisterJobHandler(UsageRollupTaskName, asynq.HandlerFunc(usageRollupHandler.Handle))
	j.Scheduler.RegisterTask("0 */6 * * *", ScheduleQueueName, UsageRollupTaskName) // every 6 hours

	bandwidthResetHandler := handlers.NewBandwidthResetHandler(j.store.Billing)
	j.Executor.RegisterJobHandler(BandwidthResetTaskName, asynq.HandlerFunc(bandwidthResetHandler.Handle))
	j.Scheduler.RegisterTask("0 0 * * *", ScheduleQueueName, BandwidthResetTaskName) // daily at midnight

	// Register retention cleanup handler if billing and quota services are available
	if j.quotaService != nil {
		retentionHandler := handlers.NewRetentionCleanupHandler(
			j.store.Billing,
			j.store.Artifacts,
			j.store.Runs,
			j.store.Cache,
			j.quotaService,
		)
		j.Executor.RegisterJobHandler(RetentionCleanupTaskName, asynq.HandlerFunc(retentionHandler.Handle))
		j.Scheduler.RegisterTask("0 3 * * *", ScheduleQueueName, RetentionCleanupTaskName) // daily at 03:00
	}

	// Register AI handlers when AI repo is available.
	// The project's dagryn.toml controls whether AI is enabled per-run;
	// handlers are always registered so jobs enqueued by any project can be processed.

	aiHandler := handlers.NewAIAnalysisHandler(
		j.store.Runs,
		j.store.Workflows,
		j.store.AI,
		j.encrypter,
		j.aiConfig,
		j.Client,
		log.Logger,
		j.quotaService,
		j.store.Billing,
		j.metrics,
	)
	j.Executor.RegisterJobHandler(AIAnalysisTaskName, asynq.HandlerFunc(aiHandler.Handle))

	// Register AI publish handler
	pubHandler := handlers.NewAIPublishHandler(
		j.store.AI,
		j.store.Runs,
		j.store.Projects,
		j.store.ProviderTokens,
		j.providerEncrypt,
		j.githubApp,
		j.store.GitHubInstallations,
		j.encrypter,
		j.baseURL,
		log.Logger,
		j.metrics,
	)
	j.Executor.RegisterJobHandler(AIPublishTaskName, asynq.HandlerFunc(pubHandler.Handle))

	// Register AI suggest run handler (v2 — generate inline code suggestions).
	// Provider is now built per-job from the project config in the payload.
	suggestHandler := handlers.NewAISuggestHandler(
		j.store.AI,
		j.store.Runs,
		j.encrypter,
		handlers.DefaultAISuggestConfig(),
		j.aiConfig,
		log.Logger,
		j.metrics,
	)
	j.Executor.RegisterJobHandler(AISuggestRunTaskName, asynq.HandlerFunc(suggestHandler.Handle))

	// Register AI suggest publish handler (v2 — posts suggestions as GitHub PR review)
	suggestPubHandler := handlers.NewAISuggestPublishHandler(
		j.store.AI,
		j.store.Runs,
		j.store.Projects,
		j.store.ProviderTokens,
		j.providerEncrypt,
		j.githubApp,
		j.store.GitHubInstallations,
		j.encrypter,
		log.Logger,
	)
	j.Executor.RegisterJobHandler(AISuggestPublishTaskName, asynq.HandlerFunc(suggestPubHandler.Handle))

	// Register AI blob cleanup handler (daily at 04:00)
	blobCleanupHandler := handlers.NewAIBlobCleanupHandler(j.store.AI, 168, log.Logger)
	j.Executor.RegisterJobHandler(AIBlobCleanupTaskName, asynq.HandlerFunc(blobCleanupHandler.Handle))
	j.Scheduler.RegisterTask("0 4 * * *", ScheduleQueueName, AIBlobCleanupTaskName)

	// Start scheduler
	if err := j.Scheduler.Start(); err != nil {
		return err
	}

	// Start executor
	return j.Executor.Start()
}

// Stop gracefully stops the job system.
func (j *Job) Stop() {
	j.Executor.Stop()
	j.Scheduler.Stop()
}
