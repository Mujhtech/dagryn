package doctor

import (
	"bytes"
	"testing"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/stretchr/testify/assert"
)

func TestDoctorCommandRuns(t *testing.T) {
	flags := &cli.Flags{
		Verbose: false,
		CfgFile: "dagryn.toml",
	}
	cmd := NewCmd(flags)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	_ = cmd.RunE(cmd, nil)

	output := buf.String()
	assert.Contains(t, output, "Dagryn Doctor")
}
