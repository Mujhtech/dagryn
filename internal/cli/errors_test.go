package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapErrorConfigNotFound(t *testing.T) {
	err := wrapError(fmt.Errorf("open dagryn.toml: no such file or directory"), nil)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Contains(t, cliErr.Message, "No dagryn.toml")
	assert.Contains(t, cliErr.Suggestion, "dagryn init")
}

func TestWrapErrorNotLoggedIn(t *testing.T) {
	err := wrapError(fmt.Errorf("not logged in"), nil)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Contains(t, cliErr.Suggestion, "dagryn auth login")
}

func TestWrapErrorNoProjectLinked(t *testing.T) {
	err := wrapError(fmt.Errorf("no project linked"), nil)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Contains(t, cliErr.Suggestion, "dagryn init --remote")
}

func TestWrapErrorDocker(t *testing.T) {
	err := wrapError(fmt.Errorf("Docker not available"), nil)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Contains(t, cliErr.Suggestion, "dagryn doctor")
}

func TestWrapErrorRemoteCacheNotEnabled(t *testing.T) {
	err := wrapError(fmt.Errorf("remote cache is not enabled in config"), nil)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Contains(t, cliErr.Suggestion, "cache.remote")
}

func TestWrapErrorTaskNotFoundWithSuggestion(t *testing.T) {
	cfg := &config.Config{
		Tasks: map[string]config.TaskConfig{
			"build": {},
			"test":  {},
			"lint":  {},
		},
	}
	err := wrapError(fmt.Errorf("task \"bild\" not found"), cfg)
	var cliErr *CLIError
	require.True(t, errors.As(err, &cliErr))
	assert.Contains(t, cliErr.Suggestion, "build")
}

func TestWrapErrorPassthrough(t *testing.T) {
	original := fmt.Errorf("some unknown error")
	err := wrapError(original, nil)
	assert.Equal(t, original, err)
}

func TestWrapErrorNil(t *testing.T) {
	assert.Nil(t, wrapError(nil, nil))
}

func TestLevenshtein(t *testing.T) {
	assert.Equal(t, 0, levenshtein("abc", "abc"))
	assert.Equal(t, 1, levenshtein("abc", "ab"))
	assert.Equal(t, 1, levenshtein("abc", "adc"))
	assert.Equal(t, 3, levenshtein("abc", "xyz"))
	assert.Equal(t, 3, levenshtein("", "abc"))
	assert.Equal(t, 3, levenshtein("abc", ""))
}
