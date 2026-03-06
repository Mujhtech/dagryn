package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const compositeCleanupTimeout = 30 * time.Second

// CompositeExecutor executes composite plugin steps.
type CompositeExecutor struct {
	projectRoot string
	logger      Logger
	stdout      io.Writer
	stderr      io.Writer
}

// CompositeSetupResult carries state from setup execution to cleanup.
type CompositeSetupResult struct {
	Inputs map[string]string
	Env    map[string]string
}

// NewCompositeExecutor creates a new composite executor.
func NewCompositeExecutor(projectRoot string, logger Logger) *CompositeExecutor {
	if logger == nil {
		logger = &defaultLogger{}
	}
	return &CompositeExecutor{
		projectRoot: projectRoot,
		logger:      logger,
	}
}

// SetOutput sets the stdout and stderr writers for step output streaming.
// When set, step output is written to these writers in real time instead of
// being captured silently.
//
// Deprecated: Use WithOutput for concurrent use. SetOutput mutates shared state
// and is not safe to call from multiple goroutines.
func (e *CompositeExecutor) SetOutput(stdout, stderr io.Writer) {
	e.stdout = stdout
	e.stderr = stderr
}

// WithOutput returns a shallow copy of the executor with the given writers set.
// The copy is safe to use concurrently with the original or other copies.
func (e *CompositeExecutor) WithOutput(stdout, stderr io.Writer) *CompositeExecutor {
	return &CompositeExecutor{
		projectRoot: e.projectRoot,
		logger:      e.logger,
		stdout:      stdout,
		stderr:      stderr,
	}
}

// Execute runs all steps of a composite plugin sequentially.
// Cleanup steps are guaranteed to run in reverse order after main steps,
// regardless of success or failure.
func (e *CompositeExecutor) Execute(ctx context.Context, manifest *Manifest, inputs, env map[string]string, workdir string) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if !manifest.IsComposite() {
		return fmt.Errorf("manifest is not a composite plugin")
	}

	setup, err := e.ExecuteSetup(ctx, manifest, inputs, env, workdir)

	// Ensure cleanup steps run in reverse order, even on failure
	defer func() {
		if setup != nil {
			e.RunCleanup(manifest, setup, workdir)
		}
	}()

	return err
}

// ExecuteSetup runs only main steps and returns context required for deferred cleanup.
func (e *CompositeExecutor) ExecuteSetup(ctx context.Context, manifest *Manifest, inputs, env map[string]string, workdir string) (*CompositeSetupResult, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest is nil")
	}
	if !manifest.IsComposite() {
		return nil, fmt.Errorf("manifest is not a composite plugin")
	}

	// Validate required inputs
	mergedInputs, err := e.mergeInputs(manifest, inputs)
	if err != nil {
		return nil, err
	}

	// Track cleanup context (environment variables set during execution)
	cleanupEnv := make(map[string]string)
	for k, v := range env {
		cleanupEnv[k] = v
	}

	// Execute each step
	for i, step := range manifest.Steps {
		// Evaluate conditional
		if step.If != "" {
			condResult := substituteVars(step.If, mergedInputs)
			if condResult == "false" || condResult == "" {
				e.logger.Debug("skipping step %d (%s): condition not met", i, step.Name)
				continue
			}
		}

		// Substitute variables in command
		command := substituteVars(step.Command, mergedInputs)

		e.logger.Info("running step %d: %s", i, step.Name)

		// Build environment
		stepEnv := make([]string, 0)
		for k, v := range cleanupEnv {
			stepEnv = append(stepEnv, fmt.Sprintf("%s=%s", k, v))
		}
		for k, v := range step.Env {
			resolved := substituteVars(v, mergedInputs)
			stepEnv = append(stepEnv, fmt.Sprintf("%s=%s", k, resolved))
			// Track for cleanup
			cleanupEnv[k] = resolved
		}

		// Execute via shell
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		if workdir != "" {
			cmd.Dir = workdir
		} else {
			cmd.Dir = e.projectRoot
		}
		if len(stepEnv) > 0 {
			cmd.Env = append(cmd.Environ(), stepEnv...)
		}

		// Stream output when writers are set; otherwise capture to buffer.
		if e.stdout != nil || e.stderr != nil {
			stdout := e.stdout
			if stdout == nil {
				stdout = io.Discard
			}
			stderr := e.stderr
			if stderr == nil {
				stderr = io.Discard
			}
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			if err := cmd.Run(); err != nil {
				return &CompositeSetupResult{
					Inputs: mergedInputs,
					Env:    cleanupEnv,
				}, fmt.Errorf("step %d (%s) failed: %w", i, step.Name, err)
			}
		} else {
			output, err := cmd.CombinedOutput()
			if err != nil {
				return &CompositeSetupResult{
					Inputs: mergedInputs,
					Env:    cleanupEnv,
				}, fmt.Errorf("step %d (%s) failed: %w\nOutput: %s", i, step.Name, err, string(output))
			}
			e.logger.Debug("step %d output: %s", i, string(output))
		}
	}

	return &CompositeSetupResult{
		Inputs: mergedInputs,
		Env:    cleanupEnv,
	}, nil
}

