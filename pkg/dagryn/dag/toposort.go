package dag

import "fmt"

// ExecutionPlan represents an ordered execution plan for the DAG.
// Tasks are grouped by levels, where all tasks in a level can run in parallel.
type ExecutionPlan struct {
	// Levels contains tasks grouped by dependency level.
	// Level 0 contains tasks with no dependencies.
	// Level 1 contains tasks that depend only on level 0 tasks, etc.
	Levels [][]string
}

// AllTasks returns all tasks in execution order (flattened levels).
func (p *ExecutionPlan) AllTasks() []string {
	var tasks []string
	for _, level := range p.Levels {
		tasks = append(tasks, level...)
	}
	return tasks
}

// TotalTasks returns the total number of tasks in the plan.
func (p *ExecutionPlan) TotalTasks() int {
	count := 0
	for _, level := range p.Levels {
		count += len(level)
	}
	return count
}

// TopoSort performs a topological sort on the graph using Kahn's algorithm.
// Returns tasks grouped by levels for parallel execution.
func TopoSort(g *Graph) (*ExecutionPlan, error) {
	if err := DetectCycle(g); err != nil {
		return nil, err
	}

	if g.Size() == 0 {
		return &ExecutionPlan{Levels: [][]string{}}, nil
	}

	// Calculate in-degrees (number of nodes depending on each node)
	// Note: In our graph, edges point FROM dependent TO dependency
	// So we need to count how many nodes each node depends on
	inDegree := make(map[string]int)
	for _, node := range g.Nodes() {
		inDegree[node.Name] = len(node.Edges)
	}

	// Find all root nodes (nodes with no dependencies - inDegree 0)
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	plan := &ExecutionPlan{Levels: [][]string{}}
	processed := 0

	for len(queue) > 0 {
		// All nodes in queue can be executed in parallel (same level)
		level := make([]string, len(queue))
		copy(level, queue)
		plan.Levels = append(plan.Levels, level)
		processed += len(level)

		// Process next level
		var nextQueue []string
		for _, node := range queue {
			// Find all nodes that depend on this node
			dependents := g.GetDependents(node)
			for _, dep := range dependents {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					nextQueue = append(nextQueue, dep)
				}
			}
		}
		queue = nextQueue
	}

	if processed != g.Size() {
		return nil, fmt.Errorf("graph contains a cycle")
	}

	return plan, nil
}

// TopoSortFrom performs a topological sort starting from specific target nodes.
// It only includes nodes that are dependencies (direct or transitive) of the targets.
func TopoSortFrom(g *Graph, targets []string) (*ExecutionPlan, error) {
	if len(targets) == 0 {
		return &ExecutionPlan{Levels: [][]string{}}, nil
	}

	// Find all nodes needed for the targets (BFS from targets following dependencies)
	needed := make(map[string]bool)
	queue := make([]string, len(targets))
	copy(queue, targets)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if needed[node] {
			continue
		}
		needed[node] = true

		// Add all dependencies to the queue
		for _, dep := range g.GetDependencies(node) {
			if !needed[dep] {
				queue = append(queue, dep)
			}
		}
	}

	// Create a subgraph with only the needed nodes
	subgraph := New()
	for name := range needed {
		_ = subgraph.AddNode(name)
	}
	for name := range needed {
		origNode, _ := g.GetNode(name)
		for _, dep := range origNode.Edges {
			if needed[dep] {
				_ = subgraph.AddEdge(name, dep)
			}
		}
	}

	return TopoSort(subgraph)
}

// BuildFromWorkflow creates a graph from a workflow's task dependencies.
func BuildFromWorkflow(tasks map[string][]string) (*Graph, error) {
	g := New()

	// Add all nodes
	for name := range tasks {
		if err := g.AddNode(name); err != nil {
			return nil, err
		}
	}

	// Add edges (from task to its dependencies)
	for name, deps := range tasks {
		for _, dep := range deps {
			if err := g.AddEdge(name, dep); err != nil {
				return nil, fmt.Errorf("task %q depends on unknown task %q", name, dep)
			}
		}
	}

	return g, nil
}
