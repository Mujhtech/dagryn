package executor

import (
	"bytes"
	"io"
	"sync"
)

// OutputCapture captures command output for logging and caching.
type OutputCapture struct {
	mu     sync.Mutex
	stdout bytes.Buffer
	stderr bytes.Buffer

	// Optional writers to stream output in real-time
	stdoutWriter io.Writer
	stderrWriter io.Writer
}

// NewOutputCapture creates a new output capture.
func NewOutputCapture() *OutputCapture {
	return &OutputCapture{}
}

// NewOutputCaptureWithWriters creates a new output capture with streaming writers.
func NewOutputCaptureWithWriters(stdout, stderr io.Writer) *OutputCapture {
	return &OutputCapture{
		stdoutWriter: stdout,
		stderrWriter: stderr,
	}
}

// StdoutWriter returns a writer for stdout.
func (o *OutputCapture) StdoutWriter() io.Writer {
	return &captureWriter{
		capture:    o,
		isStderr:   false,
		downstream: o.stdoutWriter,
	}
}

// StderrWriter returns a writer for stderr.
func (o *OutputCapture) StderrWriter() io.Writer {
	return &captureWriter{
		capture:    o,
		isStderr:   true,
		downstream: o.stderrWriter,
	}
}

// Stdout returns the captured stdout.
func (o *OutputCapture) Stdout() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.stdout.String()
}

// Stderr returns the captured stderr.
func (o *OutputCapture) Stderr() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.stderr.String()
}

// Combined returns combined stdout and stderr output.
func (o *OutputCapture) Combined() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.stdout.String() + o.stderr.String()
}

// Reset clears the captured output.
func (o *OutputCapture) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stdout.Reset()
	o.stderr.Reset()
}

// captureWriter is an io.Writer that captures and optionally streams output.
type captureWriter struct {
	capture    *OutputCapture
	isStderr   bool
	downstream io.Writer
}

func (w *captureWriter) Write(p []byte) (n int, err error) {
	w.capture.mu.Lock()
	if w.isStderr {
		n, err = w.capture.stderr.Write(p)
	} else {
		n, err = w.capture.stdout.Write(p)
	}
	w.capture.mu.Unlock()

	// Stream to downstream writer if present
	if w.downstream != nil {
		_, _ = w.downstream.Write(p)
	}

	return n, err
}
