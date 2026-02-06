// Package models contains database model definitions.
package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user account authenticated via OAuth.
type User struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Email      string    `json:"email" db:"email"`
	Name       *string   `json:"name,omitempty" db:"name"`
	AvatarURL  *string   `json:"avatar_url,omitempty" db:"avatar_url"`
	Provider   string    `json:"provider" db:"provider"` // 'github' or 'google'
	ProviderID string    `json:"provider_id" db:"provider_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// AuthProvider represents supported OAuth providers.
type AuthProvider string

const (
	AuthProviderGitHub AuthProvider = "github"
	AuthProviderGoogle AuthProvider = "google"
)

// IsValidProvider checks if the provider is valid.
func IsValidProvider(p string) bool {
	switch AuthProvider(p) {
	case AuthProviderGitHub, AuthProviderGoogle:
		return true
	}
	return false
}
