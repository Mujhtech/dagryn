package cliui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderNoColor(t *testing.T) {
	got := Render(StyleError, "hello", true)
	assert.Equal(t, "hello", got)
}

func TestRenderWithColorPassesThrough(t *testing.T) {
	got := Render(StyleError, "hello", false)
	// The result always contains the original text.
	// In a non-TTY environment lipgloss may skip ANSI codes,
	// so we only verify the text is present.
	assert.Contains(t, got, "hello")
}
