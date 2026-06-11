package factory

import (
	"fmt"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/provider/anthropic"
	"github.com/startvibecoding/vibecoding/internal/provider/google"
	"github.com/startvibecoding/vibecoding/internal/provider/openai"
)

// Create creates a provider and model from settings without changing the config schema.
func Create(settings *config.Settings, providerName, modelID string) (provider.Provider, *provider.Model, error) {
	return CreateWithOptions(settings, providerName, modelID, Options{})
}

// Options controls compatibility behavior outside the settings schema.
type Options struct {
	BuiltinAnthropicCacheControl *bool
}

// CreateWithOptions creates a provider and model from settings with runtime-only options.
func CreateWithOptions(settings *config.Settings, providerName, modelID string, opts Options) (provider.Provider, *provider.Model, error) {
	if providerName == "" {
		providerName = settings.DefaultProvider
	}
	if modelID == "" {
		modelID = settings.DefaultModel
	}

	pc := settings.GetProviderConfig(providerName)
	if pc != nil {
		apiKey := settings.ResolveKey(providerName)
		models := ConvertModelConfigs(providerName, pc.Models)
		resolved := provider.ResolveAdapterConfig(pc)

		var p provider.Provider
		switch resolved.API {
		case "anthropic-messages":
			ap, err := anthropic.NewProviderWithModelsAndProxy(apiKey, resolved.BaseURL, pc.HTTPProxy, models)
			if err != nil {
				return nil, nil, err
			}
			if resolved.ThinkingFormat != "" {
				ap.SetThinkingFormat(resolved.ThinkingFormat)
			}
			if resolved.CacheControl != nil {
				ap.SetCacheControlEnabled(resolved.CacheControl)
			}
			ConfigureRetry(ap, settings)
			p = ap
		case "openai-chat", "openai", "openai-responses", "responses":
			op, err := openai.NewProviderWithModelsAndProxy(apiKey, resolved.BaseURL, pc.HTTPProxy, models)
			if err != nil {
				return nil, nil, err
			}
			if resolved.ThinkingFormat != "" {
				op.SetThinkingFormat(resolved.ThinkingFormat)
			}
			if resolved.API == "openai-responses" || resolved.API == "responses" {
				op.SetUseResponsesAPI(true)
				op.SetResponsesConfig(pc.Responses)
			}
			ConfigureRetry(op, settings)
			p = op
		case "google-gemini":
			gp, err := google.NewGeminiProviderWithModelsAndProxy(apiKey, resolved.BaseURL, pc.HTTPProxy, models)
			if err != nil {
				return nil, nil, err
			}
			ConfigureRetry(gp, settings)
			p = gp
		case "google-vertex":
			gp, err := google.NewVertexProviderWithModelsAndProxy(apiKey, resolved.BaseURL, pc.HTTPProxy, models)
			if err != nil {
				return nil, nil, err
			}
			ConfigureRetry(gp, settings)
			p = gp
		default:
			return nil, nil, fmt.Errorf("unsupported API type: %s (use 'openai-chat', 'openai-responses', 'anthropic-messages', 'google-gemini', or 'google-vertex')", resolved.API)
		}

		ConfigureHeaders(p, settings, providerName)
		model := p.GetModel(modelID)
		if model == nil {
			if len(models) > 0 {
				model = models[0]
			} else {
				return nil, nil, fmt.Errorf("no models configured for provider %s", providerName)
			}
		}
		return p, model, nil
	}

	var p provider.Provider
	switch strings.ToLower(providerName) {
	case "openai":
		p = openai.NewProvider(settings.ResolveKey(providerName), "")
	case "anthropic":
		ap := anthropic.NewProvider(settings.ResolveKey(providerName), "")
		if opts.BuiltinAnthropicCacheControl != nil {
			ap.SetCacheControlEnabled(opts.BuiltinAnthropicCacheControl)
		}
		p = ap
	case "google-gemini":
		p = google.NewGeminiProvider(settings.ResolveKey(providerName), "")
	case "google-vertex":
		p = google.NewVertexProvider(settings.ResolveKey(providerName), "")
	default:
		return nil, nil, fmt.Errorf("unknown provider: %s (add it to settings.json providers section)", providerName)
	}
	ConfigureRetry(p, settings)

	model := p.GetModel(modelID)
	if model == nil {
		models := p.Models()
		if len(models) > 0 {
			model = models[0]
		} else {
			return nil, nil, fmt.Errorf("no models available for provider %s", providerName)
		}
	}
	return p, model, nil
}

