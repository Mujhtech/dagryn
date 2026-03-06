package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAISuggestRepo struct {
	repo.AIStore // embed for interface satisfaction
	analyses     map[uuid.UUID]*models.AIAnalysis
	suggestions  []*models.AISuggestion
}

func newMockAISuggestRepo() *mockAISuggestRepo {
	return &mockAISuggestRepo{
		analyses: make(map[uuid.UUID]*models.AIAnalysis),
	}
}

func (m *mockAISuggestRepo) GetAnalysisByID(_ context.Context, id uuid.UUID) (*models.AIAnalysis, error) {
	if a, ok := m.analyses[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockAISuggestRepo) CreateSuggestion(_ context.Context, s *models.AISuggestion) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	m.suggestions = append(m.suggestions, s)
	return nil
}

func (m *mockAISuggestRepo) UpdateSuggestionStatus(_ context.Context, _ uuid.UUID, _ models.AISuggestionStatus, _ *string, _ *string) error {
	return nil
}

func TestAISuggest_HappyPath(t *testing.T) {
	analysisID := uuid.New()
	runID := uuid.New()
	projectID := uuid.New()

	summary := "Test failure in handler"
	confidence := 0.9
	evidence, _ := json.Marshal([]aitypes.EvidenceItem{{Task: "test", Reason: "nil pointer"}})

	mockRepo := newMockAISuggestRepo()
	mockRepo.analyses[analysisID] = &models.AIAnalysis{
		ID:           analysisID,
		RunID:        runID,
		ProjectID:    projectID,
		Status:       models.AIAnalysisStatusSuccess,
		Summary:      &summary,
		Confidence:   &confidence,
		EvidenceJSON: evidence,
	}

	branch := "main"
	commit := "abc123"
	mockRuns := &mockRunRepo{
		runs: map[uuid.UUID]*models.Run{
			runID: {ID: runID, ProjectID: projectID, GitBranch: &branch, GitCommit: &commit},
		},
	}

	handler := NewAISuggestHandler(
		store.Store{
			AI:   mockRepo,
			Runs: mockRuns,
		},
		encrypt.NewNoOpEncrypt(),
		DefaultAISuggestConfig(),
		&AIAnalysisConfig{
			Enabled:     true,
			BackendMode: "managed",
			Provider:    "openai",
			APIKey:      "test-key",
		},
		zerolog.Nop(),
	)

	payload := aiSuggestPayload{
		AnalysisID: analysisID.String(),
		RunID:      runID.String(),
		ProjectID:  projectID.String(),
	}
	payloadBytes := encodePayload(t, payload)
	task := asynq.NewTask("ai_suggest:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	// Provider construction may fail in test (no real API), but handler should not error.
	require.NoError(t, err)
}

func TestAISuggest_AnalysisNotSuccess_Skips(t *testing.T) {
	analysisID := uuid.New()

	mockRepo := newMockAISuggestRepo()
	mockRepo.analyses[analysisID] = &models.AIAnalysis{
		ID:     analysisID,
		Status: models.AIAnalysisStatusFailed,
	}

	handler := NewAISuggestHandler(
		store.Store{
			AI:   mockRepo,
			Runs: &mockRunRepo{runs: make(map[uuid.UUID]*models.Run)},
		},
		encrypt.NewNoOpEncrypt(),
		DefaultAISuggestConfig(),
		nil, // no server config
		zerolog.Nop(),
	)

	payload := aiSuggestPayload{
		AnalysisID: analysisID.String(),
		RunID:      uuid.New().String(),
		ProjectID:  uuid.New().String(),
	}
	payloadBytes := encodePayload(t, payload)
	task := asynq.NewTask("ai_suggest:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	assert.Empty(t, mockRepo.suggestions)
}

func TestAISuggest_NoConfig_Skips(t *testing.T) {
	analysisID := uuid.New()
	runID := uuid.New()

	summary := "fail"
	confidence := 0.9
	mockRepo := newMockAISuggestRepo()
	mockRepo.analyses[analysisID] = &models.AIAnalysis{
		ID:         analysisID,
		RunID:      runID,
		Status:     models.AIAnalysisStatusSuccess,
		Summary:    &summary,
		Confidence: &confidence,
	}

	branch := "main"
	mockRuns := &mockRunRepo{
		runs: map[uuid.UUID]*models.Run{
			runID: {ID: runID, GitBranch: &branch},
		},
	}

	// No server config and no AI config in payload — should skip suggestions.
	handler := NewAISuggestHandler(
		store.Store{
			AI:   mockRepo,
			Runs: mockRuns,
		},
		encrypt.NewNoOpEncrypt(),
		DefaultAISuggestConfig(),
		nil, // No server config
		zerolog.Nop(),
	)

	payload := aiSuggestPayload{
		AnalysisID: analysisID.String(),
		RunID:      runID.String(),
		ProjectID:  uuid.New().String(),
	}
	payloadBytes := encodePayload(t, payload)
	task := asynq.NewTask("ai_suggest:run", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	assert.Empty(t, mockRepo.suggestions)
}

func TestValidateSuggestion_BlockedPath(t *testing.T) {
	cfg := AISuggestConfig{
		MinConfidence:      0.5,
		MaxSuggestionLines: 20,
		BlockedPaths:       []string{"internal/db/migrations/*"},
	}

	reason := validateSuggestion(aitypes.SuggestionOutput{
		FilePath:      "internal/db/migrations/001.sql",
		StartLine:     1,
		EndLine:       1,
		SuggestedCode: "new",
		Confidence:    0.9,
	}, cfg)
	assert.Contains(t, reason, "blocked path")
}

func TestValidateSuggestion_EmptySuggestedCode(t *testing.T) {
	cfg := AISuggestConfig{MinConfidence: 0.5, MaxSuggestionLines: 20}

	reason := validateSuggestion(aitypes.SuggestionOutput{
		FilePath:      "file.go",
		StartLine:     1,
		EndLine:       1,
		SuggestedCode: "",
		Confidence:    0.9,
	}, cfg)
	assert.Equal(t, "empty suggested code", reason)
}

func TestValidateSuggestion_BelowConfidence(t *testing.T) {
	cfg := DefaultAISuggestConfig()

	reason := validateSuggestion(aitypes.SuggestionOutput{
		FilePath:      "handler.go",
		StartLine:     1,
		EndLine:       1,
		OriginalCode:  "old",
		SuggestedCode: "new",
		Explanation:   "fix",
		Confidence:    0.3, // Below default 0.70 threshold
	}, cfg)
	assert.Contains(t, reason, "below threshold")
}

func TestValidateSuggestion_Valid(t *testing.T) {
	cfg := DefaultAISuggestConfig()

	reason := validateSuggestion(aitypes.SuggestionOutput{
		FilePath:      "handler.go",
		StartLine:     1,
		EndLine:       1,
		OriginalCode:  "old",
		SuggestedCode: "new",
		Explanation:   "fix",
		Confidence:    0.85,
	}, cfg)
	assert.Empty(t, reason)
}

func TestAISuggest_BuildProviderConfig_BYOK(t *testing.T) {
	handler := &AISuggestHandler{
		serverConfig: &AIAnalysisConfig{
			BackendMode: "managed",
			Provider:    "openai",
			APIKey:      "server-key",
		},
	}

	projCfg := &aiProjectConfig{
		BackendMode: "byok",
		Provider:    "openai",
		APIKey:      "user-key",
	}

	cfg := handler.buildSuggestProviderConfig(projCfg)
	assert.Equal(t, "byok", cfg.BackendMode)
	assert.Equal(t, "user-key", cfg.APIKey)
}

func TestAISuggest_BuildProviderConfig_Managed_Fallback(t *testing.T) {
	handler := &AISuggestHandler{
		serverConfig: &AIAnalysisConfig{
			BackendMode:    "managed",
			Provider:       "openai",
			APIKey:         "server-key",
			TimeoutSeconds: 30,
		},
	}

	projCfg := &aiProjectConfig{
		BackendMode: "managed",
		Provider:    "", // Should fall back to server
	}

	cfg := handler.buildSuggestProviderConfig(projCfg)
	assert.Equal(t, "managed", cfg.BackendMode)
	assert.Equal(t, "server-key", cfg.APIKey)
	assert.Equal(t, "openai", cfg.Provider)
	assert.Equal(t, 30, cfg.TimeoutSeconds)
}

func TestAISuggest_BuildProviderConfig_NilProjectConfig(t *testing.T) {
	handler := &AISuggestHandler{
		serverConfig: &AIAnalysisConfig{
			BackendMode: "managed",
			Provider:    "openai",
			APIKey:      "server-key",
		},
	}

	cfg := handler.buildSuggestProviderConfig(nil)
	assert.Equal(t, "managed", cfg.BackendMode)
	assert.Equal(t, "server-key", cfg.APIKey)
}
