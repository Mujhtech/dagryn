package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationRegistry_Register(t *testing.T) {
	executor := NewHookExecutor(nil)
	registry := NewIntegrationRegistry(executor)

	assert.Equal(t, 0, registry.Count())

	registry.Register(IntegrationPlugin{
		Name: "test-plugin",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "integration"},
			Hooks: map[string]HookDef{
				HookOnRunStart: {Command: "echo start"},
			},
		},
	})

	assert.Equal(t, 1, registry.Count())
}

func TestIntegrationRegistry_DispatchHook(t *testing.T) {
	executor := NewHookExecutor(nil)
	registry := NewIntegrationRegistry(executor)

	tmpDir := t.TempDir()
	markerFile := filepath.Join(tmpDir, "hook_ran")

	registry.Register(IntegrationPlugin{
		Name: "plugin-a",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "integration"},
			Hooks: map[string]HookDef{
				HookOnRunStart: {Command: "touch " + markerFile},
			},
		},
	})

	hctx := &HookContext{ProjectRoot: tmpDir}
	registry.DispatchHook(context.Background(), HookOnRunStart, hctx)

	_, err := os.Stat(markerFile)
	require.NoError(t, err, "hook should have created marker file")
}

func TestIntegrationRegistry_DispatchHook_MissingHookSkipped(t *testing.T) {
	executor := NewHookExecutor(nil)
	registry := NewIntegrationRegistry(executor)

	registry.Register(IntegrationPlugin{
		Name: "plugin-a",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "integration"},
			Hooks: map[string]HookDef{
				HookOnRunSuccess: {Command: "echo success"},
			},
		},
	})

	hctx := &HookContext{ProjectRoot: t.TempDir()}
	// Dispatch a hook that this plugin doesn't have - should not panic
	registry.DispatchHook(context.Background(), HookOnRunStart, hctx)
}

func TestIntegrationRegistry_DispatchHook_MultiplePlugins(t *testing.T) {
	executor := NewHookExecutor(nil)
	registry := NewIntegrationRegistry(executor)

	tmpDir := t.TempDir()
	markerA := filepath.Join(tmpDir, "a_ran")
	markerB := filepath.Join(tmpDir, "b_ran")

	registry.Register(IntegrationPlugin{
		Name: "plugin-a",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "integration"},
			Hooks: map[string]HookDef{
				HookOnRunEnd: {Command: "touch " + markerA},
			},
		},
	})

	registry.Register(IntegrationPlugin{
		Name: "plugin-b",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "integration"},
			Hooks: map[string]HookDef{
				HookOnRunEnd: {Command: "touch " + markerB},
			},
		},
	})

	hctx := &HookContext{ProjectRoot: tmpDir}
	registry.DispatchHook(context.Background(), HookOnRunEnd, hctx)

	_, errA := os.Stat(markerA)
	require.NoError(t, errA, "plugin-a hook should have run")
	_, errB := os.Stat(markerB)
	require.NoError(t, errB, "plugin-b hook should have run")
}

func TestIntegrationRegistry_DispatchHook_NonFatalFailure(t *testing.T) {
	executor := NewHookExecutor(nil)
	registry := NewIntegrationRegistry(executor)

	tmpDir := t.TempDir()
	markerB := filepath.Join(tmpDir, "b_ran")

	registry.Register(IntegrationPlugin{
		Name: "plugin-failing",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "integration"},
			Hooks: map[string]HookDef{
				HookOnRunEnd: {Command: "exit 1"},
			},
		},
	})

	registry.Register(IntegrationPlugin{
		Name: "plugin-ok",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "integration"},
			Hooks: map[string]HookDef{
				HookOnRunEnd: {Command: "touch " + markerB},
			},
		},
	})

	hctx := &HookContext{ProjectRoot: tmpDir}
	registry.DispatchHook(context.Background(), HookOnRunEnd, hctx)

	// Second plugin should still have run despite first failing
	_, err := os.Stat(markerB)
	require.NoError(t, err, "second plugin should have run despite first failure")
}
