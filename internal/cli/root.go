package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	verbose   bool
	noCache   bool
	noPlugins bool
	cfgFile   string
)

var rootCmd = &cobra.Command{
	Use:   "dagryn",
	Short: "Dagryn - local-first workflow orchestrator",
	Long: `Dagryn is a local-first, self-hosted developer workflow orchestrator
focused on speed, determinism, and great developer experience.

It lets you define workflows as explicit task graphs (DAGs) that run
the same way locally and in CI.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "disable caching")
	rootCmd.PersistentFlags().BoolVar(&noPlugins, "no-plugins", false, "disable plugin installation")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "dagryn.toml", "config file")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(newPluginCmd())
	rootCmd.AddCommand(newAuthCmd())
}

// getProjectRoot returns the current working directory as the project root.
func getProjectRoot() (string, error) {
	return os.Getwd()
}
