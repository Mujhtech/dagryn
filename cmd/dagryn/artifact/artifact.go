package artifact

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	projectID      string
	artifactOutput string
)

// NewCmd creates the artifact command.
func NewCmd(flags *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Manage run artifacts",
		Long:  `Commands for listing and downloading artifacts from remote runs.`,
	}

	cmd.PersistentFlags().StringVar(&projectID, "project", "", "project ID for artifact operations")

	cmd.AddCommand(newArtifactListCmd())
	cmd.AddCommand(newArtifactDownloadCmd(flags))
	return cmd
}

func newArtifactListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <run-id>",
		Short: "List artifacts for a run",
		Long:  `List all artifacts uploaded for a specific run.`,
		Example: `  dagryn artifact list 550e8400-e29b-41d4-a716-446655440000
  dagryn artifact list --project <id> <run-id>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid run ID: %w", err)
			}

			apiClient, projID, err := resolveArtifactClient()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			artifacts, err := apiClient.ListRunArtifacts(ctx, projID, runID)
			if err != nil {
				return fmt.Errorf("failed to list artifacts: %w", err)
			}

			if len(artifacts) == 0 {
				fmt.Println("No artifacts found for this run.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tTASK\tSIZE\tCREATED")
			_, _ = fmt.Fprintln(w, "----\t----\t----\t-------")
			for _, a := range artifacts {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					a.Name, a.TaskName, formatBytes(a.SizeBytes), a.CreatedAt.Format(time.RFC3339))
			}
			_ = w.Flush()
			fmt.Printf("\nTotal: %d artifacts\n", len(artifacts))
			return nil
		},
	}
}

func newArtifactDownloadCmd(flags *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <run-id> [artifact-name]",
		Short: "Download artifacts from a run",
		Long: `Download one or all artifacts from a run.

If an artifact name is specified, only that artifact is downloaded.
Otherwise all artifacts are downloaded.`,
		Example: `  dagryn artifact download <run-id>
  dagryn artifact download <run-id> build-output
  dagryn artifact download <run-id> --output ./artifacts`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(flags.Verbose)

			runID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid run ID: %w", err)
			}

			var artifactName string
			if len(args) > 1 {
				artifactName = args[1]
			}

			apiClient, projID, err := resolveArtifactClient()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			artifacts, err := apiClient.ListRunArtifacts(ctx, projID, runID)
			if err != nil {
				return fmt.Errorf("failed to list artifacts: %w", err)
			}

			if len(artifacts) == 0 {
				log.Info("No artifacts found for this run.")
				return nil
			}

			// Filter if a specific artifact name is given
			if artifactName != "" {
				var filtered []client.ArtifactResponse
				for _, a := range artifacts {
					if a.Name == artifactName {
						filtered = append(filtered, a)
					}
				}
				if len(filtered) == 0 {
					return fmt.Errorf("artifact %q not found in run %s", artifactName, runID)
				}
				artifacts = filtered
			}

			outputDir := artifactOutput
			if outputDir == "" {
				outputDir = "."
			}

			spinner := cliui.NewSpinner(os.Stderr, fmt.Sprintf("Downloading %d artifact(s)...", len(artifacts)))
			spinner.Start()

			downloaded := 0
			for _, a := range artifacts {
				body, err := apiClient.DownloadArtifact(ctx, projID, runID, a.ID)
				if err != nil {
					spinner.Stop("")
					log.Errorf("Failed to download %s: %v", a.Name, err)
					continue
				}

				destPath := filepath.Join(outputDir, a.FileName)
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					_ = body.Close()
					spinner.Stop("")
					return fmt.Errorf("failed to create directory: %w", err)
				}

				f, err := os.Create(destPath)
				if err != nil {
					_ = body.Close()
					spinner.Stop("")
					return fmt.Errorf("failed to create file: %w", err)
				}

				_, err = io.Copy(f, body)
				_ = body.Close()
				_ = f.Close()
				if err != nil {
					spinner.Stop("")
					return fmt.Errorf("failed to write file: %w", err)
				}
				downloaded++
			}

			spinner.Stop(fmt.Sprintf("Downloaded %d artifact(s) to %s", downloaded, outputDir))
			return nil
		},
	}

	cmd.Flags().StringVarP(&artifactOutput, "output", "o", "", "output directory (default: current directory)")
	return cmd
}

// resolveArtifactClient creates an API client with project resolution.
func resolveArtifactClient() (*client.Client, uuid.UUID, error) {
	store, err := client.NewCredentialsStore()
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil || creds == nil {
		return nil, uuid.Nil, fmt.Errorf("not logged in. Run 'dagryn auth login' first")
	}

	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 30 * time.Second,
	})
	apiClient.SetCredentials(creds)
	apiClient.SetCredentialsStore(store)

	// Resolve project ID: --project flag > .dagryn/project.json > .dagryn/context.json
	var projID uuid.UUID
	if projectID != "" {
		projID, err = uuid.Parse(projectID)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("invalid project ID: %w", err)
		}
	} else {
		projectRoot, err := cli.GetProjectRoot()
		if err != nil {
			return nil, uuid.Nil, err
		}
		projectStore := client.NewProjectConfigStore(projectRoot)
		projectConfig, err := projectStore.Load()
		if err == nil && projectConfig != nil {
			projID = projectConfig.ProjectID
		} else {
			// Fallback to context.json
			contextID := cli.LoadContextProjectID(projectRoot)
			if contextID != "" {
				projID, err = uuid.Parse(contextID)
				if err != nil {
					return nil, uuid.Nil, fmt.Errorf("invalid context project ID: %w", err)
				}
			}
		}
	}

	if projID == uuid.Nil {
		return nil, uuid.Nil, fmt.Errorf("no project linked. Run 'dagryn init --remote' or 'dagryn use <project-id>'")
	}

	return apiClient, projID, nil
}

// formatBytes formats a byte count for display.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
