package config

import (
	"fmt"
	"time"

	"github.com/mujhtech/dagryn/internal/dag"
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
		if tc.Command == "" {
			errors = append(errors, ValidationError{
				Task:    name,
				Message: "command is required",
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
