package service

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations ---

// mockAuditRepo is a test double for repo.AuditLogStore.
type mockAuditRepo struct {
	created    []*models.AuditLog
	lastHash   string
	lastSeq    int64
	chainLogs  []models.AuditLog
	policies   []models.AuditLogRetentionPolicy
	retention  *models.AuditLogRetentionPolicy
	deleted    int64
	listResult *repo.AuditLogListResult

	// Tracking calls
	createCalls int
	deleteCalls int
}

func (m *mockAuditRepo) Create(_ context.Context, log *models.AuditLog) error {
	m.createCalls++
	log.ID = uuid.New()
	log.SequenceNum = int64(m.createCalls)
	log.CreatedAt = time.Now().UTC()
	m.created = append(m.created, log)
	m.lastHash = log.EntryHash
	m.lastSeq = log.SequenceNum
	return nil
}

func (m *mockAuditRepo) GetByID(_ context.Context, id uuid.UUID) (*models.AuditLog, error) {
	for _, l := range m.created {
		if l.ID == id {
			return l, nil
		}
	}
	return nil, repo.ErrNotFound
}

func (m *mockAuditRepo) List(_ context.Context, _ repo.AuditLogFilter) (*repo.AuditLogListResult, error) {
	if m.listResult != nil {
		return m.listResult, nil
	}
	return &repo.AuditLogListResult{Data: []models.AuditLog{}}, nil
}

func (m *mockAuditRepo) DeleteBefore(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	m.deleteCalls++
	return m.deleted, nil
}

func (m *mockAuditRepo) GetLastHash(_ context.Context, _ uuid.UUID) (string, int64, error) {
	return m.lastHash, m.lastSeq, nil
}

func (m *mockAuditRepo) GetRetentionPolicy(_ context.Context, _ uuid.UUID) (*models.AuditLogRetentionPolicy, error) {
	if m.retention != nil {
		return m.retention, nil
	}
	return nil, repo.ErrNotFound
}

func (m *mockAuditRepo) UpsertRetentionPolicy(_ context.Context, p *models.AuditLogRetentionPolicy) error {
	m.retention = p
	return nil
}

func (m *mockAuditRepo) ListAllRetentionPolicies(_ context.Context) ([]models.AuditLogRetentionPolicy, error) {
	return m.policies, nil
}

func (m *mockAuditRepo) CountByTeam(_ context.Context, _ uuid.UUID) (int64, error) {
	return int64(len(m.created)), nil
}

func (m *mockAuditRepo) ListChain(_ context.Context, _ uuid.UUID, afterSeq int64, _ int) ([]models.AuditLog, error) {
	// Filter chain logs to those with sequence_num > afterSeq (simulates DB behavior)
	var result []models.AuditLog
	for _, l := range m.chainLogs {
		if l.SequenceNum > afterSeq {
			result = append(result, l)
		}
	}
	return result, nil
}

// --- Webhook mock methods (satisfy the extended AuditLogStore interface) ---

func (m *mockAuditRepo) CreateWebhook(_ context.Context, _ *models.AuditWebhook) error {
	return nil
}
func (m *mockAuditRepo) GetWebhookByID(_ context.Context, _ uuid.UUID) (*models.AuditWebhook, error) {
	return nil, repo.ErrNotFound
}
func (m *mockAuditRepo) ListWebhooksByTeam(_ context.Context, _ uuid.UUID) ([]models.AuditWebhook, error) {
	return []models.AuditWebhook{}, nil
}
func (m *mockAuditRepo) UpdateWebhook(_ context.Context, _ *models.AuditWebhook) error {
	return nil
}
func (m *mockAuditRepo) DeleteWebhook(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *mockAuditRepo) ListActiveWebhooksByTeam(_ context.Context, _ uuid.UUID) ([]models.AuditWebhook, error) {
	return []models.AuditWebhook{}, nil
}

// mockEntitlementChecker is a test double for entitlement.Checker.
type mockEntitlementChecker struct {
	features map[string]bool
}

func (m *mockEntitlementChecker) HasFeature(_ context.Context, feature string) bool {
	return m.features[feature]
}

func (m *mockEntitlementChecker) CheckQuota(_ context.Context, _ string, _ uuid.UUID, _ int64) error {
	return nil
}

func (m *mockEntitlementChecker) RecordUsage(_ context.Context, _ string, _ uuid.UUID, _ int64) {}

func (m *mockEntitlementChecker) OnProjectCreated(_ context.Context, _ entitlement.ProjectCreatedEvent) error {
	return nil
}

