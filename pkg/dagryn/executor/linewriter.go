package executor

import (
	"bytes"
	"io"
	"sync"
)

// LineCallback is called for each complete line of output.
// The line does not include the trailing newline character.
type LineCallback func(line string)

// LineWriter buffers writes and calls a callback for each complete line.
// It also passes all data through to a downstream writer if provided.
// This is useful for streaming log lines in real-time while still
// capturing full output.
type LineWriter struct {
	downstream io.Writer    // Optional pass-through writer
	callback   LineCallback // Called for each complete line
	buffer     bytes.Buffer // Accumulates partial lines
	mu         sync.Mutex   // Protects buffer
}

// NewLineWriter creates a new line-buffered writer.
// downstream: optional writer to pass all data through (can be nil)
// callback: called for each complete line (without trailing newline)
func NewLineWriter(downstream io.Writer, cb LineCallback) *LineWriter {
	return &LineWriter{
		downstream: downstream,
		callback:   cb,
	}
}

// Write implements io.Writer.
// It buffers incoming data, extracts complete lines, calls the callback
// for each line, and passes all data through to the downstream writer.
func (w *LineWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Pass through to downstream first (if present)
	if w.downstream != nil {
		n, err = w.downstream.Write(p)
		if err != nil {
			return n, err
		}
	} else {
		n = len(p)
	}

	// Process for line callbacks
	if w.callback == nil {
		return n, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Write to buffer
	w.buffer.Write(p)

	// Extract and process complete lines
	w.processLines()

	return n, nil
}

// processLines extracts complete lines from the buffer and calls the callback.
// Must be called with mutex held.
func (w *LineWriter) processLines() {
	for {
		// Find the next newline
		data := w.buffer.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			// No complete line yet
			break
		}

		// Extract the line (without the newline)
		line := string(data[:idx])

		// Call the callback
		if w.callback != nil {
			w.callback(line)
		}

		// Advance the buffer past this line (including the newline)
		w.buffer.Next(idx + 1)
	}
}

// Flush sends any remaining partial line to the callback.
// This should be called when the writer is done (e.g., task completion)
// to ensure partial lines are not lost.
func (w *LineWriter) Flush() {
	if w.callback == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// If there's remaining data without a trailing newline, send it
	if w.buffer.Len() > 0 {
		line := w.buffer.String()
		w.callback(line)
		w.buffer.Reset()
	}
}

// Reset clears the internal buffer without calling the callback.
func (w *LineWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer.Reset()
}

// Buffered returns the number of bytes in the buffer awaiting a newline.
func (w *LineWriter) Buffered() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Len()
}
