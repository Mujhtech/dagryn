package cliui

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpinnerNonTTY(t *testing.T) {
	buf := new(bytes.Buffer)
	s := NewSpinner(buf, "loading")
	assert.False(t, s.isTTY)

	s.Start()
	assert.Contains(t, buf.String(), "loading...")

	s.Stop("done")
	assert.Contains(t, buf.String(), "done")
}

func TestSpinnerDoubleStart(t *testing.T) {
	buf := new(bytes.Buffer)
	s := NewSpinner(buf, "test")

	s.Start()
	s.Start() // should be no-op

	s.Stop("")
}

func TestSpinnerStopWithoutStart(t *testing.T) {
	buf := new(bytes.Buffer)
	s := NewSpinner(buf, "test")
	s.Stop("final")
	assert.Empty(t, buf.String())
}

func TestSpinnerStopClearsLine(t *testing.T) {
	buf := new(bytes.Buffer)
	s := NewSpinner(buf, "processing")
	s.Start()
	time.Sleep(10 * time.Millisecond)
	s.Stop("")
	// Non-TTY doesn't write ANSI codes
	assert.NotContains(t, buf.String(), "\033[K")
}
