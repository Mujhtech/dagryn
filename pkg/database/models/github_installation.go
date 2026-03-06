package models

import (
	"time"

	"github.com/google/uuid"
)

// GitHubInstallation represents a GitHub App installation that Dagryn knows about.
type GitHubInstallation struct {
	ID             uuid.UUID `json:"id" db:"id"`
	InstallationID int64     `json:"installation_id" db:"installation_id"`
	AccountLogin   string    `json:"account_login" db:"account_login"`
	AccountType    string    `json:"account_type" db:"account_type"`
	AccountID      int64     `json:"account_id" db:"account_id"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}
