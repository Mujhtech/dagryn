package plugin

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		spec       string
		wantSource SourceType
		wantName   string
		wantVer    string
		wantErr    bool
	}{
		{
			name:       "github plugin",
			spec:       "github:golangci/golangci-lint@v1.55.0",
			wantSource: SourceGitHub,
			wantName:   "golangci-lint",
			wantVer:    "v1.55.0",
		},
		{
			name:       "go install plugin",
			spec:       "go:golang.org/x/tools/cmd/goimports@latest",
			wantSource: SourceGo,
			wantName:   "goimports",
			wantVer:    "latest",
		},
		{
			name:       "npm plugin",
			spec:       "npm:prettier@3.0.0",
			wantSource: SourceNPM,
			wantName:   "prettier",
			wantVer:    "3.0.0",
		},
		{
			name:       "npm scoped plugin",
			spec:       "npm:@angular/cli@17.0.0",
			wantSource: SourceNPM,
			wantName:   "cli",
			wantVer:    "17.0.0",
		},
		{
			name:       "pip plugin",
			spec:       "pip:black@23.12.0",
			wantSource: SourcePip,
			wantName:   "black",
			wantVer:    "23.12.0",
		},
		{
			name:       "cargo plugin",
			spec:       "cargo:ripgrep@14.0.3",
			wantSource: SourceCargo,
			wantName:   "ripgrep",
			wantVer:    "14.0.3",
		},
		{
			name:    "invalid - empty",
			spec:    "",
			wantErr: true,
		},
		{
			name:       "short format - github implicit",
			spec:       "dagryn/setup-go@v1",
			wantSource: SourceGitHub,
			wantName:   "setup-go",
			wantVer:    "v1",
		},
		{
			name:       "short format - exact version",
			spec:       "golangci/golangci-lint@v1.55.0",
			wantSource: SourceGitHub,
			wantName:   "golangci-lint",
			wantVer:    "v1.55.0",
		},
		{
			name:       "short format - latest",
			spec:       "owner/repo@latest",
			wantSource: SourceGitHub,
			wantName:   "repo",
			wantVer:    "latest",
		},
		{
			name:    "invalid - no source no owner",
			spec:    "golangci-lint@v1.55.0",
			wantErr: true,
		},
		{
			name:    "invalid - no version",
			spec:    "github:golangci/golangci-lint",
			wantErr: true,
		},
		{
			name:    "invalid - unknown source",
			spec:    "unknown:package@v1.0.0",
			wantErr: true,
		},
		{
			name:       "local plugin - relative path",
			spec:       "local:./plugins/setup-node",
			wantSource: SourceLocal,
			wantName:   "setup-node",
			wantVer:    "",
		},
		{
			name:       "local plugin - with version",
			spec:       "local:./plugins/setup-go@1.0.0",
			wantSource: SourceLocal,
			wantName:   "setup-go",
			wantVer:    "1.0.0",
		},
		{
			name:       "local plugin - no dot slash",
			spec:       "local:plugins/eslint",
			wantSource: SourceLocal,
			wantName:   "eslint",
			wantVer:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := Parse(tt.spec)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if plugin.Source != tt.wantSource {
				t.Errorf("Parse() source = %v, want %v", plugin.Source, tt.wantSource)
			}

			if plugin.Name != tt.wantName {
				t.Errorf("Parse() name = %v, want %v", plugin.Name, tt.wantName)
			}

			if plugin.Version != tt.wantVer {
				t.Errorf("Parse() version = %v, want %v", plugin.Version, tt.wantVer)
			}
		})
	}
}

func TestParse_ShortFormat_Owner(t *testing.T) {
	p, err := Parse("dagryn/setup-go@v1.0.0")
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if p.Owner != "dagryn" {
		t.Errorf("Parse() owner = %v, want %v", p.Owner, "dagryn")
	}
	if p.Repo != "setup-go" {
		t.Errorf("Parse() repo = %v, want %v", p.Repo, "setup-go")
	}
	if p.Raw != "dagryn/setup-go@v1.0.0" {
		t.Errorf("Parse() raw = %v, want %v", p.Raw, "dagryn/setup-go@v1.0.0")
	}
}

// Verify long format still takes precedence
func TestParse_LongFormatPrecedence(t *testing.T) {
	// This matches long format (github:owner/repo@version)
	p, err := Parse("github:owner/repo@v1.0.0")
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if p.Source != SourceGitHub {
		t.Errorf("Parse() source = %v, want %v", p.Source, SourceGitHub)
	}
}

func TestPlugin_IsExactVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"v1.0.0", true},
		{"1.0.0", true},
		{"v1.55.2", true},
		{"v1.0.0-rc1", true},
		{"latest", false},
		{"^1.0.0", false},
		{"~1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			p := &Plugin{Version: tt.version}
			if got := p.IsExactVersion(); got != tt.want {
				t.Errorf("IsExactVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlugin_IsSemverRange(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"^1.0.0", true},
		{"~1.0.0", true},
		{"v1.0.0", false},
		{"latest", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			p := &Plugin{Version: tt.version}
			if got := p.IsSemverRange(); got != tt.want {
				t.Errorf("IsSemverRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlugin_CacheKey(t *testing.T) {
	p := &Plugin{
		Source:          SourceGitHub,
		Name:            "golangci-lint",
		Version:         "v1.55.0",
		ResolvedVersion: "v1.55.0",
	}

	key := p.CacheKey()
	expected := "github/golangci-lint/v1.55.0"
	if key != expected {
		t.Errorf("CacheKey() = %v, want %v", key, expected)
	}
}

func TestSpec_UnmarshalTOML(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		want    []string
		wantErr bool
	}{
		{
			name: "single string",
			data: "github:owner/repo@v1.0.0",
			want: []string{"github:owner/repo@v1.0.0"},
		},
		{
			name: "array of strings",
			data: []interface{}{"github:a/b@v1", "npm:c@v2"},
			want: []string{"github:a/b@v1", "npm:c@v2"},
		},
		{
			name: "empty string",
			data: "",
			want: nil,
		},
		{
			name: "nil",
			data: nil,
			want: nil,
		},
		{
			name:    "invalid type",
			data:    123,
			wantErr: true,
		},
		{
			name:    "array with non-string",
			data:    []interface{}{"valid", 123},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var spec Spec
			err := spec.UnmarshalTOML(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalTOML() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("UnmarshalTOML() unexpected error: %v", err)
				return
			}

			if len(spec.Plugins) != len(tt.want) {
				t.Errorf("UnmarshalTOML() got %d plugins, want %d", len(spec.Plugins), len(tt.want))
				return
			}

			for i, p := range spec.Plugins {
				if p != tt.want[i] {
					t.Errorf("UnmarshalTOML() plugin[%d] = %v, want %v", i, p, tt.want[i])
				}
			}
		})
	}
}

func TestPlatformAliases(t *testing.T) {
	p := Platform{OS: "darwin", Arch: "arm64"}
	aliases := p.PlatformAliases()

	// Should contain darwin-arm64 and macos-arm64 among others
	found := false
	for _, a := range aliases {
		if a == "darwin-arm64" || a == "macos-arm64" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("PlatformAliases() should contain darwin-arm64 or macos-arm64")
	}
}

func TestCurrentPlatform(t *testing.T) {
	p := CurrentPlatform()

	if p.OS == "" {
		t.Error("CurrentPlatform() OS is empty")
	}

	if p.Arch == "" {
		t.Error("CurrentPlatform() Arch is empty")
	}
}
