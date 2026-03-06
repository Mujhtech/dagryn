package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/mujhtech/dagryn/pkg/dagryn/plugin"
	"github.com/rs/zerolog"
)

// Logger provides structured logging for Devflow.
type Logger struct {
	zlog    zerolog.Logger
	verbose bool
	writer  io.Writer
	noColor bool
}

// New creates a new logger.
func New(verbose bool) *Logger {
	noColor := os.Getenv("NO_COLOR") != ""

	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "",
		NoColor:    noColor,
	}

	level := zerolog.InfoLevel
	if verbose {
		level = zerolog.DebugLevel
	}

	return &Logger{
		zlog:    zerolog.New(output).Level(level).With().Timestamp().Logger(),
		verbose: verbose,
		writer:  os.Stderr,
		noColor: noColor,
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
		noColor: true,
	}
}

// render applies a cliui style if color is enabled.
func (l *Logger) render(style cliui.Style, s string) string {
	return cliui.Render(style, s, l.noColor)
}

// Warn prints a warning message in yellow.
func (l *Logger) Warn(msg string) {
	_, _ = fmt.Fprintf(l.writer, "%s %s\n", l.render(cliui.StyleWarn, "!"), msg)
}

// Warnf prints a formatted warning message in yellow.
func (l *Logger) Warnf(format string, args ...any) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Success prints a success message in green.
func (l *Logger) Success(msg string) {
	_, _ = fmt.Fprintf(l.writer, "%s %s\n", l.render(cliui.StyleSuccess, "✓"), msg)
}

// Successf prints a formatted success message in green.
func (l *Logger) Successf(format string, args ...any) {
	l.Success(fmt.Sprintf(format, args...))
}

// Infof prints a formatted info message.
func (l *Logger) Infof(format string, args ...any) {
	l.Info(fmt.Sprintf(format, args...))
}

// Errorf prints a formatted error message in red.
func (l *Logger) Errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.writer, "%s %s\n", l.render(cliui.StyleError, "✗"), fmt.Sprintf(format, args...))
}

// TaskStart logs the start of a task.
func (l *Logger) TaskStart(name string) {
	_, _ = fmt.Fprintf(l.writer, "● %s\n", name)
}

// TaskEnd logs the completion of a task.
func (l *Logger) TaskEnd(name string, result *executor.Result, cacheHit bool) {
	status := l.coloredStatusSymbol(result.Status)
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

	_ = skipped // used in the count above

	totalTasks := len(results)
	cacheInfo := ""
	if cacheHits > 0 {
		cacheInfo = fmt.Sprintf(" (%d cached)", cacheHits)
	}

	if failed > 0 {
		msg := fmt.Sprintf("✗ %d/%d tasks failed in %s%s",
			failed, totalTasks, formatDuration(total), cacheInfo)
		_, _ = fmt.Fprintln(l.writer, l.render(cliui.StyleError, msg))
	} else {
		msg := fmt.Sprintf("✓ %d tasks completed in %s%s",
			succeeded, formatDuration(total), cacheInfo)
		_, _ = fmt.Fprintln(l.writer, l.render(cliui.StyleSuccess, msg))
	}
}

// Error logs an error message.
func (l *Logger) Error(msg string, err error) {
	_, _ = fmt.Fprintf(l.writer, "%s %s: %v\n", l.render(cliui.StyleError, "Error:"), msg, err)
}

// Info logs an info message.
func (l *Logger) Info(msg string) {
	_, _ = fmt.Fprintf(l.writer, "%s\n", msg)
}

// Debug logs a debug message (only in verbose mode).
func (l *Logger) Debug(msg string) {
	if l.verbose {
		_, _ = fmt.Fprintf(l.writer, "%s %s\n", l.render(cliui.StyleDim, "[DEBUG]"), msg)
	}
}

// Verbose returns whether verbose mode is enabled.
func (l *Logger) Verbose() bool {
	return l.verbose
}

// coloredStatusSymbol returns a colored symbol for a task status.
func (l *Logger) coloredStatusSymbol(status executor.Status) string {
	sym := statusSymbol(status)
	switch status {
	case executor.Success:
		return l.render(cliui.StyleSuccess, sym)
	case executor.Cached:
		return l.render(cliui.StyleSuccess, sym)
	case executor.Failed, executor.TimedOut:
		return l.render(cliui.StyleError, sym)
	case executor.Skipped, executor.Cancelled:
		return l.render(cliui.StyleDim, sym)
	default:
		return sym
	}
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
		_, _ = fmt.Fprintf(l.writer, "    %s Installed %s %s\n", l.render(cliui.StyleSuccess, "✓"), result.Plugin.Name, result.Plugin.ResolvedVersion)
	case plugin.StatusCached:
		_, _ = fmt.Fprintf(l.writer, "    %s %s [CACHED]\n", l.render(cliui.StyleSuccess, "✓"), result.Plugin.Name)
	case plugin.StatusFailed:
		_, _ = fmt.Fprintf(l.writer, "    %s Failed to install %s: %v\n", l.render(cliui.StyleError, "✗"), spec, result.Error)
	}
}
