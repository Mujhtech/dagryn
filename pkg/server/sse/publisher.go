package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/redis"
)

const (
	// RedisSSEChannel is the Redis pub/sub channel for SSE events.
	RedisSSEChannel = "dagryn:sse:events"
)

// PublishMessage wraps an Event with its routing topics for Redis transport.
type PublishMessage struct {
	Topics []string `json:"topics"`
	Event  Event    `json:"event"`
}

// EventPublisher publishes SSE events (e.g. to Redis) so that separate server
// processes can forward them to connected browser clients.
type EventPublisher interface {
	PublishRunEvent(ctx context.Context, eventType EventType, runID, projectID uuid.UUID, status, errorMessage string)
	PublishTaskEvent(ctx context.Context, eventType EventType, runID uuid.UUID, taskName, status string, exitCode *int, durationMs *int64, cacheHit bool, cacheKey string)
	PublishLogEvent(ctx context.Context, runID uuid.UUID, taskName, stream, line string, lineNum int)
}

// RedisEventPublisher publishes SSE events to a Redis pub/sub channel.
type RedisEventPublisher struct {
	rds *redis.Redis
}

// NewRedisEventPublisher creates a new RedisEventPublisher.
func NewRedisEventPublisher(rds *redis.Redis) *RedisEventPublisher {
	return &RedisEventPublisher{rds: rds}
}

func (p *RedisEventPublisher) publish(ctx context.Context, msg PublishMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Warn("sse: failed to marshal publish message", "error", err)
		return
	}
	if err := p.rds.Client().Publish(ctx, RedisSSEChannel, data).Err(); err != nil {
		slog.Warn("sse: failed to publish event to Redis", "error", err)
	}
}

func (p *RedisEventPublisher) PublishRunEvent(ctx context.Context, eventType EventType, runID, projectID uuid.UUID, status, errorMessage string) {
	topics := []string{
		fmt.Sprintf("run:%s", runID),
		fmt.Sprintf("project:%s", projectID),
	}
	p.publish(ctx, PublishMessage{
		Topics: topics,
		Event: Event{
			ID:   uuid.New().String(),
			Type: eventType,
			Data: RunEventData{
				RunID:        runID,
				ProjectID:    projectID,
				Status:       status,
				ErrorMessage: errorMessage,
			},
		},
	})
}

func (p *RedisEventPublisher) PublishTaskEvent(ctx context.Context, eventType EventType, runID uuid.UUID, taskName, status string, exitCode *int, durationMs *int64, cacheHit bool, cacheKey string) {
	topics := []string{
		fmt.Sprintf("run:%s", runID),
	}
	p.publish(ctx, PublishMessage{
		Topics: topics,
		Event: Event{
			ID:   uuid.New().String(),
			Type: eventType,
			Data: TaskEventData{
				RunID:      runID,
				TaskName:   taskName,
				Status:     status,
				ExitCode:   exitCode,
				DurationMs: durationMs,
				CacheHit:   cacheHit,
				CacheKey:   cacheKey,
			},
		},
	})
}

func (p *RedisEventPublisher) PublishLogEvent(ctx context.Context, runID uuid.UUID, taskName, stream, line string, lineNum int) {
	topics := []string{
		fmt.Sprintf("run:%s", runID),
		fmt.Sprintf("logs:%s", runID),
	}
	p.publish(ctx, PublishMessage{
		Topics: topics,
		Event: Event{
			ID:   uuid.New().String(),
			Type: EventLogLine,
			Data: LogEventData{
				RunID:    runID,
				TaskName: taskName,
				Stream:   stream,
				Line:     line,
				LineNum:  lineNum,
			},
		},
	})
}

// NoOpEventPublisher discards all events. Used as a fallback when Redis is unavailable.
type NoOpEventPublisher struct{}

func (NoOpEventPublisher) PublishRunEvent(context.Context, EventType, uuid.UUID, uuid.UUID, string, string) {
}
func (NoOpEventPublisher) PublishTaskEvent(context.Context, EventType, uuid.UUID, string, string, *int, *int64, bool, string) {
}
func (NoOpEventPublisher) PublishLogEvent(context.Context, uuid.UUID, string, string, string, int) {
}
