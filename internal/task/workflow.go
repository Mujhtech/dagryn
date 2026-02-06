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
