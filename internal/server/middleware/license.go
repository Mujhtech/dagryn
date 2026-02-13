package middleware

import (
	"fmt"
	"net/http"

	"github.com/mujhtech/dagryn/internal/license"
	"github.com/mujhtech/dagryn/internal/server/response"
)

// RequireFeature returns middleware that blocks requests when a feature is not licensed.
// When gate is nil (cloud mode), the middleware passes through — cloud deployments use
// the billing/quota system instead of license gating.
func RequireFeature(gate *license.FeatureGate, feature license.Feature) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// nil gate = cloud mode; skip license checks
			if gate == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !gate.HasFeature(feature) {
				_ = response.Forbidden(w, r, fmt.Errorf(
					"%s requires a %s license. Visit https://dagryn.dev/pricing for details",
					feature, requiredEdition(feature),
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireEdition returns middleware that requires a minimum edition.
// When gate is nil (cloud mode), the middleware passes through.
func RequireEdition(gate *license.FeatureGate, minEdition license.Edition) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// nil gate = cloud mode; skip license checks
			if gate == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !editionAtLeast(gate.Edition(), minEdition) {
				_ = response.Forbidden(w, r, fmt.Errorf(
					"this feature requires a %s license", minEdition,
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// editionAtLeast returns true if current is at least minEdition.
// Order: community < pro < enterprise.
func editionAtLeast(current, min license.Edition) bool {
	return editionRank(current) >= editionRank(min)
}

func editionRank(e license.Edition) int {
	switch e {
	case license.EditionEnterprise:
		return 3
	case license.EditionPro:
		return 2
	case license.EditionCommunity:
		return 1
	default:
		return 0
	}
}

// requiredEdition returns the minimum edition for a given feature.
func requiredEdition(f license.Feature) license.Edition {
	switch f {
	case license.FeatureSSO, license.FeatureAuditLogs, license.FeatureCustomRBAC,
		license.FeatureMultiCluster, license.FeaturePriorityQueue:
		return license.EditionEnterprise
	default:
		return license.EditionPro
	}
}
