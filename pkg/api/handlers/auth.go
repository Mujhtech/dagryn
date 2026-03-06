package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/api/dto"
	"github.com/mujhtech/dagryn/pkg/authn"
	"github.com/mujhtech/dagryn/pkg/authz"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	jwtService           *authz.JWTService
	deviceCodeService    *authz.DeviceCodeService
	userRepo             repo.UserStore
	providerTokenRepo    repo.ProviderTokenStore
	providerTokenEncrypt encrypt.Encrypt
	providers            map[string]authn.Provider
	baseURL              string
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(
	jwtService *authz.JWTService,
	deviceCodeService *authz.DeviceCodeService,
	userRepo repo.UserStore,
	providerTokenRepo repo.ProviderTokenStore,
	providerTokenEncrypt encrypt.Encrypt,
	providers map[string]authn.Provider,
	baseURL string,
) *AuthHandler {
	return &AuthHandler{
		jwtService:           jwtService,
		deviceCodeService:    deviceCodeService,
		userRepo:             userRepo,
		providerTokenRepo:    providerTokenRepo,
		providerTokenEncrypt: providerTokenEncrypt,
		providers:            providers,
		baseURL:              baseURL,
	}
}

// ListProviders godoc
//
//	@Summary		List authentication providers
//	@Description	Returns the list of available OAuth authentication providers
//	@Tags			auth
//	@Produce		json
//	@Accept			json
//	@Security		BearerAuth
//	@Success		200	{object}	response.ServerResponse{data=dto.AuthProviderResponse}	"Authentication providers retrieved"
//	@Failure		400	{object}	response.ServerResponse									"Bad request"
//	@Failure		401	{object}	response.ServerResponse									"Unauthorized"
//	@Failure		500	{object}	response.ServerResponse									"Internal server error"
//	@Router			/api/v1/auth/providers [get]
func (h *AuthHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	var providers []dto.AuthProvider
	for name, p := range h.providers {
		providers = append(providers, dto.AuthProvider{
			ID:      name,
			Name:    providerDisplayName(name),
			AuthURL: p.AuthURL(""),
			Enabled: true,
		})
	}

	_ = response.Ok(w, r, "Authentication providers retrieved", providers)
}

// StartOAuth godoc
//
//	@Summary		Start OAuth flow
//	@Description	Initiates OAuth authentication with the specified provider
//	@Tags			auth
//	@Produce		json
//	@Param			provider	path		string	true	"OAuth provider (github, google)"
//	@Success		200			{object}	OAuthStartResponse
//	@Failure		400			{object}	ErrorResponse
//	@Router			/api/v1/auth/{provider} [get]
func (h *AuthHandler) StartOAuth(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")

	provider, ok := h.providers[providerName]
	if !ok {
		_ = response.BadRequest(w, r, errors.New("unknown OAuth provider"))
		return
	}

	// Generate state for CSRF protection
	state, err := authz.GenerateRandomState()
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to generate state"))
		return
	}

	// In a real implementation, you'd store the state in a session or database
	// For now, return the auth URL and state
	authURL := provider.AuthURL(state)

	_ = response.Ok(w, r, "oAuth flow initiated", OAuthStartResponse{
		AuthURL: authURL,
		State:   state,
	})
}