type retryConfigurable interface {
	SetRetryConfig(cfg *provider.RetryConfig)
}

type headersConfigurable interface {
	SetHeaders(headers map[string]string)
}

// ConfigureRetry sets retry config on a provider if it supports it.
func ConfigureRetry(p provider.Provider, settings *config.Settings) {
	if rc, ok := p.(retryConfigurable); ok {
		rc.SetRetryConfig(&provider.RetryConfig{
			Enabled:     settings.Retry.Enabled,
			MaxRetries:  settings.Retry.MaxRetries,
			BaseDelayMs: settings.Retry.BaseDelayMs,
		})
	}
}

// ConfigureHeaders sets custom provider headers if the provider supports it.
func ConfigureHeaders(p provider.Provider, settings *config.Settings, providerName string) {
	if hc, ok := p.(headersConfigurable); ok {
		hc.SetHeaders(settings.ResolveProviderHeaders(providerName))
	}
}

// ConvertModelConfigs converts config.ModelConfig to provider.Model.
func ConvertModelConfigs(providerName string, models []config.ModelConfig) []*provider.Model {
	result := make([]*provider.Model, 0, len(models))
	for _, m := range models {
		input := m.Input
		if len(input) == 0 {
			input = []string{"text"}
		}
		var cost provider.ModelPricing
		if m.Cost != nil {
			cost = provider.ModelPricing{
				Input:      m.Cost.Input,
				Output:     m.Cost.Output,
				CacheRead:  m.Cost.CacheRead,
				CacheWrite: m.Cost.CacheWrite,
			}
		}
		result = append(result, &provider.Model{
			ID:            m.ID,
			Name:          m.Name,
			Provider:      providerName,
			Reasoning:     m.Reasoning,
			Input:         input,
			Cost:          cost,
			ContextWindow: m.ContextWindow,
			MaxTokens:     m.MaxTokens,
			Temperature:   m.Temperature,
			TopP:          m.TopP,
			Compat:        convertCompat(m.Compat),
		})
	}
	return result
}

func convertCompat(c *config.ModelCompat) *provider.ModelCompat {
	if c == nil {
		return nil
	}
	return &provider.ModelCompat{
		ThinkingFormat:                      c.ThinkingFormat,
		RequiresReasoningContentOnAssistant: c.RequiresReasoningContentOnAssistant || c.RequiresReasoningContentOnAssistantMessages,
		ForceAdaptiveThinking:               c.ForceAdaptiveThinking,
		SupportsDeveloperRole:               cloneBoolPtr(c.SupportsDeveloperRole),
		SupportsStore:                       cloneBoolPtr(c.SupportsStore),
		SupportsReasoningEffort:             cloneBoolPtr(c.SupportsReasoningEffort),
		SupportsStrictMode:                  cloneBoolPtr(c.SupportsStrictMode),
		MaxTokensField:                      c.MaxTokensField,
		SupportsCacheControlOnTools:         cloneBoolPtr(c.SupportsCacheControlOnTools),
		SupportsLongCacheRetention:          cloneBoolPtr(c.SupportsLongCacheRetention),
		SupportsPromptCacheKey:              cloneBoolPtr(c.SupportsPromptCacheKey),
		SupportsReasoningSummary:            cloneBoolPtr(c.SupportsReasoningSummary),
		SendSessionAffinityHeaders:          c.SendSessionAffinityHeaders,
		SupportsEagerToolInputStreaming:     cloneBoolPtr(c.SupportsEagerToolInputStreaming),
	}
}

func cloneBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}
