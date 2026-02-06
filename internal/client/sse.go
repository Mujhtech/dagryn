package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// RunEventData contains data for run events.
type RunEventData struct {
	RunID        uuid.UUID `json:"run_id"`
	ProjectID    uuid.UUID `json:"project_id"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// TaskEventData contains data for task events.
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

// SSEHandler is a function that handles SSE events.
type SSEHandler func(event SSEEvent) error

// StreamRunEvents streams run events via SSE.
func (c *Client) StreamRunEvents(ctx context.Context, projectID, runID uuid.UUID, handler SSEHandler) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/events", projectID, runID)
	return c.streamSSE(ctx, path, handler)
}

// StreamRunLogs streams run logs via SSE.
func (c *Client) StreamRunLogs(ctx context.Context, projectID, runID uuid.UUID, handler SSEHandler) error {
	path := fmt.Sprintf("/api/v1/projects/%s/runs/%s/logs", projectID, runID)
	return c.streamSSE(ctx, path, handler)
}

func (c *Client) streamSSE(ctx context.Context, path string, handler SSEHandler) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Add authentication if available
	if c.creds != nil && c.creds.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	var event SSEEvent
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Empty line = end of event
		if line == "" {
			if len(dataLines) > 0 {
				dataStr := strings.Join(dataLines, "\n")
				if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
					// Try to handle partial events
					continue
				}

				if err := handler(event); err != nil {
					return err
				}
			}
			// Reset for next event
			event = SSEEvent{}
			dataLines = nil
			continue
		}

		// Parse SSE fields
		if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		} else if strings.HasPrefix(line, "event:") {
			event.Type = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	return nil
}

// ParseRunEvent parses run event data.
func ParseRunEvent(event SSEEvent) (*RunEventData, error) {
	var data RunEventData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse run event: %w", err)
	}
	return &data, nil
}

// ParseTaskEvent parses task event data.
func ParseTaskEvent(event SSEEvent) (*TaskEventData, error) {
	var data TaskEventData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse task event: %w", err)
	}
	return &data, nil
}

// ParseLogEvent parses log event data.
func ParseLogEvent(event SSEEvent) (*LogEventData, error) {
	var data LogEventData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse log event: %w", err)
	}
	return &data, nil
}
