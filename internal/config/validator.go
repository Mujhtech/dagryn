package config

import (
	"fmt"
	"regexp"
	"time"

	"github.com/mujhtech/dagryn/internal/dag"
	"github.com/mujhtech/dagryn/internal/plugin"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Task    string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Task != "" {
		return fmt.Sprintf("task %q: %s", e.Task, e.Message)
	}
	return e.Message
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("%d validation errors: %s (and %d more)", len(e), e[0].Error(), len(e)-1)
}

// Validate performs all validation checks on the configuration.
func Validate(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	errors = append(errors, validateWorkflow(cfg)...)
	errors = append(errors, validateTasks(cfg)...)
	errors = append(errors, validateDependencies(cfg)...)
	errors = append(errors, validateTimeouts(cfg)...)
	errors = append(errors, validateNoCycles(cfg)...)
	errors = append(errors, validateCache(cfg)...)
	errors = append(errors, validatePlugins(cfg)...)
	errors = append(errors, validateGroups(cfg)...)
	errors = append(errors, validateTriggers(cfg)...)

	return errors
}

// validateWorkflow validates the workflow configuration.
func validateWorkflow(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	if cfg.Workflow.Name == "" {
		errors = append(errors, ValidationError{
			Message: "workflow name is required",
		})
	}

	return errors
}

// validateTasks validates individual task configurations.
func validateTasks(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	if len(cfg.Tasks) == 0 {
		errors = append(errors, ValidationError{
			Message: "at least one task is required",
		})
		return errors
	}

	for name, tc := range cfg.Tasks {
		// A task needs either a command or a uses spec (for composite plugins)
		if tc.Command == "" && tc.Uses.IsEmpty() {
			errors = append(errors, ValidationError{
				Task:    name,
				Message: "command is required (or 'uses' for composite plugins)",
			})
		}

		// with requires uses
		if len(tc.With) > 0 && tc.Uses.IsEmpty() {
			errors = append(errors, ValidationError{
				Task:    name,
				Message: "'with' requires 'uses' to be set",
			})
		}
	}

	return errors
}

// validateDependencies validates that all task dependencies exist.
func validateDependencies(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	for name, tc := range cfg.Tasks {
		for _, dep := range tc.Needs {
			if _, exists := cfg.Tasks[dep]; !exists {
				errors = append(errors, ValidationError{
					Task:    name,
					Message: fmt.Sprintf("depends on unknown task %q", dep),
				})
			}
		}
	}

	return errors
}

// validateTimeouts validates timeout formats.
func validateTimeouts(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	for name, tc := range cfg.Tasks {
		if tc.Timeout != "" {
			if _, err := time.ParseDuration(tc.Timeout); err != nil {
				errors = append(errors, ValidationError{
					Task:    name,
					Message: fmt.Sprintf("invalid timeout %q: %v", tc.Timeout, err),
				})
			}
		}
	}

	return errors
}

// validateNoCycles checks for cyclic dependencies in the task graph.
func validateNoCycles(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	// Build the graph
	g := dag.New()
	for name := range cfg.Tasks {
		_ = g.AddNode(name)
	}
	for name, tc := range cfg.Tasks {
		for _, dep := range tc.Needs {
			if g.HasNode(dep) {
				_ = g.AddEdge(name, dep)
			}
		}
	}

	// Detect cycles using DFS
	cycle := detectCycleDFS(g)
	if cycle != nil {
		cyclePath := formatCyclePath(cycle)
		errors = append(errors, ValidationError{
			Message: fmt.Sprintf("cyclic dependency detected: %s", cyclePath),
		})
	}

	return errors
}

