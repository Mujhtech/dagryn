package licensing

// communityLimits defines limits for the free Community edition.
var communityLimits = struct {
	seats                 int
	maxProjects           int
	maxTeamMembers        int
	maxConcurrentRuns     int
	maxArtifactBytes      int64
	maxArtifactUploadSize int64
	logRetentionDays      int
}{
	seats:                 3,
	maxProjects:           5,
	maxTeamMembers:        3,
	maxConcurrentRuns:     3,
	maxArtifactBytes:      1 * 1024 * 1024 * 1024, // 1 GB
	maxArtifactUploadSize: 200 * 1024 * 1024,       // 200 MB per file
	logRetentionDays:      7,
}

// CommunityLogRetentionDays returns the default log retention for Community edition.
func CommunityLogRetentionDays() int {
	return communityLimits.logRetentionDays
}
