package cliui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Writer provides colored, structured output for CLI commands.
// It wraps an io.Writer and applies lipgloss styles when color is enabled.
type Writer struct {
	Out     io.Writer
	NoColor bool
}

// NewWriter creates a Writer that writes to stderr.
// Color is disabled when the NO_COLOR env var is set.
func NewWriter() *Writer {
	return &Writer{
		Out:     os.Stderr,
		NoColor: os.Getenv("NO_COLOR") != "",
	}
}

// NewWriterTo creates a Writer for a specific io.Writer with color disabled
// (useful for testing or piped output).
func NewWriterTo(w io.Writer) *Writer {
	return &Writer{
		Out:     w,
		NoColor: true,
	}
}

// render is a convenience shortcut.
func (w *Writer) render(s lipgloss.Style, text string) string {
	return Render(s, text, w.NoColor)
}

// Warn prints a warning prefixed with a yellow "!".
func (w *Writer) Warn(msg string) {
	_, _ = fmt.Fprintf(w.Out, "%s %s\n", w.render(StyleWarn, "!"), msg)
}

// Warnf prints a formatted warning.
func (w *Writer) Warnf(format string, args ...any) {
	w.Warn(fmt.Sprintf(format, args...))
}

// Success prints a message prefixed with a green check mark.
func (w *Writer) Success(msg string) {
	_, _ = fmt.Fprintf(w.Out, "%s %s\n", w.render(StyleSuccess, "✓"), msg)
}

// Successf prints a formatted success message.
func (w *Writer) Successf(format string, args ...any) {
	w.Success(fmt.Sprintf(format, args...))
}

// Error prints a message prefixed with a red cross.
func (w *Writer) Error(msg string) {
	_, _ = fmt.Fprintf(w.Out, "%s %s\n", w.render(StyleError, "✗"), msg)
}

// Errorf prints a formatted error message.
func (w *Writer) Errorf(format string, args ...any) {
	w.Error(fmt.Sprintf(format, args...))
}

// Info prints an undecorated informational line.
func (w *Writer) Info(msg string) {
	_, _ = fmt.Fprintln(w.Out, msg)
}

// Infof prints a formatted informational line.
func (w *Writer) Infof(format string, args ...any) {
	w.Info(fmt.Sprintf(format, args...))
}
