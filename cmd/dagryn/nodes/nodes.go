// Package nodes provides the "dagryn nodes" CLI commands for worker management.
// Named "nodes" instead of "worker" to avoid collision with the existing
// cmd/dagryn/worker/ (background job worker). See review issue #13.
package nodes

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/config"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/spf13/cobra"
)

// NewCmd creates the "dagryn nodes" command group.
func NewCmd(flags *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "nodes",
		Short:   "Worker node management",
		Long:    "List, drain, and remove worker nodes from the cluster.",
		GroupID: "remote",
	}

	cmd.AddCommand(newListNodesCmd(flags))
	cmd.AddCommand(newDrainCmd(flags))
	cmd.AddCommand(newRemoveCmd(flags))

	return cmd
}

func getClusterStore(flags *cli.Flags) (repo.ClusterStore, func(), error) {
	config.LoadDotEnv()
	cfg, err := config.LoadConfig(config.ConfigOpts{ConfigFile: flags.CfgFile})
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	db, err := database.New(context.Background(), cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}

	return repo.NewClusterRepo(db.Pool()), func() { db.Close() }, nil
}

func newListNodesCmd(flags *cli.Flags) *cobra.Command {
	var clusterFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List worker nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, cleanup, err := getClusterStore(flags)
			if err != nil {
				return err
			}
			defer cleanup()

			var clusterID *uuid.UUID
			if clusterFilter != "" {
				id, err := uuid.Parse(clusterFilter)
				if err != nil {
					return fmt.Errorf("invalid cluster ID: %w", err)
				}
				clusterID = &id
			}

			workers, err := store.ListWorkers(cmd.Context(), clusterID, nil)
			if err != nil {
				return err
			}

			if len(workers) == 0 {
				cmd.Println("No workers found.")
				return nil
			}

			cmd.Printf("%-36s  %-20s  %-10s  %-8s  %-10s  %s\n", "ID", "HOSTNAME", "STATUS", "TASKS", "ENV", "ARCH")
			for _, w := range workers {
				cmd.Printf("%-36s  %-20s  %-10s  %-8d  %-10s  %s/%s\n",
					w.ID, w.Hostname, w.Status, w.ActiveTasks, w.Environment, w.OS, w.Arch)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "Filter by cluster ID")
	return cmd
}

func newDrainCmd(flags *cli.Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "drain [worker-id]",
		Short: "Drain a worker node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, cleanup, err := getClusterStore(flags)
			if err != nil {
				return err
			}
			defer cleanup()

			id, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid worker ID: %w", err)
			}

			if err := store.UpdateWorkerStatus(cmd.Context(), id, models.WorkerStatusDraining); err != nil {
				return err
			}

			cmd.Printf("Worker %s set to draining\n", args[0])
			return nil
		},
	}
}

func newRemoveCmd(flags *cli.Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove [worker-id]",
		Short: "Deregister a worker node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, cleanup, err := getClusterStore(flags)
			if err != nil {
				return err
			}
			defer cleanup()

			id, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid worker ID: %w", err)
			}

			if err := store.DeleteWorker(cmd.Context(), id); err != nil {
				return err
			}

			cmd.Printf("Worker %s removed\n", args[0])
			return nil
		},
	}
}
