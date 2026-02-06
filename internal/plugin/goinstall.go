package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoResolver resolves plugins via go install.
type GoResolver struct {
	platform Platform
}

// NewGoResolver creates a new Go install resolver.
func NewGoResolver() *GoResolver {
	return &GoResolver{
		platform: CurrentPlatform(),
	}
}

// Name returns the resolver name.
func (r *GoResolver) Name() string {
	return "go"
}

// CanResolve returns true if this resolver can handle the plugin.
func (r *GoResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourceGo
}

// Resolve resolves the plugin version.
// For go install, we don't need to resolve versions - Go handles that.
func (r *GoResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	resolved := *plugin // Copy
	resolved.ResolvedVersion = plugin.Version
	return &resolved, nil
}

// Install downloads and installs the plugin via go install.
func (r *GoResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	result := &InstallResult{
		Plugin: plugin,
		Status: StatusInstalling,
	}

	// Create GOBIN directory
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to create bin directory: %w", err)
		return result, result.Error
	}

	// Build the module path with version
	modulePath := plugin.Repo
	if plugin.Version != "" && plugin.Version != "latest" {
		modulePath = fmt.Sprintf("%s@%s", plugin.Repo, plugin.Version)
	} else {
		modulePath = fmt.Sprintf("%s@latest", plugin.Repo)
	}

	// Run go install with GOBIN set to our install directory
	cmd := exec.CommandContext(ctx, "go", "install", modulePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOBIN=%s", binDir))

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("go install failed: %w\nOutput: %s", err, string(output))
		return result, result.Error
	}

	// Find the installed binary
	binaryName := plugin.BinaryName + r.platform.BinaryExtension()
	binaryPath := filepath.Join(binDir, binaryName)

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try to find any binary in the bin directory
		entries, _ := os.ReadDir(binDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				binaryPath = filepath.Join(binDir, entry.Name())
				plugin.BinaryName = strings.TrimSuffix(entry.Name(), r.platform.BinaryExtension())
				break
			}
		}
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("binary not found after installation")
		return result, result.Error
	}

	plugin.InstallPath = installDir
	plugin.BinaryPath = binaryPath

	result.Status = StatusInstalled
	result.Message = fmt.Sprintf("Installed %s via go install", plugin.Name)
	return result, nil
}

// Verify checks if the plugin is correctly installed.
func (r *GoResolver) Verify(ctx context.Context, plugin *Plugin) error {
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
