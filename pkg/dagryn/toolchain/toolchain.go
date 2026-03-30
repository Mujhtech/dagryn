package toolchain

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Runtime identifies a language runtime/toolchain that can be bootstrapped.
type Runtime string

const (
	RuntimeGo   Runtime = "go"
	RuntimeNode Runtime = "node"
)

const (
	defaultGoVersion   = "1.25.1"
	defaultNodeVersion = "20.20.0"
)

// Toolchain describes a resolved runtime environment.
type Toolchain struct {
	Runtime Runtime
	Version string
	BinDir  string
	Env     map[string]string
	System  bool
}

var installLocks sync.Map // map[string]*sync.Mutex

// DetectRuntimes infers required runtimes from a shell command string.
func DetectRuntimes(command string) []Runtime {
	cmd := strings.ToLower(command)
	needsGo := containsWord(cmd, "go") || containsWord(cmd, "gofmt")
	needsNode := containsWord(cmd, "node") ||
		containsWord(cmd, "npm") ||
		containsWord(cmd, "npx") ||
		containsWord(cmd, "pnpm") ||
		containsWord(cmd, "yarn")

	runtimes := make([]Runtime, 0, 2)
	if needsGo {
		runtimes = append(runtimes, RuntimeGo)
	}
	if needsNode {
		runtimes = append(runtimes, RuntimeNode)
	}
	return runtimes
}

// EnsureRuntime ensures a runtime is available using the current process PATH.
func EnsureRuntime(ctx context.Context, rt Runtime) (*Toolchain, error) {
	return EnsureRuntimeForPath(ctx, rt, os.Getenv("PATH"))
}

// EnsureRuntimeForPath ensures a runtime is available for the provided PATH.
func EnsureRuntimeForPath(ctx context.Context, rt Runtime, path string) (*Toolchain, error) {
	switch rt {
	case RuntimeGo:
		return ensureGo(ctx, path)
	case RuntimeNode:
		return ensureNode(ctx, path)
	default:
		return nil, fmt.Errorf("unsupported runtime %q", rt)
	}
}

// CommandExistsInPath returns true when command is found on the given PATH.
func CommandExistsInPath(command, path string) bool {
	_, ok := findExecutableInPath(command, path)
	return ok
}

// PrependPath prepends a path entry to a PATH-like string.
func PrependPath(prefix, current string) string {
	if prefix == "" {
		return current
	}
	if current == "" {
		return prefix
	}
	return prefix + string(filepath.ListSeparator) + current
}

func ensureGo(ctx context.Context, path string) (*Toolchain, error) {
	if bin, ok := findExecutableInPath("go", path); ok {
		return &Toolchain{
			Runtime: RuntimeGo,
			BinDir:  filepath.Dir(bin),
			System:  true,
		}, nil
	}
	if autoBootstrapDisabled() {
		return nil, fmt.Errorf("go binary not found and auto toolchain bootstrap is disabled")
	}

	platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	installDir := filepath.Join(toolchainRootDir(), "go", defaultGoVersion, platformKey)
	goRoot := filepath.Join(installDir, "go")
	goBinDir := filepath.Join(goRoot, "bin")
	goBinary := filepath.Join(goBinDir, binaryName("go"))

	lockKey := "go:" + defaultGoVersion + ":" + platformKey
	if err := withInstallLock(lockKey, func() error {
		if isExecutable(goBinary) {
			return nil
		}
		if err := installGo(ctx, installDir, defaultGoVersion); err != nil {
			return err
		}
		if !isExecutable(goBinary) {
			return fmt.Errorf("go bootstrap completed but %s is missing", goBinary)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &Toolchain{
		Runtime: RuntimeGo,
		Version: defaultGoVersion,
		BinDir:  goBinDir,
		Env: map[string]string{
			"GOROOT": goRoot,
		},
	}, nil
}

func ensureNode(ctx context.Context, path string) (*Toolchain, error) {
	if bin, ok := findExecutableInPath("node", path); ok {
		return &Toolchain{
			Runtime: RuntimeNode,
			BinDir:  filepath.Dir(bin),
			System:  true,
		}, nil
	}
	if autoBootstrapDisabled() {
		return nil, fmt.Errorf("node binary not found and auto toolchain bootstrap is disabled")
	}
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("automatic node bootstrap is not supported on windows")
	}

	nodeArch, err := nodeArchiveArch(runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	libc := detectLinuxLibc()
	platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, nodeArch)
	if libc != "" {
		platformKey += "-" + libc
	}

	installDir := filepath.Join(toolchainRootDir(), "node", defaultNodeVersion, platformKey)
	nodeBinary := filepath.Join(installDir, "bin", binaryName("node"))

	lockKey := "node:" + defaultNodeVersion + ":" + platformKey
	if err := withInstallLock(lockKey, func() error {
		if isExecutable(nodeBinary) {
			return nil
		}
		if err := installNode(ctx, installDir, defaultNodeVersion, nodeArch, libc); err != nil {
			return err
		}
		if !isExecutable(nodeBinary) {
			return fmt.Errorf("node bootstrap completed but %s is missing", nodeBinary)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &Toolchain{
		Runtime: RuntimeNode,
		Version: defaultNodeVersion,
		BinDir:  filepath.Join(installDir, "bin"),
	}, nil
}

func installGo(ctx context.Context, installDir, version string) error {
	archiveArch, err := goArchiveArch(runtime.GOARCH)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(installDir), 0755); err != nil {
		return fmt.Errorf("create go toolchain dir: %w", err)
	}

	tarball := fmt.Sprintf("go%s.%s-%s.tar.gz", version, runtime.GOOS, archiveArch)
	url := "https://go.dev/dl/" + tarball

	tmpArchive, err := os.CreateTemp("", "dagryn-go-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp archive: %w", err)
	}
	tmpArchivePath := tmpArchive.Name()
	_ = tmpArchive.Close()
	defer func() { _ = os.Remove(tmpArchivePath) }()

	if err := downloadFile(ctx, url, tmpArchivePath); err != nil {
		return fmt.Errorf("download go toolchain: %w", err)
	}

	staging := installDir + ".tmp"
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0755); err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	if err := extractTarGz(tmpArchivePath, staging); err != nil {
		return fmt.Errorf("extract go toolchain: %w", err)
	}

	_ = os.RemoveAll(installDir)
	if err := os.Rename(staging, installDir); err != nil {
		return fmt.Errorf("promote go toolchain: %w", err)
	}
	return nil
}

func installNode(ctx context.Context, installDir, version, nodeArch, libc string) error {
	if err := os.MkdirAll(filepath.Dir(installDir), 0755); err != nil {
		return fmt.Errorf("create node toolchain dir: %w", err)
	}

	url := nodeDownloadURL(version, runtime.GOOS, nodeArch, libc)
	if url == "" {
		return fmt.Errorf("unsupported node platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	tmpArchive, err := os.CreateTemp("", "dagryn-node-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp archive: %w", err)
	}
	tmpArchivePath := tmpArchive.Name()
	_ = tmpArchive.Close()
	defer func() { _ = os.Remove(tmpArchivePath) }()

	if err := downloadFile(ctx, url, tmpArchivePath); err != nil {
		return fmt.Errorf("download node toolchain: %w", err)
	}

	staging := installDir + ".tmp"
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0755); err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	if err := extractTarGz(tmpArchivePath, staging); err != nil {
		return fmt.Errorf("extract node toolchain: %w", err)
	}

	rootDir, err := findNodeRootDir(staging)
	if err != nil {
		return err
	}

	_ = os.RemoveAll(installDir)
	if err := os.Rename(rootDir, installDir); err != nil {
		return fmt.Errorf("promote node toolchain: %w", err)
	}
	return nil
}

func findNodeRootDir(staging string) (string, error) {
	if isExecutable(filepath.Join(staging, "bin", binaryName("node"))) {
		return staging, nil
	}

	entries, err := os.ReadDir(staging)
	if err != nil {
		return "", fmt.Errorf("read node staging dir: %w", err)
	}

	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(staging, entry.Name())
		if isExecutable(filepath.Join(candidate, "bin", binaryName("node"))) {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("node binary not found after extraction")
	}
	sort.Strings(candidates)
	return candidates[0], nil
}

func withInstallLock(key string, fn func() error) error {
	lockAny, _ := installLocks.LoadOrStore(key, &sync.Mutex{})
	lock := lockAny.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

func autoBootstrapDisabled() bool {
	return os.Getenv("DAGRYN_DISABLE_AUTO_TOOLCHAIN") == "1"
}

func toolchainRootDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "dagryn-toolchains")
	}
	return filepath.Join(home, ".dagryn", "toolchains")
}

func binaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func nodeDownloadURL(version, goos, arch, libc string) string {
	switch goos {
	case "linux":
		if libc == "musl" {
			return fmt.Sprintf(
				"https://unofficial-builds.nodejs.org/download/release/v%s/node-v%s-linux-%s-musl.tar.gz",
				version, version, arch,
			)
		}
		return fmt.Sprintf(
			"https://nodejs.org/dist/v%s/node-v%s-linux-%s.tar.gz",
			version, version, arch,
		)
	case "darwin":
		return fmt.Sprintf(
			"https://nodejs.org/dist/v%s/node-v%s-darwin-%s.tar.gz",
			version, version, arch,
		)
	default:
		return ""
	}
}

func goArchiveArch(goarch string) (string, error) {
	switch goarch {
	case "amd64", "arm64", "386":
		return goarch, nil
	default:
		return "", fmt.Errorf("unsupported go architecture %q", goarch)
	}
}

func nodeArchiveArch(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "x64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported node architecture %q", goarch)
	}
}

func detectLinuxLibc() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return "musl"
	}

	out, err := exec.Command("sh", "-c", "ldd --version 2>&1").Output()
	if err != nil {
		return "glibc"
	}
	if strings.Contains(strings.ToLower(string(out)), "musl") {
		return "musl"
	}
	return "glibc"
}

func downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func extractTarGz(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dest, hdr.Name)
		cleanDest := filepath.Clean(dest) + string(filepath.Separator)
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, cleanDest) && cleanTarget != filepath.Clean(dest) {
			return fmt.Errorf("archive path escapes destination: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return err
			}
			_ = os.Remove(cleanTarget)
			if err := os.Symlink(hdr.Linkname, cleanTarget); err != nil {
				return err
			}
		}
	}
	return nil
}

func findExecutableInPath(command, path string) (string, bool) {
	if command == "" {
		return "", false
	}
	if strings.ContainsRune(command, filepath.Separator) {
		if isExecutable(command) {
			return command, true
		}
		return "", false
	}

	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, binaryName(command))
		if isExecutable(candidate) {
			return candidate, true
		}
		// Handle command names that already include extension (windows safety).
		candidate = filepath.Join(dir, command)
		if isExecutable(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0111 != 0
}

func containsWord(s, word string) bool {
	for i := 0; i < len(s); i++ {
		if !isWordBoundary(s, i-1) {
			continue
		}
		if !strings.HasPrefix(s[i:], word) {
			continue
		}
		j := i + len(word)
		if isWordBoundary(s, j) {
			return true
		}
	}
	return false
}

func isWordBoundary(s string, idx int) bool {
	if idx < 0 || idx >= len(s) {
		return true
	}
	c := s[idx]
	return !((c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_' || c == '-')
}
