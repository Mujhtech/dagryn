package job

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/mujhtech/dagryn/internal/redis"
	"github.com/stretchr/testify/require"
)

func TestCancelManager_SignalAndWatch(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	host, portStr, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)

	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	rds := redis.New(redis.Config{Host: host, Port: port})
	mgr := NewCancelManager(rds)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	runID := "run-123"
	ch := mgr.Watch(ctx, runID)

	require.NoError(t, mgr.Signal(ctx, runID))

	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatalf("timeout waiting for cancel signal")
	}
}

func TestCancelManager_WatchExistingKey(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	host, portStr, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)

	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	rds := redis.New(redis.Config{Host: host, Port: port})
	mgr := NewCancelManager(rds)
	ctx := context.Background()

	runID := "run-456"
	require.NoError(t, mgr.Signal(ctx, runID))

	ch := mgr.Watch(ctx, runID)
	select {
	case <-ch:
	default:
		t.Fatalf("expected channel to close immediately when key exists")
	}
}

func TestCancelManager_Clear(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	host, portStr, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)

	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	rds := redis.New(redis.Config{Host: host, Port: port})
	mgr := NewCancelManager(rds)
	ctx := context.Background()

	runID := "run-789"
	require.NoError(t, mgr.Signal(ctx, runID))
	require.NoError(t, mgr.Clear(ctx, runID))

	exists, err := rds.Client().Exists(ctx, cancelKey(runID)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)
}
