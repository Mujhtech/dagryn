// Package job provides background job processing using asynq.
package job

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/encrypt"
	"github.com/mujhtech/dagryn/internal/job/handlers"
	"github.com/mujhtech/dagryn/internal/redis"
	"github.com/mujhtech/dagryn/internal/server/sse"
	"github.com/mujhtech/dagryn/internal/service"
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
	runs            *repo.RunRepo
	projects        *repo.ProjectRepo
	workflows       *repo.WorkflowRepo
	providerTokens  *repo.ProviderTokenRepo
	providerEncrypt encrypt.Encrypt
	githubApp       interface {
		FetchInstallationToken(ctx context.Context, installationID int64) (*handlers.InstallationToken, error)
	}
	githubInstallations *repo.GitHubInstallationRepo
	cacheService        *service.CacheService
	artifactService     *service.ArtifactService
	containerDefaults   *handlers.ContainerDefaults
	eventPublisher      sse.EventPublisher
}

// Config holds the configuration for the job system.
type Config struct {
	// Concurrency is the number of concurrent workers.
	Concurrency int
	// EncryptionKey is used to encrypt job payloads.
	EncryptionKey string
	// RunRepo is the repository for run operations.
	RunRepo *repo.RunRepo
	// ProjectRepo is the repository for project operations (required for ExecuteRun).
	ProjectRepo *repo.ProjectRepo
	// WorkflowRepo is the repository for workflow operations (used to link workflows to runs).
	WorkflowRepo *repo.WorkflowRepo
	// ProviderTokenRepo is used to fetch the repo-linked user's GitHub token for private clones.
	ProviderTokenRepo *repo.ProviderTokenRepo
	// ProviderTokenEncrypt is used to decrypt provider tokens (same key as server: JWT secret truncated).
	ProviderTokenEncrypt encrypt.Encrypt
	// GitHubAppClient is used to fetch installation tokens for GitHub App-based repos.
	GitHubAppClient interface {
		FetchInstallationToken(ctx context.Context, installationID int64) (*handlers.InstallationToken, error)
	}
	// GitHubInstallations is the repository for GitHub App installations.
	GitHubInstallations *repo.GitHubInstallationRepo
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
		encrypter:           enc,
		Client:              NewClient(rds, enc),
		Executor:            NewExecutor(appCtx, rds, cfg.Concurrency),
		Scheduler:           NewScheduler(rds),
		CancelManager:       cancelMgr,
		runs:                cfg.RunRepo,
		projects:            cfg.ProjectRepo,
		workflows:           cfg.WorkflowRepo,
		providerTokens:      cfg.ProviderTokenRepo,
		providerEncrypt:     cfg.ProviderTokenEncrypt,
		githubApp:           cfg.GitHubAppClient,
		githubInstallations: cfg.GitHubInstallations,
		cacheService:        cfg.CacheService,
		artifactService:     cfg.ArtifactService,
		containerDefaults:   cfg.ContainerDefaults,
		eventPublisher:      eventPub,
	}, nil
}

// RegisterAndStart registers all job handlers and starts the executor and scheduler.
func (j *Job) RegisterAndStart() error {
	// Register job handlers
	j.Executor.RegisterJobHandler(WebhookTaskName, asynq.HandlerFunc(handlers.HandleWebhook(j.encrypter)))

	// Register stale runs handler if RunRepo is available
	if j.runs != nil {
		staleRunsHandler := handlers.NewStaleRunsHandler(j.runs)
		j.Executor.RegisterJobHandler(StaleRunsTaskName, asynq.HandlerFunc(staleRunsHandler.Handle))

		// Schedule stale runs check to run every minute
		j.Scheduler.RegisterTask("*/1 * * * *", ScheduleQueueName, StaleRunsTaskName)
	}

	// Register ExecuteRun handler when RunRepo and ProjectRepo are available
	if j.runs != nil && j.projects != nil {
		execHandler := handlers.NewExecuteRunHandler(j.runs, j.projects, j.workflows, j.encrypter, j.providerTokens, j.providerEncrypt, j.githubApp, j.githubInstallations, j.cacheService, j.artifactService, j.CancelManager, j.containerDefaults, j.eventPublisher)
		j.Executor.RegisterJobHandler(ExecuteRunTaskName, asynq.HandlerFunc(execHandler.Handle))
	}

	// Register cache GC handler if cache service is available
	if j.cacheService != nil && j.projects != nil {
		cacheGCHandler := handlers.NewCacheGCHandler(j.cacheService, j.projects)
		j.Executor.RegisterJobHandler(CacheGCTaskName, asynq.HandlerFunc(cacheGCHandler.Handle))
		j.Scheduler.RegisterTask("0 * * * *", ScheduleQueueName, CacheGCTaskName) // every hour
	}

	// Register artifact cleanup handler if artifact service is available
	if j.artifactService != nil {
		artifactCleanupHandler := handlers.NewArtifactCleanupHandler(j.artifactService)
		j.Executor.RegisterJobHandler(ArtifactCleanupTaskName, asynq.HandlerFunc(artifactCleanupHandler.Handle))
		j.Scheduler.RegisterTask("0 2 * * *", ScheduleQueueName, ArtifactCleanupTaskName) // daily at 02:00
	}

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
