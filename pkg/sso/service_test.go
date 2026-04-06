package sso

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations ---

type mockSSORepo struct {
	connections map[uuid.UUID]*models.SSOConnection
	byTeamID    map[uuid.UUID]*models.SSOConnection
	states      map[string]*models.SSOState
}

func newMockSSORepo() *mockSSORepo {
	return &mockSSORepo{
		connections: make(map[uuid.UUID]*models.SSOConnection),
		byTeamID:    make(map[uuid.UUID]*models.SSOConnection),
		states:      make(map[string]*models.SSOState),
	}
}

func (m *mockSSORepo) CreateConnection(ctx context.Context, conn *models.SSOConnection) error {
	conn.ID = uuid.New()
	conn.CreatedAt = time.Now()
	conn.UpdatedAt = time.Now()
	m.connections[conn.ID] = conn
	m.byTeamID[conn.TeamID] = conn
	return nil
}

func (m *mockSSORepo) GetConnectionByID(ctx context.Context, id uuid.UUID) (*models.SSOConnection, error) {
	conn, ok := m.connections[id]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return conn, nil
}

func (m *mockSSORepo) GetConnectionByTeamID(ctx context.Context, teamID uuid.UUID) (*models.SSOConnection, error) {
	conn, ok := m.byTeamID[teamID]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return conn, nil
}

func (m *mockSSORepo) GetConnectionBySPEntityID(ctx context.Context, spEntityID string) (*models.SSOConnection, error) {
	for _, conn := range m.connections {
		if conn.SPEntityID == spEntityID {
			return conn, nil
		}
	}
	return nil, repo.ErrNotFound
}

func (m *mockSSORepo) UpdateConnection(ctx context.Context, conn *models.SSOConnection) error {
	m.connections[conn.ID] = conn
	m.byTeamID[conn.TeamID] = conn
	return nil
}

func (m *mockSSORepo) DeleteConnection(ctx context.Context, id uuid.UUID) error {
	conn, ok := m.connections[id]
	if !ok {
		return repo.ErrNotFound
	}
	delete(m.byTeamID, conn.TeamID)
	delete(m.connections, id)
	return nil
}

func (m *mockSSORepo) CreateState(ctx context.Context, state *models.SSOState) error {
	state.ID = uuid.New()
	state.CreatedAt = time.Now()
	m.states[state.RelayState] = state
	return nil
}

func (m *mockSSORepo) GetStateByRelayState(ctx context.Context, relayState string) (*models.SSOState, error) {
	state, ok := m.states[relayState]
	if !ok {
		return nil, repo.ErrNotFound
	}
	if time.Now().After(state.ExpiresAt) {
		return nil, repo.ErrNotFound
	}
	return state, nil
}

func (m *mockSSORepo) DeleteState(ctx context.Context, id uuid.UUID) error {
	for key, state := range m.states {
		if state.ID == id {
			delete(m.states, key)
			return nil
		}
	}
	return nil
}

func (m *mockSSORepo) CleanupExpiredStates(ctx context.Context) (int64, error) {
	return 0, nil
}

type mockTeamRepo struct {
	teams   map[uuid.UUID]*models.Team
	bySlugs map[string]*models.Team
	members map[uuid.UUID]map[uuid.UUID]*models.TeamMember
}

func newMockTeamRepo() *mockTeamRepo {
	return &mockTeamRepo{
		teams:   make(map[uuid.UUID]*models.Team),
		bySlugs: make(map[string]*models.Team),
		members: make(map[uuid.UUID]map[uuid.UUID]*models.TeamMember),
	}
}

func (m *mockTeamRepo) Create(ctx context.Context, team *models.Team) error {
	team.ID = uuid.New()
	m.teams[team.ID] = team
	m.bySlugs[team.Slug] = team
	m.members[team.ID] = make(map[uuid.UUID]*models.TeamMember)
	return nil
}

func (m *mockTeamRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	team, ok := m.teams[id]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return team, nil
}

func (m *mockTeamRepo) GetBySlug(ctx context.Context, slug string) (*models.Team, error) {
	team, ok := m.bySlugs[slug]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return team, nil
}

