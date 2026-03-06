package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mujhtech/dagryn/pkg/api/handlers"
	"github.com/mujhtech/dagryn/pkg/api/middleware"
	"github.com/mujhtech/dagryn/pkg/authn"
	"github.com/mujhtech/dagryn/pkg/authz"
	"github.com/mujhtech/dagryn/pkg/config"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/githubapp"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/mujhtech/dagryn/pkg/server/dashboard"
	"github.com/mujhtech/dagryn/pkg/server/sse"
	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/mujhtech/dagryn/pkg/telemetry"
	"github.com/mujhtech/dagryn/pkg/worker"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type API struct {
	cfg               *config.Config
	jobClient         *worker.Client
	telemetry         *telemetry.Provider
	h                 *handlers.Handler
	db                *database.DB
	store             store.Store
	githubApp         *githubapp.Client
	jwtService        *authz.JWTService
	deviceCodeService *authz.DeviceCodeService
	oauthProviders    map[string]authn.Provider
	providerEnc       encrypt.Encrypt
	entitlements      entitlement.Checker
	featureGate       *licensing.FeatureGate // Used for detailed license info in /license endpoint.
	extraRoutes       func(chi.Router)       // Injected by cloud binary via RegisterExtraRoutes.
	dashboardHandler  http.Handler           // Overrides default embedded dashboard (set by cloud binary).
}

func New(
	cfg *config.Config,
	jobClient *worker.Client,
	telemetry *telemetry.Provider,
	db *database.DB,
	store store.Store,
	githubApp *githubapp.Client,
	jwtService *authz.JWTService,
	deviceCodeService *authz.DeviceCodeService,
	oauthProviders map[string]authn.Provider,
	sseHub *sse.Hub,
	providerEncrypt encrypt.Encrypt,
	readyCheckDatabase bool,
	readyCheckRedis bool,
	redisForReady handlers.ReadyChecker,
	cacheService *service.CacheService,
	artifactService *service.ArtifactService,
	cancelManager *worker.CancelManager,
	registryService *service.PluginRegistryService,
) (*API, error) {

	h, err := handlers.New(
		db,
		store,
		sseHub,
		jobClient,
		providerEncrypt,
		readyCheckDatabase,
		readyCheckRedis,
		redisForReady,
		githubApp,
		cacheService,
		artifactService,
		cancelManager,
		registryService,
		cfg.Server.BaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("create handlers: %w", err)
	}

	return &API{
		cfg:               cfg,
		jobClient:         jobClient,
		telemetry:         telemetry,
		h:                 h,
		store:             store,
		db:                db,
		githubApp:         githubApp,
		providerEnc:       providerEncrypt,
		jwtService:        jwtService,
		deviceCodeService: deviceCodeService,
		oauthProviders:    oauthProviders,
	}, nil
}

// SetEntitlementChecker sets the unified entitlement checker for both
// middleware and handlers. In the OSS binary this is a LicenseChecker;
// the cloud binary overrides it with a BillingChecker.
func (a *API) SetEntitlementChecker(c entitlement.Checker) {
	a.entitlements = c
	a.h.SetEntitlementChecker(c)
}

// RegisterExtraRoutes allows an external binary (e.g. dagryn-cloud) to
// inject additional routes (e.g. /billing/*) into the protected API group.
// Must be called before BuildRouter.
func (a *API) RegisterExtraRoutes(fn func(chi.Router)) {
	a.extraRoutes = fn
}

// SetDashboardHandler allows an external binary to override the default
// embedded dashboard handler.
func (a *API) SetDashboardHandler(h http.Handler) {
	a.dashboardHandler = h
}

// SetFeatureGate delegates to the handler for detailed license info.
func (a *API) SetFeatureGate(gate *licensing.FeatureGate) {
	a.featureGate = gate
	a.h.SetFeatureGate(gate)
}

