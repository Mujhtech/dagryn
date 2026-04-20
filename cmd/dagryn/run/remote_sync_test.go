package run

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestAppendLogRedactsSensitiveValues(t *testing.T) {
	t.Setenv("TEST_SECRET_KEY", "supersecretvalue")

	rs := &RemoteSync{
		RunID:        uuid.New(),
		logBuffer:    make([]client.LogEntry, 0, 10),
		taskLineNums: map[string]int{},
	}

	rs.AppendLog("build", "stdout", "token=supersecretvalue")
	require.Len(t, rs.logBuffer, 1)
	require.NotContains(t, rs.logBuffer[0].Line, "supersecretvalue")
	require.Contains(t, rs.logBuffer[0].Line, "[REDACTED]")
}
