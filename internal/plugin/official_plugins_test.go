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
		"docker-build",
		"eslint",
		"golangci-lint",
		"jest",
		"prettier",
		"pytest",
		"setup-go",
		"setup-node",
		"setup-python",
		"setup-rust",
		"slack-notify",
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

			// All official plugins should be composite
			assert.Equal(t, "composite", manifest.Plugin.Type, "official plugins should be composite")
			assert.True(t, manifest.IsComposite(), "should be composite")

			// All should have metadata
			assert.NotEmpty(t, manifest.Plugin.Name, "should have a name")
			assert.NotEmpty(t, manifest.Plugin.Description, "should have a description")
			assert.Equal(t, "1.0.0", manifest.Plugin.Version, "should be version 1.0.0")
			assert.Equal(t, "dagryn", manifest.Plugin.Author, "should have author dagryn")
			assert.Equal(t, "MIT", manifest.Plugin.License, "should have MIT license")

			// All should have at least one step
			assert.NotEmpty(t, manifest.Steps, "should have at least one step")

			// Every step should have a name and command
			for i, step := range manifest.Steps {
				assert.NotEmpty(t, step.Name, "step %d should have a name", i)
				assert.NotEmpty(t, step.Command, "step %d (%s) should have a command", i, step.Name)
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
