package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultOfficialPluginsRepo = "https://github.com/mujhtech/dagryn-plugins.git"
const defaultOfficialPluginsAPI = "https://api.github.com"

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
	apiBase     string
	client      *http.Client
}

func NewOfficialResolver(projectRoot string) *OfficialResolver {
	return &OfficialResolver{
		projectRoot: projectRoot,
		fallback:    NewLocalResolver(projectRoot),
		apiBase:     defaultOfficialPluginsAPI,
		client:      &http.Client{Timeout: 30 * time.Second},
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
		if p, err := r.resolveFromDir(ctx, &resolved, resolved.InstallPath); err == nil {
			return p, nil
		}
	}

	if envRoot := strings.TrimSpace(os.Getenv("DAGRYN_PLUGINS_DIR")); envRoot != "" {
		candidate := filepath.Join(envRoot, resolved.Name)
		if p, err := r.resolveFromDir(ctx, &resolved, candidate); err == nil {
			return p, nil
		}
	}

	// Manager will install on-demand via sparse checkout when not found locally.
	// Return a resolvable descriptor without requiring pre-existing local files.
	if resolved.Owner == "" {
		resolved.Owner = "dagryn"
	}
	if resolved.Repo == "" {
		resolved.Repo = resolved.Name
	}

	// Resolve semver-like refs for official plugins:
	// - latest -> main
	// - v1 -> highest v1.x.x tag
	// - ^1.2.0 / ~1.2.0 -> best matching tag
	if v, err := r.resolveOfficialVersion(ctx, resolved.Version); err == nil {
		resolved.ResolvedVersion = v
	} else {
		return nil, err
	}

	resolved.InstallPath = ""
	resolved.Manifest = nil
	resolved.Version = resolved.ResolvedVersion
	return &resolved, nil
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

	ref := plugin.ResolvedVersion
	if ref == "" {
		ref = plugin.Version
	}
	if ref == "" {
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

func (r *OfficialResolver) resolveOfficialVersion(ctx context.Context, version string) (string, error) {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		return "main", nil
	}

	if isMajorAlias(v) || strings.HasPrefix(v, "^") || strings.HasPrefix(v, "~") {
		tags, err := r.listOfficialTags(ctx)
		if err != nil {
			return "", err
		}
		resolved, err := resolveSemverConstraint(v, tags)
		if err != nil {
			return "", err
		}
		return resolved, nil
	}

	return v, nil
}

func isMajorAlias(v string) bool {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return false
	}
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

type githubRef struct {
	Ref string `json:"ref"`
}

func (r *OfficialResolver) listOfficialTags(ctx context.Context) ([]string, error) {
	repoURL := strings.TrimSpace(os.Getenv("DAGRYN_OFFICIAL_PLUGINS_REPO"))
	if repoURL == "" {
		repoURL = defaultOfficialPluginsRepo
	}
	owner, repo, err := parseGitHubRepo(repoURL)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/git/matching-refs/tags", r.apiBase, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list official plugin tags: status %d", resp.StatusCode)
	}

	var refs []githubRef
	if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
		return nil, err
	}

	tags := make([]string, 0, len(refs))
	for _, ref := range refs {
		const prefix = "refs/tags/"
		if strings.HasPrefix(ref.Ref, prefix) {
			tag := strings.TrimPrefix(ref.Ref, prefix)
			if isSemverTag(tag) {
				tags = append(tags, tag)
			}
		}
	}
	return tags, nil
}

func parseGitHubRepo(repoURL string) (owner, repo string, err error) {
	u := strings.TrimSpace(repoURL)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "git@")
	u = strings.TrimSuffix(u, ".git")
	u = strings.ReplaceAll(u, ":", "/")
	parts := strings.Split(u, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("invalid official plugins repo url: %s", repoURL)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}

func isSemverTag(tag string) bool {
	_, ok := parseSemverTag(tag)
	return ok
}

type semver struct{ major, minor, patch int }

func parseSemverTag(tag string) (semver, bool) {
	t := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	parts := strings.Split(t, ".")
	if len(parts) < 3 {
		return semver{}, false
	}
	patchPart := strings.Split(parts[2], "-")[0]
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	pat, err3 := strconv.Atoi(patchPart)
	if err1 != nil || err2 != nil || err3 != nil {
		return semver{}, false
	}
	return semver{major: maj, minor: min, patch: pat}, true
}

func resolveSemverConstraint(constraint string, tags []string) (string, error) {
	if len(tags) == 0 {
		return "", fmt.Errorf("no semver tags found for official plugins")
	}

	type candidate struct {
		tag string
		v   semver
	}
	cands := make([]candidate, 0, len(tags))
	for _, tag := range tags {
		if v, ok := parseSemverTag(tag); ok {
			cands = append(cands, candidate{tag: tag, v: v})
		}
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("no semver tags found for official plugins")
	}

	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i].v, cands[j].v
		if a.major != b.major {
			return a.major > b.major
		}
		if a.minor != b.minor {
			return a.minor > b.minor
		}
		return a.patch > b.patch
	})

	if isMajorAlias(constraint) {
		m, _ := strconv.Atoi(strings.TrimPrefix(constraint, "v"))
		for _, c := range cands {
			if c.v.major == m {
				return c.tag, nil
			}
		}
		return "", fmt.Errorf("no tag matches %s", constraint)
	}

	prefix := constraint[:1]
	base, ok := parseSemverTag(strings.TrimPrefix(constraint[1:], "v"))
	if !ok {
		return "", fmt.Errorf("invalid semver constraint: %s", constraint)
	}

	for _, c := range cands {
		switch prefix {
		case "^":
			if c.v.major == base.major && (c.v.minor > base.minor || (c.v.minor == base.minor && c.v.patch >= base.patch)) {
				return c.tag, nil
			}
		case "~":
			if c.v.major == base.major && c.v.minor == base.minor && c.v.patch >= base.patch {
				return c.tag, nil
			}
		}
	}

	return "", fmt.Errorf("no tag matches %s", constraint)
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
