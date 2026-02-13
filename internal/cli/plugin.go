package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/mujhtech/dagryn/internal/config"
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
	cmd.AddCommand(newPluginInfoCmd())
	cmd.AddCommand(newPluginUpdateCmd())
	cmd.AddCommand(newPluginInitCmd())
	cmd.AddCommand(newPluginValidateCmd())

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
			_, _ = fmt.Fprintln(w, "NAME\tSOURCE\tVERSION\tPATH")
			_, _ = fmt.Fprintln(w, "----\t------\t-------\t----")

			for _, p := range plugins {
				version := p.ResolvedVersion
				if version == "" {
					version = p.Version
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Source, version, p.BinaryPath)
			}
			_ = w.Flush()

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
				_, _ = fmt.Scanln(&response)
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

  # Install using short format (defaults to GitHub)
  dagryn plugin install golangci/golangci-lint@v1.55.0

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

func newPluginInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <plugin-spec>",
		Short: "Show plugin information",
		Long: `Fetch and display plugin.toml metadata from a plugin source.

Supports GitHub plugins (owner/repo), local plugins (local:path), and full specs (source:name@version).
Displays the plugin's name, description, type, platforms, and inputs.`,
		Example: `  # Show info for a GitHub plugin
  dagryn plugin info dagryn/setup-go
  # Show info for a local plugin
  dagryn plugin info local:./plugins/setup-node`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]

			// Try to parse the spec directly first (handles local: and full specs)
			spec := ref
			p, err := plugin.Parse(spec)
			if err != nil {
				// Fall back to short ref with dummy version for GitHub shorthand
				spec = ref + "@latest"
				p, err = plugin.Parse(spec)
				if err != nil {
					return fmt.Errorf("invalid plugin reference %q: %w", ref, err)
				}
			}

			// Resolve the plugin to fetch its manifest
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			manager := plugin.NewManager(projectRoot)
			resolved, err := manager.Resolve(cmd.Context(), spec)
			if err != nil {
				return fmt.Errorf("failed to resolve plugin: %w", err)
			}

			_ = p // used for initial parsing

			if resolved.Manifest == nil {
				fmt.Printf("Plugin: %s\n", resolved.Name)
				fmt.Printf("Source:  %s\n", resolved.Source)
				fmt.Printf("Version: %s\n", resolved.ResolvedVersion)
				fmt.Println("No plugin.toml manifest found.")
				return nil
			}

			m := resolved.Manifest
			fmt.Printf("Name:        %s\n", m.Plugin.Name)
			fmt.Printf("Description: %s\n", m.Plugin.Description)
			fmt.Printf("Version:     %s\n", m.Plugin.Version)
			fmt.Printf("Type:        %s\n", pluginTypeDisplay(m))
			if m.Plugin.Author != "" {
				fmt.Printf("Author:      %s\n", m.Plugin.Author)
			}
			if m.Plugin.License != "" {
				fmt.Printf("License:     %s\n", m.Plugin.License)
			}
			if m.Plugin.Homepage != "" {
				fmt.Printf("Homepage:    %s\n", m.Plugin.Homepage)
			}

			if len(m.Platforms) > 0 {
				fmt.Println("\nPlatforms:")
				for platform, asset := range m.Platforms {
					fmt.Printf("  %s -> %s\n", platform, asset)
				}
			}

			if len(m.Inputs) > 0 {
				fmt.Println("\nInputs:")
				for name, input := range m.Inputs {
					req := ""
					if input.Required {
						req = " (required)"
					}
					def := ""
					if input.Default != "" {
						def = fmt.Sprintf(" [default: %s]", input.Default)
					}
					fmt.Printf("  %s%s%s\n", name, req, def)
					if input.Description != "" {
						fmt.Printf("    %s\n", input.Description)
					}
				}
			}

			if m.IsComposite() && len(m.Steps) > 0 {
				fmt.Printf("\nSteps: %d\n", len(m.Steps))
				for i, step := range m.Steps {
					fmt.Printf("  %d. %s\n", i+1, step.Name)
				}
			}

			if m.IsIntegration() && len(m.Hooks) > 0 {
				fmt.Printf("\nHooks: %d\n", len(m.Hooks))
				for name := range m.Hooks {
					fmt.Printf("  - %s\n", name)
				}
			}

			return nil
		},
	}
}

func newPluginUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update plugins to latest versions",
		Long: `Check for newer versions of installed plugins and update them.

Reads plugin specs from dagryn.toml and compares with the lock file.
Plugins with "latest" or semver range versions will be re-resolved.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			// Load config to get plugin specs
			cfg, err := config.Parse(filepath.Join(projectRoot, config.DefaultConfigFile))
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Collect all plugin specs from tasks
			specs := make(map[string]bool)
			for _, spec := range cfg.Plugins {
				specs[spec] = true
			}
			for _, tc := range cfg.Tasks {
				for _, spec := range tc.GetPlugins() {
					// Resolve global plugin references
					if fullSpec, ok := cfg.Plugins[spec]; ok {
						specs[fullSpec] = true
					} else {
						specs[spec] = true
					}
				}
			}

			if len(specs) == 0 {
				fmt.Println("No plugins configured.")
				return nil
			}

			manager := plugin.NewManager(projectRoot)
			updated := 0

			for spec := range specs {
				p, err := plugin.Parse(spec)
				if err != nil {
					fmt.Printf("  Skipping %s: %v\n", spec, err)
					continue
				}

				// Only update non-exact versions
				if p.IsExactVersion() {
					continue
				}

				fmt.Printf("Checking %s...\n", spec)

				// Clean existing and re-install
				_ = manager.CleanPlugin(spec)

				result, err := manager.Install(cmd.Context(), spec)
				if err != nil {
					fmt.Printf("  Failed to update %s: %v\n", spec, err)
					continue
				}

				fmt.Printf("  Updated %s to %s\n", result.Plugin.Name, result.Plugin.ResolvedVersion)
				updated++
			}

			if updated == 0 {
				fmt.Println("All plugins are up to date.")
			} else {
				fmt.Printf("\nUpdated %d plugins.\n", updated)
			}

			return nil
		},
	}
}

func newPluginInitCmd() *cobra.Command {
	var pluginType string

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold a new plugin project",
		Long: `Create a new plugin project directory with a plugin.toml template,
README.md, LICENSE, and (for tool plugins) a GitHub Actions release workflow.

