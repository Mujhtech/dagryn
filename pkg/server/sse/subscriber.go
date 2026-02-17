package sse

import (
	"context"
	"encoding/json"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"

	"github.com/mujhtech/dagryn/pkg/redis"
)

// RedisSubscriber subscribes to the Redis SSE channel and forwards events to the Hub.
type RedisSubscriber struct {
	rds    *redis.Redis
	hub    *Hub
	cancel context.CancelFunc
	done   chan struct{}
}

// NewRedisSubscriber creates a new RedisSubscriber.
func NewRedisSubscriber(rds *redis.Redis, hub *Hub) *RedisSubscriber {
	return &RedisSubscriber{
		rds:  rds,
		hub:  hub,
		done: make(chan struct{}),
	}
}

// Start begins subscribing to the Redis SSE channel in a goroutine.
// It blocks until ctx is cancelled or Stop is called.
func (s *RedisSubscriber) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	go func() {
		defer close(s.done)
		s.run(ctx)
	}()
}

func (s *RedisSubscriber) run(ctx context.Context) {
	pubsub := s.rds.Client().Subscribe(ctx, RedisSSEChannel)
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel(goredis.WithChannelHealthCheckInterval(0))

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var pm PublishMessage
			if err := json.Unmarshal([]byte(msg.Payload), &pm); err != nil {
				slog.Warn("sse subscriber: failed to unmarshal message", "error", err)
				continue
			}
			s.hub.Publish(pm.Topics, pm.Event)
		}
	}
}

// Stop gracefully stops the subscriber.
func (s *RedisSubscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
}
