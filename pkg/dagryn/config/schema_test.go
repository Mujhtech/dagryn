package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- TriggerConfig matching tests ---

func TestTriggerConfig_MatchesPush_NilTrigger(t *testing.T) {
	var tc *TriggerConfig
	assert.True(t, tc.MatchesPush("any-branch"))
}

func TestTriggerConfig_MatchesPush_NilPush(t *testing.T) {
	tc := &TriggerConfig{Push: nil}
	assert.True(t, tc.MatchesPush("any-branch"))
}

func TestTriggerConfig_MatchesPush_EmptyBranches(t *testing.T) {
	tc := &TriggerConfig{Push: &PushTriggerConfig{Branches: []string{}}}
	assert.True(t, tc.MatchesPush("any-branch"))
}

func TestTriggerConfig_MatchesPush_BranchMatch(t *testing.T) {
	tc := &TriggerConfig{
		Push: &PushTriggerConfig{Branches: []string{"main", "develop"}},
	}
	assert.True(t, tc.MatchesPush("main"))
	assert.True(t, tc.MatchesPush("develop"))
}

func TestTriggerConfig_MatchesPush_BranchNoMatch(t *testing.T) {
	tc := &TriggerConfig{
		Push: &PushTriggerConfig{Branches: []string{"main"}},
	}
	assert.False(t, tc.MatchesPush("feature/foo"))
	assert.False(t, tc.MatchesPush("develop"))
}

func TestTriggerConfig_MatchesPullRequest_NilTrigger(t *testing.T) {
	var tc *TriggerConfig
	assert.True(t, tc.MatchesPullRequest("main", "opened"))
}

func TestTriggerConfig_MatchesPullRequest_NilPR(t *testing.T) {
	tc := &TriggerConfig{PullRequest: nil}
	assert.True(t, tc.MatchesPullRequest("main", "opened"))
}

func TestTriggerConfig_MatchesPullRequest_BranchAndTypeMatch(t *testing.T) {
	tc := &TriggerConfig{
		PullRequest: &PullRequestTriggerConfig{
			Branches: []string{"main"},
			Types:    []string{"opened", "synchronize"},
		},
	}
	assert.True(t, tc.MatchesPullRequest("main", "opened"))
	assert.True(t, tc.MatchesPullRequest("main", "synchronize"))
}

func TestTriggerConfig_MatchesPullRequest_BranchNoMatch(t *testing.T) {
	tc := &TriggerConfig{
		PullRequest: &PullRequestTriggerConfig{
			Branches: []string{"main"},
			Types:    []string{"opened"},
		},
	}
	assert.False(t, tc.MatchesPullRequest("develop", "opened"))
}

func TestTriggerConfig_MatchesPullRequest_TypeNoMatch(t *testing.T) {
	tc := &TriggerConfig{
		PullRequest: &PullRequestTriggerConfig{
			Branches: []string{"main"},
			Types:    []string{"opened"},
		},
	}
	assert.False(t, tc.MatchesPullRequest("main", "closed"))
}

func TestTriggerConfig_MatchesPullRequest_EmptyBranches(t *testing.T) {
	tc := &TriggerConfig{
		PullRequest: &PullRequestTriggerConfig{
			Types: []string{"opened"},
		},
	}
	// No branch filter, any branch matches
	assert.True(t, tc.MatchesPullRequest("any-branch", "opened"))
}

func TestTriggerConfig_MatchesPullRequest_EmptyTypes(t *testing.T) {
	tc := &TriggerConfig{
		PullRequest: &PullRequestTriggerConfig{
			Branches: []string{"main"},
		},
	}
	// No type filter, any action matches
	assert.True(t, tc.MatchesPullRequest("main", "any-action"))
}
