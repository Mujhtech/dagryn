package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/service"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations ---

// mockAuditLogStore is a test double for repo.AuditLogStore.
type mockAuditLogStore struct {
	created    []*models.AuditLog
	lastHash   string
	lastSeq    int64
	chainLogs  []models.AuditLog
	policies   []models.AuditLogRetentionPolicy
	retention  *models.AuditLogRetentionPolicy
	deleted    int64
	listResult *repo.AuditLogListResult
	webhooks   []models.AuditWebhook
	webhook    *models.AuditWebhook

	createCalls int
}

func (m *mockAuditLogStore) Create(_ context.Context, log *models.AuditLog) error {
	m.createCalls++
	log.ID = uuid.New()
	log.SequenceNum = int64(m.createCalls)
	log.CreatedAt = time.Now().UTC()
	m.created = append(m.created, log)
	m.lastHash = log.EntryHash
	m.lastSeq = log.SequenceNum
	return nil
}

func (m *mockAuditLogStore) GetByID(_ context.Context, id uuid.UUID) (*models.AuditLog, error) {
	for _, l := range m.created {
		if l.ID == id {
			return l, nil
		}
	}
	return nil, repo.ErrNotFound
}

func (m *mockAuditLogStore) List(_ context.Context, _ repo.AuditLogFilter) (*repo.AuditLogListResult, error) {
	if m.listResult != nil {
		return m.listResult, nil
	}
	return &repo.AuditLogListResult{Data: []models.AuditLog{}}, nil
}

func (m *mockAuditLogStore) DeleteBefore(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	return m.deleted, nil
}

func (m *mockAuditLogStore) GetLastHash(_ context.Context, _ uuid.UUID) (string, int64, error) {
	return m.lastHash, m.lastSeq, nil
}

func (m *mockAuditLogStore) GetRetentionPolicy(_ context.Context, _ uuid.UUID) (*models.AuditLogRetentionPolicy, error) {
	if m.retention != nil {
		return m.retention, nil
	}
	return nil, repo.ErrNotFound
}

func (m *mockAuditLogStore) UpsertRetentionPolicy(_ context.Context, p *models.AuditLogRetentionPolicy) error {
	m.retention = p
	return nil
}

func (m *mockAuditLogStore) ListAllRetentionPolicies(_ context.Context) ([]models.AuditLogRetentionPolicy, error) {
	return m.policies, nil
}

func (m *mockAuditLogStore) CountByTeam(_ context.Context, _ uuid.UUID) (int64, error) {
	return int64(len(m.created)), nil
}

func (m *mockAuditLogStore) ListChain(_ context.Context, _ uuid.UUID, afterSeq int64, _ int) ([]models.AuditLog, error) {
	var result []models.AuditLog
	for _, l := range m.chainLogs {
		if l.SequenceNum > afterSeq {
			result = append(result, l)
		}
	}
	return result, nil
}

func (m *mockAuditLogStore) CreateWebhook(_ context.Context, w *models.AuditWebhook) error {
	w.ID = uuid.New()
	w.CreatedAt = time.Now().UTC()
	w.UpdatedAt = time.Now().UTC()
	m.webhooks = append(m.webhooks, *w)
	return nil
}

func (m *mockAuditLogStore) GetWebhookByID(_ context.Context, id uuid.UUID) (*models.AuditWebhook, error) {
	if m.webhook != nil && m.webhook.ID == id {
		return m.webhook, nil
	}
	for i := range m.webhooks {
		if m.webhooks[i].ID == id {
			return &m.webhooks[i], nil
		}
	}
	return nil, repo.ErrNotFound
}

func (m *mockAuditLogStore) ListWebhooksByTeam(_ context.Context, _ uuid.UUID) ([]models.AuditWebhook, error) {
	if m.webhooks == nil {
		return []models.AuditWebhook{}, nil
	}
	return m.webhooks, nil
}

