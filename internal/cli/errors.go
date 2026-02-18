package cli

import (
	"fmt"
	"strings"

	"github.com/mujhtech/dagryn/pkg/dagryn/config"
)

// CLIError is a user-facing error with an optional suggestion and exit code.
type CLIError struct {
	Err        error
	Message    string
	Suggestion string
	ExitCode   int
}

func (e *CLIError) Error() string {
	return e.Message
}

func (e *CLIError) Unwrap() error {
	return e.Err
}

// wrapError converts known Go errors into user-friendly CLIErrors.
// If the error is not recognized, it is returned unchanged.
func wrapError(err error, cfg *config.Config) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	// Config file not found
	if strings.Contains(msg, "no such file or directory") && strings.Contains(msg, "dagryn.toml") ||
		strings.Contains(msg, "config file") && strings.Contains(msg, "not found") {
		return &CLIError{
			Err:        err,
			Message:    "No dagryn.toml found in the current directory.",
			Suggestion: "Run 'dagryn init' to create one.",
			ExitCode:   1,
		}
	}

	// Not logged in / credential errors
	if strings.Contains(msg, "not logged in") || strings.Contains(msg, "credentials") && strings.Contains(msg, "load") {
		return &CLIError{
			Err:        err,
			Message:    "You are not logged in.",
			Suggestion: "Run 'dagryn auth login' to authenticate.",
			ExitCode:   1,
		}
	}

	// No project linked
	if strings.Contains(msg, "no project linked") {
		return &CLIError{
			Err:        err,
			Message:    "No project is linked to this directory.",
			Suggestion: "Run 'dagryn init --remote' or 'dagryn use <project-id>' to link a project.",
			ExitCode:   1,
		}
	}

	// Docker errors
	if strings.Contains(strings.ToLower(msg), "docker") && (strings.Contains(msg, "not found") || strings.Contains(msg, "not available") || strings.Contains(msg, "Cannot connect")) {
		return &CLIError{
			Err:        err,
			Message:    "Docker is not available.",
			Suggestion: "Install Docker or run 'dagryn doctor' to diagnose.",
			ExitCode:   1,
		}
	}

	// Task not found — suggest closest match
	if strings.Contains(msg, "task") && strings.Contains(msg, "not found") {
		taskName := extractQuotedString(msg)
		suggestion := ""
		if taskName != "" && cfg != nil {
			if closest := findClosestTask(taskName, cfg); closest != "" {
				suggestion = fmt.Sprintf("Did you mean '%s'?", closest)
			}
		}
		if suggestion == "" {
			suggestion = "Run 'dagryn run' (no arguments) to see available tasks."
		}
		return &CLIError{
			Err:        err,
			Message:    msg,
			Suggestion: suggestion,
			ExitCode:   1,
		}
	}

	// Remote cache not enabled
	if strings.Contains(msg, "remote cache is not enabled") {
		return &CLIError{
			Err:        err,
			Message:    "Remote cache is not enabled in your configuration.",
			Suggestion: "Add [cache.remote] section to dagryn.toml with enabled = true.",
			ExitCode:   1,
		}
	}

	return err
}

// extractQuotedString extracts a string between quotes from a message.
func extractQuotedString(msg string) string {
	start := strings.IndexByte(msg, '"')
	if start == -1 {
		return ""
	}
	end := strings.IndexByte(msg[start+1:], '"')
	if end == -1 {
		return ""
	}
	return msg[start+1 : start+1+end]
}

// findClosestTask finds the task name in the config that is closest
// to the given name using Levenshtein distance.
func findClosestTask(name string, cfg *config.Config) string {
	if cfg == nil {
		return ""
	}

	bestDist := len(name) + 1
	bestMatch := ""

	for taskName := range cfg.Tasks {
		d := levenshtein(name, taskName)
		if d < bestDist && d <= 3 {
			bestDist = d
			bestMatch = taskName
		}
	}

	return bestMatch
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}
