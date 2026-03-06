package container

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

// DockerRuntime implements the Runtime interface using the Docker Engine SDK.
// It also works with Podman's Docker-compatible API socket.
type DockerRuntime struct {
	cli *client.Client
}

// NewDockerRuntime creates a DockerRuntime using the default Docker host
// (DOCKER_HOST env var or /var/run/docker.sock).
func NewDockerRuntime() (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &DockerRuntime{cli: cli}, nil
}

// Available checks whether the Docker daemon is reachable.
func (d *DockerRuntime) Available(ctx context.Context) bool {
	_, err := d.cli.Ping(ctx)
	return err == nil
}

// Pull pulls a container image.
func (d *DockerRuntime) Pull(ctx context.Context, img string) error {
	reader, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", img, err)
	}
	defer func() { _ = reader.Close() }()
	// Consume the pull output to completion
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// Create creates a new container from the given configuration.
func (d *DockerRuntime) Create(ctx context.Context, cfg ContainerConfig) (string, error) {
	env := make([]string, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}

	containerCfg := &container.Config{
		Image:      cfg.Image,
		Cmd:        cfg.Command,
		WorkingDir: cfg.WorkDir,
		Env:        env,
	}

	hostCfg := &container.HostConfig{
		Resources: container.Resources{},
	}

	// Resource limits
	if cfg.CPULimit > 0 {
		hostCfg.NanoCPUs = cfg.CPULimit
	}
	if cfg.MemoryLimit > 0 {
		hostCfg.Memory = cfg.MemoryLimit
	}

	// Mounts
	if len(cfg.Mounts) > 0 {
		mounts := make([]mount.Mount, len(cfg.Mounts))
		for i, m := range cfg.Mounts {
			mounts[i] = mount.Mount{
				Type:     mount.TypeBind,
				Source:   m.Source,
				Target:   m.Target,
				ReadOnly: m.ReadOnly,
			}
		}
		hostCfg.Mounts = mounts
	}

	// Network mode
	if cfg.Network != "" {
		hostCfg.NetworkMode = container.NetworkMode(cfg.Network)
	}

	resp, err := d.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	return resp.ID, nil
}

// Start starts a created container.
func (d *DockerRuntime) Start(ctx context.Context, containerID string) error {
	return d.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// Wait blocks until the container exits and returns the exit code.
func (d *DockerRuntime) Wait(ctx context.Context, containerID string) (int, error) {
	statusCh, errCh := d.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return -1, fmt.Errorf("wait container: %w", err)
		}
	case body := <-statusCh:
		if body.Error != nil {
			return int(body.StatusCode), fmt.Errorf("container exited with error: %s", body.Error.Message)
		}
		return int(body.StatusCode), nil
	case <-ctx.Done():
		return -1, ctx.Err()
	}
	return -1, fmt.Errorf("unexpected end of wait")
}

// Logs streams container stdout and stderr to the provided writers.
func (d *DockerRuntime) Logs(ctx context.Context, containerID string, stdout, stderr io.Writer) error {
	opts := container.LogsOptions{
		ShowStdout: stdout != nil,
		ShowStderr: stderr != nil,
		Follow:     true,
	}
	reader, err := d.cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return fmt.Errorf("container logs: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Docker multiplexes stdout and stderr into a single stream with an 8-byte header.
	// stdcopy.StdCopy demuxes them.
	if stdout != nil && stderr != nil {
		_, err = stdCopy(stdout, stderr, reader)
	} else if stdout != nil {
		_, err = io.Copy(stdout, reader)
	} else if stderr != nil {
		_, err = io.Copy(stderr, reader)
	}
	return err
}

// Stop sends a stop signal to the container with a grace period.
func (d *DockerRuntime) Stop(ctx context.Context, containerID string, timeout time.Duration) error {
	timeoutSec := int(timeout.Seconds())
	return d.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeoutSec,
	})
}

// Remove removes a container, forcing removal if it's still running.
func (d *DockerRuntime) Remove(ctx context.Context, containerID string) error {
	return d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
}

// Close releases the Docker client resources.
func (d *DockerRuntime) Close() error {
	return d.cli.Close()
}
