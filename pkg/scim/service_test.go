package scim

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

type mockUserRepo struct {
	users    map[uuid.UUID]*models.User
	byEmail  map[string]*models.User
	bySCIMID map[string]*models.User
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
	if user.SCIMExternalID != nil {
		m.bySCIMID[*user.SCIMExternalID] = user
	}
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
	if user.SCIMExternalID != nil {
		m.bySCIMID[*user.SCIMExternalID] = user
	}
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
	// Return all users (simplified for testing)
	users := make([]models.User, 0)
	for _, u := range m.users {
		users = append(users, *u)
	}
	return users, len(users), nil
}

type mockTeamRepo struct {
	teams   map[uuid.UUID]*models.Team
	members map[uuid.UUID]map[uuid.UUID]*models.TeamMember
}

func newMockTeamRepo() *mockTeamRepo {
	return &mockTeamRepo{
		teams:   make(map[uuid.UUID]*models.Team),
		members: make(map[uuid.UUID]map[uuid.UUID]*models.TeamMember),
	}
}

func (m *mockTeamRepo) Create(ctx context.Context, team *models.Team) error { return nil }
func (m *mockTeamRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	team, ok := m.teams[id]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return team, nil
}

func (m *mockTeamRepo) GetBySlug(ctx context.Context, slug string) (*models.Team, error) {
	return nil, repo.ErrNotFound
}

func (m *mockTeamRepo) Update(ctx context.Context, team *models.Team) error     { return nil }
func (m *mockTeamRepo) Delete(ctx context.Context, id uuid.UUID) error          { return nil }
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

// Compile-time interface checks
var _ repo.UserStore = (*mockUserRepo)(nil)
var _ repo.TeamStore = (*mockTeamRepo)(nil)

// --- Tests ---

