package models

import (
	"time"

	"github.com/google/uuid"
)

// ProviderToken stores an OAuth provider access token for a user (e.g. GitHub for listing repos).
type ProviderToken struct {
	ID                   uuid.UUID `json:"id" db:"id"`
	UserID               uuid.UUID `json:"user_id" db:"user_id"`
	Provider             string    `json:"provider" db:"provider"`
	AccessTokenEncrypted string    `json:"-" db:"access_token_encrypted"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}
