package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/mujhtech/dagryn/cmd/dagryn/ai"
	"github.com/mujhtech/dagryn/cmd/dagryn/artifact"
	"github.com/mujhtech/dagryn/cmd/dagryn/auth"
	"github.com/mujhtech/dagryn/cmd/dagryn/cache"
	"github.com/mujhtech/dagryn/cmd/dagryn/completion"
	"github.com/mujhtech/dagryn/cmd/dagryn/config"
	"github.com/mujhtech/dagryn/cmd/dagryn/doctor"
	"github.com/mujhtech/dagryn/cmd/dagryn/graph"
	"github.com/mujhtech/dagryn/cmd/dagryn/init"
	"github.com/mujhtech/dagryn/cmd/dagryn/license"
	"github.com/mujhtech/dagryn/cmd/dagryn/migrate"
	"github.com/mujhtech/dagryn/cmd/dagryn/plugin"
	"github.com/mujhtech/dagryn/cmd/dagryn/run"
	"github.com/mujhtech/dagryn/cmd/dagryn/server"
	"github.com/mujhtech/dagryn/cmd/dagryn/use"
	"github.com/mujhtech/dagryn/cmd/dagryn/version"
	"github.com/mujhtech/dagryn/cmd/dagryn/worker"
	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/spf13/cobra"
)

func main() {
	root := cli.NewRootCmd()
	flags := cli.GlobalFlags

	// Core commands
	addCmd(root, "core", initcmd.NewCmd(flags))
	addCmd(root, "core", run.NewCmd(flags))
	addCmd(root, "core", graph.NewCmd(flags))
	addCmd(root, "core", config.NewCmd(flags))

	// Remote commands
	addCmd(root, "remote", auth.NewCmd(flags))
	addCmd(root, "remote", cache.NewCmd(flags))
	addCmd(root, "remote", use.NewCmd(flags))
	addCmd(root, "remote", artifact.NewCmd(flags))

	// Tool commands
	addCmd(root, "tools", plugin.NewCmd(flags))
	addCmd(root, "tools", doctor.NewCmd(flags))
	addCmd(root, "tools", completion.NewCmd(flags, root))
	addCmd(root, "tools", version.NewCmd(flags))
	addCmd(root, "tools", ai.NewCmd(flags))

	// Server commands
	addCmd(root, "server", server.NewCmd(flags))
	addCmd(root, "server", worker.NewCmd(flags))
	addCmd(root, "server", migrate.NewCmd(flags))
	addCmd(root, "server", license.NewCmd(flags))

	// Register hooks (update check, etc.)
	cli.RegisterHooks(root)

	if err := root.Execute(); err != nil {
		noColor := os.Getenv("NO_COLOR") != ""

		var cliErr *cli.CLIError
		if errors.As(err, &cliErr) {
			msg := cliui.Render(cliui.StyleError, cliErr.Message, noColor)
			fmt.Fprintln(os.Stderr, msg)
			if cliErr.Suggestion != "" {
				hint := cliui.Render(cliui.StyleDim, cliErr.Suggestion, noColor)
				fmt.Fprintln(os.Stderr, hint)
			}
			code := cliErr.ExitCode
			if code == 0 {
				code = 1
			}
			os.Exit(code)
		}
		msg := cliui.Render(cliui.StyleError, err.Error(), noColor)
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

func addCmd(root *cobra.Command, group string, cmd *cobra.Command) {
	cmd.GroupID = group
	root.AddCommand(cmd)
}
