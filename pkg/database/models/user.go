// Package models contains database model definitions.
package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user account authenticated via OAuth or SAML.
type User struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	Email          string     `json:"email" db:"email"`
	Name           *string    `json:"name,omitempty" db:"name"`
	AvatarURL      *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	Provider       string     `json:"provider" db:"provider"` // 'github', 'google', or 'saml'
	ProviderID     string     `json:"provider_id" db:"provider_id"`
	DeactivatedAt  *time.Time `json:"deactivated_at,omitempty" db:"deactivated_at"`
	SCIMExternalID *string    `json:"scim_external_id,omitempty" db:"scim_external_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// AuthProvider represents supported OAuth providers.
type AuthProvider string

const (
	AuthProviderGitHub AuthProvider = "github"
	AuthProviderGoogle AuthProvider = "google"
	AuthProviderSAML   AuthProvider = "saml"
)

// IsValidProvider checks if the provider is valid.
func IsValidProvider(p string) bool {
	switch AuthProvider(p) {
	case AuthProviderGitHub, AuthProviderGoogle, AuthProviderSAML:
		return true
	}
	return false
}
