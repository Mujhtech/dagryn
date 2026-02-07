// Package plugin provides a plugin system for Dagryn that allows tasks
// to declare tool dependencies that are automatically installed and cached.
package plugin

import (
	"fmt"
	"regexp"
	"strings"
)

// SourceType represents the type of plugin source.
type SourceType string

const (
	// SourceGitHub represents plugins from GitHub releases.
	SourceGitHub SourceType = "github"
	// SourceGo represents plugins installed via go install.
	SourceGo SourceType = "go"
	// SourceNPM represents plugins installed via npm.
	SourceNPM SourceType = "npm"
	// SourcePip represents plugins installed via pip.
	SourcePip SourceType = "pip"
	// SourceCargo represents plugins installed via cargo install.
	SourceCargo SourceType = "cargo"
)

// Plugin represents a resolved plugin with all its metadata.
type Plugin struct {
	// Name is the human-readable name of the plugin (e.g., "golangci-lint").
	Name string

	// Source is the type of plugin source (github, go, npm, pip, cargo).
	Source SourceType

	// Owner is the repository owner (for GitHub) or empty for other sources.
	Owner string

	// Repo is the repository name (for GitHub) or package name for other sources.
	Repo string

	// Version is the version constraint (e.g., "v1.55.0", "latest", "^1.0.0").
	Version string

	// ResolvedVersion is the actual resolved version after semver resolution.
	ResolvedVersion string

	// BinaryName is the name of the binary executable.
	BinaryName string

	// InstallPath is the absolute path where the plugin is installed.
	InstallPath string

	// BinaryPath is the absolute path to the plugin binary.
	BinaryPath string

	// Raw is the original plugin specification string.
	Raw string

	// Manifest is the parsed plugin.toml manifest (populated when available).
	Manifest *Manifest
}

// Spec represents a plugin specification that can be a single string or array.
type Spec struct {
	Plugins []string
}

// UnmarshalTOML implements custom TOML unmarshaling to handle both string and array.
func (s *Spec) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case string:
		if v != "" {
			s.Plugins = []string{v}
		}
	case []interface{}:
		s.Plugins = make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				s.Plugins = append(s.Plugins, str)
			} else {
				return fmt.Errorf("plugin spec array must contain only strings")
			}
		}
	case nil:
		s.Plugins = nil
	default:
		return fmt.Errorf("plugin spec must be a string or array of strings, got %T", data)
	}
	return nil
}

// IsEmpty returns true if the spec has no plugins.
func (s *Spec) IsEmpty() bool {
	return len(s.Plugins) == 0
}

// pluginPattern matches plugin specifications in the format "source:name@version"
// Examples:
//   - github:owner/repo@v1.0.0
//   - go:golang.org/x/tools/cmd/goimports@latest
//   - npm:prettier@3.0.0
//   - pip:black@23.12.0
//   - cargo:ripgrep@14.0.3
var pluginPattern = regexp.MustCompile(`^(github|go|npm|pip|cargo):(.+)@(.+)$`)

// shortRefPattern matches short reference format "owner/repo@version" (defaults to GitHub).
// Examples:
//   - dagryn/setup-go@v1
//   - golangci/golangci-lint@v1.55.0
var shortRefPattern = regexp.MustCompile(`^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.-]+)@(.+)$`)

