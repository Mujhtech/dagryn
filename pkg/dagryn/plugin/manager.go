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
	noCache     bool
	logger      Logger
}

// Logger interface for plugin manager logging.
type Logger interface {
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// defaultLogger is a no-op logger.
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, args ...interface{})  {}
func (l *defaultLogger) Debug(msg string, args ...interface{}) {}
func (l *defaultLogger) Warn(msg string, args ...interface{})  {}
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

// WithNoPluginCache disables the disk cache for plugin manifests and releases.
func WithNoPluginCache() ManagerOption {
	return func(m *Manager) {
		m.noCache = true
	}
}

// NewManager creates a new plugin manager.
func NewManager(projectRoot string, opts ...ManagerOption) *Manager {
	pluginDir := filepath.Join(projectRoot, ".dagryn", PluginDir)

	m := &Manager{
		projectRoot: projectRoot,
		pluginDir:   pluginDir,
		installed:   make(map[string]*Plugin),
		logger:      &defaultLogger{},
	}

	for _, opt := range opts {
		opt(m)
	}

	// Initialize disk cache for GitHub API responses
	cacheDir := filepath.Join(pluginDir, ".cache")
	diskCache := NewDiskCache(cacheDir)
	if m.noCache || os.Getenv("DAGRYN_NO_PLUGIN_CACHE") == "1" {
		diskCache.Disable()
	}

	// Create default registry with all resolvers
	registry := NewResolverRegistry()
	registry.Register(SourceGitHub, NewGitHubResolver(WithDiskCache(diskCache)))
	registry.Register(SourceGo, NewGoResolver())
	registry.Register(SourceNPM, NewNPMResolver())
	registry.Register(SourcePip, NewPipResolver())
	registry.Register(SourceCargo, NewCargoResolver())
	registry.Register(SourceLocal, NewLocalResolver(projectRoot))
	m.registry = registry

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
	if p, ok := m.installed[spec]; ok {
		m.mu.RUnlock()
		// If the cached plugin has its manifest, return immediately.
		// Otherwise fall through to re-resolve (manifest may have been lost
		// during lock file serialization).
		if p.Manifest != nil {
			return p, nil
		}
	} else {
		m.mu.RUnlock()
	}

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

	// Update the installed cache so subsequent resolves return the manifest
	m.mu.Lock()
	m.installed[spec] = resolved
	m.mu.Unlock()

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

		// Composite plugins don't need a binary — just check the directory exists
		isComposite := plugin.Manifest != nil && plugin.Manifest.IsComposite()
		if isComposite {
			m.mu.Lock()
			m.installed[spec] = plugin
			m.mu.Unlock()

			return &InstallResult{
				Plugin:  plugin,
				Status:  StatusCached,
				Message: fmt.Sprintf("Plugin %s found in cache", plugin.Name),
			}, nil
		}

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

// Register adds a resolved plugin to the installed cache and persists it to the lock file.
// This is used for composite plugins that don't need binary installation.
func (m *Manager) Register(spec string, plugin *Plugin) {
	m.mu.Lock()
	m.installed[spec] = plugin
	m.mu.Unlock()

	m.saveLockFile()
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
	if err := os.Remove(lockPath); err != nil {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

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
	case SourceLocal:
		// Local composite plugins have no binary; local tool plugins
		// have their binary inside the plugin directory.
		if plugin.BinaryName != "" {
			searchPaths = []string{
				filepath.Join(installDir, plugin.BinaryName+platform.BinaryExtension()),
				filepath.Join(installDir, "bin", plugin.BinaryName+platform.BinaryExtension()),
			}
		}
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
	BinaryPath      string    `json:"binary_path,omitempty"`
	InstallPath     string    `json:"install_path,omitempty"`
	IsComposite     bool      `json:"is_composite,omitempty"`
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
		// Composite plugins have no binary — verify install path instead
		if entry.IsComposite {
			installPath := entry.InstallPath
			if installPath == "" {
				continue
			}
			// Verify the plugin directory still exists
			if _, err := os.Stat(installPath); err != nil {
				continue
			}
			p := &Plugin{
				Name:            entry.Name,
				Source:          SourceType(entry.Source),
				Version:         entry.Version,
				ResolvedVersion: entry.ResolvedVersion,
				InstallPath:     installPath,
				Raw:             entry.Spec,
			}
			// Restore manifest from disk so composite plugins work on subsequent runs
			manifestPath := filepath.Join(installPath, "plugin.toml")
			if data, err := os.ReadFile(manifestPath); err == nil {
				if manifest, err := ParseManifest(data); err == nil {
					p.Manifest = manifest
				}
			}
			m.installed[entry.Spec] = p
			continue
		}

		// Binary plugins — verify the binary still exists
		if entry.BinaryPath == "" {
			continue
		}
		if _, err := os.Stat(entry.BinaryPath); err != nil {
			continue // Binary removed, skip
		}

		installPath := entry.InstallPath
		if installPath == "" {
			installPath = filepath.Dir(filepath.Dir(entry.BinaryPath))
		}

		p := &Plugin{
			Name:            entry.Name,
			Source:          SourceType(entry.Source),
			Version:         entry.Version,
			ResolvedVersion: entry.ResolvedVersion,
			BinaryPath:      entry.BinaryPath,
			InstallPath:     installPath,
			Raw:             entry.Spec,
		}
		m.installed[entry.Spec] = p
	}
}

// saveLockFile saves the plugin lock file.
func (m *Manager) saveLockFile() {
	lockPath := filepath.Join(m.projectRoot, ".dagryn", LockFileName)

	// Ensure directory exists
	_ = os.MkdirAll(filepath.Dir(lockPath), 0755)

	lockFile := LockFile{
		Version: 1,
		Plugins: make([]LockFileEntry, 0, len(m.installed)),
	}

	for spec, plugin := range m.installed {
		isComposite := plugin.Manifest != nil && plugin.Manifest.IsComposite()
		lockFile.Plugins = append(lockFile.Plugins, LockFileEntry{
			Spec:            spec,
			Source:          string(plugin.Source),
			Name:            plugin.Name,
			Version:         plugin.Version,
			ResolvedVersion: plugin.ResolvedVersion,
			BinaryPath:      plugin.BinaryPath,
			InstallPath:     plugin.InstallPath,
			IsComposite:     isComposite,
			InstalledAt:     time.Now(),
		})
	}

	data, err := json.MarshalIndent(lockFile, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(lockPath, data, 0644)
}
