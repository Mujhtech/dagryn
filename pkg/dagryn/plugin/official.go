package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultOfficialPluginsRepo = "https://github.com/mujhtech/dagryn-plugins.git"

// OfficialResolver resolves official Dagryn plugins from the dagryn-plugins
// repository using sparse checkout for only the requested plugin directory.
//
// Supported specs:
//   - official:setup-node
//   - official:setup-node@v1
//
// Resolution order:
//  1. Already cached install path (set by manager)
//  2. DAGRYN_PLUGINS_DIR (pre-vendored local mirror)
//  3. Sparse checkout from DAGRYN_OFFICIAL_PLUGINS_REPO (or default repo URL)
type OfficialResolver struct {
	projectRoot string
	fallback    *LocalResolver
}

func NewOfficialResolver(projectRoot string) *OfficialResolver {
	return &OfficialResolver{
		projectRoot: projectRoot,
		fallback:    NewLocalResolver(projectRoot),
	}
}

func (r *OfficialResolver) Name() string {
	return "official"
}

func (r *OfficialResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourceOfficial
}

func (r *OfficialResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	resolved := *plugin
	if resolved.Version == "" {
		resolved.Version = "latest"
	}

	// If manager already points us to a cached install path, use it directly.
	if resolved.InstallPath != "" {
		return r.resolveFromDir(ctx, &resolved, resolved.InstallPath)
	}

	if envRoot := strings.TrimSpace(os.Getenv("DAGRYN_PLUGINS_DIR")); envRoot != "" {
		candidate := filepath.Join(envRoot, resolved.Name)
		if p, err := r.resolveFromDir(ctx, &resolved, candidate); err == nil {
			return p, nil
		}
	}

	// Bridge mode from sparse checkout (manager install dir cache).
	version := resolved.Version
	if version == "latest" {
		version = "main"
	}
	cachedDir := filepath.Join(r.projectRoot, ".dagryn", PluginDir, string(SourceOfficial), resolved.Name, version)
	if p, err := r.resolveFromDir(ctx, &resolved, cachedDir); err == nil {
		return p, nil
	}

	return nil, fmt.Errorf("official plugin %q not available locally; run install to fetch via sparse checkout", resolved.Name)
}

func (r *OfficialResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	result := &InstallResult{Plugin: plugin, Status: StatusInstalling}

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		result.Status = StatusFailed
		result.Error = err
		return result, err
	}

	if envRoot := strings.TrimSpace(os.Getenv("DAGRYN_PLUGINS_DIR")); envRoot != "" {
		src := filepath.Join(envRoot, plugin.Name)
		if err := copyDir(src, installDir); err == nil {
			plugin.InstallPath = installDir
			resolved, err := r.resolveFromDir(ctx, plugin, installDir)
			if err != nil {
				result.Status = StatusFailed
				result.Error = err
				return result, err
			}
			result.Plugin = resolved
			result.Status = StatusInstalled
			result.Message = fmt.Sprintf("Installed official plugin %s from DAGRYN_PLUGINS_DIR", plugin.Name)
			return result, nil
		}
	}

	ref := plugin.Version
	if ref == "" || ref == "latest" {
		ref = "main"
	}

	repoURL := strings.TrimSpace(os.Getenv("DAGRYN_OFFICIAL_PLUGINS_REPO"))
	if repoURL == "" {
		repoURL = defaultOfficialPluginsRepo
	}

	if err := sparseCheckoutPlugin(ctx, repoURL, ref, plugin.Name, installDir); err != nil {
		result.Status = StatusFailed
		result.Error = err
		return result, err
	}

	resolved, err := r.resolveFromDir(ctx, plugin, installDir)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err
		return result, err
	}

	result.Plugin = resolved
	result.Status = StatusInstalled
	result.Message = fmt.Sprintf("Installed official plugin %s@%s", plugin.Name, ref)
	return result, nil
}

func (r *OfficialResolver) Verify(ctx context.Context, plugin *Plugin) error {
	if plugin.InstallPath == "" {
		return fmt.Errorf("official plugin %s is not installed", plugin.Name)
	}
	_, err := r.resolveFromDir(ctx, plugin, plugin.InstallPath)
	return err
}

func (r *OfficialResolver) resolveFromDir(ctx context.Context, plugin *Plugin, dir string) (*Plugin, error) {
	manifestPath := filepath.Join(dir, "plugin.toml")
	if st, err := os.Stat(manifestPath); err != nil || st.IsDir() {
		return nil, fmt.Errorf("plugin manifest not found at %s", manifestPath)
	}

	localPlugin := *plugin
	localPlugin.Source = SourceLocal
	localPlugin.Repo = dir
	localPlugin.Raw = "local:" + dir

	resolved, err := r.fallback.Resolve(ctx, &localPlugin)
	if err != nil {
		return nil, err
	}

	resolved.Source = SourceOfficial
	resolved.Owner = "dagryn"
	resolved.Repo = plugin.Name
	resolved.Name = plugin.Name
	resolved.Raw = plugin.Raw
	resolved.InstallPath = dir

	if plugin.Version != "" {
		resolved.Version = plugin.Version
	}
	if resolved.Version == "" {
		resolved.Version = "latest"
	}

	return resolved, nil
}

func sparseCheckoutPlugin(ctx context.Context, repoURL, ref, pluginName, installDir string) error {
	tmpDir, err := os.MkdirTemp("", "dagryn-official-plugin-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = tmpDir
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return fmt.Errorf("git %s failed: %w (%s)", strings.Join(args, " "), runErr, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("init"); err != nil {
		return err
	}
	if err := run("remote", "add", "origin", repoURL); err != nil {
		return err
	}
	if err := run("sparse-checkout", "init", "--cone"); err != nil {
		return err
	}
	if err := run("sparse-checkout", "set", pluginName); err != nil {
		return err
	}
	if err := run("fetch", "--depth", "1", "origin", ref); err != nil {
		return err
	}
	if err := run("checkout", "FETCH_HEAD"); err != nil {
		return err
	}

	srcDir := filepath.Join(tmpDir, pluginName)
	if _, statErr := os.Stat(filepath.Join(srcDir, "plugin.toml")); statErr != nil {
		// Backward-compat fallback while old layout may still exist.
		srcDir = filepath.Join(tmpDir, "plugins", pluginName)
	}
	if err := copyDir(srcDir, installDir); err != nil {
		return fmt.Errorf("failed to copy official plugin %q from repo: %w", pluginName, err)
	}
	return nil
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFileMode(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFileMode(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	st, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
