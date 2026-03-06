package dag

import "fmt"

// CycleError represents a cyclic dependency error.
type CycleError struct {
	Path []string
}

// Error returns the error message.
func (e *CycleError) Error() string {
	return fmt.Sprintf("cyclic dependency detected: %s", e.FormatPath())
}

// FormatPath formats the cycle path as a string.
func (e *CycleError) FormatPath() string {
	if len(e.Path) == 0 {
		return ""
	}
	result := e.Path[0]
	for i := 1; i < len(e.Path); i++ {
		result += " → " + e.Path[i]
	}
	// Close the cycle
	result += " → " + e.Path[0]
	return result
}

// DetectCycle checks if the graph contains any cycles.
// Returns a CycleError if a cycle is found, nil otherwise.
func DetectCycle(g *Graph) *CycleError {
	const (
		white = 0 // unvisited
		gray  = 1 // visiting (in current path)
		black = 2 // visited (finished)
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	var dfs func(node string) []string
	dfs = func(node string) []string {
		color[node] = gray

		for _, dep := range g.GetDependencies(node) {
			if color[dep] == gray {
				// Found a cycle - reconstruct the path
				path := []string{node, dep}
				for curr := node; parent[curr] != "" && parent[curr] != dep; curr = parent[curr] {
					path = append([]string{parent[curr]}, path...)
				}
				return path
			}

			if color[dep] == white {
				parent[dep] = node
				if cycle := dfs(dep); cycle != nil {
					return cycle
				}
			}
		}

		color[node] = black
		return nil
	}

	for _, node := range g.Nodes() {
		if color[node.Name] == white {
			if cycle := dfs(node.Name); cycle != nil {
				return &CycleError{Path: cycle}
			}
		}
	}

	return nil
}

// HasCycle returns true if the graph contains a cycle.
func HasCycle(g *Graph) bool {
	return DetectCycle(g) != nil
}
