// Package serverctx provides context utilities for request handling.
package serverctx

import (
	"context"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
)

// Context keys for auth information
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "user"
	// UserIDContextKey is the context key for the user ID.
	UserIDContextKey contextKey = "user_id"
	// AuthMethodContextKey is the context key for the authentication method.
	AuthMethodContextKey contextKey = "auth_method"
	// APIKeyContextKey is the context key for the API key (if used).
	APIKeyContextKey contextKey = "api_key"
)

// AuthMethod represents how the request was authenticated.
type AuthMethod string

const (
	// AuthMethodJWT indicates JWT bearer token authentication.
	AuthMethodJWT AuthMethod = "jwt"
	// AuthMethodAPIKey indicates API key authentication.
	AuthMethodAPIKey AuthMethod = "api_key"
)

// WithUser adds the user to the context.
func WithUser(ctx context.Context, user *models.User) context.Context {
	ctx = context.WithValue(ctx, UserContextKey, user)
	if user != nil {
		ctx = context.WithValue(ctx, UserIDContextKey, user.ID)
	}
	return ctx
}

// WithAuthMethod adds the auth method to the context.
func WithAuthMethod(ctx context.Context, method AuthMethod) context.Context {
	return context.WithValue(ctx, AuthMethodContextKey, method)
}

// WithAPIKey adds the API key to the context.
func WithAPIKey(ctx context.Context, key *models.APIKey) context.Context {
	return context.WithValue(ctx, APIKeyContextKey, key)
}

// GetUser returns the authenticated user from the context, or nil if not authenticated.
func GetUser(ctx context.Context) *models.User {
	user, _ := ctx.Value(UserContextKey).(*models.User)
	return user
}

// GetUserID returns the authenticated user ID from the context, or uuid.Nil if not authenticated.
func GetUserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(UserIDContextKey).(uuid.UUID)
	return id
}

// GetAuthMethod returns the authentication method used, or empty string if not authenticated.
func GetAuthMethod(ctx context.Context) AuthMethod {
	method, _ := ctx.Value(AuthMethodContextKey).(AuthMethod)
	return method
}

// GetAPIKey returns the API key used for authentication, or nil if not API key auth.
func GetAPIKey(ctx context.Context) *models.APIKey {
	key, _ := ctx.Value(APIKeyContextKey).(*models.APIKey)
	return key
}

// IsAuthenticated returns true if the request is authenticated.
func IsAuthenticated(ctx context.Context) bool {
	return GetUser(ctx) != nil
}
