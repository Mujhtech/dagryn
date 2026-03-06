package handlers

import (
	"errors"
	"time"

	"github.com/mujhtech/dagryn/pkg/database/models"
)

func projectModelToResponse(project *models.Project, role models.Role) ProjectResponse {
	resp := ProjectResponse{
		ID:         project.ID,
		Name:       project.Name,
		Slug:       project.Slug,
		Visibility: string(project.Visibility),
		CreatedAt:  project.CreatedAt,
		UpdatedAt:  project.UpdatedAt,
		ConfigPath: project.ConfigPath,
		// RepoLinkedByUserID: project.RepoLinkedByUserID,
		LastRunAt: project.LastRunAt,
	}

	if project.RepoURL != nil {
		resp.RepoURL = *project.RepoURL
	}

	if project.TeamID != nil {
		resp.TeamID = *project.TeamID
	}
	if project.Description != nil {
		resp.Description = *project.Description
	}
	return resp
}

func projectWithMemberToResponse(project *models.ProjectWithMember) ProjectResponse {
	resp := ProjectResponse{
		ID:         project.ID,
		Name:       project.Name,
		Slug:       project.Slug,
		Visibility: string(project.Visibility),
		CreatedAt:  project.CreatedAt,
		UpdatedAt:  project.UpdatedAt,
		// RepoLinkedByUserID: project.RepoLinkedByUserID,
		LastRunAt:  project.LastRunAt,
		ConfigPath: project.ConfigPath,
	}
	if project.RepoURL != nil {
		resp.RepoURL = *project.RepoURL
	}
	if project.TeamID != nil {
		resp.TeamID = *project.TeamID
	}
	if project.Description != nil {
		resp.Description = *project.Description
	}
	return resp
}

func projectMemberWithUserToResponse(member *models.ProjectMemberWithUser) ProjectMemberResponse {
	return ProjectMemberResponse{
		User:     userModelToResponse(&member.User),
		Role:     string(member.Role),
		JoinedAt: member.JoinedAt,
	}
}

func apiKeyModelToResponse(key *models.APIKey) APIKeyResponse {
	return APIKeyResponse{
		ID:         key.ID,
		Name:       key.Name,
		Prefix:     key.KeyPrefix,
		Scope:      string(key.Scope),
		ProjectID:  key.ProjectID,
		LastUsedAt: key.LastUsedAt,
		ExpiresAt:  key.ExpiresAt,
		CreatedAt:  key.CreatedAt,
	}
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parseDuration parses a duration string like "90d", "30d", "1y"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, errors.New("invalid duration")
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]

	var value int
	for _, c := range valueStr {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid duration value")
		}
		value = value*10 + int(c-'0')
	}

	switch unit {
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(value) * 365 * 24 * time.Hour, nil
	case 'h':
		return time.Duration(value) * time.Hour, nil
	default:
		return 0, errors.New("invalid duration unit")
	}
}
