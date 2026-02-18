package logger

import (
	"bytes"
	"testing"
	"time"

	"github.com/mujhtech/dagryn/pkg/dagryn/executor"
	"github.com/stretchr/testify/assert"
)

func TestNewWithWriterNoColor(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	assert.True(t, log.noColor)
}

func TestSuccess(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	log.Success("it worked")
	assert.Contains(t, buf.String(), "it worked")
	// No color: the raw symbol should appear
	assert.Contains(t, buf.String(), "✓")
}

func TestSuccessf(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	log.Successf("count: %d", 42)
	assert.Contains(t, buf.String(), "count: 42")
}

func TestWarn(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	log.Warn("careful")
	assert.Contains(t, buf.String(), "careful")
	assert.Contains(t, buf.String(), "!")
}

func TestWarnf(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	log.Warnf("value: %s", "x")
	assert.Contains(t, buf.String(), "value: x")
}

func TestErrorf(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	log.Errorf("bad: %d", 1)
	assert.Contains(t, buf.String(), "bad: 1")
	assert.Contains(t, buf.String(), "✗")
}

func TestInfof(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	log.Infof("val=%d", 7)
	assert.Contains(t, buf.String(), "val=7")
}

func TestSummarySuccess(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	results := []*executor.Result{
		{Status: executor.Success, Duration: 100 * time.Millisecond},
	}
	log.Summary(results, 100*time.Millisecond, 0)
	assert.Contains(t, buf.String(), "1 tasks completed")
}

func TestSummaryFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	results := []*executor.Result{
		{Status: executor.Failed, Duration: 50 * time.Millisecond},
		{Status: executor.Success, Duration: 50 * time.Millisecond},
	}
	log.Summary(results, 100*time.Millisecond, 0)
	assert.Contains(t, buf.String(), "1/2 tasks failed")
}

func TestDebugVerboseOff(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)
	log.Debug("hidden")
	assert.Empty(t, buf.String())
}

func TestDebugVerboseOn(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(true, buf)
	log.Debug("visible")
	assert.Contains(t, buf.String(), "visible")
}

func TestNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	log := New(false)
	assert.True(t, log.noColor)
}

func TestColoredStatusSymbolNoColor(t *testing.T) {
	buf := new(bytes.Buffer)
	log := NewWithWriter(false, buf)

	// With noColor=true, the symbol should be the plain text
	assert.Equal(t, "✓", log.coloredStatusSymbol(executor.Success))
	assert.Equal(t, "✗", log.coloredStatusSymbol(executor.Failed))
	assert.Equal(t, "●", log.coloredStatusSymbol(executor.Cached))
	assert.Equal(t, "○", log.coloredStatusSymbol(executor.Skipped))
}
