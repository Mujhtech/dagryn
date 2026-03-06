package middleware

import (
	"fmt"
	"net/http"

	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// RequireFeature returns middleware that blocks requests when a feature is
// not enabled for the current entitlement context. The checker implementation
// determines how the decision is made:
//   - LicenseChecker: checks license claims
//   - BillingChecker (cloud): checks the billing plan
//   - NoopChecker (tests): always allows
//
// If checker is nil, the middleware passes through (safe default during
// server startup before entitlements are wired).
func RequireFeature(checker entitlement.Checker, feature string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if checker == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !checker.HasFeature(r.Context(), feature) {
				_ = response.Forbidden(w, r, fmt.Errorf(
					"%s is not available on your current plan. Visit https://dagryn.dev/pricing for details",
					feature,
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
