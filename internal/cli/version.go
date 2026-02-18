package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/mujhtech/dagryn/internal/version"
	"github.com/spf13/cobra"
)

var versionJSON bool

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of dagryn",
		Long:  `Print the version, commit hash, build date, Go version, and OS/arch.`,
		Example: `  dagryn version
  dagryn version --json`,
		RunE: runVersion,
	}
	cmd.Flags().BoolVar(&versionJSON, "json", false, "output version info as JSON")
	return cmd
}

func runVersion(cmd *cobra.Command, args []string) error {
	if versionJSON {
		info := map[string]string{
			"version":   version.Version,
			"commit":    version.Commit,
			"buildDate": version.BuildDate,
			"go":        runtime.Version(),
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
		}
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), version.Info())
	return nil
}
