package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentAdapter_HealthCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL, AgentToken: "test-token"}, zerolog.Nop())
	err := a.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestAgentAdapter_HealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "degraded"})
	}))
	defer srv.Close()

	a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL}, zerolog.Nop())
	err := a.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy")
}

func TestAgentAdapter_HealthCheck_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL}, zerolog.Nop())
	err := a.HealthCheck(context.Background())
	require.Error(t, err)
	var pe *aitypes.ProviderError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 503, pe.StatusCode)
}

func TestAgentAdapter_AnalyzeFailure_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/analyze", r.URL.Path)
		assert.Equal(t, "Bearer agent-key", r.Header.Get("Authorization"))
		assert.Equal(t, aitypes.ContractVersion, r.Header.Get("X-Dagryn-Contract-Version"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		_, _ = w.Write([]byte(validResponseJSON()))
	}))
	defer srv.Close()

	a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL, AgentToken: "agent-key"}, zerolog.Nop())
	out, err := a.AnalyzeFailure(context.Background(), testInput())
	require.NoError(t, err)
	assert.Equal(t, "Tests failed", out.Summary)
}

func TestAgentAdapter_AnalyzeFailure_ErrorCodes(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryable bool
	}{
		{"bad_request", 400, false},
		{"unprocessable", 422, false},
		{"rate_limit", 429, true},
		{"server_error", 500, true},
		{"unavailable", 503, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("error"))
			}))
			defer srv.Close()

			a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL}, zerolog.Nop())
			_, err := a.AnalyzeFailure(context.Background(), testInput())
			require.Error(t, err)
			var pe *aitypes.ProviderError
			require.ErrorAs(t, err, &pe)
			assert.Equal(t, tt.status, pe.StatusCode)
			assert.Equal(t, tt.retryable, pe.Retryable)
		})
	}
}

func TestAgentAdapter_ContractVersionHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1", r.Header.Get("X-Dagryn-Contract-Version"))
		_, _ = w.Write([]byte(validResponseJSON()))
	}))
	defer srv.Close()

	a := NewAgentAdapter(ProviderConfig{AgentEndpoint: srv.URL}, zerolog.Nop())
	_, err := a.AnalyzeFailure(context.Background(), testInput())
	assert.NoError(t, err)
}
