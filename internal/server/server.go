// Package server provides the HTTP server for the Dagryn API.
//
// @title Dagryn API
// @version 1.0
// @description Local-first, self-hosted developer workflow orchestrator API
// @termsOfService http://swagger.io/terms/
//
// @contact.name Dagryn Support
// @contact.url https://github.com/mujhtech/dagryn
// @contact.email support@dagryn.dev
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:9000
// @BasePath /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token authentication
//
// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name X-API-Key
// @description API key authentication
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mujhtech/dagryn/internal/db"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/encrypt"
	"github.com/mujhtech/dagryn/internal/githubapp"
	"github.com/mujhtech/dagryn/internal/job"
	"github.com/mujhtech/dagryn/internal/license"
	"github.com/mujhtech/dagryn/internal/redis"
	"github.com/mujhtech/dagryn/internal/server/auth"
	"github.com/mujhtech/dagryn/internal/server/auth/oauth"
	"github.com/mujhtech/dagryn/internal/server/handlers"
	"github.com/mujhtech/dagryn/internal/server/middleware"
	"github.com/mujhtech/dagryn/internal/server/sse"
	"github.com/mujhtech/dagryn/internal/service"
	dagrynstripe "github.com/mujhtech/dagryn/internal/stripe"
	"github.com/mujhtech/dagryn/internal/telemetry"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/rs/zerolog/log"
)

// Server represents the HTTP server.
type Server struct {
	config    *Config
	router    *chi.Mux
	server    *http.Server
	db        *db.DB
	telemetry *telemetry.Provider

	// Repositories
	repos *Repositories

	// GitHub App client (optional; nil when not configured)
	githubApp *githubapp.Client

	// Auth services
	jwtService        *auth.JWTService
	deviceCodeService *auth.DeviceCodeService
	oauthProviders    map[string]oauth.Provider

	// SSE Hub for real-time events
	sseHub *sse.Hub

	// SSE Redis subscriber (nil when Redis is unavailable)
	sseSubscriber *sse.RedisSubscriber
}

// Repositories holds all repository instances.
type Repositories struct {
	Users               *repo.UserRepo
	Tokens              *repo.TokenRepo
	Teams               *repo.TeamRepo
	Projects            *repo.ProjectRepo
	APIKeys             *repo.APIKeyRepo
	Invitations         *repo.InvitationRepo
	Runs                *repo.RunRepo
	Artifacts           *repo.ArtifactRepo
	ProviderTokens      *repo.ProviderTokenRepo
	GitHubInstallations *repo.GitHubInstallationRepo
	Workflows           *repo.WorkflowRepo
	PluginRegistry      *repo.PluginRegistryRepo
	Billing             *repo.BillingRepo
}

