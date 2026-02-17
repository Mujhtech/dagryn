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

		info, err := os.Stat(absPath)
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

// AddFileToTar adds a single file to a tar writer using a path relative to projectRoot.
func AddFileToTar(tw *tar.Writer, projectRoot, absPath string, info os.FileInfo) error {
	relPath, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = relPath

	if err := tw.WriteHeader(header); err != nil {
		return err
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
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory %s: %w", cleanName, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent for %s: %w", cleanName, err)
			}
			f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", cleanName, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("write file %s: %w", cleanName, err)
			}
			_ = f.Close()
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
