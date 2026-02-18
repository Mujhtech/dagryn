package use

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/spf13/cobra"
)

// NewCmd creates the use command.
func NewCmd(flags *cli.Flags) *cobra.Command {
	var useClear bool

	cmd := &cobra.Command{
		Use:   "use <project-id>",
		Short: "Set the active project context",
		Long: `Set or clear the active project context.

This saves the project ID to .dagryn/context.json so that commands
like 'dagryn run --sync' can automatically resolve the project
without needing the --project flag.`,
		Example: `  dagryn use 550e8400-e29b-41d4-a716-446655440000
  dagryn use --clear`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUse(cmd, args, flags, useClear)
		},
	}
	cmd.Flags().BoolVar(&useClear, "clear", false, "clear the active project context")
	return cmd
}

func runUse(cmd *cobra.Command, args []string, flags *cli.Flags, useClear bool) error {
	log := logger.New(flags.Verbose)

	projectRoot, err := cli.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	contextPath := filepath.Join(projectRoot, ".dagryn", "context.json")

	if useClear {
		if err := os.Remove(contextPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clear context: %w", err)
		}
		log.Success("Project context cleared")
		return nil
	}

	if len(args) == 0 {
		// Show current context
		data, err := os.ReadFile(contextPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Info("No active project context.")
				log.Info("Run 'dagryn use <project-id>' to set one.")
				return nil
			}
			return fmt.Errorf("failed to read context: %w", err)
		}
		var ctx cli.ContextConfig
		if err := json.Unmarshal(data, &ctx); err != nil {
			return fmt.Errorf("failed to parse context: %w", err)
		}
		log.Infof("Active project: %s", ctx.ProjectID)
		if ctx.ProjectName != "" {
			log.Infof("  Name: %s", ctx.ProjectName)
		}
		log.Infof("  Set:  %s", ctx.SetAt)
		return nil
	}

	projectIDStr := args[0]
	projID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return fmt.Errorf("invalid project ID %q: must be a valid UUID", projectIDStr)
	}

	// Attempt to validate project exists on server
	projectName := ""
	store, err := client.NewCredentialsStore()
	if err == nil {
		creds, err := store.Load()
		if err == nil && creds != nil {
			apiClient := client.New(client.Config{
				BaseURL: creds.ServerURL,
				Timeout: 10 * time.Second,
			})
			apiClient.SetCredentials(creds)
			apiClient.SetCredentialsStore(store)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			project, err := apiClient.GetProject(ctx, projID)
			if err != nil {
				log.Warnf("Could not validate project on server: %v", err)
				log.Info("Setting context anyway — the project will be validated on next sync.")
			} else {
				projectName = project.Name
			}
		}
	}

	// Save context
	if err := os.MkdirAll(filepath.Dir(contextPath), 0755); err != nil {
		return fmt.Errorf("failed to create .dagryn directory: %w", err)
	}

	ctx := cli.ContextConfig{
		ProjectID:   projID.String(),
		ProjectName: projectName,
		SetAt:       time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(contextPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save context: %w", err)
	}

	if projectName != "" {
		log.Successf("Active project set to: %s (%s)", projectName, projID)
	} else {
		log.Successf("Active project set to: %s", projID)
	}
	return nil
}
