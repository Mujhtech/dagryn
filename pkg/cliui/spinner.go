package cliui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an inline animated spinner for long-running operations.
// It uses ANSI escape codes (\r + clear-to-EOL) and does not take over
// the terminal like a full TUI framework.
type Spinner struct {
	writer  io.Writer
	message string
	done    chan struct{}
	mu      sync.Mutex
	active  bool
	isTTY   bool
}

// NewSpinner creates a new spinner. Animation is skipped when output is not a TTY.
func NewSpinner(w io.Writer, message string) *Spinner {
	isTTY := false
	if f, ok := w.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}
	return &Spinner{
		writer:  w,
		message: message,
		done:    make(chan struct{}),
		isTTY:   isTTY,
	}
}

// Start begins the spinner animation in a background goroutine.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.mu.Unlock()

	if !s.isTTY {
		// Non-TTY: just print the message once.
		_, _ = fmt.Fprintf(s.writer, "%s...\n", s.message)
		return
	}

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				frame := spinnerFrames[i%len(spinnerFrames)]
				_, _ = fmt.Fprintf(s.writer, "\r\033[K%s %s", frame, s.message)
				i++
			}
		}
	}()
}

// Stop ends the spinner and prints a final message on the same line.
// If finalMessage is empty the line is cleared.
func (s *Spinner) Stop(finalMessage string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}
	s.active = false
	close(s.done)

	if !s.isTTY {
		if finalMessage != "" {
			_, _ = fmt.Fprintln(s.writer, finalMessage)
		}
		return
	}

	if finalMessage != "" {
		_, _ = fmt.Fprintf(s.writer, "\r\033[K%s\n", finalMessage)
	} else {
		_, _ = fmt.Fprint(s.writer, "\r\033[K")
	}
}
