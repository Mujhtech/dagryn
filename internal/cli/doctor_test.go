package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoctorCommandRuns(t *testing.T) {
	cmd := newDoctorCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Doctor should run without panicking.
	// It may return an error if checks fail (e.g. no config), but should not crash.
	_ = cmd.RunE(cmd, nil)

	output := buf.String()
	// The header "Dagryn Doctor" is written to stdout via cmd.OutOrStdout()
	assert.Contains(t, output, "Dagryn Doctor")
}
