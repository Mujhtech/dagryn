package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

// suggestionSystemPrompt instructs the model to produce JSON matching []SuggestionOutput.
const suggestionSystemPrompt = `You are a CI/CD code suggestion assistant. Given an analysis of a build/test failure, propose inline code fixes. Respond with a JSON array of suggestion objects matching this exact schema:

[
  {
    "file_path": "path/to/file.go",
    "start_line": 42,
    "end_line": 44,
    "original_code": "the original code that needs changing",
    "suggested_code": "the corrected code",
    "explanation": "why this change fixes the issue",
    "confidence": 0.0 to 1.0
  }
]

Rules:
- file_path must be a non-empty relative path
- start_line and end_line must be positive integers with start_line <= end_line
- suggested_code must be non-empty and different from original_code
- confidence should reflect how certain you are this fix addresses the root cause
- Focus on the most impactful changes; fewer high-quality suggestions are better than many low-quality ones
- Respond ONLY with a valid JSON array, no markdown or explanation`

// ProposeSuggestions sends the analysis context to the AI provider and returns inline code suggestions.
func (p *OpenAIProvider) ProposeSuggestions(ctx context.Context, input aitypes.SuggestionInput) ([]aitypes.SuggestionOutput, error) {
	jsonObj := shared.NewResponseFormatJSONObjectParam()

	completion, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:               p.model,
		MaxCompletionTokens: param.NewOpt(int64(p.maxTokens)),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &jsonObj,
		},
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(suggestionSystemPrompt),
			openai.UserMessage(buildSuggestionUserMessage(input)),
		},
	})
	if err != nil {
		return nil, mapSDKError(ctx, err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("%w: no choices in suggestion response", aitypes.ErrInvalidResponse)
	}

	choice := completion.Choices[0]
	if choice.FinishReason == "length" {
		return nil, fmt.Errorf("%w: suggestion response truncated (max_tokens %d may be too low)", aitypes.ErrInvalidResponse, p.maxTokens)
	}

	content := choice.Message.Content
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("%w: empty suggestion response from provider", aitypes.ErrInvalidResponse)
	}

	suggestions, err := parseSuggestionOutput([]byte(content))
	if err != nil {
		return nil, err
	}

	p.logger.Debug().
		Str("model", p.model).
		Int("suggestions", len(suggestions)).
		Msg("suggestions generated")

	return suggestions, nil
}

// buildSuggestionUserMessage constructs a structured text message from the suggestion input.
func buildSuggestionUserMessage(input aitypes.SuggestionInput) string {
	var b strings.Builder

	b.WriteString("## Analysis Summary\n")
	fmt.Fprintf(&b, "- Summary: %s\n", input.Analysis.Summary)
	fmt.Fprintf(&b, "- Root Cause: %s\n", input.Analysis.RootCause)
	fmt.Fprintf(&b, "- Confidence: %.2f\n", input.Analysis.Confidence)

	if len(input.Analysis.Evidence) > 0 {
		b.WriteString("\n## Evidence\n")
		for _, e := range input.Analysis.Evidence {
			fmt.Fprintf(&b, "- Task: %s — %s", e.Task, e.Reason)
			if e.LogExcerpt != "" {
				fmt.Fprintf(&b, "\n  Log: %s", e.LogExcerpt)
			}
			b.WriteString("\n")
		}
	}

	if len(input.Analysis.LikelyFiles) > 0 {
		b.WriteString("\n## Likely Files\n")
		for _, f := range input.Analysis.LikelyFiles {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}

	if len(input.Analysis.RecommendedActions) > 0 {
		b.WriteString("\n## Recommended Actions\n")
		for _, a := range input.Analysis.RecommendedActions {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}

	if input.GitBranch != "" || input.GitCommit != "" {
		b.WriteString("\n## Git Context\n")
		if input.GitBranch != "" {
			fmt.Fprintf(&b, "- Branch: %s\n", input.GitBranch)
		}
		if input.GitCommit != "" {
			fmt.Fprintf(&b, "- Commit: %s\n", input.GitCommit)
		}
	}

	if len(input.FailedTasks) > 0 {
		b.WriteString("\n## Failed Tasks\n")
		for _, ft := range input.FailedTasks {
			fmt.Fprintf(&b, "\n### %s (exit code: %d)\n", ft.TaskName, ft.ExitCode)
			if ft.Command != "" {
				fmt.Fprintf(&b, "Command: %s\n", ft.Command)
			}
			if ft.ErrorMessage != "" {
				fmt.Fprintf(&b, "Error: %s\n", ft.ErrorMessage)
			}
			if ft.StderrTail != "" {
				fmt.Fprintf(&b, "\nStderr (tail):\n```\n%s\n```\n", ft.StderrTail)
			}
		}
	}

	return b.String()
}

// parseSuggestionOutput unmarshals and validates suggestion output.
func parseSuggestionOutput(raw []byte) ([]aitypes.SuggestionOutput, error) {
	// Try direct array parse first.
	var suggestions []aitypes.SuggestionOutput
	if err := json.Unmarshal(raw, &suggestions); err != nil {
		// Try wrapped object (OpenAI json_object mode may wrap in {"suggestions": [...]}).
		var wrapped struct {
			Suggestions []aitypes.SuggestionOutput `json:"suggestions"`
		}
		if err2 := json.Unmarshal(raw, &wrapped); err2 != nil {
			return nil, fmt.Errorf("%w: parse suggestions: %v", aitypes.ErrInvalidResponse, err)
		}
		suggestions = wrapped.Suggestions
	}

	// Validate and filter.
	var valid []aitypes.SuggestionOutput
	for _, s := range suggestions {
		if s.FilePath == "" || s.StartLine <= 0 || s.EndLine <= 0 || s.StartLine > s.EndLine {
			continue
		}
		if s.SuggestedCode == "" {
			continue
		}
		if s.Confidence < 0 {
			s.Confidence = 0
		}
		if s.Confidence > 1 {
			s.Confidence = 1
		}
		valid = append(valid, s)
	}

	return valid, nil
}
