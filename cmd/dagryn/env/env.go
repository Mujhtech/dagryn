package env

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/spf13/cobra"
)

// NewCmd creates the env command group.
func NewCmd(flags *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage project environment variables and secrets",
		Long:  "Set, list, resolve, pull, and inject project-scoped environment variables.",
	}

	cmd.AddCommand(newEnvListCmd(flags))
	cmd.AddCommand(newEnvSetCmd(flags))
	cmd.AddCommand(newEnvSeedCmd(flags))
	cmd.AddCommand(newEnvPullCmd(flags))
	cmd.AddCommand(newEnvInjectCmd(flags))
	cmd.AddCommand(newEnvDeleteCmd(flags))
	cmd.AddCommand(newEnvRotateCmd(flags))

	return cmd
}

func newEnvListCmd(flags *cli.Flags) *cobra.Command {
	var projectID string
	var environment string
	var branch string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project env metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(flags.Verbose)
			apiClient, projID, err := resolveEnvClient(projectID)
			if err != nil {
				return err
			}

			var envPtr, branchPtr *string
			if strings.TrimSpace(environment) != "" {
				envPtr = &environment
			}
			if strings.TrimSpace(branch) != "" {
				branchPtr = &branch
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			items, err := apiClient.ListProjectEnvVars(ctx, projID, client.ListProjectEnvVarsRequest{
				Environment: envPtr,
				Branch:      branchPtr,
			})
			if err != nil {
				return err
			}

			if len(items) == 0 {
				log.Info("No env vars found for this scope.")
				return nil
			}

			sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
			for _, item := range items {
				scope := "default"
				if item.Environment != nil && *item.Environment != "" {
					scope = *item.Environment
				}
				if item.Branch != nil && *item.Branch != "" {
					scope = scope + "/" + *item.Branch
				}

				kind := "plain"
				if item.ValueType == string(models.EnvValueTypeSecret) {
					kind = "secret"
				}
				log.Infof("%s  [%s]  scope=%s required=%t", item.Key, kind, scope, item.Required)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID")
	cmd.Flags().StringVar(&environment, "environment", "", "environment scope (e.g. dev, staging, prod)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch scope override")

	return cmd
}

func newEnvSetCmd(flags *cli.Flags) *cobra.Command {
	var projectID string
	var key string
	var value string
	var fromStdin bool
	var environment string
	var branch string
	var required bool
	var secret bool
	var description string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a project env var",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("--key is required")
			}
			if fromStdin {
				v, err := readSingleLineFromStdin()
				if err != nil {
					return err
				}
				value = v
			}
			if value == "" {
				return fmt.Errorf("value is required (use --value or --from-stdin)")
			}

			apiClient, projID, err := resolveEnvClient(projectID)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			req := client.SetProjectEnvVarRequest{
				Key:         key,
				Value:       value,
				Environment: emptyToNil(environment),
				Branch:      emptyToNil(branch),
				Required:    required,
				Secret:      secret,
				Description: emptyToNil(description),
			}

			if _, err := apiClient.SetProjectEnvVar(ctx, projID, req); err != nil {
				return err
			}

			logger.New(flags.Verbose).Successf("Set env var %s", key)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID")
	cmd.Flags().StringVar(&key, "key", "", "environment variable key")
	cmd.Flags().StringVar(&value, "value", "", "environment variable value")
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read value from stdin")
	cmd.Flags().StringVar(&environment, "environment", "", "environment scope (e.g. dev, staging, prod)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch scope override")
	cmd.Flags().BoolVar(&required, "required", false, "mark this key as required at runtime")
	cmd.Flags().BoolVar(&secret, "secret", false, "store as secret")
	cmd.Flags().StringVar(&description, "description", "", "optional metadata description")

	return cmd
}

func newEnvPullCmd(flags *cli.Flags) *cobra.Command {
	var projectID string
	var environment string
	var branch string
	var output string

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull resolved env vars into dotenv output",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(environment) == "" {
				return fmt.Errorf("--environment is required")
			}

			apiClient, projID, err := resolveEnvClient(projectID)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			items, err := apiClient.ResolveProjectEnv(ctx, projID, client.ResolveProjectEnvRequest{
				Environment: environment,
				Branch:      branch,
				Reveal:      true,
			})
			if err != nil {
				return err
			}

			lines := make([]string, 0, len(items))
			for _, item := range items {
				if item.Value == nil {
					continue
				}
				lines = append(lines, fmt.Sprintf("%s=%s", item.Key, quoteIfNeeded(*item.Value)))
			}

			payload := strings.Join(lines, "\n")
			if payload != "" {
				payload += "\n"
			}

			if output == "" {
				_, _ = fmt.Fprint(os.Stdout, payload)
			} else {
				if err := os.WriteFile(output, []byte(payload), 0600); err != nil {
					return fmt.Errorf("failed to write %s: %w", output, err)
				}
			}

			logger.New(flags.Verbose).Successf("Pulled %d env vars", len(lines))
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID")
	cmd.Flags().StringVar(&environment, "environment", "", "environment scope (e.g. dev, staging, prod)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch scope override")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output dotenv file path (default stdout)")

	return cmd
}

