package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/sso"
	"golang.org/x/crypto/bcrypt"
)

// --- SSO Admin Endpoints (on Handler receiver, under /api/v1/teams/{teamID}/sso) ---

type createSSOConnectionInput struct {
	IDPEntityID    string  `json:"idp_entity_id"`
	IDPSsoURL      string  `json:"idp_sso_url"`
	IDPMetadataURL *string `json:"idp_metadata_url,omitempty"`
	IDPMetadataXML *string `json:"idp_metadata_xml,omitempty"`
	Certificate    string  `json:"certificate"`
}

type updateSSOConnectionInput struct {
	IDPEntityID    *string `json:"idp_entity_id,omitempty"`
	IDPSsoURL      *string `json:"idp_sso_url,omitempty"`
	IDPMetadataURL *string `json:"idp_metadata_url,omitempty"`
	IDPMetadataXML *string `json:"idp_metadata_xml,omitempty"`
	Certificate    *string `json:"certificate,omitempty"`
}

type toggleSSOEnforcementInput struct {
	Enforce bool `json:"enforce"`
}

// GetSSOConnection returns the SSO connection for a team.
func (h *Handler) GetSSOConnection(w http.ResponseWriter, r *http.Request) {
	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	conn, err := h.store.SSO.GetConnectionByTeamID(r.Context(), teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			_ = response.NotFound(w, r, fmt.Errorf("SSO connection not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "SSO connection retrieved", conn)
}

// CreateSSOConnection creates an SSO connection for a team.
func (h *Handler) CreateSSOConnection(w http.ResponseWriter, r *http.Request) {
	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	var input createSSOConnectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		_ = response.BadRequest(w, r, fmt.Errorf("invalid request body"))
		return
	}

	// Get the team to build SP URLs
	team, err := h.store.Teams.GetByID(r.Context(), teamID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	// Auto-derive SP entity ID and ACS URL from baseURL + team slug
	spEntityID := fmt.Sprintf("%s/api/v1/sso/%s", h.baseURL, team.Slug)
	spAcsURL := fmt.Sprintf("%s/api/v1/sso/%s/acs", h.baseURL, team.Slug)

	// Parse IdP metadata if URL provided
	if input.IDPMetadataURL != nil && *input.IDPMetadataURL != "" {
		metadata, err := sso.ParseIDPMetadataURL(*input.IDPMetadataURL)
		if err != nil {
			_ = response.BadRequest(w, r, fmt.Errorf("failed to fetch IdP metadata: %w", err))
			return
		}
		input.IDPEntityID = metadata.EntityID
		if len(metadata.IDPSSODescriptors) > 0 {
			for _, ssoSvc := range metadata.IDPSSODescriptors[0].SingleSignOnServices {
				input.IDPSsoURL = ssoSvc.Location
				break
			}
		}
	}

	conn := &models.SSOConnection{
		TeamID:         teamID,
		IDPEntityID:    input.IDPEntityID,
		IDPSsoURL:      input.IDPSsoURL,
		IDPMetadataURL: input.IDPMetadataURL,
		IDPMetadataXML: input.IDPMetadataXML,
		Certificate:    input.Certificate,
		SPEntityID:     spEntityID,
		SPAcsURL:       spAcsURL,
	}

	if err := h.store.SSO.CreateConnection(r.Context(), conn); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Created(w, r, "SSO connection created", conn)
}

// UpdateSSOConnection updates an SSO connection.
func (h *Handler) UpdateSSOConnection(w http.ResponseWriter, r *http.Request) {
	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	conn, err := h.store.SSO.GetConnectionByTeamID(r.Context(), teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			_ = response.NotFound(w, r, fmt.Errorf("SSO connection not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	var input updateSSOConnectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		_ = response.BadRequest(w, r, fmt.Errorf("invalid request body"))
		return
	}

	if input.IDPEntityID != nil {
		conn.IDPEntityID = *input.IDPEntityID
	}
	if input.IDPSsoURL != nil {
		conn.IDPSsoURL = *input.IDPSsoURL
	}
	if input.IDPMetadataURL != nil {
		conn.IDPMetadataURL = input.IDPMetadataURL

		// Re-parse metadata
		metadata, err := sso.ParseIDPMetadataURL(*input.IDPMetadataURL)
		if err == nil {
			conn.IDPEntityID = metadata.EntityID
			if len(metadata.IDPSSODescriptors) > 0 {
				for _, ssoSvc := range metadata.IDPSSODescriptors[0].SingleSignOnServices {
					conn.IDPSsoURL = ssoSvc.Location
					break
				}
			}
		}
	}
	if input.IDPMetadataXML != nil {
		conn.IDPMetadataXML = input.IDPMetadataXML
	}
	if input.Certificate != nil {
		conn.Certificate = *input.Certificate
	}

	if err := h.store.SSO.UpdateConnection(r.Context(), conn); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "SSO connection updated", conn)
}

// DeleteSSOConnection deletes an SSO connection.
func (h *Handler) DeleteSSOConnection(w http.ResponseWriter, r *http.Request) {
	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	conn, err := h.store.SSO.GetConnectionByTeamID(r.Context(), teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			_ = response.NotFound(w, r, fmt.Errorf("SSO connection not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	if err := h.store.SSO.DeleteConnection(r.Context(), conn.ID); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.NoContent(w, r)
}

// TestSSOConnection tests an SSO connection by attempting to build the SP.
func (h *Handler) TestSSOConnection(w http.ResponseWriter, r *http.Request) {
	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	conn, err := h.store.SSO.GetConnectionByTeamID(r.Context(), teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			_ = response.NotFound(w, r, fmt.Errorf("SSO connection not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	if h.ssoService == nil {
		_ = response.InternalServerError(w, r, fmt.Errorf("SSO service not configured"))
		return
	}

	_, err = h.ssoService.BuildSP(conn)
	if err != nil {
		_ = response.Ok(w, r, "SSO connection test failed", map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	_ = response.Ok(w, r, "SSO connection test passed", map[string]interface{}{
		"success": true,
	})
}

// ToggleSSOEnforcement toggles SSO enforcement for a team.
func (h *Handler) ToggleSSOEnforcement(w http.ResponseWriter, r *http.Request) {
	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	conn, err := h.store.SSO.GetConnectionByTeamID(r.Context(), teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			_ = response.NotFound(w, r, fmt.Errorf("SSO connection not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	var input toggleSSOEnforcementInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		_ = response.BadRequest(w, r, fmt.Errorf("invalid request body"))
		return
	}

	conn.EnforceSSO = input.Enforce
	if err := h.store.SSO.UpdateConnection(r.Context(), conn); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "SSO enforcement updated", conn)
}

// GenerateSCIMToken generates a new SCIM Bearer token for provisioning.
func (h *Handler) GenerateSCIMToken(w http.ResponseWriter, r *http.Request) {
	teamID, err := getTeamIDFromPath(r)
	if err != nil {
		_ = response.BadRequest(w, r, err)
		return
	}

	conn, err := h.store.SSO.GetConnectionByTeamID(r.Context(), teamID)
	if err != nil {
		if err == repo.ErrNotFound {
			_ = response.NotFound(w, r, fmt.Errorf("SSO connection not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Store bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	hashStr := string(hash)
	conn.SCIMTokenHash = &hashStr
	conn.SCIMEnabled = true
	if err := h.store.SSO.UpdateConnection(r.Context(), conn); err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "SCIM token generated", map[string]string{
		"token": token,
	})
}

// RotateSCIMToken rotates the SCIM Bearer token.
func (h *Handler) RotateSCIMToken(w http.ResponseWriter, r *http.Request) {
	// Same as GenerateSCIMToken — just regenerates
	h.GenerateSCIMToken(w, r)
}
