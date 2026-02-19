package cliui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.0", "v1.1.0", true},
		{"v1.1.0", "v1.2.0", false},
		{"v1.1.1", "v1.1.0", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.0", "v1.0.0", false},
		{"1.2.0", "1.1.0", true},          // no v prefix
		{"v1.2.0-rc1", "v1.1.0", true},    // pre-release stripped
		{"v0.0.1-dirty", "v0.0.1", false}, // same after strip
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, isNewer(tt.latest, tt.current),
			"isNewer(%q, %q)", tt.latest, tt.current)
	}
}

func TestIsNewerInvalidInput(t *testing.T) {
	assert.False(t, isNewer("not-a-version", "v1.0.0"))
	assert.False(t, isNewer("v1.0.0", "not-a-version"))
	assert.False(t, isNewer("", ""))
}

func TestParseSemver(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3}, parseSemver("v1.2.3"))
	assert.Equal(t, []int{0, 1, 0}, parseSemver("0.1.0"))
	assert.Equal(t, []int{1, 0, 0}, parseSemver("v1.0.0-dirty"))
	assert.Nil(t, parseSemver("abc"))
	assert.Nil(t, parseSemver(""))
}

func TestUpdateCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	uc := NewUpdateChecker("v1.0.0", "mujhtech/dagryn", dir)

	c := &updateCache{
		LastChecked:   time.Now().Truncate(time.Second),
		LatestVersion: "v1.1.0",
	}
	require.NoError(t, uc.writeCache(c))

	got, err := uc.readCache()
	require.NoError(t, err)
	assert.Equal(t, c.LatestVersion, got.LatestVersion)
	assert.WithinDuration(t, c.LastChecked, got.LastChecked, time.Second)
}

func TestUpdateCacheCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	uc := NewUpdateChecker("v1.0.0", "mujhtech/dagryn", dir)
	require.NoError(t, uc.writeCache(&updateCache{
		LastChecked:   time.Now(),
		LatestVersion: "v1.1.0",
	}))
	_, err := os.Stat(filepath.Join(dir, "update-check.json"))
	assert.NoError(t, err)
}

func TestCheckInBackgroundSkipsDev(t *testing.T) {
	uc := NewUpdateChecker("dev", "mujhtech/dagryn", t.TempDir())
	uc.CheckInBackground()
	result := uc.CollectResult(time.Second)
	assert.Nil(t, result)
}

func TestCheckInBackgroundUsesCache(t *testing.T) {
	dir := t.TempDir()
	// Pre-populate cache with a recent check showing v2.0.0
	c := updateCache{
		LastChecked:   time.Now(),
		LatestVersion: "v2.0.0",
	}
	data, _ := json.Marshal(c)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "update-check.json"), data, 0o644))

	uc := NewUpdateChecker("v1.0.0", "mujhtech/dagryn", dir)
	uc.CheckInBackground()
	result := uc.CollectResult(time.Second)
	require.NotNil(t, result)
	assert.True(t, result.UpdateAvail)
	assert.Equal(t, "v2.0.0", result.LatestVersion)
}

func TestCheckInBackgroundCacheNotStale(t *testing.T) {
	dir := t.TempDir()
	// Cache says we're on the latest — no update
	c := updateCache{
		LastChecked:   time.Now(),
		LatestVersion: "v1.0.0",
	}
	data, _ := json.Marshal(c)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "update-check.json"), data, 0o644))

	uc := NewUpdateChecker("v1.0.0", "mujhtech/dagryn", dir)
	uc.CheckInBackground()
	result := uc.CollectResult(time.Second)
	assert.Nil(t, result) // no update available
}

func TestPrintUpdateNoticeNoUpdate(t *testing.T) {
	buf := new(bytes.Buffer)
	PrintUpdateNotice(buf, "v1.0.0", nil, true)
	assert.Empty(t, buf.String())

	PrintUpdateNotice(buf, "v1.0.0", &UpdateResult{UpdateAvail: false}, true)
	assert.Empty(t, buf.String())
}

func TestPrintUpdateNoticeWithUpdate(t *testing.T) {
	buf := new(bytes.Buffer)
	PrintUpdateNotice(buf, "v1.0.0", &UpdateResult{
		LatestVersion: "v2.0.0",
		UpdateAvail:   true,
	}, true)
	out := buf.String()
	assert.Contains(t, out, "Update available:")
	assert.Contains(t, out, "v1.0.0")
	assert.Contains(t, out, "v2.0.0")
}

func TestCollectResultTimeout(t *testing.T) {
	// Checker that will never finish (no CheckInBackground called, channel never closed)
	uc := &UpdateChecker{
		done: make(chan struct{}),
	}
	result := uc.CollectResult(10 * time.Millisecond)
	assert.Nil(t, result)
}
