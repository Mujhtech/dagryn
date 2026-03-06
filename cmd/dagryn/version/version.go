package version

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/mujhtech/dagryn/internal/cli"
	pkgversion "github.com/mujhtech/dagryn/internal/version"
	"github.com/spf13/cobra"
)

// NewCmd creates the version command.
func NewCmd(_ *cli.Flags) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of dagryn",
		Long:  `Print the version, commit hash, build date, Go version, and OS/arch.`,
		Example: `  dagryn version
  dagryn version --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				info := map[string]string{
					"version":   pkgversion.Version,
					"commit":    pkgversion.Commit,
					"buildDate": pkgversion.BuildDate,
					"go":        runtime.Version(),
					"os":        runtime.GOOS,
					"arch":      runtime.GOARCH,
				}
				data, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), pkgversion.Info())
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output version info as JSON")
	return cmd
}
