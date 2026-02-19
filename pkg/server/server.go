// Package server provides the HTTP server for the Dagryn API.
//
//	@title						Dagryn API
//	@version					1.0
//	@description				Local-first, self-hosted developer workflow orchestrator API
//	@termsOfService				http://swagger.io/terms/
//
//	@contact.name				Dagryn Support
//	@contact.url				https://github.com/mujhtech/dagryn
//	@contact.email				support@dagryn.dev
//
//	@license.name				MIT
//	@license.url				https://opensource.org/licenses/MIT
//
//	@host						localhost:9000
//	@BasePath					/api/v1
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT Bearer token authentication
//
//	@securityDefinitions.apikey	APIKeyAuth
//	@in							header
//	@name						X-API-Key
//	@description				API key authentication
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mujhtech/dagryn/pkg/api"
	"github.com/mujhtech/dagryn/pkg/api/handlers"
	"github.com/mujhtech/dagryn/pkg/authn"
	"github.com/mujhtech/dagryn/pkg/authz"
	"github.com/mujhtech/dagryn/pkg/cache"
	"github.com/mujhtech/dagryn/pkg/config"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/githubapp"
	"github.com/mujhtech/dagryn/pkg/redis"
	"github.com/mujhtech/dagryn/pkg/server/sse"
	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/mujhtech/dagryn/pkg/worker"
	"github.com/rs/zerolog/log"
)

// Server represents the HTTP server.
type Server struct {
	config            *config.Config
	router            *chi.Mux
	server            *http.Server
	telemetry         *telemetry.Provider
	db                *database.DB
	store             store.Store
	githubApp         *githubapp.Client
	jwtService        *authz.JWTService
	deviceCodeService *authz.DeviceCodeService
	oauthProviders    map[string]authn.Provider
	sseHub            *sse.Hub
	sseSubscriber     *sse.RedisSubscriber
	entitlements      entitlement.Checker
	extraRoutes       func(chi.Router)
	dashboardHandler  http.Handler
}

// SetEntitlementChecker allows an external binary to override the default
// Must be called before Initialize.
func (s *Server) SetEntitlementChecker(c entitlement.Checker) {
	s.entitlements = c
}

// RegisterExtraRoutes allows an external binary to add routes
// Must be called before Initialize.
func (s *Server) RegisterExtraRoutes(fn func(chi.Router)) {
	s.extraRoutes = fn
}

// SetDashboardHandler allows an external binary to override the default
// Must be called before Initialize.
func (s *Server) SetDashboardHandler(h http.Handler) {
	s.dashboardHandler = h
}

// SetDatabase allows an external binary to inject a pre-created database
// connection. Must be called before Initialize.
func (s *Server) SetDatabase(db *database.DB) {
	s.db = db
}

