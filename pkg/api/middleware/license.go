package middleware

import (
	"fmt"
	"net/http"

	"github.com/mujhtech/dagryn/pkg/http/response"
	"github.com/mujhtech/dagryn/pkg/licensing"
)

// RequireFeature returns middleware that blocks requests when a feature is not licensed.
// When gate is nil (cloud mode), the middleware passes through — cloud deployments use
// the billing/quota system instead of license gating.
func RequireFeature(gate *licensing.FeatureGate, feature licensing.Feature) func(http.Handler) http.Handler {
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
func RequireEdition(gate *licensing.FeatureGate, minEdition licensing.Edition) func(http.Handler) http.Handler {
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
func editionAtLeast(current, min licensing.Edition) bool {
	return editionRank(current) >= editionRank(min)
}

func editionRank(e licensing.Edition) int {
	switch e {
	case licensing.EditionEnterprise:
		return 3
	case licensing.EditionPro:
		return 2
	case licensing.EditionCommunity:
		return 1
	default:
		return 0
	}
}

// requiredEdition returns the minimum edition for a given feature.
func requiredEdition(f licensing.Feature) licensing.Edition {
	switch f {
	case licensing.FeatureSSO, licensing.FeatureAuditLogs, licensing.FeatureCustomRBAC,
		licensing.FeatureMultiCluster, licensing.FeaturePriorityQueue:
		return licensing.EditionEnterprise
	default:
		return licensing.EditionPro
	}
}
