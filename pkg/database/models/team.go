package models

import (
	"time"

	"github.com/google/uuid"
)

// Team represents a team for grouping projects and users.
type Team struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	OwnerID     uuid.UUID `json:"owner_id" db:"owner_id"`
	Description *string   `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// TeamMember represents a user's membership in a team.
type TeamMember struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	TeamID    uuid.UUID  `json:"team_id" db:"team_id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Role      Role       `json:"role" db:"role"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty" db:"invited_by"`
	JoinedAt  time.Time  `json:"joined_at" db:"joined_at"`
}

// TeamWithMember combines team data with membership info.
type TeamWithMember struct {
	Team
	Role     Role      `json:"role" db:"role"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

// TeamMemberWithUser combines membership data with user info.
type TeamMemberWithUser struct {
	TeamMember
	User User `json:"user"`
}
