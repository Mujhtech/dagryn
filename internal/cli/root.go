package cli

import (
	"os"

	"github.com/mujhtech/dagryn/internal/version"
	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	verbose       bool
	noCache       bool
	noPlugins     bool
	noRemoteCache bool
	cfgFile       string
)

var rootCmd = &cobra.Command{
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

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Version
	rootCmd.Version = version.Short()

	// Cobra suggestions for typo correction
	rootCmd.SuggestionsMinimumDistance = 2

	// Persistent flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "disable caching")
	rootCmd.PersistentFlags().BoolVar(&noPlugins, "no-plugins", false, "disable plugin installation")
	rootCmd.PersistentFlags().BoolVar(&noRemoteCache, "no-remote-cache", false, "disable remote caching")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "dagryn.toml", "config file")

	// Command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "remote", Title: "Remote Commands:"},
		&cobra.Group{ID: "tools", Title: "Tool Commands:"},
		&cobra.Group{ID: "server", Title: "Server Commands:"},
	)

	// Core commands
	initCmd.GroupID = "core"
	runCmd.GroupID = "core"
	graphCmd.GroupID = "core"

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(graphCmd)

	configCmd := newConfigCmd()
	configCmd.GroupID = "core"
	rootCmd.AddCommand(configCmd)

	// Remote commands
	authCmd := newAuthCmd()
	authCmd.GroupID = "remote"
	rootCmd.AddCommand(authCmd)

	cacheCmd := newCacheCmd()
	cacheCmd.GroupID = "remote"
	rootCmd.AddCommand(cacheCmd)

	useCmd := newUseCmd()
	useCmd.GroupID = "remote"
	rootCmd.AddCommand(useCmd)

	artifactCmd := newArtifactCmd()
	artifactCmd.GroupID = "remote"
	rootCmd.AddCommand(artifactCmd)

	// Tool commands
	pluginCmd := newPluginCmd()
	pluginCmd.GroupID = "tools"
	rootCmd.AddCommand(pluginCmd)

	doctorCmd := newDoctorCmd()
	doctorCmd.GroupID = "tools"
	rootCmd.AddCommand(doctorCmd)

	completionCmd := newCompletionCmd()
	completionCmd.GroupID = "tools"
	rootCmd.AddCommand(completionCmd)

	versionCmd := newVersionCmd()
	versionCmd.GroupID = "tools"
	rootCmd.AddCommand(versionCmd)

	// Register Cobra hooks (update check, etc.)
	registerHooks(rootCmd)
}

// getProjectRoot returns the current working directory as the project root.
func getProjectRoot() (string, error) {
	return os.Getwd()
}
