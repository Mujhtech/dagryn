package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mujhtech/dagryn/pkg/dagryn/toolchain"
)

// NPMResolver resolves plugins via npm.
type NPMResolver struct {
	platform Platform
}

// NewNPMResolver creates a new npm resolver.
func NewNPMResolver() *NPMResolver {
	return &NPMResolver{
		platform: CurrentPlatform(),
	}
}

// Name returns the resolver name.
func (r *NPMResolver) Name() string {
	return "npm"
}

// CanResolve returns true if this resolver can handle the plugin.
func (r *NPMResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourceNPM
}

// Resolve resolves the plugin version by querying npm registry.
func (r *NPMResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	resolved := *plugin // Copy

	if plugin.Version == "latest" {
		// Query npm registry for latest version
		version, err := r.getLatestVersion(ctx, plugin.Repo)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve latest version: %w", err)
		}
		resolved.ResolvedVersion = version
	} else {
		resolved.ResolvedVersion = plugin.Version
	}

	return &resolved, nil
}

// Install downloads and installs the plugin via npm.
func (r *NPMResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	result := &InstallResult{
		Plugin: plugin,
		Status: StatusInstalling,
	}

	// Create the install directory
	if err := os.MkdirAll(installDir, 0755); err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to create install directory: %w", err)
		return result, result.Error
	}

	// Initialize a minimal package.json if it doesn't exist
	packageJSONPath := filepath.Join(installDir, "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		packageJSON := map[string]interface{}{
			"name":    "dagryn-plugins",
			"version": "1.0.0",
			"private": true,
		}
		data, _ := json.MarshalIndent(packageJSON, "", "  ")
		if err := os.WriteFile(packageJSONPath, data, 0644); err != nil {
			result.Status = StatusFailed
			result.Error = fmt.Errorf("failed to create package.json: %w", err)
			return result, result.Error
		}
	}

	// Build package spec with version
	packageSpec := plugin.Repo
	version := plugin.ResolvedVersion
	if version == "" {
		version = plugin.Version
	}
	if version != "" && version != "latest" {
		packageSpec = fmt.Sprintf("%s@%s", plugin.Repo, version)
	}

	// Run npm install
	cmd, err := r.newNPMCommand(ctx, "install", "--save", packageSpec)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err
		return result, result.Error
	}
	cmd.Dir = installDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("npm install failed: %w\nOutput: %s", err, string(output))
		return result, result.Error
	}

	// Find the binary in node_modules/.bin
	binDir := filepath.Join(installDir, "node_modules", ".bin")
	binaryName := plugin.BinaryName
	if r.platform.OS == "windows" {
		binaryName += ".cmd"
	}
	binaryPath := filepath.Join(binDir, binaryName)

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try to find any binary that matches the package name
		entries, _ := os.ReadDir(binDir)
		for _, entry := range entries {
			name := entry.Name()
			if r.platform.OS == "windows" {
				name = strings.TrimSuffix(name, ".cmd")
			}
			if strings.EqualFold(name, plugin.Name) {
				binaryPath = filepath.Join(binDir, entry.Name())
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
	result.Message = fmt.Sprintf("Installed %s via npm", plugin.Name)
	return result, nil
}

// Verify checks if the plugin is correctly installed.
func (r *NPMResolver) Verify(ctx context.Context, plugin *Plugin) error {
	if plugin.BinaryPath == "" {
		return fmt.Errorf("plugin binary path not set")
	}

	if _, err := os.Stat(plugin.BinaryPath); err != nil {
		return fmt.Errorf("binary not found: %w", err)
	}

	return nil
}

// getLatestVersion queries npm registry for the latest version.
func (r *NPMResolver) getLatestVersion(ctx context.Context, packageName string) (string, error) {
	cmd, err := r.newNPMCommand(ctx, "view", packageName, "version")
	if err != nil {
		return "", err
	}
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *NPMResolver) newNPMCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	tc, err := toolchain.EnsureRuntime(ctx, toolchain.RuntimeNode)
	if err != nil {
		return nil, fmt.Errorf("node/npm runtime not available: %w", err)
	}

	cmd := exec.CommandContext(ctx, "npm", args...)
	if tc == nil {
		return cmd, nil
	}

	env := envSliceToMap(os.Environ())
	if tc.BinDir != "" {
		env["PATH"] = toolchain.PrependPath(tc.BinDir, env["PATH"])
	}
	for k, v := range tc.Env {
		env[k] = v
	}
	cmd.Env = envMapToSlice(env)
	return cmd, nil
}

func envSliceToMap(items []string) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		if k, v, ok := strings.Cut(item, "="); ok {
			out[k] = v
		}
	}
	return out
}

func envMapToSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
