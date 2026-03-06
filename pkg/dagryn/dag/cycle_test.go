package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectCycle_NoCycle(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	err := DetectCycle(g)
	assert.Nil(t, err)
}

func TestDetectCycle_SimpleCycle(t *testing.T) {
	g := New()
	_ = g.AddNode("a")
	_ = g.AddNode("b")
	_ = g.AddEdge("a", "b")
	_ = g.AddEdge("b", "a")

	err := DetectCycle(g)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "cyclic dependency detected")
}

func TestDetectCycle_ThreeNodeCycle(t *testing.T) {
	g := New()
	_ = g.AddNode("a")
	_ = g.AddNode("b")
	_ = g.AddNode("c")
	_ = g.AddEdge("a", "b")
	_ = g.AddEdge("b", "c")
	_ = g.AddEdge("c", "a")

	err := DetectCycle(g)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "cyclic dependency detected")
}

func TestDetectCycle_SelfLoop(t *testing.T) {
	g := New()
	_ = g.AddNode("a")
	_ = g.AddEdge("a", "a")

	err := DetectCycle(g)
	require.NotNil(t, err)
}

func TestDetectCycle_DisconnectedWithCycle(t *testing.T) {
	g := New()
	// Disconnected component without cycle
	_ = g.AddNode("x")
	_ = g.AddNode("y")
	_ = g.AddEdge("x", "y")

	// Component with cycle
	_ = g.AddNode("a")
	_ = g.AddNode("b")
	_ = g.AddEdge("a", "b")
	_ = g.AddEdge("b", "a")

	err := DetectCycle(g)
	require.NotNil(t, err)
}

func TestDetectCycle_EmptyGraph(t *testing.T) {
	g := New()
	err := DetectCycle(g)
	assert.Nil(t, err)
}

func TestDetectCycle_SingleNode(t *testing.T) {
	g := New()
	_ = g.AddNode("a")

	err := DetectCycle(g)
	assert.Nil(t, err)
}

func TestHasCycle(t *testing.T) {
	g := New()
	_ = g.AddNode("a")
	_ = g.AddNode("b")
	_ = g.AddEdge("a", "b")

	assert.False(t, HasCycle(g))

	_ = g.AddEdge("b", "a")
	assert.True(t, HasCycle(g))
}

func TestCycleError_FormatPath(t *testing.T) {
	err := &CycleError{Path: []string{"a", "b", "c"}}
	assert.Equal(t, "a → b → c → a", err.FormatPath())

	err2 := &CycleError{Path: []string{}}
	assert.Equal(t, "", err2.FormatPath())
}
