package provider

import (
	"strings"
	"testing"

	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validOutput() *aitypes.AnalysisOutput {
	return &aitypes.AnalysisOutput{
		Summary:    "Test failed due to nil pointer",
		RootCause:  "Missing nil check in handler",
		Confidence: 0.85,
		Evidence: []aitypes.EvidenceItem{
			{Task: "test", Reason: "panic in line 42"},
		},
		LikelyFiles:        []string{"handler.go"},
		RecommendedActions: []string{"Add nil check"},
	}
}

func TestValidateAnalysisOutput_Valid(t *testing.T) {
	out := validOutput()
	err := ValidateAnalysisOutput(out)
	assert.NoError(t, err)
}

func TestValidateAnalysisOutput_EmptySummary(t *testing.T) {
	out := validOutput()
	out.Summary = ""
	err := ValidateAnalysisOutput(out)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Fields, "summary")
}

func TestValidateAnalysisOutput_LongSummary_Truncated(t *testing.T) {
	out := validOutput()
	out.Summary = strings.Repeat("a", aitypes.MaxSummaryLength+100)
	err := ValidateAnalysisOutput(out)
	assert.NoError(t, err)
	assert.Len(t, out.Summary, aitypes.MaxSummaryLength)
}

func TestValidateAnalysisOutput_EmptyRootCause(t *testing.T) {
	out := validOutput()
	out.RootCause = ""
	err := ValidateAnalysisOutput(out)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Fields, "root_cause")
}

func TestValidateAnalysisOutput_ConfidenceOutOfRange(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
	}{
		{"negative", -0.1},
		{"over_one", 1.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := validOutput()
			out.Confidence = tt.confidence
			err := ValidateAnalysisOutput(out)
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Contains(t, ve.Fields, "confidence")
		})
	}
}

func TestValidateAnalysisOutput_NilSlicesInitialized(t *testing.T) {
	out := validOutput()
	out.Evidence = nil
	out.LikelyFiles = nil
	out.RecommendedActions = nil
	err := ValidateAnalysisOutput(out)
	assert.NoError(t, err)
	assert.NotNil(t, out.Evidence)
	assert.NotNil(t, out.LikelyFiles)
	assert.NotNil(t, out.RecommendedActions)
}

func TestValidateAnalysisOutput_EmptyTaskInEvidence(t *testing.T) {
	out := validOutput()
	out.Evidence = []aitypes.EvidenceItem{{Task: "", Reason: "reason"}}
	err := ValidateAnalysisOutput(out)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Fields, "evidence[0].task")
}

func TestValidateAnalysisOutput_MultipleErrors(t *testing.T) {
	out := &aitypes.AnalysisOutput{
		Summary:    "",
		RootCause:  "",
		Confidence: 2.0,
	}
	err := ValidateAnalysisOutput(out)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Len(t, ve.Fields, 3)
}

func TestParseAnalysisOutput_ValidJSON(t *testing.T) {
	raw := `{
		"summary": "Build failed",
		"root_cause": "Syntax error",
		"confidence": 0.9,
		"evidence": [{"task": "build", "reason": "parse error"}],
		"likely_files": ["main.go"],
		"recommended_actions": ["Fix syntax"]
	}`
	out, err := ParseAnalysisOutput([]byte(raw))
	require.NoError(t, err)
	assert.Equal(t, "Build failed", out.Summary)
	assert.Equal(t, 0.9, out.Confidence)
}

func TestParseAnalysisOutput_InvalidJSON(t *testing.T) {
	_, err := ParseAnalysisOutput([]byte("{not json"))
	require.Error(t, err)
	assert.ErrorIs(t, err, aitypes.ErrInvalidResponse)
}

func TestParseAnalysisOutput_ValidationFails(t *testing.T) {
	raw := `{"summary":"","root_cause":"","confidence":-1}`
	_, err := ParseAnalysisOutput([]byte(raw))
	require.Error(t, err)
	var ve *ValidationError
	assert.ErrorAs(t, err, &ve)
}
