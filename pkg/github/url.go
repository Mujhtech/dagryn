package github

import (
	"errors"
	"strings"
)

const baseURL = "https://api.github.com"

// ErrInvalidRepoURL is returned when a GitHub repository URL cannot be parsed.
var ErrInvalidRepoURL = errors.New("invalid GitHub repository URL")

// ParseOwnerRepo extracts the owner and repository name from a GitHub URL.
// Supports https://github.com/owner/repo, https://github.com/owner/repo.git,
// and git@github.com:owner/repo.git.
func ParseOwnerRepo(repoURL string) (owner, repo string, err error) {
	u := strings.TrimSpace(repoURL)
	u = strings.TrimSuffix(u, ".git")

	var parts []string
	if strings.HasPrefix(u, "git@github.com:") {
		u = strings.TrimPrefix(u, "git@github.com:")
		parts = strings.Split(u, "/")
	} else if strings.Contains(u, "github.com/") {
		i := strings.Index(u, "github.com/")
		u = u[i+len("github.com/"):]
		parts = strings.Split(u, "/")
	} else {
		return "", "", ErrInvalidRepoURL
	}

	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrInvalidRepoURL
	}
	return parts[0], parts[1], nil
}

// SplitFullName splits "owner/repo" into its two components.
func SplitFullName(fullName string) (string, string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(fullName), ".git")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("repo name must be in owner/repo format")
	}
	return parts[0], parts[1], nil
}
