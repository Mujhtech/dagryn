package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend is a test double for Backend.
type mockBackend struct {
	entries    map[string]bool // "taskName/key" → exists
	saveErr    error
	checkErr   error
	restoreErr error
	saveCalls  int
}

func newMockBackend() *mockBackend {
	return &mockBackend{entries: make(map[string]bool)}
}

func entryKey(taskName, key string) string {
	return taskName + "/" + key
}

func (m *mockBackend) Check(_ context.Context, taskName, key string) (bool, error) {
	if m.checkErr != nil {
		return false, m.checkErr
	}
	return m.entries[entryKey(taskName, key)], nil
}

func (m *mockBackend) Restore(_ context.Context, taskName, key string) error {
	if m.restoreErr != nil {
		return m.restoreErr
	}
	if !m.entries[entryKey(taskName, key)] {
		return fmt.Errorf("not found")
	}
	return nil
}

func (m *mockBackend) Save(_ context.Context, taskName, key string, _ []string, _ Metadata) error {
	m.saveCalls++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.entries[entryKey(taskName, key)] = true
	return nil
}

func (m *mockBackend) Clear(_ context.Context, taskName string) error {
	for k := range m.entries {
		if len(k) > len(taskName) && k[:len(taskName)+1] == taskName+"/" {
			delete(m.entries, k)
		}
	}
	return nil
}

func (m *mockBackend) ClearAll(_ context.Context) error {
	m.entries = make(map[string]bool)
	return nil
}

// --- local-first strategy tests ---

func TestHybridBackend_CheckLocalHit(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	local.entries[entryKey("build", "k1")] = true

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.True(t, hit)
}

func TestHybridBackend_CheckRemoteHit(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.entries[entryKey("build", "k1")] = true

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.True(t, hit)
}

