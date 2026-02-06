package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/mujhtech/dagryn/internal/job"
	"github.com/mujhtech/dagryn/internal/server/dashboard"
	"github.com/mujhtech/dagryn/internal/server/handlers"
	"github.com/mujhtech/dagryn/internal/server/middleware"
	"github.com/mujhtech/dagryn/internal/server/response"
)

// setupRoutes configures all API routes.
func (s *Server) setupRoutes(h *handlers.Handler, authHandler *handlers.AuthHandler, authMiddleware func(http.Handler) http.Handler, jobClient *job.Client) {
	r := s.router

	// Health check (no auth required)
	r.Get("/health", h.Health)
	r.Get("/ready", h.Ready)

	// Swagger documentation
	if s.config.Server.Swagger.Enabled {
		swaggerPath := s.config.Server.Swagger.Path
		if swaggerPath == "" {
			swaggerPath = "/swagger"
		}
		r.Get(swaggerPath+"/*", httpSwagger.Handler(
			httpSwagger.URL(swaggerPath+"/doc.json"),
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("list"),
			httpSwagger.DomID("swagger-ui"),
		))
	}

	// Metrics endpoint (if prometheus is enabled and on same port)
	if s.telemetry != nil && s.config.Telemetry.Metrics.Enabled {
		metricsHandler := s.telemetry.MetricsHandler()
		if metricsHandler != nil {
			r.Handle("/metrics", metricsHandler)
		}
	}

	// Queue monitoring (asynqmon) - only when job client is configured
	if jobClient != nil {
		r.Route("/queue", func(r chi.Router) {
			r.Handle("/monitoring/*", jobClient.Monitor())
		})
	}

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Provider webhooks (public, no auth) - GitHub, GitLab, Bitbucket.
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/github", h.GitHubWebhook)
			// Future: /gitlab, /bitbucket
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
				r.Get("/me", h.GetCurrentUser)
				r.Patch("/me", h.UpdateCurrentUser)
			})

			// Team routes
			r.Route("/teams", func(r chi.Router) {
				r.Get("/", h.ListTeams)
				r.Post("/", h.CreateTeam)
				r.Route("/{teamID}", func(r chi.Router) {
					r.Get("/", h.GetTeam)
					r.Patch("/", h.UpdateTeam)
					r.Delete("/", h.DeleteTeam)

					// Team members
					r.Route("/members", func(r chi.Router) {
						r.Get("/", h.ListTeamMembers)
						r.Post("/", h.AddTeamMember)
						r.Delete("/{userID}", h.RemoveTeamMember)
						r.Patch("/{userID}/role", h.UpdateTeamMemberRole)
					})

					// Team invitations
					r.Route("/invitations", func(r chi.Router) {
						r.Get("/", h.ListTeamInvitations)
						r.Post("/", h.CreateTeamInvitation)
						r.Delete("/{invitationID}", h.RevokeTeamInvitation)
					})
				})
			})

			// Provider routes (e.g. list repos for Import from GitHub)
			r.Route("/providers", func(r chi.Router) {
				r.Route("/github", func(r chi.Router) {
					r.Get("/repos", h.ListGitHubRepos)
					r.Route("/app", func(r chi.Router) {
						r.Get("/installations", h.ListGitHubAppInstallations)
						r.Get("/installations/{installationID}/repos", h.ListGitHubAppRepos)
					})
				})
			})

			// Project routes
			r.Route("/projects", func(r chi.Router) {
				r.Get("/", h.ListProjects)
				r.Post("/", h.CreateProject)
				r.Route("/{projectID}", func(r chi.Router) {
					r.Get("/", h.GetProject)
					r.Patch("/", h.UpdateProject)
					r.Delete("/", h.DeleteProject)
					r.Post("/connect-github", h.ConnectProjectToGitHub)

					// Project members
					r.Route("/members", func(r chi.Router) {
						r.Get("/", h.ListProjectMembers)
						r.Post("/", h.AddProjectMember)
						r.Delete("/{userID}", h.RemoveProjectMember)
						r.Patch("/{userID}/role", h.UpdateProjectMemberRole)
					})

					// Project invitations
					r.Route("/invitations", func(r chi.Router) {
						r.Get("/", h.ListProjectInvitations)
						r.Post("/", h.CreateProjectInvitation)
						r.Delete("/{invitationID}", h.RevokeProjectInvitation)
					})

					// Project runs
					r.Route("/runs", func(r chi.Router) {
						r.Get("/", h.ListRuns)
						r.Post("/", h.TriggerRun)
						r.Route("/{runID}", func(r chi.Router) {
							r.Get("/", h.GetRun)
							r.Get("/detail", h.GetRunDetail)
							r.Get("/tasks", h.GetRunTasks)
							r.Get("/logs", h.StreamRunLogs)     // SSE endpoint for logs
							r.Get("/logs/history", h.GetLogs)   // Get persisted logs (paginated)
							r.Get("/events", h.StreamRunEvents) // SSE endpoint for run status
							r.Post("/cancel", h.CancelRun)
							r.Patch("/status", h.UpdateRunStatus)            // Update run status
							r.Post("/tasks", h.CreateTask)                   // Create task result
							r.Patch("/tasks/{taskName}", h.UpdateTaskStatus) // Update task status
							r.Post("/logs", h.AppendLog)                     // Append log lines
							r.Post("/heartbeat", h.Heartbeat)                // Heartbeat for offline detection
							r.Get("/workflow", h.GetRunWorkflow)             // Get workflow snapshot for this run
						})
					})

					// Project API keys
					r.Route("/api-keys", func(r chi.Router) {
						r.Get("/", h.ListProjectAPIKeys)
						r.Post("/", h.CreateProjectAPIKey)
						r.Delete("/{keyID}", h.RevokeProjectAPIKey)
					})

					// Project workflows
					r.Route("/workflows", func(r chi.Router) {
						r.Get("/", h.ListProjectWorkflows)
						r.Post("/sync", h.SyncProjectWorkflow)
					})
				})
			})

			// User API keys (user-scoped, all projects)
			r.Route("/api-keys", func(r chi.Router) {
				r.Get("/", h.ListUserAPIKeys)
				r.Post("/", h.CreateUserAPIKey)
				r.Delete("/{keyID}", h.RevokeUserAPIKey)
			})

			// Invitations (for accepting)
			r.Route("/invitations", func(r chi.Router) {
				r.Get("/", h.ListPendingInvitations)
				r.Post("/{token}/accept", h.AcceptInvitation)
				r.Post("/{token}/decline", h.DeclineInvitation)
			})
		})
	})

	// Dashboard - serve SPA for non-API routes
	dashboardHandler := dashboard.Handler()
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Don't serve dashboard for API routes or special paths
		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/health") ||
			strings.HasPrefix(r.URL.Path, "/ready") ||
			strings.HasPrefix(r.URL.Path, "/metrics") ||
			strings.HasPrefix(r.URL.Path, "/swagger") ||
			strings.HasPrefix(r.URL.Path, "/queue") {
			response.NotFound(w, r, errors.New("The requested resource was not found"))
			return
		}
		dashboardHandler.ServeHTTP(w, r)
	})

	// 404 handler for API routes
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.NotFound(w, r, errors.New("The requested resource was not found"))
	})

	// 405 handler
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.MethodNotAllowed(w, r, errors.New("The requested method is not allowed for this resource"))
	})
}

// createAuthMiddleware creates the auth middleware with proper dependencies.
func (s *Server) createAuthMiddleware() func(http.Handler) http.Handler {
	return middleware.Auth(middleware.AuthConfig{
		JWTService: s.jwtService,
		UserRepo:   s.repos.Users,
		APIKeyRepo: s.repos.APIKeys,
	})
}