Use --type to select the plugin type:
  tool        - Wraps an external binary
  composite   - Multi-step shell commands (task template)
  integration - Lifecycle hooks (on_run_start, on_task_end, etc.)`,
		Example: `  # Create a new tool plugin
  dagryn plugin init my-tool --type tool

  # Create a new composite plugin
  dagryn plugin init setup-go --type composite

  # Create an integration plugin
  dagryn plugin init my-notifier --type integration

  # Interactive type selection
  dagryn plugin init my-plugin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if _, err := os.Stat(name); err == nil {
				return fmt.Errorf("directory %q already exists", name)
			}

			// Interactive type selection if not specified
			if pluginType == "" {
				fmt.Println("Select plugin type:")
				fmt.Println("  1. tool        - Wraps an external binary")
				fmt.Println("  2. composite   - Multi-step shell commands (task template)")
				fmt.Println("  3. integration - Lifecycle hooks (on_run_start, on_task_end, etc.)")
				fmt.Println()
				fmt.Print("Choice [1-3]: ")

				var choice string
				_, _ = fmt.Scanln(&choice)
				switch choice {
				case "1", "tool":
					pluginType = "tool"
				case "2", "composite":
					pluginType = "composite"
				case "3", "integration":
					pluginType = "integration"
				default:
					return fmt.Errorf("invalid choice %q; expected 1, 2, or 3", choice)
				}
			}

			// Validate type
			switch pluginType {
			case "tool", "composite", "integration":
			default:
				return fmt.Errorf("invalid plugin type %q; expected tool, composite, or integration", pluginType)
			}

			// Create directory structure
			if err := os.MkdirAll(name, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

			// Write plugin.toml based on type
			var manifest string
			switch pluginType {
			case "tool":
				manifest = pluginInitToolManifest(name)
			case "composite":
				manifest = pluginInitCompositeManifest(name)
			case "integration":
				manifest = pluginInitIntegrationManifest(name)
			}

			if err := os.WriteFile(filepath.Join(name, "plugin.toml"), []byte(manifest), 0644); err != nil {
				return fmt.Errorf("failed to write plugin.toml: %w", err)
			}

			// Write README.md
			readme := pluginInitReadme(name, pluginType)
			if err := os.WriteFile(filepath.Join(name, "README.md"), []byte(readme), 0644); err != nil {
				return fmt.Errorf("failed to write README.md: %w", err)
			}

			// Write LICENSE
			license := pluginInitLicense()
			if err := os.WriteFile(filepath.Join(name, "LICENSE"), []byte(license), 0644); err != nil {
				return fmt.Errorf("failed to write LICENSE: %w", err)
			}

			// Write release workflow only for tool plugins
			if pluginType == "tool" {
				if err := os.MkdirAll(filepath.Join(name, ".github", "workflows"), 0755); err != nil {
					return fmt.Errorf("failed to create workflow directory: %w", err)
				}
				workflow := pluginInitReleaseWorkflow(name)
				if err := os.WriteFile(filepath.Join(name, ".github", "workflows", "release.yml"), []byte(workflow), 0644); err != nil {
					return fmt.Errorf("failed to write release workflow: %w", err)
				}
			}

			// Print summary
			fmt.Printf("Created plugin project: %s/\n", name)
			fmt.Printf("  %s/plugin.toml\n", name)
			fmt.Printf("  %s/README.md\n", name)
			fmt.Printf("  %s/LICENSE\n", name)
			if pluginType == "tool" {
				fmt.Printf("  %s/.github/workflows/release.yml\n", name)
			}
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Edit plugin.toml with your plugin details")
			fmt.Println("  2. Run 'dagryn plugin validate' to verify")
			fmt.Println("  3. Push to GitHub and create a release")

			return nil
		},
	}

	cmd.Flags().StringVar(&pluginType, "type", "", "Plugin type: tool, composite, or integration")

	return cmd
}

func pluginInitToolManifest(name string) string {
	return fmt.Sprintf(`[plugin]
name = %q
description = "A Dagryn plugin"
version = "0.1.0"
type = "tool"
author = ""
license = "MIT"

[tool]
binary = %q

[platforms]
# "darwin-arm64" = "%s-darwin-arm64.tar.gz"
# "darwin-amd64" = "%s-darwin-amd64.tar.gz"
# "linux-amd64"  = "%s-linux-amd64.tar.gz"

# [inputs.config]
# description = "Path to configuration file"
# default = ""
`, name, name, name, name, name)
}

func pluginInitCompositeManifest(name string) string {
	return fmt.Sprintf(`[plugin]
name = %q
description = "A composite Dagryn plugin"
version = "0.1.0"
type = "composite"
author = ""
license = "MIT"

[inputs.example-input]
required = true
description = "An example required input"

[inputs.optional-input]
description = "An optional input with a default"
default = "default-value"

[[step]]
name = "validate"
command = 'echo "Running with input: ${inputs.example-input}"'

[[step]]
name = "execute"
command = 'echo "Executing main logic"'

[[cleanup]]
name = "teardown"
command = 'echo "Cleanup complete"'
`, name)
}

