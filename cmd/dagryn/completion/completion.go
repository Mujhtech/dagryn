package completion

import (
	"fmt"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/spf13/cobra"
)

// NewCmd creates the completion command.
// It needs the root command to generate shell completions.
func NewCmd(_ *cli.Flags, root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		Long:      `Generate shell completion scripts for dagryn.`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Example: `  # Bash (add to ~/.bashrc)
  source <(dagryn completion bash)

  # Zsh (add to ~/.zshrc)
  source <(dagryn completion zsh)

  # Fish
  dagryn completion fish | source

  # PowerShell
  dagryn completion powershell | Out-String | Invoke-Expression`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
	return cmd
}
