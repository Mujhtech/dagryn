package dag

import "fmt"

// Node represents a node in the DAG.
type Node struct {
	Name  string
	Edges []string // outgoing edges (tasks this node depends on)
}

// Graph represents a directed acyclic graph.
type Graph struct {
	nodes map[string]*Node
}

// New creates a new empty graph.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
	}
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(name string) error {
	if _, exists := g.nodes[name]; exists {
		return fmt.Errorf("node %q already exists", name)
	}
	g.nodes[name] = &Node{
		Name:  name,
		Edges: make([]string, 0),
	}
	return nil
}

// AddEdge adds a directed edge from 'from' to 'to'.
// This represents that 'from' depends on 'to'.
func (g *Graph) AddEdge(from, to string) error {
	fromNode, ok := g.nodes[from]
	if !ok {
		return fmt.Errorf("node %q does not exist", from)
	}
	if _, ok := g.nodes[to]; !ok {
		return fmt.Errorf("node %q does not exist", to)
	}

	// Check for duplicate edge
	for _, edge := range fromNode.Edges {
		if edge == to {
			return nil // edge already exists, no-op
		}
	}

	fromNode.Edges = append(fromNode.Edges, to)
	return nil
}

// GetNode returns a node by name.
func (g *Graph) GetNode(name string) (*Node, bool) {
	node, ok := g.nodes[name]
	return node, ok
}

// HasNode returns true if the node exists.
func (g *Graph) HasNode(name string) bool {
	_, ok := g.nodes[name]
	return ok
}

// GetDependencies returns the direct dependencies of a node.
// These are the nodes that 'name' depends on (outgoing edges).
func (g *Graph) GetDependencies(name string) []string {
	node, ok := g.nodes[name]
	if !ok {
		return nil
	}
	result := make([]string, len(node.Edges))
	copy(result, node.Edges)
	return result
}

// GetDependents returns nodes that depend on the given node.
// These are nodes that have 'name' as one of their edges.
func (g *Graph) GetDependents(name string) []string {
	dependents := make([]string, 0)
	for nodeName, node := range g.nodes {
		for _, edge := range node.Edges {
			if edge == name {
				dependents = append(dependents, nodeName)
				break
			}
		}
	}
	return dependents
}

// Nodes returns all nodes in the graph.
func (g *Graph) Nodes() []*Node {
	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// NodeNames returns all node names in the graph.
func (g *Graph) NodeNames() []string {
	names := make([]string, 0, len(g.nodes))
	for name := range g.nodes {
		names = append(names, name)
	}
	return names
}

// Size returns the number of nodes in the graph.
func (g *Graph) Size() int {
	return len(g.nodes)
}

// RootNodes returns nodes with no dependencies (no outgoing edges).
func (g *Graph) RootNodes() []string {
	roots := make([]string, 0)
	for name, node := range g.nodes {
		if len(node.Edges) == 0 {
			roots = append(roots, name)
		}
	}
	return roots
}

// LeafNodes returns nodes that no other node depends on.
func (g *Graph) LeafNodes() []string {
	// Build set of all dependencies
	depended := make(map[string]bool)
	for _, node := range g.nodes {
		for _, edge := range node.Edges {
			depended[edge] = true
		}
	}

	// Find nodes not in the depended set
	leaves := make([]string, 0)
	for name := range g.nodes {
		if !depended[name] {
			leaves = append(leaves, name)
		}
	}
	return leaves
}

// InDegree returns the number of nodes that depend on the given node.
func (g *Graph) InDegree(name string) int {
	count := 0
	for _, node := range g.nodes {
		for _, edge := range node.Edges {
			if edge == name {
				count++
				break
			}
		}
	}
	return count
}

// OutDegree returns the number of dependencies of the given node.
func (g *Graph) OutDegree(name string) int {
	node, ok := g.nodes[name]
	if !ok {
		return 0
	}
	return len(node.Edges)
}

// Clone creates a deep copy of the graph.
func (g *Graph) Clone() *Graph {
	clone := New()
	for name, node := range g.nodes {
		_ = clone.AddNode(name)
		clone.nodes[name].Edges = make([]string, len(node.Edges))
		copy(clone.nodes[name].Edges, node.Edges)
	}
	return clone
}
