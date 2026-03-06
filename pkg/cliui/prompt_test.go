package cliui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfirmYes(t *testing.T) {
	r := strings.NewReader("y\n")
	w := new(bytes.Buffer)
	assert.True(t, ConfirmFromReader(r, w, "Delete?", false))
	assert.Contains(t, w.String(), "Delete?")
	assert.Contains(t, w.String(), "[y/N]")
}

func TestConfirmNo(t *testing.T) {
	r := strings.NewReader("n\n")
	w := new(bytes.Buffer)
	assert.False(t, ConfirmFromReader(r, w, "Delete?", true))
}

func TestConfirmDefaultYes(t *testing.T) {
	r := strings.NewReader("\n")
	w := new(bytes.Buffer)
	assert.True(t, ConfirmFromReader(r, w, "Continue?", true))
	assert.Contains(t, w.String(), "[Y/n]")
}

func TestConfirmDefaultNo(t *testing.T) {
	r := strings.NewReader("\n")
	w := new(bytes.Buffer)
	assert.False(t, ConfirmFromReader(r, w, "Continue?", false))
}

func TestConfirmCaseInsensitive(t *testing.T) {
	r := strings.NewReader("YES\n")
	w := new(bytes.Buffer)
	assert.True(t, ConfirmFromReader(r, w, "Ok?", false))
}

func TestConfirmEmptyInput(t *testing.T) {
	r := strings.NewReader("")
	w := new(bytes.Buffer)
	// EOF with no input falls back to default
	assert.False(t, ConfirmFromReader(r, w, "Ok?", false))
}