func (m *mockEntitlementChecker) Mode() string    { return "self_hosted" }
func (m *mockEntitlementChecker) Edition() string { return "enterprise" }

// Compile-time assertions for interface compliance.
var _ repo.AuditLogStore = (*mockAuditRepo)(nil)
var _ entitlement.Checker = (*mockEntitlementChecker)(nil)

// --- Tests ---

func TestComputeEntryHash_Deterministic(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	h1 := computeEntryHash("prev", "action", "user@test.com", "res-1", ts)
	h2 := computeEntryHash("prev", "action", "user@test.com", "res-1", ts)
	assert.Equal(t, h1, h2, "same inputs must produce same hash")
	assert.Len(t, h1, 64, "SHA-256 hex digest must be 64 chars")
}

func TestComputeEntryHash_DifferentInputs(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	h1 := computeEntryHash("prev", "action.a", "user@test.com", "res-1", ts)
	h2 := computeEntryHash("prev", "action.b", "user@test.com", "res-1", ts)
	assert.NotEqual(t, h1, h2, "different actions must produce different hashes")
}

func TestComputeEntryHash_PrevHashChains(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	h1 := computeEntryHash("", "action", "user@test.com", "res-1", ts)
	h2 := computeEntryHash(h1, "action", "user@test.com", "res-1", ts)
	assert.NotEqual(t, h1, h2, "different prev_hash must chain correctly")
}

func TestAuditService_Log_FeatureGateDisabled(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())
	svc.SetEntitlements(&mockEntitlementChecker{
		features: map[string]bool{"audit_logs": false},
	})

	teamID := uuid.New()
	svc.Log(context.Background(), AuditEntry{
		TeamID:   teamID,
		Action:   models.AuditActionProjectCreated,
		Category: models.AuditCategoryProject,
	})

	assert.Equal(t, 0, mockRepo.createCalls, "should not create entry when feature is disabled")
}

func TestAuditService_Log_FeatureGateEnabled(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())
	svc.SetEntitlements(&mockEntitlementChecker{
		features: map[string]bool{"audit_logs": true},
	})

	teamID := uuid.New()
	svc.Log(context.Background(), AuditEntry{
		TeamID:       teamID,
		Action:       models.AuditActionProjectCreated,
		Category:     models.AuditCategoryProject,
		ResourceType: "project",
		ResourceID:   "proj-1",
		Description:  "Project created",
	})

	assert.Equal(t, 1, mockRepo.createCalls, "should create entry when feature is enabled")
	require.Len(t, mockRepo.created, 1)
	assert.Equal(t, teamID, mockRepo.created[0].TeamID)
	assert.Equal(t, models.AuditActionProjectCreated, mockRepo.created[0].Action)
	assert.Equal(t, models.AuditActorSystem, mockRepo.created[0].ActorType, "no user in ctx => system actor")
}

func TestAuditService_Log_NoEntitlementChecker(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())
	// No entitlement checker set — should still log (community passthrough).

	svc.Log(context.Background(), AuditEntry{
		TeamID:   uuid.New(),
		Action:   models.AuditActionProjectCreated,
		Category: models.AuditCategoryProject,
	})

	assert.Equal(t, 1, mockRepo.createCalls, "should create entry when no entitlement checker is set")
}

func TestAuditService_Log_ExtractsUserFromContext(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	userID := uuid.New()
	user := &models.User{ID: userID, Email: "test@example.com"}
	ctx := apiCtx.WithUser(context.Background(), user)

	svc.Log(ctx, AuditEntry{
		TeamID:   uuid.New(),
		Action:   models.AuditActionProjectCreated,
		Category: models.AuditCategoryProject,
	})

	require.Len(t, mockRepo.created, 1)
	assert.Equal(t, models.AuditActorUser, mockRepo.created[0].ActorType)
	assert.Equal(t, &userID, mockRepo.created[0].ActorID)
	assert.Equal(t, "test@example.com", mockRepo.created[0].ActorEmail)
}

func TestAuditService_LogWithActor_UsesProvidedUser(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	userID := uuid.New()
	user := &models.User{ID: userID, Email: "actor@example.com"}

	// Context has NO user, but we pass one explicitly
	svc.LogWithActor(context.Background(), AuditEntry{
		TeamID:   uuid.New(),
		Action:   models.AuditActionAuthLogin,
		Category: models.AuditCategoryAuth,
	}, user)

	require.Len(t, mockRepo.created, 1)
	assert.Equal(t, models.AuditActorUser, mockRepo.created[0].ActorType)
	assert.Equal(t, "actor@example.com", mockRepo.created[0].ActorEmail)
}

