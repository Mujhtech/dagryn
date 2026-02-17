package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mujhtech/dagryn/pkg/githubapp"
	"github.com/mujhtech/dagryn/pkg/server/sse"
	"github.com/mujhtech/dagryn/pkg/service"
	dagrynstripe "github.com/mujhtech/dagryn/pkg/stripe"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/mujhtech/dagryn/pkg/cache"
	"github.com/mujhtech/dagryn/pkg/config"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/mujhtech/dagryn/pkg/redis"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/mujhtech/dagryn/pkg/worker"
	jobhandlers "github.com/mujhtech/dagryn/pkg/worker/handlers"
)

func init() {
	rootCmd.AddCommand(newWorkerCmd())
}

// WorkerConfigOpts holds CLI options for the worker command.
type WorkerConfigOpts struct {
	ConfigFile    string
	RedisHost     string
	RedisPort     int
	RedisPassword string
	Concurrency   int
	EncryptionKey string
}

func newWorkerCmd() *cobra.Command {
	var opts WorkerConfigOpts

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start the Dagryn background job worker",
		Long: `Starts the Dagryn background job worker which processes:

- Webhook deliveries
- Scheduled tasks
- Async workflow operations

The worker connects to Redis for job queue management and requires
the same configuration as the server for database access.

Configuration priority: CLI flags > environment variables > config file > defaults`,
		Example: `  # Start with defaults
  dagryn worker

  # Start with custom config file
  dagryn worker --config /etc/dagryn/server.toml

  # Start with custom Redis and concurrency
  dagryn worker --redis-host redis.local --concurrency 20

  # Start with environment variables
  REDIS_HOST=redis.local JOB_ENCRYPTION_KEY=xxx dagryn worker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorker(opts)
		},
	}

	// Config file
	cmd.Flags().StringVarP(&opts.ConfigFile, "config", "c", "", "Config file path")

	// Redis flags
	cmd.Flags().StringVar(&opts.RedisHost, "redis-host", "", "Redis host (default: localhost)")
	cmd.Flags().IntVar(&opts.RedisPort, "redis-port", 0, "Redis port (default: 6379)")
	cmd.Flags().StringVar(&opts.RedisPassword, "redis-password", "", "Redis password")

	// Worker flags
	cmd.Flags().IntVar(&opts.Concurrency, "concurrency", 0, "Number of concurrent workers (default: 10)")
	cmd.Flags().StringVar(&opts.EncryptionKey, "encryption-key", "", "Job payload encryption key (32 chars for AES-256)")

	return cmd
}

func runWorker(opts WorkerConfigOpts) error {
	// Load configuration using server config loader
	serverOpts := config.ConfigOpts{
		ConfigFile: opts.ConfigFile,
	}
	cfg, err := config.LoadConfig(serverOpts)
	if err != nil {
		// Worker doesn't need OAuth, so we can proceed even if validation fails
		// for OAuth-related fields. Load defaults and apply env vars manually.
		cfg = &config.Config{}
		*cfg = config.DefaultConfig()
		applyWorkerEnvVars(cfg)
	}

	// Apply CLI flag overrides
	applyWorkerCLIFlags(cfg, opts)

	// Create context with cancellation (used for DB and job system)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create Redis client
	rds := redis.New(cfg.Redis)

	// Create SSE event publisher for real-time browser updates via Redis pub/sub
	eventPublisher := sse.NewRedisEventPublisher(rds)

	// Connect to database for RunRepo, ProjectRepo, ProviderTokenRepo (required for ExecuteRun and stale_runs)
	var db *database.DB
	if cfg.Database.URL != "" {
		var err error
		db, err = database.New(ctx, cfg.Database)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}

	}

	cache, err := cache.NewCache(cfg, rds)

	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}

	store := store.New(cache, db)

	// Provider token decrypter (same as server: JWT secret truncated for AES) for cloning with repo owner's GitHub token
	var providerEncrypt encrypt.Encrypt
	if key := cfg.Auth.JWTSecret; len(key) >= 16 {
		if len(key) > 32 {
			key = key[:32]
		}
		if enc, err := encrypt.NewAESEncrypt(key); err == nil {
			providerEncrypt = enc
		}
	}
	if providerEncrypt == nil {
		providerEncrypt = encrypt.NewNoOpEncrypt()
	}

	// Initialize GitHub App client if configured (for installation token-based cloning)
	var githubAppClient jobhandlers.GitHubAppClient
	if cfg.GitHubApp.AppID != 0 && cfg.GitHubApp.PrivateKey != "" {
		githubAppClientInst, err := githubapp.NewClient(githubapp.Config{
			AppID:         cfg.GitHubApp.AppID,
			PrivateKey:    cfg.GitHubApp.PrivateKey,
			WebhookSecret: cfg.GitHubApp.WebhookSecret,
		})
		if err == nil {
			githubAppClient = jobhandlers.NewGitHubAppClientAdapter(githubAppClientInst)
			log.Debug().Msg("GitHub App client initialized for worker")
		} else {
			log.Warn().Err(err).Msg("GitHub App client not initialized (invalid configuration)")
		}
	}

	// Initialize cache service for GC jobs (if cache storage is configured)
	var cacheService *service.CacheService
	if cfg.CacheStorage.Provider != "" && db != nil {
		storageCfg := storage.Config{
			Provider:        storage.ProviderType(cfg.CacheStorage.Provider),
			Bucket:          cfg.CacheStorage.Bucket,
			Region:          cfg.CacheStorage.Region,
			Endpoint:        cfg.CacheStorage.Endpoint,
			AccessKeyID:     cfg.CacheStorage.AccessKeyID,
			SecretAccessKey: cfg.CacheStorage.SecretAccessKey,
			UsePathStyle:    cfg.CacheStorage.UsePathStyle,
			BasePath:        cfg.CacheStorage.BasePath,
			Prefix:          cfg.CacheStorage.Prefix,
			CredentialsFile: cfg.CacheStorage.CredentialsFile,
		}
		cacheBucket, err := storage.NewBucket(storageCfg)
		if err != nil {
			log.Warn().Err(err).Msg("Cache storage not initialized (invalid configuration)")
		} else {
			cacheService = service.NewCacheService(store.Cache, cacheBucket, log.Logger)
			log.Debug().Str("provider", cfg.CacheStorage.Provider).Msg("Cache service initialized for GC")
		}
	}

	// Initialize artifact service (if artifact storage is configured)
	var artifactService *service.ArtifactService
	if cfg.ArtifactStorage.Provider != "" && db != nil {
		storageCfg := storage.Config{
			Provider:        storage.ProviderType(cfg.ArtifactStorage.Provider),
			Bucket:          cfg.ArtifactStorage.Bucket,
			Region:          cfg.ArtifactStorage.Region,
			Endpoint:        cfg.ArtifactStorage.Endpoint,
			AccessKeyID:     cfg.ArtifactStorage.AccessKeyID,
			SecretAccessKey: cfg.ArtifactStorage.SecretAccessKey,
			UsePathStyle:    cfg.ArtifactStorage.UsePathStyle,
			BasePath:        cfg.ArtifactStorage.BasePath,
			Prefix:          cfg.ArtifactStorage.Prefix,
			CredentialsFile: cfg.ArtifactStorage.CredentialsFile,
		}
		artifactBucket, err := storage.NewBucket(storageCfg)
		if err != nil {
			log.Warn().Err(err).Msg("Artifact storage not initialized (invalid configuration)")
		} else {
			artifactService = service.NewArtifactService(store.Artifacts, artifactBucket, log.Logger)
			log.Debug().Str("provider", cfg.ArtifactStorage.Provider).Msg("Artifact service initialized")
		}
	}

	// Validate license key (self-hosted only).
	// In cloud mode, the billing system handles quota/features;
	// license gating is skipped entirely.
	var featureGate *licensing.FeatureGate
	if cfg.CloudMode {
		log.Info().Msg("Cloud mode enabled -- license system disabled")
	} else if cfg.License.Key != "" {
		keys, err := licensing.ParsePublicKeys()
		if err != nil || len(keys) == 0 {
			log.Warn().Err(err).Msg("License keyring unavailable -- running as Community edition")
			featureGate = licensing.NewFeatureGate(nil, log.Logger)
		} else {
			validator := licensing.NewValidator(keys)
			claims, err := validator.Validate(cfg.License.Key)
			if err != nil {
				log.Warn().Err(err).Msg("Invalid license key -- running as Community edition")
				featureGate = licensing.NewFeatureGate(nil, log.Logger)
			} else {
				featureGate = licensing.NewFeatureGate(claims, log.Logger)
				log.Info().
					Str("edition", string(claims.Edition)).
					Str("customer", claims.Subject).
					Int("days_remaining", claims.DaysUntilExpiry()).
					Msg("Worker license validated")
			}
		}
	} else {
		featureGate = licensing.NewFeatureGate(nil, log.Logger)
	}

	// Create job configuration
	// Build container defaults from server config.
	// In cloud mode, containers are always allowed (billing handles limits).
	// In self-hosted mode, container execution requires a Pro or Enterprise license.
	var containerDefaults *jobhandlers.ContainerDefaults
	if cfg.Container.Enabled {
		if cfg.CloudMode {
			containerDefaults = &jobhandlers.ContainerDefaults{
				Enabled:      cfg.Container.Enabled,
				DefaultImage: cfg.Container.DefaultImage,
				MemoryLimit:  cfg.Container.MemoryLimit,
				CPULimit:     cfg.Container.CPULimit,
				Network:      cfg.Container.Network,
			}
		} else if featureGate == nil || !featureGate.HasFeature(licensing.FeatureContainerExecution) {
			log.Warn().Msg("Container execution requires a Pro or Enterprise license -- disabled")
		} else {
			containerDefaults = &jobhandlers.ContainerDefaults{
				Enabled:      cfg.Container.Enabled,
				DefaultImage: cfg.Container.DefaultImage,
				MemoryLimit:  cfg.Container.MemoryLimit,
				CPULimit:     cfg.Container.CPULimit,
				Network:      cfg.Container.Network,
			}
		}
	}

	// Initialize AI repo and config for AI analysis jobs.
	// Always create aiRepo when database is available — the project's dagryn.toml
	// controls whether AI is enabled, not the server config.
	// Server config (aiConfig) serves as the managed-mode fallback.

	var aiConfig *jobhandlers.AIAnalysisConfig
	if db != nil {
		aiConfig = &jobhandlers.AIAnalysisConfig{
			Enabled:               cfg.AI.Enabled,
			BackendMode:           cfg.AI.BackendMode,
			Provider:              cfg.AI.Provider,
			APIKey:                cfg.AI.APIKey,
			TimeoutSeconds:        cfg.AI.TimeoutSeconds,
			MaxTokens:             cfg.AI.MaxTokens,
			AgentEndpoint:         cfg.AI.AgentEndpoint,
			AgentToken:            cfg.AI.AgentToken,
			MaxAnalysesPerHour:    cfg.AI.MaxAnalysesPerHour,
			CooldownSeconds:       cfg.AI.CooldownSeconds,
			MaxConcurrentAnalyses: cfg.AI.MaxConcurrentAnalyses,
		}
		log.Debug().Msg("AI analysis repo and config initialized for worker")
	}

	// Initialize billing repo and related repos for usage rollup, bandwidth reset, and retention jobs
	var stripeClient *dagrynstripe.Client

	if cfg.Stripe.SecretKey != "" {
		stripeClient = dagrynstripe.New(dagrynstripe.Config{
			SecretKey:      cfg.Stripe.SecretKey,
			WebhookSecret:  cfg.Stripe.WebhookSecret,
			PublishableKey: cfg.Stripe.PublishableKey,
		})
		log.Debug().Msg("Stripe client initialized for worker")
	}

	// Initialize quota service for plan enforcement in job handlers

	quotaService := service.NewQuotaService(store.Billing, store.Projects, log.Logger)
	if cacheService != nil {
		cacheService.SetQuotaService(quotaService)
	}
	log.Debug().Msg("Quota enforcement service initialized for worker")

	// Initialize telemetry metrics for job handlers
	var metrics *telemetry.Metrics
	if cfg.Telemetry.Enabled {
		tp, err := telemetry.Init(ctx, cfg.Telemetry)
		if err != nil {
			log.Warn().Err(err).Msg("Telemetry not initialized for worker")
		} else {
			_ = tp // keep reference for shutdown
			m, mErr := telemetry.NewMetrics()
			if mErr != nil {
				log.Warn().Err(mErr).Msg("Metrics instruments not created")
			} else {
				metrics = m
				log.Debug().Msg("Telemetry metrics initialized for worker")
			}
		}
	}

	jobCfg := worker.Config{
		Concurrency:          cfg.Job.Concurrency,
		EncryptionKey:        cfg.Job.EncryptionKey,
		Store:                store,
		ProviderTokenEncrypt: providerEncrypt,
		GitHubAppClient:      githubAppClient,
		CacheService:         cacheService,
		ArtifactService:      artifactService,
		ContainerDefaults:    containerDefaults,
		EventPublisher:       eventPublisher,
		StripeClient:         stripeClient,
		QuotaService:         quotaService,
		AIConfig:             aiConfig,
		Metrics:              metrics,
		BaseURL:              cfg.Server.BaseURL,
	}

	// Create job system
	jobSystem, err := worker.New(jobCfg, ctx, rds)
	if err != nil {
		return err
	}

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		log.Info().Msg("Received shutdown signal, stopping worker...")
		cancel()
		jobSystem.Stop()
	}()

	log.Info().
		Str("redis_host", cfg.Redis.Host).
		Int("redis_port", cfg.Redis.Port).
		Int("concurrency", cfg.Job.Concurrency).
		Msg("Starting Dagryn background worker")

	// Start the job system (asynq Server.Start() returns immediately; we must block so the process stays alive)
	if err := jobSystem.RegisterAndStart(); err != nil {
		return err
	}

	// Block until shutdown signal; the signal handler above will cancel ctx and call jobSystem.Stop()
	<-ctx.Done()
	return nil
}

// applyWorkerEnvVars applies environment variables for worker configuration.
func applyWorkerEnvVars(cfg *config.Config) {
	// Database
	if v := getWorkerEnvAny("DATABASE_URL", "DAGRYN_DATABASE_URL", "POSTGRES_URL"); v != "" {
		cfg.Database.URL = v
	}
	// Redis
	if v := getWorkerEnvAny("REDIS_HOST", "DAGRYN_REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := getWorkerEnvAny("REDIS_PORT", "DAGRYN_REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.Redis.Port = port
		}
	}
	if v := getWorkerEnvAny("REDIS_PASSWORD", "DAGRYN_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	// Job
	if v := getWorkerEnvAny("JOB_ENCRYPTION_KEY", "DAGRYN_JOB_ENCRYPTION_KEY"); v != "" {
		cfg.Job.EncryptionKey = v
	}
	if v := getWorkerEnvAny("JOB_CONCURRENCY", "DAGRYN_JOB_CONCURRENCY"); v != "" {
		if concurrency, err := strconv.Atoi(v); err == nil && concurrency > 0 {
			cfg.Job.Concurrency = concurrency
		}
	}
}

// applyWorkerCLIFlags applies CLI flag overrides for worker configuration.
func applyWorkerCLIFlags(cfg *config.Config, opts WorkerConfigOpts) {
	if opts.RedisHost != "" {
		cfg.Redis.Host = opts.RedisHost
	}
	if opts.RedisPort > 0 {
		cfg.Redis.Port = opts.RedisPort
	}
	if opts.RedisPassword != "" {
		cfg.Redis.Password = opts.RedisPassword
	}
	if opts.Concurrency > 0 {
		cfg.Job.Concurrency = opts.Concurrency
	}
	if opts.EncryptionKey != "" {
		cfg.Job.EncryptionKey = opts.EncryptionKey
	}
}

// getWorkerEnvAny returns the value of the first non-empty environment variable.
func getWorkerEnvAny(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
