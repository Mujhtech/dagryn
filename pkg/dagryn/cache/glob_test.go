package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveFilePatterns_DoublestarNestedGoFiles(t *testing.T) {
	root := t.TempDir()

	// Create nested directory structure
	for _, rel := range []string{
		"src/main.go",
		"src/pkg/util.go",
		"src/pkg/deep/nested.go",
		"src/readme.md",
	} {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0755))
		require.NoError(t, os.WriteFile(abs, []byte("x"), 0644))
	}

	files, err := ResolveFilePatterns(root, []string{"src/**/*.go"})
	require.NoError(t, err)

	assert.Len(t, files, 3)
	assert.Contains(t, files, filepath.Join(root, "src/main.go"))
	assert.Contains(t, files, filepath.Join(root, "src/pkg/util.go"))
	assert.Contains(t, files, filepath.Join(root, "src/pkg/deep/nested.go"))
	// Should NOT contain the .md file
	assert.NotContains(t, files, filepath.Join(root, "src/readme.md"))
}

func TestResolveFilePatterns_DoublestarAllFiles(t *testing.T) {
	root := t.TempDir()

	// Simulate node_modules-like deep nesting
	for _, rel := range []string{
		"node_modules/pkg-a/index.js",
		"node_modules/pkg-a/lib/core.js",
		"node_modules/pkg-b/index.js",
		"node_modules/pkg-b/node_modules/pkg-c/index.js",
	} {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0755))
		require.NoError(t, os.WriteFile(abs, []byte("x"), 0644))
	}

	files, err := ResolveFilePatterns(root, []string{"node_modules/**"})
	require.NoError(t, err)

	assert.Len(t, files, 4)
}

func TestResolveFilePatterns_StandardGlobExpandsDirs(t *testing.T) {
	root := t.TempDir()

	// dist/* should match both files and expand directories into their files
	for _, rel := range []string{
		"dist/index.html",
		"dist/assets/app.js",
		"dist/assets/style.css",
	} {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0755))
		require.NoError(t, os.WriteFile(abs, []byte("x"), 0644))
	}

	files, err := ResolveFilePatterns(root, []string{"dist/*"})
	require.NoError(t, err)

	assert.Len(t, files, 3)
	assert.Contains(t, files, filepath.Join(root, "dist/index.html"))
	assert.Contains(t, files, filepath.Join(root, "dist/assets/app.js"))
	assert.Contains(t, files, filepath.Join(root, "dist/assets/style.css"))
}

func TestResolveFilePatterns_DoublestarAloneMatchesAll(t *testing.T) {
	root := t.TempDir()

	for _, rel := range []string{
		"a.txt",
		"sub/b.txt",
		"sub/deep/c.txt",
	} {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0755))
		require.NoError(t, os.WriteFile(abs, []byte("x"), 0644))
	}

	files, err := ResolveFilePatterns(root, []string{"**"})
	require.NoError(t, err)

	assert.Len(t, files, 3)
}

func TestResolveFilePatterns_NoMatchReturnsEmpty(t *testing.T) {
	root := t.TempDir()

	files, err := ResolveFilePatterns(root, []string{"nonexistent/**/*.go"})
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestResolveFilePatterns_DeduplicationAcrossPatterns(t *testing.T) {
	root := t.TempDir()

	abs := filepath.Join(root, "src/main.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0755))
	require.NoError(t, os.WriteFile(abs, []byte("x"), 0644))

	// Two overlapping patterns that match the same file
	files, err := ResolveFilePatterns(root, []string{"src/**/*.go", "src/main.go"})
	require.NoError(t, err)

	assert.Len(t, files, 1)
	assert.Equal(t, filepath.Join(root, "src/main.go"), files[0])
}

func TestResolveFilePatterns_ResultsSorted(t *testing.T) {
	root := t.TempDir()

	for _, rel := range []string{"c.txt", "a.txt", "b.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0644))
	}

	files, err := ResolveFilePatterns(root, []string{"*.txt"})
	require.NoError(t, err)

	assert.Equal(t, []string{
		filepath.Join(root, "a.txt"),
		filepath.Join(root, "b.txt"),
		filepath.Join(root, "c.txt"),
	}, files)
}

func TestResolveFilePatterns_EmptyPatterns(t *testing.T) {
	root := t.TempDir()

	files, err := ResolveFilePatterns(root, nil)
	require.NoError(t, err)
	assert.Empty(t, files)
}
