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

func testSuggestionInput() aitypes.SuggestionInput {
	return aitypes.SuggestionInput{
		RunID:     "run-1",
		ProjectID: "proj-1",
		Analysis: aitypes.AnalysisOutput{
			Summary:    "Tests failed due to nil pointer",
			RootCause:  "Missing nil check in handler.go",
			Confidence: 0.9,
			Evidence: []aitypes.EvidenceItem{
				{Task: "test", Reason: "panic: nil pointer dereference"},
			},
			LikelyFiles:        []string{"handler.go"},
			RecommendedActions: []string{"Add nil check before accessing field"},
		},
		GitBranch: "main",
		GitCommit: "abc123",
		FailedTasks: []aitypes.FailedTaskEvidence{
			{TaskName: "test", ExitCode: 1, StderrTail: "panic: nil pointer dereference"},
		},
	}
}

func validSuggestionsJSON() string {
	suggestions := []aitypes.SuggestionOutput{
		{
			FilePath:      "handler.go",
			StartLine:     42,
			EndLine:       44,
			OriginalCode:  "val := ptr.Field",
			SuggestedCode: "if ptr != nil {\n  val := ptr.Field\n}",
			Explanation:   "Add nil check to prevent panic",
			Confidence:    0.85,
		},
	}
	b, _ := json.Marshal(suggestions)
	return string(b)
}

func TestOpenAISuggestions_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAITestResponse(validSuggestionsJSON()))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "test-key", Model: "gpt-4o"}, zerolog.Nop(), srv.URL+"/")

	suggestions, err := p.ProposeSuggestions(context.Background(), testSuggestionInput())
	require.NoError(t, err)
	require.Len(t, suggestions, 1)
	assert.Equal(t, "handler.go", suggestions[0].FilePath)
	assert.Equal(t, 42, suggestions[0].StartLine)
	assert.Equal(t, 44, suggestions[0].EndLine)
	assert.Equal(t, 0.85, suggestions[0].Confidence)
}

func TestOpenAISuggestions_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAITestResponse("[]"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	suggestions, err := p.ProposeSuggestions(context.Background(), testSuggestionInput())
	require.NoError(t, err)
	assert.Empty(t, suggestions)
}

func TestOpenAISuggestions_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(openAIErrorResponse("internal error"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	_, err := p.ProposeSuggestions(context.Background(), testSuggestionInput())
	require.Error(t, err)
	var pe *aitypes.ProviderError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 500, pe.StatusCode)
	assert.True(t, pe.Retryable)
}

func TestOpenAISuggestions_WrappedObject(t *testing.T) {
	// OpenAI json_object mode wraps array in object.
	wrapped := map[string]any{
		"suggestions": []aitypes.SuggestionOutput{
			{
				FilePath:      "main.go",
				StartLine:     10,
				EndLine:       10,
				OriginalCode:  "old",
				SuggestedCode: "new",
				Explanation:   "fix",
				Confidence:    0.9,
			},
		},
	}
	wrappedJSON, _ := json.Marshal(wrapped)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAITestResponse(string(wrappedJSON)))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	suggestions, err := p.ProposeSuggestions(context.Background(), testSuggestionInput())
	require.NoError(t, err)
	require.Len(t, suggestions, 1)
	assert.Equal(t, "main.go", suggestions[0].FilePath)
}

func TestOpenAISuggestions_InvalidSuggestionsFiltered(t *testing.T) {
	// Mix of valid and invalid suggestions — invalid ones should be filtered.
	suggestions := []aitypes.SuggestionOutput{
		{FilePath: "", StartLine: 1, EndLine: 1, SuggestedCode: "new", Confidence: 0.9},     // empty path
		{FilePath: "f.go", StartLine: 0, EndLine: 1, SuggestedCode: "new", Confidence: 0.9}, // start_line 0
		{FilePath: "f.go", StartLine: 5, EndLine: 3, SuggestedCode: "new", Confidence: 0.9}, // start > end
		{FilePath: "f.go", StartLine: 1, EndLine: 1, SuggestedCode: "", Confidence: 0.9},    // empty code
		{FilePath: "f.go", StartLine: 1, EndLine: 1, SuggestedCode: "fix", Confidence: 0.9}, // valid
	}
	b, _ := json.Marshal(suggestions)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAITestResponse(string(b)))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	result, err := p.ProposeSuggestions(context.Background(), testSuggestionInput())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "f.go", result[0].FilePath)
}

func TestOpenAISuggestions_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := NewOpenAIProvider(ProviderConfig{APIKey: "key"}, zerolog.Nop(), srv.URL+"/")

	_, err := p.ProposeSuggestions(ctx, testSuggestionInput())
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrProviderTimeout)
}

func TestBuildSuggestionUserMessage_Content(t *testing.T) {
	input := testSuggestionInput()
	msg := buildSuggestionUserMessage(input)

	assert.Contains(t, msg, "## Analysis Summary")
	assert.Contains(t, msg, "nil pointer")
	assert.Contains(t, msg, "## Evidence")
	assert.Contains(t, msg, "## Likely Files")
	assert.Contains(t, msg, "handler.go")
	assert.Contains(t, msg, "## Git Context")
	assert.Contains(t, msg, "main")
	assert.Contains(t, msg, "## Failed Tasks")
	assert.Contains(t, msg, "panic")
}
