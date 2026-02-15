package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mujhtech/dagryn/internal/ai/aitypes"
)

// ProposeSuggestions sends the suggestion input to the BYOA agent and returns suggestions.
func (a *AgentAdapter) ProposeSuggestions(ctx context.Context, input aitypes.SuggestionInput) ([]aitypes.SuggestionOutput, error) {
	bodyBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("agent: marshal suggest request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+"/suggest", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("agent: create suggest request: %w", err)
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
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("agent: read suggest response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == 429 || resp.StatusCode >= 500
		return nil, &aitypes.ProviderError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Retryable:  retryable,
		}
	}

	var suggestions []aitypes.SuggestionOutput
	if err := json.Unmarshal(respBody, &suggestions); err != nil {
		return nil, fmt.Errorf("%w: parse agent suggestions: %v", aitypes.ErrInvalidResponse, err)
	}

	a.logger.Debug().
		Int("suggestions", len(suggestions)).
		Msg("agent suggestions complete")

	return suggestions, nil
}
