package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseOfficialPlugin(t *testing.T) {
	p, err := Parse("official:setup-node")
	if err != nil {
		t.Fatalf("parse official plugin: %v", err)
	}
	if p.Source != SourceOfficial {
		t.Fatalf("expected source official, got %s", p.Source)
	}
	if p.Name != "setup-node" {
		t.Fatalf("expected name setup-node, got %s", p.Name)
	}
	if p.Version != "latest" {
		t.Fatalf("expected default version latest, got %s", p.Version)
	}
}

func TestOfficialResolverUsesDagrynPluginsDir(t *testing.T) {
	root := t.TempDir()
	pluginsRoot := filepath.Join(root, "dagryn-plugins")
	pluginDir := filepath.Join(pluginsRoot, "setup-node")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	manifest := []byte("[plugin]\nname = \"setup-node\"\nversion = \"1.0.0\"\n[composite]\nsteps = []\n")
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Setenv("DAGRYN_PLUGINS_DIR", pluginsRoot)

	r := NewOfficialResolver(root)
	p, err := Parse("official:setup-node")
	if err != nil {
		t.Fatalf("parse official spec: %v", err)
	}

	resolved, err := r.Resolve(context.Background(), p)
	if err != nil {
		t.Fatalf("resolve official plugin: %v", err)
	}

	if resolved.InstallPath != pluginDir {
		t.Fatalf("expected install path %s, got %s", pluginDir, resolved.InstallPath)
	}
	if resolved.Source != SourceOfficial {
		t.Fatalf("expected official source, got %s", resolved.Source)
	}
}

func TestOfficialResolverResolveFromInstallPath(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "cached", "setup-node")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	manifest := []byte("[plugin]\nname = \"setup-node\"\nversion = \"1.2.3\"\n[composite]\nsteps = []\n")
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	r := NewOfficialResolver(root)
	p := &Plugin{
		Source:      SourceOfficial,
		Name:        "setup-node",
		Version:     "v1",
		InstallPath: pluginDir,
		Raw:         "official:setup-node@v1",
	}

	resolved, err := r.Resolve(context.Background(), p)
	if err != nil {
		t.Fatalf("resolve official plugin from install path: %v", err)
	}

	if resolved.InstallPath != pluginDir {
		t.Fatalf("expected install path %s, got %s", pluginDir, resolved.InstallPath)
	}
}

func TestResolveSemverConstraintMajorAlias(t *testing.T) {
	tags := []string{"v0.1.0", "v1.0.0", "v1.2.3", "v2.0.0"}
	got, err := resolveSemverConstraint("v1", tags)
	if err != nil {
		t.Fatalf("resolveSemverConstraint(v1): %v", err)
	}
	if got != "v1.2.3" {
		t.Fatalf("expected v1.2.3, got %s", got)
	}
}

func TestResolveSemverConstraintCaretAndTilde(t *testing.T) {
	tags := []string{"v1.2.3", "v1.2.9", "v1.3.0", "v2.0.0"}

	got, err := resolveSemverConstraint("^1.2.3", tags)
	if err != nil {
		t.Fatalf("resolveSemverConstraint(^1.2.3): %v", err)
	}
	if got != "v1.3.0" {
		t.Fatalf("expected v1.3.0, got %s", got)
	}

	got, err = resolveSemverConstraint("~1.2.3", tags)
	if err != nil {
		t.Fatalf("resolveSemverConstraint(~1.2.3): %v", err)
	}
	if got != "v1.2.9" {
		t.Fatalf("expected v1.2.9, got %s", got)
	}
}
