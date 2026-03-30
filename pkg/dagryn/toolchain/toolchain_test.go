package toolchain

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectRuntimes(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantGo  bool
		wantN   bool
	}{
		{name: "go build", command: "go build ./...", wantGo: true},
		{name: "gofmt", command: "test -z \"$(gofmt -l .)\"", wantGo: true},
		{name: "npm ci", command: "npm ci", wantN: true},
		{name: "pnpm build", command: "pnpm build", wantN: true},
		{name: "yarn test", command: "yarn test", wantN: true},
		{name: "node direct", command: "node --version", wantN: true},
		{name: "mixed", command: "go test ./... && npm test", wantGo: true, wantN: true},
		{name: "no runtime", command: "echo hello"},
		{name: "word boundary", command: "echo golang", wantGo: false, wantN: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectRuntimes(tt.command)
			hasGo := false
			hasNode := false
			for _, rt := range got {
				if rt == RuntimeGo {
					hasGo = true
				}
				if rt == RuntimeNode {
					hasNode = true
				}
			}

			if hasGo != tt.wantGo {
				t.Fatalf("hasGo=%v want=%v (runtimes=%v)", hasGo, tt.wantGo, got)
			}
			if hasNode != tt.wantN {
				t.Fatalf("hasNode=%v want=%v (runtimes=%v)", hasNode, tt.wantN, got)
			}
		})
	}
}

func TestCommandExistsInPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, binaryName("mycmd"))
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho ok\n"), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(bin, 0755); err != nil {
			t.Fatalf("chmod fake binary: %v", err)
		}
	}

	if !CommandExistsInPath("mycmd", dir) {
		t.Fatalf("expected mycmd to exist in %s", dir)
	}
	if CommandExistsInPath("missing-cmd", dir) {
		t.Fatalf("did not expect missing-cmd to exist in %s", dir)
	}
}

func TestPrependPath(t *testing.T) {
	if got := PrependPath("/a", "/b"); got != "/a"+string(filepath.ListSeparator)+"/b" {
		t.Fatalf("unexpected prepend result: %s", got)
	}
	if got := PrependPath("", "/b"); got != "/b" {
		t.Fatalf("unexpected prepend result for empty prefix: %s", got)
	}
	if got := PrependPath("/a", ""); got != "/a" {
		t.Fatalf("unexpected prepend result for empty current: %s", got)
	}
}
