package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/cliui"
)

func main() {
	if err := cli.Execute(); err != nil {
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
		// Print the error since Cobra's SilenceErrors is set.
		msg := cliui.Render(cliui.StyleError, err.Error(), noColor)
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
