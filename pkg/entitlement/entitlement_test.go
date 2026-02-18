package entitlement_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/rs/zerolog"
)

// --- Interface compliance ---

var _ entitlement.Checker = (*entitlement.LicenseChecker)(nil)
var _ entitlement.Checker = (*entitlement.NoopChecker)(nil)

// --- LicenseChecker tests ---

func TestLicenseChecker_Mode(t *testing.T) {
	gate := licensing.NewFeatureGate(nil, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	if mode := checker.Mode(); mode != "self_hosted" {
		t.Errorf("expected mode 'self_hosted', got %q", mode)
	}
}

func TestLicenseChecker_HasFeature_Community(t *testing.T) {
	gate := licensing.NewFeatureGate(nil, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	ctx := context.Background()

	// Community edition has no features
	if checker.HasFeature(ctx, "container_execution") {
		t.Error("expected community edition to not have container_execution")
	}
	if checker.HasFeature(ctx, "sso") {
		t.Error("expected community edition to not have sso")
	}
}

func TestLicenseChecker_HasFeature_Enterprise(t *testing.T) {
	claims := &licensing.Claims{
		LicenseID: "test-001",
		KeyID:     "dev",
		Issuer:    "Dagryn Inc.",
		Subject:   "Test Corp",
		Email:     "test@example.com",
		Edition:   licensing.EditionEnterprise,
		Seats:     50,
		Features: []licensing.Feature{
			licensing.FeatureContainerExecution,
			licensing.FeatureSSO,
			licensing.FeatureAuditLogs,
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := licensing.NewFeatureGate(claims, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	ctx := context.Background()

	if !checker.HasFeature(ctx, "container_execution") {
		t.Error("expected enterprise to have container_execution")
	}
	if !checker.HasFeature(ctx, "sso") {
		t.Error("expected enterprise to have sso")
	}
	if checker.HasFeature(ctx, "priority_queue") {
		t.Error("expected enterprise to not have unlicensed priority_queue")
	}
}

func TestLicenseChecker_CheckQuota_CommunityLimits(t *testing.T) {
	gate := licensing.NewFeatureGate(nil, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	ctx := context.Background()
	projectID := uuid.New()

	// Community: max_projects = 5, so 4 should pass, 5 should fail
	if err := checker.CheckQuota(ctx, "max_projects", projectID, 4); err != nil {
		t.Errorf("expected under-limit to pass, got: %v", err)
	}
	if err := checker.CheckQuota(ctx, "max_projects", projectID, 5); err == nil {
		t.Error("expected at-limit to fail")
	}
}

func TestLicenseChecker_CheckQuota_UnlimitedLicense(t *testing.T) {
	claims := &licensing.Claims{
		LicenseID: "test-002",
		KeyID:     "dev",
		Issuer:    "Dagryn Inc.",
		Subject:   "Unlimited Corp",
		Edition:   licensing.EditionEnterprise,
		Seats:     100,
		Limits:    licensing.Limits{}, // All nil = unlimited
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := licensing.NewFeatureGate(claims, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	ctx := context.Background()

	// nil limits = unlimited
	if err := checker.CheckQuota(ctx, "max_projects", uuid.New(), 10000); err != nil {
		t.Errorf("expected unlimited to pass, got: %v", err)
	}
}

func TestLicenseChecker_CheckQuota_WithLimits(t *testing.T) {
	maxProjects := 25
	claims := &licensing.Claims{
		LicenseID: "test-003",
		KeyID:     "dev",
		Issuer:    "Dagryn Inc.",
		Subject:   "Limited Corp",
		Edition:   licensing.EditionPro,
		Seats:     10,
		Limits: licensing.Limits{
			MaxProjects: &maxProjects,
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := licensing.NewFeatureGate(claims, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	ctx := context.Background()

	if err := checker.CheckQuota(ctx, "max_projects", uuid.New(), 24); err != nil {
		t.Errorf("expected under-limit to pass, got: %v", err)
	}
	if err := checker.CheckQuota(ctx, "max_projects", uuid.New(), 25); err == nil {
		t.Error("expected at-limit to fail")
	}
}

func TestLicenseChecker_RecordUsage_NoOp(t *testing.T) {
	gate := licensing.NewFeatureGate(nil, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	// Should not panic
	checker.RecordUsage(context.Background(), "bandwidth", uuid.New(), 1024)
}

func TestLicenseChecker_Edition_Community(t *testing.T) {
	gate := licensing.NewFeatureGate(nil, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	if ed := checker.Edition(); ed != "community" {
		t.Errorf("expected edition 'community', got %q", ed)
	}
}

func TestLicenseChecker_Edition_Enterprise(t *testing.T) {
	claims := &licensing.Claims{
		LicenseID: "test-ed",
		KeyID:     "dev",
		Issuer:    "Dagryn Inc.",
		Subject:   "Test Corp",
		Edition:   licensing.EditionEnterprise,
		Seats:     50,
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := licensing.NewFeatureGate(claims, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	if ed := checker.Edition(); ed != "enterprise" {
		t.Errorf("expected edition 'enterprise', got %q", ed)
	}
}

func TestLicenseChecker_Gate(t *testing.T) {
	gate := licensing.NewFeatureGate(nil, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	if checker.Gate() != gate {
		t.Error("expected Gate() to return the underlying FeatureGate")
	}
}

// --- NoopChecker tests ---

func TestNoopChecker_AllowsEverything(t *testing.T) {
	checker := entitlement.NewNoopChecker()
	ctx := context.Background()

	if !checker.HasFeature(ctx, "anything") {
		t.Error("expected NoopChecker to allow all features")
	}
	if err := checker.CheckQuota(ctx, "any_resource", uuid.New(), 999999); err != nil {
		t.Errorf("expected NoopChecker to allow all quotas, got: %v", err)
	}
}

func TestNoopChecker_Mode(t *testing.T) {
	checker := entitlement.NewNoopChecker()
	if mode := checker.Mode(); mode != "test" {
		t.Errorf("expected mode 'test', got %q", mode)
	}
}

func TestNoopChecker_CustomMode(t *testing.T) {
	checker := &entitlement.NoopChecker{ModeValue: "cloud"}
	if mode := checker.Mode(); mode != "cloud" {
		t.Errorf("expected mode 'cloud', got %q", mode)
	}
}

func TestNoopChecker_RecordUsage_NoOp(t *testing.T) {
	checker := entitlement.NewNoopChecker()
	// Should not panic
	checker.RecordUsage(context.Background(), "bandwidth", uuid.New(), 1024)
}

// --- QuotaError tests ---

func TestQuotaError_Message(t *testing.T) {
	err := &entitlement.QuotaError{
		Resource: "storage",
		Current:  500,
		Limit:    100,
	}
	expected := "storage quota exceeded (500/100)"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestIsQuotaError(t *testing.T) {
	qe := &entitlement.QuotaError{Resource: "storage", Current: 10, Limit: 5}
	if !entitlement.IsQuotaError(qe) {
		t.Error("expected IsQuotaError to return true for *QuotaError")
	}
	if entitlement.IsQuotaError(nil) {
		t.Error("expected IsQuotaError to return false for nil")
	}
	if entitlement.IsQuotaError(context.DeadlineExceeded) {
		t.Error("expected IsQuotaError to return false for non-QuotaError")
	}
}

func TestLicenseChecker_CheckQuota_ReturnsQuotaError(t *testing.T) {
	gate := licensing.NewFeatureGate(nil, zerolog.Nop())
	checker := entitlement.NewLicenseChecker(gate)
	ctx := context.Background()

	// Community edition: max_projects = 5, so 5 should fail with *QuotaError
	err := checker.CheckQuota(ctx, "max_projects", uuid.New(), 5)
	if err == nil {
		t.Fatal("expected error for at-limit check")
	}
	if !entitlement.IsQuotaError(err) {
		t.Errorf("expected *QuotaError, got %T: %v", err, err)
	}
}
