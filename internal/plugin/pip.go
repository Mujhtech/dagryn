package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PipResolver resolves plugins via pip.
type PipResolver struct {
	platform Platform
}

// NewPipResolver creates a new pip resolver.
func NewPipResolver() *PipResolver {
	return &PipResolver{
		platform: CurrentPlatform(),
	}
}

// Name returns the resolver name.
func (r *PipResolver) Name() string {
	return "pip"
}

// CanResolve returns true if this resolver can handle the plugin.
func (r *PipResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourcePip
}

// Resolve resolves the plugin version.
func (r *PipResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	resolved := *plugin // Copy

	if plugin.Version == "latest" {
		// Query PyPI for latest version
		version, err := r.getLatestVersion(ctx, plugin.Repo)
		if err != nil {
			// If we can't get the latest version, just use empty (pip will get latest)
			resolved.ResolvedVersion = ""
		} else {
			resolved.ResolvedVersion = version
		}
	} else {
		resolved.ResolvedVersion = plugin.Version
	}

	return &resolved, nil
}

// Install downloads and installs the plugin via pip in a virtualenv.
func (r *PipResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	result := &InstallResult{
		Plugin: plugin,
		Status: StatusInstalling,
	}

	// Create virtualenv directory
	venvDir := filepath.Join(installDir, "venv")

	// Create virtualenv if it doesn't exist
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, "python3", "-m", "venv", venvDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			result.Status = StatusFailed
			result.Error = fmt.Errorf("failed to create virtualenv: %w\nOutput: %s", err, string(output))
			return result, result.Error
		}
	}

	// Determine pip and bin paths based on platform
	var pipPath, binDir string
	if r.platform.OS == "windows" {
		pipPath = filepath.Join(venvDir, "Scripts", "pip.exe")
		binDir = filepath.Join(venvDir, "Scripts")
	} else {
		pipPath = filepath.Join(venvDir, "bin", "pip")
		binDir = filepath.Join(venvDir, "bin")
	}

	// Build package spec with version
	packageSpec := plugin.Repo
	version := plugin.ResolvedVersion
	if version == "" {
		version = plugin.Version
	}
	if version != "" && version != "latest" {
		packageSpec = fmt.Sprintf("%s==%s", plugin.Repo, version)
	}

	// Run pip install
	cmd := exec.CommandContext(ctx, pipPath, "install", packageSpec)
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("pip install failed: %w\nOutput: %s", err, string(output))
		return result, result.Error
	}

	// Find the binary
	binaryName := plugin.BinaryName
	if r.platform.OS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(binDir, binaryName)

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try common alternative names
		alternatives := []string{
			plugin.Name,
			strings.ReplaceAll(plugin.Name, "-", "_"),
			strings.ReplaceAll(plugin.Name, "_", "-"),
		}

		for _, alt := range alternatives {
			altPath := filepath.Join(binDir, alt)
			if r.platform.OS == "windows" {
				altPath += ".exe"
			}
			if _, err := os.Stat(altPath); err == nil {
				binaryPath = altPath
				break
			}
		}
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("binary not found after installation in %s", binDir)
		return result, result.Error
	}

	plugin.InstallPath = installDir
	plugin.BinaryPath = binaryPath

	result.Status = StatusInstalled
	result.Message = fmt.Sprintf("Installed %s via pip in virtualenv", plugin.Name)
	return result, nil
}

// Verify checks if the plugin is correctly installed.
func (r *PipResolver) Verify(ctx context.Context, plugin *Plugin) error {
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

	return nil
}

// getLatestVersion queries PyPI for the latest version.
func (r *PipResolver) getLatestVersion(ctx context.Context, packageName string) (string, error) {
	cmd := exec.CommandContext(ctx, "pip", "index", "versions", packageName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse output like "package (1.2.3)"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "(") && strings.Contains(line, ")") {
			start := strings.Index(line, "(")
			end := strings.Index(line, ")")
			if start >= 0 && end > start {
				return strings.TrimSpace(line[start+1 : end]), nil
			}
		}
	}

	return "", fmt.Errorf("could not parse version from pip output")
}
