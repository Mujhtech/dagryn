package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/mujhtech/dagryn/pkg/redis"
)

const (
	cancelKeyPrefix   = "cancel:run:"
	cancelTTL         = time.Hour
	cancelChannelPref = "cancel:run:"
)

// CancelManager coordinates run cancellation signals via Redis.
type CancelManager struct {
	rds *redis.Redis
}

// NewCancelManager creates a new CancelManager.
func NewCancelManager(rds *redis.Redis) *CancelManager {
	if rds == nil {
		return nil
	}
	return &CancelManager{rds: rds}
}

func cancelKey(runID string) string {
	return fmt.Sprintf("%s%s", cancelKeyPrefix, runID)
}

func cancelChannel(runID string) string {
	return fmt.Sprintf("%s%s", cancelChannelPref, runID)
}

// Signal publishes a cancellation signal and sets a TTL key for late subscribers.
func (m *CancelManager) Signal(ctx context.Context, runID string) error {
	if m == nil || m.rds == nil {
		return nil
	}
	key := cancelKey(runID)
	if err := m.rds.Client().Set(ctx, key, "1", cancelTTL).Err(); err != nil {
		return err
	}
	return m.rds.Client().Publish(ctx, cancelChannel(runID), "cancel").Err()
}

// Watch subscribes for a cancellation signal and returns a channel that closes on cancel.
func (m *CancelManager) Watch(ctx context.Context, runID string) <-chan struct{} {
	ch := make(chan struct{})
	if m == nil || m.rds == nil {
		close(ch)
		return ch
	}

	key := cancelKey(runID)
	if exists, err := m.rds.Client().Exists(ctx, key).Result(); err == nil && exists > 0 {
		close(ch)
		return ch
	}

	pubsub := m.rds.Client().Subscribe(ctx, cancelChannel(runID))
	msgs := pubsub.Channel()

	go func() {
		defer func() {
			_ = pubsub.Close()
			select {
			case <-ch:
			default:
				close(ch)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-msgs:
				if !ok {
					return
				}
				return
			}
		}
	}()

	return ch
}

// Clear removes the cancellation key.
func (m *CancelManager) Clear(ctx context.Context, runID string) error {
	if m == nil || m.rds == nil {
		return nil
	}
	return m.rds.Client().Del(ctx, cancelKey(runID)).Err()
}