func pluginInitIntegrationManifest(name string) string {
	return fmt.Sprintf(`[plugin]
name = %q
description = "An integration Dagryn plugin"
version = "0.1.0"
type = "integration"
author = ""
license = "MIT"

[inputs.webhook-url]
required = true
description = "Webhook URL for notifications"

[hooks.on_run_success]
command = 'echo "Run $DAGRYN_RUN_ID succeeded"'

[hooks.on_run_failure]
command = 'echo "Run $DAGRYN_RUN_ID failed"'
`, name)
}

func pluginInitReadme(name, pluginType string) string {
	var usage, sections string

	switch pluginType {
	case "tool":
		usage = fmt.Sprintf(`[tasks.example]
uses = "your-org/%s@v0.1.0"
command = "%s --help"`, name, name)
		sections = `## Platforms

| Platform | Asset |
|----------|-------|
| darwin-arm64 | (configure in plugin.toml) |
| darwin-amd64 | (configure in plugin.toml) |
| linux-amd64  | (configure in plugin.toml) |`

	case "composite":
		usage = fmt.Sprintf(`[tasks.example]
uses = "your-org/%s@v0.1.0"
with = { example-input = "my-value" }`, name)
		sections = `## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| example-input | Yes | - | An example required input |
| optional-input | No | default-value | An optional input with a default |

## Steps

1. **validate** - Validates inputs
2. **execute** - Executes main logic`

	case "integration":
		usage = fmt.Sprintf(`[plugins.%s]
spec = "your-org/%s@v0.1.0"
with = { webhook-url = "https://example.com/webhook" }`, name, name)
		sections = `## Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| webhook-url | Yes | - | Webhook URL for notifications |

## Hooks

- **on_run_success** - Triggered when a run completes successfully
- **on_run_failure** - Triggered when a run fails`
	}

	return fmt.Sprintf(`# %s

A %s Dagryn plugin.

## Usage

`+"```toml\n%s\n```"+`

%s
`, name, pluginType, usage, sections)
}

func pluginInitLicense() string {
	return `MIT License

Copyright (c) <YEAR> <AUTHOR>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`
}

func pluginInitReleaseWorkflow(name string) string {
	return fmt.Sprintf(`name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build
        run: |
          # Add your build steps here
          echo "Building %s"

      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            # Add your release artifacts here
            # dist/*
`, name)
}

func newPluginValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate plugin.toml",
		Long:  `Read and validate the plugin.toml manifest in the current directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile("plugin.toml")
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no plugin.toml found in current directory")
				}
				return fmt.Errorf("failed to read plugin.toml: %w", err)
			}

			m, err := plugin.ParseManifest(data)
			if err != nil {
				fmt.Printf("Parse error: %v\n", err)
				return err
			}

			if err := plugin.ValidateManifest(m); err != nil {
				fmt.Printf("Validation error: %v\n", err)
				return err
			}

			fmt.Println("plugin.toml is valid.")
			fmt.Printf("  Name:    %s\n", m.Plugin.Name)
			fmt.Printf("  Version: %s\n", m.Plugin.Version)
			fmt.Printf("  Type:    %s\n", pluginTypeDisplay(m))

			if len(m.Platforms) > 0 {
				fmt.Printf("  Platforms: %d\n", len(m.Platforms))
			}
			if len(m.Inputs) > 0 {
				fmt.Printf("  Inputs:    %d\n", len(m.Inputs))
			}
			if m.IsComposite() {
				fmt.Printf("  Steps:     %d\n", len(m.Steps))
			}

			// Warn about missing documentation files.
			if _, err := os.Stat("README.md"); os.IsNotExist(err) {
				fmt.Println("\n  Warning: README.md not found. Consider adding a README for your plugin.")
			}
			if _, err := os.Stat("LICENSE"); os.IsNotExist(err) {
				fmt.Println("  Warning: LICENSE not found. Consider adding a license file for your plugin.")
			}

			return nil
		},
	}
}

func pluginTypeDisplay(m *plugin.Manifest) string {
	if m.IsIntegration() {
		return "integration"
	}
	if m.IsComposite() {
		return "composite"
	}
	return "tool"
}
