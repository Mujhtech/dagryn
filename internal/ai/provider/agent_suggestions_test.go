package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mujhtech/dagryn/internal/ai/aitypes"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSuggestions_HappyPath(t *testing.T) {
	suggestions := []aitypes.SuggestionOutput{
		{
			FilePath:      "handler.go",
			StartLine:     42,
			EndLine:       42,
			OriginalCode:  "val := ptr.Field",
			SuggestedCode: "if ptr != nil { val := ptr.Field }",
			Explanation:   "Add nil check",
			Confidence:    0.85,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/suggest", r.URL.Path)
		assert.Equal(t, "Bearer agent-key", r.Header.Get("Authorization"))
		assert.Equal(t, aitypes.ContractVersion, r.Header.Get("X-Dagryn-Contract-Version"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		_ = json.NewEncoder(w).Encode(suggestions)
	}))
	defer srv.Close()

	a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL, AgentToken: "agent-key"}, zerolog.Nop())
	result, err := a.ProposeSuggestions(context.Background(), testSuggestionInput())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "handler.go", result[0].FilePath)
	assert.Equal(t, 0.85, result[0].Confidence)
}

func TestAgentSuggestions_Error(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryable bool
	}{
		{"bad_request", 400, false},
		{"rate_limit", 429, true},
		{"server_error", 500, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("error"))
			}))
			defer srv.Close()

			a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL}, zerolog.Nop())
			_, err := a.ProposeSuggestions(context.Background(), testSuggestionInput())
			require.Error(t, err)
			var pe *aitypes.ProviderError
			require.ErrorAs(t, err, &pe)
			assert.Equal(t, tt.status, pe.StatusCode)
			assert.Equal(t, tt.retryable, pe.Retryable)
		})
	}
}

func TestAgentSuggestions_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]aitypes.SuggestionOutput{})
	}))
	defer srv.Close()

	a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL}, zerolog.Nop())
	result, err := a.ProposeSuggestions(context.Background(), testSuggestionInput())
	require.NoError(t, err)
	assert.Empty(t, result)
}
