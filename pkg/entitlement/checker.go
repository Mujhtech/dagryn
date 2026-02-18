// Package entitlement defines the unified interface for feature and quota
// entitlement checks. The OSS binary uses LicenseChecker (license-backed),
// while the cloud binary uses a billing-backed implementation from the
// private dagryn-cloud repo.
package entitlement

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ProjectCreatedEvent carries context about a newly created project.
// Used by OnProjectCreated to link the project to a billing account (cloud)
// or no-op (self-hosted).
type ProjectCreatedEvent struct {
	ProjectID  uuid.UUID
	OwnerID    uuid.UUID
	OwnerEmail string
	OwnerName  string
	TeamID     *uuid.UUID
}

// Checker is the unified interface for feature/quota entitlement.
// The OSS binary uses a license-backed implementation.
// The cloud binary uses a billing-backed implementation.
type Checker interface {
	// HasFeature returns true if the feature is enabled for the current context.
	HasFeature(ctx context.Context, feature string) bool

	// CheckQuota returns nil if the operation is within quota, or a QuotaError.
	CheckQuota(ctx context.Context, resource string, projectID uuid.UUID, requested int64) error

	// RecordUsage records metered usage (bandwidth, storage, etc.).
	// Implementations may no-op if usage tracking is not applicable.
	RecordUsage(ctx context.Context, resource string, projectID uuid.UUID, amount int64)

	// OnProjectCreated is a lifecycle hook called after a project is created.
	// Cloud implementations link the project to a billing account;
	// self-hosted implementations no-op.
	OnProjectCreated(ctx context.Context, event ProjectCreatedEvent) error

	// Mode returns "cloud" or "self_hosted".
	Mode() string

	// Edition returns the current edition label (e.g. "community", "pro",
	// "enterprise", "cloud"). Used by the capabilities and health endpoints.
	Edition() string
}

// QuotaError is returned when a quota check fails.
type QuotaError struct {
	Resource  string
	Current   int64
	Limit     int64
	ProjectID uuid.UUID
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("%s quota exceeded (%d/%d)", e.Resource, e.Current, e.Limit)
}

// IsQuotaError returns true if err (or any wrapped error) is a *QuotaError.
func IsQuotaError(err error) bool {
	var qe *QuotaError
	return errors.As(err, &qe)
}
