package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidateValid(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "dagryn.toml")
	content := `[workflow]
name = "test"
default = true

[tasks.build]
command = "echo build"
`
	err := os.WriteFile(configFile, []byte(content), 0644)
	require.NoError(t, err)

	// Save and restore the global cfgFile
	oldCfg := cfgFile
	cfgFile = configFile
	defer func() { cfgFile = oldCfg }()

	cmd := newConfigValidateCmd()
	err = cmd.RunE(cmd, nil)
	assert.NoError(t, err)
}

func TestConfigValidateMissingFile(t *testing.T) {
	oldCfg := cfgFile
	cfgFile = filepath.Join(t.TempDir(), "nonexistent.toml")
	defer func() { cfgFile = oldCfg }()

	cmd := newConfigValidateCmd()
	err := cmd.RunE(cmd, nil)
	assert.Error(t, err)
}

func TestConfigValidateInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "dagryn.toml")
	// Missing command in task
	content := `[workflow]
name = "test"

[tasks.build]
`
	err := os.WriteFile(configFile, []byte(content), 0644)
	require.NoError(t, err)

	oldCfg := cfgFile
	cfgFile = configFile
	defer func() { cfgFile = oldCfg }()

	cmd := newConfigValidateCmd()
	err = cmd.RunE(cmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation error")
}