// detectCycleDFS performs cycle detection using DFS with three-color marking.
func detectCycleDFS(g *dag.Graph) []string {
	const (
		white = 0 // unvisited
		gray  = 1 // visiting (in current path)
		black = 2 // visited (finished)
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	var cyclePath []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = gray

		for _, dep := range g.GetDependencies(node) {
			if color[dep] == gray {
				// Found a cycle - reconstruct the path
				cyclePath = []string{dep, node}
				for curr := node; curr != dep; {
					curr = parent[curr]
					if curr == "" {
						break
					}
					cyclePath = append([]string{curr}, cyclePath...)
				}
				return true
			}

			if color[dep] == white {
				parent[dep] = node
				if dfs(dep) {
					return true
				}
			}
		}

		color[node] = black
		return false
	}

	for _, node := range g.Nodes() {
		if color[node.Name] == white {
			if dfs(node.Name) {
				return cyclePath
			}
		}
	}

	return nil
}

// validateCache validates the cache configuration.
func validateCache(cfg *Config) ValidationErrors {
	var errors ValidationErrors
	rc := cfg.Cache.Remote

	if !rc.Enabled {
		return errors
	}

	// Cloud mode: server handles storage, no provider/bucket/base_path needed.
	if !rc.Cloud {
		switch rc.Provider {
		case "s3":
			if rc.Bucket == "" {
				errors = append(errors, ValidationError{
					Message: "cache.remote.bucket is required when provider is \"s3\"",
				})
			}
		case "filesystem":
			if rc.BasePath == "" {
				errors = append(errors, ValidationError{
					Message: "cache.remote.base_path is required when provider is \"filesystem\"",
				})
			}
		case "":
			errors = append(errors, ValidationError{
				Message: "cache.remote.provider is required when remote cache is enabled",
			})
		default:
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("cache.remote.provider %q is not supported (use \"s3\" or \"filesystem\")", rc.Provider),
			})
		}
	}

	if rc.Strategy != "" {
		switch rc.Strategy {
		case "local-first", "remote-first", "write-through":
			// valid
		default:
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("cache.remote.strategy %q is not supported (use \"local-first\", \"remote-first\", or \"write-through\")", rc.Strategy),
			})
		}
	}

	return errors
}

// validatePlugins validates that global plugin specs are parseable.
func validatePlugins(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	for name, spec := range cfg.Plugins {
		if _, err := plugin.Parse(spec); err != nil {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("global plugin %q has invalid spec %q: %v", name, spec, err),
			})
		}
	}

	return errors
}

// validGroupNameRegex defines valid group name pattern (same as task names).
var validGroupNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// validateGroups validates task group names.
func validateGroups(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	for name, tc := range cfg.Tasks {
		if tc.Group == "" {
			continue
		}

		// Group names must match valid name pattern
		if !validGroupNameRegex.MatchString(tc.Group) {
			errors = append(errors, ValidationError{
				Task:    name,
				Message: fmt.Sprintf("group %q has invalid name: must start with a letter and contain only letters, numbers, underscores, and hyphens", tc.Group),
			})
		}

		// Group names must not collide with task names
		if _, exists := cfg.Tasks[tc.Group]; exists {
			errors = append(errors, ValidationError{
				Task:    name,
				Message: fmt.Sprintf("group %q collides with a task name", tc.Group),
			})
		}
	}

	return errors
}

// knownPRTypes is the set of valid pull_request event action types.
var knownPRTypes = map[string]bool{
	"opened":           true,
	"synchronize":      true,
	"reopened":         true,
	"closed":           true,
	"edited":           true,
	"ready_for_review": true,
}

// validateTriggers validates the workflow trigger configuration.
func validateTriggers(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	if cfg.Workflow.Trigger == nil {
		return errors
	}

	if pr := cfg.Workflow.Trigger.PullRequest; pr != nil {
		for _, t := range pr.Types {
			if !knownPRTypes[t] {
				errors = append(errors, ValidationError{
					Message: fmt.Sprintf("workflow.trigger.pull_request.types contains unknown type %q", t),
				})
			}
		}
	}

	return errors
}

// formatCyclePath formats a cycle path as a string.
func formatCyclePath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	result := path[0]
	for i := 1; i < len(path); i++ {
		result += " → " + path[i]
	}
	// Add the closing arrow to show the cycle
	result += " → " + path[0]
	return result
}
