package factory

import (
	"testing"

	"github.com/startvibecoding/vibecoding/internal/config"
)

func TestCreateAppliesExplicitVendorDefaults(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Providers = map[string]*config.ProviderConfig{
		"custom-deepseek": {
			Vendor:  "deepseek",
			BaseURL: "https://example.com/v1",
			APIKey:  "fake-key",
			API:     "openai-chat",
			Models: []config.ModelConfig{
				{ID: "m1", Name: "M1", Reasoning: true},
			},
		},
	}
	settings.DefaultProvider = "custom-deepseek"
	settings.DefaultModel = "m1"

	p, model, err := Create(settings, "", "")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if p.Name() != "openai" {
		t.Fatalf("provider name = %q, want openai", p.Name())
	}
	if model == nil || model.ID != "m1" {
		t.Fatalf("model = %#v, want m1", model)
	}
}

func TestConvertModelConfigsPreservesCompat(t *testing.T) {
	supportsReasoningEffort := false
	models := ConvertModelConfigs("test", []config.ModelConfig{
		{
			ID:        "m1",
			Name:      "M1",
			Reasoning: true,
			Compat: &config.ModelCompat{
				ThinkingFormat:          "deepseek",
				SupportsReasoningEffort: &supportsReasoningEffort,
				MaxTokensField:          "max_completion_tokens",
			},
		},
	})
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	compat := models[0].Compat
	if compat == nil {
		t.Fatal("compat = nil")
	}
	if compat.ThinkingFormat != "deepseek" {
		t.Fatalf("ThinkingFormat = %q, want deepseek", compat.ThinkingFormat)
	}
	if compat.SupportsReasoningEffort == nil || *compat.SupportsReasoningEffort {
		t.Fatalf("SupportsReasoningEffort = %#v, want false", compat.SupportsReasoningEffort)
	}
	if compat.MaxTokensField != "max_completion_tokens" {
		t.Fatalf("MaxTokensField = %q, want max_completion_tokens", compat.MaxTokensField)
	}
}

func TestConvertModelConfigsSupportsReferenceReasoningAlias(t *testing.T) {
	models := ConvertModelConfigs("test", []config.ModelConfig{
		{
			ID:   "m1",
			Name: "M1",
			Compat: &config.ModelCompat{
				RequiresReasoningContentOnAssistantMessages: true,
			},
		},
	})
	compat := models[0].Compat
	if compat == nil || !compat.RequiresReasoningContentOnAssistant {
		t.Fatalf("RequiresReasoningContentOnAssistant = %#v, want true", compat)
	}
}
