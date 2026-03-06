package cloud

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mujhtech/dagryn/pkg/dagryn/cache"
)

// CreateArchive builds a tar.gz archive from files matching the output patterns
// rooted at projectRoot. Files whose relative paths fall under any of skipDirs
// are excluded from the archive.
func CreateArchive(projectRoot string, outputPatterns []string, skipDirs []string) (*os.File, error) {
	tmp, err := os.CreateTemp("", "dagryn-cache-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	gw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gw)

	files, err := cache.ResolveFilePatterns(projectRoot, outputPatterns)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("resolve output patterns: %w", err)
	}
	for _, absPath := range files {
		relPath, _ := filepath.Rel(projectRoot, absPath)
		if shouldSkip(relPath, skipDirs) {
			continue
		}

		// Use Lstat so symlinks are detected rather than followed.
		info, err := os.Lstat(absPath)
		if err != nil {
			continue
		}
		if err := AddFileToTar(tw, projectRoot, absPath, info); err != nil {
			continue
		}
	}

	if err := tw.Close(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}

	// Seek to beginning so it can be read
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("seek temp file: %w", err)
	}

	return tmp, nil
}

// AddFileToTar adds a single file or symlink to a tar writer using a path
// relative to projectRoot. Symlinks are stored as-is (not followed).
func AddFileToTar(tw *tar.Writer, projectRoot, absPath string, info os.FileInfo) error {
	relPath, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return err
	}

	// Read symlink target if this is a symlink.
	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(absPath)
		if err != nil {
			return err
		}
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	header.Name = relPath

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// Symlinks have no body to write.
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(tw, f)
	return err
}

// ExtractArchive reads a tar.gz stream and writes files to projectRoot.
// It validates paths to prevent directory traversal.
func ExtractArchive(projectRoot string, r io.Reader) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip reader: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		// Security: prevent directory traversal
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}

		target := filepath.Join(projectRoot, cleanName)
		// Double check the resolved path is within projectRoot
		if !strings.HasPrefix(target, filepath.Clean(projectRoot)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := mkdirAllReplacingSymlinks(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory %s: %w", cleanName, err)
			}
		case tar.TypeReg:
			if err := mkdirAllReplacingSymlinks(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent for %s: %w", cleanName, err)
			}
			// Remove any existing symlink at the target path before creating the file.
			_ = os.Remove(target)
			f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", cleanName, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("write file %s: %w", cleanName, err)
			}
			_ = f.Close()
		case tar.TypeSymlink:
			// Security: validate that the symlink target resolves within projectRoot.
			linkTarget := header.Linkname
			var resolved string
			if filepath.IsAbs(linkTarget) {
				resolved = filepath.Clean(linkTarget)
			} else {
				resolved = filepath.Clean(filepath.Join(filepath.Dir(target), linkTarget))
			}
			if !strings.HasPrefix(resolved, filepath.Clean(projectRoot)+string(os.PathSeparator)) {
				continue // skip symlinks that escape the project root
			}

			if err := mkdirAllReplacingSymlinks(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent for symlink %s: %w", cleanName, err)
			}
			// Remove any existing file/symlink before creating.
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("create symlink %s -> %s: %w", cleanName, header.Linkname, err)
			}
		}
	}

	return nil
}

// mkdirAllReplacingSymlinks works like os.MkdirAll but replaces any symlink
// that occupies a path component with a real directory when the symlink does
// not point to a valid directory. This is needed because pnpm's node_modules
// contains symlinks at paths that later entries in the tar archive expect to
// be directories (e.g. a symlink at "pkg-name" and a file at "pkg-name/LICENSE").
func mkdirAllReplacingSymlinks(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err == nil {
		return nil
	}
	// Walk each component and replace only symlinks whose targets are not
	// valid directories. System symlinks (e.g. /var -> /private/var on macOS)
	// point to real directories and must be left untouched.
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	current := ""
	if filepath.IsAbs(path) {
		current = string(filepath.Separator)
	}
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		fi, err := os.Lstat(current)
		if err != nil {
			// Doesn't exist yet — create the rest in one shot.
			return os.MkdirAll(path, perm)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			// Check if the symlink target is a directory — if so, leave it
			// (it behaves like a directory for MkdirAll purposes).
			if target, statErr := os.Stat(current); statErr == nil && target.IsDir() {
				continue
			}
			// Symlink to non-directory or broken — replace with real directory.
			if err := os.Remove(current); err != nil {
				return err
			}
			if err := os.Mkdir(current, perm); err != nil {
				return err
			}
		} else if !fi.IsDir() {
			// Regular file in the way — remove it.
			if err := os.Remove(current); err != nil {
				return err
			}
			if err := os.Mkdir(current, perm); err != nil {
				return err
			}
		}
	}
	return nil
}

// shouldSkip returns true if relPath falls inside any of the given skip directories.
func shouldSkip(relPath string, skipDirs []string) bool {
	for _, dir := range skipDirs {
		if relPath == dir || strings.HasPrefix(relPath, dir+string(filepath.Separator)) {
			return true
		}
		nested := string(filepath.Separator) + dir + string(filepath.Separator)
		if strings.Contains(relPath, nested) {
			return true
		}
	}
	return false
}
