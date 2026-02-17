package licensing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ServerConfig holds the license server connection settings.
type ServerConfig struct {
	BaseURL         string        // https://license.dagryn.dev
	Timeout         time.Duration // default 10s
	CheckRevocation bool          // default true, set false for air-gapped
}

// ActivationRequest is sent to the License Server to activate a key.
type ActivationRequest struct {
	LicenseKey   string `json:"license_key"`
	InstanceID   string `json:"instance_id"`
	InstanceName string `json:"instance_name,omitempty"`
	Version      string `json:"dagryn_version"`
	Hostname     string `json:"hostname,omitempty"`
}

// ActivationResponse is returned from the activate endpoint.
type ActivationResponse struct {
	Activated bool   `json:"activated"`
	LicenseID string `json:"license_id"`
	Customer  string `json:"customer"`
	Edition   string `json:"edition"`
	Seats     int    `json:"seats"`
	ExpiresAt string `json:"expires_at"`
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
}

// CheckRequest is sent for periodic validity checks.
type CheckRequest struct {
	LicenseID  string `json:"license_id"`
	InstanceID string `json:"instance_id"`
	Version    string `json:"dagryn_version"`
}

// CheckResponse is returned from the check endpoint.
type CheckResponse struct {
	Valid            bool   `json:"valid"`
	Revoked          bool   `json:"revoked"`
	RenewalAvailable bool   `json:"renewal_available"`
	RenewalURL       string `json:"renewal_url,omitempty"`
}

// DeactivateRequest is sent to free an activation slot.
type DeactivateRequest struct {
	LicenseID  string `json:"license_id"`
	InstanceID string `json:"instance_id"`
}

// ServerClient communicates with the external License Server.
type ServerClient struct {
	config     ServerConfig
	httpClient *http.Client
}

// NewServerClient creates a new License Server client.
func NewServerClient(cfg ServerConfig) *ServerClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &ServerClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Activate registers this instance with the License Server.
func (c *ServerClient) Activate(ctx context.Context, req ActivationRequest) (*ActivationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal activation request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/v1/activate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create activation request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("license server unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result ActivationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode activation response: %w", err)
	}
	return &result, nil
}

// Check performs a periodic validity + revocation check.
func (c *ServerClient) Check(ctx context.Context, req CheckRequest) (*CheckResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal check request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/v1/check", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create check request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("license server unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result CheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode check response: %w", err)
	}
	return &result, nil
}

// Deactivate removes this instance's activation.
func (c *ServerClient) Deactivate(ctx context.Context, req DeactivateRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal deactivate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/v1/deactivate", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create deactivate request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("license server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deactivation failed with status %d", resp.StatusCode)
	}
	return nil
}