func (m *mockTeamRepo) Update(ctx context.Context, team *models.Team) error       { return nil }
func (m *mockTeamRepo) Delete(ctx context.Context, id uuid.UUID) error            { return nil }
func (m *mockTeamRepo) SlugExists(ctx context.Context, slug string) (bool, error) { return false, nil }

func (m *mockTeamRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.TeamWithMember, error) {
	return nil, nil
}

func (m *mockTeamRepo) AddMember(ctx context.Context, teamID, userID uuid.UUID, role models.Role, invitedBy *uuid.UUID) error {
	if m.members[teamID] == nil {
		m.members[teamID] = make(map[uuid.UUID]*models.TeamMember)
	}
	m.members[teamID][userID] = &models.TeamMember{TeamID: teamID, UserID: userID, Role: role}
	return nil
}

func (m *mockTeamRepo) UpdateMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.Role) error {
	return nil
}

func (m *mockTeamRepo) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	if m.members[teamID] != nil {
		delete(m.members[teamID], userID)
	}
	return nil
}

func (m *mockTeamRepo) GetMember(ctx context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error) {
	if members, ok := m.members[teamID]; ok {
		if member, ok := members[userID]; ok {
			return member, nil
		}
	}
	return nil, repo.ErrNotFound
}

func (m *mockTeamRepo) ListMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMemberWithUser, error) {
	return nil, nil
}

func (m *mockTeamRepo) CountMembersByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return 0, nil
}

type mockUserRepo struct {
	users       map[uuid.UUID]*models.User
	byEmail     map[string]*models.User
	bySCIMID    map[string]*models.User
	createCalls int
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:    make(map[uuid.UUID]*models.User),
		byEmail:  make(map[string]*models.User),
		bySCIMID: make(map[string]*models.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	m.byEmail[user.Email] = user
	m.createCalls++
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user, ok := m.byEmail[email]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepo) GetByProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	return nil, repo.ErrNotFound
}

func (m *mockUserRepo) Update(ctx context.Context, user *models.User) error {
	m.users[user.ID] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *mockUserRepo) UpsertByProvider(ctx context.Context, user *models.User) error { return nil }
func (m *mockUserRepo) Delete(ctx context.Context, id uuid.UUID) error                { return nil }

func (m *mockUserRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	if user, ok := m.users[id]; ok {
		now := time.Now()
		user.DeactivatedAt = &now
	}
	return nil
}

func (m *mockUserRepo) Reactivate(ctx context.Context, id uuid.UUID) error {
	if user, ok := m.users[id]; ok {
		user.DeactivatedAt = nil
	}
	return nil
}

func (m *mockUserRepo) GetBySCIMExternalID(ctx context.Context, externalID string) (*models.User, error) {
	user, ok := m.bySCIMID[externalID]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepo) ListByTeamForSCIM(ctx context.Context, teamID uuid.UUID, startIndex, count int) ([]models.User, int, error) {
	return nil, 0, nil
}

// Compile-time interface checks
var _ repo.SSOStore = (*mockSSORepo)(nil)
var _ repo.TeamStore = (*mockTeamRepo)(nil)
var _ repo.UserStore = (*mockUserRepo)(nil)

// --- Tests ---

func TestBuildSP(t *testing.T) {
	certPEM, keyPEM, err := GenerateSPCertificate()
	require.NoError(t, err)

	svc := NewService(Config{
		SPCertPEM: string(certPEM),
		SPKeyPEM:  string(keyPEM),
		BaseURL:   "https://app.dagryn.dev",
	}, newMockSSORepo(), newMockUserRepo(), newMockTeamRepo())

	conn := &models.SSOConnection{
		IDPEntityID: "https://idp.example.com",
		IDPSsoURL:   "https://idp.example.com/sso",
		SPEntityID:  "https://app.dagryn.dev/api/v1/sso/test-team",
		SPAcsURL:    "https://app.dagryn.dev/api/v1/sso/test-team/acs",
		Certificate: string(certPEM), // reuse for test
	}

	sp, err := svc.BuildSP(conn)
	require.NoError(t, err)
	assert.Equal(t, "https://app.dagryn.dev/api/v1/sso/test-team", sp.EntityID)
	assert.NotNil(t, sp.Key)
	assert.NotNil(t, sp.Certificate)
	assert.NotNil(t, sp.IDPMetadata)
}