// semverPattern matches semantic versions with optional 'v' prefix
var semverPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[\w.]+)?(\+[\w.]+)?$`)

// Parse parses a plugin specification string into a Plugin struct.
// Supports two formats:
//   - Long format:  "source:name@version" (e.g., "github:golangci/golangci-lint@v1.55.0")
//   - Short format: "owner/repo@version" (e.g., "dagryn/setup-go@v1", defaults to GitHub)
func Parse(spec string) (*Plugin, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty plugin specification")
	}

	// Try long format first (source:name@version)
	matches := pluginPattern.FindStringSubmatch(spec)
	if matches != nil {
		return parseLongFormat(matches, spec)
	}

	// Try short format (owner/repo@version -> implicit GitHub)
	shortMatches := shortRefPattern.FindStringSubmatch(spec)
	if shortMatches != nil {
		return parseShortFormat(shortMatches, spec)
	}

	return nil, fmt.Errorf("invalid plugin specification %q: must be in format 'source:name@version' or 'owner/repo@version'", spec)
}

// parseLongFormat parses the long format "source:name@version".
func parseLongFormat(matches []string, spec string) (*Plugin, error) {
	source := SourceType(matches[1])
	name := matches[2]
	version := matches[3]

	plugin := &Plugin{
		Source:  source,
		Version: version,
		Raw:     spec,
	}

	switch source {
	case SourceGitHub:
		parts := strings.Split(name, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid GitHub plugin %q: must be in format 'owner/repo'", name)
		}
		plugin.Owner = parts[0]
		plugin.Repo = parts[1]
		plugin.Name = parts[1]
		plugin.BinaryName = parts[1]

	case SourceGo:
		// Extract binary name from module path (last component)
		parts := strings.Split(name, "/")
		plugin.Repo = name
		plugin.Name = parts[len(parts)-1]
		plugin.BinaryName = parts[len(parts)-1]

	case SourceNPM:
		// Handle scoped packages (@org/package)
		plugin.Repo = name
		if strings.HasPrefix(name, "@") {
			parts := strings.Split(name, "/")
			if len(parts) >= 2 {
				plugin.Name = parts[1]
			} else {
				plugin.Name = name
			}
		} else {
			plugin.Name = name
		}
		plugin.BinaryName = plugin.Name

	case SourcePip:
		plugin.Repo = name
		plugin.Name = name
		plugin.BinaryName = name

	case SourceCargo:
		plugin.Repo = name
		plugin.Name = name
		plugin.BinaryName = name

	default:
		return nil, fmt.Errorf("unsupported plugin source: %s", source)
	}

	return plugin, nil
}

// parseShortFormat parses the short format "owner/repo@version" (implicit GitHub).
func parseShortFormat(matches []string, spec string) (*Plugin, error) {
	owner := matches[1]
	repo := matches[2]
	version := matches[3]

	return &Plugin{
		Source:     SourceGitHub,
		Owner:      owner,
		Repo:       repo,
		Name:       repo,
		BinaryName: repo,
		Version:    version,
		Raw:        spec,
	}, nil
}

// IsExactVersion returns true if the version is an exact version (not a range).
func (p *Plugin) IsExactVersion() bool {
	v := p.Version
	if v == "latest" {
		return false
	}
	// Check if it starts with ^ or ~ (semver range)
	if strings.HasPrefix(v, "^") || strings.HasPrefix(v, "~") {
		return false
	}
	return semverPattern.MatchString(v)
}

// IsSemverRange returns true if the version is a semver range.
func (p *Plugin) IsSemverRange() bool {
	v := p.Version
	return strings.HasPrefix(v, "^") || strings.HasPrefix(v, "~")
}

// CacheKey returns a unique key for caching this plugin.
func (p *Plugin) CacheKey() string {
	version := p.ResolvedVersion
	if version == "" {
		version = p.Version
	}
	return fmt.Sprintf("%s/%s/%s", p.Source, p.Name, version)
}

// String returns a human-readable representation of the plugin.
func (p *Plugin) String() string {
	return fmt.Sprintf("%s:%s@%s", p.Source, p.Name, p.Version)
}

// Status represents the installation status of a plugin.
type Status int

const (
	// StatusPending means the plugin has not been installed yet.
	StatusPending Status = iota
	// StatusInstalling means the plugin is currently being installed.
	StatusInstalling
	// StatusInstalled means the plugin was freshly installed.
	StatusInstalled
	// StatusCached means the plugin was found in cache.
	StatusCached
	// StatusFailed means the plugin installation failed.
	StatusFailed
)

// String returns the string representation of the status.
func (s Status) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusInstalling:
		return "INSTALLING"
	case StatusInstalled:
		return "INSTALLED"
	case StatusCached:
		return "CACHED"
	case StatusFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// InstallResult represents the result of installing a plugin.
type InstallResult struct {
	Plugin  *Plugin
	Status  Status
	Error   error
	Message string
}

// IsSuccess returns true if the plugin was installed successfully.
func (r *InstallResult) IsSuccess() bool {
	return r.Status == StatusInstalled || r.Status == StatusCached
}
