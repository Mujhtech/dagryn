package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAISuggestPublishRepo struct {
	analyses     map[uuid.UUID]*models.AIAnalysis
	suggestions  map[uuid.UUID][]models.AISuggestion
	publications map[string]*models.AIPublication
	created      []*models.AIPublication
	updated      []mockSuggestPubUpdate
}

type mockSuggestPubUpdate struct {
	ID              uuid.UUID
	Status          models.AISuggestionStatus
	GitHubCommentID *string
	FailureReason   *string
}

func newMockAISuggestPublishRepo() *mockAISuggestPublishRepo {
	return &mockAISuggestPublishRepo{
		analyses:     make(map[uuid.UUID]*models.AIAnalysis),
		suggestions:  make(map[uuid.UUID][]models.AISuggestion),
		publications: make(map[string]*models.AIPublication),
	}
}

func (m *mockAISuggestPublishRepo) GetAnalysisByID(_ context.Context, id uuid.UUID) (*models.AIAnalysis, error) {
	if a, ok := m.analyses[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockAISuggestPublishRepo) ListPendingSuggestionsByAnalysis(_ context.Context, analysisID uuid.UUID) ([]models.AISuggestion, error) {
	return m.suggestions[analysisID], nil
}

func (m *mockAISuggestPublishRepo) UpdateSuggestionStatus(_ context.Context, id uuid.UUID, status models.AISuggestionStatus, githubCommentID *string, failureReason *string) error {
	m.updated = append(m.updated, mockSuggestPubUpdate{ID: id, Status: status, GitHubCommentID: githubCommentID, FailureReason: failureReason})
	return nil
}

func (m *mockAISuggestPublishRepo) GetPublicationByRunAndDestination(_ context.Context, runID uuid.UUID, dest models.AIPublicationDestination) (*models.AIPublication, error) {
	key := fmt.Sprintf("%s:%s", runID, dest)
	if p, ok := m.publications[key]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockAISuggestPublishRepo) CreatePublication(_ context.Context, p *models.AIPublication) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	m.created = append(m.created, p)
	key := fmt.Sprintf("%s:%s", p.RunID, p.Destination)
	m.publications[key] = p
	return nil
}

func (m *mockAISuggestPublishRepo) UpdatePublication(_ context.Context, id uuid.UUID, status models.AIPublicationStatus, externalID *string, errorMessage *string) error {
	return nil
}

