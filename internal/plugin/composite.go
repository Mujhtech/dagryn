package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// CompositeExecutor executes composite plugin steps.
type CompositeExecutor struct {
	projectRoot string
	logger      Logger
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

	// Validate required inputs
	mergedInputs, err := e.mergeInputs(manifest, inputs)
	if err != nil {
		return err
	}

	// Track cleanup context (environment variables set during execution)
	cleanupEnv := make(map[string]string)
	for k, v := range env {
		cleanupEnv[k] = v
	}

	// Ensure cleanup steps run in reverse order, even on failure
	defer func() {
		if len(manifest.Cleanup) > 0 {
			e.executeCleanup(ctx, manifest.Cleanup, mergedInputs, cleanupEnv, workdir)
		}
	}()

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

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("step %d (%s) failed: %w\nOutput: %s", i, step.Name, err, string(output))
		}

		e.logger.Debug("step %d output: %s", i, string(output))
	}

	return nil
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

		output, err := cmd.CombinedOutput()
		if err != nil {
			// Log error but continue with other cleanup steps
			e.logger.Warn("cleanup step %d (%s) failed (continuing): %v\nOutput: %s", i, step.Name, err, string(output))
		} else {
			e.logger.Debug("cleanup step %d output: %s", i, string(output))
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

// substituteVars replaces ${inputs.key}, ${os}, and ${arch} in a string.
func substituteVars(s string, inputs map[string]string) string {
	// Replace ${inputs.key} patterns
	for k, v := range inputs {
		s = strings.ReplaceAll(s, fmt.Sprintf("${inputs.%s}", k), v)
	}

	// Replace built-in variables
	s = strings.ReplaceAll(s, "${os}", runtime.GOOS)
	s = strings.ReplaceAll(s, "${arch}", runtime.GOARCH)

	return s
}