func TestAuditService_Log_HashChain(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	teamID := uuid.New()

	// First entry: prev_hash should be empty
	svc.Log(context.Background(), AuditEntry{
		TeamID:   teamID,
		Action:   "action.first",
		Category: models.AuditCategoryProject,
	})
	require.Len(t, mockRepo.created, 1)
	assert.Empty(t, mockRepo.created[0].PrevHash, "first entry should have empty prev_hash")
	assert.NotEmpty(t, mockRepo.created[0].EntryHash, "entry_hash must be set")

	firstHash := mockRepo.created[0].EntryHash

	// Second entry: prev_hash should be the first entry's hash
	svc.Log(context.Background(), AuditEntry{
		TeamID:   teamID,
		Action:   "action.second",
		Category: models.AuditCategoryProject,
	})
	require.Len(t, mockRepo.created, 2)
	assert.Equal(t, firstHash, mockRepo.created[1].PrevHash, "second entry should chain from first")
	assert.NotEqual(t, firstHash, mockRepo.created[1].EntryHash, "each entry should have a unique hash")
}

func TestAuditService_Log_ExtractsIPAndUserAgent(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	ctx := context.Background()
	ctx = apiCtx.WithIPAddress(ctx, "192.168.1.1")
	ctx = apiCtx.WithUserAgent(ctx, "TestAgent/1.0")
	ctx = apiCtx.WithRequestID(ctx, "req-123")

	svc.Log(ctx, AuditEntry{
		TeamID:   uuid.New(),
		Action:   models.AuditActionProjectCreated,
		Category: models.AuditCategoryProject,
	})

	require.Len(t, mockRepo.created, 1)
	assert.Equal(t, "192.168.1.1", mockRepo.created[0].IPAddress)
	assert.Equal(t, "TestAgent/1.0", mockRepo.created[0].UserAgent)
	assert.Equal(t, "req-123", mockRepo.created[0].RequestID)
}

func TestAuditService_Log_MetadataSerialization(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	svc.Log(context.Background(), AuditEntry{
		TeamID:   uuid.New(),
		Action:   models.AuditActionMemberAdded,
		Category: models.AuditCategoryMember,
		Metadata: map[string]interface{}{
			"role":  "admin",
			"count": float64(3),
		},
	})

	require.Len(t, mockRepo.created, 1)

	var meta map[string]interface{}
	err := json.Unmarshal(mockRepo.created[0].Metadata, &meta)
	require.NoError(t, err)
	assert.Equal(t, "admin", meta["role"])
	assert.Equal(t, float64(3), meta["count"])
}

func TestAuditService_Log_NilMetadata(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	svc.Log(context.Background(), AuditEntry{
		TeamID:   uuid.New(),
		Action:   models.AuditActionProjectCreated,
		Category: models.AuditCategoryProject,
		Metadata: nil,
	})

	require.Len(t, mockRepo.created, 1)
	assert.Equal(t, []byte("{}"), mockRepo.created[0].Metadata)
}

func TestAuditService_ExportCSV(t *testing.T) {
	teamID := uuid.New()
	logID := uuid.New()
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	svc := NewAuditService(nil, zerolog.Nop())

	var buf bytes.Buffer
	err := svc.exportCSV(&buf, []models.AuditLog{
		{
			ID:           logID,
			TeamID:       teamID,
			ActorType:    models.AuditActorUser,
			ActorEmail:   "user@test.com",
			Action:       models.AuditActionProjectCreated,
			Category:     models.AuditCategoryProject,
			ResourceType: "project",
			ResourceID:   "proj-1",
			Description:  "Project created",
			IPAddress:    "10.0.0.1",
			CreatedAt:    ts,
		},
	})

	require.NoError(t, err)
	csv := buf.String()
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	require.Len(t, lines, 2, "header + 1 data row")
	assert.Contains(t, lines[0], "id,timestamp,actor_type")
	assert.Contains(t, lines[1], logID.String())
	assert.Contains(t, lines[1], "user@test.com")
	assert.Contains(t, lines[1], "project.created")
}

