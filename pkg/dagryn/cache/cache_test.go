package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mujhtech/dagryn/pkg/dagryn/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashTask(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	task1 := &task.Task{
		Name:    "build",
		Command: "go build",
		Env:     map[string]string{"GO111MODULE": "on"},
	}

	task2 := &task.Task{
		Name:    "build",
		Command: "go build",
		Env:     map[string]string{"GO111MODULE": "on"},
	}

	task3 := &task.Task{
		Name:    "build",
		Command: "go build -v", // Different command
		Env:     map[string]string{"GO111MODULE": "on"},
	}

	hash1, err := HashTask(task1, tmpDir)
	require.NoError(t, err)

	hash2, err := HashTask(task2, tmpDir)
	require.NoError(t, err)

	hash3, err := HashTask(task3, tmpDir)
	require.NoError(t, err)

	// Same task should produce same hash
	assert.Equal(t, hash1, hash2)

	// Different command should produce different hash
	assert.NotEqual(t, hash1, hash3)
}

func TestHashTask_WithInputFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files
	err = os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)

	task1 := &task.Task{
		Name:    "build",
		Command: "cat file1.txt",
		Inputs:  []string{"file1.txt"},
	}

	hash1, err := HashTask(task1, tmpDir)
	require.NoError(t, err)

	// Modify file
	err = os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	hash2, err := HashTask(task1, tmpDir)
	require.NoError(t, err)

	// Hash should change when file content changes
	assert.NotEqual(t, hash1, hash2)
}

func TestHashFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	// Create test files
	err = os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	hash, err := HashFiles([]string{"*.txt"}, tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestHashString(t *testing.T) {
	hash1 := HashString("hello")
	hash2 := HashString("hello")
	hash3 := HashString("world")

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
	assert.Len(t, hash1, 64) // SHA256 hex string
}

func TestStore_SaveAndRestore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	// Create output file
	outputDir := filepath.Join(tmpDir, "dist")
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(outputDir, "output.txt"), []byte("output content"), 0644)
	require.NoError(t, err)

	store := NewStore(tmpDir)

	// Save cache
	meta := Metadata{
		TaskName:  "build",
		CacheKey:  "abc123",
		CreatedAt: time.Now(),
		Duration:  5 * time.Second,
	}
	err = store.Save("build", "abc123", []string{"dist/*"}, meta)
	require.NoError(t, err)

	// Check cache exists
	assert.True(t, store.Exists("build", "abc123"))

	// Delete original output
	err = os.RemoveAll(outputDir)
	require.NoError(t, err)

	// Restore cache
	err = store.Restore("build", "abc123")
	require.NoError(t, err)

	// Check output was restored
	content, err := os.ReadFile(filepath.Join(tmpDir, "dist", "output.txt"))
	require.NoError(t, err)
	assert.Equal(t, "output content", string(content))
}

func TestStore_GetMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	store := NewStore(tmpDir)

	// Save with metadata
	meta := Metadata{
		TaskName:  "build",
		CacheKey:  "abc123",
		CreatedAt: time.Now(),
		Duration:  5 * time.Second,
	}
	err = store.Save("build", "abc123", []string{}, meta)
	require.NoError(t, err)

	// Get metadata
	retrieved, err := store.GetMetadata("build", "abc123")
	require.NoError(t, err)
	assert.Equal(t, "build", retrieved.TaskName)
	assert.Equal(t, "abc123", retrieved.CacheKey)
	assert.Equal(t, 5*time.Second, retrieved.Duration)
}

func TestStore_Clear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	store := NewStore(tmpDir)

	// Save cache
	meta := Metadata{TaskName: "build", CacheKey: "abc123"}
	err = store.Save("build", "abc123", []string{}, meta)
	require.NoError(t, err)

	assert.True(t, store.Exists("build", "abc123"))

	// Clear
	err = store.Clear("build")
	require.NoError(t, err)

	assert.False(t, store.Exists("build", "abc123"))
}

