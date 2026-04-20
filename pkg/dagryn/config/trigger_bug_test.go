package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTriggerBug_SampleConfig verifies trigger matching with the exact config
// from dagryn.toml - triggers should filter branches when declared.
func TestTriggerBug_SampleConfig(t *testing.T) {
	toml := `
[workflow]
name = "ci"
default = true

[workflow.trigger]
[workflow.trigger.push]
branches = [
    "main",
    "design",
]
[workflow.trigger.pull_request]
branches = [
    "main",
    "design",
]
types = ["opened", "synchronize"]
[workflow.trigger.tag]
patterns = ["v*"]

[tasks.build]
command = "go build ./..."
`
	cfg, err := ParseBytes([]byte(toml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Workflow.Trigger)
	require.NotNil(t, cfg.Workflow.Trigger.Push)
	require.NotNil(t, cfg.Workflow.Trigger.PullRequest)
	require.NotNil(t, cfg.Workflow.Trigger.Tag)

	t.Log("Trigger.Push.Branches:", cfg.Workflow.Trigger.Push.Branches)
	t.Log("Trigger.PullRequest.Branches:", cfg.Workflow.Trigger.PullRequest.Branches)
	t.Log("Trigger.PullRequest.Types:", cfg.Workflow.Trigger.PullRequest.Types)
	t.Log("Trigger.Tag.Patterns:", cfg.Workflow.Trigger.Tag.Patterns)

	tc := cfg.Workflow.Trigger

	// Push: main and design should match
	assert.True(t, tc.MatchesPush("main"), "push to main should match")
	assert.True(t, tc.MatchesPush("design"), "push to design should match")
	assert.True(t, tc.MatchesPush("refs/heads/main"), "push to refs/heads/main should match")
	assert.True(t, tc.MatchesPush("refs/heads/design"), "push to refs/heads/design should match")

	// Push: other branches should NOT match
	assert.False(t, tc.MatchesPush("feature/something"), "push to feature branch should NOT match")
	assert.False(t, tc.MatchesPush("refs/heads/feature/something"), "push to refs/heads/feature should NOT match")
	assert.False(t, tc.MatchesPush("develop"), "push to develop should NOT match")

	// PR: main/design base with opened/synchronize should match
	assert.True(t, tc.MatchesPullRequest("main", "opened"), "PR to main with opened should match")
	assert.True(t, tc.MatchesPullRequest("main", "synchronize"), "PR to main with synchronize should match")
	assert.True(t, tc.MatchesPullRequest("design", "opened"), "PR to design with opened should match")

	// PR: other base branches should NOT match
	assert.False(t, tc.MatchesPullRequest("develop", "opened"), "PR to develop should NOT match")
	assert.False(t, tc.MatchesPullRequest("feature/x", "opened"), "PR to feature should NOT match")

	// PR: wrong action types should NOT match
	assert.False(t, tc.MatchesPullRequest("main", "closed"), "PR closed should NOT match")
	assert.False(t, tc.MatchesPullRequest("main", "reopened"), "PR reopened should NOT match")

	// Tags: matching patterns
	assert.True(t, tc.MatchesPush("refs/tags/v1.0.0"), "tag v1.0.0 should match")
	assert.True(t, tc.MatchesPush("refs/tags/v2.3.4"), "tag v2.3.4 should match")

	// Tags: non-matching patterns
	assert.False(t, tc.MatchesPush("refs/tags/release-1"), "tag release-1 should NOT match")
}

// TestTriggerBug_NoTriggerDeclared verifies all events match when trigger is not declared.
func TestTriggerBug_NoTriggerDeclared(t *testing.T) {
	toml := `
[workflow]
name = "ci"
default = true

[tasks.build]
command = "go build ./..."
`
	cfg, err := ParseBytes([]byte(toml))
	require.NoError(t, err)
	assert.Nil(t, cfg.Workflow.Trigger, "trigger should be nil when not declared")

	// A nil trigger config means MatchesPush/MatchesPullRequest/MatchesTag should all return true
	var tc *TriggerConfig
	assert.True(t, tc.MatchesPush("any-branch"))
	assert.True(t, tc.MatchesPullRequest("any-branch", "any-action"))
	assert.True(t, tc.MatchesTag("any-tag"))
}

// TestTriggerBug_EmptyTriggerSection verifies behavior with declared but empty trigger section.
func TestTriggerBug_EmptyTriggerSection(t *testing.T) {
	// This simulates [workflow.trigger] with no sub-sections
	toml := `
[workflow]
name = "ci"

[workflow.trigger]

[tasks.build]
command = "go build ./..."
`
	cfg, err := ParseBytes([]byte(toml))
	require.NoError(t, err)
	t.Logf("Trigger is nil: %v", cfg.Workflow.Trigger == nil)
	if cfg.Workflow.Trigger != nil {
		t.Logf("Push is nil: %v", cfg.Workflow.Trigger.Push == nil)
		t.Logf("PullRequest is nil: %v", cfg.Workflow.Trigger.PullRequest == nil)
		t.Logf("Tag is nil: %v", cfg.Workflow.Trigger.Tag == nil)
	}
}

// TestTriggerBug_PushOnlyTrigger verifies behavior when only push trigger is declared.
func TestTriggerBug_PushOnlyTrigger(t *testing.T) {
	toml := `
[workflow]
name = "ci"

[workflow.trigger.push]
branches = ["main"]

[tasks.build]
command = "go build ./..."
`
	cfg, err := ParseBytes([]byte(toml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Workflow.Trigger)
	require.NotNil(t, cfg.Workflow.Trigger.Push)
	assert.Nil(t, cfg.Workflow.Trigger.PullRequest, "PR should be nil when not declared")
	assert.Nil(t, cfg.Workflow.Trigger.Tag, "Tag should be nil when not declared")

	tc := cfg.Workflow.Trigger

	// Push to main should match
	assert.True(t, tc.MatchesPush("main"))
	// Push to other branches should NOT match
	assert.False(t, tc.MatchesPush("develop"))

	// PR matching with nil PR config — currently returns true (match all)
	// This is the potential semantic issue: should it allow PRs or not?
	t.Logf("MatchesPullRequest with nil PullRequest config: %v", tc.MatchesPullRequest("main", "opened"))
}
