package authn

import (
	"context"

	"github.com/mujhtech/dagryn/pkg/database/models"
)

// UserInfo represents user information from an OAuth provider.
type UserInfo struct {
	ID        string
	Email     string
	Name      string
	AvatarURL string
	Provider  models.AuthProvider
}

// Provider defines the interface for OAuth providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// AuthURL returns the URL to redirect users for authentication.
	AuthURL(state string) string

	// Exchange exchanges an authorization code for tokens.
	Exchange(ctx context.Context, code string) (*Tokens, error)

	// GetUserInfo fetches user information using the access token.
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)

	// RefreshToken refreshes an access token using a refresh token.
	RefreshToken(ctx context.Context, refreshToken string) (*Tokens, error)
}

// Tokens represents OAuth tokens.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

// Config holds common OAuth configuration.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// DeviceCodeProvider defines additional methods for device code flow.
type DeviceCodeProvider interface {
	Provider

	// RequestDeviceCode initiates the device code flow.
	RequestDeviceCode(ctx context.Context) (*DeviceCode, error)

	// PollDeviceCode polls for the device code authorization.
	PollDeviceCode(ctx context.Context, deviceCode string) (*Tokens, error)
}

// DeviceCode represents a device authorization response.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}
