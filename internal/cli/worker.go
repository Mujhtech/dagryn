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

	"github.com/mujhtech/dagryn/internal/db"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/encrypt"
	"github.com/mujhtech/dagryn/internal/githubapp"
	"github.com/mujhtech/dagryn/internal/job"
	jobhandlers "github.com/mujhtech/dagryn/internal/job/handlers"
	"github.com/mujhtech/dagryn/internal/redis"
	"github.com/mujhtech/dagryn/internal/server"
	"github.com/mujhtech/dagryn/internal/service"
	"github.com/mujhtech/dagryn/pkg/storage"
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
	serverOpts := server.ConfigOpts{
		ConfigFile: opts.ConfigFile,
	}
	cfg, err := server.LoadConfig(serverOpts)
	if err != nil {
		// Worker doesn't need OAuth, so we can proceed even if validation fails
		// for OAuth-related fields. Load defaults and apply env vars manually.
		cfg = &server.Config{}
		*cfg = server.DefaultConfig()
		applyWorkerEnvVars(cfg)
	}

	// Apply CLI flag overrides
	applyWorkerCLIFlags(cfg, opts)

	// Create context with cancellation (used for DB and job system)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create Redis client
	rds := redis.New(cfg.Redis)

	// Connect to database for RunRepo, ProjectRepo, ProviderTokenRepo (required for ExecuteRun and stale_runs)
	var database *db.DB
	var runRepo *repo.RunRepo
	var projectRepo *repo.ProjectRepo
	var providerTokenRepo *repo.ProviderTokenRepo
	var githubInstallations *repo.GitHubInstallationRepo
	if cfg.Database.URL != "" {
		var err error
		database, err = db.New(ctx, cfg.Database)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		pool := database.Pool()
		runRepo = repo.NewRunRepo(pool)
		projectRepo = repo.NewProjectRepo(pool)
		providerTokenRepo = repo.NewProviderTokenRepo(pool)
		githubInstallations = repo.NewGitHubInstallationRepo(pool)
	}

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
	if cfg.CacheStorage.Provider != "" && database != nil {
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
			cacheRepo := repo.NewCacheRepo(database.Pool())
			cacheService = service.NewCacheService(cacheRepo, cacheBucket, log.Logger)
			log.Debug().Str("provider", cfg.CacheStorage.Provider).Msg("Cache service initialized for GC")
		}
	}

	// Create job configuration
	jobCfg := job.Config{
		Concurrency:          cfg.Job.Concurrency,
		EncryptionKey:        cfg.Job.EncryptionKey,
		RunRepo:              runRepo,
		ProjectRepo:          projectRepo,
		ProviderTokenRepo:    providerTokenRepo,
		ProviderTokenEncrypt: providerEncrypt,
		GitHubAppClient:      githubAppClient,
		GitHubInstallations:  githubInstallations,
		CacheService:         cacheService,
	}

	// Create job system
	jobSystem, err := job.New(jobCfg, ctx, rds)
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
func applyWorkerEnvVars(cfg *server.Config) {
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
func applyWorkerCLIFlags(cfg *server.Config, opts WorkerConfigOpts) {
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
