package middleware

import (
	"net/http"

	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// EnforceSSO returns middleware that blocks non-SAML authenticated users
// when SSO enforcement is enabled for a team.
func EnforceSSO(ssoRepo repo.SSOStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := apiCtx.GetUser(r.Context())
			if user == nil {
				next.ServeHTTP(w, r)
				return
			}

			team := apiCtx.GetTeam(r.Context())
			if team == nil {
				next.ServeHTTP(w, r)
				return
			}

			conn, err := ssoRepo.GetConnectionByTeamID(r.Context(), team.ID)
			if err != nil {
				// No SSO connection or error — allow through
				next.ServeHTTP(w, r)
				return
			}

			if conn.EnforceSSO && user.Provider != string(models.AuthProviderSAML) {
				_ = response.Forbidden(w, r, errSSORequired)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

var errSSORequired = &ssoRequiredError{}

type ssoRequiredError struct{}

func (e *ssoRequiredError) Error() string {
	return "SSO authentication is required for this team. Please sign in via your identity provider."
}
