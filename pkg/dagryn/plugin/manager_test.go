package plugin

import (
	"context"
	"testing"
)

type testOfficialResolver struct {
	resolveCalls int
}

func (r *testOfficialResolver) Name() string {
	return "official"
}

func (r *testOfficialResolver) CanResolve(plugin *Plugin) bool {
	return plugin.Source == SourceOfficial
}

func (r *testOfficialResolver) Resolve(ctx context.Context, plugin *Plugin) (*Plugin, error) {
	r.resolveCalls++
	resolved := *plugin
	if resolved.Version == "" {
		resolved.Version = "latest"
	}
	resolved.ResolvedVersion = "main"
	resolved.Manifest = nil
	return &resolved, nil
}

func (r *testOfficialResolver) Install(ctx context.Context, plugin *Plugin, installDir string) (*InstallResult, error) {
	installed := *plugin
	installed.InstallPath = installDir
	installed.Manifest = &Manifest{
		Plugin: ManifestPlugin{
			Name:    plugin.Name,
			Version: "1.0.0",
			Type:    "composite",
		},
	}

	return &InstallResult{
		Plugin:  &installed,
		Status:  StatusInstalled,
		Message: "installed",
	}, nil
}

func (r *testOfficialResolver) Verify(ctx context.Context, plugin *Plugin) error {
	return nil
}

func TestManagerInstallCachesResolverResultPlugin(t *testing.T) {
	root := t.TempDir()
	m := NewManager(root)

	resolver := &testOfficialResolver{}
	m.registry.Register(SourceOfficial, resolver)

	const spec = "official:setup-pnpm@v0.2.0"

	result, err := m.Install(context.Background(), spec)
	if err != nil {
		t.Fatalf("install plugin: %v", err)
	}
	if result == nil || result.Plugin == nil {
		t.Fatalf("expected install result plugin")
	}
	if result.Plugin.Manifest == nil || !result.Plugin.Manifest.IsComposite() {
		t.Fatalf("expected install result to include composite manifest")
	}

	resolved, err := m.Resolve(context.Background(), spec)
	if err != nil {
		t.Fatalf("resolve plugin: %v", err)
	}
	if resolved.Manifest == nil || !resolved.Manifest.IsComposite() {
		t.Fatalf("expected cached resolved plugin to keep composite manifest")
	}

	if resolver.resolveCalls != 1 {
		t.Fatalf("expected resolver.Resolve to be called once, got %d", resolver.resolveCalls)
	}
}
