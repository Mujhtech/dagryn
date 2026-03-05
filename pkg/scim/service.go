package scim

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
)

// Service provides SCIM 2.0 provisioning operations.
type Service struct {
	userRepo repo.UserStore
	teamRepo repo.TeamStore
	baseURL  string
}

// NewService creates a new SCIM service.
func NewService(userRepo repo.UserStore, teamRepo repo.TeamStore, baseURL string) *Service {
	return &Service{
		userRepo: userRepo,
		teamRepo: teamRepo,
		baseURL:  baseURL,
	}
}

// ListUsers returns a paginated list of SCIM users for a team.
func (s *Service) ListUsers(ctx context.Context, teamID uuid.UUID, filter *Filter, startIndex, count int) (*ListResponse, error) {
	if startIndex < 1 {
		startIndex = 1
	}
	if count <= 0 || count > 100 {
		count = 100
	}

	// If there's a filter, handle it specially
	if filter != nil {
		return s.listUsersFiltered(ctx, teamID, filter, startIndex, count)
	}

	users, total, err := s.userRepo.ListByTeamForSCIM(ctx, teamID, startIndex, count)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	resources := make([]SCIMUser, 0, len(users))
	for _, u := range users {
		resources = append(resources, userToSCIM(u, s.baseURL))
	}

	return &ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: total,
		StartIndex:   startIndex,
		ItemsPerPage: count,
		Resources:    resources,
	}, nil
}

func (s *Service) listUsersFiltered(ctx context.Context, teamID uuid.UUID, filter *Filter, startIndex, count int) (*ListResponse, error) {
	var user *models.User
	var err error

	switch filter.Attribute {
	case "userName", "emails.value":
		user, err = s.userRepo.GetByEmail(ctx, filter.Value)
	case "externalId":
		user, err = s.userRepo.GetBySCIMExternalID(ctx, filter.Value)
	default:
		return &ListResponse{
			Schemas:      []string{SchemaListResponse},
			TotalResults: 0,
			StartIndex:   startIndex,
			ItemsPerPage: count,
			Resources:    []SCIMUser{},
		}, nil
	}

	if err != nil {
		if err == repo.ErrNotFound {
			return &ListResponse{
				Schemas:      []string{SchemaListResponse},
				TotalResults: 0,
				StartIndex:   startIndex,
				ItemsPerPage: count,
				Resources:    []SCIMUser{},
			}, nil
		}
		return nil, err
	}

	// Verify user is a member of the team
	_, err = s.teamRepo.GetMember(ctx, teamID, user.ID)
	if err != nil {
		return &ListResponse{
			Schemas:      []string{SchemaListResponse},
			TotalResults: 0,
			StartIndex:   startIndex,
			ItemsPerPage: count,
			Resources:    []SCIMUser{},
		}, nil
	}

	return &ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources:    []SCIMUser{userToSCIM(*user, s.baseURL)},
	}, nil
}

// GetUser returns a SCIM user by ID.
func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (*SCIMUser, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := userToSCIM(*user, s.baseURL)
	return &result, nil
}

