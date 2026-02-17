package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

const hookTimeout = 30 * time.Second

// HookContext carries runtime context for hook execution.
type HookContext struct {
	RunID          string
	TaskName       string
	TaskStatus     string
	TaskDurationMs int64
	RunStatus      string
	ProjectRoot    string
}

// Env converts HookContext to DAGRYN_* environment variables.
func (hc *HookContext) Env() map[string]string {
	env := map[string]string{
		"DAGRYN_PROJECT_ROOT": hc.ProjectRoot,
	}
	if hc.RunID != "" {
		env["DAGRYN_RUN_ID"] = hc.RunID
	}
	if hc.TaskName != "" {
		env["DAGRYN_TASK_NAME"] = hc.TaskName
	}
	if hc.TaskStatus != "" {
		env["DAGRYN_TASK_STATUS"] = hc.TaskStatus
	}
	if hc.TaskDurationMs > 0 {
		env["DAGRYN_TASK_DURATION_MS"] = strconv.FormatInt(hc.TaskDurationMs, 10)
	}
	if hc.RunStatus != "" {
		env["DAGRYN_RUN_STATUS"] = hc.RunStatus
	}
	return env
}

// HookExecutor executes integration plugin hooks.
type HookExecutor struct {
	logger Logger
}

// NewHookExecutor creates a new HookExecutor.
func NewHookExecutor(logger Logger) *HookExecutor {
	if logger == nil {
		logger = &defaultLogger{}
	}
	return &HookExecutor{logger: logger}
}

// RunHook executes a single hook command with context and inputs.
// Errors are non-fatal and returned for logging.
func (e *HookExecutor) RunHook(ctx context.Context, pluginName, hookName string, hook HookDef, inputs map[string]string, hctx *HookContext) error {
	// Evaluate condition
	if hook.If != "" {
		condResult := substituteVars(hook.If, inputs)
		if condResult == "false" || condResult == "" {
			e.logger.Debug("skipping hook %s/%s: condition not met", pluginName, hookName)
			return nil
		}
	}

	// Substitute variables in command
	command := substituteVars(hook.Command, inputs)

	e.logger.Info("running hook %s/%s", pluginName, hookName)

	// Build environment
	hookCtx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(hookCtx, "sh", "-c", command)
	if hctx != nil && hctx.ProjectRoot != "" {
		cmd.Dir = hctx.ProjectRoot
	}

	// Merge environment: hook context env + hook-specific env + inputs
	envSlice := make([]string, 0)
	if hctx != nil {
		for k, v := range hctx.Env() {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
		}
	}
	for k, v := range hook.Env {
		resolved := substituteVars(v, inputs)
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, resolved))
	}
	if len(envSlice) > 0 {
		cmd.Env = append(cmd.Environ(), envSlice...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook %s/%s failed: %w\nOutput: %s", pluginName, hookName, err, string(output))
	}

	e.logger.Debug("hook %s/%s output: %s", pluginName, hookName, string(output))
	return nil
}