func (m *mockAuditLogStore) UpdateWebhook(_ context.Context, _ *models.AuditWebhook) error {
	return nil
}

func (m *mockAuditLogStore) DeleteWebhook(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockAuditLogStore) ListActiveWebhooksByTeam(_ context.Context, _ uuid.UUID) ([]models.AuditWebhook, error) {
	return []models.AuditWebhook{}, nil
}

// mockTeamStore is a minimal test double for repo.TeamStore.
type mockTeamStore struct {
	members map[string]*models.TeamMember // key: "teamID:userID"
}

func newMockTeamStore() *mockTeamStore {
	return &mockTeamStore{
		members: make(map[string]*models.TeamMember),
	}
}

func (m *mockTeamStore) addMember(teamID, userID uuid.UUID, role models.Role) {
	key := teamID.String() + ":" + userID.String()
	m.members[key] = &models.TeamMember{
		ID:       uuid.New(),
		TeamID:   teamID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now().UTC(),
	}
}

func (m *mockTeamStore) Create(_ context.Context, _ *models.Team) error { return nil }
func (m *mockTeamStore) GetByID(_ context.Context, _ uuid.UUID) (*models.Team, error) {
	return nil, repo.ErrNotFound
}
func (m *mockTeamStore) GetBySlug(_ context.Context, _ string) (*models.Team, error) {
	return nil, repo.ErrNotFound
}
func (m *mockTeamStore) Update(_ context.Context, _ *models.Team) error { return nil }
func (m *mockTeamStore) Delete(_ context.Context, _ uuid.UUID) error    { return nil }
func (m *mockTeamStore) ListByUser(_ context.Context, _ uuid.UUID) ([]models.TeamWithMember, error) {
	return nil, nil
}
func (m *mockTeamStore) AddMember(_ context.Context, _, _ uuid.UUID, _ models.Role, _ *uuid.UUID) error {
	return nil
}
func (m *mockTeamStore) UpdateMemberRole(_ context.Context, _, _ uuid.UUID, _ models.Role) error {
	return nil
}
func (m *mockTeamStore) RemoveMember(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockTeamStore) GetMember(_ context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error) {
	key := teamID.String() + ":" + userID.String()
	member, ok := m.members[key]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return member, nil
}
func (m *mockTeamStore) ListMembers(_ context.Context, _ uuid.UUID) ([]models.TeamMemberWithUser, error) {
	return nil, nil
}
func (m *mockTeamStore) SlugExists(_ context.Context, _ string) (bool, error) {
	return false, nil
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

// mockEncrypter is a test double for encrypt.Encrypt.
type mockEncrypter struct{}

func (m *mockEncrypter) Encrypt(plaintext []byte) (string, error)  { return string(plaintext), nil }
func (m *mockEncrypter) Decrypt(ciphertext string) (string, error) { return ciphertext, nil }

// Compile-time interface assertions.
var _ repo.AuditLogStore = (*mockAuditLogStore)(nil)
var _ repo.TeamStore = (*mockTeamStore)(nil)
var _ entitlement.Checker = (*mockEntitlementChecker)(nil)

// --- Test helpers ---

// newTestHandler creates a Handler with the provided test doubles.
func newTestHandler(teamStore repo.TeamStore, auditLogStore repo.AuditLogStore) *Handler {
	h := &Handler{
		store: store.Store{
			Teams:     teamStore,
			AuditLogs: auditLogStore,
		},
	}
	return h
}

// newTestAuditService creates an AuditService backed by the given mock store.
func newTestAuditService(auditLogStore repo.AuditLogStore) *service.AuditService {
	svc := service.NewAuditService(auditLogStore, zerolog.Nop())
	return svc
}

// parseResponseBody decodes a JSON response body into a map.
func parseResponseBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err, "response body should be valid JSON")
	return body
}

