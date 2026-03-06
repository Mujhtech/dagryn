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

func testInput() aitypes.AnalysisInput {
	return aitypes.AnalysisInput{
		RunID:           "run-1",
		ProjectID:       "proj-1",
		WorkflowName:    "ci",
		GitBranch:       "main",
		TotalTasks:      3,
		CompletedTasks:  2,
		FailedTaskCount: 1,
		TaskGraph: []aitypes.TaskNode{
			{Name: "lint", Status: "success"},
			{Name: "test", Status: "failed", Needs: []string{"lint"}},
		},
		FailedTasks: []aitypes.FailedTaskEvidence{
			{TaskName: "test", ExitCode: 1, StderrTail: "FAIL TestFoo"},
		},
	}
}

func validResponseJSON() string {
	out := aitypes.AnalysisOutput{
		Summary:            "Tests failed",
		RootCause:          "Missing nil check",
		Confidence:         0.85,
		Evidence:           []aitypes.EvidenceItem{{Task: "test", Reason: "panic"}},
		LikelyFiles:        []string{"handler.go"},
		RecommendedActions: []string{"Add nil check"},
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// openAITestResponse builds a chat completion response matching the SDK expectations.
func openAITestResponse(content string) map[string]any {
	return map[string]any{
		"id": "chatcmpl-test", "object": "chat.completion", "model": "gpt-4o",
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": "stop",
		}},
	}
}

// openAIErrorResponse builds a JSON error response matching the SDK expectations.
func openAIErrorResponse(message string) map[string]any {
	return map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    "error",
		},
	}
}

func TestOpenAI_AnalyzeFailure_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAITestResponse(validResponseJSON()))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "test-key", Model: "gpt-4o"}, zerolog.Nop(), srv.URL+"/")

	out, err := p.AnalyzeFailure(context.Background(), testInput())
	require.NoError(t, err)
	assert.Equal(t, "Tests failed", out.Summary)
	assert.Equal(t, 0.85, out.Confidence)
}

func TestOpenAI_AnalyzeFailure_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(openAIErrorResponse("internal error"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	_, err := p.AnalyzeFailure(context.Background(), testInput())
	require.Error(t, err)
	var pe *aitypes.ProviderError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 500, pe.StatusCode)
	assert.True(t, pe.Retryable)
}

func TestOpenAI_AnalyzeFailure_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(openAIErrorResponse("rate limited"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	_, err := p.AnalyzeFailure(context.Background(), testInput())
	require.Error(t, err)
	var pe *aitypes.ProviderError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 429, pe.StatusCode)
	assert.True(t, pe.Retryable)
}

func TestOpenAI_AnalyzeFailure_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(openAIErrorResponse("invalid key"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "bad"}, zerolog.Nop(), srv.URL+"/")

	_, err := p.AnalyzeFailure(context.Background(), testInput())
	require.Error(t, err)
	var pe *aitypes.ProviderError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 401, pe.StatusCode)
	assert.False(t, pe.Retryable)
}

func TestOpenAI_AnalyzeFailure_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAITestResponse("not json at all"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	_, err := p.AnalyzeFailure(context.Background(), testInput())
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrInvalidResponse)
}

func TestOpenAI_AnalyzeFailure_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	_, err := p.AnalyzeFailure(ctx, testInput())
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrProviderTimeout)
}

func TestBuildUserMessage_Content(t *testing.T) {
	input := testInput()
	input.GitCommit = "abc123"
	input.CommitMessage = "fix tests"
	input.PRTitle = "Fix failing tests"
	input.PRNumber = 42

	msg := buildUserMessage(input)
	assert.Contains(t, msg, "## Run Context")
	assert.Contains(t, msg, "run-1")
	assert.Contains(t, msg, "## Git Context")
	assert.Contains(t, msg, "abc123")
	assert.Contains(t, msg, "#42 Fix failing tests")
	assert.Contains(t, msg, "## Task Graph")
	assert.Contains(t, msg, "## Failed Tasks")
	assert.Contains(t, msg, "FAIL TestFoo")
}

func TestSystemPrompt_ContainsSchema(t *testing.T) {
	assert.Contains(t, systemPrompt, "summary")
	assert.Contains(t, systemPrompt, "root_cause")
	assert.Contains(t, systemPrompt, "confidence")
	assert.Contains(t, systemPrompt, "evidence")
	assert.Contains(t, systemPrompt, "likely_files")
	assert.Contains(t, systemPrompt, "recommended_actions")
}

func TestNewProvider_GeminiRouting(t *testing.T) {
	// Verify that "google" provider creates a provider with the Gemini default model.
	p, err := NewProvider(ProviderConfig{
		BackendMode: "byok",
		Provider:    "google",
		APIKey:      "test-key",
	}, zerolog.Nop())
	require.NoError(t, err)

	oai, ok := p.(*OpenAIProvider)
	require.True(t, ok)
	assert.Equal(t, ManagedModels["google"][1], oai.model)
}
