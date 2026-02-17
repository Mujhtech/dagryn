// Package container provides container-based task isolation using Docker or Podman.
package container

import (
	"context"
	"io"
	"time"
)

// Runtime is the interface for container runtimes (Docker, Podman, etc.).
type Runtime interface {
	// Available returns true if the container runtime is reachable.
	Available(ctx context.Context) bool

	// Pull pulls a container image.
	Pull(ctx context.Context, image string) error

	// Create creates a new container and returns its ID.
	Create(ctx context.Context, cfg ContainerConfig) (containerID string, err error)

	// Start starts a created container.
	Start(ctx context.Context, containerID string) error

	// Wait blocks until the container exits and returns its exit code.
	Wait(ctx context.Context, containerID string) (exitCode int, err error)

	// Logs streams container stdout/stderr to the provided writers.
	Logs(ctx context.Context, containerID string, stdout, stderr io.Writer) error

	// Stop stops a running container with a timeout for graceful shutdown.
	Stop(ctx context.Context, containerID string, timeout time.Duration) error

	// Remove removes a container.
	Remove(ctx context.Context, containerID string) error
}

// ContainerConfig describes how to create a container.
type ContainerConfig struct {
	Image       string
	Command     []string
	WorkDir     string
	Env         map[string]string
	Mounts      []Mount
	CPULimit    int64  // CPU quota in units of 10^-9 CPUs (NanoCPUs)
	MemoryLimit int64  // Memory limit in bytes
	Network     string // e.g. "bridge", "none", "host"
}

// Mount describes a bind mount from host to container.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}
