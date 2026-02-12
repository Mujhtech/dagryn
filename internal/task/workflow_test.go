package task

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkflow(t *testing.T) {
	w := NewWorkflow("ci", nil)
	assert.Equal(t, "ci", w.Name)
	assert.NotNil(t, w.Tasks)
	assert.Empty(t, w.Tasks)
}

func TestWorkflow_AddTask(t *testing.T) {
	w := NewWorkflow("ci", nil)

	task := &Task{Name: "build", Command: "go build"}
	err := w.AddTask(task)
	require.NoError(t, err)

	// Adding duplicate should fail
	err = w.AddTask(task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate task name")
}

func TestWorkflow_GetTask(t *testing.T) {
	w := NewWorkflow("ci", nil)
	task := &Task{Name: "build", Command: "go build"}
	_ = w.AddTask(task)

	// Existing task
	found, ok := w.GetTask("build")
	assert.True(t, ok)
	assert.Equal(t, task, found)

	// Non-existing task
	_, ok = w.GetTask("nonexistent")
	assert.False(t, ok)
}

func TestWorkflow_ListTasks(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "go build"})
	_ = w.AddTask(&Task{Name: "test", Command: "go test"})

	tasks := w.ListTasks()
	assert.Len(t, tasks, 2)
}

func TestWorkflow_TaskNames(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "go build"})
	_ = w.AddTask(&Task{Name: "test", Command: "go test"})

	names := w.TaskNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "build")
	assert.Contains(t, names, "test")
}

func TestWorkflow_RootTasks(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "install", Command: "npm install"})
	_ = w.AddTask(&Task{Name: "build", Command: "npm build", Needs: []string{"install"}})
	_ = w.AddTask(&Task{Name: "lint", Command: "npm lint"})

	roots := w.RootTasks()
	assert.Len(t, roots, 2)

	names := make([]string, len(roots))
	for i, r := range roots {
		names[i] = r.Name
	}
	assert.Contains(t, names, "install")
	assert.Contains(t, names, "lint")
}

func TestWorkflow_LeafTasks(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "install", Command: "npm install"})
	_ = w.AddTask(&Task{Name: "build", Command: "npm build", Needs: []string{"install"}})
	_ = w.AddTask(&Task{Name: "test", Command: "npm test", Needs: []string{"build"}})
	_ = w.AddTask(&Task{Name: "lint", Command: "npm lint"})

	leaves := w.LeafTasks()
	assert.Len(t, leaves, 2)

	names := make([]string, len(leaves))
	for i, l := range leaves {
		names[i] = l.Name
	}
	assert.Contains(t, names, "test")
	assert.Contains(t, names, "lint")
}

func TestWorkflow_Validate(t *testing.T) {
	// Valid workflow
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "go build"})
	err := w.Validate()
	require.NoError(t, err)

	// Empty workflow name
	w2 := NewWorkflow("", nil)
	err = w2.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow name cannot be empty")

	// Invalid task in workflow
	w3 := NewWorkflow("ci", nil)
	_ = w3.AddTask(&Task{Name: "build", Command: ""}) // empty command
	err = w3.Validate()
	require.Error(t, err)
}

func TestWorkflow_Size(t *testing.T) {
	w := NewWorkflow("ci", nil)
	assert.Equal(t, 0, w.Size())

	_ = w.AddTask(&Task{Name: "build", Command: "go build"})
	assert.Equal(t, 1, w.Size())

	_ = w.AddTask(&Task{Name: "test", Command: "go test"})
	assert.Equal(t, 2, w.Size())
}

// --- Group and ResolveTargets tests ---

func TestWorkflow_TasksByGroup(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "echo build", Group: "backend"})
	_ = w.AddTask(&Task{Name: "test", Command: "echo test", Group: "backend"})
	_ = w.AddTask(&Task{Name: "lint-web", Command: "echo lint", Group: "frontend"})
	_ = w.AddTask(&Task{Name: "deploy", Command: "echo deploy"})

	backend := w.TasksByGroup("backend")
	assert.Len(t, backend, 2)
	names := extractNames(backend)
	sort.Strings(names)
	assert.Equal(t, []string{"build", "test"}, names)

	frontend := w.TasksByGroup("frontend")
	assert.Len(t, frontend, 1)
	assert.Equal(t, "lint-web", frontend[0].Name)

	none := w.TasksByGroup("nonexistent")
	assert.Empty(t, none)
}

func TestWorkflow_GroupNames(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "echo build", Group: "backend"})
	_ = w.AddTask(&Task{Name: "test", Command: "echo test", Group: "backend"})
	_ = w.AddTask(&Task{Name: "lint", Command: "echo lint", Group: "frontend"})
	_ = w.AddTask(&Task{Name: "deploy", Command: "echo deploy"})

	names := w.GroupNames()
	sort.Strings(names)
	assert.Equal(t, []string{"backend", "frontend"}, names)
}

func TestWorkflow_HasGroup(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "echo build", Group: "backend"})

	assert.True(t, w.HasGroup("backend"))
	assert.False(t, w.HasGroup("frontend"))
	assert.False(t, w.HasGroup(""))
}

func TestWorkflow_ResolveTargets_GroupExpansion(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "echo build", Group: "backend"})
	_ = w.AddTask(&Task{Name: "test", Command: "echo test", Group: "backend"})
	_ = w.AddTask(&Task{Name: "lint", Command: "echo lint", Group: "frontend"})

	resolved := w.ResolveTargets([]string{"backend"})
	sort.Strings(resolved)
	assert.Equal(t, []string{"build", "test"}, resolved)
}

func TestWorkflow_ResolveTargets_MixedGroupAndTaskNames(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "echo build", Group: "backend"})
	_ = w.AddTask(&Task{Name: "test", Command: "echo test", Group: "backend"})
	_ = w.AddTask(&Task{Name: "lint", Command: "echo lint", Group: "frontend"})
	_ = w.AddTask(&Task{Name: "deploy", Command: "echo deploy"})

	resolved := w.ResolveTargets([]string{"backend", "deploy"})
	sort.Strings(resolved)
	assert.Equal(t, []string{"build", "deploy", "test"}, resolved)
}

func TestWorkflow_ResolveTargets_Dedup(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "echo build", Group: "backend"})
	_ = w.AddTask(&Task{Name: "test", Command: "echo test", Group: "backend"})

	// "build" appears both as a direct target and in the "backend" group
	resolved := w.ResolveTargets([]string{"build", "backend"})
	sort.Strings(resolved)
	assert.Equal(t, []string{"build", "test"}, resolved)
}

func TestWorkflow_ResolveTargets_UnknownPassthrough(t *testing.T) {
	w := NewWorkflow("ci", nil)
	_ = w.AddTask(&Task{Name: "build", Command: "echo build"})

	// Unknown target passes through for later validation
	resolved := w.ResolveTargets([]string{"nonexistent"})
	assert.Equal(t, []string{"nonexistent"}, resolved)
}

func TestWorkflow_TaskClone_CopiesGroupAndIf(t *testing.T) {
	original := &Task{
		Name:    "deploy",
		Command: "make deploy",
		Group:   "backend",
		If:      "branch == 'main'",
	}

	clone := original.Clone()
	require.NotNil(t, clone)
	assert.Equal(t, "backend", clone.Group)
	assert.Equal(t, "branch == 'main'", clone.If)

	// Verify independence
	clone.Group = "changed"
	clone.If = "changed"
	assert.Equal(t, "backend", original.Group)
	assert.Equal(t, "branch == 'main'", original.If)
}

func extractNames(tasks []*Task) []string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = t.Name
	}
	return names
}
