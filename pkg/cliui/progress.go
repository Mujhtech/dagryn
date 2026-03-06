package cliui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

const (
	defaultBarWidth = 40
	barFill         = "█"
	barEmpty        = "░"
)

// ProgressBar displays an inline progress bar for operations with known total.
// It uses ANSI \r + clear-to-EOL to update in place on TTYs.
type ProgressBar struct {
	writer  io.Writer
	total   int
	current int
	width   int
	label   string
	mu      sync.Mutex
	isTTY   bool
	noColor bool
	done    bool
}

// NewProgressBar creates a progress bar.
// total is the number of items to complete. label is shown before the bar.
func NewProgressBar(w io.Writer, label string, total int) *ProgressBar {
	isTTY := false
	if f, ok := w.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}
	return &ProgressBar{
		writer:  w,
		total:   total,
		width:   defaultBarWidth,
		label:   label,
		isTTY:   isTTY,
		noColor: os.Getenv("NO_COLOR") != "",
	}
}

// SetWidth overrides the bar width (default 40).
func (p *ProgressBar) SetWidth(w int) {
	if w > 0 {
		p.width = w
	}
}

// Increment advances the bar by one step and redraws.
func (p *ProgressBar) Increment() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.done {
		return
	}

	p.current++
	if p.current > p.total {
		p.current = p.total
	}
	p.draw()
}

// Set sets the current count directly and redraws.
func (p *ProgressBar) Set(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.done {
		return
	}

	if n < 0 {
		n = 0
	}
	if n > p.total {
		n = p.total
	}
	p.current = n
	p.draw()
}

// Complete finishes the bar at 100% and prints a final newline.
func (p *ProgressBar) Complete(finalMessage string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.done {
		return
	}
	p.done = true
	p.current = p.total
	p.draw()

	if p.isTTY {
		if finalMessage != "" {
			_, _ = fmt.Fprintf(p.writer, "\r\033[K%s\n", finalMessage)
		} else {
			_, _ = fmt.Fprint(p.writer, "\n")
		}
	} else if finalMessage != "" {
		_, _ = fmt.Fprintln(p.writer, finalMessage)
	}
}

func (p *ProgressBar) draw() {
	pct := float64(0)
	if p.total > 0 {
		pct = float64(p.current) / float64(p.total)
	}
	filled := min(int(pct*float64(p.width)), p.width)
	empty := p.width - filled

	bar := repeat(barFill, filled) + repeat(barEmpty, empty)

	if !p.noColor {
		bar = Render(StyleSuccess, repeat(barFill, filled), false) +
			Render(StyleDim, repeat(barEmpty, empty), false)
	}

	line := fmt.Sprintf("%s %s %3.0f%% (%d/%d)",
		p.label, bar, pct*100, p.current, p.total)

	if p.isTTY {
		_, _ = fmt.Fprintf(p.writer, "\r\033[K%s", line)
	} else {
		// Non-TTY: only print at 0%, 25%, 50%, 75%, 100% thresholds
		if p.shouldPrintNonTTY() {
			_, _ = fmt.Fprintln(p.writer, line)
		}
	}
}

func (p *ProgressBar) shouldPrintNonTTY() bool {
	if p.total == 0 {
		return p.current == 0
	}
	pct := p.current * 100 / p.total
	prev := (p.current - 1) * 100 / p.total
	// Print when crossing a 25% threshold or at 0 and 100
	for _, t := range []int{0, 25, 50, 75, 100} {
		if pct >= t && prev < t {
			return true
		}
	}
	return p.current == 1 || p.current == p.total
}

func repeat(s string, n int) string {
	var b strings.Builder
	for range n {
		b.WriteString(s)
	}
	return b.String()
}
