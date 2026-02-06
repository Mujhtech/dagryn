// Package sse provides Server-Sent Events (SSE) functionality for real-time updates.
package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of SSE event.
type EventType string

const (
	// Run events
	EventRunStarted   EventType = "run.started"
	EventRunCompleted EventType = "run.completed"
	EventRunFailed    EventType = "run.failed"
	EventRunCancelled EventType = "run.cancelled"

	// Task events
	EventTaskStarted   EventType = "task.started"
	EventTaskCompleted EventType = "task.completed"
	EventTaskFailed    EventType = "task.failed"
	EventTaskCached    EventType = "task.cached"
	EventTaskSkipped   EventType = "task.skipped"

	// Log events
	EventLogLine EventType = "log"

	// Heartbeat
	EventHeartbeat EventType = "heartbeat"
)

// Event represents an SSE event to be sent to clients.
type Event struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// RunEventData contains data for run-related events.
type RunEventData struct {
	RunID        uuid.UUID `json:"run_id"`
	ProjectID    uuid.UUID `json:"project_id"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// TaskEventData contains data for task-related events.
type TaskEventData struct {
	RunID      uuid.UUID `json:"run_id"`
	TaskName   string    `json:"task_name"`
	Status     string    `json:"status"`
	ExitCode   *int      `json:"exit_code,omitempty"`
	DurationMs *int64    `json:"duration_ms,omitempty"`
	CacheHit   bool      `json:"cache_hit,omitempty"`
	CacheKey   string    `json:"cache_key,omitempty"`
}

// LogEventData contains data for log events.
type LogEventData struct {
	RunID    uuid.UUID `json:"run_id"`
	TaskName string    `json:"task_name,omitempty"`
	Stream   string    `json:"stream"` // "stdout" or "stderr"
	Line     string    `json:"line"`
	LineNum  int       `json:"line_num"`
}

// Client represents a connected SSE client.
type Client struct {
	ID       string
	Channel  chan Event
	Topics   map[string]bool // Subscribed topics (e.g., "run:{runID}", "project:{projectID}")
	mu       sync.RWMutex
	closed   bool
	closedMu sync.RWMutex
}

// NewClient creates a new SSE client.
func NewClient() *Client {
	return &Client{
		ID:      uuid.New().String(),
		Channel: make(chan Event, 256), // Buffered channel
		Topics:  make(map[string]bool),
	}
}

// Subscribe adds a topic subscription.
func (c *Client) Subscribe(topic string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Topics[topic] = true
}

// Unsubscribe removes a topic subscription.
func (c *Client) Unsubscribe(topic string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Topics, topic)
}

// IsSubscribed checks if client is subscribed to a topic.
func (c *Client) IsSubscribed(topic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Topics[topic]
}

// Close marks the client as closed.
func (c *Client) Close() {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.Channel)
	}
}

// IsClosed checks if the client is closed.
func (c *Client) IsClosed() bool {
	c.closedMu.RLock()
	defer c.closedMu.RUnlock()
	return c.closed
}

// Hub manages SSE clients and event distribution.
type Hub struct {
	clients    map[string]*Client
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMessage
	done       chan struct{}
}

type broadcastMessage struct {
	topics []string
	event  Event
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan broadcastMessage, 1024),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-h.done:
			// Clean up all clients
			h.mu.Lock()
			for _, client := range h.clients {
				client.Close()
			}
			h.clients = make(map[string]*Client)
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				client.Close()
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				// Check if client is subscribed to any of the topics
				for _, topic := range msg.topics {
					if client.IsSubscribed(topic) {
						// Non-blocking send
						select {
						case client.Channel <- msg.event:
						default:
							// Channel full, skip this event for this client
						}
						break
					}
				}
			}
			h.mu.RUnlock()

		case <-heartbeatTicker.C:
			// Send heartbeat to all clients
			event := Event{
				ID:        uuid.New().String(),
				Type:      EventHeartbeat,
				Timestamp: time.Now(),
				Data:      map[string]string{"status": "ok"},
			}
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.Channel <- event:
				default:
					// Skip if buffer full
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Stop stops the hub.
func (h *Hub) Stop() {
	close(h.done)
}

// Register registers a new client.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Publish sends an event to clients subscribed to the given topics.
func (h *Hub) Publish(topics []string, event Event) {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	h.broadcast <- broadcastMessage{topics: topics, event: event}
}

// PublishRunEvent publishes a run-related event.
func (h *Hub) PublishRunEvent(eventType EventType, runID, projectID uuid.UUID, status, errorMessage string) {
	topics := []string{
		fmt.Sprintf("run:%s", runID),
		fmt.Sprintf("project:%s", projectID),
	}
	h.Publish(topics, Event{
		Type: eventType,
		Data: RunEventData{
			RunID:        runID,
			ProjectID:    projectID,
			Status:       status,
			ErrorMessage: errorMessage,
		},
	})
}

// PublishTaskEvent publishes a task-related event.
func (h *Hub) PublishTaskEvent(eventType EventType, runID uuid.UUID, taskName, status string, exitCode *int, durationMs *int64, cacheHit bool, cacheKey string) {
	topics := []string{
		fmt.Sprintf("run:%s", runID),
	}
	h.Publish(topics, Event{
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
	})
}

// PublishLogEvent publishes a log line event.
func (h *Hub) PublishLogEvent(runID uuid.UUID, taskName, stream, line string, lineNum int) {
	topics := []string{
		fmt.Sprintf("run:%s", runID),
		fmt.Sprintf("logs:%s", runID),
	}
	h.Publish(topics, Event{
		Type: EventLogLine,
		Data: LogEventData{
			RunID:    runID,
			TaskName: taskName,
			Stream:   stream,
			Line:     line,
			LineNum:  lineNum,
		},
	})
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeHTTP handles an SSE connection.
func ServeSSE(w http.ResponseWriter, r *http.Request, hub *Hub, topics []string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Check if response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create and register client
	client := NewClient()
	for _, topic := range topics {
		client.Subscribe(topic)
	}
	hub.Register(client)

	// Send initial connection event
	writeSSEEvent(w, Event{
		ID:        uuid.New().String(),
		Type:      "connected",
		Timestamp: time.Now(),
		Data:      map[string]string{"client_id": client.ID},
	})
	flusher.Flush()

	// Use request context for cancellation
	ctx := r.Context()

	// Cleanup on exit
	defer func() {
		hub.Unregister(client)
	}()

	// Stream events
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-client.Channel:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeSSEEvent writes a single SSE event to the response writer.
func writeSSEEvent(w http.ResponseWriter, event Event) error {
	// Format: id: <id>\nevent: <type>\ndata: <json>\n\n
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, string(data))
	return err
}