// CreateUser creates a new user via SCIM provisioning.
func (s *Service) CreateUser(ctx context.Context, teamID uuid.UUID, scimUser SCIMUser) (*SCIMUser, error) {
	email := ""
	if len(scimUser.Emails) > 0 {
		email = scimUser.Emails[0].Value
	}
	if email == "" {
		email = scimUser.UserName
	}

	name := scimUser.DisplayName
	if name == "" && scimUser.Name != nil {
		name = scimUser.Name.Formatted
		if name == "" {
			name = fmt.Sprintf("%s %s", scimUser.Name.GivenName, scimUser.Name.FamilyName)
		}
	}

	// Check if user already exists by email
	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil && err != repo.ErrNotFound {
		return nil, fmt.Errorf("check existing user: %w", err)
	}

	var user *models.User
	if existingUser != nil {
		// Update SCIM external ID
		existingUser.SCIMExternalID = &scimUser.ExternalID
		if err := s.userRepo.Update(ctx, existingUser); err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
		user = existingUser

		// Reactivate if deactivated
		if user.DeactivatedAt != nil {
			_ = s.userRepo.Reactivate(ctx, user.ID)
			user.DeactivatedAt = nil
		}
	} else {
		// Create new user
		externalID := scimUser.ExternalID
		user = &models.User{
			Email:          email,
			Name:           &name,
			Provider:       string(models.AuthProviderSAML),
			ProviderID:     email,
			SCIMExternalID: &externalID,
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
	}

	// Ensure team membership
	_, err = s.teamRepo.GetMember(ctx, teamID, user.ID)
	if err != nil {
		if err == repo.ErrNotFound {
			if err := s.teamRepo.AddMember(ctx, teamID, user.ID, models.RoleMember, nil); err != nil {
				return nil, fmt.Errorf("add team member: %w", err)
			}
		} else {
			return nil, fmt.Errorf("check team member: %w", err)
		}
	}

	result := userToSCIM(*user, s.baseURL)
	return &result, nil
}

// UpdateUser replaces a user via SCIM (PUT).
func (s *Service) UpdateUser(ctx context.Context, userID uuid.UUID, scimUser SCIMUser) (*SCIMUser, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	email := ""
	if len(scimUser.Emails) > 0 {
		email = scimUser.Emails[0].Value
	}
	if email == "" {
		email = scimUser.UserName
	}
	if email != "" {
		user.Email = email
	}

	name := scimUser.DisplayName
	if name == "" && scimUser.Name != nil {
		name = scimUser.Name.Formatted
	}
	if name != "" {
		user.Name = &name
	}

	if scimUser.ExternalID != "" {
		user.SCIMExternalID = &scimUser.ExternalID
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// Handle active flag
	if !scimUser.Active && user.DeactivatedAt == nil {
		_ = s.userRepo.Deactivate(ctx, user.ID)
	} else if scimUser.Active && user.DeactivatedAt != nil {
		_ = s.userRepo.Reactivate(ctx, user.ID)
	}

	result := userToSCIM(*user, s.baseURL)
	return &result, nil
}

// PatchUser applies a SCIM PATCH operation to a user.
func (s *Service) PatchUser(ctx context.Context, userID uuid.UUID, patch PatchOp) (*SCIMUser, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, op := range patch.Operations {
		switch op.Op {
		case "replace", "Replace":
			s.applyUserPatchReplace(ctx, user, op)
		case "add", "Add":
			s.applyUserPatchReplace(ctx, user, op) // same logic for add
		}
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	result := userToSCIM(*user, s.baseURL)
	return &result, nil
}

func (s *Service) applyUserPatchReplace(ctx context.Context, user *models.User, op PatchOperation) {
	switch op.Path {
	case "active":
		if active, ok := op.Value.(bool); ok {
			if !active && user.DeactivatedAt == nil {
				_ = s.userRepo.Deactivate(ctx, user.ID)
			} else if active && user.DeactivatedAt != nil {
				_ = s.userRepo.Reactivate(ctx, user.ID)
			}
		}
	case "userName":
		if v, ok := op.Value.(string); ok {
			user.Email = v
		}
	case "displayName":
		if v, ok := op.Value.(string); ok {
			user.Name = &v
		}
	case "externalId":
		if v, ok := op.Value.(string); ok {
			user.SCIMExternalID = &v
		}
	case "":
		// Bulk replace (value is a map)
		if attrs, ok := op.Value.(map[string]interface{}); ok {
			if active, ok := attrs["active"].(bool); ok {
				if !active && user.DeactivatedAt == nil {
					_ = s.userRepo.Deactivate(ctx, user.ID)
				} else if active && user.DeactivatedAt != nil {
					_ = s.userRepo.Reactivate(ctx, user.ID)
				}
			}
		}
	}
}

// DeleteUser soft-deletes (deactivates) a user via SCIM.
func (s *Service) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.Deactivate(ctx, userID)
}

// ListGroups lists teams as SCIM groups (a team maps to one group).
func (s *Service) ListGroups(ctx context.Context, teamID uuid.UUID) (*ListResponse, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	members, err := s.teamRepo.ListMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}

	group := teamToSCIMGroup(*team, members, s.baseURL)

	return &ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources:    []SCIMGroup{group},
	}, nil
}

