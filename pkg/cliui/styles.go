package cliui

import "github.com/charmbracelet/lipgloss"

// Style is a type alias for lipgloss.Style so consumers don't need
// to import lipgloss directly.
type Style = lipgloss.Style

// Shared lipgloss styles for CLI output.
var (
	StyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	StyleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	StyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	StyleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // cyan
	StyleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // dim gray
)

// Render applies a lipgloss style when color is enabled.
// If noColor is true the original string is returned unchanged.
func Render(style lipgloss.Style, s string, noColor bool) string {
	if noColor {
		return s
	}
	return style.Render(s)
}
