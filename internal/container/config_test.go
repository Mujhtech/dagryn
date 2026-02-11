package container

import (
	"testing"

	"github.com/mujhtech/dagryn/internal/task"
	"github.com/stretchr/testify/assert"
)

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"", 0},
		{"0", 0},
		{"512m", 512 * 1024 * 1024},
		{"2g", 2 * 1024 * 1024 * 1024},
		{"1024k", 1024 * 1024},
		{"1.5g", int64(1.5 * 1024 * 1024 * 1024)},
		{"256M", 256 * 1024 * 1024},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseMemoryLimit(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseCPULimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"", 0},
		{"1.0", 1_000_000_000},
		{"2.0", 2_000_000_000},
		{"0.5", 500_000_000},
		{"0.25", 250_000_000},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseCPULimit(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigMerge(t *testing.T) {
	t.Run("no task override", func(t *testing.T) {
		cfg := &Config{
			Enabled:     true,
			Image:       "golang:1.25",
			MemoryLimit: "2g",
			CPULimit:    "2.0",
			Network:     "bridge",
		}

		result := cfg.Merge(nil)
		assert.Equal(t, "golang:1.25", result.Image)
		assert.Equal(t, int64(2*1024*1024*1024), result.MemoryLimit)
		assert.Equal(t, int64(2_000_000_000), result.CPULimit)
		assert.Equal(t, "bridge", result.Network)
	})

	t.Run("task overrides image and memory", func(t *testing.T) {
		cfg := &Config{
			Enabled:     true,
			Image:       "golang:1.25",
			MemoryLimit: "2g",
			CPULimit:    "2.0",
			Network:     "bridge",
		}

		taskCfg := &task.TaskContainerConfig{
			Image:       "golang:1.25-alpine",
			MemoryLimit: "4g",
		}

		result := cfg.Merge(taskCfg)
		assert.Equal(t, "golang:1.25-alpine", result.Image)
		assert.Equal(t, int64(4*1024*1024*1024), result.MemoryLimit)
		assert.Equal(t, int64(2_000_000_000), result.CPULimit) // inherited
		assert.Equal(t, "bridge", result.Network)               // inherited
	})

	t.Run("task overrides all", func(t *testing.T) {
		cfg := &Config{
			Image:       "golang:1.25",
			MemoryLimit: "2g",
			CPULimit:    "2.0",
			Network:     "bridge",
		}

		taskCfg := &task.TaskContainerConfig{
			Image:       "node:20",
			MemoryLimit: "1g",
			CPULimit:    "0.5",
			Network:     "none",
		}

		result := cfg.Merge(taskCfg)
		assert.Equal(t, "node:20", result.Image)
		assert.Equal(t, int64(1024*1024*1024), result.MemoryLimit)
		assert.Equal(t, int64(500_000_000), result.CPULimit)
		assert.Equal(t, "none", result.Network)
	})

	t.Run("empty config with task override", func(t *testing.T) {
		cfg := &Config{}
		taskCfg := &task.TaskContainerConfig{
			Image: "python:3.12",
		}

		result := cfg.Merge(taskCfg)
		assert.Equal(t, "python:3.12", result.Image)
		assert.Equal(t, int64(0), result.MemoryLimit)
		assert.Equal(t, int64(0), result.CPULimit)
		assert.Equal(t, "", result.Network)
	})
}