func TestBuildSP_WithMetadataXML(t *testing.T) {
	certPEM, keyPEM, err := GenerateSPCertificate()
	require.NoError(t, err)

	metadataXML := `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">
		<IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
			<SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
		</IDPSSODescriptor>
	</EntityDescriptor>`

	svc := NewService(Config{
		SPCertPEM: string(certPEM),
		SPKeyPEM:  string(keyPEM),
		BaseURL:   "https://app.dagryn.dev",
	}, newMockSSORepo(), newMockUserRepo(), newMockTeamRepo())

	conn := &models.SSOConnection{
		IDPEntityID:    "https://idp.example.com",
		IDPSsoURL:      "https://idp.example.com/sso",
		IDPMetadataXML: &metadataXML,
		SPEntityID:     "https://app.dagryn.dev/api/v1/sso/team",
		SPAcsURL:       "https://app.dagryn.dev/api/v1/sso/team/acs",
	}

	sp, err := svc.BuildSP(conn)
	require.NoError(t, err)
	assert.Equal(t, "https://idp.example.com", sp.IDPMetadata.EntityID)
}

func TestIsSSOMandatory(t *testing.T) {
	ctx := context.Background()
	ssoRepo := newMockSSORepo()
	svc := NewService(Config{}, ssoRepo, newMockUserRepo(), newMockTeamRepo())

	teamID := uuid.New()

	t.Run("returns false when no connection", func(t *testing.T) {
		mandatory, err := svc.IsSSOMandatory(ctx, teamID)
		require.NoError(t, err)
		assert.False(t, mandatory)
	})

	t.Run("returns false when enforce_sso is false", func(t *testing.T) {
		conn := &models.SSOConnection{
			TeamID:     teamID,
			EnforceSSO: false,
		}
		require.NoError(t, ssoRepo.CreateConnection(ctx, conn))

		mandatory, err := svc.IsSSOMandatory(ctx, teamID)
		require.NoError(t, err)
		assert.False(t, mandatory)
	})

	t.Run("returns true when enforce_sso is true", func(t *testing.T) {
		conn, _ := ssoRepo.GetConnectionByTeamID(ctx, teamID)
		conn.EnforceSSO = true
		require.NoError(t, ssoRepo.UpdateConnection(ctx, conn))

		mandatory, err := svc.IsSSOMandatory(ctx, teamID)
		require.NoError(t, err)
		assert.True(t, mandatory)
	})
}

func TestRelayStateFlow(t *testing.T) {
	ctx := context.Background()
	ssoRepo := newMockSSORepo()

	connID := uuid.New()
	relayState := "test-relay-state"

	// Create state
	state := &models.SSOState{
		ConnectionID: connID,
		RelayState:   relayState,
		RedirectURL:  "/dashboard",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
	require.NoError(t, ssoRepo.CreateState(ctx, state))

	// Retrieve state
	retrieved, err := ssoRepo.GetStateByRelayState(ctx, relayState)
	require.NoError(t, err)
	assert.Equal(t, connID, retrieved.ConnectionID)
	assert.Equal(t, "/dashboard", retrieved.RedirectURL)

	// Delete state (prevents replay)
	require.NoError(t, ssoRepo.DeleteState(ctx, retrieved.ID))

	// State should be gone
	_, err = ssoRepo.GetStateByRelayState(ctx, relayState)
	assert.Equal(t, repo.ErrNotFound, err)
}

func TestRelayStateExpiry(t *testing.T) {
	ctx := context.Background()
	ssoRepo := newMockSSORepo()

	state := &models.SSOState{
		ConnectionID: uuid.New(),
		RelayState:   "expired-state",
		RedirectURL:  "/",
		ExpiresAt:    time.Now().Add(-1 * time.Minute), // already expired
	}
	require.NoError(t, ssoRepo.CreateState(ctx, state))

	_, err := ssoRepo.GetStateByRelayState(ctx, "expired-state")
	assert.Equal(t, repo.ErrNotFound, err)
}
