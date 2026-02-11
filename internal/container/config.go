package container

import (
	"fmt"
	"strings"

	"github.com/mujhtech/dagryn/internal/task"
)

// Config holds the project-level container isolation configuration.
type Config struct {
	Enabled     bool   `toml:"enabled"`
	Image       string `toml:"image"`        // Default image, e.g. "golang:1.25"
	MemoryLimit string `toml:"memory_limit"` // e.g. "2g", "512m"
	CPULimit    string `toml:"cpu_limit"`    // e.g. "2.0", "0.5"
	Network     string `toml:"network"`      // e.g. "bridge", "none"
}

// Merge returns a ContainerConfig with task-level overrides applied on top of project defaults.
func (c *Config) Merge(taskCfg *task.TaskContainerConfig) ContainerConfig {
	image := c.Image
	memLimit := c.MemoryLimit
	cpuLimit := c.CPULimit
	network := c.Network

	if taskCfg != nil {
		if taskCfg.Image != "" {
			image = taskCfg.Image
		}
		if taskCfg.MemoryLimit != "" {
			memLimit = taskCfg.MemoryLimit
		}
		if taskCfg.CPULimit != "" {
			cpuLimit = taskCfg.CPULimit
		}
		if taskCfg.Network != "" {
			network = taskCfg.Network
		}
	}

	return ContainerConfig{
		Image:       image,
		MemoryLimit: ParseMemoryLimit(memLimit),
		CPULimit:    ParseCPULimit(cpuLimit),
		Network:     network,
	}
}

// ParseMemoryLimit converts a human-readable memory string (e.g. "2g", "512m") to bytes.
func ParseMemoryLimit(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(strings.ToLower(s))
	var multiplier int64 = 1

	switch {
	case strings.HasSuffix(s, "g"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "m"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "k"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "k")
	}

	var value float64
	if _, err := fmt.Sscanf(s, "%f", &value); err != nil {
		return 0
	}
	return int64(value * float64(multiplier))
}

// ParseCPULimit converts a CPU limit string (e.g. "2.0", "0.5") to NanoCPUs.
func ParseCPULimit(s string) int64 {
	if s == "" {
		return 0
	}
	var value float64
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%f", &value); err != nil {
		return 0
	}
	return int64(value * 1e9)
}