// RunCleanup executes cleanup steps for a previously completed setup.
func (e *CompositeExecutor) RunCleanup(manifest *Manifest, setup *CompositeSetupResult, workdir string) {
	if manifest == nil || !manifest.IsComposite() || len(manifest.Cleanup) == 0 {
		return
	}
	if setup == nil {
		e.logger.Warn("cleanup skipped: missing setup context")
		return
	}

	// Use a bounded background context so cleanup still runs when the
	// execution context is canceled, while preventing unbounded hangs.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), compositeCleanupTimeout)
	defer cancel()
	e.executeCleanup(cleanupCtx, manifest.Cleanup, setup.Inputs, setup.Env, workdir)
}

// executeCleanup runs cleanup steps in reverse order (LIFO).
// Errors are logged but do not fail the cleanup process.
func (e *CompositeExecutor) executeCleanup(ctx context.Context, cleanupSteps []CompositeStep, inputs, env map[string]string, workdir string) {
	e.logger.Info("running cleanup steps (%d total)", len(cleanupSteps))

	// Execute in reverse order (LIFO - like defer)
	for i := len(cleanupSteps) - 1; i >= 0; i-- {
		step := cleanupSteps[i]

		// Evaluate conditional
		if step.If != "" {
			condResult := substituteVars(step.If, inputs)
			if condResult == "false" || condResult == "" {
				e.logger.Debug("skipping cleanup step %d (%s): condition not met", i, step.Name)
				continue
			}
		}

		// Substitute variables in command
		command := substituteVars(step.Command, inputs)

		e.logger.Info("running cleanup step %d: %s", i, step.Name)

		// Build environment
		stepEnv := make([]string, 0)
		for k, v := range env {
			stepEnv = append(stepEnv, fmt.Sprintf("%s=%s", k, v))
		}
		for k, v := range step.Env {
			resolved := substituteVars(v, inputs)
			stepEnv = append(stepEnv, fmt.Sprintf("%s=%s", k, resolved))
		}

		// Execute via shell
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		if workdir != "" {
			cmd.Dir = workdir
		} else {
			cmd.Dir = e.projectRoot
		}
		if len(stepEnv) > 0 {
			cmd.Env = append(cmd.Environ(), stepEnv...)
		}

		if e.stdout != nil || e.stderr != nil {
			stdout := e.stdout
			if stdout == nil {
				stdout = io.Discard
			}
			stderr := e.stderr
			if stderr == nil {
				stderr = io.Discard
			}
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			if err := cmd.Run(); err != nil {
				e.logger.Warn("cleanup step %d (%s) failed (continuing): %v", i, step.Name, err)
			}
		} else {
			output, err := cmd.CombinedOutput()
			if err != nil {
				e.logger.Warn("cleanup step %d (%s) failed (continuing): %v\nOutput: %s", i, step.Name, err, string(output))
			} else {
				e.logger.Debug("cleanup step %d output: %s", i, string(output))
			}
		}
	}

	e.logger.Info("cleanup completed")
}

