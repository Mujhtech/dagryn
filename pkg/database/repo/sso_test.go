package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/stretchr/testify/assert"
)

func TestSSOConnection_Model(t *testing.T) {
	now := time.Now()
	teamID := uuid.New()
	connID := uuid.New()

	conn := models.SSOConnection{
		ID:          connID,
		TeamID:      teamID,
		IDPEntityID: "https://idp.example.com/entity",
		IDPSsoURL:   "https://idp.example.com/sso",
		SPEntityID:  "https://app.dagryn.dev/api/v1/sso/test-team",
		SPAcsURL:    "https://app.dagryn.dev/api/v1/sso/test-team/acs",
		Certificate: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		EnforceSSO:  false,
		SCIMEnabled: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, connID, conn.ID)
	assert.Equal(t, teamID, conn.TeamID)
	assert.Equal(t, "https://idp.example.com/entity", conn.IDPEntityID)
	assert.Equal(t, "https://idp.example.com/sso", conn.IDPSsoURL)
	assert.False(t, conn.EnforceSSO)
	assert.False(t, conn.SCIMEnabled)
	assert.Nil(t, conn.IDPMetadataURL)
	assert.Nil(t, conn.SCIMTokenHash)
}

func TestSSOConnection_WithOptionalFields(t *testing.T) {
	metadataURL := "https://idp.example.com/metadata"
	tokenHash := "$2a$10$somehash"

	conn := models.SSOConnection{
		ID:             uuid.New(),
		TeamID:         uuid.New(),
		IDPEntityID:    "https://idp.example.com",
		IDPSsoURL:      "https://idp.example.com/sso",
		IDPMetadataURL: &metadataURL,
		SPEntityID:     "https://app.dagryn.dev/api/v1/sso/team",
		SPAcsURL:       "https://app.dagryn.dev/api/v1/sso/team/acs",
		SCIMEnabled:    true,
		SCIMTokenHash:  &tokenHash,
		EnforceSSO:     true,
	}

	assert.NotNil(t, conn.IDPMetadataURL)
	assert.Equal(t, metadataURL, *conn.IDPMetadataURL)
	assert.True(t, conn.SCIMEnabled)
	assert.NotNil(t, conn.SCIMTokenHash)
	assert.True(t, conn.EnforceSSO)
}

func TestSSOState_Model(t *testing.T) {
	now := time.Now()
	connID := uuid.New()
	stateID := uuid.New()

	state := models.SSOState{
		ID:           stateID,
		ConnectionID: connID,
		RelayState:   "random-relay-state",
		RedirectURL:  "/dashboard",
		ExpiresAt:    now.Add(5 * time.Minute),
		CreatedAt:    now,
	}

	assert.Equal(t, stateID, state.ID)
	assert.Equal(t, connID, state.ConnectionID)
	assert.Equal(t, "random-relay-state", state.RelayState)
	assert.Equal(t, "/dashboard", state.RedirectURL)
	assert.True(t, state.ExpiresAt.After(now))
}
