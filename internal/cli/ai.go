package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/ai/aitypes"
	"github.com/mujhtech/dagryn/internal/ai/provider"
	"github.com/mujhtech/dagryn/internal/client"
	"github.com/mujhtech/dagryn/internal/config"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newAICmd())
}

func newAICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI-powered CI analysis tools",
	}
	cmd.AddCommand(newAIAnalyzeCmd())
	return cmd
}

func newAIAnalyzeCmd() *cobra.Command {
	var (
		runID       string
		projectRoot string
		mode        string
		backendMode string
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a failed run using AI",
		Long: `Analyzes a failed CI run using an AI provider and prints a summary.

Reads run data from the local .dagryn/runs/ directory and the AI
configuration from dagryn.toml. Requires AI to be enabled in the config.`,
		Example: `  # Analyze the most recent failed run
  dagryn ai analyze

  # Analyze a specific run by ID
  dagryn ai analyze --run-id <uuid>

  # Use a specific AI mode
  dagryn ai analyze --mode summarize_and_suggest`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAIAnalyze(runID, projectRoot, mode, backendMode)
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to analyze (defaults to most recent failed run)")
	cmd.Flags().StringVar(&projectRoot, "project-root", "", "Project root directory (defaults to current directory)")
	cmd.Flags().StringVar(&mode, "mode", "", "Analysis mode: summarize or summarize_and_suggest (overrides config)")
	cmd.Flags().StringVar(&backendMode, "backend-mode", "", "Backend mode: byok, managed, or agent (overrides config)")

	return cmd
}

func runAIAnalyze(runIDStr, projectRoot, mode, backendMode string) error {
	log := logger.New(verbose)

	// Resolve project root
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Parse config
	cfg, err := config.Parse(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if !cfg.AI.IsEnabled() {
		return fmt.Errorf("AI analysis is not enabled in config; set [ai] enabled = true in dagryn.toml")
	}

	// Resolve backend mode
	bm := cfg.AI.Backend.Mode
	if backendMode != "" {
		bm = backendMode
	}
	if bm == "" {
		bm = "byok"
	}

	// Resolve analysis mode
	analysisMode := cfg.AI.Mode
	if mode != "" {
		analysisMode = mode
	}
	if analysisMode == "" {
		analysisMode = "summarize"
	}

	// Resolve API key from environment
	apiKey := os.Getenv(cfg.AI.Backend.BYOK.APIKeyEnv)
	if apiKey == "" && cfg.AI.Backend.BYOK.APIKeyEnv == "" {
		// Try common default env var names
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	// Build provider config
	provCfg := provider.ProviderConfig{
		BackendMode:    bm,
		Provider:       cfg.AI.Provider,
		Model:          cfg.AI.Model,
		APIKey:         apiKey,
		MaxTokens:      4096,
		TimeoutSeconds: 60,
		AgentEndpoint:  cfg.AI.Backend.Agent.Endpoint,
		AgentToken:     os.Getenv(cfg.AI.Backend.Agent.AuthTokenEnv),
	}

	if cfg.AI.Backend.Agent.TimeoutSeconds > 0 {
		provCfg.TimeoutSeconds = cfg.AI.Backend.Agent.TimeoutSeconds
	}

	zlog := zerolog.New(os.Stderr).With().Timestamp().Logger()
	aiProvider, err := provider.NewProvider(provCfg, zlog)
	if err != nil {
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Load run data from local store
	store := client.NewRunStore(projectRoot)

	var localRun *client.LocalRun
	if runIDStr != "" {
		rid, err := uuid.Parse(runIDStr)
		if err != nil {
			return fmt.Errorf("invalid run ID %q: %w", runIDStr, err)
		}
		localRun, err = store.GetRun(rid)
		if err != nil {
			return fmt.Errorf("failed to read run: %w", err)
		}
		if localRun == nil {
			return fmt.Errorf("run %s not found in .dagryn/runs/", runIDStr)
		}
	} else {
		// Find the most recent failed run
		runs, err := store.ListRuns()
		if err != nil {
			return fmt.Errorf("failed to list runs: %w", err)
		}
		for _, r := range runs {
			if r.Status == "failed" {
				localRun = r
				break
			}
		}
		if localRun == nil {
			return fmt.Errorf("no failed runs found in .dagryn/runs/; specify --run-id explicitly")
		}
	}

	log.Info(fmt.Sprintf("Analyzing run %s (status: %s)", localRun.RunID, localRun.Status))

	// Read logs for the run
	logEntries, err := store.ReadLogs(localRun.RunID)
	if err != nil {
		return fmt.Errorf("failed to read run logs: %w", err)
	}

	// Build evidence from logs: group by task, collect stderr tails for failed tasks
	taskLogs := make(map[string][]client.RunLogEntry)
	for _, entry := range logEntries {
		taskLogs[entry.TaskName] = append(taskLogs[entry.TaskName], entry)
	}

	var failedTasks []aitypes.FailedTaskEvidence
	for taskName, entries := range taskLogs {
		var stderrTail, stdoutTail string
		for _, e := range entries {
			if e.Stream == "stderr" {
				stderrTail += e.Line + "\n"
			} else {
				stdoutTail += e.Line + "\n"
			}
		}

		// Truncate to limits
		if len(stderrTail) > aitypes.MaxLogTailBytes {
			stderrTail = stderrTail[len(stderrTail)-aitypes.MaxLogTailBytes:]
		}
		if len(stdoutTail) > aitypes.MaxLogTailBytes {
			stdoutTail = stdoutTail[len(stdoutTail)-aitypes.MaxLogTailBytes:]
		}

		failedTasks = append(failedTasks, aitypes.FailedTaskEvidence{
			TaskName:   taskName,
			ExitCode:   1,
			StdoutTail: stdoutTail,
			StderrTail: stderrTail,
		})

		if len(failedTasks) >= aitypes.MaxFailedTasks {
			break
		}
	}

	if len(failedTasks) == 0 {
		log.Info("No task logs found for analysis")
		return nil
	}

	// Build analysis input
	input := aitypes.AnalysisInput{
		RunID:           localRun.RunID.String(),
		ProjectID:       localRun.ProjectID.String(),
		GitBranch:       localRun.GitBranch,
		GitCommit:       localRun.GitCommit,
		FailedTasks:     failedTasks,
		RunErrorMessage: localRun.ErrorMsg,
		FailedTaskCount: len(failedTasks),
	}

	log.Info(fmt.Sprintf("Sending %d failed task(s) to AI provider (mode=%s, backend=%s)...", len(failedTasks), analysisMode, bm))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(provCfg.TimeoutSeconds)*time.Second)
	defer cancel()

	output, err := aiProvider.AnalyzeFailure(ctx, input)
	if err != nil {
		return fmt.Errorf("AI analysis failed: %w", err)
	}

	// Print results
	fmt.Println()
	fmt.Println("=== AI Analysis ===")
	fmt.Println()
	fmt.Printf("Summary:    %s\n", output.Summary)
	fmt.Printf("Root Cause: %s\n", output.RootCause)
	fmt.Printf("Confidence: %.0f%%\n", output.Confidence*100)

	if len(output.Evidence) > 0 {
		fmt.Println()
		fmt.Println("Evidence:")
		for _, e := range output.Evidence {
			fmt.Printf("  - [%s] %s\n", e.Task, e.Reason)
		}
	}

	if len(output.LikelyFiles) > 0 {
		fmt.Println()
		fmt.Println("Likely Files:")
		for _, f := range output.LikelyFiles {
			fmt.Printf("  - %s\n", f)
		}
	}

	if len(output.RecommendedActions) > 0 {
		fmt.Println()
		fmt.Println("Recommended Actions:")
		for i, a := range output.RecommendedActions {
			fmt.Printf("  %d. %s\n", i+1, a)
		}
	}

	// Also write JSON to stdout if verbose
	if verbose {
		fmt.Println()
		fmt.Println("--- Raw JSON ---")
		raw, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(raw))
	}

	return nil
}