// makeRequest builds an http.Request for the given method, path, and optional body.
func makeRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		err := json.NewEncoder(&buf).Encode(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// --- ListTeamAuditLogs tests ---

func TestListTeamAuditLogs_Success(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{
		listResult: &repo.AuditLogListResult{
			Data: []models.AuditLog{
				{
					ID:         uuid.New(),
					TeamID:     teamID,
					ActorType:  models.AuditActorUser,
					ActorEmail: "admin@example.com",
					Action:     models.AuditActionProjectCreated,
					Category:   models.AuditCategoryProject,
					CreatedAt:  time.Now().UTC(),
				},
			},
			HasNext: false,
		},
	}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamID}/audit-logs", h.ListTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Success", body["message"])
	assert.NotNil(t, body["data"])
}

func TestListTeamAuditLogs_Unauthorized(t *testing.T) {
	teamID := uuid.New()
	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamID}/audit-logs", h.ListTeamAuditLogs)

	// No user in context => unauthorized.
	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Unauthorized", body["message"])
}

func TestListTeamAuditLogs_Forbidden_NotMember(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "outsider@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	// User is NOT a member of the team.

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamID}/audit-logs", h.ListTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Forbidden", body["message"])
}

func TestListTeamAuditLogs_Forbidden_NoPermission(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "viewer@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	// RoleViewer does NOT have PermissionAuditLogsView.
	teamStore.addMember(teamID, userID, models.RoleViewer)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamID}/audit-logs", h.ListTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Contains(t, body["error"], "permission")
}

func TestListTeamAuditLogs_ServiceUnavailable(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	// Do NOT set auditService => h.auditService is nil.

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamID}/audit-logs", h.ListTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Service Unavailable", body["message"])
}

// --- ExportTeamAuditLogs tests ---

func TestExportTeamAuditLogs_Success_CSV(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	logID := uuid.New()
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	auditStore := &mockAuditLogStore{
		listResult: &repo.AuditLogListResult{
			Data: []models.AuditLog{
				{
					ID:           logID,
					TeamID:       teamID,
					ActorType:    models.AuditActorUser,
					ActorEmail:   "admin@example.com",
					Action:       models.AuditActionProjectCreated,
					Category:     models.AuditCategoryProject,
					ResourceType: "project",
					ResourceID:   "proj-1",
					Description:  "Project created",
					IPAddress:    "10.0.0.1",
					Metadata:     []byte("{}"),
					CreatedAt:    ts,
				},
			},
			HasNext: false,
		},
	}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/export", h.ExportTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/export?format=csv", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/csv", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "audit-logs.csv")

	csvBody := rec.Body.String()
	assert.Contains(t, csvBody, "id,timestamp,actor_type")
	assert.Contains(t, csvBody, logID.String())
	assert.Contains(t, csvBody, "admin@example.com")
	assert.Contains(t, csvBody, "project.created")
}

// --- GetAuditRetentionPolicy tests ---

func TestGetAuditRetentionPolicy_Success(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{
		retention: &models.AuditLogRetentionPolicy{
			TeamID:        teamID,
			RetentionDays: 365,
			UpdatedAt:     time.Now().UTC(),
		},
	}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/retention", h.GetAuditRetentionPolicy)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/retention", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Success", body["message"])

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "data should be a map")
	assert.Equal(t, float64(365), data["retention_days"])
}

func TestGetAuditRetentionPolicy_DefaultWhenNoneSet(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	// No retention policy set => service returns default 90 days.
	auditStore := &mockAuditLogStore{}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/retention", h.GetAuditRetentionPolicy)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/retention", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := parseResponseBody(t, rec)
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(90), data["retention_days"])
}

// --- VerifyAuditChain tests ---

func TestVerifyAuditChain_Success(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	// Empty chain => valid, 0 entries.
	auditStore := &mockAuditLogStore{
		chainLogs: []models.AuditLog{},
	}

	teamStore := newMockTeamStore()
	// VerifyAuditChain requires PermissionAuditLogsManage => need admin or owner.
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/verify", h.VerifyAuditChain)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/verify", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Chain verification complete", body["message"])

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, data["valid"])
	assert.Equal(t, float64(0), data["total_checked"])
}

