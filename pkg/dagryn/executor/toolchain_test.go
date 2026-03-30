package executor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureToolchainsForCommand_RuntimeAlreadyAvailable(t *testing.T) {
	tmp := t.TempDir()
	fakeGo := filepath.Join(tmp, "go")
	fakeNode := filepath.Join(tmp, "node")

	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\necho go\n"), 0755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}
	if err := os.WriteFile(fakeNode, []byte("#!/bin/sh\necho node\n"), 0755); err != nil {
		t.Fatalf("write fake node: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(fakeGo, 0755); err != nil {
			t.Fatalf("chmod fake go: %v", err)
		}
		if err := os.Chmod(fakeNode, 0755); err != nil {
			t.Fatalf("chmod fake node: %v", err)
		}
	}

	env := map[string]string{
		"PATH": tmp,
	}

	got, err := ensureToolchainsForCommand(context.Background(), "go test ./... && npm test", env, nil)
	if err != nil {
		t.Fatalf("ensureToolchainsForCommand returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected no env updates when runtimes already exist, got: %v", got)
	}
}

func TestEnsureToolchainsForCommand_UsesPluginPathForDetection(t *testing.T) {
	tmp := t.TempDir()
	pluginBin := filepath.Join(tmp, "plugins")
	if err := os.MkdirAll(pluginBin, 0755); err != nil {
		t.Fatalf("mkdir plugin bin: %v", err)
	}
	fakeNode := filepath.Join(pluginBin, "node")
	if err := os.WriteFile(fakeNode, []byte("#!/bin/sh\necho node\n"), 0755); err != nil {
		t.Fatalf("write fake node: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(fakeNode, 0755); err != nil {
			t.Fatalf("chmod fake node: %v", err)
		}
	}

	env := map[string]string{
		"PATH": "",
	}

	got, err := ensureToolchainsForCommand(context.Background(), "npm ci", env, []string{pluginBin})
	if err != nil {
		t.Fatalf("ensureToolchainsForCommand returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected no env updates when runtime is available from plugin path, got: %v", got)
	}
}
