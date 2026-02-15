package handlers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock blob cleanup repo ---

type mockAIBlobCleanupRepo struct {
	deletedCutoff time.Time
	deletedCount  int64
	deleteErr     error
}

func newMockAIBlobCleanupRepo() *mockAIBlobCleanupRepo {
	return &mockAIBlobCleanupRepo{}
}

func (m *mockAIBlobCleanupRepo) DeleteExpiredBlobKeys(_ context.Context, olderThan time.Time) (int64, error) {
	m.deletedCutoff = olderThan
	if m.deleteErr != nil {
		return 0, m.deleteErr
	}
	return m.deletedCount, nil
}

// --- Tests ---

func TestAIBlobCleanup_HappyPath(t *testing.T) {
	mockRepo := newMockAIBlobCleanupRepo()
	mockRepo.deletedCount = 5

	handler := NewAIBlobCleanupHandler(mockRepo, 168, zerolog.Nop())

	task := asynq.NewTask("ai_blob_cleanup:run", nil)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)

	// Cutoff should be approximately 168 hours ago.
	expectedCutoff := time.Now().Add(-168 * time.Hour)
	assert.WithinDuration(t, expectedCutoff, mockRepo.deletedCutoff, 5*time.Second)
}

func TestAIBlobCleanup_NothingToClear(t *testing.T) {
	mockRepo := newMockAIBlobCleanupRepo()
	mockRepo.deletedCount = 0

	handler := NewAIBlobCleanupHandler(mockRepo, 72, zerolog.Nop())

	task := asynq.NewTask("ai_blob_cleanup:run", nil)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)

	// Cutoff should be approximately 72 hours ago.
	expectedCutoff := time.Now().Add(-72 * time.Hour)
	assert.WithinDuration(t, expectedCutoff, mockRepo.deletedCutoff, 5*time.Second)
}

func TestAIBlobCleanup_RepoError_ReturnsError(t *testing.T) {
	mockRepo := newMockAIBlobCleanupRepo()
	mockRepo.deleteErr = fmt.Errorf("db connection lost")

	handler := NewAIBlobCleanupHandler(mockRepo, 168, zerolog.Nop())

	task := asynq.NewTask("ai_blob_cleanup:run", nil)
	err := handler.Handle(context.Background(), task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestAIBlobCleanup_DefaultTTL(t *testing.T) {
	mockRepo := newMockAIBlobCleanupRepo()
	mockRepo.deletedCount = 0

	// Pass 0 — should default to 168 hours.
	handler := NewAIBlobCleanupHandler(mockRepo, 0, zerolog.Nop())

	task := asynq.NewTask("ai_blob_cleanup:run", nil)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)

	expectedCutoff := time.Now().Add(-168 * time.Hour)
	assert.WithinDuration(t, expectedCutoff, mockRepo.deletedCutoff, 5*time.Second)
}

func TestAIBlobCleanup_NegativeTTL_DefaultsTo168(t *testing.T) {
	mockRepo := newMockAIBlobCleanupRepo()
	mockRepo.deletedCount = 0

	handler := NewAIBlobCleanupHandler(mockRepo, -10, zerolog.Nop())

	task := asynq.NewTask("ai_blob_cleanup:run", nil)
	err := handler.Handle(context.Background(), task)
	require.NoError(t, err)

	expectedCutoff := time.Now().Add(-168 * time.Hour)
	assert.WithinDuration(t, expectedCutoff, mockRepo.deletedCutoff, 5*time.Second)
}
