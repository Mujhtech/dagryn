package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	crossCompile := r.platform.OS != runtime.GOOS || r.platform.Arch != runtime.GOARCH

	// Create bin directory (final destination for the binary)
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to create bin directory: %w", err)
		return result, result.Error
	}

	// Build the module path with version
	var modulePath string
	if plugin.Version != "" && plugin.Version != "latest" {
		modulePath = fmt.Sprintf("%s@%s", plugin.Repo, plugin.Version)
	} else {
		modulePath = fmt.Sprintf("%s@latest", plugin.Repo)
	}

	// Run go install.
	// When cross-compiling, Go refuses GOBIN ("cannot install cross-compiled
	// binaries when GOBIN is set"). Use a temporary GOPATH instead; Go places
	// cross-compiled binaries in $GOPATH/bin/$GOOS_$GOARCH/.
	cmd := exec.CommandContext(ctx, "go", "install", modulePath)
	var env []string

	if crossCompile {
		gopath, err := os.MkdirTemp("", "dagryn-gopath-*")
		if err != nil {
			result.Status = StatusFailed
			result.Error = fmt.Errorf("failed to create temp GOPATH: %w", err)
			return result, result.Error
		}
		defer func() {
			_ = os.RemoveAll(gopath)
		}()

		env = append(os.Environ(),
			fmt.Sprintf("GOPATH=%s", gopath),
			fmt.Sprintf("GOOS=%s", r.platform.OS),
			fmt.Sprintf("GOARCH=%s", r.platform.Arch),
			"CGO_ENABLED=0",
		)
		cmd.Env = env

		output, err := cmd.CombinedOutput()
		if err != nil {
			result.Status = StatusFailed
			result.Error = fmt.Errorf("go install failed: %w\nOutput: %s", err, string(output))
			return result, result.Error
		}

		// Go places cross-compiled binaries in $GOPATH/bin/$GOOS_$GOARCH/
		crossBinDir := filepath.Join(gopath, "bin",
			fmt.Sprintf("%s_%s", r.platform.OS, r.platform.Arch))

		// Move the binary from the temp GOPATH to our install dir
		if err := r.moveCrossCompiledBinary(plugin, crossBinDir, binDir); err != nil {
			result.Status = StatusFailed
			result.Error = err
			return result, result.Error
		}
	} else {
		env = append(os.Environ(), fmt.Sprintf("GOBIN=%s", binDir))
		cmd.Env = env

		output, err := cmd.CombinedOutput()
		if err != nil {
			result.Status = StatusFailed
			result.Error = fmt.Errorf("go install failed: %w\nOutput: %s", err, string(output))
			return result, result.Error
		}
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

// moveCrossCompiledBinary moves the cross-compiled binary from the temporary
// GOPATH bin directory to the final install bin directory.
func (r *GoResolver) moveCrossCompiledBinary(plugin *Plugin, srcDir, destDir string) error {
	binaryName := plugin.BinaryName + r.platform.BinaryExtension()
	srcPath := filepath.Join(srcDir, binaryName)

	// Check exact name first
	if _, err := os.Stat(srcPath); err == nil {
		return os.Rename(srcPath, filepath.Join(destDir, binaryName))
	}

	// Fall back to finding any binary in the directory
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("cross-compiled binary directory not found: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			srcPath = filepath.Join(srcDir, entry.Name())
			return os.Rename(srcPath, filepath.Join(destDir, entry.Name()))
		}
	}

	return fmt.Errorf("no cross-compiled binary found in %s", srcDir)
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
