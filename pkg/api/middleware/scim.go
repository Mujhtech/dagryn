package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"golang.org/x/crypto/bcrypt"
)

// SCIMAuth returns middleware that authenticates SCIM requests using Bearer token.
// It extracts the teamSlug from the URL, looks up the team and SSO connection,
// and validates the token against the stored scim_token_hash.
func SCIMAuth(ssoRepo repo.SSOStore, teamRepo repo.TeamStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			teamSlug := chi.URLParam(r, "teamSlug")
			if teamSlug == "" {
				writeSCIMError(w, http.StatusBadRequest, "missing team slug")
				return
			}

			// Look up the team
			team, err := teamRepo.GetBySlug(r.Context(), teamSlug)
			if err != nil {
				writeSCIMError(w, http.StatusNotFound, "team not found")
				return
			}

			// Look up the SSO connection
			conn, err := ssoRepo.GetConnectionByTeamID(r.Context(), team.ID)
			if err != nil {
				writeSCIMError(w, http.StatusNotFound, "SSO not configured for this team")
				return
			}

			if !conn.SCIMEnabled {
				writeSCIMError(w, http.StatusForbidden, "SCIM is not enabled for this team")
				return
			}

			if conn.SCIMTokenHash == nil || *conn.SCIMTokenHash == "" {
				writeSCIMError(w, http.StatusForbidden, "SCIM token not configured")
				return
			}

			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeSCIMError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")

			// Validate token against stored hash
			if err := bcrypt.CompareHashAndPassword([]byte(*conn.SCIMTokenHash), []byte(token)); err != nil {
				writeSCIMError(w, http.StatusUnauthorized, "invalid SCIM token")
				return
			}

			// Set team in context
			ctx := apiCtx.WithTeam(r.Context(), team)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		"detail":  detail,
		"status":  status,
	}
	data, _ := json.Marshal(resp)
	_, _ = w.Write(data)
}
