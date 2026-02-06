package models

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key for CLI/CI authentication.
type APIKey struct {
	ID         uuid.UUID   `json:"id" db:"id"`
	UserID     uuid.UUID   `json:"user_id" db:"user_id"`
	ProjectID  *uuid.UUID  `json:"project_id,omitempty" db:"project_id"` // NULL = user scope
	Name       string      `json:"name" db:"name"`
	KeyHash    string      `json:"-" db:"key_hash"` // Never expose hash
	KeyPrefix  string      `json:"key_prefix" db:"key_prefix"`
	Scope      APIKeyScope `json:"scope" db:"scope"`
	LastUsedAt *time.Time  `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt  *time.Time  `json:"expires_at,omitempty" db:"expires_at"` // NULL = never expires
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
	RevokedAt  *time.Time  `json:"revoked_at,omitempty" db:"revoked_at"`
}

// APIKeyScope represents the scope of an API key.
type APIKeyScope string

const (
	APIKeyScopeUser    APIKeyScope = "user"    // Access all user's projects
	APIKeyScopeProject APIKeyScope = "project" // Access specific project only
)

// APIKeyPrefix constants for different key types.
const (
	APIKeyPrefixLive    = "dg_live_" // User scope, production
	APIKeyPrefixProject = "dg_proj_" // Project scope
	APIKeyPrefixTest    = "dg_test_" // Test/dev environment
)

// IsRevoked returns true if the key has been revoked.
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// IsExpired returns true if the key has expired.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false // Never expires
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid returns true if the key is neither revoked nor expired.
func (k *APIKey) IsValid() bool {
	return !k.IsRevoked() && !k.IsExpired()
}

// APIKeyWithProject combines API key data with project info.
type APIKeyWithProject struct {
	APIKey
	ProjectName *string `json:"project_name,omitempty" db:"project_name"`
	ProjectSlug *string `json:"project_slug,omitempty" db:"project_slug"`
}

// ExpiryOption represents predefined expiry options.
type ExpiryOption struct {
	Label    string
	Duration *time.Duration // nil = never expires
}

// ExpiryOptions provides the available expiry options (like GitHub).
var ExpiryOptions = []ExpiryOption{
	{Label: "7 days", Duration: durationPtr(7 * 24 * time.Hour)},
	{Label: "30 days", Duration: durationPtr(30 * 24 * time.Hour)},
	{Label: "60 days", Duration: durationPtr(60 * 24 * time.Hour)},
	{Label: "90 days", Duration: durationPtr(90 * 24 * time.Hour)},
	{Label: "1 year", Duration: durationPtr(365 * 24 * time.Hour)},
	{Label: "No expiration", Duration: nil},
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}
