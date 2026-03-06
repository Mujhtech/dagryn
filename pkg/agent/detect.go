package agent

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DetectedEnvironment holds auto-detected information about the worker's environment.
type DetectedEnvironment struct {
	Type         string            // "kubernetes", "docker", "bare-metal"
	OS           string
	Arch         string
	Hostname     string
	Labels       map[string]string
	Capabilities []string
}

// DetectEnvironment auto-detects the worker's execution environment.
func DetectEnvironment() *DetectedEnvironment {
	env := &DetectedEnvironment{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Labels: make(map[string]string),
	}

	hostname, err := os.Hostname()
	if err == nil {
		env.Hostname = hostname
	}

	// Detect environment type
	switch {
	case os.Getenv("KUBERNETES_SERVICE_HOST") != "":
		env.Type = "kubernetes"
		env.Labels["environment"] = "kubernetes"
		if ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			env.Labels["k8s.namespace"] = strings.TrimSpace(string(ns))
		}
	case fileExists("/.dockerenv"):
		env.Type = "docker"
		env.Labels["environment"] = "docker"
	default:
		env.Type = "bare-metal"
		env.Labels["environment"] = "bare-metal"
	}

	env.Labels["os"] = env.OS
	env.Labels["arch"] = env.Arch

	// Detect capabilities
	// Always add arch and OS as capabilities for routing
	env.Capabilities = append(env.Capabilities, env.Arch) // e.g. "arm64", "amd64"
	env.Capabilities = append(env.Capabilities, env.OS)   // e.g. "linux", "darwin"
	if dockerAvailable() {
		env.Capabilities = append(env.Capabilities, "docker")
	}
	if gpuAvailable() {
		env.Capabilities = append(env.Capabilities, "gpu")
	}
	env.Labels["cpu_count"] = fmt.Sprintf("%d", runtime.NumCPU())

	return env
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func gpuAvailable() bool {
	cmd := exec.Command("nvidia-smi")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}
