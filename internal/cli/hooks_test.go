package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRegisterHooksPreservesExisting(t *testing.T) {
	preCalled := false
	postCalled := false

	cmd := &cobra.Command{
		Use: "test",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			preCalled = true
			return nil
		},
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			postCalled = true
			return nil
		},
	}

	RegisterHooks(cmd)

	// The hooks should wrap the existing ones
	assert.NotNil(t, cmd.PersistentPreRunE)
	assert.NotNil(t, cmd.PersistentPostRunE)

	// Run the hooks — original callbacks should still fire
	err := cmd.PersistentPreRunE(cmd, nil)
	assert.NoError(t, err)
	assert.True(t, preCalled)

	err = cmd.PersistentPostRunE(cmd, nil)
	assert.NoError(t, err)
	assert.True(t, postCalled)
}

func TestRegisterHooksNilExisting(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	RegisterHooks(cmd)

	// Should not panic with nil pre-existing hooks
	err := cmd.PersistentPreRunE(cmd, nil)
	assert.NoError(t, err)

	err = cmd.PersistentPostRunE(cmd, nil)
	assert.NoError(t, err)
}

func TestHookPreRunSkipsInCI(t *testing.T) {
	old := updateChecker
	updateChecker = nil
	defer func() { updateChecker = old }()

	t.Setenv("CI", "true")
	cmd := &cobra.Command{Use: "run"}

	hookPreRun(cmd)
	assert.Nil(t, updateChecker, "should skip update check in CI")
}

func TestHookPreRunSkipsCompletion(t *testing.T) {
	old := updateChecker
	updateChecker = nil
	defer func() { updateChecker = old }()

	cmd := &cobra.Command{Use: "completion"}
	hookPreRun(cmd)
	assert.Nil(t, updateChecker, "should skip update check for completion")
}
