package entitlement

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/licensing"
)

// LicenseChecker implements Checker using the license-based FeatureGate.
// This is the default implementation for the OSS/self-hosted binary.
type LicenseChecker struct {
	gate *licensing.FeatureGate
}

// NewLicenseChecker creates a Checker backed by a license FeatureGate.
// A nil gate behaves as Community edition (all features disabled, default limits).
func NewLicenseChecker(gate *licensing.FeatureGate) *LicenseChecker {
	return &LicenseChecker{gate: gate}
}

// HasFeature returns true if the feature is enabled in the license.
func (c *LicenseChecker) HasFeature(_ context.Context, feature string) bool {
	return c.gate.HasFeature(licensing.Feature(feature))
}

// CheckQuota checks the resource limit from the license claims.
// The requested parameter is treated as the current count (matching
// FeatureGate.CheckLimit semantics where current >= limit triggers error).
// Returns *QuotaError on limit exceeded so callers can use IsQuotaError.
func (c *LicenseChecker) CheckQuota(_ context.Context, resource string, _ uuid.UUID, requested int64) error {
	err := c.gate.CheckLimit(resource, requested)
	if err == nil {
		return nil
	}
	var le *licensing.LimitExceededError
	if errors.As(err, &le) {
		return &QuotaError{
			Resource: le.Resource,
			Current:  le.Current,
			Limit:    le.Limit,
		}
	}
	return err
}

// RecordUsage is a no-op for self-hosted — there is no metered billing.
func (c *LicenseChecker) RecordUsage(_ context.Context, _ string, _ uuid.UUID, _ int64) {
	// No-op: self-hosted has no metered billing
}

// OnProjectCreated is a no-op for self-hosted — no billing linkage needed.
func (c *LicenseChecker) OnProjectCreated(_ context.Context, _ ProjectCreatedEvent) error {
	return nil
}

// Mode returns "self_hosted".
func (c *LicenseChecker) Mode() string {
	return "self_hosted"
}

// Edition returns the license edition (e.g. "community", "pro", "enterprise").
func (c *LicenseChecker) Edition() string {
	return string(c.gate.Edition())
}

// Gate returns the underlying FeatureGate for license-specific operations
// (e.g., reading claims, checking expiry) that are not part of the
// generic Checker interface.
func (c *LicenseChecker) Gate() *licensing.FeatureGate {
	return c.gate
}