// OAuthCallback godoc
//
//	@Summary		OAuth callback
//	@Description	Handles OAuth callback from providers and returns tokens
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			provider	path		string					true	"OAuth provider (github, google)"
//	@Param			body		body		OAuthCallbackRequest	true	"OAuth callback data"
//	@Success		200			{object}	TokenResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/v1/auth/{provider}/callback [post]
func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")

	provider, ok := h.providers[providerName]
	if !ok {
		_ = response.BadRequest(w, r, errors.New("unknown OAuth provider"))
		return
	}

	var req OAuthCallbackRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.Code == "" {
		_ = response.BadRequest(w, r, errors.New("authorization code is required"))
		return
	}

	ctx := r.Context()

	// Exchange code for tokens
	oauthTokens, err := provider.Exchange(ctx, req.Code)
	if err != nil {
		_ = response.Unauthorized(w, r, errors.New("failed to exchange authorization code"))
		return
	}

	// Get user info from provider
	userInfo, err := provider.GetUserInfo(ctx, oauthTokens.AccessToken)
	if err != nil {
		_ = response.Unauthorized(w, r, errors.New("failed to get user info"))
		return
	}

	// Upsert user in database
	user := &models.User{
		Email:      userInfo.Email,
		Provider:   string(userInfo.Provider),
		ProviderID: userInfo.ID,
	}
	if userInfo.Name != "" {
		user.Name = &userInfo.Name
	}
	if userInfo.AvatarURL != "" {
		user.AvatarURL = &userInfo.AvatarURL
	}

	if err := h.userRepo.UpsertByProvider(ctx, user); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create user"))
		return
	}

	// Store provider access token for GitHub (list repos, webhooks)
	if providerName == "github" && h.providerTokenRepo != nil && h.providerTokenEncrypt != nil {
		encrypted, err := h.providerTokenEncrypt.Encrypt([]byte(oauthTokens.AccessToken))
		if err == nil {
			_ = h.providerTokenRepo.Upsert(ctx, user.ID, "github", encrypted)
		}
	}

	// Generate JWT tokens
	tokenPair, err := h.jwtService.GenerateTokenPair(ctx, user)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to generate tokens"))
		return
	}

	_ = response.Ok(w, r, "Authentication successful", TokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		ExpiresAt:    tokenPair.ExpiresAt,
		User:         userToResponse(user),
	})
}

// RefreshToken godoc
//
//	@Summary		Refresh access token
//	@Description	Exchanges a refresh token for a new access token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		RefreshTokenRequest	true	"Refresh token request"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.RefreshToken == "" {
		_ = response.BadRequest(w, r, errors.New("refresh token is required"))
		return
	}

	ctx := r.Context()

	// Validate refresh token and get claims
	claims, err := h.jwtService.ValidateRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		handleAuthError(w, r, err)
		return
	}

	// Get user from database
	user, err := h.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.Unauthorized(w, r, errors.New("user no longer exists"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get user"))
		return
	}

	// Generate new token pair
	tokenPair, err := h.jwtService.RefreshTokens(ctx, req.RefreshToken, user)
	if err != nil {
		handleAuthError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "Token refreshed successfully", TokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		ExpiresAt:    tokenPair.ExpiresAt,
		User:         userToResponse(user),
	})
}

// Logout godoc
//
//	@Summary		Logout
//	@Description	Revokes the current access token and optionally all tokens
//	@Tags			auth
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LogoutRequest	false	"Logout options"
//	@Success		200		{object}	SuccessResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req LogoutRequest
	// Ignore parse errors - body is optional
	_ = ParseJSON(r, &req)

	if req.RevokeAll {
		// Revoke all tokens for user
		if err := h.jwtService.RevokeAllUserTokens(ctx, user.ID); err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to revoke tokens"))
			return
		}
		_ = response.Ok(w, r, "All tokens revoked successfully", SuccessResponse{
			Message: "All tokens revoked successfully",
		})
		return
	}

	// Just acknowledge logout - token validation will handle expiry
	_ = response.Ok(w, r, "Logged out successfully", SuccessResponse{
		Message: "Logged out successfully",
	})
}

// RequestDeviceCode godoc
//
//	@Summary		Request device code
//	@Description	Initiates device code flow for CLI authentication
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	DeviceCodeResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/api/v1/auth/device [post]
func (h *AuthHandler) RequestDeviceCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	deviceCode, err := h.deviceCodeService.RequestDeviceCode(ctx)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to generate device code"))
		return
	}

	_ = response.Ok(w, r, "Device code generated", DeviceCodeResponse{
		DeviceCode:      deviceCode.DeviceCode,
		UserCode:        deviceCode.UserCode,
		VerificationURI: deviceCode.VerificationURI,
		ExpiresIn:       deviceCode.ExpiresIn,
		Interval:        deviceCode.Interval,
	})
}