func newEnvSeedCmd(flags *cli.Flags) *cobra.Command {
	var projectID string
	var environment string
	var branch string
	var file string
	var secret bool
	var required bool

	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Bulk seed env vars from dotenv file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("--file is required")
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", file, err)
			}
			items := parseDotenvItems(string(data), environment, branch, secret, required)
			if len(items) == 0 {
				return fmt.Errorf("no valid key=value lines found in %s", file)
			}

			apiClient, projID, err := resolveEnvClient(projectID)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			out, err := apiClient.SeedProjectEnvVars(ctx, projID, client.SeedProjectEnvVarsRequest{Items: items})
			if err != nil {
				return err
			}
			logger.New(flags.Verbose).Successf("Seeded %d env vars", len(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID")
	cmd.Flags().StringVar(&environment, "environment", "", "environment scope (e.g. dev, staging, prod)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch scope override")
	cmd.Flags().StringVar(&file, "file", "", "dotenv file path")
	cmd.Flags().BoolVar(&secret, "secret", false, "store seeded keys as secret")
	cmd.Flags().BoolVar(&required, "required", false, "mark seeded keys as required")

	return cmd
}

func newEnvDeleteCmd(flags *cli.Flags) *cobra.Command {
	var projectID string
	var id string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an env var by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("--id is required")
			}
			envVarID, err := uuid.Parse(id)
			if err != nil {
				return fmt.Errorf("invalid env var ID: %w", err)
			}
			apiClient, projID, err := resolveEnvClient(projectID)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if err := apiClient.DeleteProjectEnvVar(ctx, projID, envVarID); err != nil {
				return err
			}
			logger.New(flags.Verbose).Successf("Deleted env var %s", envVarID)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID")
	cmd.Flags().StringVar(&id, "id", "", "env var ID")
	return cmd
}

func newEnvRotateCmd(flags *cli.Flags) *cobra.Command {
	var projectID string
	var id string
	var value string
	var fromStdin bool

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate secret value/reference for an env var",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("--id is required")
			}
			envVarID, err := uuid.Parse(id)
			if err != nil {
				return fmt.Errorf("invalid env var ID: %w", err)
			}
			if fromStdin {
				v, err := readSingleLineFromStdin()
				if err != nil {
					return err
				}
				value = v
			}

			req := client.RotateProjectEnvVarRequest{Value: value}

			apiClient, projID, err := resolveEnvClient(projectID)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if _, err := apiClient.RotateProjectEnvVar(ctx, projID, envVarID, req); err != nil {
				return err
			}
			logger.New(flags.Verbose).Successf("Rotated env var %s", envVarID)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID")
	cmd.Flags().StringVar(&id, "id", "", "env var ID")
	cmd.Flags().StringVar(&value, "value", "", "new secret value")
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read new value from stdin")
	return cmd
}

