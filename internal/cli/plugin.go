package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/mujhtech/dagryn/internal/plugin"
	"github.com/spf13/cobra"
)

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage plugins",
		Long:  `Commands for managing Dagryn plugins.`,
	}

	cmd.AddCommand(newPluginListCmd())
	cmd.AddCommand(newPluginCleanCmd())
	cmd.AddCommand(newPluginInstallCmd())

	return cmd
}

func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Long:  `Lists all plugins currently installed in the project's .dagryn/plugins directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			manager := plugin.NewManager(projectRoot)
			plugins := manager.List()

			if len(plugins) == 0 {
				fmt.Println("No plugins installed.")
				fmt.Println()
				fmt.Println("Plugins are automatically installed when you run tasks that use them.")
				fmt.Println("You can also install plugins manually with: dagryn plugin install <spec>")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSOURCE\tVERSION\tPATH")
			fmt.Fprintln(w, "----\t------\t-------\t----")

			for _, p := range plugins {
				version := p.ResolvedVersion
				if version == "" {
					version = p.Version
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Source, version, p.BinaryPath)
			}
			w.Flush()

			fmt.Printf("\nTotal: %d plugins\n", len(plugins))
			fmt.Printf("Location: %s\n", manager.PluginDir())

			return nil
		},
	}
}

func newPluginCleanCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "clean [spec]",
		Short: "Remove installed plugins",
		Long: `Remove installed plugins from the cache.

Without arguments, this command will ask for confirmation before cleaning.
With a specific plugin spec, only that plugin will be removed.
Use --all to remove all plugins without confirmation.`,
		Example: `  # Remove a specific plugin
  dagryn plugin clean github:golangci/golangci-lint@v1.55.0

  # Remove all plugins
  dagryn plugin clean --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			manager := plugin.NewManager(projectRoot)

			if len(args) > 0 {
				// Remove specific plugin
				spec := args[0]
				if err := manager.CleanPlugin(spec); err != nil {
					return fmt.Errorf("failed to remove plugin: %w", err)
				}
				fmt.Printf("Removed plugin: %s\n", spec)
				return nil
			}

			if !all {
				// Check if there are plugins to clean
				plugins := manager.List()
				if len(plugins) == 0 {
					fmt.Println("No plugins installed.")
					return nil
				}

				fmt.Printf("This will remove %d installed plugins.\n", len(plugins))
				fmt.Print("Continue? [y/N] ")

				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			if err := manager.Clean(); err != nil {
				return fmt.Errorf("failed to clean plugins: %w", err)
			}

			fmt.Println("All plugins removed.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Remove all plugins without confirmation")

	return cmd
}

func newPluginInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <spec>",
		Short: "Install a plugin",
		Long: `Manually install a plugin.

Plugins are usually installed automatically when running tasks that use them.
This command allows you to pre-install plugins.`,
		Example: `  # Install from GitHub releases
  dagryn plugin install github:golangci/golangci-lint@v1.55.0

  # Install via go install
  dagryn plugin install go:golang.org/x/tools/cmd/goimports@latest

  # Install via npm
  dagryn plugin install npm:prettier@3.0.0

  # Install via pip
  dagryn plugin install pip:black@23.12.0

  # Install via cargo
  dagryn plugin install cargo:ripgrep@14.0.3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec := args[0]

			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			manager := plugin.NewManager(projectRoot)

			fmt.Printf("Installing %s...\n", spec)

			result, err := manager.Install(cmd.Context(), spec)
			if err != nil {
				return fmt.Errorf("failed to install plugin: %w", err)
			}

			switch result.Status {
			case plugin.StatusInstalled:
				fmt.Printf("Installed %s\n", result.Plugin.Name)
				fmt.Printf("  Version: %s\n", result.Plugin.ResolvedVersion)
				fmt.Printf("  Binary:  %s\n", result.Plugin.BinaryPath)
			case plugin.StatusCached:
				fmt.Printf("Plugin %s already installed (cached)\n", result.Plugin.Name)
				fmt.Printf("  Binary: %s\n", result.Plugin.BinaryPath)
			default:
				return fmt.Errorf("unexpected status: %s", result.Status)
			}

			return nil
		},
	}
}

// pluginDirExists checks if the plugin directory exists.
func pluginDirExists(projectRoot string) bool {
	pluginDir := filepath.Join(projectRoot, ".dagryn", "plugins")
	info, err := os.Stat(pluginDir)
	return err == nil && info.IsDir()
}
