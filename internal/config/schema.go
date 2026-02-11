package config

import (
	"github.com/mujhtech/dagryn/internal/plugin"
	"github.com/mujhtech/dagryn/internal/task"
)

// ContainerConfig holds the project-level container isolation configuration.
type ContainerConfig struct {
	Enabled     bool   `toml:"enabled"`
	Image       string `toml:"image"`        // Default image, e.g. "golang:1.25"
	MemoryLimit string `toml:"memory_limit"` // e.g. "2g", "512m"
	CPULimit    string `toml:"cpu_limit"`    // e.g. "2.0", "0.5"
	Network     string `toml:"network"`      // e.g. "bridge", "none"
}

// Config represents the root configuration loaded from dagryn.toml.
type Config struct {
	Workflow  WorkflowConfig        `toml:"workflow"`
	Tasks     map[string]TaskConfig `toml:"tasks"`
	Plugins   map[string]string     `toml:"plugins"`   // Global plugins available to all tasks
	Cache     CacheConfig           `toml:"cache"`
	Container ContainerConfig       `toml:"container"` // Project-level container isolation settings
}

// CacheConfig controls local and remote caching.
type CacheConfig struct {
	Enabled *bool             `toml:"enabled"` // default true when nil
	Dir     string            `toml:"dir"`     // override local cache directory
	Remote  RemoteCacheConfig `toml:"remote"`
}

// IsEnabled returns whether local caching is enabled (defaults to true).
func (c CacheConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// RemoteCacheConfig configures the remote cache backend.
type RemoteCacheConfig struct {
	Enabled         bool   `toml:"enabled"`
	Cloud           bool   `toml:"cloud"`    // Use Dagryn Cloud cache API
	Provider        string `toml:"provider"` // "s3", "filesystem" (ignored when cloud=true)
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	UsePathStyle    bool   `toml:"use_path_style"`
	Prefix          string `toml:"prefix"`
	BasePath        string `toml:"base_path"`
	Strategy        string `toml:"strategy"`          // default "local-first"
	FallbackOnError *bool  `toml:"fallback_on_error"` // default true when nil
}

// IsFallbackOnError returns whether remote errors are non-fatal (defaults to true).
func (rc RemoteCacheConfig) IsFallbackOnError() bool {
	if rc.FallbackOnError == nil {
		return true
	}
	return *rc.FallbackOnError
}

// WorkflowConfig represents the workflow section of the config.
type WorkflowConfig struct {
	Name    string `toml:"name"`
	Default bool   `toml:"default"`
}

// TaskConfig represents a task definition in the config file.
type TaskConfig struct {
	Command   string                        `toml:"command"`
	Uses      plugin.Spec                   `toml:"uses"` // Plugin dependencies (single string or array)
	Inputs    []string                      `toml:"inputs"`
	Outputs   []string                      `toml:"outputs"`
	Needs     []string                      `toml:"needs"`
	Env       map[string]string             `toml:"env"`
	Timeout   string                        `toml:"timeout"` // e.g., "30s", "5m"
	Workdir   string                        `toml:"workdir"`
	With      map[string]string             `toml:"with"`      // Inputs for composite plugins
	Container *task.TaskContainerConfig `toml:"container"` // Per-task container overrides
}

// HasPlugins returns true if the task has any plugin dependencies.
func (t *TaskConfig) HasPlugins() bool {
	return !t.Uses.IsEmpty()
}

// GetPlugins returns the list of plugin specs for this task.
func (t *TaskConfig) GetPlugins() []string {
	return t.Uses.Plugins
}