func TestAISuggestPublish_NoSuggestions_Skips(t *testing.T) {
	analysisID := uuid.New()
	runID := uuid.New()
	projectID := uuid.New()

	summary := "failure"
	confidence := 0.9
	mockRepo := newMockAISuggestPublishRepo()
	mockRepo.analyses[analysisID] = &models.AIAnalysis{
		ID:         analysisID,
		RunID:      runID,
		ProjectID:  projectID,
		Status:     models.AIAnalysisStatusSuccess,
		Summary:    &summary,
		Confidence: &confidence,
	}
	// No suggestions.

	handler := NewAISuggestPublishHandler(
		mockRepo, &mockRunRepo{runs: make(map[uuid.UUID]*models.Run)}, &mockProjectRepo{projects: make(map[uuid.UUID]*models.Project)},
		nil, nil, nil, nil,
		encrypt.NewNoOpEncrypt(),
		zerolog.Nop(),
	)

	payload := aiSuggestPublishPayload{
		AnalysisID: analysisID.String(),
		RunID:      runID.String(),
		ProjectID:  projectID.String(),
	}
	payloadBytes := encodePayload(t, payload)
	task := asynq.NewTask("ai_suggest:publish", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	assert.Empty(t, mockRepo.created)
}

func TestAISuggestPublish_NoPR_Skips(t *testing.T) {
	analysisID := uuid.New()
	runID := uuid.New()
	projectID := uuid.New()

	summary := "failure"
	confidence := 0.9
	commit := "abc123"
	repoURL := "https://github.com/test/repo"

	mockRepo := newMockAISuggestPublishRepo()
	mockRepo.analyses[analysisID] = &models.AIAnalysis{
		ID:         analysisID,
		RunID:      runID,
		ProjectID:  projectID,
		Status:     models.AIAnalysisStatusSuccess,
		Summary:    &summary,
		Confidence: &confidence,
	}
	mockRepo.suggestions[analysisID] = []models.AISuggestion{
		{ID: uuid.New(), AnalysisID: analysisID, RunID: runID, FilePath: "file.go", StartLine: 1, EndLine: 1, SuggestedCode: "new", Confidence: 0.9},
	}

	mockRuns := &mockRunRepo{runs: map[uuid.UUID]*models.Run{
		runID: {ID: runID, GitCommit: &commit, PRNumber: nil}, // No PR
	}}
	mockProjects := &mockProjectRepo{projects: map[uuid.UUID]*models.Project{
		projectID: {ID: projectID, RepoURL: &repoURL},
	}}

	handler := NewAISuggestPublishHandler(
		mockRepo, mockRuns, mockProjects,
		nil, nil, nil, nil,
		encrypt.NewNoOpEncrypt(),
		zerolog.Nop(),
	)

	payload := aiSuggestPublishPayload{
		AnalysisID: analysisID.String(),
		RunID:      runID.String(),
		ProjectID:  projectID.String(),
	}
	payloadBytes := encodePayload(t, payload)
	task := asynq.NewTask("ai_suggest:publish", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	assert.Empty(t, mockRepo.created)
}

func TestAISuggestPublish_AlreadyPublished_Skips(t *testing.T) {
	analysisID := uuid.New()
	runID := uuid.New()
	projectID := uuid.New()

	summary := "failure"
	confidence := 0.9

	mockRepo := newMockAISuggestPublishRepo()
	mockRepo.analyses[analysisID] = &models.AIAnalysis{
		ID:         analysisID,
		Status:     models.AIAnalysisStatusSuccess,
		Summary:    &summary,
		Confidence: &confidence,
	}
	mockRepo.suggestions[analysisID] = []models.AISuggestion{
		{ID: uuid.New(), AnalysisID: analysisID, RunID: runID, FilePath: "file.go", StartLine: 1, EndLine: 1, SuggestedCode: "new", Confidence: 0.9},
	}

	commit := "abc"
	pr := 42
	repoURL := "https://github.com/test/repo"

	mockRuns := &mockRunRepo{runs: map[uuid.UUID]*models.Run{
		runID: {ID: runID, GitCommit: &commit, PRNumber: &pr},
	}}
	mockProjects := &mockProjectRepo{projects: map[uuid.UUID]*models.Project{
		projectID: {ID: projectID, RepoURL: &repoURL},
	}}

	// Pre-existing publication.
	existingExtID := "12345"
	key := fmt.Sprintf("%s:%s", runID, models.AIPublicationDestGitHubPRReview)
	mockRepo.publications[key] = &models.AIPublication{
		ID:         uuid.New(),
		RunID:      runID,
		ExternalID: &existingExtID,
		Status:     models.AIPublicationStatusSent,
	}

	handler := NewAISuggestPublishHandler(
		mockRepo, mockRuns, mockProjects,
		nil, nil, nil, nil,
		encrypt.NewNoOpEncrypt(),
		zerolog.Nop(),
	)

	payload := aiSuggestPublishPayload{
		AnalysisID: analysisID.String(),
		RunID:      runID.String(),
		ProjectID:  projectID.String(),
	}
	payloadBytes := encodePayload(t, payload)
	task := asynq.NewTask("ai_suggest:publish", payloadBytes)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)
	// No new publication should be created.
	assert.Empty(t, mockRepo.created)
}

func TestBuildSuggestionCommentBody(t *testing.T) {
	s := models.AISuggestion{
		FilePath:      "handler.go",
		StartLine:     10,
		EndLine:       12,
		SuggestedCode: "if ptr != nil {\n    val := ptr.Field\n}",
		Explanation:   "Add nil check to prevent panic",
		Confidence:    0.85,
	}

	body := buildSuggestionCommentBody(s)
	assert.Contains(t, body, "Dagryn AI Suggestion")
	assert.Contains(t, body, "85%")
	assert.Contains(t, body, "nil check")
	assert.Contains(t, body, "```suggestion")
	assert.Contains(t, body, "ptr.Field")
}

func TestBuildReviewBody(t *testing.T) {
	summary := "Test suite failed due to nil pointer"
	analysis := &models.AIAnalysis{Summary: &summary}
	suggestions := []models.AISuggestion{
		{FilePath: "a.go"},
		{FilePath: "b.go"},
	}

	body := buildReviewBody(analysis, suggestions)
	assert.Contains(t, body, "Dagryn AI Code Suggestions")
	assert.Contains(t, body, "2 suggestion(s)")
	assert.Contains(t, body, "nil pointer")
	assert.Contains(t, body, "Apply suggestion")
}

func TestBuildReviewComments(t *testing.T) {
	suggestions := []models.AISuggestion{
		{
			FilePath:      "handler.go",
			StartLine:     10,
			EndLine:       12,
			SuggestedCode: "fixed code",
			Explanation:   "Fix nil pointer",
			Confidence:    0.9,
		},
		{
			FilePath:      "", // Invalid — should be skipped
			SuggestedCode: "something",
		},
		{
			FilePath:      "other.go",
			SuggestedCode: "", // Empty — should be skipped
		},
	}

	comments, nonDiff := buildReviewCommentsFiltered(suggestions, nil)
	assert.Len(t, comments, 1) // Only the first is valid
	assert.Len(t, nonDiff, 0)
	assert.Equal(t, "handler.go", comments[0].Path)
	assert.Equal(t, 12, *comments[0].Line)
}
