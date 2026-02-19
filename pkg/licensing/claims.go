package licensing

import "time"

// Edition represents the product tier.
type Edition string

const (
	EditionCommunity  Edition = "community"
	EditionPro        Edition = "pro"
	EditionEnterprise Edition = "enterprise"
)

// Feature is a named feature flag embedded in the license.
type Feature string

const (
	FeatureContainerExecution Feature = "container_execution"
	FeaturePriorityQueue      Feature = "priority_queue"
	FeatureSSO                Feature = "sso"
	FeatureAuditLogs          Feature = "audit_logs"
	FeatureCustomRBAC         Feature = "custom_rbac"
	FeatureMultiCluster       Feature = "multi_cluster"
	FeatureDashboardFull      Feature = "dashboard_full"
	FeatureCloudCache         Feature = "cloud_cache"
	FeatureArtifactRetention  Feature = "artifact_retention"
	FeatureLogRetention       Feature = "log_retention"
	FeatureStorage            Feature = "storage"
	FeatureCacheTTL           Feature = "cache_ttl"
	FeatureSaml               Feature = "saml"
	FeatureAIAnalysis         Feature = "ai_analysis"
	FeatureAISuggestions      Feature = "ai_suggestions"
)

// Limits holds numeric resource limits. Nil means unlimited.
type Limits struct {
	MaxProjects           *int   `json:"max_projects"`
	MaxTeamMembers        *int   `json:"max_team_members"`
	MaxConcurrentRuns     *int   `json:"max_concurrent_runs"`
	MaxArtifactBytes      *int64 `json:"max_artifact_bytes"`
	MaxArtifactUploadSize *int64 `json:"max_artifact_upload_size"`
	LogRetentionDays      *int   `json:"log_retention_days"`
	ArtifactRetentionDays *int   `json:"artifact_retention_days"`
}

// Claims is the signed payload inside a license key.
type Claims struct {
	LicenseID string    `json:"lid"`
	KeyID     string    `json:"kid"`
	Issuer    string    `json:"iss"`
	Subject   string    `json:"sub"`
	Email     string    `json:"email"`
	Edition   Edition   `json:"edition"`
	Seats     int       `json:"seats"`
	Features  []Feature `json:"features"`
	Limits    Limits    `json:"limits"`
	IssuedAt  int64     `json:"iat"`
	ExpiresAt int64     `json:"exp"`
	GraceDays int       `json:"grace_days"`
}

// ExpiryTime returns the expiry as a time.Time.
func (c *Claims) ExpiryTime() time.Time {
	return time.Unix(c.ExpiresAt, 0)
}

// GraceDeadline returns the hard expiry (expiry + grace period).
func (c *Claims) GraceDeadline() time.Time {
	return c.ExpiryTime().Add(time.Duration(c.GraceDays) * 24 * time.Hour)
}

// IsExpired returns true if past the expiry date (but may still be in grace).
func (c *Claims) IsExpired() bool {
	return time.Now().After(c.ExpiryTime())
}

// IsHardExpired returns true if past the grace deadline.
func (c *Claims) IsHardExpired() bool {
	return time.Now().After(c.GraceDeadline())
}

// InGracePeriod returns true if expired but still within the grace window.
func (c *Claims) InGracePeriod() bool {
	return c.IsExpired() && !c.IsHardExpired()
}

// DaysUntilExpiry returns days until expiry (negative if expired).
func (c *Claims) DaysUntilExpiry() int {
	return int(time.Until(c.ExpiryTime()).Hours() / 24)
}