func TestVerifyAuditChain_Forbidden_MemberRole(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "member@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	// RoleMember does NOT have PermissionAuditLogsManage.
	teamStore.addMember(teamID, userID, models.RoleMember)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/verify", h.VerifyAuditChain)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/verify", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- ListAuditWebhooks tests ---

func TestListAuditWebhooks_Success(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	webhookID := uuid.New()
	auditStore := &mockAuditLogStore{
		webhooks: []models.AuditWebhook{
			{
				ID:          webhookID,
				TeamID:      teamID,
				URL:         "https://siem.example.com/audit",
				Description: "Forward to Splunk",
				EventFilter: []string{},
				IsActive:    true,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			},
		},
	}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/webhooks", h.ListAuditWebhooks)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Success", body["message"])

	data, ok := body["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	require.Len(t, data, 1)

	first, ok := data[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, webhookID.String(), first["id"])
	assert.Equal(t, "https://siem.example.com/audit", first["url"])
}

func TestListAuditWebhooks_Forbidden_MemberRole(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "member@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	// RoleMember does NOT have PermissionAuditLogsManage.
	teamStore.addMember(teamID, userID, models.RoleMember)

	h := newTestHandler(teamStore, auditStore)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/webhooks", h.ListAuditWebhooks)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Contains(t, body["error"], "permission")
}

func TestListAuditWebhooks_Unauthorized(t *testing.T) {
	teamID := uuid.New()
	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()

	h := newTestHandler(teamStore, auditStore)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/webhooks", h.ListAuditWebhooks)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- CreateAuditWebhook tests ---

func TestCreateAuditWebhook_Success(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	h.SetEncrypter(&mockEncrypter{})

	router := chi.NewRouter()
	router.Post("/api/v1/teams/{teamId}/audit-logs/webhooks", h.CreateAuditWebhook)

	reqBody := CreateAuditWebhookRequest{
		URL:         "https://siem.example.com/audit",
		Description: "Forward to SIEM",
	}

	req := makeRequest(t, http.MethodPost, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", reqBody)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Equal(t, "Webhook created", body["message"])

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https://siem.example.com/audit", data["url"])
	assert.NotEmpty(t, data["secret"], "secret should be returned on creation")
	assert.Equal(t, true, data["is_active"])
}

func TestCreateAuditWebhook_BadRequest_MissingURL(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	h.SetEncrypter(&mockEncrypter{})

	router := chi.NewRouter()
	router.Post("/api/v1/teams/{teamId}/audit-logs/webhooks", h.CreateAuditWebhook)

	// URL field is empty.
	reqBody := CreateAuditWebhookRequest{
		URL:         "",
		Description: "Missing URL",
	}

	req := makeRequest(t, http.MethodPost, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", reqBody)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := parseResponseBody(t, rec)
	assert.Contains(t, body["error"], "url is required")
}

func TestCreateAuditWebhook_Forbidden_MemberRole(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "member@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleMember)

	h := newTestHandler(teamStore, auditStore)
	h.SetEncrypter(&mockEncrypter{})

	router := chi.NewRouter()
	router.Post("/api/v1/teams/{teamId}/audit-logs/webhooks", h.CreateAuditWebhook)

	reqBody := CreateAuditWebhookRequest{
		URL: "https://siem.example.com/audit",
	}

	req := makeRequest(t, http.MethodPost, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", reqBody)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCreateAuditWebhook_Unauthorized(t *testing.T) {
	teamID := uuid.New()
	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()

	h := newTestHandler(teamStore, auditStore)
	h.SetEncrypter(&mockEncrypter{})

	router := chi.NewRouter()
	router.Post("/api/v1/teams/{teamId}/audit-logs/webhooks", h.CreateAuditWebhook)

	reqBody := CreateAuditWebhookRequest{
		URL: "https://siem.example.com/audit",
	}

	req := makeRequest(t, http.MethodPost, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", reqBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- Additional edge case tests ---

func TestListTeamAuditLogs_MemberWithPermission(t *testing.T) {
	// RoleMember has PermissionAuditLogsView in the current permission model.
	// Verify that a member CAN see audit logs (they should NOT, based on role permissions).
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "member@example.com"}

	auditStore := &mockAuditLogStore{
		listResult: &repo.AuditLogListResult{
			Data:    []models.AuditLog{},
			HasNext: false,
		},
	}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleMember)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamID}/audit-logs", h.ListTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// RoleMember does NOT have PermissionAuditLogsView in the role permissions map.
	// This should be Forbidden.
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestExportTeamAuditLogs_Unauthorized(t *testing.T) {
	teamID := uuid.New()
	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/export", h.ExportTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/export", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestExportTeamAuditLogs_DefaultFormatJSON(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{
		listResult: &repo.AuditLogListResult{
			Data:    []models.AuditLog{},
			HasNext: false,
		},
	}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/export", h.ExportTeamAuditLogs)

	// No format query param => defaults to JSON.
	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/export", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "audit-logs.json")
}

func TestGetAuditRetentionPolicy_Unauthorized(t *testing.T) {
	teamID := uuid.New()
	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/retention", h.GetAuditRetentionPolicy)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/retention", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestVerifyAuditChain_Unauthorized(t *testing.T) {
	teamID := uuid.New()
	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/verify", h.VerifyAuditChain)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/verify", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestVerifyAuditChain_ServiceUnavailable(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)
	// Do NOT set auditService => nil.

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/verify", h.VerifyAuditChain)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/verify", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestCreateAuditWebhook_WithEventFilter(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{}
	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleOwner)

	h := newTestHandler(teamStore, auditStore)
	h.SetEncrypter(&mockEncrypter{})

	router := chi.NewRouter()
	router.Post("/api/v1/teams/{teamId}/audit-logs/webhooks", h.CreateAuditWebhook)

	reqBody := CreateAuditWebhookRequest{
		URL:         "https://siem.example.com/events",
		Description: "Only project events",
		EventFilter: []string{"project.created", "project.deleted"},
	}

	req := makeRequest(t, http.MethodPost, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", reqBody)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	body := parseResponseBody(t, rec)
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https://siem.example.com/events", data["url"])
	assert.Equal(t, "Only project events", data["description"])
}

func TestListAuditWebhooks_EmptyList(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "admin@example.com"}

	auditStore := &mockAuditLogStore{
		webhooks: nil, // will return empty list
	}
	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleAdmin)

	h := newTestHandler(teamStore, auditStore)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamId}/audit-logs/webhooks", h.ListAuditWebhooks)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs/webhooks", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := parseResponseBody(t, rec)
	data, ok := body["data"].([]interface{})
	require.True(t, ok)
	assert.Len(t, data, 0)
}

func TestListTeamAuditLogs_OwnerRole(t *testing.T) {
	teamID := uuid.New()
	userID := uuid.New()
	user := &models.User{ID: userID, Email: "owner@example.com"}

	auditStore := &mockAuditLogStore{
		listResult: &repo.AuditLogListResult{
			Data:    []models.AuditLog{},
			HasNext: false,
		},
	}

	teamStore := newMockTeamStore()
	teamStore.addMember(teamID, userID, models.RoleOwner)

	h := newTestHandler(teamStore, auditStore)
	auditSvc := newTestAuditService(auditStore)
	h.SetAuditService(auditSvc)

	router := chi.NewRouter()
	router.Get("/api/v1/teams/{teamID}/audit-logs", h.ListTeamAuditLogs)

	req := makeRequest(t, http.MethodGet, "/api/v1/teams/"+teamID.String()+"/audit-logs", nil)
	ctx := apiCtx.WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