func TestUserToSCIM(t *testing.T) {
	name := "John Doe"
	extID := "ext-123"
	now := time.Now()
	userID := uuid.New()

	user := models.User{
		ID:             userID,
		Email:          "john@example.com",
		Name:           &name,
		Provider:       "saml",
		ProviderID:     "john@example.com",
		SCIMExternalID: &extID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	scimUser := userToSCIM(user, "https://app.dagryn.dev")

	assert.Equal(t, []string{SchemaUser}, scimUser.Schemas)
	assert.Equal(t, userID.String(), scimUser.ID)
	assert.Equal(t, "ext-123", scimUser.ExternalID)
	assert.Equal(t, "john@example.com", scimUser.UserName)
	assert.Equal(t, "John Doe", scimUser.DisplayName)
	assert.True(t, scimUser.Active)
	require.Len(t, scimUser.Emails, 1)
	assert.Equal(t, "john@example.com", scimUser.Emails[0].Value)
	assert.True(t, scimUser.Emails[0].Primary)
	assert.Equal(t, "User", scimUser.Meta.ResourceType)
	assert.Contains(t, scimUser.Meta.Location, userID.String())
}

func TestUserToSCIM_Deactivated(t *testing.T) {
	now := time.Now()
	user := models.User{
		ID:            uuid.New(),
		Email:         "inactive@example.com",
		DeactivatedAt: &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	scimUser := userToSCIM(user, "https://app.dagryn.dev")
	assert.False(t, scimUser.Active)
}

func TestUserToSCIM_NilOptionalFields(t *testing.T) {
	user := models.User{
		ID:        uuid.New(),
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	scimUser := userToSCIM(user, "https://app.dagryn.dev")
	assert.Empty(t, scimUser.ExternalID)
	assert.Empty(t, scimUser.DisplayName)
	assert.True(t, scimUser.Active)
}

func TestCreateUser_NewUser(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	teamRepo := newMockTeamRepo()
	teamID := uuid.New()
	teamRepo.teams[teamID] = &models.Team{ID: teamID, Name: "Test"}
	teamRepo.members[teamID] = make(map[uuid.UUID]*models.TeamMember)

	svc := NewService(userRepo, teamRepo, "https://app.dagryn.dev")

	scimUser := SCIMUser{
		UserName:    "new@example.com",
		DisplayName: "New User",
		ExternalID:  "ext-new",
		Emails: []SCIMEmail{
			{Value: "new@example.com", Type: "work", Primary: true},
		},
	}

	result, err := svc.CreateUser(ctx, teamID, scimUser)
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", result.UserName)
	assert.Equal(t, "New User", result.DisplayName)
	assert.True(t, result.Active)

	// Verify team membership was created
	assert.Len(t, teamRepo.members[teamID], 1)
}

func TestCreateUser_ExistingUser(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	teamRepo := newMockTeamRepo()
	teamID := uuid.New()
	teamRepo.teams[teamID] = &models.Team{ID: teamID, Name: "Test"}
	teamRepo.members[teamID] = make(map[uuid.UUID]*models.TeamMember)

	// Pre-create user
	existingUser := &models.User{
		Email:      "existing@example.com",
		Provider:   "github",
		ProviderID: "12345",
	}
	require.NoError(t, userRepo.Create(ctx, existingUser))

	svc := NewService(userRepo, teamRepo, "https://app.dagryn.dev")

	scimUser := SCIMUser{
		UserName:   "existing@example.com",
		ExternalID: "ext-existing",
		Emails: []SCIMEmail{
			{Value: "existing@example.com"},
		},
	}

	result, err := svc.CreateUser(ctx, teamID, scimUser)
	require.NoError(t, err)
	assert.Equal(t, "existing@example.com", result.UserName)
	// Should have updated SCIM external ID
	assert.Equal(t, "ext-existing", result.ExternalID)
}

func TestDeleteUser_Deactivates(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	teamRepo := newMockTeamRepo()

	user := &models.User{Email: "del@example.com", Provider: "saml", ProviderID: "del@example.com"}
	require.NoError(t, userRepo.Create(ctx, user))
	assert.Nil(t, user.DeactivatedAt)

	svc := NewService(userRepo, teamRepo, "https://app.dagryn.dev")

	err := svc.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify user is deactivated, not deleted
	u, err := userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.NotNil(t, u.DeactivatedAt)
}

func TestListUsers_Pagination(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	teamRepo := newMockTeamRepo()
	teamID := uuid.New()

	svc := NewService(userRepo, teamRepo, "https://app.dagryn.dev")

	// Test default pagination
	result, err := svc.ListUsers(ctx, teamID, nil, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{SchemaListResponse}, result.Schemas)
	assert.Equal(t, 1, result.StartIndex) // corrected from 0
}

func TestListUsers_WithFilter(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	teamRepo := newMockTeamRepo()
	teamID := uuid.New()
	teamRepo.members[teamID] = make(map[uuid.UUID]*models.TeamMember)

	// Create a user and add to team
	user := &models.User{Email: "filter@example.com", Provider: "saml", ProviderID: "filter@example.com"}
	require.NoError(t, userRepo.Create(ctx, user))
	teamRepo.members[teamID][user.ID] = &models.TeamMember{TeamID: teamID, UserID: user.ID}

	svc := NewService(userRepo, teamRepo, "https://app.dagryn.dev")

	filter := &Filter{Attribute: "userName", Operator: "eq", Value: "filter@example.com"}
	result, err := svc.ListUsers(ctx, teamID, filter, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalResults)
}

func TestListUsers_FilterNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	teamRepo := newMockTeamRepo()
	teamID := uuid.New()

	svc := NewService(userRepo, teamRepo, "https://app.dagryn.dev")

	filter := &Filter{Attribute: "userName", Operator: "eq", Value: "nonexistent@example.com"}
	result, err := svc.ListUsers(ctx, teamID, filter, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, result.TotalResults)
}

func TestPatchUser_Deactivate(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	teamRepo := newMockTeamRepo()

	user := &models.User{Email: "patch@example.com", Provider: "saml", ProviderID: "patch@example.com"}
	require.NoError(t, userRepo.Create(ctx, user))

	svc := NewService(userRepo, teamRepo, "https://app.dagryn.dev")

	patch := PatchOp{
		Schemas: []string{SchemaPatchOp},
		Operations: []PatchOperation{
			{Op: "replace", Path: "active", Value: false},
		},
	}

	result, err := svc.PatchUser(ctx, user.ID, patch)
	require.NoError(t, err)
	// User should be deactivated
	u, _ := userRepo.GetByID(ctx, user.ID)
	assert.NotNil(t, u.DeactivatedAt)
	_ = result
}
