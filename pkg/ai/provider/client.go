package provider

import (
	"fmt"

	"github.com/mujhtech/dagryn/pkg/ai/aitypes"
	"github.com/rs/zerolog"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1/"
	defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai/"
)

// ManagedModels lists the models supported in managed mode, keyed by provider.
// BYOK and agent modes skip this validation — users control their own models.
var ManagedModels = map[string][]string{
	"openai": {
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		"o3-mini",
		"o4-mini",
	},
	"google": {
		"gemini-3-pro-preview",
		"gemini-3-flash-preview",
		"gemini-2.5-pro-preview",
		"gemini-2.5-flash-preview",
		"gemini-2.0-flash",
		"gemini-2.0-flash-lite",
	},
}

// IsSupportedManagedModel reports whether model is in the managed-mode allow
// list for the given provider. The provider value "gemini" is treated as an
// alias for "google". An empty provider defaults to "openai".
func IsSupportedManagedModel(providerName, model string) bool {
	key := providerName
	if key == "" {
		key = "openai"
	}
	if key == "gemini" {
		key = "google"
	}
	allowed, ok := ManagedModels[key]
	if !ok {
		return false
	}
	for _, m := range allowed {
		if model == m {
			return true
		}
	}
	return false
}

// ProviderConfig holds configuration for creating a provider.
type ProviderConfig struct {
	BackendMode    string // "managed", "byok", "agent"
	Provider       string // "openai", "google", "gemini"
	Model          string
	APIKey         string // resolved value
	MaxTokens      int
	TimeoutSeconds int
	AgentEndpoint  string
	AgentToken     string
}

// NewProvider creates the appropriate Provider based on config.
func NewProvider(cfg ProviderConfig, logger zerolog.Logger) (aitypes.Provider, error) {
	switch cfg.BackendMode {
	case "agent":
		return NewAgentAdapter(cfg, logger), nil
	case "byok", "managed":
		switch cfg.Provider {
		case "openai", "":
			return NewOpenAIProvider(cfg, logger, defaultOpenAIBaseURL), nil
		case "google", "gemini":
			if cfg.Model == "" {
				cfg.Model = ManagedModels["google"][1] // default to gemini-3-flash-preview
			}
			return NewOpenAIProvider(cfg, logger, defaultGeminiBaseURL), nil
		default:
			return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
		}
	default:
		return nil, fmt.Errorf("unsupported backend mode: %s", cfg.BackendMode)
	}
}

// NewSuggestionProvider creates the appropriate SuggestionProvider based on config.
func NewSuggestionProvider(cfg ProviderConfig, logger zerolog.Logger) (aitypes.SuggestionProvider, error) {
	switch cfg.BackendMode {
	case "agent":
		return NewAgentAdapter(cfg, logger), nil
	case "byok", "managed":
		switch cfg.Provider {
		case "openai", "":
			return NewOpenAIProvider(cfg, logger, defaultOpenAIBaseURL), nil
		case "google", "gemini":
			if cfg.Model == "" {
				cfg.Model = ManagedModels["google"][1] // default to gemini-3-flash-preview
			}
			return NewOpenAIProvider(cfg, logger, defaultGeminiBaseURL), nil
		default:
			return nil, fmt.Errorf("unsupported suggestion provider: %s", cfg.Provider)
		}
	default:
		return nil, fmt.Errorf("unsupported suggestion backend mode: %s", cfg.BackendMode)
	}
}
