package cluster

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/config"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/spf13/cobra"
)

// NewCmd creates the "dagryn cluster" command group.
func NewCmd(flags *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Cluster management commands",
		Long:    "Manage clusters for distributed worker pools.",
		GroupID: "remote",
	}

	cmd.AddCommand(newListCmd(flags))
	cmd.AddCommand(newCreateCmd(flags))
	cmd.AddCommand(newDeleteCmd(flags))

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

func newListCmd(flags *cli.Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, cleanup, err := getClusterStore(flags)
			if err != nil {
				return err
			}
			defer cleanup()

			clusters, err := store.ListClusters(cmd.Context())
			if err != nil {
				return err
			}

			if len(clusters) == 0 {
				cmd.Println("No clusters found.")
				return nil
			}

			cmd.Printf("%-36s  %-20s  %s\n", "ID", "NAME", "DESCRIPTION")
			for _, c := range clusters {
				cmd.Printf("%-36s  %-20s  %s\n", c.ID, c.Name, c.Description)
			}
			return nil
		},
	}
}

func newCreateCmd(flags *cli.Flags) *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, cleanup, err := getClusterStore(flags)
			if err != nil {
				return err
			}
			defer cleanup()

			cluster := &models.Cluster{
				Name:        args[0],
				Description: description,
				Labels:      json.RawMessage(`{}`),
			}

			if err := store.CreateCluster(cmd.Context(), cluster); err != nil {
				return err
			}

			cmd.Printf("Cluster created: %s (%s)\n", cluster.Name, cluster.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Cluster description")
	return cmd
}

func newDeleteCmd(flags *cli.Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, cleanup, err := getClusterStore(flags)
			if err != nil {
				return err
			}
			defer cleanup()

			id, err := uuid.Parse(args[0])
			if err != nil {
				return err
			}

			if err := store.DeleteCluster(cmd.Context(), id); err != nil {
				return err
			}

			cmd.Printf("Cluster deleted: %s\n", args[0])
			return nil
		},
	}
}
