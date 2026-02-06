package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mujhtech/dagryn/internal/task"
)

// DefaultConfigFile is the default configuration file name.
const DefaultConfigFile = "dagryn.toml"

// Parse loads and parses a configuration file from the given path.
func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes parses configuration from raw TOML bytes.
func ParseBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Tasks == nil {
		cfg.Tasks = make(map[string]TaskConfig)
	}

	if cfg.Plugins == nil {
		cfg.Plugins = make(map[string]string)
	}

	return &cfg, nil
}

// ToWorkflow converts the config into a Workflow with Tasks.
func (c *Config) ToWorkflow() (*task.Workflow, error) {
	w := task.NewWorkflow(c.Workflow.Name, nil)
	w.Default = c.Workflow.Default

	for name, tc := range c.Tasks {
		t, err := taskConfigToTask(name, tc, c.Plugins)
		if err != nil {
			return nil, err
		}
		if err := w.AddTask(t); err != nil {
			return nil, err
		}
	}

	return w, nil
}

// taskConfigToTask converts a TaskConfig to a Task.
func taskConfigToTask(name string, tc TaskConfig, globalPlugins map[string]string) (*task.Task, error) {
	t := &task.Task{
		Name:    name,
		Command: tc.Command,
		Uses:    resolvePluginRefs(tc.GetPlugins(), globalPlugins),
		Inputs:  tc.Inputs,
		Outputs: tc.Outputs,
		Needs:   tc.Needs,
		Env:     tc.Env,
		Workdir: tc.Workdir,
	}

	// Parse timeout if specified
	if tc.Timeout != "" {
		duration, err := time.ParseDuration(tc.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout for task %q: %w", name, err)
		}
		t.Timeout = duration
	}

	return t, nil
}

// resolvePluginRefs resolves plugin references to their full specs.
// If a plugin name matches a key in globalPlugins, it's replaced with the full spec.
func resolvePluginRefs(plugins []string, globalPlugins map[string]string) []string {
	if len(plugins) == 0 {
		return nil
	}

	resolved := make([]string, 0, len(plugins))
	for _, p := range plugins {
		// Check if it's a reference to a global plugin
		if spec, ok := globalPlugins[p]; ok {
			resolved = append(resolved, spec)
		} else {
			// It's a direct plugin spec
			resolved = append(resolved, p)
		}
	}
	return resolved
}