func (a *API) BuildRouter(router *chi.Mux) *chi.Mux {

	authMiddleware := a.createAuthMiddleware()

	// Create auth handler
	authHandler := handlers.NewAuthHandler(
		a.jwtService,
		a.deviceCodeService,
		a.store.Users,
		a.store.ProviderTokens,
		a.providerEnc,
		a.oauthProviders,
		fmt.Sprintf("http://%s", a.cfg.Server.Address()),
	)

	// Request ID
	router.Use(middleware.RequestID)

	// Real IP
	router.Use(middleware.RealIP)

	// Logging
	router.Use(middleware.Logger)

	// Recoverer
	router.Use(middleware.Recoverer)

	// OpenTelemetry
	if a.cfg.Telemetry.Enabled {
		router.Use(middleware.OTel("dagryn-api"))
	}

	// CORS
	router.Use(middleware.CORS())

	// Timeout — use a shorter timeout for normal requests and a longer
	// one for uploads/downloads/SSE so large file transfers aren't killed.
	uploadTimeout := a.cfg.Server.UploadTimeout
	if uploadTimeout == 0 {
		uploadTimeout = 10 * time.Minute
	}
	router.Use(middleware.AdaptiveTimeout(30*time.Second, uploadTimeout))

	// For OSS, we need to wrap this in license checker only enterprise edition will have access to this route
	workerRoutePrefix := a.cfg.Worker.RoutePrefix
	if workerRoutePrefix == "" {
		workerRoutePrefix = "worker"
	}

	// Swagger documentation
	if a.cfg.Server.Swagger.Enabled {
		swaggerPath := a.cfg.Server.Swagger.Path
		if swaggerPath == "" {
			swaggerPath = "/swagger"
		}
		router.Get(swaggerPath+"/*", httpSwagger.Handler(
			httpSwagger.URL(swaggerPath+"/doc.json"),
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("list"),
			httpSwagger.DomID("swagger-ui"),
		))
	}

	// Queue monitoring (asynqmon) - only when job client is configured
	router.Route(fmt.Sprintf("/%s", workerRoutePrefix), func(r chi.Router) {
		r.Handle("/monitoring/*", a.jobClient.Monitor())
	})

	// Metrics endpoint (if prometheus is enabled and on same port)
	if a.telemetry != nil && a.cfg.Telemetry.Metrics.Enabled {
		metricsHandler := a.telemetry.MetricsHandler()
		if metricsHandler != nil {
			router.Handle("/metrics", metricsHandler)
		}
	}

	// Health check (no auth required)
	router.Get("/health", a.h.Health)
	router.Get("/ready", a.h.Ready)

	// API v1 routes
	router.Route("/api/v1", func(r chi.Router) {
		// Public capabilities endpoint — used by the dashboard to
		// discover mode, features, and visible nav items before auth.
		r.Get("/capabilities", a.h.GetCapabilities)

		// Public workflow tooling endpoints.
		r.Post("/workflows/translate", a.h.TranslateGitHubWorkflowYAML)

		// Public template endpoints.
		r.Get("/templates/sample", a.h.GetSampleTemplate)

		// Provider webhooks (public, no auth) - GitHub, GitLab, Bitbucket.
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/github", a.h.GitHubWebhook)
		})

		// Auth routes (mixed public and protected)
		r.Route("/auth", func(r chi.Router) {
			// Public auth endpoints
			r.Get("/providers", authHandler.ListProviders)
			r.Get("/{provider}", authHandler.StartOAuth)
			r.Post("/{provider}/callback", authHandler.OAuthCallback)
			r.Post("/refresh", authHandler.RefreshToken)

			// Device code flow (for CLI) - public
			r.Post("/device", authHandler.RequestDeviceCode)
			r.Post("/device/poll", authHandler.PollDeviceCode)

			// Protected auth endpoints (require authentication)
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware)

				r.Post("/logout", authHandler.Logout)

				// Device authorization (user approves in browser)
				r.Post("/device/authorize", authHandler.AuthorizeDevice)
				r.Post("/device/deny", authHandler.DenyDevice)
			})
		})

		// Protected routes (auth required)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			// User routes
			r.Route("/users", func(r chi.Router) {
				r.Get("/me", a.h.GetCurrentUser)
				r.Patch("/me", a.h.UpdateCurrentUser)
			})

			// Dashboard overview
			r.Get("/dashboard/overview", a.h.GetDashboardOverview)

			// Team routes
			r.Route("/teams", func(r chi.Router) {
				r.Get("/", a.h.ListTeams)
				r.Post("/", a.h.CreateTeam)
				r.Route("/{teamID}", func(r chi.Router) {
					r.Get("/", a.h.GetTeam)
					r.Patch("/", a.h.UpdateTeam)
					r.Delete("/", a.h.DeleteTeam)

					// Team members
					r.Route("/members", func(r chi.Router) {
						r.Get("/", a.h.ListTeamMembers)
						r.Post("/", a.h.AddTeamMember)
						r.Delete("/{userID}", a.h.RemoveTeamMember)
						r.Patch("/{userID}/role", a.h.UpdateTeamMemberRole)
					})

					// Team invitations
					r.Route("/invitations", func(r chi.Router) {
						r.Get("/", a.h.ListTeamInvitations)
						r.Post("/", a.h.CreateTeamInvitation)
						r.Delete("/{invitationID}", a.h.RevokeTeamInvitation)
					})
				})
			})

			// Provider routes (e.g. list repos for Import from GitHub)
			r.Route("/providers", func(r chi.Router) {
				r.Route("/github", func(r chi.Router) {
					r.Get("/repos", a.h.ListGitHubRepos)
					r.Post("/workflows/translate", a.h.TranslateGitHubWorkflows)
					r.Route("/app", func(r chi.Router) {
						r.Get("/installations", a.h.ListGitHubAppInstallations)
						r.Get(fmt.Sprintf("/installations/{%s}/repos", handlers.InstallationIDParam), a.h.ListGitHubAppRepos)
					})
				})
			})

			// Project routes
			r.Route("/projects", func(r chi.Router) {
				r.Get("/", a.h.ListProjects)
				r.Post("/", a.h.CreateProject)
				r.Route(fmt.Sprintf("/{%s}", handlers.ProjectIDParam), func(r chi.Router) {
					r.Get("/", a.h.GetProject)
					r.Patch("/", a.h.UpdateProject)
					r.Delete("/", a.h.DeleteProject)
					r.Post("/connect-github", a.h.ConnectProjectToGitHub)

					// Project members
					r.Route("/members", func(r chi.Router) {
						r.Get("/", a.h.ListProjectMembers)
						r.Post("/", a.h.AddProjectMember)
						r.Delete(fmt.Sprintf("/{%s}", handlers.UserIDParam), a.h.RemoveProjectMember)
						r.Patch(fmt.Sprintf("/{%s}/role", handlers.UserIDParam), a.h.UpdateProjectMemberRole)
					})

					// Project invitations
					r.Route("/invitations", func(r chi.Router) {
						r.Get("/", a.h.ListProjectInvitations)
						r.Post("/", a.h.CreateProjectInvitation)
						r.Delete(fmt.Sprintf("/{%s}", handlers.InvitationIDParam), a.h.RevokeProjectInvitation)
					})

					// Project runs
					r.Route("/runs", func(r chi.Router) {
						r.Get("/", a.h.ListRuns)
						r.Get("/summary", a.h.GetRunDashboardSummary)
						r.Post("/", a.h.TriggerRun)
						r.Route(fmt.Sprintf("/{%s}", handlers.RunIDParam), func(r chi.Router) {
							r.Get("/", a.h.GetRun)
							r.Get("/detail", a.h.GetRunDetail)
							r.Get("/tasks", a.h.GetRunTasks)
							r.Get("/logs", a.h.StreamRunLogs)     // SSE endpoint for logs
							r.Get("/logs/history", a.h.GetLogs)   // Get persisted logs (paginated)
							r.Get("/events", a.h.StreamRunEvents) // SSE endpoint for run status
							r.Post("/cancel", a.h.CancelRun)
							r.Patch("/status", a.h.UpdateRunStatus)                                           // Update run status
							r.Post("/tasks", a.h.CreateTask)                                                  // Create task result
							r.Patch(fmt.Sprintf("/tasks/{%s}", handlers.TaskNameParam), a.h.UpdateTaskStatus) // Update task status
							r.Post("/logs", a.h.AppendLog)                                                    // Append log lines
							r.Post("/heartbeat", a.h.Heartbeat)                                               // Heartbeat for offline detection
							r.Get("/workflow", a.h.GetRunWorkflow)                                            // Get workflow snapshot for this run

							r.Route("/artifacts", func(r chi.Router) {
								r.Get("/", a.h.ListRunArtifacts)
								r.Post("/", a.h.UploadArtifact)
								r.Route(fmt.Sprintf("/{%s}", handlers.ArtifactIDParam), func(r chi.Router) {
									r.Get("/", a.h.GetArtifact)
									r.Get("/download", a.h.DownloadArtifact)
									r.Delete("/", a.h.DeleteArtifact)
								})
							})

							// AI analysis endpoints (gated by ai_analysis feature)
							r.Group(func(r chi.Router) {
								r.Use(middleware.RequireFeature(a.entitlements, string(licensing.FeatureAIAnalysis)))
								r.Get("/ai-analysis", a.h.GetAIAnalysis)
								r.Post("/ai-analysis/retry", a.h.RetryAIAnalysis)
								r.Get("/ai-suggestions", a.h.GetAISuggestions)
								r.Post("/ai-suggestions/post", a.h.PostAISuggestions)
							})
						})
					})

					// Project cache (gated by cloud_cache feature)
					r.Route("/cache", func(r chi.Router) {
						r.Use(middleware.RequireFeature(a.entitlements, string(licensing.FeatureCloudCache)))
						r.Get("/stats", a.h.GetCacheStats)
						r.Get("/analytics", a.h.GetCacheAnalytics)
						r.Post("/gc", a.h.TriggerCacheGC)
						r.Route(fmt.Sprintf("/{%s}/{%s}", handlers.TaskNameParam, handlers.CacheKeyParam), func(r chi.Router) {
							r.Get("/", a.h.CheckCache)
							r.Put("/", a.h.UploadCache)
							r.Get("/download", a.h.DownloadCache)
							r.Delete("/", a.h.DeleteCache)
						})
					})

					// Project API keys
					r.Route("/api-keys", func(r chi.Router) {
						r.Get("/", a.h.ListProjectAPIKeys)
						r.Post("/", a.h.CreateProjectAPIKey)
						r.Delete("/{keyID}", a.h.RevokeProjectAPIKey)
					})

					// Project workflows
					r.Route("/workflows", func(r chi.Router) {
						r.Get("/", a.h.ListProjectWorkflows)
						r.Post("/sync", a.h.SyncProjectWorkflow)
						r.Post("/sync-from-toml", a.h.SyncProjectWorkflowFromToml)
					})

					// Project AI analyses (gated by ai_analysis feature)
					r.Group(func(r chi.Router) {
						r.Use(middleware.RequireFeature(a.entitlements, string(licensing.FeatureAIAnalysis)))
						r.Get("/ai-analyses", a.h.ListAIAnalyses)
					})

					// Project plugins
					r.Route("/plugins", func(r chi.Router) {
						r.Get("/", a.h.ListProjectPlugins)
						r.Post("/", a.h.InstallPlugin)
						r.Delete(fmt.Sprintf("/{%s}", handlers.PluginNameParam), a.h.UninstallPlugin)
					})
				})
			})

			// User API keys (user-scoped, all projects)
			r.Route("/api-keys", func(r chi.Router) {
				r.Get("/", a.h.ListUserAPIKeys)
				r.Post("/", a.h.CreateUserAPIKey)
				r.Delete(fmt.Sprintf("/{%s}", handlers.KeyIDParam), a.h.RevokeUserAPIKey)
			})

			// Plugin management routes
			r.Route("/plugins", func(r chi.Router) {
				r.Get("/", a.h.ListPlugins)
				r.Get("/official", a.h.ListOfficialPlugins)
				r.Get(fmt.Sprintf("/{%s}/{%s}", handlers.PublisherParam, handlers.NameParam), a.h.GetPluginByPublisherName)
				r.Get(fmt.Sprintf("/{%s}", handlers.PluginNameParam), a.h.GetPluginManifest)
			})

			// Plugin registry routes
			r.Route("/registry", func(r chi.Router) {
				// Public endpoints

				r.Route("/plugins", func(r chi.Router) {
					r.Get("/", a.h.SearchRegistryPlugins)
					r.Route(fmt.Sprintf("/{%s}/{%s}", handlers.PublisherParam, handlers.NameParam), func(r chi.Router) {
						r.Get("/", a.h.GetRegistryPlugin)
						r.Get("/analytics", a.h.GetRegistryPluginAnalytics)
						r.Post("/download", a.h.TrackPluginDownload)

						//
						r.Route("/versions", func(r chi.Router) {
							r.Get("/", a.h.GetRegistryPluginVersions)
							r.Post("/", a.h.PublishPluginVersion)
							r.Delete(fmt.Sprintf("/{%s}", handlers.VersionParam), a.h.YankPluginVersion)
						})
					})
				})

				r.Get("/featured", a.h.ListFeaturedPlugins)
				r.Get("/trending", a.h.ListTrendingPlugins)

				// Auth-required endpoints
				r.Post("/publishers", a.h.CreatePublisher)
				r.Get(fmt.Sprintf("/publishers/{%s}", handlers.PublisherParam), a.h.GetPublisher)

			})

			// License status and activation
			r.Route("/license", func(r chi.Router) {
				r.Get("/", a.h.GetLicenseStatus)
				r.Post("/activate", a.h.ActivateLicense)
			})

			// Invitations (for accepting)
			r.Route("/invitations", func(r chi.Router) {
				r.Get("/", a.h.ListPendingInvitations)
				r.Post("/{token}/accept", a.h.AcceptInvitation)
				r.Post("/{token}/decline", a.h.DeclineInvitation)
			})

			// Extra routes injected by the cloud binary (e.g. billing
			// routes from dagryn-cloud via RegisterExtraRoutes).
			if a.extraRoutes != nil {
				a.extraRoutes(r)
			}
		})
	})

	// Dashboard - serve SPA for non-API routes
	var dh http.Handler
	if a.dashboardHandler != nil {
		dh = a.dashboardHandler
	} else {
		dh = dashboard.Handler()
	}
	router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Don't serve dashboard for API routes or special paths
		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/health") ||
			strings.HasPrefix(r.URL.Path, "/ready") ||
			strings.HasPrefix(r.URL.Path, "/metrics") ||
			strings.HasPrefix(r.URL.Path, "/swagger") ||
			strings.HasPrefix(r.URL.Path, "/"+workerRoutePrefix) {
			_ = response.NotFound(w, r, errors.New("the requested resource was not found"))
			return
		}
		dh.ServeHTTP(w, r)
	})

	// 404 handler for API routes
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		_ = response.NotFound(w, r, errors.New("the requested resource was not found"))
	})

	// 405 handler
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		_ = response.MethodNotAllowed(w, r, errors.New("the requested method is not allowed for this resource"))
	})

	return router
}

func (a *API) createAuthMiddleware() func(http.Handler) http.Handler {
	return middleware.Auth(middleware.AuthConfig{
		JWTService: a.jwtService,
		UserRepo:   a.store.Users,
		APIKeyRepo: a.store.APIKeys,
	})
}
