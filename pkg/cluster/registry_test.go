package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchLabels(t *testing.T) {
	worker := map[string]string{"arch": "arm64", "zone": "us-east", "gpu": "true"}

	assert.True(t, matchLabels(worker, map[string]string{"arch": "arm64"}))
	assert.True(t, matchLabels(worker, map[string]string{"arch": "arm64", "zone": "us-east"}))
	assert.False(t, matchLabels(worker, map[string]string{"arch": "x86"}))
	assert.False(t, matchLabels(worker, map[string]string{"missing": "label"}))
	assert.True(t, matchLabels(worker, nil))
	assert.True(t, matchLabels(worker, map[string]string{}))
}

func TestMatchCapabilities(t *testing.T) {
	worker := []string{"docker", "gpu", "arm64"}

	assert.True(t, matchCapabilities(worker, []string{"docker"}))
	assert.True(t, matchCapabilities(worker, []string{"docker", "gpu"}))
	assert.False(t, matchCapabilities(worker, []string{"podman"}))
	assert.True(t, matchCapabilities(worker, nil))
	assert.True(t, matchCapabilities(worker, []string{}))
}

func TestFindWorkers(t *testing.T) {
	r := &WorkerRegistry{
		workers: map[string]*ConnectedWorker{
			"w1": {
				ID:        "w1",
				Info:      &WorkerInfo{Labels: map[string]string{"arch": "arm64", "zone": "us"}, Capabilities: []string{"docker"}},
				LastSeen:  time.Now(),
				TaskCh:    make(chan *TaskDispatch, 1),
				ControlCh: make(chan *ControlAction, 1),
			},
			"w2": {
				ID:        "w2",
				Info:      &WorkerInfo{Labels: map[string]string{"arch": "x86", "zone": "eu"}, Capabilities: []string{"docker", "gpu"}},
				LastSeen:  time.Now(),
				TaskCh:    make(chan *TaskDispatch, 1),
				ControlCh: make(chan *ControlAction, 1),
			},
		},
	}

	// Find by label
	found := r.FindWorkers(map[string]string{"arch": "arm64"}, nil)
	require.Len(t, found, 1)
	assert.Equal(t, "w1", found[0].ID)

	// Find by capability
	found = r.FindWorkers(nil, []string{"gpu"})
	require.Len(t, found, 1)
	assert.Equal(t, "w2", found[0].ID)

	// Find all
	found = r.FindWorkers(nil, nil)
	assert.Len(t, found, 2)
}

func TestListOnline(t *testing.T) {
	r := &WorkerRegistry{
		workers: map[string]*ConnectedWorker{
			"w1": {ID: "w1", Info: &WorkerInfo{Hostname: "host1"}},
			"w2": {ID: "w2", Info: &WorkerInfo{Hostname: "host2"}},
		},
	}

	online := r.ListOnline()
	assert.Len(t, online, 2)
}

func TestGetWorker(t *testing.T) {
	r := &WorkerRegistry{
		workers: map[string]*ConnectedWorker{
			"w1": {ID: "w1", Info: &WorkerInfo{Hostname: "host1"}},
		},
	}

	w, ok := r.GetWorker("w1")
	assert.True(t, ok)
	assert.Equal(t, "host1", w.Info.Hostname)

	_, ok = r.GetWorker("nonexistent")
	assert.False(t, ok)
}
