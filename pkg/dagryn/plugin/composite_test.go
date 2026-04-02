package plugin

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteVars(t *testing.T) {
	inputs := map[string]string{
		"go-version": "1.22",
		"name":       "world",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "go${inputs.go-version}",
			expected: "go1.22",
		},
		{
			input:    "hello ${inputs.name}!",
			expected: "hello world!",
		},
		{
			input:    "${os}-${arch}",
			expected: runtime.GOOS + "-" + runtime.GOARCH,
		},
		{
			input:    "no vars here",
			expected: "no vars here",
		},
		{
			input:    "${inputs.missing}",
			expected: "",
		},
		{
			input:    "X${inputs.node-version}Y",
			expected: "XY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := substituteVars(tt.input, inputs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompositeExecutor_MergeInputs(t *testing.T) {
	e := NewCompositeExecutor("/tmp", nil)

	manifest := &Manifest{
		Inputs: map[string]InputDef{
			"required-input": {Required: true, Description: "A required input"},
			"optional-input": {Description: "An optional input", Default: "default-value"},
		},
	}

	t.Run("all inputs provided", func(t *testing.T) {
		inputs := map[string]string{
			"required-input": "value1",
			"optional-input": "value2",
		}
		merged, err := e.mergeInputs(manifest, inputs)
		require.NoError(t, err)
		assert.Equal(t, "value1", merged["required-input"])
		assert.Equal(t, "value2", merged["optional-input"])
	})

	t.Run("default applied", func(t *testing.T) {
		inputs := map[string]string{
			"required-input": "value1",
		}
		merged, err := e.mergeInputs(manifest, inputs)
		require.NoError(t, err)
		assert.Equal(t, "value1", merged["required-input"])
		assert.Equal(t, "default-value", merged["optional-input"])
	})

	t.Run("missing required input", func(t *testing.T) {
		inputs := map[string]string{
			"optional-input": "value2",
		}
		_, err := e.mergeInputs(manifest, inputs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required input")
	})
}

func TestCompositeExecutor_Execute(t *testing.T) {
	e := NewCompositeExecutor(t.TempDir(), nil)
	ctx := context.Background()

	t.Run("simple steps", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
			Steps: []CompositeStep{
				{Name: "step1", Command: "echo hello"},
				{Name: "step2", Command: "echo world"},
			},
		}

		err := e.Execute(ctx, manifest, nil, nil, "")
		assert.NoError(t, err)
	})

	t.Run("with input substitution", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
			Inputs: map[string]InputDef{
				"name": {Required: true},
			},
			Steps: []CompositeStep{
				{Name: "greet", Command: "echo hello ${inputs.name}"},
			},
		}

		inputs := map[string]string{"name": "dagryn"}
		err := e.Execute(ctx, manifest, inputs, nil, "")
		assert.NoError(t, err)
	})

	t.Run("step failure", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
			Steps: []CompositeStep{
				{Name: "fail", Command: "exit 1"},
			},
		}

		err := e.Execute(ctx, manifest, nil, nil, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "step 0 (fail) failed")
	})

	t.Run("nil manifest", func(t *testing.T) {
		err := e.Execute(ctx, nil, nil, nil, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "manifest is nil")
	})

	t.Run("non-composite manifest", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "tool"},
		}
		err := e.Execute(ctx, manifest, nil, nil, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a composite plugin")
	})

	t.Run("missing required input", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
			Inputs: map[string]InputDef{
				"version": {Required: true},
			},
			Steps: []CompositeStep{
				{Name: "step", Command: "echo ${inputs.version}"},
			},
		}

		err := e.Execute(ctx, manifest, nil, nil, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required input")
	})

	t.Run("conditional step skipped", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
			Steps: []CompositeStep{
				{Name: "always", Command: "echo always"},
				{Name: "skipped", Command: "exit 1", If: "false"},
			},
		}

		err := e.Execute(ctx, manifest, nil, nil, "")
		assert.NoError(t, err)
	})

	t.Run("step with env", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
			Inputs: map[string]InputDef{
				"val": {Default: "hello"},
			},
			Steps: []CompositeStep{
				{
					Name:    "with-env",
					Command: "echo $MY_VAR",
					Env:     map[string]string{"MY_VAR": "${inputs.val}"},
				},
			},
		}

		err := e.Execute(ctx, manifest, nil, nil, "")
		assert.NoError(t, err)
	})

	t.Run("missing optional hyphenated input does not break shell", func(t *testing.T) {
		manifest := &Manifest{
			Plugin: ManifestPlugin{Type: "composite"},
			Inputs: map[string]InputDef{
				"node-version": {Description: "optional"},
			},
			Steps: []CompositeStep{
				{Name: "safe-optional", Command: "NODE_VERSION=\"${inputs.node-version}\"; [ -z \"$NODE_VERSION\" ]"},
			},
		}

		err := e.Execute(ctx, manifest, nil, nil, "")
		assert.NoError(t, err)
	})
}

func TestExpandWithEnv(t *testing.T) {
	env := map[string]string{"HOME": "/test", "PATH": "/usr/bin"}

	assert.Equal(t, "/test/.tools/bin:/usr/bin",
		expandWithEnv("$HOME/.tools/bin:$PATH", env))

	assert.Equal(t, "hello",
		expandWithEnv("hello", env))

	// Falls back to os.Getenv for keys not in env map
	assert.Equal(t, os.Getenv("USER"),
		expandWithEnv("$USER", map[string]string{}))
}

func TestExecuteSetup_EnvAccumulation(t *testing.T) {
	e := NewCompositeExecutor(t.TempDir(), nil)
	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Steps: []CompositeStep{
			{
				Name:    "set-tool-path",
				Command: "true",
				Env:     map[string]string{"PATH": "/custom/bin:$PATH"},
			},
			{
				Name:    "verify-path",
				Command: "echo $PATH | grep -q /custom/bin",
			},
		},
	}
	setup, err := e.ExecuteSetup(context.Background(), manifest, nil, nil, "")
	require.NoError(t, err)
	assert.Contains(t, setup.Env["PATH"], "/custom/bin")
	assert.NotContains(t, setup.Env["PATH"], "$PATH") // must be expanded
}

func TestCollectStepEnv_Accumulated(t *testing.T) {
	e := NewCompositeExecutor(t.TempDir(), nil)
	manifest := &Manifest{
		Plugin: ManifestPlugin{Type: "composite"},
		Steps: []CompositeStep{
			{Name: "s1", Command: "true", Env: map[string]string{"PATH": "/a:$PATH"}},
			{Name: "s2", Command: "true", Env: map[string]string{"PATH": "/b:$PATH"}},
		},
	}
	env := e.CollectStepEnv(manifest, nil)
	assert.Contains(t, env["PATH"], "/b:")
	assert.Contains(t, env["PATH"], "/a:") // must include step 1's addition
}
