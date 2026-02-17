package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mujhtech/dagryn/pkg/database"
)

func init() {
	rootCmd.AddCommand(newMigrateCmd())
}

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
		Long:  `Commands for managing database migrations.`,
	}

	cmd.AddCommand(newMigrateUpCmd())
	cmd.AddCommand(newMigrateDownCmd())
	cmd.AddCommand(newMigrateStatusCmd())

	return cmd
}

func newMigrateUpCmd() *cobra.Command {
	var databaseURL string

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Run all pending migrations",
		Long:  `Applies all pending database migrations in order.`,
		Example: `  # Run migrations using DATABASE_URL env var
  dagryn migrate up

  # Run migrations with explicit database URL
  dagryn migrate up --database "postgres://localhost/dagryn"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateUp(databaseURL)
		},
	}

	cmd.Flags().StringVar(&databaseURL, "database", "", "Database URL (or set DATABASE_URL env var)")

	return cmd
}

func newMigrateDownCmd() *cobra.Command {
	var databaseURL string
	var steps int

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Rollback the last migration",
		Long:  `Rolls back the most recently applied migration.`,
		Example: `  # Rollback last migration
  dagryn migrate down

  # Rollback multiple migrations
  dagryn migrate down --steps 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateDown(databaseURL, steps)
		},
	}

	cmd.Flags().StringVar(&databaseURL, "database", "", "Database URL (or set DATABASE_URL env var)")
	cmd.Flags().IntVar(&steps, "steps", 1, "Number of migrations to rollback")

	return cmd
}

func newMigrateStatusCmd() *cobra.Command {
	var databaseURL string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		Long:  `Shows the current status of all migrations.`,
		Example: `  # Show migration status
  dagryn migrate status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateStatus(databaseURL)
		},
	}

	cmd.Flags().StringVar(&databaseURL, "database", "", "Database URL (or set DATABASE_URL env var)")

	return cmd
}

func getDatabaseURL(override string) string {
	if override != "" {
		return override
	}
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	if url := os.Getenv("DAGRYN_DATABASE_URL"); url != "" {
		return url
	}
	if url := os.Getenv("POSTGRES_URL"); url != "" {
		return url
	}
	return database.DefaultConfig().URL
}

func runMigrateUp(databaseURL string) error {
	ctx := context.Background()

	cfg := database.DefaultConfig()
	cfg.URL = getDatabaseURL(databaseURL)

	log.Info().Str("database", maskDatabaseURL(cfg.URL)).Msg("Connecting to database")

	database, err := database.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	log.Info().Msg("Running migrations")

	if err := database.Migrate(ctx); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Info().Msg("Migrations completed successfully")
	return nil
}

func runMigrateDown(databaseURL string, steps int) error {
	ctx := context.Background()

	cfg := database.DefaultConfig()
	cfg.URL = getDatabaseURL(databaseURL)

	log.Info().Str("database", maskDatabaseURL(cfg.URL)).Msg("Connecting to database")

	database, err := database.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	for i := 0; i < steps; i++ {
		log.Info().Int("step", i+1).Int("total", steps).Msg("Rolling back migration")

		if err := database.MigrateDown(ctx); err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}
	}

	log.Info().Int("steps", steps).Msg("Rollback completed successfully")
	return nil
}

func runMigrateStatus(databaseURL string) error {
	ctx := context.Background()

	cfg := database.DefaultConfig()
	cfg.URL = getDatabaseURL(databaseURL)

	database, err := database.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	status, err := database.MigrationStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	if len(status) == 0 {
		fmt.Println("No migrations found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tSTATUS\tAPPLIED AT")
	_, _ = fmt.Fprintln(w, "-------\t----\t------\t----------")

	var pending, applied int
	for _, m := range status {
		statusStr := "pending"
		appliedAt := "-"
		if m.Applied {
			statusStr = "applied"
			applied++
			if m.AppliedAt != nil {
				appliedAt = m.AppliedAt.Format("2006-01-02 15:04:05")
			}
		} else {
			pending++
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", m.Version, m.Name, statusStr, appliedAt)
	}
	_ = w.Flush()

	fmt.Printf("\nTotal: %d migrations (%d applied, %d pending)\n", len(status), applied, pending)

	return nil
}

// maskDatabaseURL masks sensitive parts of the database URL for logging.
func maskDatabaseURL(url string) string {
	// Simple masking - hide password if present
	// postgres://user:password@host/db -> postgres://user:***@host/db
	for i := 0; i < len(url); i++ {
		if url[i] == ':' && i+2 < len(url) && url[i+1] == '/' && url[i+2] == '/' {
			// Found ://
			start := i + 3
			// Find @ symbol
			for j := start; j < len(url); j++ {
				if url[j] == '@' {
					// Found user:pass@
					// Find the : in user:pass
					for k := start; k < j; k++ {
						if url[k] == ':' {
							// Mask the password
							return url[:k+1] + "***" + url[j:]
						}
					}
					break
				}
			}
			break
		}
	}
	return url
}
