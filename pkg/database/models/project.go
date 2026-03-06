package models

import (
	"time"

	"github.com/google/uuid"
)

// Project represents a linked workflow project.
type Project struct {
	ID                   uuid.UUID  `json:"id" db:"id"`
	TeamID               *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	Name                 string     `json:"name" db:"name"`
	Slug                 string     `json:"slug" db:"slug"`
	PathHash             *string    `json:"path_hash,omitempty" db:"path_hash"`                           // SHA-256 of absolute path
	RepoURL              *string    `json:"repo_url,omitempty" db:"repo_url"`                             // For remote run execution (clone on trigger)
	RepoLinkedByUserID   *uuid.UUID `json:"repo_linked_by_user_id,omitempty" db:"repo_linked_by_user_id"` // User whose provider token is used to clone (legacy OAuth/PAT)
	GitHubInstallationID *uuid.UUID `json:"github_installation_id,omitempty" db:"github_installation_id"` // GitHub App installation that owns this repo
	GitHubRepoID         *int64     `json:"github_repo_id,omitempty" db:"github_repo_id"`                 // Numeric GitHub repository ID
	BillingAccountID     *uuid.UUID `json:"billing_account_id,omitempty" db:"billing_account_id"`         // Links to billing account for quota enforcement
	DefaultBranch        *string    `json:"default_branch,omitempty" db:"default_branch"`                 // Default git branch (e.g. "main")
	Description          *string    `json:"description,omitempty" db:"description"`
	Visibility           Visibility `json:"visibility" db:"visibility"`
	ConfigPath           string     `json:"config_path" db:"config_path"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
	LastRunAt            *time.Time `json:"last_run_at,omitempty" db:"last_run_at"`
}

// Visibility represents project visibility.
type Visibility string

const (
	VisibilityPrivate Visibility = "private" // Only members can access
	VisibilityTeam    Visibility = "team"    // All team members can access
	VisibilityPublic  Visibility = "public"  // Anyone can view (read-only)
)

// IsValidVisibility checks if the visibility is valid.
func IsValidVisibility(v Visibility) bool {
	switch v {
	case VisibilityPrivate, VisibilityTeam, VisibilityPublic:
		return true
	}
	return false
}

// ProjectMember represents a user's membership in a project.
type ProjectMember struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	ProjectID uuid.UUID  `json:"project_id" db:"project_id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Role      Role       `json:"role" db:"role"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty" db:"invited_by"`
	JoinedAt  time.Time  `json:"joined_at" db:"joined_at"`
}

// ProjectWithMember combines project data with membership info.
type ProjectWithMember struct {
	Project
	Role     Role      `json:"role" db:"role"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

// ProjectMemberWithUser combines membership data with user info.
type ProjectMemberWithUser struct {
	ProjectMember
	User User `json:"user"`
}

// ProjectWithTeam combines project data with team info.
type ProjectWithTeam struct {
	Project
	TeamName *string `json:"team_name,omitempty" db:"team_name"`
	TeamSlug *string `json:"team_slug,omitempty" db:"team_slug"`
}
