package cliui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Confirm prompts the user with a yes/no question and returns the answer.
// defaultYes controls what happens when the user presses enter without input.
// Returns true for yes, false for no.
func Confirm(prompt string, defaultYes bool) bool {
	return ConfirmFromReader(os.Stdin, os.Stderr, prompt, defaultYes)
}

// ConfirmFromReader is the testable version of Confirm.
func ConfirmFromReader(r io.Reader, w io.Writer, prompt string, defaultYes bool) bool {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	_, _ = fmt.Fprintf(w, "%s %s ", prompt, hint)

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return defaultYes
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch answer {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultYes
	}
}
