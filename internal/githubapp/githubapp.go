package githubapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Client provides helpers for interacting with the GitHub App API.
type Client struct {
	appID         int64
	privateKey    []byte
	webhookSecret string
}

// Config holds the minimal configuration needed to talk to the GitHub App API.
type Config struct {
	AppID         int64
	PrivateKey    string
	WebhookSecret string
}

// NewClient constructs a Client from Config.
func NewClient(cfg Config) (*Client, error) {
	if cfg.AppID == 0 || cfg.PrivateKey == "" {
		return nil, fmt.Errorf("github app configuration is incomplete")
	}
	return &Client{
		appID:         cfg.AppID,
		privateKey:    decodePrivateKey(cfg.PrivateKey),
		webhookSecret: cfg.WebhookSecret,
	}, nil
}

// AppJWT generates a short-lived JWT for authenticating as the GitHub App.
func (c *Client) AppJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(8 * time.Minute).Unix(),
		"iss": c.appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	key, err := jwt.ParseRSAPrivateKeyFromPEM(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse GitHub App private key: %w", err)
	}
	return token.SignedString(key)
}

// InstallationToken represents a GitHub installation access token.
type InstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

// VerifyWebhookSignature verifies the X-Hub-Signature-256 header and returns true
// if the payload is valid.
func (c *Client) VerifyWebhookSignature(payload []byte, signatureHeader string) bool {
	if c.webhookSecret == "" {
		// If no secret is configured, skip verification (not recommended for prod).
		return true
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	signatureHex := signatureHeader[len(prefix):]
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	actual, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, actual)
}

// decodePrivateKey allows the private key to be provided either as raw PEM or
// base64-encoded; it returns PEM bytes.
func decodePrivateKey(raw string) []byte {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(trimmed, "-----BEGIN") {
		if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
			return decoded
		}
	}
	return []byte(trimmed)
}

// installationTokenResponse matches the GitHub API response for installation tokens.
type installationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// FetchInstallationToken exchanges an installation ID for an installation access token.
func (c *Client) FetchInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error) {
	jwtToken, err := c.AppJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID),
		strings.NewReader(`{}`),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github installation token request failed with status %d", resp.StatusCode)
	}

	var body installationTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	return &InstallationToken{
		Token:     body.Token,
		ExpiresAt: body.ExpiresAt,
	}, nil
}
