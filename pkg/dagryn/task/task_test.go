package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTask_Validate(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid task",
			task: Task{
				Name:    "build",
				Command: "go build ./...",
			},
			wantErr: false,
		},
		{
			name: "valid task with all fields",
			task: Task{
				Name:    "test-task_1",
				Command: "npm test",
				Inputs:  []string{"src/**"},
				Outputs: []string{"dist/**"},
				Needs:   []string{"build"},
				Env:     map[string]string{"NODE_ENV": "test"},
				Timeout: 30 * time.Second,
				Workdir: "./packages/app",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			task: Task{
				Name:    "",
				Command: "echo hello",
			},
			wantErr: true,
			errMsg:  "task name cannot be empty",
		},
		{
			name: "invalid name - starts with number",
			task: Task{
				Name:    "1task",
				Command: "echo hello",
			},
			wantErr: true,
			errMsg:  "invalid name",
		},
		{
			name: "invalid name - contains space",
			task: Task{
				Name:    "my task",
				Command: "echo hello",
			},
			wantErr: true,
			errMsg:  "invalid name",
		},
		{
			name: "empty command without uses",
			task: Task{
				Name:    "build",
				Command: "",
			},
			wantErr: true,
			errMsg:  "has no command",
		},
		{
			name: "uses without command (composite)",
			task: Task{
				Name: "setup",
				Uses: []string{"dagryn/setup-go@v1"},
				With: map[string]string{"go-version": "1.22"},
			},
			wantErr: false,
		},
		{
			name: "with without uses",
			task: Task{
				Name:    "build",
				Command: "go build",
				With:    map[string]string{"key": "value"},
			},
			wantErr: true,
			errMsg:  "has 'with' but no 'uses'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTask_HasDependencies(t *testing.T) {
	taskWithDeps := Task{Needs: []string{"install"}}
	taskWithoutDeps := Task{}

	assert.True(t, taskWithDeps.HasDependencies())
	assert.False(t, taskWithoutDeps.HasDependencies())
}

func TestTask_HasInputs(t *testing.T) {
	taskWithInputs := Task{Inputs: []string{"src/**"}}
	taskWithoutInputs := Task{}

	assert.True(t, taskWithInputs.HasInputs())
	assert.False(t, taskWithoutInputs.HasInputs())
}

func TestTask_HasOutputs(t *testing.T) {
	taskWithOutputs := Task{Outputs: []string{"dist/**"}}
	taskWithoutOutputs := Task{}

	assert.True(t, taskWithOutputs.HasOutputs())
	assert.False(t, taskWithoutOutputs.HasOutputs())
}

func TestTask_Clone(t *testing.T) {
	original := &Task{
		Name:    "build",
		Command: "go build",
		Inputs:  []string{"*.go"},
		Outputs: []string{"bin/*"},
		Needs:   []string{"install"},
		Env:     map[string]string{"GO111MODULE": "on"},
		Timeout: 5 * time.Minute,
		Workdir: "./cmd",
	}

	clone := original.Clone()

	// Verify all fields are copied
	assert.Equal(t, original.Name, clone.Name)
	assert.Equal(t, original.Command, clone.Command)
	assert.Equal(t, original.Inputs, clone.Inputs)
	assert.Equal(t, original.Outputs, clone.Outputs)
	assert.Equal(t, original.Needs, clone.Needs)
	assert.Equal(t, original.Env, clone.Env)
	assert.Equal(t, original.Timeout, clone.Timeout)
	assert.Equal(t, original.Workdir, clone.Workdir)

	// Verify deep copy - modifying clone shouldn't affect original
	clone.Inputs[0] = "modified"
	assert.NotEqual(t, original.Inputs[0], clone.Inputs[0])

	clone.Env["NEW_KEY"] = "value"
	_, exists := original.Env["NEW_KEY"]
	assert.False(t, exists)
}

func TestTask_IsComposite(t *testing.T) {
	composite := Task{
		Name: "setup",
		Uses: []string{"dagryn/setup-go@v1"},
	}
	assert.True(t, composite.IsComposite())

	// Has command -> not composite
	notComposite := Task{
		Name:    "build",
		Command: "go build",
		Uses:    []string{"github:golangci/golangci-lint@v1.55.0"},
	}
	assert.False(t, notComposite.IsComposite())

	// Multiple uses -> not composite
	multiUses := Task{
		Name: "multi",
		Uses: []string{"a@v1", "b@v2"},
	}
	assert.False(t, multiUses.IsComposite())

	// No uses -> not composite
	noUses := Task{
		Name:    "simple",
		Command: "echo hello",
	}
	assert.False(t, noUses.IsComposite())
}

func TestTask_Clone_WithField(t *testing.T) {
	original := &Task{
		Name: "setup",
		Uses: []string{"dagryn/setup-go@v1"},
		With: map[string]string{"go-version": "1.22"},
	}

	clone := original.Clone()
	assert.Equal(t, original.With, clone.With)

	// Verify deep copy
	clone.With["go-version"] = "1.23"
	assert.NotEqual(t, original.With["go-version"], clone.With["go-version"])
}

func TestTask_String(t *testing.T) {
	task := Task{Name: "build", Command: "go build"}
	str := task.String()

	assert.Contains(t, str, "build")
	assert.Contains(t, str, "go build")
}