func TestCache_CheckAndSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	// Create input and output files
	err = os.WriteFile(filepath.Join(tmpDir, "input.txt"), []byte("input"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "output.txt"), []byte("output"), 0644)
	require.NoError(t, err)

	c := New(tmpDir, true)
	ctx := context.Background()

	tk := &task.Task{
		Name:    "build",
		Command: "cat input.txt > output.txt",
		Inputs:  []string{"input.txt"},
		Outputs: []string{"output.txt"},
	}

	// First check - cache miss
	hit, key, err := c.Check(ctx, tk)
	require.NoError(t, err)
	assert.False(t, hit)
	assert.NotEmpty(t, key)

	// Save to cache
	err = c.Save(ctx, tk, key, time.Second)
	require.NoError(t, err)

	// Second check - cache hit
	hit, key2, err := c.Check(ctx, tk)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, key, key2)
}

func TestCache_Disabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	c := New(tmpDir, false)
	ctx := context.Background()

	tk := &task.Task{
		Name:    "build",
		Command: "echo hello",
		Inputs:  []string{"*.txt"},
		Outputs: []string{"output.txt"},
	}

	// Check always returns miss when disabled
	hit, key, err := c.Check(ctx, tk)
	require.NoError(t, err)
	assert.False(t, hit)
	assert.Empty(t, key)

	assert.False(t, c.IsEnabled())
}

func TestHashTask_WithWorkdir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files under a subdirectory (simulating workdir)
	webDir := filepath.Join(tmpDir, "web")
	require.NoError(t, os.MkdirAll(webDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{"name":"test"}`), 0644))

	tk := &task.Task{
		Name:    "web-install",
		Command: "pnpm install",
		Workdir: "web",
		Inputs:  []string{"package.json"},
	}

	hash1, err := HashTask(tk, tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)

	// Modify the file — hash should change
	require.NoError(t, os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{"name":"changed"}`), 0644))
	hash2, err := HashTask(tk, tmpDir)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "hash should change when workdir file changes")

	// Same task without workdir should not match workdir file at project root
	tkNoWd := &task.Task{
		Name:    "web-install",
		Command: "pnpm install",
		Inputs:  []string{"package.json"},
	}
	hashNoWd, err := HashTask(tkNoWd, tmpDir)
	require.NoError(t, err)
	assert.NotEqual(t, hash2, hashNoWd, "workdir and non-workdir should produce different hashes")
}

func TestCache_SaveAndRestore_WithWorkdir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create output files under workdir subdirectory
	webDir := filepath.Join(tmpDir, "web")
	distDir := filepath.Join(webDir, "dist")
	require.NoError(t, os.MkdirAll(distDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "bundle.js"), []byte("console.log('hi')"), 0644))

	c := New(tmpDir, true)
	ctx := context.Background()

	tk := &task.Task{
		Name:    "web-build",
		Command: "pnpm build",
		Workdir: "web",
		Inputs:  []string{"package.json"},
		Outputs: []string{"dist/*"},
	}

	// Save to cache — outputs should resolve as web/dist/* (not just dist/*)
	err := c.Save(ctx, tk, "testkey", time.Second)
	require.NoError(t, err)

	// Verify cache entry exists
	assert.True(t, c.store.Exists("web-build", "testkey"))

	// Delete original outputs
	require.NoError(t, os.RemoveAll(distDir))

	// Restore from cache
	err = c.Restore(ctx, tk, "testkey")
	require.NoError(t, err)

	// Verify file was restored under web/dist/ (not at project root dist/)
	content, err := os.ReadFile(filepath.Join(webDir, "dist", "bundle.js"))
	require.NoError(t, err)
	assert.Equal(t, "console.log('hi')", string(content))

	// Verify file was NOT restored at project root
	_, err = os.Stat(filepath.Join(tmpDir, "dist", "bundle.js"))
	assert.True(t, os.IsNotExist(err), "output should not be restored at project root")
}