func newEnvInjectCmd(flags *cli.Flags) *cobra.Command {
	var projectID string
	var environment string
	var branch string

	cmd := &cobra.Command{
		Use:   "inject -- <command ...>",
		Short: "Resolve project env and inject into a local command",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("command is required after --")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(environment) == "" {
				return fmt.Errorf("--environment is required")
			}
			log := logger.New(flags.Verbose)
			apiClient, projID, err := resolveEnvClient(projectID)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			items, err := apiClient.ResolveProjectEnv(ctx, projID, client.ResolveProjectEnvRequest{
				Environment: environment,
				Branch:      branch,
				Reveal:      true,
			})
			if err != nil {
				return err
			}

			env := os.Environ()
			for _, item := range items {
				if item.Value == nil {
					continue
				}
				env = append(env, fmt.Sprintf("%s=%s", item.Key, *item.Value))
			}

			execCmd := args[0]
			execArgs := []string{}
			if len(args) > 1 {
				execArgs = args[1:]
			}

			proc := os.ProcAttr{Env: env, Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}}
			p, err := os.StartProcess(execCmd, append([]string{execCmd}, execArgs...), &proc)
			if err != nil {
				return fmt.Errorf("failed to start command: %w", err)
			}

			state, err := p.Wait()
			if err != nil {
				return fmt.Errorf("failed waiting for command: %w", err)
			}

			if !state.Success() {
				return fmt.Errorf("command exited with status: %s", state.String())
			}
			log.Successf("Injected %d env vars", len(items))
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID")
	cmd.Flags().StringVar(&environment, "environment", "", "environment scope (e.g. dev, staging, prod)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch scope override")

	return cmd
}

func resolveEnvClient(projectIDFlag string) (*client.Client, uuid.UUID, error) {
	store, err := client.NewCredentialsStore()
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil || creds == nil {
		return nil, uuid.Nil, fmt.Errorf("not logged in. Run 'dagryn auth login' first")
	}

	apiClient := client.New(client.Config{BaseURL: creds.ServerURL, Timeout: 30 * time.Second})
	apiClient.SetCredentials(creds)
	apiClient.SetCredentialsStore(store)

	if strings.TrimSpace(projectIDFlag) != "" {
		id, err := uuid.Parse(projectIDFlag)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("invalid project ID: %w", err)
		}
		return apiClient, id, nil
	}

	projectRoot, err := cli.GetProjectRoot()
	if err != nil {
		return nil, uuid.Nil, err
	}

	projectStore := client.NewProjectConfigStore(projectRoot)
	if cfg, err := projectStore.Load(); err == nil && cfg != nil {
		return apiClient, cfg.ProjectID, nil
	}

	contextID := cli.LoadContextProjectID(projectRoot)
	if contextID != "" {
		id, err := uuid.Parse(contextID)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("invalid context project ID: %w", err)
		}
		return apiClient, id, nil
	}

	return nil, uuid.Nil, fmt.Errorf("no project linked. Run 'dagryn init --remote' or 'dagryn use <project-id>'")
}

func readSingleLineFromStdin() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to inspect stdin: %w", err)
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", fmt.Errorf("--from-stdin provided but stdin is empty")
	}
	s := bufio.NewScanner(os.Stdin)
	if !s.Scan() {
		if err := s.Err(); err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		return "", fmt.Errorf("stdin did not contain a value")
	}
	return strings.TrimSpace(s.Text()), nil
}

func emptyToNil(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func quoteIfNeeded(v string) string {
	if strings.ContainsAny(v, " \t\n\r#\"") {
		escaped := strings.ReplaceAll(v, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		return "\"" + escaped + "\""
	}
	return v
}

func parseDotenvItems(content, environment, branch string, secret, required bool) []client.SetProjectEnvVarRequest {
	items := make([]client.SetProjectEnvVarRequest, 0)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		if k == "" {
			continue
		}
		v = strings.Trim(v, "\"")
		item := client.SetProjectEnvVarRequest{
			Key:         k,
			Value:       v,
			Environment: emptyToNil(environment),
			Branch:      emptyToNil(branch),
			Required:    required,
			Secret:      secret,
		}
		items = append(items, item)
	}
	return items
}