// GetGroup returns a team as a SCIM group.
func (s *Service) GetGroup(ctx context.Context, teamID uuid.UUID) (*SCIMGroup, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	members, err := s.teamRepo.ListMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}

	group := teamToSCIMGroup(*team, members, s.baseURL)
	return &group, nil
}

// PatchGroup applies a SCIM PATCH operation to a group (team).
func (s *Service) PatchGroup(ctx context.Context, teamID uuid.UUID, patch PatchOp) (*SCIMGroup, error) {
	for _, op := range patch.Operations {
		switch op.Op {
		case "add", "Add":
			if op.Path == "members" {
				s.addGroupMembers(ctx, teamID, op.Value)
			}
		case "remove", "Remove":
			if op.Path == "members" {
				s.removeGroupMembers(ctx, teamID, op.Value)
			}
		}
	}

	return s.GetGroup(ctx, teamID)
}

func (s *Service) addGroupMembers(ctx context.Context, teamID uuid.UUID, value interface{}) {
	members, ok := value.([]interface{})
	if !ok {
		return
	}
	for _, m := range members {
		member, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if memberID, ok := member["value"].(string); ok {
			uid, err := uuid.Parse(memberID)
			if err != nil {
				continue
			}
			_ = s.teamRepo.AddMember(ctx, teamID, uid, models.RoleMember, nil)
		}
	}
}

func (s *Service) removeGroupMembers(ctx context.Context, teamID uuid.UUID, value interface{}) {
	members, ok := value.([]interface{})
	if !ok {
		return
	}
	for _, m := range members {
		member, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if memberID, ok := member["value"].(string); ok {
			uid, err := uuid.Parse(memberID)
			if err != nil {
				continue
			}
			_ = s.teamRepo.RemoveMember(ctx, teamID, uid)
		}
	}
}

// userToSCIM converts a database User to a SCIM User resource.
func userToSCIM(u models.User, baseURL string) SCIMUser {
	name := ""
	if u.Name != nil {
		name = *u.Name
	}

	externalID := ""
	if u.SCIMExternalID != nil {
		externalID = *u.SCIMExternalID
	}

	return SCIMUser{
		Schemas:    []string{SchemaUser},
		ID:         u.ID.String(),
		ExternalID: externalID,
		UserName:   u.Email,
		DisplayName: name,
		Name: &SCIMName{
			Formatted: name,
		},
		Emails: []SCIMEmail{
			{Value: u.Email, Type: "work", Primary: true},
		},
		Active: u.DeactivatedAt == nil,
		Meta: SCIMMeta{
			ResourceType: "User",
			Created:      u.CreatedAt,
			LastModified: u.UpdatedAt,
			Location:     fmt.Sprintf("%s/scim/Users/%s", baseURL, u.ID.String()),
		},
	}
}

// teamToSCIMGroup converts a Team to a SCIM Group resource.
func teamToSCIMGroup(t models.Team, members []models.TeamMemberWithUser, baseURL string) SCIMGroup {
	scimMembers := make([]SCIMMember, 0, len(members))
	for _, m := range members {
		name := ""
		if m.User.Name != nil {
			name = *m.User.Name
		}
		scimMembers = append(scimMembers, SCIMMember{
			Value:   m.UserID.String(),
			Display: name,
		})
	}

	return SCIMGroup{
		Schemas:     []string{SchemaGroup},
		ID:          t.ID.String(),
		DisplayName: t.Name,
		Members:     scimMembers,
		Meta: SCIMMeta{
			ResourceType: "Group",
			Created:      t.CreatedAt,
			LastModified: t.UpdatedAt,
			Location:     fmt.Sprintf("%s/scim/Groups/%s", baseURL, t.ID.String()),
		},
	}
}
