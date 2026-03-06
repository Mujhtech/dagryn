package task

import "fmt"

// Workflow represents a collection of tasks forming a DAG.
type Workflow struct {
	Name    string
	Default bool
	Tasks   map[string]*Task
}

// NewWorkflow creates a new workflow with the given name and tasks.
func NewWorkflow(name string, tasks map[string]*Task) *Workflow {
	if tasks == nil {
		tasks = make(map[string]*Task)
	}
	return &Workflow{
		Name:  name,
		Tasks: tasks,
	}
}

// GetTask returns a task by name.
func (w *Workflow) GetTask(name string) (*Task, bool) {
	task, ok := w.Tasks[name]
	return task, ok
}

// AddTask adds a task to the workflow.
func (w *Workflow) AddTask(task *Task) error {
	if _, exists := w.Tasks[task.Name]; exists {
		return fmt.Errorf("duplicate task name: %q", task.Name)
	}
	w.Tasks[task.Name] = task
	return nil
}

// ListTasks returns all tasks in the workflow.
func (w *Workflow) ListTasks() []*Task {
	tasks := make([]*Task, 0, len(w.Tasks))
	for _, task := range w.Tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// TaskNames returns all task names in the workflow.
func (w *Workflow) TaskNames() []string {
	names := make([]string, 0, len(w.Tasks))
	for name := range w.Tasks {
		names = append(names, name)
	}
	return names
}

// RootTasks returns tasks that have no dependencies.
func (w *Workflow) RootTasks() []*Task {
	roots := make([]*Task, 0)
	for _, task := range w.Tasks {
		if !task.HasDependencies() {
			roots = append(roots, task)
		}
	}
	return roots
}

// LeafTasks returns tasks that are not depended upon by any other task.
func (w *Workflow) LeafTasks() []*Task {
	// Build set of all dependencies
	depended := make(map[string]bool)
	for _, task := range w.Tasks {
		for _, dep := range task.Needs {
			depended[dep] = true
		}
	}

	// Find tasks not in the depended set
	leaves := make([]*Task, 0)
	for name, task := range w.Tasks {
		if !depended[name] {
			leaves = append(leaves, task)
		}
	}
	return leaves
}

// Validate validates all tasks in the workflow.
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("workflow name cannot be empty")
	}

	for _, task := range w.Tasks {
		if err := task.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Size returns the number of tasks in the workflow.
func (w *Workflow) Size() int {
	return len(w.Tasks)
}

// TasksByGroup returns all tasks that belong to the given group.
func (w *Workflow) TasksByGroup(group string) []*Task {
	var tasks []*Task
	for _, t := range w.Tasks {
		if t.Group == group {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// GroupNames returns all unique group names in the workflow.
func (w *Workflow) GroupNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, t := range w.Tasks {
		if t.Group != "" && !seen[t.Group] {
			seen[t.Group] = true
			names = append(names, t.Group)
		}
	}
	return names
}

// HasGroup returns true if any task belongs to the given group.
func (w *Workflow) HasGroup(name string) bool {
	for _, t := range w.Tasks {
		if t.Group == name {
			return true
		}
	}
	return false
}

// ResolveTargets expands group names to task names, passes through direct task names,
// and deduplicates the result. A target is treated as a group name if it matches a
// group and does not match a task name (groups must not collide with task names per validation).
func (w *Workflow) ResolveTargets(targets []string) []string {
	seen := make(map[string]bool)
	var resolved []string

	for _, target := range targets {
		// If it's a direct task name, keep it
		if _, ok := w.Tasks[target]; ok {
			if !seen[target] {
				seen[target] = true
				resolved = append(resolved, target)
			}
			continue
		}

		// Try to expand as a group
		if w.HasGroup(target) {
			for _, t := range w.TasksByGroup(target) {
				if !seen[t.Name] {
					seen[t.Name] = true
					resolved = append(resolved, t.Name)
				}
			}
			continue
		}

		// Neither a task nor a group - pass through for later validation
		if !seen[target] {
			seen[target] = true
			resolved = append(resolved, target)
		}
	}

	return resolved
}
