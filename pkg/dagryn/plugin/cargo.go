package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CargoResolver resolves plugins via cargo install.
type CargoResolver struct {
	platform Platform
}

// NewCargoResolver creates a new Cargo resolver.
func NewCargoResolver() *CargoResolver {
	return &CargoResolver{
		platform: CurrentPlatform(),
	}
}

// Name returns the resolver name.
func (r *CargoResolver) Name() string {
	return "cargo"
}

// CanResolve returns true if this resolver can handle the plugin.
func (r *CargoResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourceCargo
}

// Resolve resolves the plugin version.
func (r *CargoResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	resolved := *plugin // Copy
	resolved.ResolvedVersion = plugin.Version
	return &resolved, nil
}

// Install downloads and installs the plugin via cargo install.
func (r *CargoResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	result := &InstallResult{
		Plugin: plugin,
		Status: StatusInstalling,
	}

	// Create bin directory
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to create bin directory: %w", err)
		return result, result.Error
	}

	// Build cargo install arguments
	args := []string{"install", "--root", installDir}

	// Add version if specified
	version := plugin.ResolvedVersion
	if version == "" {
		version = plugin.Version
	}
	if version != "" && version != "latest" {
		args = append(args, "--version", version)
	}

	args = append(args, plugin.Repo)

	// Run cargo install
	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("CARGO_HOME=%s", installDir))

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("cargo install failed: %w\nOutput: %s", err, string(output))
		return result, result.Error
	}

	// Find the binary
	binaryName := plugin.BinaryName
	if r.platform.OS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(binDir, binaryName)

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try to find any executable in the bin directory
		entries, _ := os.ReadDir(binDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				info, _ := entry.Info()
				if info != nil && info.Mode()&0111 != 0 {
					binaryPath = filepath.Join(binDir, entry.Name())
					plugin.BinaryName = strings.TrimSuffix(entry.Name(), r.platform.BinaryExtension())
					break
				}
			}
		}
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("binary not found after installation")
		return result, result.Error
	}

	plugin.InstallPath = installDir
	plugin.BinaryPath = binaryPath

	result.Status = StatusInstalled
	result.Message = fmt.Sprintf("Installed %s via cargo", plugin.Name)
	return result, nil
}

// Verify checks if the plugin is correctly installed.
func (r *CargoResolver) Verify(ctx context.Context, plugin *Plugin) error {
	if plugin.BinaryPath == "" {
		return fmt.Errorf("plugin binary path not set")
	}

	info, err := os.Stat(plugin.BinaryPath)
	if err != nil {
		return fmt.Errorf("binary not found: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("binary path is a directory")
	}

	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable")
	}

	return nil
}
