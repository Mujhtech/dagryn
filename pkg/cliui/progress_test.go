package cliui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressBarNonTTY(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := NewProgressBar(buf, "Downloading", 4)

	pb.Increment() // 1/4 = 25%
	pb.Increment() // 2/4 = 50%
	pb.Increment() // 3/4 = 75%
	pb.Increment() // 4/4 = 100%

	out := buf.String()
	assert.Contains(t, out, "Downloading")
	assert.Contains(t, out, "4/4")
}

func TestProgressBarComplete(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := NewProgressBar(buf, "Upload", 10)

	pb.Set(5)
	pb.Complete("All done!")

	out := buf.String()
	assert.Contains(t, out, "All done!")
}

func TestProgressBarCompleteNoMessage(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := NewProgressBar(buf, "Work", 2)
	pb.Increment()
	pb.Complete("")
	// Should not panic, and should contain some output
	assert.Contains(t, buf.String(), "Work")
}

func TestProgressBarSetBounds(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := NewProgressBar(buf, "Test", 10)

	pb.Set(-5) // should clamp to 0
	pb.Set(20) // should clamp to 10
	pb.Complete("")

	out := buf.String()
	assert.Contains(t, out, "10/10")
}

func TestProgressBarDoubleComplete(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := NewProgressBar(buf, "Test", 2)
	pb.Increment()
	pb.Complete("first")
	pb.Complete("second") // should be no-op
	pb.Increment()        // should be no-op

	out := buf.String()
	assert.Contains(t, out, "first")
	assert.NotContains(t, out, "second")
}

func TestProgressBarZeroTotal(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := NewProgressBar(buf, "Empty", 0)
	pb.Complete("done")
	assert.Contains(t, buf.String(), "done")
}

func TestProgressBarSetWidth(t *testing.T) {
	buf := new(bytes.Buffer)
	pb := NewProgressBar(buf, "W", 1)
	pb.SetWidth(20)
	assert.Equal(t, 20, pb.width)
	pb.SetWidth(0) // should not change
	assert.Equal(t, 20, pb.width)
}
