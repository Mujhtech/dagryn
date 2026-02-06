package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mujhtech/dagryn/internal/task"
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

	cache := New(tmpDir, true)

	task := &task.Task{
		Name:    "build",
		Command: "cat input.txt > output.txt",
		Inputs:  []string{"input.txt"},
		Outputs: []string{"output.txt"},
	}

	// First check - cache miss
	hit, key, err := cache.Check(task)
	require.NoError(t, err)
	assert.False(t, hit)
	assert.NotEmpty(t, key)

	// Save to cache
	err = cache.Save(task, key, time.Second)
	require.NoError(t, err)

	// Second check - cache hit
	hit, key2, err := cache.Check(task)
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

	cache := New(tmpDir, false)

	task := &task.Task{
		Name:    "build",
		Command: "echo hello",
		Inputs:  []string{"*.txt"},
		Outputs: []string{"output.txt"},
	}

	// Check always returns miss when disabled
	hit, key, err := cache.Check(task)
	require.NoError(t, err)
	assert.False(t, hit)
	assert.Empty(t, key)

	assert.False(t, cache.IsEnabled())
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

	cache := New(tmpDir, true)

	// Task without inputs/outputs should not be cached
	task := &task.Task{
		Name:    "clean",
		Command: "rm -rf dist",
	}

	hit, key, err := cache.Check(task)
	require.NoError(t, err)
	assert.False(t, hit)
	assert.Empty(t, key)
}
