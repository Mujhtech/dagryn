package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mujhtech/dagryn/internal/ai/aitypes"
	"github.com/rs/zerolog"
)

const defaultAgentTimeout = 60 * time.Second

// AgentAdapter implements AgentClient for BYOA (bring your own agent) endpoints.
type AgentAdapter struct {
	endpoint   string
	authToken  string
	timeout    time.Duration
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewAgentAdapter creates an adapter for an external agent endpoint.
func NewAgentAdapter(cfg ProviderConfig, logger zerolog.Logger) *AgentAdapter {
	timeout := defaultAgentTimeout
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	return &AgentAdapter{
		endpoint:   cfg.AgentEndpoint,
		authToken:  cfg.AgentToken,
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger.With().Str("provider", "agent").Logger(),
	}
}

// HealthCheck verifies the agent endpoint is reachable.
func (a *AgentAdapter) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint+"/health", nil)
	if err != nil {
		return fmt.Errorf("agent: create health request: %w", err)
	}
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", aitypes.ErrProviderUnavailable, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &aitypes.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Retryable:  false,
		}
	}

	var health struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("%w: decode health response: %v", aitypes.ErrInvalidResponse, err)
	}
	if health.Status != "ok" {
		return fmt.Errorf("agent: unhealthy status: %s", health.Status)
	}
	return nil
}

// AnalyzeFailure sends the evidence to the BYOA agent and returns the analysis.
func (a *AgentAdapter) AnalyzeFailure(ctx context.Context, input aitypes.AnalysisInput) (*aitypes.AnalysisOutput, error) {
	bodyBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("agent: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+"/analyze", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("agent: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dagryn-Contract-Version", aitypes.ContractVersion)
	if a.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.authToken)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, aitypes.ErrProviderTimeout
		}
		return nil, fmt.Errorf("%w: %v", aitypes.ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("agent: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == 429 || resp.StatusCode >= 500
		return nil, &aitypes.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Retryable:  retryable,
		}
	}

	output, err := ParseAnalysisOutput(respBody)
	if err != nil {
		return nil, err
	}

	a.logger.Debug().
		Float64("confidence", output.Confidence).
		Msg("agent analysis complete")

	return output, nil
}