// PollDeviceCode godoc
//
//	@Summary		Poll device code
//	@Description	Polls for device code authorization status
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		DeviceCodePollRequest	true	"Device code poll request"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		428		{object}	DeviceCodePendingResponse	"Authorization pending"
//	@Router			/api/v1/auth/device/poll [post]
func (h *AuthHandler) PollDeviceCode(w http.ResponseWriter, r *http.Request) {
	var req DeviceCodePollRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.DeviceCode == "" {
		_ = response.BadRequest(w, r, errors.New("device code is required"))
		return
	}

	ctx := r.Context()

	// Poll for authorization
	userID, err := h.deviceCodeService.Poll(ctx, req.DeviceCode)
	if err != nil {
		switch {
		case errors.Is(err, authz.ErrDeviceCodePending):
			_ = response.PreconditionRequired(w, r, "authorization_pending", DeviceCodePendingResponse{
				Error:       "authorization_pending",
				Description: "The user has not yet authorized the device",
			})
			return
		case errors.Is(err, authz.ErrDeviceCodeExpired):
			_ = response.BadRequest(w, r, errors.New("device code has expired"))
			return
		case errors.Is(err, authz.ErrDeviceCodeNotFound):
			_ = response.BadRequest(w, r, errors.New("invalid device code"))
			return
		default:
			_ = response.InternalServerError(w, r, errors.New("failed to poll device code"))
			return
		}
	}

	// Get user
	user, err := h.userRepo.GetByID(ctx, *userID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to get user"))
		return
	}

	// Consume the device code
	if err := h.deviceCodeService.Consume(ctx, req.DeviceCode); err != nil {
		// Log but don't fail - tokens should still be issued
		slog.Error("failed to consume device code", "error", err)
	}

	// Generate JWT tokens
	tokenPair, err := h.jwtService.GenerateTokenPair(ctx, user)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to generate tokens"))
		return
	}

	_ = response.Ok(w, r, "Device authenticated successfully", TokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		ExpiresAt:    tokenPair.ExpiresAt,
		User:         userToResponse(user),
	})
}

// AuthorizeDevice godoc
//
//	@Summary		Authorize device
//	@Description	Authorizes a device code (called by user in browser)
//	@Tags			auth
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		DeviceAuthorizeRequest	true	"Device authorize request"
//	@Success		200		{object}	SuccessResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/v1/auth/device/authorize [post]
func (h *AuthHandler) AuthorizeDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req DeviceAuthorizeRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.UserCode == "" {
		_ = response.BadRequest(w, r, errors.New("user code is required"))
		return
	}

	// Authorize the device
	if err := h.deviceCodeService.Authorize(ctx, req.UserCode, user.ID); err != nil {
		switch {
		case errors.Is(err, authz.ErrDeviceCodeNotFound):
			_ = response.BadRequest(w, r, errors.New("invalid user code"))
			return
		case errors.Is(err, authz.ErrDeviceCodeExpired):
			_ = response.BadRequest(w, r, errors.New("user code has expired"))
			return
		case errors.Is(err, authz.ErrDeviceCodeAlreadyUsed):
			_ = response.BadRequest(w, r, errors.New("user code has already been used"))
			return
		default:
			_ = response.InternalServerError(w, r, errors.New("failed to authorize device"))
			return
		}
	}

	_ = response.Ok(w, r, "Device authorized successfully", SuccessResponse{
		Message: "Device authorized successfully",
	})
}

// DenyDevice godoc
//
//	@Summary		Deny device
//	@Description	Denies a device code authorization
//	@Tags			auth
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		DeviceAuthorizeRequest	true	"Device deny request"
//	@Success		200		{object}	SuccessResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/v1/auth/device/deny [post]
func (h *AuthHandler) DenyDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req DeviceAuthorizeRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	if req.UserCode == "" {
		_ = response.BadRequest(w, r, errors.New("user code is required"))
		return
	}

	// Deny the device
	if err := h.deviceCodeService.Deny(ctx, req.UserCode); err != nil {
		switch {
		case errors.Is(err, authz.ErrDeviceCodeNotFound):
			_ = response.BadRequest(w, r, errors.New("invalid user code"))
			return
		default:
			_ = response.InternalServerError(w, r, errors.New("failed to deny device"))
			return
		}
	}

	_ = response.Ok(w, r, "Device authorization denied", SuccessResponse{
		Message: "Device authorization denied",
	})
}

