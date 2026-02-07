package plugin

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// GitHubResolver resolves plugins from GitHub releases.
type GitHubResolver struct {
	client    *http.Client
	apiBase   string
	platform  Platform
	authToken string // Optional GitHub token for higher rate limits
}

// GitHubResolverOption is a functional option for configuring GitHubResolver.
type GitHubResolverOption func(*GitHubResolver)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) GitHubResolverOption {
	return func(r *GitHubResolver) {
		r.client = client
	}
}

// WithGitHubToken sets the GitHub API token for authentication.
func WithGitHubToken(token string) GitHubResolverOption {
	return func(r *GitHubResolver) {
		r.authToken = token
	}
}

// WithPlatform sets a custom platform (useful for testing).
func WithPlatform(platform Platform) GitHubResolverOption {
	return func(r *GitHubResolver) {
		r.platform = platform
	}
}

// NewGitHubResolver creates a new GitHub release resolver.
func NewGitHubResolver(opts ...GitHubResolverOption) *GitHubResolver {
	r := &GitHubResolver{
		client: &http.Client{
			Timeout: 5 * time.Minute, // Long timeout for large downloads
		},
		apiBase:  "https://api.github.com",
		platform: CurrentPlatform(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Name returns the resolver name.
func (r *GitHubResolver) Name() string {
	return "github"
}

// CanResolve returns true if this resolver can handle the plugin.
func (r *GitHubResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourceGitHub
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Prerelease  bool          `json:"prerelease"`
	Draft       bool          `json:"draft"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// Resolve resolves the plugin version and fetches its manifest if available.
func (r *GitHubResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	resolved := *plugin // Copy

	if plugin.Version == "latest" {
		// Fetch latest release
		release, err := r.getLatestRelease(ctx, plugin.Owner, plugin.Repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest release: %w", err)
		}
		resolved.ResolvedVersion = release.TagName
	} else if plugin.IsSemverRange() {
		// Fetch all releases and find best match
		releases, err := r.listReleases(ctx, plugin.Owner, plugin.Repo)
		if err != nil {
			return nil, fmt.Errorf("failed to list releases: %w", err)
		}
		version, err := r.matchSemverRange(plugin.Version, releases)
		if err != nil {
			return nil, fmt.Errorf("no matching version for %s: %w", plugin.Version, err)
		}
		resolved.ResolvedVersion = version
	} else {
		// Exact version
		resolved.ResolvedVersion = plugin.Version
	}

	// Try to fetch plugin.toml manifest from the repo
	tag := resolved.ResolvedVersion
	manifest, err := r.fetchManifest(ctx, plugin.Owner, plugin.Repo, tag)
	if err == nil && manifest != nil {
		resolved.Manifest = manifest
		// Use binary name from manifest if available
		if manifest.Tool.Binary != "" {
			resolved.BinaryName = manifest.Tool.Binary
		}
	}

	return &resolved, nil
}

// Install downloads and installs the plugin.
func (r *GitHubResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	result := &InstallResult{
		Plugin: plugin,
		Status: StatusInstalling,
	}

	// Fetch release to get assets
	version := plugin.ResolvedVersion
	if version == "" {
		version = plugin.Version
	}

	release, err := r.getRelease(ctx, plugin.Owner, plugin.Repo, version)
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to get release %s: %w", version, err)
		return result, result.Error
	}

	// Find appropriate asset for current platform
	var asset *GitHubAsset
	if plugin.Manifest != nil {
		// Use manifest platforms for deterministic asset selection
		asset, err = r.findAssetFromManifest(release.Assets, plugin.Manifest)
	}
	if asset == nil {
		// Fall back to heuristic scoring if no manifest or manifest didn't match
		asset, err = r.findAsset(release.Assets, plugin.BinaryName)
	}
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("no suitable asset found for %s: %w", r.platform.String(), err)
		return result, result.Error
	}

	// Create install directory
	if err := os.MkdirAll(installDir, 0755); err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to create install directory: %w", err)
		return result, result.Error
	}

	// Download asset
	tempFile, err := r.downloadAsset(ctx, asset)
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to download asset: %w", err)
		return result, result.Error
	}
	defer func() { _ = os.Remove(tempFile) }()

	// Extract or copy binary
	binaryPath, err := r.extractBinary(tempFile, asset.Name, plugin.BinaryName, installDir)
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to extract binary: %w", err)
		return result, result.Error
	}

	// Make binary executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("failed to make binary executable: %w", err)
		return result, result.Error
	}

	// Update plugin paths
	plugin.InstallPath = installDir
	plugin.BinaryPath = binaryPath

	result.Status = StatusInstalled
	result.Message = fmt.Sprintf("Installed %s %s from GitHub release", plugin.Name, version)
	return result, nil
}

// Verify checks if the plugin is correctly installed.
func (r *GitHubResolver) Verify(ctx context.Context, plugin *Plugin) error {
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

	// Check if executable
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable")
	}

	return nil
}

// getLatestRelease fetches the latest release from GitHub.
func (r *GitHubResolver) getLatestRelease(ctx context.Context, owner, repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", r.apiBase, owner, repo)
	return r.fetchRelease(ctx, url)
}

// getRelease fetches a specific release by tag.
func (r *GitHubResolver) getRelease(ctx context.Context, owner, repo, tag string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", r.apiBase, owner, repo, tag)
	return r.fetchRelease(ctx, url)
}

// listReleases fetches all releases from GitHub.
func (r *GitHubResolver) listReleases(ctx context.Context, owner, repo string) ([]GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", r.apiBase, owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	r.setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	return releases, nil
}

// fetchRelease fetches a release from a URL.
func (r *GitHubResolver) fetchRelease(ctx context.Context, url string) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	r.setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// setHeaders sets common headers for GitHub API requests.
func (r *GitHubResolver) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Dagryn/1.0")
	if r.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.authToken)
	}
}

// findAsset finds the best matching asset for the current platform.
func (r *GitHubResolver) findAsset(assets []GitHubAsset, binaryName string) (*GitHubAsset, error) {
	aliases := r.platform.PlatformAliases()
	extensions := r.platform.ArchiveExtensions()

	// Score each asset based on how well it matches
	type scoredAsset struct {
		asset *GitHubAsset
		score int
	}

	var scored []scoredAsset
	for i := range assets {
		asset := &assets[i]
		name := strings.ToLower(asset.Name)

		// Skip checksums and signatures
		if strings.HasSuffix(name, ".sha256") ||
			strings.HasSuffix(name, ".sha512") ||
			strings.HasSuffix(name, ".sig") ||
			strings.HasSuffix(name, ".asc") ||
			strings.HasSuffix(name, ".sbom") {
			continue
		}

		score := 0

		// Check for platform match
		for i, alias := range aliases {
			if strings.Contains(name, strings.ToLower(alias)) {
				// Earlier aliases are better matches
				score += 100 - i
				break
			}
		}

		// Check for known archive extension
		for _, ext := range extensions {
			if strings.HasSuffix(name, ext) {
				score += 50
				break
			}
		}

		// Bonus for containing binary name
		if strings.Contains(name, strings.ToLower(binaryName)) {
			score += 25
		}

		// Penalty for source archives
		if strings.Contains(name, "source") || strings.Contains(name, "src") {
			score -= 100
		}

		if score > 0 {
			scored = append(scored, scoredAsset{asset: asset, score: score})
		}
	}

	if len(scored) == 0 {
		return nil, fmt.Errorf("no matching asset found for platform %s", r.platform.String())
	}

	// Sort by score (highest first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return scored[0].asset, nil
}

// downloadAsset downloads an asset to a temporary file.
func (r *GitHubResolver) downloadAsset(ctx context.Context, asset *GitHubAsset) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}

	if r.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.authToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Create temp file with same extension as asset
	ext := filepath.Ext(asset.Name)
	if strings.Contains(asset.Name, ".tar.") {
		// Handle .tar.gz, .tar.xz, etc.
		ext = ".tar" + ext
	}

	tempFile, err := os.CreateTemp("", "dagryn-plugin-*"+ext)
	if err != nil {
		return "", err
	}
	defer func() { _ = tempFile.Close() }()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// extractBinary extracts the binary from an archive or copies it directly.
func (r *GitHubResolver) extractBinary(archivePath, archiveName, binaryName, destDir string) (string, error) {
	lowerName := strings.ToLower(archiveName)

	// Determine binary file name with platform extension
	targetBinary := binaryName + r.platform.BinaryExtension()
	destPath := filepath.Join(destDir, targetBinary)

	// Handle different archive types
	switch {
	case strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz"):
		return r.extractFromTarGz(archivePath, binaryName, destDir)
	case strings.HasSuffix(lowerName, ".zip"):
		return r.extractFromZip(archivePath, binaryName, destDir)
	case strings.HasSuffix(lowerName, r.platform.BinaryExtension()) || !strings.Contains(lowerName, "."):
		// Direct binary, just copy it
		if err := copyFile(archivePath, destPath); err != nil {
			return "", err
		}
		return destPath, nil
	default:
		// Try to extract as tar.gz first, then zip
		if path, err := r.extractFromTarGz(archivePath, binaryName, destDir); err == nil {
			return path, nil
		}
		if path, err := r.extractFromZip(archivePath, binaryName, destDir); err == nil {
			return path, nil
		}
		// Last resort: assume it's the binary itself
		if err := copyFile(archivePath, destPath); err != nil {
			return "", err
		}
		return destPath, nil
	}
}

// extractFromTarGz extracts a binary from a tar.gz archive.
func (r *GitHubResolver) extractFromTarGz(archivePath, binaryName, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	binaryPattern := regexp.MustCompile(fmt.Sprintf(`(?i)(^|/)%s(%s)?$`, regexp.QuoteMeta(binaryName), regexp.QuoteMeta(r.platform.BinaryExtension())))

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Check if this is the binary we're looking for
		if binaryPattern.MatchString(header.Name) {
			targetName := binaryName + r.platform.BinaryExtension()
			destPath := filepath.Join(destDir, targetName)

			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				_ = outFile.Close()
				return "", err
			}
			_ = outFile.Close()

			return destPath, nil
		}
	}

	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

// extractFromZip extracts a binary from a zip archive.
func (r *GitHubResolver) extractFromZip(archivePath, binaryName, destDir string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := zr.Close(); err != nil {
			log.Printf("Failed to close zip reader: %v", err)
		}
	}()

	binaryPattern := regexp.MustCompile(fmt.Sprintf(`(?i)(^|/)%s(%s)?$`, regexp.QuoteMeta(binaryName), regexp.QuoteMeta(r.platform.BinaryExtension())))

	for _, f := range zr.File {
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Check if this is the binary we're looking for
		if binaryPattern.MatchString(f.Name) {
			targetName := binaryName + r.platform.BinaryExtension()
			destPath := filepath.Join(destDir, targetName)

			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			defer func() {
				if err := rc.Close(); err != nil {
					log.Printf("Failed to close zip reader: %v", err)
				}
			}()

			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}

			defer func() {
				if err := outFile.Close(); err != nil {
					log.Printf("Failed to close destination file: %v", err)
				}
			}()

			if _, err := io.Copy(outFile, rc); err != nil {
				return "", err
			}

			return destPath, nil
		}
	}

	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

// fetchManifest fetches plugin.toml from the repository at the given tag.
func (r *GitHubResolver) fetchManifest(ctx context.Context, owner, repo, tag string) (*Manifest, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/plugin.toml", owner, repo, tag)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if r.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.authToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // No manifest, not an error
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch plugin.toml: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return ParseManifest(data)
}

// findAssetFromManifest finds the asset using the manifest's platform mappings.
func (r *GitHubResolver) findAssetFromManifest(assets []GitHubAsset, manifest *Manifest) (*GitHubAsset, error) {
	if manifest == nil || len(manifest.Platforms) == 0 {
		return nil, fmt.Errorf("manifest has no platform mappings")
	}

	// Build platform key: "os-arch"
	platformKey := fmt.Sprintf("%s-%s", r.platform.OS, r.platform.Arch)
	assetName := manifest.PlatformAsset(platformKey)
	if assetName == "" {
		return nil, fmt.Errorf("no platform mapping for %s in manifest", platformKey)
	}

	// Find the asset with that name
	for i := range assets {
		if assets[i].Name == assetName {
			return &assets[i], nil
		}
	}

	return nil, fmt.Errorf("asset %q from manifest not found in release assets", assetName)
}

// matchSemverRange finds the best matching version for a semver range.
func (r *GitHubResolver) matchSemverRange(constraint string, releases []GitHubRelease) (string, error) {
	// Parse constraint (^1.0.0 or ~1.0.0)
	prefix := constraint[0:1]
	versionStr := strings.TrimPrefix(constraint[1:], "v")
	parts := strings.Split(versionStr, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid semver constraint: %s", constraint)
	}

	var major, minor, patch int
	_, _ = fmt.Sscanf(parts[0], "%d", &major)
	_, _ = fmt.Sscanf(parts[1], "%d", &minor)
	_, _ = fmt.Sscanf(parts[2], "%d", &patch)

	// Filter valid releases
	var candidates []string
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		tag := strings.TrimPrefix(release.TagName, "v")
		tagParts := strings.Split(tag, ".")
		if len(tagParts) < 3 {
			continue
		}

		var tagMajor, tagMinor, tagPatch int
		_, _ = fmt.Sscanf(tagParts[0], "%d", &tagMajor)
		_, _ = fmt.Sscanf(tagParts[1], "%d", &tagMinor)
		// Handle pre-release suffixes like "1.55.2-rc1"
		patchStr := strings.Split(tagParts[2], "-")[0]
		_, _ = fmt.Sscanf(patchStr, "%d", &tagPatch)

		matches := false
		switch prefix {
		case "^":
			// ^1.2.3 allows >=1.2.3 and <2.0.0
			if tagMajor == major && (tagMinor > minor || (tagMinor == minor && tagPatch >= patch)) {
				matches = true
			}
		case "~":
			// ~1.2.3 allows >=1.2.3 and <1.3.0
			if tagMajor == major && tagMinor == minor && tagPatch >= patch {
				matches = true
			}
		}

		if matches {
			candidates = append(candidates, release.TagName)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no version matches constraint %s", constraint)
	}

	// Return the highest version (candidates are already sorted by GitHub)
	return candidates[0], nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			log.Printf("Failed to close destination file: %v", err)
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
