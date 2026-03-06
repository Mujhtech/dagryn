package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LocalResolver resolves plugins from local directories.
type LocalResolver struct {
	projectRoot string
}

// NewLocalResolver creates a new local plugin resolver.
func NewLocalResolver(projectRoot string) *LocalResolver {
	return &LocalResolver{projectRoot: projectRoot}
}

// Name returns the resolver name.
func (r *LocalResolver) Name() string {
	return "local"
}

// CanResolve returns true if this resolver can handle the plugin.
func (r *LocalResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourceLocal
}

// Resolve reads the plugin manifest from the local directory.
func (r *LocalResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	resolved := *plugin // Copy

	pluginDir := r.resolveDir(plugin.Repo)

	// Read plugin.toml from the local directory
	manifestPath := filepath.Join(pluginDir, "plugin.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", manifestPath, err)
	}

	manifest, err := ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", manifestPath, err)
	}

	if err := ValidateManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest at %s: %w", manifestPath, err)
	}

	resolved.Manifest = manifest
	resolved.Name = manifest.Plugin.Name
	resolved.InstallPath = pluginDir

	// Use version from manifest if not specified in the spec
	if resolved.Version == "" {
		resolved.Version = manifest.Plugin.Version
	}
	resolved.ResolvedVersion = manifest.Plugin.Version

	// For tool plugins, set the binary name from manifest
	if manifest.Tool.Binary != "" {
		resolved.BinaryName = manifest.Tool.Binary
	}

	return &resolved, nil
}

// Install handles local plugin installation. For composite plugins, no binary
// installation is needed. For tool plugins, the binary path is set from the
// local directory.
func (r *LocalResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	result := &InstallResult{
		Plugin: plugin,
		Status: StatusInstalling,
	}

	pluginDir := r.resolveDir(plugin.Repo)

	// Verify the directory still exists
	if _, err := os.Stat(pluginDir); err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("local plugin directory not found: %w", err)
		return result, result.Error
	}

	plugin.InstallPath = pluginDir

	// For tool plugins, look for the binary
	if plugin.Manifest != nil && plugin.Manifest.IsTool() && plugin.BinaryName != "" {
		binaryPath := filepath.Join(pluginDir, plugin.BinaryName)
		if _, err := os.Stat(binaryPath); err == nil {
			plugin.BinaryPath = binaryPath
		}
	}

	result.Status = StatusInstalled
	result.Message = fmt.Sprintf("Loaded local plugin %s from %s", plugin.Name, pluginDir)
	return result, nil
}

// Verify checks if the local plugin directory exists.
func (r *LocalResolver) Verify(ctx context.Context, plugin *Plugin) error {
	pluginDir := r.resolveDir(plugin.Repo)

	manifestPath := filepath.Join(pluginDir, "plugin.toml")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("plugin manifest not found at %s: %w", manifestPath, err)
	}

	return nil
}

// resolveDir resolves the plugin path relative to the project root.
func (r *LocalResolver) resolveDir(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.projectRoot, path)
}
