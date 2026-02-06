package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// PluginDir is the subdirectory within .dagryn for plugins.
	PluginDir = "plugins"
	// LockFileName is the name of the plugin lock file.
	LockFileName = "plugins.lock"
)

// Manager handles plugin resolution, installation, and caching.
type Manager struct {
	projectRoot string
	pluginDir   string
	registry    *ResolverRegistry
	installed   map[string]*Plugin // Cache of installed plugins by spec
	mu          sync.RWMutex
	verbose     bool
	logger      Logger
}

// Logger interface for plugin manager logging.
type Logger interface {
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// defaultLogger is a no-op logger.
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, args ...interface{})  {}
func (l *defaultLogger) Debug(msg string, args ...interface{}) {}
func (l *defaultLogger) Error(msg string, args ...interface{}) {}

// ManagerOption is a functional option for configuring Manager.
type ManagerOption func(*Manager)

// WithVerbose enables verbose logging.
func WithVerbose(verbose bool) ManagerOption {
	return func(m *Manager) {
		m.verbose = verbose
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger Logger) ManagerOption {
	return func(m *Manager) {
		m.logger = logger
	}
}

// WithGitHubTokenForManager sets GitHub token for the GitHub resolver.
func WithGitHubTokenForManager(token string) ManagerOption {
	return func(m *Manager) {
		// Re-register GitHub resolver with token
		m.registry.Register(SourceGitHub, NewGitHubResolver(WithGitHubToken(token)))
	}
}

// NewManager creates a new plugin manager.
func NewManager(projectRoot string, opts ...ManagerOption) *Manager {
	pluginDir := filepath.Join(projectRoot, ".dagryn", PluginDir)

	// Create default registry with all resolvers
	registry := NewResolverRegistry()
	registry.Register(SourceGitHub, NewGitHubResolver())
	registry.Register(SourceGo, NewGoResolver())
	registry.Register(SourceNPM, NewNPMResolver())
	registry.Register(SourcePip, NewPipResolver())
	registry.Register(SourceCargo, NewCargoResolver())

	m := &Manager{
		projectRoot: projectRoot,
		pluginDir:   pluginDir,
		registry:    registry,
		installed:   make(map[string]*Plugin),
		logger:      &defaultLogger{},
	}

	for _, opt := range opts {
		opt(m)
	}

	// Load existing lock file if present
	m.loadLockFile()

	return m
}

// PluginDir returns the plugin directory path.
func (m *Manager) PluginDir() string {
	return m.pluginDir
}

// Resolve parses and resolves a plugin specification.
func (m *Manager) Resolve(ctx context.Context, spec string) (*Plugin, error) {
	// Check if already installed
	m.mu.RLock()
	if plugin, ok := m.installed[spec]; ok {
		m.mu.RUnlock()
		return plugin, nil
	}
	m.mu.RUnlock()

	// Parse the specification
	plugin, err := Parse(spec)
	if err != nil {
		return nil, err
	}

	// Get the appropriate resolver
	resolver, err := m.registry.GetForPlugin(plugin)
	if err != nil {
		return nil, err
	}

	// Resolve the version
	resolved, err := resolver.Resolve(ctx, plugin)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", spec, err)
	}

	return resolved, nil
}