// New creates a new server instance.
func New(cfg *config.Config) *Server {
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

	// Connect to database (skip if already injected via SetDatabase)
	if s.db == nil {
		db, err := database.New(ctx, s.config.Database)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		s.db = db
	}

	rds := redis.New(s.config.Redis)

	cache, err := cache.NewCache(s.config, rds)

	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}

	// Initialize repositories
	s.store = store.New(cache, s.db)

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

	// Start SSE Redis subscriber to receive events published by the worker
	s.sseSubscriber = sse.NewRedisSubscriber(rds, s.sseHub)
	s.sseSubscriber.Start(ctx)
	log.Debug().Msg("SSE Redis subscriber started")

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
	jobClient := worker.NewClient(rds, enc)
	log.Debug().Msg("Job client initialized for server-triggered runs")

	// Redis readiness checker (implements handlers.ReadyChecker)
	var redisForReady handlers.ReadyChecker
	if s.config.Health.ReadyCheckRedis && rds != nil {
		redisForReady = &redisReadyChecker{rds}
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

		cacheService = service.NewCacheService(s.store.Cache, cacheBucket, log.Logger)
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
			artifactService = service.NewArtifactService(s.store.Artifacts, artifactBucket, log.Logger)
			log.Debug().Str("provider", s.config.ArtifactStorage.Provider).Msg("Artifact service initialized")
		}
	}

	cancelManager := worker.NewCancelManager(rds)

	registryService := service.NewPluginRegistryService(s.store.PluginRegistry, log.Logger)

	featureGate := service.NewLicensingService(s.config.License.Key, s.entitlements)

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

	// Create handlers
	apiHandler, err := api.New(
		s.config,
		jobClient,
		s.telemetry,
		s.db,
		s.store,
		s.githubApp,
		s.jwtService,
		s.deviceCodeService,
		s.oauthProviders,
		s.sseHub,
		providerEnc,
		s.config.Health.ReadyCheckDatabase,
		s.config.Health.ReadyCheckRedis,
		redisForReady,
		cacheService,
		artifactService,
		cancelManager,
		registryService,
	)
	if err != nil {
		return fmt.Errorf("create API handlers: %w", err)
	}

	apiHandler.SetFeatureGate(featureGate)

	// Wire the unified entitlement checker.
	// If the cloud binary has already injected one via SetEntitlementChecker,
	// use that. Otherwise default to the license-backed checker.
	var checker entitlement.Checker
	if s.entitlements != nil {
		checker = s.entitlements
	} else {
		checker = entitlement.NewLicenseChecker(featureGate)
	}
	apiHandler.SetEntitlementChecker(checker)

	// Wire dashboard override from the cloud binary.
	if s.dashboardHandler != nil {
		apiHandler.SetDashboardHandler(s.dashboardHandler)
	}

	// Wire entitlements to services for quota enforcement.
	if cacheService != nil {
		cacheService.SetEntitlements(checker)
	}
	if artifactService != nil {
		artifactService.SetEntitlements(checker)
	}

	// Wire extra routes from the cloud binary (e.g. billing routes).
	if s.extraRoutes != nil {
		apiHandler.RegisterExtraRoutes(s.extraRoutes)
	}

	// Setup routes
	apiHandler.BuildRouter(s.router)

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
	jwtConfig := authz.JWTConfig{
		Secret:        s.config.Auth.JWTSecret,
		AccessExpiry:  s.config.Auth.JWTAccessExpiry,
		RefreshExpiry: s.config.Auth.JWTRefreshExpiry,
		Issuer:        "dagryn",
	}
	s.jwtService = authz.NewJWTService(jwtConfig, s.store.Tokens)

	// Initialize device code service
	deviceCodeConfig := authz.DeviceCodeConfig{
		Expiry:          15 * time.Minute,
		PollInterval:    5 * time.Second,
		VerificationURI: fmt.Sprintf("http://%s/auth/device", s.config.Server.Address()),
	}
	s.deviceCodeService = authz.NewDeviceCodeService(s.db.Pool(), deviceCodeConfig)

	// Initialize OAuth providers
	s.oauthProviders = make(map[string]authn.Provider)

	// GitHub provider (repo scope for listing repos / Import from GitHub)
	if s.config.OAuth.GitHub.ClientID != "" && s.config.OAuth.GitHub.ClientSecret != "" {
		s.oauthProviders["github"] = authn.NewGitHubProvider(authn.Config{
			ClientID:     s.config.OAuth.GitHub.ClientID,
			ClientSecret: s.config.OAuth.GitHub.ClientSecret,
			RedirectURL:  fmt.Sprintf("http://%s/auth/github/callback", s.config.Server.Address()),
			Scopes:       []string{"read:user", "user:email", "repo"},
		})
		log.Debug().Msg("GitHub OAuth provider initialized")
	}

	// Google provider
	if s.config.OAuth.Google.ClientID != "" && s.config.OAuth.Google.ClientSecret != "" {
		s.oauthProviders["google"] = authn.NewGoogleProvider(authn.Config{
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
func (s *Server) DB() *database.DB {
	return s.db
}

// Config returns the server configuration.
func (s *Server) Config() *config.Config {
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
