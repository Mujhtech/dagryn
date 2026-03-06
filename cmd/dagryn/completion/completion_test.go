package completion

import (
	"bytes"
	"testing"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionBash(t *testing.T) {
	root := cli.NewRootCmd()
	cmd := NewCmd(cli.GlobalFlags, root)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"bash"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "bash")
}

func TestCompletionZsh(t *testing.T) {
	root := cli.NewRootCmd()
	cmd := NewCmd(cli.GlobalFlags, root)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"zsh"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestCompletionFish(t *testing.T) {
	root := cli.NewRootCmd()
	cmd := NewCmd(cli.GlobalFlags, root)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"fish"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestCompletionPowershell(t *testing.T) {
	root := cli.NewRootCmd()
	cmd := NewCmd(cli.GlobalFlags, root)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"powershell"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestCompletionInvalidShell(t *testing.T) {
	root := cli.NewRootCmd()
	cmd := NewCmd(cli.GlobalFlags, root)
	cmd.SetArgs([]string{"invalid"})
	err := cmd.Execute()
	assert.Error(t, err)
}
