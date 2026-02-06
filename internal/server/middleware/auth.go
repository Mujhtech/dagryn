package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/mujhtech/dagryn/internal/db/repo"
	"github.com/mujhtech/dagryn/internal/server/auth"
	serverctx "github.com/mujhtech/dagryn/internal/server/context"
)

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	JWTService *auth.JWTService
	UserRepo   *repo.UserRepo
	APIKeyRepo *repo.APIKeyRepo
}

// Auth returns a middleware that authenticates requests using JWT or API key.
// It supports two authentication methods:
//   - Bearer token in Authorization header (for dashboard/web)
//   - API key in X-API-Key header (for CLI/CI)
func Auth(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Try API key first (preferred for programmatic access)
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				user, key, err := authenticateAPIKey(ctx, config, apiKey)
				if err != nil {
					handleAuthError(w, err)
					return
				}

				ctx = serverctx.WithUser(ctx, user)
				ctx = serverctx.WithAuthMethod(ctx, serverctx.AuthMethodAPIKey)
				ctx = serverctx.WithAPIKey(ctx, key)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try Bearer token
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				if strings.HasPrefix(authHeader, "Bearer ") {
					token := strings.TrimPrefix(authHeader, "Bearer ")
					user, err := authenticateJWT(ctx, config, token)
					if err != nil {
						handleAuthError(w, err)
						return
					}

					ctx = serverctx.WithUser(ctx, user)
					ctx = serverctx.WithAuthMethod(ctx, serverctx.AuthMethodJWT)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// No authentication provided
			writeJSON(w, http.StatusUnauthorized, errorResponse{
				Error:   "unauthorized",
				Message: "Authentication required. Provide a Bearer token or X-API-Key header.",
			})
		})
	}
}

// authenticateJWT validates a JWT token and returns the associated user.
func authenticateJWT(ctx context.Context, config AuthConfig, token string) (*models.User, error) {
	claims, err := config.JWTService.ValidateAccessToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Get user from database
	user, err := config.UserRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, auth.ErrInvalidToken
		}
		return nil, err
	}

	// Update token last used (async, don't block request)
	go func() {
		if err := config.JWTService.UpdateLastUsed(context.Background(), claims.ID); err != nil {
			slog.Error("failed to update token last used", "error", err)
		}
	}()

	return user, nil
}

// authenticateAPIKey validates an API key and returns the associated user and key.
func authenticateAPIKey(ctx context.Context, config AuthConfig, rawKey string) (*models.User, *models.APIKey, error) {
	key, err := config.APIKeyRepo.ValidateKey(ctx, rawKey)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil, auth.ErrInvalidToken
		}
		return nil, nil, err
	}

	// Get user from database
	user, err := config.UserRepo.GetByID(ctx, key.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil, auth.ErrInvalidToken
		}
		return nil, nil, err
	}

	return user, key, nil
}

// handleAuthError writes an appropriate error response for authentication errors.
func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidToken):
		writeJSON(w, http.StatusUnauthorized, errorResponse{
			Error:   "invalid_token",
			Message: "The provided token is invalid.",
		})
	case errors.Is(err, auth.ErrExpiredToken):
		writeJSON(w, http.StatusUnauthorized, errorResponse{
			Error:   "token_expired",
			Message: "The provided token has expired.",
		})
	case errors.Is(err, auth.ErrRevokedToken):
		writeJSON(w, http.StatusUnauthorized, errorResponse{
			Error:   "token_revoked",
			Message: "The provided token has been revoked.",
		})
	default:
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "auth_error",
			Message: "An error occurred during authentication.",
		})
	}
}

// RequireAuth is a convenience wrapper that ensures authentication.
// Use Auth middleware instead for most cases.
func RequireAuth(jwtService *auth.JWTService, userRepo *repo.UserRepo, apiKeyRepo *repo.APIKeyRepo) func(http.Handler) http.Handler {
	return Auth(AuthConfig{
		JWTService: jwtService,
		UserRepo:   userRepo,
		APIKeyRepo: apiKeyRepo,
	})
}

// OptionalAuth returns a middleware that attempts authentication but allows unauthenticated requests.
// Useful for endpoints that behave differently for authenticated vs anonymous users.
func OptionalAuth(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Try API key first
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				user, key, err := authenticateAPIKey(ctx, config, apiKey)
				if err == nil {
					ctx = serverctx.WithUser(ctx, user)
					ctx = serverctx.WithAuthMethod(ctx, serverctx.AuthMethodAPIKey)
					ctx = serverctx.WithAPIKey(ctx, key)
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try Bearer token
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				if strings.HasPrefix(authHeader, "Bearer ") {
					token := strings.TrimPrefix(authHeader, "Bearer ")
					user, err := authenticateJWT(ctx, config, token)
					if err == nil {
						ctx = serverctx.WithUser(ctx, user)
						ctx = serverctx.WithAuthMethod(ctx, serverctx.AuthMethodJWT)
					}
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireProjectAccess returns a middleware that checks if the authenticated user
// has access to the specified project (via API key scope or project membership).
func RequireProjectAccess(projectRepo *repo.ProjectRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			user := serverctx.GetUser(ctx)
			if user == nil {
				writeJSON(w, http.StatusUnauthorized, errorResponse{
					Error:   "unauthorized",
					Message: "Authentication required.",
				})
				return
			}

			// If using API key with project scope, check the project matches
			if apiKey := serverctx.GetAPIKey(ctx); apiKey != nil {
				if apiKey.Scope == models.APIKeyScopeProject && apiKey.ProjectID != nil {
					// Project-scoped API key - project access is implicit
					// The route handler should verify the requested project matches
					next.ServeHTTP(w, r)
					return
				}
				// User-scoped API key - check project membership
			}

			// For JWT or user-scoped API keys, project membership is checked in handlers
			next.ServeHTTP(w, r)
		})
	}
}

// errorResponse for middleware internal use.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeJSON writes a JSON response (internal helper).
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to write JSON response", "error", err)
		}
	}
}
