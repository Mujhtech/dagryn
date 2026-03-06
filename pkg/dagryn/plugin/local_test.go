package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalResolver_Resolve(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a local composite plugin
	pluginDir := filepath.Join(tmpDir, "plugins", "my-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	manifest := `[plugin]
name = "my-plugin"
description = "A test plugin"
version = "1.2.0"
type = "composite"

[[step]]
name = "hello"
command = "echo hello"
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(manifest), 0644))

	resolver := NewLocalResolver(tmpDir)
	ctx := context.Background()

	plugin := &Plugin{
		Source: SourceLocal,
		Name:   "my-plugin",
		Repo:   "plugins/my-plugin",
		Raw:    "local:plugins/my-plugin",
	}

	resolved, err := resolver.Resolve(ctx, plugin)
	require.NoError(t, err)

	assert.Equal(t, "my-plugin", resolved.Name)
	assert.Equal(t, "1.2.0", resolved.ResolvedVersion)
	assert.Equal(t, "1.2.0", resolved.Version)
	assert.NotNil(t, resolved.Manifest)
	assert.True(t, resolved.Manifest.IsComposite())
	assert.Equal(t, pluginDir, resolved.InstallPath)
	assert.Len(t, resolved.Manifest.Steps, 1)
	assert.Equal(t, "echo hello", resolved.Manifest.Steps[0].Command)
}

func TestLocalResolver_Resolve_WithExplicitVersion(t *testing.T) {
	tmpDir := t.TempDir()

	pluginDir := filepath.Join(tmpDir, "plugins", "my-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	manifest := `[plugin]
name = "my-plugin"
version = "2.0.0"
type = "composite"

[[step]]
name = "run"
command = "echo run"
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(manifest), 0644))

	resolver := NewLocalResolver(tmpDir)
	ctx := context.Background()

	plugin := &Plugin{
		Source:  SourceLocal,
		Name:    "my-plugin",
		Repo:    "plugins/my-plugin",
		Version: "1.0.0", // explicit version in spec
		Raw:     "local:plugins/my-plugin@1.0.0",
	}

	resolved, err := resolver.Resolve(ctx, plugin)
	require.NoError(t, err)

	// Explicit spec version is preserved
	assert.Equal(t, "1.0.0", resolved.Version)
	// ResolvedVersion comes from manifest
	assert.Equal(t, "2.0.0", resolved.ResolvedVersion)
}

func TestLocalResolver_Resolve_MissingManifest(t *testing.T) {
	tmpDir := t.TempDir()

	resolver := NewLocalResolver(tmpDir)
	ctx := context.Background()

	plugin := &Plugin{
		Source: SourceLocal,
		Name:   "nonexistent",
		Repo:   "plugins/nonexistent",
		Raw:    "local:plugins/nonexistent",
	}

	_, err := resolver.Resolve(ctx, plugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

func TestLocalResolver_Resolve_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	pluginDir := filepath.Join(tmpDir, "plugins", "bad-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	// Missing required fields
	manifest := `[plugin]
type = "composite"
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(manifest), 0644))

	resolver := NewLocalResolver(tmpDir)
	ctx := context.Background()

	plugin := &Plugin{
		Source: SourceLocal,
		Name:   "bad-plugin",
		Repo:   "plugins/bad-plugin",
		Raw:    "local:plugins/bad-plugin",
	}

	_, err := resolver.Resolve(ctx, plugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid manifest")
}

func TestLocalResolver_Install_Composite(t *testing.T) {
	tmpDir := t.TempDir()

	pluginDir := filepath.Join(tmpDir, "plugins", "my-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	manifest := `[plugin]
name = "my-plugin"
version = "1.0.0"
type = "composite"

[[step]]
name = "run"
command = "echo hello"
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(manifest), 0644))

	resolver := NewLocalResolver(tmpDir)
	ctx := context.Background()

	plugin := &Plugin{
		Source: SourceLocal,
		Name:   "my-plugin",
		Repo:   "plugins/my-plugin",
		Raw:    "local:plugins/my-plugin",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
		},
	}

	result, err := resolver.Install(ctx, plugin, "")
	require.NoError(t, err)

	assert.Equal(t, StatusInstalled, result.Status)
	assert.Equal(t, pluginDir, plugin.InstallPath)
}

func TestLocalResolver_Verify(t *testing.T) {
	tmpDir := t.TempDir()

	// Valid plugin
	pluginDir := filepath.Join(tmpDir, "plugins", "my-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(`[plugin]
name = "test"
version = "1.0.0"
`), 0644))

	resolver := NewLocalResolver(tmpDir)
	ctx := context.Background()

	// Should pass for valid plugin
	err := resolver.Verify(ctx, &Plugin{
		Source: SourceLocal,
		Repo:   "plugins/my-plugin",
	})
	assert.NoError(t, err)

	// Should fail for missing plugin
	err = resolver.Verify(ctx, &Plugin{
		Source: SourceLocal,
		Repo:   "plugins/nonexistent",
	})
	assert.Error(t, err)
}

func TestLocalResolver_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	pluginDir := filepath.Join(tmpDir, "my-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	manifest := `[plugin]
name = "my-plugin"
version = "1.0.0"
type = "composite"

[[step]]
name = "run"
command = "echo hello"
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(manifest), 0644))

	resolver := NewLocalResolver("/some/other/root")
	ctx := context.Background()

	// Using absolute path should work regardless of project root
	plugin := &Plugin{
		Source: SourceLocal,
		Name:   "my-plugin",
		Repo:   pluginDir, // absolute path
		Raw:    "local:" + pluginDir,
	}

	resolved, err := resolver.Resolve(ctx, plugin)
	require.NoError(t, err)
	assert.Equal(t, "my-plugin", resolved.Name)
	assert.True(t, resolved.Manifest.IsComposite())
}
