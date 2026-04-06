package cluster

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/mujhtech/dagryn/pkg/dagryn/task"
)

// TaskRouter is the strategy for assigning tasks to workers.
type TaskRouter interface {
	SelectWorker(ctx context.Context, t *task.Task, available []*ConnectedWorker) (*ConnectedWorker, error)
}

// RoundRobinRouter distributes tasks evenly across available workers.
type RoundRobinRouter struct {
	counter atomic.Uint64
}

func (r *RoundRobinRouter) SelectWorker(_ context.Context, _ *task.Task, available []*ConnectedWorker) (*ConnectedWorker, error) {
	if len(available) == 0 {
		return nil, fmt.Errorf("no available workers")
	}
	n := uint64(len(available))
	idx := (r.counter.Add(1) - 1) % n
	return available[idx], nil
}

// LeastLoadedRouter picks the worker with the fewest active tasks.
type LeastLoadedRouter struct{}

func (r *LeastLoadedRouter) SelectWorker(_ context.Context, _ *task.Task, available []*ConnectedWorker) (*ConnectedWorker, error) {
	if len(available) == 0 {
		return nil, fmt.Errorf("no available workers")
	}

	var best *ConnectedWorker
	bestLoad := int32(math.MaxInt32)
	for _, w := range available {
		if w.ActiveTasks < bestLoad {
			bestLoad = w.ActiveTasks
			best = w
		}
	}
	return best, nil
}

// LabelAffinityRouter matches task label requirements to worker labels.
type LabelAffinityRouter struct {
	fallback TaskRouter
}

// NewLabelAffinityRouter creates a label affinity router with a fallback strategy.
func NewLabelAffinityRouter(fallback TaskRouter) *LabelAffinityRouter {
	return &LabelAffinityRouter{fallback: fallback}
}

func (r *LabelAffinityRouter) SelectWorker(ctx context.Context, t *task.Task, available []*ConnectedWorker) (*ConnectedWorker, error) {
	if len(available) == 0 {
		return nil, fmt.Errorf("no available workers")
	}

	// If task has no routing labels, use fallback
	if t.Routing == nil || len(t.Routing.Labels) == 0 {
		return r.fallback.SelectWorker(ctx, t, available)
	}

	// Filter workers matching required labels
	var matching []*ConnectedWorker
	for _, w := range available {
		if matchLabels(w.Info.Labels, t.Routing.Labels) {
			matching = append(matching, w)
		}
	}

	if len(matching) == 0 {
		return nil, fmt.Errorf("no workers match required labels %v", t.Routing.Labels)
	}

	return r.fallback.SelectWorker(ctx, t, matching)
}

// NewRouter creates a TaskRouter from a strategy name.
func NewRouter(strategy string) TaskRouter {
	switch strategy {
	case "round-robin":
		return &RoundRobinRouter{}
	case "label-affinity":
		return NewLabelAffinityRouter(&LeastLoadedRouter{})
	default:
		return &LeastLoadedRouter{}
	}
}
