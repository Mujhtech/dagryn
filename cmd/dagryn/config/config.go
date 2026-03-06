package config

import (
	"fmt"

	"github.com/mujhtech/dagryn/internal/cli"
	dagconfig "github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/spf13/cobra"
)

// NewCmd creates the config command.
func NewCmd(flags *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage dagryn configuration",
		Long:  `Commands for inspecting and validating dagryn configuration.`,
	}
	cmd.AddCommand(newValidateCmd(flags))
	return cmd
}

func newValidateCmd(flags *cli.Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the dagryn.toml configuration file",
		Long:  `Parse and validate the dagryn.toml configuration file, reporting any errors.`,
		Example: `  dagryn config validate
  dagryn config validate -c custom.toml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(flags.Verbose)

			cfg, err := dagconfig.Parse(flags.CfgFile)
			if err != nil {
				log.Errorf("Failed to parse config: %v", err)
				return cli.WrapError(err, nil)
			}

			errs := dagconfig.Validate(cfg)
			if len(errs) > 0 {
				log.Errorf("Configuration has %d error(s):", len(errs))
				for _, e := range errs {
					log.Errorf("  %s", e.Error())
				}
				return fmt.Errorf("configuration has %d validation error(s)", len(errs))
			}

			// Print summary
			log.Success("Configuration is valid")
			log.Infof("  Workflow: %s", cfg.Workflow.Name)
			log.Infof("  Tasks:    %d", len(cfg.Tasks))

			if cfg.Cache.IsEnabled() {
				cacheInfo := "local"
				if cfg.Cache.Remote.Enabled {
					cacheInfo = fmt.Sprintf("local + remote (%s)", cfg.Cache.Remote.Provider)
				}
				log.Infof("  Cache:    %s", cacheInfo)
			} else {
				log.Infof("  Cache:    disabled")
			}

			if len(cfg.Plugins) > 0 {
				log.Infof("  Plugins:  %d", len(cfg.Plugins))
			}

			return nil
		},
	}
}
