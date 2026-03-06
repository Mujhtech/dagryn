package entitlement

import (
	"context"

	"github.com/google/uuid"
)

// NoopChecker is a test double that allows everything.
// Use it in tests where entitlement enforcement is not under test.
type NoopChecker struct {
	// ModeValue controls the return value of Mode(). Defaults to "test".
	ModeValue string
	// EditionValue controls the return value of Edition(). Defaults to "test".
	EditionValue string
}

// NewNoopChecker returns a Checker that permits all features and quotas.
func NewNoopChecker() *NoopChecker {
	return &NoopChecker{ModeValue: "test"}
}

// HasFeature always returns true.
func (c *NoopChecker) HasFeature(_ context.Context, _ string) bool {
	return true
}

// CheckQuota always returns nil (within quota).
func (c *NoopChecker) CheckQuota(_ context.Context, _ string, _ uuid.UUID, _ int64) error {
	return nil
}

// RecordUsage is a no-op.
func (c *NoopChecker) RecordUsage(_ context.Context, _ string, _ uuid.UUID, _ int64) {}

// OnProjectCreated is a no-op.
func (c *NoopChecker) OnProjectCreated(_ context.Context, _ ProjectCreatedEvent) error {
	return nil
}

// Mode returns c.ModeValue (defaults to "test").
func (c *NoopChecker) Mode() string {
	if c.ModeValue == "" {
		return "test"
	}
	return c.ModeValue
}

// Edition returns c.EditionValue (defaults to "test").
func (c *NoopChecker) Edition() string {
	if c.EditionValue == "" {
		return "test"
	}
	return c.EditionValue
}
