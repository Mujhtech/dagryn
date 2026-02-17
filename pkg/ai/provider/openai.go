package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
	"github.com/rs/zerolog"
)

const defaultOpenAIMaxTokens = 2048

// OpenAIProvider implements Provider using the OpenAI chat completions API.
type OpenAIProvider struct {
	client    *openai.Client
	model     string
	maxTokens int
	logger    zerolog.Logger
}

// NewOpenAIProvider creates a new OpenAI provider with the given config.
// baseURL is the API base URL (e.g. "https://api.openai.com/v1/" for OpenAI,
// or "https://generativelanguage.googleapis.com/v1beta/openai/" for Gemini).
func NewOpenAIProvider(cfg ProviderConfig, logger zerolog.Logger, baseURL string) *OpenAIProvider {
	timeout := 45 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	maxTokens := defaultOpenAIMaxTokens
	if cfg.MaxTokens > 0 {
		maxTokens = cfg.MaxTokens
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o"
	}

	client := openai.NewClient(
		option.WithAPIKey(cfg.APIKey),
		option.WithBaseURL(baseURL),
		option.WithRequestTimeout(timeout),
		option.WithMaxRetries(0),
	)

	return &OpenAIProvider{
		client:    &client,
		model:     model,
		maxTokens: maxTokens,
		logger:    logger.With().Str("provider", cfg.Provider).Logger(),
	}
}

// AnalyzeFailure sends the evidence to the AI provider and returns the analysis.
func (p *OpenAIProvider) AnalyzeFailure(ctx context.Context, input aitypes.AnalysisInput) (*aitypes.AnalysisOutput, error) {
	jsonObj := shared.NewResponseFormatJSONObjectParam()

	completion, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:               p.model,
		MaxCompletionTokens: param.NewOpt(int64(p.maxTokens)),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &jsonObj,
		},
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(buildUserMessage(input)),
		},
	})
	if err != nil {
		return nil, mapSDKError(ctx, err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("%w: no choices in response", aitypes.ErrInvalidResponse)
	}

	choice := completion.Choices[0]
	if choice.FinishReason == "length" {
		return nil, fmt.Errorf("%w: response truncated (max_tokens %d may be too low)", aitypes.ErrInvalidResponse, p.maxTokens)
	}

	content := choice.Message.Content
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("%w: empty response from provider", aitypes.ErrInvalidResponse)
	}

	output, err := ParseAnalysisOutput([]byte(content))
	if err != nil {
		return nil, err
	}

	p.logger.Debug().
		Str("model", p.model).
		Float64("confidence", output.Confidence).
		Msg("analysis complete")

	return output, nil
}

// mapSDKError classifies an SDK error into the appropriate aitypes error.
func mapSDKError(ctx context.Context, err error) error {
	// Check parent context first (caller-side cancellation or deadline).
	if ctx.Err() != nil {
		return aitypes.ErrProviderTimeout
	}

	// The SDK's per-request timeout (WithRequestTimeout) creates its own
	// context, so ctx.Err() is nil while the underlying error is still a
	// deadline/cancellation. Detect this before falling through to the
	// generic "unavailable" bucket.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return aitypes.ErrProviderTimeout
	}

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		retryable := apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
		return &aitypes.ProviderError{
			StatusCode: apiErr.StatusCode,
			Message:    apiErr.Message,
			Retryable:  retryable,
		}
	}

	return fmt.Errorf("%w: %v", aitypes.ErrProviderUnavailable, err)
}

// buildUserMessage constructs a structured text message from the analysis input.
func buildUserMessage(input aitypes.AnalysisInput) string {
	var b strings.Builder

	b.WriteString("## Run Context\n")
	fmt.Fprintf(&b, "- Run ID: %s\n", input.RunID)
	if input.WorkflowName != "" {
		fmt.Fprintf(&b, "- Workflow: %s\n", input.WorkflowName)
	}
	fmt.Fprintf(&b, "- Tasks: %d total, %d completed, %d failed, %d cache hits\n",
		input.TotalTasks, input.CompletedTasks, input.FailedTaskCount, input.CacheHits)
	if input.DurationMs > 0 {
		fmt.Fprintf(&b, "- Duration: %dms\n", input.DurationMs)
	}
	if input.RunErrorMessage != "" {
		fmt.Fprintf(&b, "- Run Error: %s\n", input.RunErrorMessage)
	}

	if input.GitBranch != "" || input.GitCommit != "" {
		b.WriteString("\n## Git Context\n")
		if input.GitBranch != "" {
			fmt.Fprintf(&b, "- Branch: %s\n", input.GitBranch)
		}
		if input.GitCommit != "" {
			fmt.Fprintf(&b, "- Commit: %s\n", input.GitCommit)
		}
		if input.CommitMessage != "" {
			fmt.Fprintf(&b, "- Message: %s\n", input.CommitMessage)
		}
		if input.CommitAuthor != "" {
			fmt.Fprintf(&b, "- Author: %s\n", input.CommitAuthor)
		}
		if input.PRTitle != "" {
			fmt.Fprintf(&b, "- PR: #%d %s\n", input.PRNumber, input.PRTitle)
		}
	}

	if len(input.TaskGraph) > 0 {
		b.WriteString("\n## Task Graph\n")
		for _, t := range input.TaskGraph {
			fmt.Fprintf(&b, "- %s [%s]", t.Name, t.Status)
			if len(t.Needs) > 0 {
				fmt.Fprintf(&b, " (depends on: %s)", strings.Join(t.Needs, ", "))
			}
			if t.Command != "" {
				fmt.Fprintf(&b, " cmd: %s", t.Command)
			}
			b.WriteString("\n")
		}
	}

	if len(input.FailedTasks) > 0 {
		b.WriteString("\n## Failed Tasks\n")
		for _, ft := range input.FailedTasks {
			fmt.Fprintf(&b, "\n### %s (exit code: %d, duration: %dms)\n", ft.TaskName, ft.ExitCode, ft.DurationMs)
			if ft.Command != "" {
				fmt.Fprintf(&b, "Command: %s\n", ft.Command)
			}
			if ft.ErrorMessage != "" {
				fmt.Fprintf(&b, "Error: %s\n", ft.ErrorMessage)
			}
			if ft.StdoutTail != "" {
				fmt.Fprintf(&b, "\nStdout (tail):\n```\n%s\n```\n", ft.StdoutTail)
			}
			if ft.StderrTail != "" {
				fmt.Fprintf(&b, "\nStderr (tail):\n```\n%s\n```\n", ft.StderrTail)
			}
		}
	}

	return b.String()
}

// System prompt instructs the model to produce JSON matching AnalysisOutput.
const systemPrompt = `You are a CI/CD failure analysis assistant. Analyze the provided build/test failure evidence and respond with a JSON object matching this exact schema:

{
  "summary": "Brief human-readable summary of the failure (max 500 chars)",
  "root_cause": "Technical root cause explanation",
  "confidence": 0.0 to 1.0,
  "evidence": [{"task": "task_name", "log_excerpt": "relevant log line", "reason": "why this is relevant"}],
  "likely_files": ["file paths that likely need changes"],
  "recommended_actions": ["specific actionable steps to fix the issue"]
}

Rules:
- confidence should reflect how certain you are about the root cause
- evidence should reference specific tasks and log lines
- likely_files should be real file paths extracted from logs or inferred from error context
- recommended_actions should be concrete and actionable
- Respond ONLY with valid JSON, no markdown or explanation`
