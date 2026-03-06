package models

import (
	"time"

	"github.com/google/uuid"
)

// Token represents a JWT reference record for revocation tracking.
type Token struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	ProjectID  *uuid.UUID `json:"project_id,omitempty" db:"project_id"` // NULL for user-scope
	TokenType  TokenType  `json:"token_type" db:"token_type"`
	JTI        string     `json:"jti" db:"jti"` // JWT ID for lookup
	IssuedAt   time.Time  `json:"issued_at" db:"issued_at"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// TokenType represents the type of token.
type TokenType string

const (
	TokenTypeAccess     TokenType = "access"
	TokenTypeRefresh    TokenType = "refresh"
	TokenTypeDeviceCode TokenType = "device_code"
)

// IsRevoked returns true if the token has been revoked.
func (t *Token) IsRevoked() bool {
	return t.RevokedAt != nil
}

// IsExpired returns true if the token has expired.
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid returns true if the token is neither revoked nor expired.
func (t *Token) IsValid() bool {
	return !t.IsRevoked() && !t.IsExpired()
}
