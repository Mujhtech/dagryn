package cliui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriterSuccess(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Success("it worked")
	assert.Contains(t, buf.String(), "✓")
	assert.Contains(t, buf.String(), "it worked")
}

func TestWriterSuccessf(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Successf("count: %d", 42)
	assert.Contains(t, buf.String(), "count: 42")
}

func TestWriterWarn(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Warn("careful")
	assert.Contains(t, buf.String(), "!")
	assert.Contains(t, buf.String(), "careful")
}

func TestWriterWarnf(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Warnf("value: %s", "x")
	assert.Contains(t, buf.String(), "value: x")
}

func TestWriterError(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Error("bad thing")
	assert.Contains(t, buf.String(), "✗")
	assert.Contains(t, buf.String(), "bad thing")
}

func TestWriterErrorf(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Errorf("bad: %d", 1)
	assert.Contains(t, buf.String(), "bad: 1")
}

func TestWriterInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Info("hello")
	assert.Contains(t, buf.String(), "hello")
}

func TestWriterInfof(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriterTo(buf)
	w.Infof("val=%d", 7)
	assert.Contains(t, buf.String(), "val=7")
}

func TestNewWriterRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	w := NewWriter()
	assert.True(t, w.NoColor)
}