// Install installs a plugin and returns its binary path.
func (m *Manager) Install(ctx context.Context, spec string) (*InstallResult, error) {
	// Check if already installed
	m.mu.RLock()
	if plugin, ok := m.installed[spec]; ok {
		m.mu.RUnlock()
		return &InstallResult{
			Plugin:  plugin,
			Status:  StatusCached,
			Message: fmt.Sprintf("Plugin %s already installed", plugin.Name),
		}, nil
	}
	m.mu.RUnlock()

	// Resolve the plugin
	plugin, err := m.Resolve(ctx, spec)
	if err != nil {
		return &InstallResult{
			Status: StatusFailed,
			Error:  err,
		}, err
	}

	// Calculate install directory
	installDir := m.getInstallDir(plugin)

	// Check if already cached on disk
	if m.isCached(plugin, installDir) {
		plugin.InstallPath = installDir
		plugin.BinaryPath = m.findCachedBinary(plugin, installDir)

		if plugin.BinaryPath != "" {
			m.mu.Lock()
			m.installed[spec] = plugin
			m.mu.Unlock()

			return &InstallResult{
				Plugin:  plugin,
				Status:  StatusCached,
				Message: fmt.Sprintf("Plugin %s found in cache", plugin.Name),
			}, nil
		}
	}

	// Get resolver and install
	resolver, err := m.registry.GetForPlugin(plugin)
	if err != nil {
		return &InstallResult{
			Plugin: plugin,
			Status: StatusFailed,
			Error:  err,
		}, err
	}

	result, err := resolver.Install(ctx, plugin, installDir)
	if err != nil {
		return result, err
	}

	// Cache the installed plugin
	m.mu.Lock()
	m.installed[spec] = plugin
	m.mu.Unlock()

	// Update lock file
	m.saveLockFile()

	return result, nil
}

// InstallAll installs multiple plugins in parallel.
func (m *Manager) InstallAll(ctx context.Context, specs []string) ([]*InstallResult, error) {
	results := make([]*InstallResult, len(specs))
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for i, spec := range specs {
		wg.Add(1)
		go func(idx int, s string) {
			defer wg.Done()

			result, err := m.Install(ctx, s)
			results[idx] = result

			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}(i, spec)
	}

	wg.Wait()
	return results, firstErr
}

// GetBinaryPath returns the path to an installed plugin's binary.
func (m *Manager) GetBinaryPath(spec string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, ok := m.installed[spec]
	if !ok {
		return "", fmt.Errorf("plugin %s not installed", spec)
	}

	return plugin.BinaryPath, nil
}

// GetBinPaths returns all binary directories for installed plugins.
// This can be prepended to PATH for task execution.
func (m *Manager) GetBinPaths(specs []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	paths := make([]string, 0, len(specs))
	seen := make(map[string]bool)

	for _, spec := range specs {
		plugin, ok := m.installed[spec]
		if !ok || plugin.BinaryPath == "" {
			continue
		}

		binDir := filepath.Dir(plugin.BinaryPath)
		if !seen[binDir] {
			seen[binDir] = true
			paths = append(paths, binDir)
		}
	}

	return paths
}

// List returns all installed plugins.
func (m *Manager) List() []*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*Plugin, 0, len(m.installed))
	for _, p := range m.installed {
		plugins = append(plugins, p)
	}
	return plugins
}

// Clean removes all installed plugins.
func (m *Manager) Clean() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.RemoveAll(m.pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	m.installed = make(map[string]*Plugin)

	// Remove lock file
	lockPath := filepath.Join(m.projectRoot, ".dagryn", LockFileName)
	os.Remove(lockPath)

	return nil
}

// CleanPlugin removes a specific installed plugin.
func (m *Manager) CleanPlugin(spec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, ok := m.installed[spec]
	if !ok {
		return fmt.Errorf("plugin %s not installed", spec)
	}

	if plugin.InstallPath != "" {
		if err := os.RemoveAll(plugin.InstallPath); err != nil {
			return fmt.Errorf("failed to remove plugin: %w", err)
		}
	}

	delete(m.installed, spec)
	m.saveLockFile()

	return nil
}

// getInstallDir returns the installation directory for a plugin.
func (m *Manager) getInstallDir(plugin *Plugin) string {
	version := plugin.ResolvedVersion
	if version == "" {
		version = plugin.Version
	}
	return filepath.Join(m.pluginDir, string(plugin.Source), plugin.Name, version)
}

// isCached checks if a plugin is already cached on disk.
func (m *Manager) isCached(plugin *Plugin, installDir string) bool {
	info, err := os.Stat(installDir)
	return err == nil && info.IsDir()
}

