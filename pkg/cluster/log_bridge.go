package cluster

import (
	"github.com/google/uuid"
	v1 "github.com/mujhtech/dagryn/pkg/cluster/v1"
	"github.com/mujhtech/dagryn/pkg/server/sse"
	"github.com/rs/zerolog/log"
)

// LogBridge bridges gRPC log entries from remote workers to the SSE hub.
type LogBridge struct {
	sseHub *sse.Hub
}

// NewLogBridge creates a new log bridge.
func NewLogBridge(sseHub *sse.Hub) *LogBridge {
	return &LogBridge{sseHub: sseHub}
}

// HandleLogEntry publishes a log entry from a remote worker to the SSE hub.
func (b *LogBridge) HandleLogEntry(entry *v1.LogEntry) {
	if b.sseHub == nil {
		return
	}

	runID, err := uuid.Parse(entry.RunId)
	if err != nil {
		log.Warn().Str("run_id", entry.RunId).Msg("Invalid run ID in log entry")
		return
	}

	b.sseHub.PublishLogEvent(runID, entry.TaskName, entry.Stream, entry.Line, 0)
}