func TestHybridBackend_CheckMiss(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestHybridBackend_CheckRemoteError_Fallback(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.checkErr = fmt.Errorf("network timeout")

	cfg := DefaultHybridConfig()
	cfg.FallbackOnError = true
	h := NewHybridBackend(local, remote, cfg)

	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestHybridBackend_CheckRemoteError_NoFallback(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.checkErr = fmt.Errorf("network timeout")

	cfg := DefaultHybridConfig()
	cfg.FallbackOnError = false
	h := NewHybridBackend(local, remote, cfg)

	_, err := h.Check(context.Background(), "build", "k1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network timeout")
}

func TestHybridBackend_SaveBoth(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	err := h.Save(context.Background(), "build", "k1", nil, Metadata{})
	require.NoError(t, err)

	assert.True(t, local.entries[entryKey("build", "k1")])
	assert.True(t, remote.entries[entryKey("build", "k1")])
}

func TestHybridBackend_SaveRemoteError_Fallback(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.saveErr = fmt.Errorf("s3 error")

	cfg := DefaultHybridConfig()
	cfg.FallbackOnError = true
	h := NewHybridBackend(local, remote, cfg)

	err := h.Save(context.Background(), "build", "k1", nil, Metadata{})
	require.NoError(t, err)

	// Local should still be saved
	assert.True(t, local.entries[entryKey("build", "k1")])
}

func TestHybridBackend_RestoreLocalFirst(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	local.entries[entryKey("build", "k1")] = true

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	err := h.Restore(context.Background(), "build", "k1")
	require.NoError(t, err)
}

func TestHybridBackend_RestoreFromRemote(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.entries[entryKey("build", "k1")] = true

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	err := h.Restore(context.Background(), "build", "k1")
	require.NoError(t, err)
}

func TestHybridBackend_RestoreFromRemote_WarmsLocal(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.entries[entryKey("build", "k1")] = true

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	err := h.Restore(context.Background(), "build", "k1")
	require.NoError(t, err)

	// Local should now have the entry (warmed)
	assert.True(t, local.entries[entryKey("build", "k1")])
	assert.Equal(t, 1, local.saveCalls, "local.Save should be called once to warm cache")
}

func TestHybridBackend_ClearBoth(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	local.entries[entryKey("build", "k1")] = true
	remote.entries[entryKey("build", "k1")] = true

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	err := h.Clear(context.Background(), "build")
	require.NoError(t, err)

	assert.False(t, local.entries[entryKey("build", "k1")])
	assert.False(t, remote.entries[entryKey("build", "k1")])
}

func TestHybridBackend_ClearAllBoth(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	local.entries[entryKey("build", "k1")] = true
	remote.entries[entryKey("test", "k2")] = true

	h := NewHybridBackend(local, remote, DefaultHybridConfig())
	err := h.ClearAll(context.Background())
	require.NoError(t, err)

	assert.Len(t, local.entries, 0)
	assert.Len(t, remote.entries, 0)
}

// --- remote-first strategy tests ---

func TestHybridBackend_RemoteFirst_CheckRemoteHit(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.entries[entryKey("build", "k1")] = true

	cfg := HybridConfig{Strategy: StrategyRemoteFirst, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.True(t, hit)
}

func TestHybridBackend_RemoteFirst_CheckFallsBackToLocal(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	local.entries[entryKey("build", "k1")] = true
	// remote has nothing

	cfg := HybridConfig{Strategy: StrategyRemoteFirst, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.True(t, hit)
}

func TestHybridBackend_RemoteFirst_CheckRemoteError_FallsBackToLocal(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.checkErr = fmt.Errorf("timeout")
	local.entries[entryKey("build", "k1")] = true

	cfg := HybridConfig{Strategy: StrategyRemoteFirst, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.True(t, hit)
}

func TestHybridBackend_RemoteFirst_RestoreFromRemote(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.entries[entryKey("build", "k1")] = true

	cfg := HybridConfig{Strategy: StrategyRemoteFirst, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	err := h.Restore(context.Background(), "build", "k1")
	require.NoError(t, err)

	// Should warm local
	assert.True(t, local.entries[entryKey("build", "k1")])
}

func TestHybridBackend_RemoteFirst_RestoreFallsBackToLocal(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.restoreErr = fmt.Errorf("s3 down")
	remote.entries[entryKey("build", "k1")] = true // check says yes, but restore fails
	local.entries[entryKey("build", "k1")] = true

	cfg := HybridConfig{Strategy: StrategyRemoteFirst, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	err := h.Restore(context.Background(), "build", "k1")
	require.NoError(t, err)
}

func TestHybridBackend_RemoteFirst_SaveRemoteFirst(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()

	cfg := HybridConfig{Strategy: StrategyRemoteFirst, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	err := h.Save(context.Background(), "build", "k1", nil, Metadata{})
	require.NoError(t, err)

	assert.True(t, local.entries[entryKey("build", "k1")])
	assert.True(t, remote.entries[entryKey("build", "k1")])
}

func TestHybridBackend_RemoteFirst_SaveRemoteError_FallsBackToLocal(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	remote.saveErr = fmt.Errorf("s3 error")

	cfg := HybridConfig{Strategy: StrategyRemoteFirst, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	err := h.Save(context.Background(), "build", "k1", nil, Metadata{})
	require.NoError(t, err)

	assert.True(t, local.entries[entryKey("build", "k1")])
}

// --- write-through strategy tests ---

func TestHybridBackend_WriteThrough_SaveBoth(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()

	cfg := HybridConfig{Strategy: StrategyWriteThrough, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	err := h.Save(context.Background(), "build", "k1", nil, Metadata{})
	require.NoError(t, err)

	assert.True(t, local.entries[entryKey("build", "k1")])
	assert.True(t, remote.entries[entryKey("build", "k1")])
}

func TestHybridBackend_WriteThrough_CheckLocalFirst(t *testing.T) {
	local := newMockBackend()
	remote := newMockBackend()
	local.entries[entryKey("build", "k1")] = true

	cfg := HybridConfig{Strategy: StrategyWriteThrough, FallbackOnError: true}
	h := NewHybridBackend(local, remote, cfg)

	hit, err := h.Check(context.Background(), "build", "k1")
	require.NoError(t, err)
	assert.True(t, hit)
}
