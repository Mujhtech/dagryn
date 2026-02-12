package condition

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluate_EmptyExpression(t *testing.T) {
	ctx := &Context{Branch: "main"}
	result, err := Evaluate("", ctx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluate_SimpleEquality(t *testing.T) {
	ctx := &Context{Branch: "main"}

	result, err := Evaluate("branch == 'main'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	result, err = Evaluate("branch == 'develop'", ctx)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestEvaluate_SimpleInequality(t *testing.T) {
	ctx := &Context{Branch: "main"}

	result, err := Evaluate("branch != 'develop'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	result, err = Evaluate("branch != 'main'", ctx)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestEvaluate_DoubleQuotedStrings(t *testing.T) {
	ctx := &Context{Branch: "main"}

	result, err := Evaluate(`branch == "main"`, ctx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluate_AndOperator(t *testing.T) {
	ctx := &Context{Branch: "main", Event: "push"}

	result, err := Evaluate("event == 'push' && branch == 'main'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	result, err = Evaluate("event == 'push' && branch == 'develop'", ctx)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestEvaluate_OrOperator(t *testing.T) {
	ctx := &Context{Event: "push"}

	result, err := Evaluate("event == 'push' || event == 'pull_request'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	ctx.Event = "pull_request"
	result, err = Evaluate("event == 'push' || event == 'pull_request'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	ctx.Event = "api"
	result, err = Evaluate("event == 'push' || event == 'pull_request'", ctx)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestEvaluate_AndOrCombined(t *testing.T) {
	ctx := &Context{Branch: "main", Event: "push"}

	// (push && main) || (pull_request && main) — first group matches
	result, err := Evaluate("event == 'push' && branch == 'main' || event == 'pull_request' && branch == 'main'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	// (push && develop) || (pull_request && main) — neither matches
	ctx.Branch = "develop"
	ctx.Event = "api"
	result, err = Evaluate("event == 'push' && branch == 'develop' || event == 'pull_request' && branch == 'main'", ctx)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestEvaluate_AllVariables(t *testing.T) {
	ctx := &Context{
		Branch:      "main",
		Event:       "pull_request",
		EventAction: "opened",
		PRNumber:    42,
		Trigger:     "ci",
	}

	result, err := Evaluate("branch == 'main'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	result, err = Evaluate("event == 'pull_request'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	result, err = Evaluate("event_action == 'opened'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	result, err = Evaluate("pr_number == '42'", ctx)
	require.NoError(t, err)
	assert.True(t, result)

	result, err = Evaluate("trigger == 'ci'", ctx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluate_UnknownVariable(t *testing.T) {
	ctx := &Context{Branch: "main"}
	_, err := Evaluate("unknown_var == 'value'", ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown variable")
}

func TestEvaluate_InvalidExpression(t *testing.T) {
	ctx := &Context{Branch: "main"}
	_, err := Evaluate("just a string", ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid condition expression")
}

func TestEvaluate_WhitespaceHandling(t *testing.T) {
	ctx := &Context{Branch: "main"}

	result, err := Evaluate("  branch  ==  'main'  ", ctx)
	require.NoError(t, err)
	assert.True(t, result)
}
