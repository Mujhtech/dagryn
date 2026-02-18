package cli

import (
	"os"

	"github.com/mujhtech/dagryn/internal/version"
	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/spf13/cobra"
)

// Flags holds the global CLI flags shared across all commands.
type Flags struct {
	Verbose       bool
	NoCache       bool
	NoPlugins     bool
	NoRemoteCache bool
	CfgFile       string
}

// GlobalFlags is the singleton holding current global flag values.
// Command packages receive a pointer to this struct via their NewCmd factory.
var GlobalFlags = &Flags{}

// NewRootCmd creates the root cobra command with persistent flags and groups.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "dagryn",
		Short: "Dagryn - local-first workflow orchestrator",
		Long: cliui.Banner(version.Short(), os.Getenv("NO_COLOR") != "") + `
Dagryn is a local-first, self-hosted developer workflow orchestrator
focused on speed, determinism, and great developer experience.

It lets you define workflows as explicit task graphs (DAGs) that run
the same way locally and in CI.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Version
	root.Version = version.Short()

	// Cobra suggestions for typo correction
	root.SuggestionsMinimumDistance = 2

	// Persistent flags bound to GlobalFlags
	root.PersistentFlags().BoolVarP(&GlobalFlags.Verbose, "verbose", "v", false, "verbose output")
	root.PersistentFlags().BoolVar(&GlobalFlags.NoCache, "no-cache", false, "disable caching")
	root.PersistentFlags().BoolVar(&GlobalFlags.NoPlugins, "no-plugins", false, "disable plugin installation")
	root.PersistentFlags().BoolVar(&GlobalFlags.NoRemoteCache, "no-remote-cache", false, "disable remote caching")
	root.PersistentFlags().StringVarP(&GlobalFlags.CfgFile, "config", "c", "dagryn.toml", "config file")

	// Command groups
	root.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "remote", Title: "Remote Commands:"},
		&cobra.Group{ID: "tools", Title: "Tool Commands:"},
		&cobra.Group{ID: "server", Title: "Server Commands:"},
	)

	return root
}

// GetProjectRoot returns the current working directory as the project root.
func GetProjectRoot() (string, error) {
	return os.Getwd()
}
