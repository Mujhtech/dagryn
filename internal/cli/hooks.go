package cli

import (
	"os"
	"path/filepath"
	"time"

	"github.com/mujhtech/dagryn/internal/version"
	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/spf13/cobra"
)

const (
	githubRepo = "mujhtech/dagryn"
	// Maximum time to wait for the update check at the end of a command.
	updateCollectTimeout = 200 * time.Millisecond
)

// updateChecker is initialised in PersistentPreRun and read in PersistentPostRun.
var updateChecker *cliui.UpdateChecker

// registerHooks wires Cobra PersistentPre/PostRunE on the root command.
// It must be called during init after all subcommands have been added.
func registerHooks(root *cobra.Command) {
	// Wrap any existing hooks so they are preserved.
	prevPre := root.PersistentPreRunE
	prevPost := root.PersistentPostRunE

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if prevPre != nil {
			if err := prevPre(cmd, args); err != nil {
				return err
			}
		}
		hookPreRun(cmd)
		return nil
	}

	root.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if prevPost != nil {
			if err := prevPost(cmd, args); err != nil {
				return err
			}
		}
		hookPostRun(cmd)
		return nil
	}
}

// hookPreRun starts non-blocking background tasks.
func hookPreRun(cmd *cobra.Command) {
	// Skip update check in CI or when explicitly opted out.
	if os.Getenv("CI") != "" || os.Getenv("DAGRYN_NO_UPDATE_CHECK") != "" {
		return
	}

	// Skip for commands where the check adds no value.
	switch cmd.Name() {
	case "completion", "help", "__complete":
		return
	}

	cacheDir := dagrynUserDir()
	if cacheDir == "" {
		return
	}

	updateChecker = cliui.NewUpdateChecker(version.Short(), githubRepo, cacheDir)
	updateChecker.CheckInBackground()
}

// hookPostRun prints results from background tasks.
func hookPostRun(_ *cobra.Command) {
	if updateChecker == nil {
		return
	}

	noColor := os.Getenv("NO_COLOR") != ""
	result := updateChecker.CollectResult(updateCollectTimeout)
	cliui.PrintUpdateNotice(os.Stderr, version.Short(), result, noColor)
}

// dagrynUserDir returns ~/.dagryn, creating it if needed.
func dagrynUserDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".dagryn")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
