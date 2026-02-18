package licensing

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
)

// LimitExceededError is returned when a resource limit is exceeded.
type LimitExceededError struct {
	Resource string
	Current  int64
	Limit    int64
}

func (e *LimitExceededError) Error() string {
	return fmt.Sprintf("%s limit exceeded (%d/%d)", e.Resource, e.Current, e.Limit)
}

// FeatureGate provides the runtime API for checking licensed features.
// It is safe for concurrent use. A nil FeatureGate behaves as Community edition.
type FeatureGate struct {
	mu      sync.RWMutex
	claims  *Claims
	revoked bool
	logger  zerolog.Logger
}

// NewFeatureGate creates a gate from validated claims.
// Pass nil claims for Community edition.
func NewFeatureGate(claims *Claims, logger zerolog.Logger) *FeatureGate {
	return &FeatureGate{claims: claims, logger: logger}
}

// Edition returns the current edition.
func (g *FeatureGate) Edition() Edition {
	if g == nil || g.claims == nil {
		return EditionCommunity
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.revoked || g.claims.IsHardExpired() {
		return EditionCommunity
	}
	return g.claims.Edition
}

// HasFeature checks if a specific feature is licensed.
func (g *FeatureGate) HasFeature(f Feature) bool {
	if g == nil || g.claims == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.revoked || g.claims.IsHardExpired() {
		return false
	}
	for _, licensed := range g.claims.Features {
		if licensed == f {
			return true
		}
	}
	return false
}

// CheckLimit returns nil if the current value is within the licensed limit,
// or a LimitExceededError if over.
func (g *FeatureGate) CheckLimit(resource string, current int64) error {
	if g == nil || g.claims == nil {
		return checkCommunityLimit(resource, current)
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.revoked || g.claims.IsHardExpired() {
		return checkCommunityLimit(resource, current)
	}
	return checkClaimsLimit(&g.claims.Limits, resource, current)
}

// Seats returns the licensed seat count.
func (g *FeatureGate) Seats() int {
	if g == nil || g.claims == nil {
		return communityLimits.seats
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.revoked || g.claims.IsHardExpired() {
		return communityLimits.seats
	}
	return g.claims.Seats
}

// Claims returns the raw claims (nil for Community).
func (g *FeatureGate) Claims() *Claims {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.claims
}

// IsExpiring returns true if the license expires within 30 days.
func (g *FeatureGate) IsExpiring() bool {
	if g == nil || g.claims == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	days := g.claims.DaysUntilExpiry()
	return days >= 0 && days <= 30
}

// InGracePeriod returns true if the license is expired but in the grace window.
func (g *FeatureGate) InGracePeriod() bool {
	if g == nil || g.claims == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.claims.InGracePeriod()
}

// Update replaces the current claims (used for hot-reload).
func (g *FeatureGate) Update(claims *Claims) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.claims = claims
	g.revoked = false
}

// MarkRevoked immediately downgrades runtime behavior to Community edition.
func (g *FeatureGate) MarkRevoked() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.revoked = true
}

// checkCommunityLimit checks a resource against Community edition limits.
func checkCommunityLimit(resource string, current int64) error {
	var limit int64
	switch resource {
	case "max_projects":
		limit = int64(communityLimits.maxProjects)
	case "max_team_members":
		limit = int64(communityLimits.maxTeamMembers)
	case "max_concurrent_runs":
		limit = int64(communityLimits.maxConcurrentRuns)
	case "max_artifact_bytes":
		limit = communityLimits.maxArtifactBytes
	case "max_artifact_upload_size":
		limit = communityLimits.maxArtifactUploadSize
	default:
		return nil // unknown resource, no limit
	}
	if current >= limit {
		return &LimitExceededError{Resource: resource, Current: current, Limit: limit}
	}
	return nil
}

// checkClaimsLimit checks a resource against the license claims limits.
// Nil limit means unlimited.
func checkClaimsLimit(limits *Limits, resource string, current int64) error {
	var limitPtr *int64
	switch resource {
	case "max_projects":
		if limits.MaxProjects != nil {
			v := int64(*limits.MaxProjects)
			limitPtr = &v
		}
	case "max_team_members":
		if limits.MaxTeamMembers != nil {
			v := int64(*limits.MaxTeamMembers)
			limitPtr = &v
		}
	case "max_concurrent_runs":
		if limits.MaxConcurrentRuns != nil {
			v := int64(*limits.MaxConcurrentRuns)
			limitPtr = &v
		}
	case "max_artifact_bytes":
		limitPtr = limits.MaxArtifactBytes
	case "max_artifact_upload_size":
		limitPtr = limits.MaxArtifactUploadSize
	default:
		return nil // unknown resource, no limit
	}

	// nil means unlimited
	if limitPtr == nil {
		return nil
	}
	if current >= *limitPtr {
		return &LimitExceededError{Resource: resource, Current: current, Limit: *limitPtr}
	}
	return nil
}
