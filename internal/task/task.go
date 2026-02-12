package task

import (
	"fmt"
	"regexp"
	"time"
)

// Task represents an atomic unit of execution in Dagryn.
// Tasks are immutable after creation.
type Task struct {
	Name      string
	Command   string
	Uses      []string // Plugin dependencies
	Inputs    []string
	Outputs   []string
	Needs     []string
	Env       map[string]string
	Timeout   time.Duration
	Workdir   string
	With      map[string]string    // Composite plugin inputs
	Container *TaskContainerConfig // Optional per-task container settings
	Group     string               // Logical group for target resolution
	If        string               // Condition expression for conditional execution
}

// TaskContainerConfig holds per-task container overrides.
type TaskContainerConfig struct {
	Image       string `toml:"image"`
	MemoryLimit string `toml:"memory_limit"`
	CPULimit    string `toml:"cpu_limit"`
	Network     string `toml:"network"`
}

// validNameRegex defines valid task name pattern
var validNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// Validate checks if the task configuration is valid
func (t *Task) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("task name cannot be empty")
	}

	if !validNameRegex.MatchString(t.Name) {
		return fmt.Errorf("task %q has invalid name: must start with a letter and contain only letters, numbers, underscores, and hyphens", t.Name)
	}

	// A task needs either a command or a uses spec (for composite plugins)
	if t.Command == "" && len(t.Uses) == 0 {
		return fmt.Errorf("task %q has no command", t.Name)
	}

	// with requires uses
	if len(t.With) > 0 && len(t.Uses) == 0 {
		return fmt.Errorf("task %q has 'with' but no 'uses'", t.Name)
	}

	return nil
}

// IsComposite returns true if this task delegates to a composite plugin
// (has no command and uses exactly one plugin).
func (t *Task) IsComposite() bool {
	return t.Command == "" && len(t.Uses) == 1
}

// HasDependencies returns true if the task has dependencies
func (t *Task) HasDependencies() bool {
	return len(t.Needs) > 0
}

// HasPlugins returns true if the task has plugin dependencies
func (t *Task) HasPlugins() bool {
	return len(t.Uses) > 0
}

// HasInputs returns true if the task has defined inputs
func (t *Task) HasInputs() bool {
	return len(t.Inputs) > 0
}

// HasOutputs returns true if the task has defined outputs
func (t *Task) HasOutputs() bool {
	return len(t.Outputs) > 0
}

// Clone creates a deep copy of the task
func (t *Task) Clone() *Task {
	clone := &Task{
		Name:    t.Name,
		Command: t.Command,
		Timeout: t.Timeout,
		Workdir: t.Workdir,
		Group:   t.Group,
		If:      t.If,
	}

	if t.Uses != nil {
		clone.Uses = make([]string, len(t.Uses))
		copy(clone.Uses, t.Uses)
	}

	if t.Inputs != nil {
		clone.Inputs = make([]string, len(t.Inputs))
		copy(clone.Inputs, t.Inputs)
	}

	if t.Outputs != nil {
		clone.Outputs = make([]string, len(t.Outputs))
		copy(clone.Outputs, t.Outputs)
	}

	if t.Needs != nil {
		clone.Needs = make([]string, len(t.Needs))
		copy(clone.Needs, t.Needs)
	}

	if t.Env != nil {
		clone.Env = make(map[string]string, len(t.Env))
		for k, v := range t.Env {
			clone.Env[k] = v
		}
	}

	if t.With != nil {
		clone.With = make(map[string]string, len(t.With))
		for k, v := range t.With {
			clone.With[k] = v
		}
	}

	if t.Container != nil {
		c := *t.Container
		clone.Container = &c
	}

	return clone
}

// String returns a string representation of the task
func (t *Task) String() string {
	return fmt.Sprintf("Task{Name: %s, Command: %s}", t.Name, t.Command)
}
