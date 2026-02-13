package license

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNilGate(t *testing.T) {
	var gate *FeatureGate
	assert.Equal(t, EditionCommunity, gate.Edition())
	assert.False(t, gate.HasFeature(FeatureSSO))
	assert.Equal(t, communityLimits.seats, gate.Seats())
	assert.Nil(t, gate.Claims())
	assert.False(t, gate.IsExpiring())
	assert.False(t, gate.InGracePeriod())
}

func TestCommunityGate(t *testing.T) {
	gate := NewFeatureGate(nil, zerolog.Nop())
	assert.Equal(t, EditionCommunity, gate.Edition())
	assert.False(t, gate.HasFeature(FeatureSSO))
	assert.Equal(t, communityLimits.seats, gate.Seats())
	assert.Nil(t, gate.Claims())
}

func TestEnterpriseGate(t *testing.T) {
	claims := &Claims{
		LicenseID: "lic_test",
		Edition:   EditionEnterprise,
		Seats:     100,
		Features: []Feature{
			FeatureSSO,
			FeatureAuditLogs,
			FeatureContainerExecution,
			FeatureDashboardFull,
		},
		Limits: Limits{
			MaxProjects:    nil,
			MaxTeamMembers: nil,
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}

	gate := NewFeatureGate(claims, zerolog.Nop())

	assert.Equal(t, EditionEnterprise, gate.Edition())
	assert.True(t, gate.HasFeature(FeatureSSO))
	assert.True(t, gate.HasFeature(FeatureAuditLogs))
	assert.True(t, gate.HasFeature(FeatureContainerExecution))
	assert.False(t, gate.HasFeature(FeatureMultiCluster))
	assert.Equal(t, 100, gate.Seats())
	assert.NotNil(t, gate.Claims())
}

func TestCheckLimitCommunity(t *testing.T) {
	gate := NewFeatureGate(nil, zerolog.Nop())

	// Under limit
	assert.NoError(t, gate.CheckLimit("max_projects", 3))

	// At limit
	err := gate.CheckLimit("max_projects", 5)
	assert.Error(t, err)
	var le *LimitExceededError
	assert.ErrorAs(t, err, &le)
	assert.Equal(t, "max_projects", le.Resource)
	assert.Equal(t, int64(5), le.Current)
	assert.Equal(t, int64(5), le.Limit)
}

func TestCheckLimitUnlimited(t *testing.T) {
	claims := &Claims{
		Edition: EditionEnterprise,
		Limits: Limits{
			MaxProjects: nil, // unlimited
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := NewFeatureGate(claims, zerolog.Nop())

	assert.NoError(t, gate.CheckLimit("max_projects", 10000))
}

func TestCheckLimitWithCap(t *testing.T) {
	maxProj := 10
	claims := &Claims{
		Edition: EditionPro,
		Limits: Limits{
			MaxProjects: &maxProj,
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := NewFeatureGate(claims, zerolog.Nop())

	assert.NoError(t, gate.CheckLimit("max_projects", 9))
	assert.Error(t, gate.CheckLimit("max_projects", 10))
}

func TestGateRevoked(t *testing.T) {
	claims := &Claims{
		Edition:  EditionEnterprise,
		Seats:    50,
		Features: []Feature{FeatureSSO},
		Limits: Limits{
			MaxProjects: nil,
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := NewFeatureGate(claims, zerolog.Nop())

	assert.Equal(t, EditionEnterprise, gate.Edition())
	assert.True(t, gate.HasFeature(FeatureSSO))

	gate.MarkRevoked()

	assert.Equal(t, EditionCommunity, gate.Edition())
	assert.False(t, gate.HasFeature(FeatureSSO))
	assert.Equal(t, communityLimits.seats, gate.Seats())
}

func TestGateExpiring(t *testing.T) {
	claims := &Claims{
		Edition:   EditionPro,
		IssuedAt:  time.Now().Add(-335 * 24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(20 * 24 * time.Hour).Unix(), // 20 days left
		GraceDays: 14,
	}
	gate := NewFeatureGate(claims, zerolog.Nop())

	assert.True(t, gate.IsExpiring())
	assert.False(t, gate.InGracePeriod())
}

func TestGateGracePeriod(t *testing.T) {
	claims := &Claims{
		Edition:   EditionPro,
		IssuedAt:  time.Now().Add(-365 * 24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(-3 * 24 * time.Hour).Unix(), // expired 3 days ago
		GraceDays: 14,
	}
	gate := NewFeatureGate(claims, zerolog.Nop())

	assert.True(t, gate.InGracePeriod())
	// Still returns Pro during grace
	assert.Equal(t, EditionPro, gate.Edition())
}

func TestGateHardExpired(t *testing.T) {
	claims := &Claims{
		Edition:   EditionEnterprise,
		Seats:     50,
		Features:  []Feature{FeatureSSO},
		IssuedAt:  time.Now().Add(-400 * 24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(-30 * 24 * time.Hour).Unix(), // expired 30 days ago
		GraceDays: 14,                                          // grace ended 16 days ago
	}
	gate := NewFeatureGate(claims, zerolog.Nop())

	// Hard expired -> community
	assert.Equal(t, EditionCommunity, gate.Edition())
	assert.False(t, gate.HasFeature(FeatureSSO))
	assert.Equal(t, communityLimits.seats, gate.Seats())
}

func TestGateUpdate(t *testing.T) {
	claims1 := &Claims{
		Edition:   EditionPro,
		Seats:     10,
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := NewFeatureGate(claims1, zerolog.Nop())
	assert.Equal(t, EditionPro, gate.Edition())
	assert.Equal(t, 10, gate.Seats())

	claims2 := &Claims{
		Edition:   EditionEnterprise,
		Seats:     50,
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate.Update(claims2)

	assert.Equal(t, EditionEnterprise, gate.Edition())
	assert.Equal(t, 50, gate.Seats())
}

func TestGateUpdateClearsRevoked(t *testing.T) {
	claims := &Claims{
		Edition:   EditionPro,
		Features:  []Feature{FeatureSSO},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
	gate := NewFeatureGate(claims, zerolog.Nop())
	gate.MarkRevoked()
	assert.False(t, gate.HasFeature(FeatureSSO))

	gate.Update(claims)
	assert.True(t, gate.HasFeature(FeatureSSO))
}

func TestCheckUnknownResource(t *testing.T) {
	gate := NewFeatureGate(nil, zerolog.Nop())
	assert.NoError(t, gate.CheckLimit("unknown_resource", 99999))
}

func TestNilGateUpdate(t *testing.T) {
	var gate *FeatureGate
	// Should not panic
	gate.Update(nil)
	gate.MarkRevoked()
}
