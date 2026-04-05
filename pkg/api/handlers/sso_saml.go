package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/mujhtech/dagryn/pkg/authz"
	"github.com/mujhtech/dagryn/pkg/database/store"
	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/sso"
)

// SSOHandler handles public SAML SP endpoints.
type SSOHandler struct {
	ssoService *sso.Service
	jwtService *authz.JWTService
	store      store.Store
	baseURL    string
}

// NewSSOHandler creates a new SSO handler.
func NewSSOHandler(ssoService *sso.Service, jwtService *authz.JWTService, store store.Store, baseURL string) *SSOHandler {
	return &SSOHandler{
		ssoService: ssoService,
		jwtService: jwtService,
		store:      store,
		baseURL:    baseURL,
	}
}

// SSOMetadata returns the SP metadata XML for a team.
func (h *SSOHandler) SSOMetadata(w http.ResponseWriter, r *http.Request) {
	teamSlug := chi.URLParam(r, TeamSlugParam)
	if teamSlug == "" {
		_ = response.BadRequest(w, r, fmt.Errorf("missing team slug"))
		return
	}

	data, err := h.ssoService.GenerateMetadata(r.Context(), teamSlug)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// SSOLogin initiates a SAML login by redirecting to the IdP.
func (h *SSOHandler) SSOLogin(w http.ResponseWriter, r *http.Request) {
	teamSlug := chi.URLParam(r, TeamSlugParam)
	if teamSlug == "" {
		_ = response.BadRequest(w, r, fmt.Errorf("missing team slug"))
		return
	}

	redirectURL := r.URL.Query().Get("redirect_url")
	if redirectURL == "" {
		redirectURL = h.baseURL
	}

	redirectTo, err := h.ssoService.InitiateLogin(r.Context(), teamSlug, redirectURL)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	http.Redirect(w, r, redirectTo, http.StatusFound)
}

// SSOACS processes the SAML assertion consumer service callback.
func (h *SSOHandler) SSOACS(w http.ResponseWriter, r *http.Request) {
	teamSlug := chi.URLParam(r, TeamSlugParam)
	if teamSlug == "" {
		_ = response.BadRequest(w, r, fmt.Errorf("missing team slug"))
		return
	}

	if err := r.ParseForm(); err != nil {
		_ = response.BadRequest(w, r, fmt.Errorf("invalid form data"))
		return
	}

	relayState := r.FormValue("RelayState")
	samlResponse := r.FormValue("SAMLResponse")

	if samlResponse == "" || relayState == "" {
		_ = response.BadRequest(w, r, fmt.Errorf("missing SAMLResponse or RelayState"))
		return
	}

	user, redirectURL, err := h.ssoService.ProcessACS(r.Context(), r, teamSlug, relayState)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	// Generate JWT tokens (same pattern as OAuth callback)
	tokenPair, err := h.jwtService.GenerateTokenPair(r.Context(), user)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	// Redirect to the SSO callback page with tokens in URL fragment
	if redirectURL == "" {
		redirectURL = h.baseURL
	}

	callbackURL := fmt.Sprintf("%s/sso-callback#access_token=%s&refresh_token=%s&expires_in=%d",
		redirectURL,
		url.QueryEscape(tokenPair.AccessToken),
		url.QueryEscape(tokenPair.RefreshToken),
		tokenPair.ExpiresIn,
	)

	http.Redirect(w, r, callbackURL, http.StatusFound)
}