// mergeInputs validates required inputs and applies defaults from the manifest.
func (e *CompositeExecutor) mergeInputs(manifest *Manifest, inputs map[string]string) (map[string]string, error) {
	merged := make(map[string]string)

	// Apply defaults first
	for name, def := range manifest.Inputs {
		if def.Default != "" {
			merged[name] = def.Default
		}
	}

	// Override with provided inputs
	for k, v := range inputs {
		merged[k] = v
	}

	// Validate required inputs
	for name, def := range manifest.Inputs {
		if def.Required {
			if _, ok := merged[name]; !ok {
				return nil, fmt.Errorf("required input %q is missing", name)
			}
		}
	}

	return merged, nil
}

// CollectStepEnv extracts all environment variables defined in composite steps,
// with plugin input variables substituted. Shell variables like $HOME and $PATH
// are expanded using the current process environment.
func (e *CompositeExecutor) CollectStepEnv(manifest *Manifest, inputs map[string]string) map[string]string {
	if manifest == nil || !manifest.IsComposite() {
		return nil
	}

	mergedInputs, err := e.mergeInputs(manifest, inputs)
	if err != nil {
		return nil
	}

	env := make(map[string]string)
	for _, step := range manifest.Steps {
		// Skip conditional steps that are explicitly false
		if step.If != "" {
			condResult := substituteVars(step.If, mergedInputs)
			if condResult == "false" || condResult == "" {
				continue
			}
		}
		for k, v := range step.Env {
			resolved := substituteVars(v, mergedInputs)
			// Expand shell variables like $HOME, $PATH
			resolved = os.ExpandEnv(resolved)
			env[k] = resolved
		}
	}

	return env
}

// GenerateSetupScript produces a shell script from a composite plugin's setup
// steps, targeting a specific OS/arch. This is used to run plugin setup inside
// a container instead of on the host. Shell variables like $HOME and $PATH are
// kept as-is (not expanded) so the container shell resolves them at runtime.
func (e *CompositeExecutor) GenerateSetupScript(manifest *Manifest, inputs map[string]string, targetOS, targetArch string) (string, error) {
	if manifest == nil || !manifest.IsComposite() {
		return "", fmt.Errorf("manifest is not a composite plugin")
	}

	mergedInputs, err := e.mergeInputs(manifest, inputs)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, step := range manifest.Steps {
		// Evaluate conditional
		if step.If != "" {
			condResult := substituteVarsForPlatform(step.If, mergedInputs, targetOS, targetArch)
			if condResult == "false" || condResult == "" {
				continue
			}
		}

		// Export step environment variables (substitute inputs/os/arch but
		// keep shell variables like $HOME, $PATH for container resolution).
		for k, v := range step.Env {
			resolved := substituteVarsForPlatform(v, mergedInputs, targetOS, targetArch)
			fmt.Fprintf(&sb, "export %s=\"%s\"\n", k, resolved)
		}

		// Wrap each step command in a subshell so that `exit 0` (used to
		// short-circuit a step) doesn't terminate the entire script and
		// skip the actual task command. Env exports above stay in the
		// outer shell so they persist for subsequent steps and the task.
		command := substituteVarsForPlatform(step.Command, mergedInputs, targetOS, targetArch)
		sb.WriteString("(\n")
		sb.WriteString(command)
		if !strings.HasSuffix(command, "\n") {
			sb.WriteByte('\n')
		}
		sb.WriteString(")\n")
	}

	return sb.String(), nil
}

// substituteVars replaces ${inputs.key}, ${os}, and ${arch} in a string.
func substituteVars(s string, inputs map[string]string) string {
	return substituteVarsForPlatform(s, inputs, runtime.GOOS, runtime.GOARCH)
}

// substituteVarsForPlatform replaces ${inputs.key}, ${os}, and ${arch} in a
// string using the supplied platform values instead of the host runtime.
func substituteVarsForPlatform(s string, inputs map[string]string, targetOS, targetArch string) string {
	// Replace ${inputs.key} patterns
	for k, v := range inputs {
		s = strings.ReplaceAll(s, fmt.Sprintf("${inputs.%s}", k), v)
	}

	// Replace built-in variables
	s = strings.ReplaceAll(s, "${os}", targetOS)
	s = strings.ReplaceAll(s, "${arch}", targetArch)

	return s
}
