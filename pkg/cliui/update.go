package cliui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultCheckInterval is how often to check for updates (24 hours).
	DefaultCheckInterval = 24 * time.Hour
	// updateCheckTimeout prevents the update check from blocking the CLI.
	updateCheckTimeout = 3 * time.Second
)

// UpdateChecker performs non-blocking CLI version checks.
type UpdateChecker struct {
	CurrentVersion string
	// GitHubRepo is "owner/repo" (e.g. "mujhtech/dagryn").
	GitHubRepo string
	// CacheDir is the directory to store the cache file (e.g. ~/.dagryn).
	CacheDir string
	// CheckInterval controls how often to hit the network.
	CheckInterval time.Duration

	mu     sync.Mutex
	result *UpdateResult
	done   chan struct{}
}

// UpdateResult holds the outcome of an update check.
type UpdateResult struct {
	LatestVersion string `json:"latest_version"`
	UpdateAvail   bool   `json:"update_available"`
}

// updateCache is persisted between runs to avoid hammering GitHub.
type updateCache struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

// NewUpdateChecker creates an update checker. Call CheckInBackground early
// in your command, then CollectResult after the command finishes.
func NewUpdateChecker(currentVersion, githubRepo, cacheDir string) *UpdateChecker {
	return &UpdateChecker{
		CurrentVersion: currentVersion,
		GitHubRepo:     githubRepo,
		CacheDir:       cacheDir,
		CheckInterval:  DefaultCheckInterval,
		done:           make(chan struct{}),
	}
}

// CheckInBackground kicks off a goroutine that checks for a newer release.
// It is safe to call from PersistentPreRun — it never blocks.
func (u *UpdateChecker) CheckInBackground() {
	go func() {
		defer close(u.done)

		// Skip if current build is a dev build.
		if u.CurrentVersion == "dev" || u.CurrentVersion == "" {
			return
		}

		// Read cache — skip network if checked recently.
		cached, _ := u.readCache()
		if cached != nil && time.Since(cached.LastChecked) < u.CheckInterval {
			if isNewer(cached.LatestVersion, u.CurrentVersion) {
				u.mu.Lock()
				u.result = &UpdateResult{
					LatestVersion: cached.LatestVersion,
					UpdateAvail:   true,
				}
				u.mu.Unlock()
			}
			return
		}

		latest := u.fetchLatestVersion()
		if latest == "" {
			return
		}

		// Write cache regardless of result.
		_ = u.writeCache(&updateCache{
			LastChecked:   time.Now(),
			LatestVersion: latest,
		})

		if isNewer(latest, u.CurrentVersion) {
			u.mu.Lock()
			u.result = &UpdateResult{
				LatestVersion: latest,
				UpdateAvail:   true,
			}
			u.mu.Unlock()
		}
	}()
}

// CollectResult waits up to the given duration for the background check
// to finish and returns the result (nil if no update or timed out).
func (u *UpdateChecker) CollectResult(timeout time.Duration) *UpdateResult {
	select {
	case <-u.done:
	case <-time.After(timeout):
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.result
}

// PrintUpdateNotice writes a one-line notice to w if an update is available.
func PrintUpdateNotice(w io.Writer, current string, result *UpdateResult, noColor bool) {
	if result == nil || !result.UpdateAvail {
		return
	}
	arrow := Render(StyleDim, "→", noColor)
	oldV := Render(StyleWarn, current, noColor)
	newV := Render(StyleSuccess, result.LatestVersion, noColor)
	label := Render(StyleInfo, "Update available:", noColor)
	_, _ = fmt.Fprintf(w, "\n%s %s %s %s\n", label, oldV, arrow, newV)
}

func (u *UpdateChecker) cacheFilePath() string {
	return filepath.Join(u.CacheDir, "update-check.json")
}

func (u *UpdateChecker) readCache() (*updateCache, error) {
	data, err := os.ReadFile(u.cacheFilePath())
	if err != nil {
		return nil, err
	}
	var c updateCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (u *UpdateChecker) writeCache(c *updateCache) error {
	if err := os.MkdirAll(u.CacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(u.cacheFilePath(), data, 0o644)
}

// fetchLatestVersion queries the GitHub releases API.
func (u *UpdateChecker) fetchLatestVersion() string {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.GitHubRepo)
	client := &http.Client{Timeout: updateCheckTimeout}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}
	return release.TagName
}

// isNewer returns true if latest is a newer semver than current.
// Both may optionally have a "v" prefix.
func isNewer(latest, current string) bool {
	l := parseSemver(latest)
	c := parseSemver(current)
	if l == nil || c == nil {
		return false
	}
	if l[0] != c[0] {
		return l[0] > c[0]
	}
	if l[1] != c[1] {
		return l[1] > c[1]
	}
	return l[2] > c[2]
}

// parseSemver extracts major.minor.patch from a version string.
// Returns nil if the string is not a recognisable semver.
func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release suffix (e.g. "-dirty", "-rc1").
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	var major, minor, patch int
	n, _ := fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
	if n < 2 {
		return nil
	}
	return []int{major, minor, patch}
}
