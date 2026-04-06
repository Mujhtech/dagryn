package cluster

import (
	"context"
	"testing"

	"github.com/mujhtech/dagryn/pkg/dagryn/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeWorkers(n int, activeTasksEach ...int32) []*ConnectedWorker {
	workers := make([]*ConnectedWorker, n)
	for i := range n {
		active := int32(0)
		if i < len(activeTasksEach) {
			active = activeTasksEach[i]
		}
		workers[i] = &ConnectedWorker{
			ID:          string(rune('A' + i)),
			ActiveTasks: active,
			Info: &WorkerInfo{
				Hostname:           "host-" + string(rune('A'+i)),
				MaxConcurrentTasks: 10,
				Labels:             map[string]string{},
			},
		}
	}
	return workers
}

func TestRoundRobinRouter(t *testing.T) {
	router := &RoundRobinRouter{}
	workers := makeWorkers(3)
	ctx := context.Background()
	dummyTask := &task.Task{Name: "test"}

	selected := make(map[string]int)
	for range 6 {
		w, err := router.SelectWorker(ctx, dummyTask, workers)
		require.NoError(t, err)
		selected[w.ID]++
	}

	// Each worker should be selected twice
	assert.Equal(t, 2, selected["A"])
	assert.Equal(t, 2, selected["B"])
	assert.Equal(t, 2, selected["C"])
}

func TestLeastLoadedRouter(t *testing.T) {
	router := &LeastLoadedRouter{}
	workers := makeWorkers(3, 5, 1, 3)
	ctx := context.Background()

	w, err := router.SelectWorker(ctx, &task.Task{Name: "test"}, workers)
	require.NoError(t, err)
	assert.Equal(t, "B", w.ID) // Least loaded
}

func TestLabelAffinityRouter(t *testing.T) {
	router := NewLabelAffinityRouter(&LeastLoadedRouter{})
	ctx := context.Background()

	workers := []*ConnectedWorker{
		{ID: "w1", ActiveTasks: 0, Info: &WorkerInfo{Labels: map[string]string{"gpu": "true"}, MaxConcurrentTasks: 4}},
		{ID: "w2", ActiveTasks: 0, Info: &WorkerInfo{Labels: map[string]string{"gpu": "false"}, MaxConcurrentTasks: 4}},
	}

	// Task requires gpu=true
	taskWithGPU := &task.Task{
		Name: "gpu-task",
		Routing: &task.TaskRoutingConfig{
			Labels: map[string]string{"gpu": "true"},
		},
	}

	w, err := router.SelectWorker(ctx, taskWithGPU, workers)
	require.NoError(t, err)
	assert.Equal(t, "w1", w.ID)

	// Task with no labels uses fallback
	taskNoLabels := &task.Task{Name: "any-task"}
	_, err = router.SelectWorker(ctx, taskNoLabels, workers)
	assert.NoError(t, err)
}

func TestNoAvailableWorkers(t *testing.T) {
	for _, router := range []TaskRouter{&RoundRobinRouter{}, &LeastLoadedRouter{}} {
		_, err := router.SelectWorker(context.Background(), &task.Task{Name: "test"}, nil)
		assert.Error(t, err)
	}
}

func TestNewRouter(t *testing.T) {
	assert.IsType(t, &LeastLoadedRouter{}, NewRouter("least-loaded"))
	assert.IsType(t, &LeastLoadedRouter{}, NewRouter(""))
	assert.IsType(t, &RoundRobinRouter{}, NewRouter("round-robin"))
	assert.IsType(t, &LabelAffinityRouter{}, NewRouter("label-affinity"))
}