func TestAuditService_ExportJSON(t *testing.T) {
	teamID := uuid.New()
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	svc := NewAuditService(nil, zerolog.Nop())

	var buf bytes.Buffer
	err := svc.exportJSON(&buf, []models.AuditLog{
		{
			ID:         uuid.New(),
			TeamID:     teamID,
			ActorType:  models.AuditActorUser,
			ActorEmail: "user@test.com",
			Action:     models.AuditActionProjectCreated,
			Category:   models.AuditCategoryProject,
			Metadata:   []byte("{}"),
			CreatedAt:  ts,
		},
	})

	require.NoError(t, err)

	var result []map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "user@test.com", result[0]["actor_email"])
	assert.Equal(t, "project.created", result[0]["action"])
}

func TestAuditService_VerifyChain_EmptyLogs(t *testing.T) {
	mockRepo := &mockAuditRepo{
		chainLogs: []models.AuditLog{},
	}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	result, err := svc.VerifyChain(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, 0, result.TotalChecked)
	assert.Equal(t, "no entries to verify", result.Message)
}

func TestAuditService_VerifyChain_ValidChain(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	teamID := uuid.New()

	h1 := computeEntryHash("", "action.a", "user@test.com", "res-1", ts)
	h2 := computeEntryHash(h1, "action.b", "user@test.com", "res-2", ts.Add(time.Minute))

	mockRepo := &mockAuditRepo{
		chainLogs: []models.AuditLog{
			{
				SequenceNum: 1,
				TeamID:      teamID,
				Action:      "action.a",
				ActorEmail:  "user@test.com",
				ResourceID:  "res-1",
				PrevHash:    "",
				EntryHash:   h1,
				CreatedAt:   ts,
			},
			{
				SequenceNum: 2,
				TeamID:      teamID,
				Action:      "action.b",
				ActorEmail:  "user@test.com",
				ResourceID:  "res-2",
				PrevHash:    h1,
				EntryHash:   h2,
				CreatedAt:   ts.Add(time.Minute),
			},
		},
	}

	svc := NewAuditService(mockRepo, zerolog.Nop())
	result, err := svc.VerifyChain(context.Background(), teamID)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, 2, result.TotalChecked)
	assert.Contains(t, result.Message, "2 entries verified successfully")
}

func TestAuditService_VerifyChain_TamperedEntry(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	teamID := uuid.New()

	h1 := computeEntryHash("", "action.a", "user@test.com", "res-1", ts)

	mockRepo := &mockAuditRepo{
		chainLogs: []models.AuditLog{
			{
				SequenceNum: 1,
				TeamID:      teamID,
				Action:      "action.a",
				ActorEmail:  "user@test.com",
				ResourceID:  "res-1",
				PrevHash:    "",
				EntryHash:   h1,
				CreatedAt:   ts,
			},
			{
				SequenceNum: 2,
				TeamID:      teamID,
				Action:      "action.b",
				ActorEmail:  "user@test.com",
				ResourceID:  "res-2",
				PrevHash:    h1,
				EntryHash:   "tampered_hash_value",
				CreatedAt:   ts.Add(time.Minute),
			},
		},
	}

	svc := NewAuditService(mockRepo, zerolog.Nop())
	result, err := svc.VerifyChain(context.Background(), teamID)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotNil(t, result.FirstBreakAt)
	assert.Equal(t, int64(2), *result.FirstBreakAt)
	assert.Contains(t, result.Message, "hash mismatch at sequence 2")
}

func TestAuditService_VerifyChain_SkipsEpochMarkers(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	teamID := uuid.New()

	h1 := computeEntryHash("", models.AuditActionRetentionEpoch, "", "", ts)

	mockRepo := &mockAuditRepo{
		chainLogs: []models.AuditLog{
			{
				SequenceNum: 1,
				TeamID:      teamID,
				Action:      models.AuditActionRetentionEpoch,
				PrevHash:    "",
				EntryHash:   h1,
				CreatedAt:   ts,
			},
		},
	}

	svc := NewAuditService(mockRepo, zerolog.Nop())
	result, err := svc.VerifyChain(context.Background(), teamID)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, 1, result.TotalChecked) // epoch is counted but not verified
}

func TestAuditService_GetRetention_Default(t *testing.T) {
	mockRepo := &mockAuditRepo{} // retention is nil => ErrNotFound
	svc := NewAuditService(mockRepo, zerolog.Nop())

	teamID := uuid.New()
	policy, err := svc.GetRetention(context.Background(), teamID)
	require.NoError(t, err)
	assert.Equal(t, 90, policy.RetentionDays)
	assert.Equal(t, teamID, policy.TeamID)
}

