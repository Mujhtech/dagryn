package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mujhtech/dagryn/internal/cli"
	pkgconfig "github.com/mujhtech/dagryn/pkg/config"
	pkgserver "github.com/mujhtech/dagryn/pkg/server"
)

// NewCmd creates the server command.
func NewCmd(_ *cli.Flags) *cobra.Command {
	var opts pkgconfig.ConfigOpts

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the Dagryn API server",
		Long: `Starts the Dagryn API server which provides:

- REST API for teams, projects, and workflow runs
- OAuth authentication (GitHub, Google)
- API key management
- Real-time log streaming via SSE
- Swagger API documentation

Configuration priority: CLI flags > environment variables > config file > defaults`,
		Example: `  # Start with defaults
  dagryn server

  # Start with custom config file
  dagryn server --config /etc/dagryn/server.toml

  # Start with custom port and database
  dagryn server --port 3000 --database "postgres://localhost/dagryn"

  # Start with environment variables
  JWT_SECRET=xxx DATABASE_URL=postgres://... dagryn server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(opts)
		},
	}

	// Server flags
	cmd.Flags().StringVarP(&opts.ConfigFile, "config", "c", "", "Config file path")
	cmd.Flags().StringVar(&opts.Host, "host", "", "Server host (default: localhost)")
	cmd.Flags().IntVarP(&opts.Port, "port", "p", 0, "Server port (default: 9000)")

	// Database flags
	cmd.Flags().StringVar(&opts.Database, "database", "", "Database URL")

	// Auth flags
	cmd.Flags().StringVar(&opts.JWTSecret, "jwt-secret", "", "JWT signing secret (min 32 chars)")

	// OAuth flags
	cmd.Flags().StringVar(&opts.GitHubClientID, "github-client-id", "", "GitHub OAuth client ID")
	cmd.Flags().StringVar(&opts.GitHubClientSecret, "github-client-secret", "", "GitHub OAuth client secret")
	cmd.Flags().StringVar(&opts.GoogleClientID, "google-client-id", "", "Google OAuth client ID")
	cmd.Flags().StringVar(&opts.GoogleClientSecret, "google-client-secret", "", "Google OAuth client secret")

	return cmd
}

func runServer(opts pkgconfig.ConfigOpts) error {
	// Load configuration
	cfg, err := pkgconfig.LoadConfig(opts)
	if err != nil {
		return err
	}

	// Create server
	srv := pkgserver.New(cfg)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize server (database, telemetry, etc.)
	if err := srv.Initialize(ctx); err != nil {
		return err
	}

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		log.Info().Msg("Received shutdown signal")
		cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Error during shutdown")
		}
	}()

	// Start server
	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Bool("swagger", cfg.Server.Swagger.Enabled).
		Msg("Starting Dagryn API server")

	return srv.Start()
}