// handleAuthError writes an appropriate error response for authentication errors.
func handleAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authz.ErrInvalidToken):
		_ = response.Unauthorized(w, r, errors.New("the provided token is invalid"))
	case errors.Is(err, authz.ErrExpiredToken):
		_ = response.Unauthorized(w, r, errors.New("the provided token has expired"))
	case errors.Is(err, authz.ErrRevokedToken):
		_ = response.Unauthorized(w, r, errors.New("the provided token has been revoked"))
	default:
		_ = response.InternalServerError(w, r, errors.New("an error occurred during authentication"))
	}
}

// userToResponse converts a user model to a response.
func userToResponse(user *models.User) UserResponse {
	resp := UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Provider:  user.Provider,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	if user.Name != nil {
		resp.Name = *user.Name
	}
	if user.AvatarURL != nil {
		resp.AvatarURL = *user.AvatarURL
	}
	return resp
}

// providerDisplayName returns a human-readable name for a provider.
func providerDisplayName(provider string) string {
	switch provider {
	case "github":
		return "GitHub"
	case "google":
		return "Google"
	default:
		return provider
	}
}

// OAuthStartResponse represents the response when starting OAuth flow.
//
//	@Description	OAuth flow start response
type OAuthStartResponse struct {
	AuthURL string `json:"auth_url" example:"https://github.com/login/oauth/authorize?client_id=..."`
	State   string `json:"state" example:"abc123xyz"`
}

// OAuthCallbackRequest represents the OAuth callback request.
//
//	@Description	OAuth callback request
type OAuthCallbackRequest struct {
	Code  string `json:"code" example:"abc123"`
	State string `json:"state" example:"xyz789"`
}

// TokenResponse represents a token response.
//
//	@Description	Authentication token response
type TokenResponse struct {
	AccessToken  string       `json:"access_token" example:"eyJhbGciOiJIUzI1NiIs..."`
	RefreshToken string       `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIs..."`
	TokenType    string       `json:"token_type" example:"Bearer"`
	ExpiresIn    int64        `json:"expires_in" example:"900"`
	ExpiresAt    time.Time    `json:"expires_at" example:"2024-01-15T10:45:00Z"`
	User         UserResponse `json:"user"`
}

// RefreshTokenRequest represents a refresh token request.
//
//	@Description	Refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIs..."`
}

// LogoutRequest represents a logout request.
//
//	@Description	Logout request
type LogoutRequest struct {
	RevokeAll bool `json:"revoke_all" example:"false"`
}

// DeviceCodeResponse represents a device code response.
//
//	@Description	Device code flow response
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code" example:"abc123xyz789..."`
	UserCode        string `json:"user_code" example:"ABCD-1234"`
	VerificationURI string `json:"verification_uri" example:"http://localhost:9000/device"`
	ExpiresIn       int    `json:"expires_in" example:"900"`
	Interval        int    `json:"interval" example:"5"`
}

// DeviceCodePollRequest represents a device code poll request.
//
//	@Description	Device code poll request
type DeviceCodePollRequest struct {
	DeviceCode string `json:"device_code" example:"abc123xyz789..."`
}

// DeviceCodePendingResponse represents a pending device code response.
//
//	@Description	Device code pending response
type DeviceCodePendingResponse struct {
	Error       string `json:"error" example:"authorization_pending"`
	Description string `json:"error_description" example:"The user has not yet authorized the device"`
}

// DeviceAuthorizeRequest represents a device authorization request.
//
//	@Description	Device authorization request
type DeviceAuthorizeRequest struct {
	UserCode string `json:"user_code" example:"ABCD-1234"`
}
