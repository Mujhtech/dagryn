package executor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MergeEnv merges task environment variables with the system environment.
// Task environment variables take precedence over system environment variables.
func MergeEnv(taskEnv map[string]string) []string {
	return MergeEnvWithPlugins(taskEnv, nil)
}

// MergeEnvWithPlugins merges task environment variables with the system environment
// and prepends plugin paths to PATH.
func MergeEnvWithPlugins(taskEnv map[string]string, pluginPaths []string) []string {
	// Start with system environment
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	// Override with task environment
	for k, v := range taskEnv {
		env[k] = v
	}

	// Prepend plugin paths to PATH
	if len(pluginPaths) > 0 {
		currentPath := env["PATH"]
		pluginPathStr := strings.Join(pluginPaths, string(filepath.ListSeparator))
		if currentPath != "" {
			env["PATH"] = pluginPathStr + string(filepath.ListSeparator) + currentPath
		} else {
			env["PATH"] = pluginPathStr
		}
	}

	// Convert back to slice
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}

	// Sort for determinism
	sort.Strings(result)
	return result
}

// EnvToMap converts environment slice to map.
func EnvToMap(env []string) map[string]string {
	result := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// MapToEnv converts environment map to slice.
func MapToEnv(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	sort.Strings(result)
	return result
}
