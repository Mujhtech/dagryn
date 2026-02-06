package plugin

import (
	"context"
	"fmt"
	"runtime"
)

// Resolver is the interface that all plugin resolvers must implement.
type Resolver interface {
	// Name returns the name of this resolver (e.g., "github", "go", "npm").
	Name() string

	// CanResolve returns true if this resolver can handle the given plugin.
	CanResolve(plugin *Plugin) bool

	// Resolve resolves the plugin version (e.g., "latest" -> "v1.55.0").
	// For semver ranges, this resolves to the best matching version.
	Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error)

	// Install downloads and installs the plugin to the specified directory.
	// Returns the path to the installed binary.
	Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error)

	// Verify checks if the plugin is correctly installed and functional.
	Verify(ctx context.Context, plugin *Plugin) error
}

// Platform represents the current OS and architecture.
type Platform struct {
	OS   string // "darwin", "linux", "windows"
	Arch string // "amd64", "arm64", "386"
}

// CurrentPlatform returns the current platform.
func CurrentPlatform() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// String returns a string representation of the platform.
func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// PlatformAliases returns common aliases for the platform used in release assets.
func (p Platform) PlatformAliases() []string {
	var osAliases []string
	var archAliases []string

	switch p.OS {
	case "darwin":
		osAliases = []string{"darwin", "macos", "osx", "mac", "apple"}
	case "linux":
		osAliases = []string{"linux", "linux-gnu"}
	case "windows":
		osAliases = []string{"windows", "win", "win64", "win32"}
	default:
		osAliases = []string{p.OS}
	}

	switch p.Arch {
	case "amd64":
		archAliases = []string{"amd64", "x86_64", "x64", "64bit", "64"}
	case "arm64":
		archAliases = []string{"arm64", "aarch64", "arm"}
	case "386":
		archAliases = []string{"386", "i386", "i686", "x86", "32bit", "32"}
	default:
		archAliases = []string{p.Arch}
	}

	// Generate all combinations
	var aliases []string
	for _, os := range osAliases {
		for _, arch := range archAliases {
			aliases = append(aliases, fmt.Sprintf("%s-%s", os, arch))
			aliases = append(aliases, fmt.Sprintf("%s_%s", os, arch))
			aliases = append(aliases, fmt.Sprintf("%s%s", os, arch))
		}
	}

	// Also add OS-only aliases (some releases don't include arch for universal binaries)
	aliases = append(aliases, osAliases...)

	return aliases
}

// BinaryExtension returns the expected binary extension for this platform.
func (p Platform) BinaryExtension() string {
	if p.OS == "windows" {
		return ".exe"
	}
	return ""
}

// ArchiveExtensions returns common archive extensions for this platform.
func (p Platform) ArchiveExtensions() []string {
	if p.OS == "windows" {
		return []string{".zip", ".tar.gz", ".tgz", ".7z"}
	}
	return []string{".tar.gz", ".tgz", ".tar.xz", ".tar.bz2", ".zip"}
}

// ResolverRegistry holds all registered resolvers.
type ResolverRegistry struct {
	resolvers map[SourceType]Resolver
}

// NewResolverRegistry creates a new resolver registry.
func NewResolverRegistry() *ResolverRegistry {
	return &ResolverRegistry{
		resolvers: make(map[SourceType]Resolver),
	}
}

// Register adds a resolver to the registry.
func (r *ResolverRegistry) Register(source SourceType, resolver Resolver) {
	r.resolvers[source] = resolver
}

// Get returns the resolver for the given source type.
func (r *ResolverRegistry) Get(source SourceType) (Resolver, bool) {
	resolver, ok := r.resolvers[source]
	return resolver, ok
}

// GetForPlugin returns the appropriate resolver for a plugin.
func (r *ResolverRegistry) GetForPlugin(plugin *Plugin) (Resolver, error) {
	resolver, ok := r.resolvers[plugin.Source]
	if !ok {
		return nil, fmt.Errorf("no resolver registered for source type: %s", plugin.Source)
	}
	if !resolver.CanResolve(plugin) {
		return nil, fmt.Errorf("resolver %s cannot handle plugin: %s", resolver.Name(), plugin.Raw)
	}
	return resolver, nil
}
