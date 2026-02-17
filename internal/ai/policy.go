package ai

import (
	"fmt"
	"path/filepath"

	"github.com/mujhtech/dagryn/internal/ai/aitypes"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/rs/zerolog"
)

// PolicyChecker enforces guardrail policies on AI analysis.
type PolicyChecker struct {
	guardrails config.AIGuardrailConfig
	rateLimit  config.AIRateLimitConfig
	logger     zerolog.Logger
}

// NewPolicyChecker creates a new policy checker.
func NewPolicyChecker(guardrails config.AIGuardrailConfig, rateLimit config.AIRateLimitConfig, logger zerolog.Logger) *PolicyChecker {
	return &PolicyChecker{
		guardrails: guardrails,
		rateLimit:  rateLimit,
		logger:     logger.With().Str("component", "policy").Logger(),
	}
}

// CheckConfidence returns ErrConfidenceTooLow if the output confidence is below
// the configured minimum threshold.
func (p *PolicyChecker) CheckConfidence(output *aitypes.AnalysisOutput) error {
	if p.guardrails.MinConfidence > 0 && output.Confidence < p.guardrails.MinConfidence {
		p.logger.Debug().
			Float64("confidence", output.Confidence).
			Float64("min", p.guardrails.MinConfidence).
			Msg("confidence below threshold")
		return fmt.Errorf("%w: %.2f < %.2f", aitypes.ErrConfidenceTooLow, output.Confidence, p.guardrails.MinConfidence)
	}
	return nil
}

// CheckFilePaths filters file paths against allowed/blocked path patterns.
// Returns only paths that pass the policy.
func (p *PolicyChecker) CheckFilePaths(files []string) []string {
	if len(p.guardrails.AllowedPaths) == 0 && len(p.guardrails.BlockedPaths) == 0 {
		return files
	}

	var filtered []string
	for _, f := range files {
		if p.isBlocked(f) {
			p.logger.Debug().Str("file", f).Msg("file blocked by policy")
			continue
		}
		if len(p.guardrails.AllowedPaths) > 0 && !p.isAllowed(f) {
			p.logger.Debug().Str("file", f).Msg("file not in allowed paths")
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}

func (p *PolicyChecker) isAllowed(file string) bool {
	for _, pattern := range p.guardrails.AllowedPaths {
		if matched, _ := filepath.Match(pattern, file); matched {
			return true
		}
	}
	return false
}

func (p *PolicyChecker) isBlocked(file string) bool {
	for _, pattern := range p.guardrails.BlockedPaths {
		if matched, _ := filepath.Match(pattern, file); matched {
			return true
		}
	}
	return false
}

// ValidatePreAnalysis verifies basic preconditions before running analysis.
func (p *PolicyChecker) ValidatePreAnalysis(input *aitypes.AnalysisInput) error {
	if len(input.FailedTasks) == 0 {
		return fmt.Errorf("%w: no failed tasks to analyze", aitypes.ErrPolicyViolation)
	}
	return nil
}