// findCachedBinary finds the binary in a cached plugin directory.
func (m *Manager) findCachedBinary(plugin *Plugin, installDir string) string {
	platform := CurrentPlatform()

	// Check common locations based on source type
	var searchPaths []string

	switch plugin.Source {
	case SourceGitHub, SourceGo, SourceCargo:
		searchPaths = []string{
			filepath.Join(installDir, plugin.BinaryName+platform.BinaryExtension()),
			filepath.Join(installDir, "bin", plugin.BinaryName+platform.BinaryExtension()),
		}
	case SourceNPM:
		binName := plugin.BinaryName
		if platform.OS == "windows" {
			binName += ".cmd"
		}
		searchPaths = []string{
			filepath.Join(installDir, "node_modules", ".bin", binName),
		}
	case SourcePip:
		binName := plugin.BinaryName
		if platform.OS == "windows" {
			binName += ".exe"
		}
		if platform.OS == "windows" {
			searchPaths = []string{
				filepath.Join(installDir, "venv", "Scripts", binName),
			}
		} else {
			searchPaths = []string{
				filepath.Join(installDir, "venv", "bin", binName),
			}
		}
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// LockFileEntry represents an entry in the lock file.
type LockFileEntry struct {
	Spec            string    `json:"spec"`
	Source          string    `json:"source"`
	Name            string    `json:"name"`
	Version         string    `json:"version"`
	ResolvedVersion string    `json:"resolved_version"`
	BinaryPath      string    `json:"binary_path"`
	InstalledAt     time.Time `json:"installed_at"`
}

// LockFile represents the plugin lock file structure.
type LockFile struct {
	Version int             `json:"version"`
	Plugins []LockFileEntry `json:"plugins"`
}

// loadLockFile loads the plugin lock file.
func (m *Manager) loadLockFile() {
	lockPath := filepath.Join(m.projectRoot, ".dagryn", LockFileName)

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return // No lock file yet
	}

	var lockFile LockFile
	if err := json.Unmarshal(data, &lockFile); err != nil {
		return // Invalid lock file
	}

	for _, entry := range lockFile.Plugins {
		// Verify the binary still exists
		if _, err := os.Stat(entry.BinaryPath); err != nil {
			continue // Binary removed, skip
		}

		plugin := &Plugin{
			Name:            entry.Name,
			Source:          SourceType(entry.Source),
			Version:         entry.Version,
			ResolvedVersion: entry.ResolvedVersion,
			BinaryPath:      entry.BinaryPath,
			InstallPath:     filepath.Dir(filepath.Dir(entry.BinaryPath)),
			Raw:             entry.Spec,
		}
		m.installed[entry.Spec] = plugin
	}
}

// saveLockFile saves the plugin lock file.
func (m *Manager) saveLockFile() {
	lockPath := filepath.Join(m.projectRoot, ".dagryn", LockFileName)

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(lockPath), 0755)

	lockFile := LockFile{
		Version: 1,
		Plugins: make([]LockFileEntry, 0, len(m.installed)),
	}

	for spec, plugin := range m.installed {
		lockFile.Plugins = append(lockFile.Plugins, LockFileEntry{
			Spec:            spec,
			Source:          string(plugin.Source),
			Name:            plugin.Name,
			Version:         plugin.Version,
			ResolvedVersion: plugin.ResolvedVersion,
			BinaryPath:      plugin.BinaryPath,
			InstalledAt:     time.Now(),
		})
	}

	data, err := json.MarshalIndent(lockFile, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(lockPath, data, 0644)
}

// ResolveGlobalPlugins resolves global plugin references to their full specs.
// This handles the case where a task uses a plugin name that refers to a global plugin.
func (m *Manager) ResolveGlobalPlugins(taskPlugins []string, globalPlugins map[string]string) []string {
	resolved := make([]string, 0, len(taskPlugins))

	for _, p := range taskPlugins {
		// Check if it's a reference to a global plugin
		if spec, ok := globalPlugins[p]; ok {
			resolved = append(resolved, spec)
		} else {
			// It's a direct plugin spec
			resolved = append(resolved, p)
		}
	}

	return resolved
}