func TestAuditService_GetRetention_Custom(t *testing.T) {
	teamID := uuid.New()
	mockRepo := &mockAuditRepo{
		retention: &models.AuditLogRetentionPolicy{
			TeamID:        teamID,
			RetentionDays: 365,
		},
	}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	policy, err := svc.GetRetention(context.Background(), teamID)
	require.NoError(t, err)
	assert.Equal(t, 365, policy.RetentionDays)
}

func TestAuditService_UpdateRetention(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	teamID := uuid.New()
	userID := uuid.New()
	err := svc.UpdateRetention(context.Background(), teamID, 180, &userID)
	require.NoError(t, err)

	require.NotNil(t, mockRepo.retention)
	assert.Equal(t, teamID, mockRepo.retention.TeamID)
	assert.Equal(t, 180, mockRepo.retention.RetentionDays)
	assert.Equal(t, &userID, mockRepo.retention.UpdatedBy)
}

func TestAuditService_RunRetentionGC_DeletesAndCreatesEpoch(t *testing.T) {
	teamID := uuid.New()
	mockRepo := &mockAuditRepo{
		policies: []models.AuditLogRetentionPolicy{
			{TeamID: teamID, RetentionDays: 30},
		},
		deleted: 5, // simulate 5 rows deleted
	}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	err := svc.RunRetentionGC(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, mockRepo.deleteCalls, "should have called DeleteBefore once")
	// Should have created an epoch marker entry
	require.Len(t, mockRepo.created, 1)
	assert.Equal(t, models.AuditActionRetentionEpoch, mockRepo.created[0].Action)
	assert.Equal(t, models.AuditCategorySystem, mockRepo.created[0].Category)
	assert.Equal(t, teamID, mockRepo.created[0].TeamID)
}

func TestAuditService_RunRetentionGC_NoDelete_NoEpoch(t *testing.T) {
	teamID := uuid.New()
	mockRepo := &mockAuditRepo{
		policies: []models.AuditLogRetentionPolicy{
			{TeamID: teamID, RetentionDays: 30},
		},
		deleted: 0, // nothing to delete
	}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	err := svc.RunRetentionGC(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, mockRepo.deleteCalls)
	assert.Empty(t, mockRepo.created, "no epoch marker when nothing was deleted")
}

func TestAuditService_RunRetentionGC_NoPolicies(t *testing.T) {
	mockRepo := &mockAuditRepo{
		policies: []models.AuditLogRetentionPolicy{},
	}
	svc := NewAuditService(mockRepo, zerolog.Nop())

	err := svc.RunRetentionGC(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, mockRepo.deleteCalls, "should not delete when no policies exist")
}

func TestAuditService_VerifyChain_BrokenLinkage(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	teamID := uuid.New()

	h1 := computeEntryHash("", "action.a", "user@test.com", "res-1", ts)
	// Entry 2 has a WRONG prev_hash (should be h1, but is "wrong_prev")
	h2 := computeEntryHash("wrong_prev", "action.b", "user@test.com", "res-2", ts.Add(time.Minute))

	mockRepo := &mockAuditRepo{
		chainLogs: []models.AuditLog{
			{
				SequenceNum: 1,
				TeamID:      teamID,
				Action:      "action.a",
				ActorEmail:  "user@test.com",
				ResourceID:  "res-1",
				PrevHash:    "",
				EntryHash:   h1,
				CreatedAt:   ts,
			},
			{
				SequenceNum: 2,
				TeamID:      teamID,
				Action:      "action.b",
				ActorEmail:  "user@test.com",
				ResourceID:  "res-2",
				PrevHash:    "wrong_prev", // should be h1
				EntryHash:   h2,           // hash is valid for the wrong prev
				CreatedAt:   ts.Add(time.Minute),
			},
		},
	}

	svc := NewAuditService(mockRepo, zerolog.Nop())
	result, err := svc.VerifyChain(context.Background(), teamID)
	require.NoError(t, err)
	assert.False(t, result.Valid, "should detect broken chain linkage")
	assert.NotNil(t, result.FirstBreakAt)
	assert.Equal(t, int64(2), *result.FirstBreakAt)
	assert.Contains(t, result.Message, "chain linkage broken")
}

func TestChainVerifyResult_Struct(t *testing.T) {
	result := &ChainVerifyResult{
		Valid:        true,
		TotalChecked: 42,
		Message:      "all good",
	}
	assert.True(t, result.Valid)
	assert.Equal(t, 42, result.TotalChecked)
	assert.Nil(t, result.FirstBreakAt)
}
