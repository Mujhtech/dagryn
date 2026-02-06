package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopoSort_SimpleChain(t *testing.T) {
	// test -> build -> install
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")

	plan, err := TopoSort(g)
	require.NoError(t, err)

	// install should be first (no deps), then build, then test
	assert.Len(t, plan.Levels, 3)
	assert.Equal(t, []string{"install"}, plan.Levels[0])
	assert.Equal(t, []string{"build"}, plan.Levels[1])
	assert.Equal(t, []string{"test"}, plan.Levels[2])
}

func TestTopoSort_Parallel(t *testing.T) {
	// build and lint both depend on install
	// test depends on build
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("lint")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")
	_ = g.AddEdge("lint", "install")

	plan, err := TopoSort(g)
	require.NoError(t, err)

	// Level 0: install
	// Level 1: build, lint (parallel)
	// Level 2: test
	assert.Len(t, plan.Levels, 3)
	assert.Equal(t, []string{"install"}, plan.Levels[0])
	assert.Len(t, plan.Levels[1], 2)
	assert.Contains(t, plan.Levels[1], "build")
	assert.Contains(t, plan.Levels[1], "lint")
}

func TestTopoSort_EmptyGraph(t *testing.T) {
	g := New()
	plan, err := TopoSort(g)
	require.NoError(t, err)
	assert.Empty(t, plan.Levels)
}

func TestTopoSort_SingleNode(t *testing.T) {
	g := New()
	_ = g.AddNode("build")

	plan, err := TopoSort(g)
	require.NoError(t, err)
	assert.Len(t, plan.Levels, 1)
	assert.Equal(t, []string{"build"}, plan.Levels[0])
}

func TestTopoSort_Cycle(t *testing.T) {
	g := New()
	_ = g.AddNode("a")
	_ = g.AddNode("b")
	_ = g.AddEdge("a", "b")
	_ = g.AddEdge("b", "a")

	_, err := TopoSort(g)
	require.Error(t, err)
}

func TestTopoSort_IndependentNodes(t *testing.T) {
	g := New()
	_ = g.AddNode("a")
	_ = g.AddNode("b")
	_ = g.AddNode("c")

	plan, err := TopoSort(g)
	require.NoError(t, err)

	// All independent - should be in one level
	assert.Len(t, plan.Levels, 1)
	assert.Len(t, plan.Levels[0], 3)
}

func TestTopoSortFrom_SpecificTarget(t *testing.T) {
	// Full graph: test -> build -> install, lint -> install
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("lint")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")
	_ = g.AddEdge("lint", "install")

	// Only need 'build' and its deps (install)
	plan, err := TopoSortFrom(g, []string{"build"})
	require.NoError(t, err)

	allTasks := plan.AllTasks()
	assert.Len(t, allTasks, 2)
	assert.Contains(t, allTasks, "build")
	assert.Contains(t, allTasks, "install")
	assert.NotContains(t, allTasks, "test")
	assert.NotContains(t, allTasks, "lint")
}

func TestTopoSortFrom_MultipleTargets(t *testing.T) {
	g := New()
	_ = g.AddNode("test")
	_ = g.AddNode("build")
	_ = g.AddNode("lint")
	_ = g.AddNode("install")
	_ = g.AddEdge("test", "build")
	_ = g.AddEdge("build", "install")
	_ = g.AddEdge("lint", "install")

	plan, err := TopoSortFrom(g, []string{"test", "lint"})
	require.NoError(t, err)

	allTasks := plan.AllTasks()
	assert.Len(t, allTasks, 4)
}

func TestTopoSortFrom_EmptyTargets(t *testing.T) {
	g := New()
	_ = g.AddNode("a")

	plan, err := TopoSortFrom(g, []string{})
	require.NoError(t, err)
	assert.Empty(t, plan.Levels)
}

func TestExecutionPlan_AllTasks(t *testing.T) {
	plan := &ExecutionPlan{
		Levels: [][]string{
			{"install"},
			{"build", "lint"},
			{"test"},
		},
	}

	tasks := plan.AllTasks()
	assert.Len(t, tasks, 4)
	assert.Equal(t, "install", tasks[0])
}

func TestExecutionPlan_TotalTasks(t *testing.T) {
	plan := &ExecutionPlan{
		Levels: [][]string{
			{"install"},
			{"build", "lint"},
			{"test"},
		},
	}

	assert.Equal(t, 4, plan.TotalTasks())
}

func TestBuildFromWorkflow(t *testing.T) {
	tasks := map[string][]string{
		"test":    {"build"},
		"build":   {"install"},
		"install": {},
		"lint":    {},
	}

	g, err := BuildFromWorkflow(tasks)
	require.NoError(t, err)

	assert.Equal(t, 4, g.Size())
	assert.Equal(t, []string{"build"}, g.GetDependencies("test"))
	assert.Equal(t, []string{"install"}, g.GetDependencies("build"))
}

func TestBuildFromWorkflow_MissingDependency(t *testing.T) {
	tasks := map[string][]string{
		"build": {"nonexistent"},
	}

	_, err := BuildFromWorkflow(tasks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown task")
}
