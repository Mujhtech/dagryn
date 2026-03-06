package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mujhtech/dagryn/internal/cli"
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

	flags := &cli.Flags{CfgFile: configFile}
	cmd := NewCmd(flags)
	// Get the validate subcommand
	validateCmd, _, err := cmd.Find([]string{"validate"})
	require.NoError(t, err)
	err = validateCmd.RunE(validateCmd, nil)
	assert.NoError(t, err)
}

func TestConfigValidateMissingFile(t *testing.T) {
	flags := &cli.Flags{CfgFile: filepath.Join(t.TempDir(), "nonexistent.toml")}
	cmd := NewCmd(flags)
	validateCmd, _, err := cmd.Find([]string{"validate"})
	require.NoError(t, err)
	err = validateCmd.RunE(validateCmd, nil)
	assert.Error(t, err)
}

func TestConfigValidateInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "dagryn.toml")
	content := `[workflow]
name = "test"

[tasks.build]
`
	err := os.WriteFile(configFile, []byte(content), 0644)
	require.NoError(t, err)

	flags := &cli.Flags{CfgFile: configFile}
	cmd := NewCmd(flags)
	validateCmd, _, err := cmd.Find([]string{"validate"})
	require.NoError(t, err)
	err = validateCmd.RunE(validateCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation error")
}
