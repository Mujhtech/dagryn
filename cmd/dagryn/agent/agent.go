package agent

import (
	"context"
	"strings"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/agent"
	"github.com/spf13/cobra"
)

// NewCmd creates the "dagryn agent" command group.
func NewCmd(_ *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "agent",
		Short:   "Worker agent commands",
		Long:    "Manage the Dagryn worker agent that connects to a control plane for distributed task execution.",
		GroupID: "remote",
	}

	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newStartCmd() *cobra.Command {
	var (
		serverAddr   string
		token        string
		labelsRaw    string
		maxTasks     int
		heartbeatSec int
		workDir      string
		clusterName  string
		tlsCertFile  string
		tlsKeyFile   string
		tlsCAFile    string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the worker agent",
		Long:  "Start a worker agent that connects to the Dagryn control plane for distributed task execution.",
		RunE: func(cmd *cobra.Command, args []string) error {
			labels := parseLabels(labelsRaw)

			cfg := agent.Config{
				ServerAddr:    serverAddr,
				Token:         token,
				Labels:        labels,
				MaxConcurrent: maxTasks,
				HeartbeatSec:  heartbeatSec,
				WorkDir:       workDir,
				ClusterName:   clusterName,
				TLSCertFile:   tlsCertFile,
				TLSKeyFile:    tlsKeyFile,
				TLSCAFile:     tlsCAFile,
			}

			a := agent.New(cfg)
			return a.Start(context.Background())
		},
	}

	cmd.Flags().StringVar(&serverAddr, "server", "localhost:9001", "Control plane gRPC address")
	cmd.Flags().StringVar(&token, "token", "", "Registration token")
	cmd.Flags().StringVar(&labelsRaw, "labels", "", "Worker labels (key=value,key=value)")
	cmd.Flags().IntVar(&maxTasks, "max-tasks", 4, "Maximum concurrent tasks")
	cmd.Flags().IntVar(&heartbeatSec, "heartbeat", 10, "Heartbeat interval in seconds")
	cmd.Flags().StringVar(&workDir, "workdir", "", "Base directory for task workspaces")
	cmd.Flags().StringVar(&clusterName, "cluster", "", "Cluster name to join")
	cmd.Flags().StringVar(&tlsCertFile, "tls-cert", "", "TLS client certificate file")
	cmd.Flags().StringVar(&tlsKeyFile, "tls-key", "", "TLS client key file")
	cmd.Flags().StringVar(&tlsCAFile, "tls-ca", "", "TLS CA certificate file")

	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show agent connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Agent status: not implemented yet")
			return nil
		},
	}
}

func parseLabels(raw string) map[string]string {
	labels := make(map[string]string)
	if raw == "" {
		return labels
	}
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			labels[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return labels
}
