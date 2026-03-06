package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseManifest_ToolPlugin(t *testing.T) {
	data := []byte(`
[plugin]
name = "golangci-lint"
description = "Go linters runner"
version = "1.55.0"
type = "tool"
author = "golangci"
license = "MIT"
homepage = "https://golangci-lint.run"

[tool]
binary = "golangci-lint"

[platforms]
"darwin-arm64" = "golangci-lint-1.55.0-darwin-arm64.tar.gz"
"darwin-amd64" = "golangci-lint-1.55.0-darwin-amd64.tar.gz"
"linux-amd64"  = "golangci-lint-1.55.0-linux-amd64.tar.gz"

[inputs.config]
description = "Path to config file"
default = ".golangci.yml"

[outputs.report]
description = "Lint report output"
`)

	m, err := ParseManifest(data)
	require.NoError(t, err)

	assert.Equal(t, "golangci-lint", m.Plugin.Name)
	assert.Equal(t, "Go linters runner", m.Plugin.Description)
	assert.Equal(t, "1.55.0", m.Plugin.Version)
	assert.Equal(t, "tool", m.Plugin.Type)
	assert.Equal(t, "golangci", m.Plugin.Author)
	assert.Equal(t, "MIT", m.Plugin.License)

	assert.Equal(t, "golangci-lint", m.Tool.Binary)

	assert.True(t, m.IsTool())
	assert.False(t, m.IsComposite())

	assert.Equal(t, "golangci-lint-1.55.0-darwin-arm64.tar.gz", m.PlatformAsset("darwin-arm64"))
	assert.Equal(t, "golangci-lint-1.55.0-linux-amd64.tar.gz", m.PlatformAsset("linux-amd64"))
	assert.Equal(t, "", m.PlatformAsset("windows-amd64"))

	require.Contains(t, m.Inputs, "config")
	assert.Equal(t, ".golangci.yml", m.Inputs["config"].Default)

	require.Contains(t, m.Outputs, "report")
	assert.Equal(t, "Lint report output", m.Outputs["report"].Description)
}

func TestParseManifest_CompositePlugin(t *testing.T) {
	data := []byte(`
[plugin]
name = "setup-go"
description = "Set up Go environment"
version = "1.0.0"
type = "composite"

[inputs.go-version]
required = true
description = "Go version to install"

[inputs.cache]
description = "Enable module cache"
default = "true"

[[step]]
name = "install"
command = "curl -sL https://go.dev/dl/go${inputs.go-version}.${os}-${arch}.tar.gz | tar xz -C /usr/local"

[[step]]
name = "set-path"
command = "export PATH=/usr/local/go/bin:$PATH"

[[step]]
name = "verify"
command = "go version"
`)

	m, err := ParseManifest(data)
	require.NoError(t, err)

	assert.Equal(t, "setup-go", m.Plugin.Name)
	assert.Equal(t, "composite", m.Plugin.Type)

	assert.True(t, m.IsComposite())
	assert.False(t, m.IsTool())

	require.Len(t, m.Steps, 3)
	assert.Equal(t, "install", m.Steps[0].Name)
	assert.Contains(t, m.Steps[0].Command, "${inputs.go-version}")
	assert.Equal(t, "verify", m.Steps[2].Name)

	require.Contains(t, m.Inputs, "go-version")
	assert.True(t, m.Inputs["go-version"].Required)

	require.Contains(t, m.Inputs, "cache")
	assert.Equal(t, "true", m.Inputs["cache"].Default)
}

func TestParseManifest_Invalid(t *testing.T) {
	_, err := ParseManifest([]byte("not valid toml [[["))
	require.Error(t, err)
}

func TestValidateManifest_Valid(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test-plugin",
			Version: "1.0.0",
			Type:    "tool",
		},
	}
	err := ValidateManifest(m)
	assert.NoError(t, err)
}

func TestValidateManifest_MissingName(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Version: "1.0.0",
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin.name is required")
}

func TestValidateManifest_MissingVersion(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name: "test",
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin.version is required")
}

func TestValidateManifest_CompositeNoSteps(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test",
			Version: "1.0.0",
			Type:    "composite",
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one step")
}

func TestValidateManifest_CompositeEmptyCommand(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test",
			Version: "1.0.0",
			Type:    "composite",
		},
		Steps: []CompositeStep{
			{Name: "step1", Command: ""},
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have a command")
}

func TestValidateManifest_UnsupportedType(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test",
			Version: "1.0.0",
			Type:    "invalid",
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported plugin type")
}

func TestValidateManifest_DefaultType(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test",
			Version: "1.0.0",
		},
	}
	err := ValidateManifest(m)
	assert.NoError(t, err)
	assert.True(t, m.IsTool())
}

func TestParseManifest_IntegrationPlugin(t *testing.T) {
	data := []byte(`
[plugin]
name = "slack-notify-integration"
description = "Send Slack notifications"
version = "1.0.0"
type = "integration"
author = "dagryn"

[inputs.webhook-url]
required = true
description = "Slack webhook URL"

[hooks.on_run_success]
command = 'echo "success"'

[hooks.on_run_failure]
command = 'echo "failure"'
`)

	m, err := ParseManifest(data)
	require.NoError(t, err)

	assert.Equal(t, "slack-notify-integration", m.Plugin.Name)
	assert.Equal(t, "integration", m.Plugin.Type)
	assert.True(t, m.IsIntegration())
	assert.False(t, m.IsComposite())
	assert.False(t, m.IsTool())

	require.Len(t, m.Hooks, 2)
	assert.NotEmpty(t, m.Hooks["on_run_success"].Command)
	assert.NotEmpty(t, m.Hooks["on_run_failure"].Command)

	require.Contains(t, m.Inputs, "webhook-url")
	assert.True(t, m.Inputs["webhook-url"].Required)
}

func TestValidateManifest_IntegrationValid(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test-integration",
			Version: "1.0.0",
			Type:    "integration",
		},
		Hooks: map[string]HookDef{
			"on_run_start": {Command: "echo start"},
		},
	}
	err := ValidateManifest(m)
	assert.NoError(t, err)
}

func TestValidateManifest_IntegrationNoHooks(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test-integration",
			Version: "1.0.0",
			Type:    "integration",
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one hook")
}

func TestValidateManifest_IntegrationInvalidHookName(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test-integration",
			Version: "1.0.0",
			Type:    "integration",
		},
		Hooks: map[string]HookDef{
			"on_invalid_event": {Command: "echo bad"},
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hook name")
}

func TestValidateManifest_IntegrationEmptyCommand(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			Name:    "test-integration",
			Version: "1.0.0",
			Type:    "integration",
		},
		Hooks: map[string]HookDef{
			"on_run_start": {Command: ""},
		},
	}
	err := ValidateManifest(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have a command")
}

func TestPlatformAsset_CaseInsensitive(t *testing.T) {
	m := &Manifest{
		Platforms: map[string]string{
			"Darwin-ARM64": "asset-darwin.tar.gz",
		},
	}
	assert.Equal(t, "asset-darwin.tar.gz", m.PlatformAsset("darwin-arm64"))
}

func TestPlatformAsset_NilPlatforms(t *testing.T) {
	m := &Manifest{}
	assert.Equal(t, "", m.PlatformAsset("darwin-arm64"))
}