// New creates a new server instance.
func New(cfg *Config) *Server {
	router := chi.NewRouter()

	return &Server{
		config: cfg,
		router: router,
		server: &http.Server{
			Addr:         cfg.Server.Address(),
			Handler:      router,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
	}
}

// Initialize sets up the server with all dependencies.
func (s *Server) Initialize(ctx context.Context) error {
	// Initialize telemetry
	if s.config.Telemetry.Enabled {
		tp, err := telemetry.Init(ctx, s.config.Telemetry)
		if err != nil {
			return fmt.Errorf("failed to initialize telemetry: %w", err)
		}
		s.telemetry = tp
	}

	// Connect to database
	database, err := db.New(ctx, s.config.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	s.db = database

	// Initialize repositories
	s.repos = &Repositories{
		Users:               repo.NewUserRepo(database.Pool()),
		Tokens:              repo.NewTokenRepo(database.Pool()),
		Teams:               repo.NewTeamRepo(database.Pool()),
		Projects:            repo.NewProjectRepo(database.Pool()),
		APIKeys:             repo.NewAPIKeyRepo(database.Pool()),
		Invitations:         repo.NewInvitationRepo(database.Pool()),
		Runs:                repo.NewRunRepo(database.Pool()),
		Artifacts:           repo.NewArtifactRepo(database.Pool()),
		ProviderTokens:      repo.NewProviderTokenRepo(database.Pool()),
		GitHubInstallations: repo.NewGitHubInstallationRepo(database.Pool()),
		Workflows:           repo.NewWorkflowRepo(database.Pool()),
		PluginRegistry:      repo.NewPluginRegistryRepo(database.Pool()),
		Billing:             repo.NewBillingRepo(database.Pool()),
	}

	// Initialize GitHub App client if configured
	if s.config.GitHubApp.AppID != 0 && s.config.GitHubApp.PrivateKey != "" {
		if client, err := githubapp.NewClient(githubapp.Config{
			AppID:         s.config.GitHubApp.AppID,
			PrivateKey:    s.config.GitHubApp.PrivateKey,
			WebhookSecret: s.config.GitHubApp.WebhookSecret,
		}); err != nil {
			log.Warn().Err(err).Msg("GitHub App client not initialized (invalid configuration)")
		} else {
			s.githubApp = client
			log.Debug().Msg("GitHub App client initialized")
		}
	}

	// Initialize auth services
	if err := s.initAuthServices(); err != nil {
		return fmt.Errorf("failed to initialize auth services: %w", err)
	}

	// Initialize SSE hub for real-time events
	s.sseHub = sse.NewHub()
	go s.sseHub.Run()
	log.Debug().Msg("SSE hub started")

	// Optional Redis (for job client and/or ready check)
	var rds *redis.Redis
	if s.config.Redis.Host != "" || s.config.Redis.Port != 0 {
		rds = redis.New(s.config.Redis)
	}

	// Start SSE Redis subscriber to receive events published by the worker
	if rds != nil {
		s.sseSubscriber = sse.NewRedisSubscriber(rds, s.sseHub)
		s.sseSubscriber.Start(ctx)
		log.Debug().Msg("SSE Redis subscriber started")
	}

	// Optional job client for enqueueing ExecuteRun
	var jobClient *job.Client
	if rds != nil {
		var enc encrypt.Encrypt
		if s.config.Job.EncryptionKey != "" {
			var err error
			enc, err = encrypt.NewAESEncrypt(s.config.Job.EncryptionKey)
			if err != nil {
				return fmt.Errorf("job encryption: %w", err)
			}
		} else {
			enc = encrypt.NewNoOpEncrypt()
		}
		jobClient = job.NewClient(rds, enc)
		log.Debug().Msg("Job client initialized for server-triggered runs")
	}

	// Redis readiness checker (implements handlers.ReadyChecker)
	var redisForReady handlers.ReadyChecker
	if s.config.Health.ReadyCheckRedis && rds != nil {
		redisForReady = &redisReadyChecker{rds}
	}

	// Provider token encrypter (for storing GitHub token; use first 32 bytes of JWT secret)
	providerEncKey := s.config.Auth.JWTSecret
	if len(providerEncKey) > 32 {
		providerEncKey = providerEncKey[:32]
	}
	var providerEnc encrypt.Encrypt
	if len(providerEncKey) >= 16 {
		if enc, err := encrypt.NewAESEncrypt(providerEncKey); err == nil {
			providerEnc = enc
		}
	}
	if providerEnc == nil {
		providerEnc = encrypt.NewNoOpEncrypt()
	}

	// Optional cache service (when cache storage is configured)
	var cacheService *service.CacheService
	if s.config.CacheStorage.Provider != "" {
		storageCfg := storage.Config{
			Provider:        storage.ProviderType(s.config.CacheStorage.Provider),
			Bucket:          s.config.CacheStorage.Bucket,
			Region:          s.config.CacheStorage.Region,
			Endpoint:        s.config.CacheStorage.Endpoint,
			AccessKeyID:     s.config.CacheStorage.AccessKeyID,
			SecretAccessKey: s.config.CacheStorage.SecretAccessKey,
			UsePathStyle:    s.config.CacheStorage.UsePathStyle,
			BasePath:        s.config.CacheStorage.BasePath,
			Prefix:          s.config.CacheStorage.Prefix,
			CredentialsFile: s.config.CacheStorage.CredentialsFile,
		}
		cacheBucket, err := storage.NewBucket(storageCfg)
		if err != nil {
			log.Warn().Err(err).Msg("Cache storage not initialized (invalid configuration)")
		}

		cacheRepo := repo.NewCacheRepo(database.Pool())
		cacheService = service.NewCacheService(cacheRepo, cacheBucket, log.Logger)
		log.Debug().Str("provider", s.config.CacheStorage.Provider).Msg("Cache service initialized")
	}

	// Optional artifact service (when artifact storage is configured)
	var artifactService *service.ArtifactService
	if s.config.ArtifactStorage.Provider != "" {
		storageCfg := storage.Config{
			Provider:        storage.ProviderType(s.config.ArtifactStorage.Provider),
			Bucket:          s.config.ArtifactStorage.Bucket,
			Region:          s.config.ArtifactStorage.Region,
			Endpoint:        s.config.ArtifactStorage.Endpoint,
			AccessKeyID:     s.config.ArtifactStorage.AccessKeyID,
			SecretAccessKey: s.config.ArtifactStorage.SecretAccessKey,
			UsePathStyle:    s.config.ArtifactStorage.UsePathStyle,
			BasePath:        s.config.ArtifactStorage.BasePath,
			Prefix:          s.config.ArtifactStorage.Prefix,
			CredentialsFile: s.config.ArtifactStorage.CredentialsFile,
		}
		artifactBucket, err := storage.NewBucket(storageCfg)
		if err != nil {
			log.Warn().Err(err).Msg("Artifact storage not initialized (invalid configuration)")
		}
		if artifactBucket != nil {
			artifactService = service.NewArtifactService(s.repos.Artifacts, artifactBucket, log.Logger)
			log.Debug().Str("provider", s.config.ArtifactStorage.Provider).Msg("Artifact service initialized")
		}
	}

	var cancelManager *job.CancelManager
	if rds != nil {
		cancelManager = job.NewCancelManager(rds)
	}

	// Initialize plugin registry service (optional)
	var registryService *service.PluginRegistryService
	if s.repos.PluginRegistry != nil {
		registryService = service.NewPluginRegistryService(s.repos.PluginRegistry, log.Logger)
	}

	// Optional quota service (when billing repo is available)
	var quotaService *service.QuotaService
	if s.repos.Billing != nil {
		quotaService = service.NewQuotaService(s.repos.Billing, s.repos.Projects, log.Logger)
		if cacheService != nil {
			cacheService.SetQuotaService(quotaService)
		}
		if artifactService != nil {
			artifactService.SetQuotaService(quotaService)
		}
		log.Debug().Msg("Quota enforcement service initialized")
	}

	// Optional Stripe client and billing service
	var stripeClient *dagrynstripe.Client
	var billingService *service.BillingService
	if s.config.Stripe.SecretKey != "" {
		stripeClient = dagrynstripe.New(dagrynstripe.Config{
			SecretKey:      s.config.Stripe.SecretKey,
			WebhookSecret:  s.config.Stripe.WebhookSecret,
			PublishableKey: s.config.Stripe.PublishableKey,
		})
		billingService = service.NewBillingService(s.repos.Billing, stripeClient, log.Logger)
		log.Debug().Msg("Stripe billing service initialized")
	}

	// Validate license key (self-hosted only).
	// In cloud mode, the billing system handles all quota/feature gating;
	// the license system is not used and featureGate remains nil.
	var featureGate *license.FeatureGate
	if s.config.CloudMode {
		log.Info().Msg("Cloud mode enabled -- license system disabled")
	} else if s.config.License.Key != "" {
		keys, err := license.ParsePublicKeys()
		if err != nil {
			log.Warn().Err(err).Msg("Invalid license keyring -- running as Community edition")
			featureGate = license.NewFeatureGate(nil, log.Logger)
		} else if len(keys) == 0 {
			log.Warn().Msg("No license public keys embedded in binary -- running as Community edition")
			featureGate = license.NewFeatureGate(nil, log.Logger)
		} else {
			validator := license.NewValidator(keys)
			claims, err := validator.Validate(s.config.License.Key)
			if err != nil {
				log.Warn().Err(err).Msg("Invalid license key -- running as Community edition")
				featureGate = license.NewFeatureGate(nil, log.Logger)
			} else {
				featureGate = license.NewFeatureGate(claims, log.Logger)
				log.Info().
					Str("edition", string(claims.Edition)).
					Str("customer", claims.Subject).
					Int("seats", claims.Seats).
					Int("days_remaining", claims.DaysUntilExpiry()).
					Msg("License validated")

				if featureGate.IsExpiring() {
					log.Warn().
						Int("days_remaining", claims.DaysUntilExpiry()).
						Msg("License expiring soon -- please renew")
				}
				if featureGate.InGracePeriod() {
					log.Warn().Msg("License expired -- running in grace period, features will be disabled soon")
				}
			}
		}
	} else {
		featureGate = license.NewFeatureGate(nil, log.Logger)
		log.Info().Msg("No license key configured -- running as Community edition")
	}

	// Create handlers
	h := handlers.New(
		s.db, s.repos.Users, s.repos.Tokens, s.repos.Teams, s.repos.Projects,
		s.repos.APIKeys, s.repos.Invitations, s.repos.Runs, s.sseHub, jobClient,
		s.repos.ProviderTokens, providerEnc,
		s.config.Health.ReadyCheckDatabase,
		s.config.Health.ReadyCheckRedis,
		redisForReady,
		s.githubApp,
		s.repos.GitHubInstallations,
		s.repos.Workflows,
		cacheService,
		artifactService,
		cancelManager,
		registryService,
		billingService,
		stripeClient,
		quotaService,
	)

	// Set cloud mode and license feature gate
	h.SetCloudMode(s.config.CloudMode)
	h.SetFeatureGate(featureGate)

	// Create auth handler
	authHandler := handlers.NewAuthHandler(
		s.jwtService,
		s.deviceCodeService,
		s.repos.Users,
		s.repos.ProviderTokens,
		providerEnc,
		s.oauthProviders,
		fmt.Sprintf("http://%s", s.config.Server.Address()),
	)

	// Setup middleware
	s.setupMiddleware()

	// Create auth middleware
	authMiddleware := s.createAuthMiddleware()

	// Setup routes
	s.setupRoutes(h, authHandler, authMiddleware, jobClient, featureGate)

	log.Info().
		Str("addr", s.config.Server.Address()).
		Bool("swagger", s.config.Server.Swagger.Enabled).
		Bool("telemetry", s.config.Telemetry.Enabled).
		Int("oauth_providers", len(s.oauthProviders)).
		Msg("Server initialized")

	return nil
}

// initAuthServices initializes all authentication services.
func (s *Server) initAuthServices() error {
	// Initialize JWT service
	jwtConfig := auth.JWTConfig{
		Secret:        s.config.Auth.JWTSecret,
		AccessExpiry:  s.config.Auth.JWTAccessExpiry,
		RefreshExpiry: s.config.Auth.JWTRefreshExpiry,
		Issuer:        "dagryn",
	}
	s.jwtService = auth.NewJWTService(jwtConfig, s.repos.Tokens)

	// Initialize device code service
	deviceCodeConfig := auth.DeviceCodeConfig{
		Expiry:          15 * time.Minute,
		PollInterval:    5 * time.Second,
		VerificationURI: fmt.Sprintf("http://%s/auth/device", s.config.Server.Address()),
	}
	s.deviceCodeService = auth.NewDeviceCodeService(s.db.Pool(), deviceCodeConfig)

	// Initialize OAuth providers
	s.oauthProviders = make(map[string]oauth.Provider)

	// GitHub provider (repo scope for listing repos / Import from GitHub)
	if s.config.OAuth.GitHub.ClientID != "" && s.config.OAuth.GitHub.ClientSecret != "" {
		s.oauthProviders["github"] = oauth.NewGitHubProvider(oauth.Config{
			ClientID:     s.config.OAuth.GitHub.ClientID,
			ClientSecret: s.config.OAuth.GitHub.ClientSecret,
			RedirectURL:  fmt.Sprintf("http://%s/auth/github/callback", s.config.Server.Address()),
			Scopes:       []string{"read:user", "user:email", "repo"},
		})
		log.Debug().Msg("GitHub OAuth provider initialized")
	}

	// Google provider
	if s.config.OAuth.Google.ClientID != "" && s.config.OAuth.Google.ClientSecret != "" {
		s.oauthProviders["google"] = oauth.NewGoogleProvider(oauth.Config{
			ClientID:     s.config.OAuth.Google.ClientID,
			ClientSecret: s.config.OAuth.Google.ClientSecret,
			RedirectURL:  fmt.Sprintf("http://%s/api/v1/auth/google/callback", s.config.Server.Address()),
			Scopes:       []string{"openid", "email", "profile"},
		})
		log.Debug().Msg("Google OAuth provider initialized")
	}

	if len(s.oauthProviders) == 0 {
		log.Warn().Msg("No OAuth providers configured - authentication will be limited")
	}

	return nil
}

// setupMiddleware configures all middleware.
func (s *Server) setupMiddleware() {
	// Request ID
	s.router.Use(middleware.RequestID)

	// Real IP
	s.router.Use(middleware.RealIP)

	// Logging
	s.router.Use(middleware.Logger)

	// Recoverer
	s.router.Use(middleware.Recoverer)

	// OpenTelemetry
	if s.config.Telemetry.Enabled {
		s.router.Use(middleware.OTel("dagryn-api"))
	}

	// CORS
	s.router.Use(middleware.CORS())

	// Timeout
	s.router.Use(middleware.Timeout(s.config.Server.WriteTimeout))
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Info().
		Str("addr", s.config.Server.Address()).
		Msg("Starting server")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.Server.ShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error shutting down HTTP server")
	}

	// Stop SSE Redis subscriber before stopping the hub
	if s.sseSubscriber != nil {
		s.sseSubscriber.Stop()
		log.Debug().Msg("SSE Redis subscriber stopped")
	}

	// Stop SSE hub
	if s.sseHub != nil {
		s.sseHub.Stop()
		log.Debug().Msg("SSE hub stopped")
	}

	// Close database connection
	if s.db != nil {
		s.db.Close()
	}

	// Shutdown telemetry
	if s.telemetry != nil {
		if err := s.telemetry.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error shutting down telemetry")
		}
	}

	log.Info().Msg("Server shutdown complete")
	return nil
}

// Router returns the chi router for testing.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// DB returns the database connection.
func (s *Server) DB() *db.DB {
	return s.db
}

// Config returns the server configuration.
func (s *Server) Config() *Config {
	return s.config
}

// redisReadyChecker adapts *redis.Redis to handlers.ReadyChecker for /ready.
type redisReadyChecker struct {
	r *redis.Redis
}

func (c *redisReadyChecker) Ready(ctx context.Context) error {
	return c.r.Connect(ctx)
}

// WaitForReady waits for the server to be ready to accept connections.
func (s *Server) WaitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("http://%s/health", s.config.Server.Address())

	client := &http.Client{Timeout: time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resp, err := client.Get(addr)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	return fmt.Errorf("server not ready after %v", timeout)
}
