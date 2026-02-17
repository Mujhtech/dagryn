package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	g := New()
	assert.NotNil(t, g)
	assert.Equal(t, 0, g.Size())
}

func TestGraph_AddNode(t *testing.T) {
	g := New()

	err := g.AddNode("build")
	require.NoError(t, err)
	assert.Equal(t, 1, g.Size())

	// Duplicate node should fail
	err = g.AddNode("build")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestGraph_AddEdge(t *testing.T) {
	g := New()
	_ = g.AddNode("build")
	_ = g.AddNode("install")

	// Valid edge
	err := g.AddEdge("build", "install")
	require.NoError(t, err)

	// Edge from non-existent node
	err = g.AddEdge("nonexistent", "install")
	require.Error(t, err)

	// Edge to non-existent node
	err = g.AddEdge("build", "nonexistent")
	require.Error(t, err)

	// Duplicate edge should be no-op
	err = g.AddEdge("build", "install")
	require.NoError(t, err)
	deps := g.GetDependencies("build")
	assert.Len(t, deps, 1)
}

func TestGraph_GetNode(t *testing.T) {
	g := New()
	_ = g.AddNode("build")

	node, ok := g.GetNode("build")
	assert.True(t, ok)
	assert.Equal(t, "build", node.Name)

	_, ok = g.GetNode("nonexistent")
	assert.False(t, ok)
}

func TestGraph_HasNode(t *testing.T) {
	g := New()
	_ = g.AddNode("build")

	assert.True(t, g.HasNode("build"))
	assert.False(t, g.HasNode("nonexistent"))
}

func TestGraph_GetDependencies(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	deps := g.GetDependencies("test")
	assert.Equal(t, []string{"build"}, deps)

	deps = g.GetDependencies("build")
	assert.Equal(t, []string{"install"}, deps)

	deps = g.GetDependencies("install")
	assert.Empty(t, deps)

	deps = g.GetDependencies("nonexistent")
	assert.Nil(t, deps)
}

func TestGraph_GetDependents(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	dependents := g.GetDependents("install")
	assert.Equal(t, []string{"build"}, dependents)

	dependents = g.GetDependents("build")
	assert.Equal(t, []string{"test"}, dependents)

	dependents = g.GetDependents("test")
	assert.Empty(t, dependents)
}

func TestGraph_RootNodes(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddNode("lint")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	roots := g.RootNodes()
	assert.Len(t, roots, 2)
	assert.Contains(t, roots, "install")
	assert.Contains(t, roots, "lint")
}

func TestGraph_LeafNodes(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddNode("lint")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	leaves := g.LeafNodes()
	assert.Len(t, leaves, 2)
	assert.Contains(t, leaves, "test")
	assert.Contains(t, leaves, "lint")
}

func TestGraph_InDegree(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	assert.Equal(t, 0, g.InDegree("test"))
	assert.Equal(t, 1, g.InDegree("build"))
	assert.Equal(t, 1, g.InDegree("install"))
}

func TestGraph_OutDegree(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	assert.Equal(t, 1, g.OutDegree("test"))
	assert.Equal(t, 1, g.OutDegree("build"))
	assert.Equal(t, 0, g.OutDegree("install"))
	assert.Equal(t, 0, g.OutDegree("nonexistent"))
}

func TestGraph_Clone(t *testing.T) {
	g := New()
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddEdge("build", "install")

	clone := g.Clone()

	// Verify structure is the same
	assert.Equal(t, g.Size(), clone.Size())
	assert.True(t, clone.HasNode("build"))
	assert.True(t, clone.HasNode("install"))
	assert.Equal(t, g.GetDependencies("build"), clone.GetDependencies("build"))

	// Verify it's a deep copy
	_ = clone.AddNode("test")
	assert.Equal(t, 2, g.Size())
	assert.Equal(t, 3, clone.Size())
}
