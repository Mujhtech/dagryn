package agent

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectEnvironment(t *testing.T) {
	env := DetectEnvironment()

	assert.NotEmpty(t, env.Type)
	assert.Equal(t, runtime.GOOS, env.OS)
	assert.Equal(t, runtime.GOARCH, env.Arch)
	assert.NotEmpty(t, env.Hostname)
	assert.Contains(t, env.Labels, "os")
	assert.Contains(t, env.Labels, "arch")
	assert.Contains(t, env.Labels, "environment")
}
