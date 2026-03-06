package models

import (
	"time"

	"github.com/google/uuid"
)

// Invitation represents a pending team or project invitation.
type Invitation struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	Email      string     `json:"email" db:"email"`
	TeamID     *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	ProjectID  *uuid.UUID `json:"project_id,omitempty" db:"project_id"`
	Role       Role       `json:"role" db:"role"`
	InvitedBy  uuid.UUID  `json:"invited_by" db:"invited_by"`
	Token      string     `json:"-" db:"token"` // Never expose token in JSON
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty" db:"accepted_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// IsExpired returns true if the invitation has expired.
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsAccepted returns true if the invitation has been accepted.
func (i *Invitation) IsAccepted() bool {
	return i.AcceptedAt != nil
}

// IsPending returns true if the invitation is still pending.
func (i *Invitation) IsPending() bool {
	return !i.IsAccepted() && !i.IsExpired()
}

// IsTeamInvitation returns true if this is a team invitation.
func (i *Invitation) IsTeamInvitation() bool {
	return i.TeamID != nil
}

// IsProjectInvitation returns true if this is a project invitation.
func (i *Invitation) IsProjectInvitation() bool {
	return i.ProjectID != nil
}

// InvitationWithDetails combines invitation data with team/project and inviter info.
type InvitationWithDetails struct {
	Invitation
	TeamName     *string `json:"team_name,omitempty" db:"team_name"`
	TeamSlug     *string `json:"team_slug,omitempty" db:"team_slug"`
	ProjectName  *string `json:"project_name,omitempty" db:"project_name"`
	ProjectSlug  *string `json:"project_slug,omitempty" db:"project_slug"`
	InviterName  *string `json:"inviter_name,omitempty" db:"inviter_name"`
	InviterEmail string  `json:"inviter_email" db:"inviter_email"`
}

// DefaultInvitationExpiry is the default expiration time for invitations.
const DefaultInvitationExpiry = 7 * 24 * time.Hour // 7 days
