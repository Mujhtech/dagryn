package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/dagryn/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackend_Check(t *testing.T) {
	projectID := uuid.New()

	t.Run("hit", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/cache/build/abc123")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "cache hit"})
		}))
		defer srv.Close()

		b := newTestBackend(t, srv, projectID)
		hit, err := b.Check(context.Background(), "build", "abc123")
		require.NoError(t, err)
		assert.True(t, hit)
	})

	t.Run("miss", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}))
		defer srv.Close()

		b := newTestBackend(t, srv, projectID)
		hit, err := b.Check(context.Background(), "build", "abc123")
		require.NoError(t, err)
		assert.False(t, hit)
	})
}

func TestBackend_SaveAndRestore(t *testing.T) {
	projectID := uuid.New()

	// Use a shared variable to pass the archive between upload and download
	var stored []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT":
			data, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			stored = data
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "created"})

		case r.Method == "GET" && contains(r.URL.Path, "/download"):
			h := sha256.Sum256(stored)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Checksum-SHA256", hex.EncodeToString(h[:]))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(stored)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(stored)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Create source project with a file
	srcRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "dist"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "dist", "output.txt"), []byte("hello cloud"), 0644))

	b := newTestBackendWithRoot(t, srv, projectID, srcRoot)

	// Save
	err := b.Save(context.Background(), "build", "key1", []string{"dist/**"}, cache.Metadata{})
	require.NoError(t, err)
	require.NotEmpty(t, stored)

	// Restore into a fresh project root
	dstRoot := t.TempDir()
	b2 := newTestBackendWithRoot(t, srv, projectID, dstRoot)
	err = b2.Restore(context.Background(), "build", "key1")
	require.NoError(t, err)

	// Verify file was restored
	data, err := os.ReadFile(filepath.Join(dstRoot, "dist", "output.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello cloud", string(data))
}

func TestBackend_SaveNoPatterns(t *testing.T) {
	projectID := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any HTTP request when no patterns")
	}))
	defer srv.Close()

	b := newTestBackend(t, srv, projectID)
	err := b.Save(context.Background(), "build", "key1", nil, cache.Metadata{})
	require.NoError(t, err)
}

func TestBackend_RestoreChecksumMismatch(t *testing.T) {
	projectID := uuid.New()

	// Create a valid archive first via Save
	var stored []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT":
			data, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			stored = data
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "created"})

		case r.Method == "GET" && contains(r.URL.Path, "/download"):
			// Send a wrong checksum to simulate corruption
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Checksum-SHA256", "0000000000000000000000000000000000000000000000000000000000000000")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(stored)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(stored)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	srcRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "dist"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "dist", "output.txt"), []byte("test"), 0644))

	b := newTestBackendWithRoot(t, srv, projectID, srcRoot)
	err := b.Save(context.Background(), "build", "key1", []string{"dist/**"}, cache.Metadata{})
	require.NoError(t, err)

	dstRoot := t.TempDir()
	b2 := newTestBackendWithRoot(t, srv, projectID, dstRoot)
	err = b2.Restore(context.Background(), "build", "key1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloud cache integrity")
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestBackend_RestoreNoChecksumHeader(t *testing.T) {
	projectID := uuid.New()

	var stored []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT":
			data, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			stored = data
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "created"})

		case r.Method == "GET" && contains(r.URL.Path, "/download"):
			// No checksum headers — backward compatibility
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(stored)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	srcRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "dist"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "dist", "out.txt"), []byte("compat"), 0644))

	b := newTestBackendWithRoot(t, srv, projectID, srcRoot)
	err := b.Save(context.Background(), "build", "key1", []string{"dist/**"}, cache.Metadata{})
	require.NoError(t, err)

	dstRoot := t.TempDir()
	b2 := newTestBackendWithRoot(t, srv, projectID, dstRoot)
	err = b2.Restore(context.Background(), "build", "key1")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dstRoot, "dist", "out.txt"))
	require.NoError(t, err)
	assert.Equal(t, "compat", string(data))
}

func TestBackend_ClearNoop(t *testing.T) {
	b := &Backend{}
	assert.NoError(t, b.Clear(context.Background(), "task"))
	assert.NoError(t, b.ClearAll(context.Background()))
}

func newTestBackend(t *testing.T, srv *httptest.Server, projectID uuid.UUID) *Backend {
	t.Helper()
	return newTestBackendWithRoot(t, srv, projectID, t.TempDir())
}

func newTestBackendWithRoot(t *testing.T, srv *httptest.Server, projectID uuid.UUID, root string) *Backend {
	t.Helper()
	c := client.New(client.Config{BaseURL: srv.URL})
	return NewBackend(c, projectID, root)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s, substr))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
