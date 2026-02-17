package ai

import (
	"testing"

	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckConfidence_AboveThreshold(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{MinConfidence: 0.5}, config.AIRateLimitConfig{}, zerolog.Nop())
	out := &aitypes.AnalysisOutput{Confidence: 0.8}
	err := pc.CheckConfidence(out)
	assert.NoError(t, err)
}

func TestCheckConfidence_BelowThreshold(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{MinConfidence: 0.7}, config.AIRateLimitConfig{}, zerolog.Nop())
	out := &aitypes.AnalysisOutput{Confidence: 0.3}
	err := pc.CheckConfidence(out)
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrConfidenceTooLow)
}

func TestCheckConfidence_ZeroThreshold(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{MinConfidence: 0}, config.AIRateLimitConfig{}, zerolog.Nop())
	out := &aitypes.AnalysisOutput{Confidence: 0.1}
	err := pc.CheckConfidence(out)
	assert.NoError(t, err)
}

func TestCheckFilePaths_NoRestrictions(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{}, config.AIRateLimitConfig{}, zerolog.Nop())
	files := []string{"main.go", "internal/handler.go"}
	result := pc.CheckFilePaths(files)
	assert.Equal(t, files, result)
}

func TestCheckFilePaths_AllowedPaths(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{
		AllowedPaths: []string{"*.go"},
	}, config.AIRateLimitConfig{}, zerolog.Nop())

	files := []string{"main.go", "config.yaml", "handler.go"}
	result := pc.CheckFilePaths(files)
	assert.Equal(t, []string{"main.go", "handler.go"}, result)
}

func TestCheckFilePaths_BlockedPaths(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{
		BlockedPaths: []string{"*.secret", "*.env"},
	}, config.AIRateLimitConfig{}, zerolog.Nop())

	files := []string{"main.go", "prod.secret", "app.env", "handler.go"}
	result := pc.CheckFilePaths(files)
	assert.Equal(t, []string{"main.go", "handler.go"}, result)
}

func TestCheckFilePaths_AllowedAndBlocked(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{
		AllowedPaths: []string{"*.go"},
		BlockedPaths: []string{"*_test.go"},
	}, config.AIRateLimitConfig{}, zerolog.Nop())

	files := []string{"main.go", "main_test.go", "handler.go"}
	result := pc.CheckFilePaths(files)
	assert.Equal(t, []string{"main.go", "handler.go"}, result)
}

func TestValidatePreAnalysis_WithFailedTasks(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{}, config.AIRateLimitConfig{}, zerolog.Nop())
	input := &aitypes.AnalysisInput{
		FailedTasks: []aitypes.FailedTaskEvidence{{TaskName: "test"}},
	}
	err := pc.ValidatePreAnalysis(input)
	assert.NoError(t, err)
}

func TestValidatePreAnalysis_NoFailedTasks(t *testing.T) {
	pc := NewPolicyChecker(config.AIGuardrailConfig{}, config.AIRateLimitConfig{}, zerolog.Nop())
	input := &aitypes.AnalysisInput{
		FailedTasks: nil,
	}
	err := pc.ValidatePreAnalysis(input)
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrPolicyViolation)
}