func TestStore_SaveAndRestore_WithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate a web build with dist/ containing files and an assets/ subdirectory.
	distDir := filepath.Join(tmpDir, "dist")
	assetsDir := filepath.Join(distDir, "assets")
	require.NoError(t, os.MkdirAll(assetsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<html></html>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("console.log('app')"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "style.css"), []byte("body{}"), 0644))

	store := NewStore(tmpDir)
	meta := Metadata{TaskName: "web-build", CacheKey: "dir-test"}

	// Save cache — dist/* matches index.html AND the assets/ directory.
	err := store.Save("web-build", "dir-test", []string{"dist/*"}, meta)
	require.NoError(t, err)

	// Delete original outputs
	require.NoError(t, os.RemoveAll(distDir))

	// Restore cache
	err = store.Restore("web-build", "dir-test")
	require.NoError(t, err)

	// Verify top-level file was restored
	content, err := os.ReadFile(filepath.Join(tmpDir, "dist", "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "<html></html>", string(content))

	// Verify assets directory was restored as a directory, not a file
	info, err := os.Stat(filepath.Join(tmpDir, "dist", "assets"))
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "assets should be restored as a directory, not a file")

	// Verify files inside assets/ were restored
	jsContent, err := os.ReadFile(filepath.Join(tmpDir, "dist", "assets", "app.js"))
	require.NoError(t, err)
	assert.Equal(t, "console.log('app')", string(jsContent))

	cssContent, err := os.ReadFile(filepath.Join(tmpDir, "dist", "assets", "style.css"))
	require.NoError(t, err)
	assert.Equal(t, "body{}", string(cssContent))
}

func TestCache_NoInputsOrOutputs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test")
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			t.Errorf("error removing temp dir: %v", err)
		}
	}()

	c := New(tmpDir, true)
	ctx := context.Background()

	// Task without inputs/outputs should not be cached
	tk := &task.Task{
		Name:    "clean",
		Command: "rm -rf dist",
	}

	hit, key, err := c.Check(ctx, tk)
	require.NoError(t, err)
	assert.False(t, hit)
	assert.Empty(t, key)
}

func TestStore_SaveAndRestore_DoublestarGlob(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate node_modules with deep nesting
	for _, rel := range []string{
		"node_modules/pkg-a/index.js",
		"node_modules/pkg-a/lib/core.js",
		"node_modules/pkg-b/index.js",
		"node_modules/pkg-b/node_modules/pkg-c/index.js",
	} {
		abs := filepath.Join(tmpDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0755))
		require.NoError(t, os.WriteFile(abs, []byte("module.exports={}"), 0644))
	}

	store := NewStore(tmpDir)
	meta := Metadata{TaskName: "install", CacheKey: "ds-test"}

	// Save with ** pattern
	err := store.Save("install", "ds-test", []string{"node_modules/**"}, meta)
	require.NoError(t, err)

	// Verify metadata includes all files
	retrieved, err := store.GetMetadata("install", "ds-test")
	require.NoError(t, err)
	assert.Len(t, retrieved.Outputs, 4, "all 4 deeply nested files should be saved")

	// Delete originals and restore
	require.NoError(t, os.RemoveAll(filepath.Join(tmpDir, "node_modules")))
	require.NoError(t, store.Restore("install", "ds-test"))

	// Verify all files restored
	for _, rel := range []string{
		"node_modules/pkg-a/index.js",
		"node_modules/pkg-a/lib/core.js",
		"node_modules/pkg-b/index.js",
		"node_modules/pkg-b/node_modules/pkg-c/index.js",
	} {
		content, err := os.ReadFile(filepath.Join(tmpDir, rel))
		require.NoError(t, err, "file %s should be restored", rel)
		assert.Equal(t, "module.exports={}", string(content))
	}
}

func TestHashFiles_DoublestarRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested Go source files
	for _, rel := range []string{
		"src/main.go",
		"src/pkg/util.go",
		"src/pkg/deep/nested.go",
	} {
		abs := filepath.Join(tmpDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0755))
		require.NoError(t, os.WriteFile(abs, []byte("package main"), 0644))
	}
	// Non-.go file that should be excluded
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src/readme.md"), []byte("# readme"), 0644))

	hash1, err := HashFiles([]string{"src/**/*.go"}, tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)

	// Modify a deeply nested file — hash should change
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src/pkg/deep/nested.go"), []byte("package changed"), 0644))
	hash2, err := HashFiles([]string{"src/**/*.go"}, tmpDir)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "hash should change when nested file changes")

	// Modifying the .md file should NOT change the hash
	hash3, err := HashFiles([]string{"src/**/*.go"}, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, hash2, hash3, "non-.go file changes should not affect hash")
}
