package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfficialPlugins_AllValid(t *testing.T) {
	pluginsDir := filepath.Join("..", "..", "plugins")

	entries, err := os.ReadDir(pluginsDir)
	require.NoError(t, err, "failed to read plugins directory")
	require.NotEmpty(t, entries, "plugins directory should not be empty")

	expectedPlugins := []string{
		"cache-s3",
		"deploy-ssh",
		"docker-build",
		"eslint",
		"golangci-lint",
		"jest",
		"notify-discord",
		"prettier",
		"pytest",
		"setup-go",
		"setup-node",
		"setup-python",
		"setup-rust",
		"slack-notify",
		"slack-notify-integration",
		"upload-artifact",
	}

	// Verify all expected plugins exist
	var foundPlugins []string
	for _, entry := range entries {
		if entry.IsDir() {
			foundPlugins = append(foundPlugins, entry.Name())
		}
	}
	assert.ElementsMatch(t, expectedPlugins, foundPlugins, "should have all expected plugins")

	// Parse and validate each plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			manifestPath := filepath.Join(pluginsDir, entry.Name(), "plugin.toml")

			data, err := os.ReadFile(manifestPath)
			require.NoError(t, err, "should be able to read plugin.toml")

			manifest, err := ParseManifest(data)
			require.NoError(t, err, "should parse plugin.toml without error")

			err = ValidateManifest(manifest)
			require.NoError(t, err, "should pass validation")

			// All should have metadata
			assert.NotEmpty(t, manifest.Plugin.Name, "should have a name")
			assert.NotEmpty(t, manifest.Plugin.Description, "should have a description")
			assert.Equal(t, "1.0.0", manifest.Plugin.Version, "should be version 1.0.0")
			assert.Equal(t, "dagryn", manifest.Plugin.Author, "should have author dagryn")
			assert.Equal(t, "MIT", manifest.Plugin.License, "should have MIT license")

			// Validate based on type
			if manifest.IsIntegration() {
				assert.NotEmpty(t, manifest.Hooks, "integration plugin should have at least one hook")
				for name, hook := range manifest.Hooks {
					assert.True(t, ValidHookNames[name], "hook name %q should be valid", name)
					assert.NotEmpty(t, hook.Command, "hook %s should have a command", name)
				}
			} else {
				// Composite plugins
				assert.Equal(t, "composite", manifest.Plugin.Type, "non-integration official plugins should be composite")
				assert.True(t, manifest.IsComposite(), "should be composite")
				assert.NotEmpty(t, manifest.Steps, "should have at least one step")
				for i, step := range manifest.Steps {
					assert.NotEmpty(t, step.Name, "step %d should have a name", i)
					assert.NotEmpty(t, step.Command, "step %d (%s) should have a command", i, step.Name)
				}
			}
		})
	}
}

func TestOfficialPlugins_RequiredInputs(t *testing.T) {
	pluginsDir := filepath.Join("..", "..", "plugins")

	tests := []struct {
		plugin         string
		requiredInputs []string
	}{
		{"docker-build", []string{"tags"}},
		{"slack-notify", []string{"webhook-url", "message"}},
		{"cache-s3", []string{"bucket", "key", "path"}},
		{"deploy-ssh", []string{"host", "user", "source", "target"}},
		{"upload-artifact", []string{"path", "name", "bucket"}},
		{"notify-discord", []string{"webhook-url", "message"}},
	}

	for _, tt := range tests {
		t.Run(tt.plugin, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(pluginsDir, tt.plugin, "plugin.toml"))
			require.NoError(t, err)

			manifest, err := ParseManifest(data)
			require.NoError(t, err)

			for _, input := range tt.requiredInputs {
				require.Contains(t, manifest.Inputs, input, "should have input %q", input)
				assert.True(t, manifest.Inputs[input].Required, "input %q should be required", input)
			}
		})
	}
}

func TestOfficialPlugins_DefaultInputs(t *testing.T) {
	pluginsDir := filepath.Join("..", "..", "plugins")

	tests := []struct {
		plugin   string
		input    string
		expected string
	}{
		{"setup-node", "node-version", "20"},
		{"setup-go", "go-version", "1.22"},
		{"setup-python", "python-version", "3.12"},
		{"setup-rust", "rust-version", "stable"},
		{"eslint", "args", "."},
		{"prettier", "args", "."},
		{"golangci-lint", "args", "./..."},
		{"cache-s3", "region", "us-east-1"},
		{"slack-notify", "username", "Dagryn"},
		{"deploy-ssh", "port", "22"},
		{"deploy-ssh", "key", "~/.ssh/id_rsa"},
		{"upload-artifact", "region", "us-east-1"},
		{"upload-artifact", "retention-days", "30"},
		{"notify-discord", "username", "Dagryn"},
	}

	for _, tt := range tests {
		t.Run(tt.plugin+"/"+tt.input, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(pluginsDir, tt.plugin, "plugin.toml"))
			require.NoError(t, err)

			manifest, err := ParseManifest(data)
			require.NoError(t, err)

			require.Contains(t, manifest.Inputs, tt.input)
			assert.Equal(t, tt.expected, manifest.Inputs[tt.input].Default)
		})
	}
}
