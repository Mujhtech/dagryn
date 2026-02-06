package task

import (
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
