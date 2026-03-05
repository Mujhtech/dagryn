package models

import (
	"time"

	"github.com/google/uuid"
)

// SSOConnection represents an SSO/SAML configuration for a team.
type SSOConnection struct {
	ID             uuid.UUID `json:"id" db:"id"`
	TeamID         uuid.UUID `json:"team_id" db:"team_id"`
	IDPEntityID    string    `json:"idp_entity_id" db:"idp_entity_id"`
	IDPSsoURL      string    `json:"idp_sso_url" db:"idp_sso_url"`
	IDPMetadataURL *string   `json:"idp_metadata_url,omitempty" db:"idp_metadata_url"`
	IDPMetadataXML *string   `json:"-" db:"idp_metadata_xml"`
	Certificate    string    `json:"-" db:"certificate"`
	SPEntityID     string    `json:"sp_entity_id" db:"sp_entity_id"`
	SPAcsURL       string    `json:"sp_acs_url" db:"sp_acs_url"`
	SCIMEnabled    bool      `json:"scim_enabled" db:"scim_enabled"`
	SCIMTokenHash  *string   `json:"-" db:"scim_token_hash"`
	EnforceSSO     bool      `json:"enforce_sso" db:"enforce_sso"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// SSOState represents a SAML relay state for CSRF protection.
type SSOState struct {
	ID           uuid.UUID `json:"id" db:"id"`
	ConnectionID uuid.UUID `json:"connection_id" db:"connection_id"`
	RelayState   string    `json:"relay_state" db:"relay_state"`
	RedirectURL  string    `json:"redirect_url" db:"redirect_url"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
