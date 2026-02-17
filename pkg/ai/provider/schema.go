package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
)

// ValidationError contains the list of invalid fields.
type ValidationError struct {
	Fields []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: invalid fields: %s", strings.Join(e.Fields, ", "))
}

// ValidateAnalysisOutput checks that an AnalysisOutput meets schema requirements.
// It truncates Summary if over MaxSummaryLength.
func ValidateAnalysisOutput(output *aitypes.AnalysisOutput) error {
	var invalid []string

	if output.Summary == "" {
		invalid = append(invalid, "summary")
	} else if len(output.Summary) > aitypes.MaxSummaryLength {
		output.Summary = output.Summary[:aitypes.MaxSummaryLength]
	}

	if output.RootCause == "" {
		invalid = append(invalid, "root_cause")
	}

	if output.Confidence < 0 || output.Confidence > 1 {
		invalid = append(invalid, "confidence")
	}

	if output.Evidence == nil {
		output.Evidence = []aitypes.EvidenceItem{}
	}
	for i, e := range output.Evidence {
		if e.Task == "" {
			invalid = append(invalid, fmt.Sprintf("evidence[%d].task", i))
		}
	}

	if output.LikelyFiles == nil {
		output.LikelyFiles = []string{}
	}
	if output.RecommendedActions == nil {
		output.RecommendedActions = []string{}
	}

	if len(invalid) > 0 {
		return &ValidationError{Fields: invalid}
	}
	return nil
}

// ParseAnalysisOutput unmarshals raw JSON and validates the output.
func ParseAnalysisOutput(raw []byte) (*aitypes.AnalysisOutput, error) {
	var output aitypes.AnalysisOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("%w: %v", aitypes.ErrInvalidResponse, err)
	}
	if err := ValidateAnalysisOutput(&output); err != nil {
		return nil, err
	}
	return &output, nil
}
