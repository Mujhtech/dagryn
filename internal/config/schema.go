package config

import "github.com/mujhtech/dagryn/internal/plugin"

// Config represents the root configuration loaded from dagryn.toml.
type Config struct {
	Workflow WorkflowConfig        `toml:"workflow"`
	Tasks    map[string]TaskConfig `toml:"tasks"`
	Plugins  map[string]string     `toml:"plugins"` // Global plugins available to all tasks
}

// WorkflowConfig represents the workflow section of the config.
type WorkflowConfig struct {
	Name    string `toml:"name"`
	Default bool   `toml:"default"`
}

// TaskConfig represents a task definition in the config file.
type TaskConfig struct {
	Command string            `toml:"command"`
	Uses    plugin.Spec       `toml:"uses"` // Plugin dependencies (single string or array)
	Inputs  []string          `toml:"inputs"`
	Outputs []string          `toml:"outputs"`
	Needs   []string          `toml:"needs"`
	Env     map[string]string `toml:"env"`
	Timeout string            `toml:"timeout"` // e.g., "30s", "5m"
	Workdir string            `toml:"workdir"`
}

// HasPlugins returns true if the task has any plugin dependencies.
func (t *TaskConfig) HasPlugins() bool {
	return !t.Uses.IsEmpty()
}

// GetPlugins returns the list of plugin specs for this task.
func (t *TaskConfig) GetPlugins() []string {
	return t.Uses.Plugins
}
