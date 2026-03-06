// Package apiCtx provides context utilities for request handling.
package apiCtx

import (
	"context"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
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
	// IPAddressContextKey is the context key for the client IP address.
	IPAddressContextKey contextKey = "ip_address"
	// UserAgentContextKey is the context key for the client user agent.
	UserAgentContextKey contextKey = "user_agent"
	// RequestIDContextKey is the context key for the request ID.
	RequestIDContextKey contextKey = "request_id"
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

// WithIPAddress adds the client IP address to the context.
func WithIPAddress(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, IPAddressContextKey, ip)
}

// GetIPAddress returns the client IP address from the context.
func GetIPAddress(ctx context.Context) string {
	ip, _ := ctx.Value(IPAddressContextKey).(string)
	return ip
}

// WithUserAgent adds the client user agent to the context.
func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, UserAgentContextKey, ua)
}

// GetUserAgent returns the client user agent from the context.
func GetUserAgent(ctx context.Context) string {
	ua, _ := ctx.Value(UserAgentContextKey).(string)
	return ua
}

// WithRequestID adds the request ID to the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDContextKey, id)
}

// GetRequestID returns the request ID from the context.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDContextKey).(string)
	return id
}
