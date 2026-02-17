package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/plugin"
	"github.com/rs/zerolog"
)

// Logger provides structured logging for Devflow.
type Logger struct {
	zlog    zerolog.Logger
	verbose bool
	writer  io.Writer
}

// New creates a new logger.
func New(verbose bool) *Logger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "",
		NoColor:    false,
	}

	level := zerolog.InfoLevel
	if verbose {
		level = zerolog.DebugLevel
	}

	return &Logger{
		zlog:    zerolog.New(output).Level(level).With().Timestamp().Logger(),
		verbose: verbose,
		writer:  os.Stderr,
	}
}

// NewWithWriter creates a new logger with a custom writer.
func NewWithWriter(verbose bool, w io.Writer) *Logger {
	output := zerolog.ConsoleWriter{
		Out:        w,
		TimeFormat: "",
		NoColor:    true,
	}

	level := zerolog.InfoLevel
	if verbose {
		level = zerolog.DebugLevel
	}

	return &Logger{
		zlog:    zerolog.New(output).Level(level).With().Timestamp().Logger(),
		verbose: verbose,
		writer:  w,
	}
}

// TaskStart logs the start of a task.
func (l *Logger) TaskStart(name string) {
	_, _ = fmt.Fprintf(l.writer, "● %s\n", name)
}

// TaskEnd logs the completion of a task.
func (l *Logger) TaskEnd(name string, result *executor.Result, cacheHit bool) {
	status := statusSymbol(result.Status)
	cacheStatus := ""
	if cacheHit {
		cacheStatus = " [CACHE HIT]"
	} else if result.Status == executor.Success {
		cacheStatus = " [CACHE MISS]"
	}

	duration := formatDuration(result.Duration)
	_, _ = fmt.Fprintf(l.writer, "%s %-12s%s%s\n", status, name, cacheStatus, duration)
}

// CacheHit logs a cache hit.
func (l *Logger) CacheHit(name string) {
	if l.verbose {
		l.zlog.Debug().Str("task", name).Msg("cache hit")
	}
}

// CacheMiss logs a cache miss.
func (l *Logger) CacheMiss(name string) {
	if l.verbose {
		l.zlog.Debug().Str("task", name).Msg("cache miss")
	}
}

// Summary prints the execution summary.
func (l *Logger) Summary(results []*executor.Result, total time.Duration, cacheHits int) {
	_, _ = fmt.Fprintln(l.writer)

	succeeded := 0
	failed := 0
	skipped := 0

	for _, r := range results {
		switch r.Status {
		case executor.Success, executor.Cached:
			succeeded++
		case executor.Failed, executor.TimedOut:
			failed++
		case executor.Skipped, executor.Cancelled:
			skipped++
		}
	}

	totalTasks := len(results)
	cacheInfo := ""
	if cacheHits > 0 {
		cacheInfo = fmt.Sprintf(" (%d cached)", cacheHits)
	}

	if failed > 0 {
		_, _ = fmt.Fprintf(l.writer, "✗ %d/%d tasks failed in %s%s\n",
			failed, totalTasks, formatDuration(total), cacheInfo)
	} else {
		_, _ = fmt.Fprintf(l.writer, "✓ %d tasks completed in %s%s\n",
			succeeded, formatDuration(total), cacheInfo)
	}
}

// Error logs an error message.
func (l *Logger) Error(msg string, err error) {
	_, _ = fmt.Fprintf(l.writer, "Error: %s: %v\n", msg, err)
}

// Info logs an info message.
func (l *Logger) Info(msg string) {
	_, _ = fmt.Fprintf(l.writer, "%s\n", msg)
}

// Debug logs a debug message (only in verbose mode).
func (l *Logger) Debug(msg string) {
	if l.verbose {
		_, _ = fmt.Fprintf(l.writer, "[DEBUG] %s\n", msg)
	}
}

// Verbose returns whether verbose mode is enabled.
func (l *Logger) Verbose() bool {
	return l.verbose
}

// statusSymbol returns the symbol for a task status.
func statusSymbol(status executor.Status) string {
	switch status {
	case executor.Success:
		return "✓"
	case executor.Failed:
		return "✗"
	case executor.Cached:
		return "●"
	case executor.Skipped:
		return "○"
	case executor.TimedOut:
		return "⏱"
	case executor.Cancelled:
		return "⊘"
	default:
		return "?"
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}

	if d < time.Second {
		return fmt.Sprintf(" %dms", d.Milliseconds())
	}
	return fmt.Sprintf(" %.2fs", d.Seconds())
}

// PrintPlan prints the execution plan.
func (l *Logger) PrintPlan(levels [][]string) {
	_, _ = fmt.Fprintln(l.writer, "Execution Plan:")
	_, _ = fmt.Fprintln(l.writer, strings.Repeat("─", 40))
	for i, level := range levels {
		_, _ = fmt.Fprintf(l.writer, "Level %d: %s\n", i, strings.Join(level, ", "))
	}
	_, _ = fmt.Fprintln(l.writer, strings.Repeat("─", 40))
}

// PluginStart logs the start of a plugin installation.
func (l *Logger) PluginStart(spec string) {
	_, _ = fmt.Fprintf(l.writer, "  ↓ %s\n", spec)
}

// PluginDone logs the completion of a plugin installation.
func (l *Logger) PluginDone(spec string, result *plugin.InstallResult) {
	if result == nil {
		return
	}

	switch result.Status {
	case plugin.StatusInstalled:
		_, _ = fmt.Fprintf(l.writer, "    ✓ Installed %s %s\n", result.Plugin.Name, result.Plugin.ResolvedVersion)
	case plugin.StatusCached:
		_, _ = fmt.Fprintf(l.writer, "    ✓ %s [CACHED]\n", result.Plugin.Name)
	case plugin.StatusFailed:
		_, _ = fmt.Fprintf(l.writer, "    ✗ Failed to install %s: %v\n", spec, result.Error)
	}
}
