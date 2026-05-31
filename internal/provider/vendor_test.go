package provider

import (
	"testing"

	"github.com/startvibecoding/vibecoding/internal/config"
)

func TestResolveAdapterConfigExplicitVendor(t *testing.T) {
	resolved := ResolveAdapterConfig(&config.ProviderConfig{
		Vendor:  "deepseek",
		BaseURL: "https://example.com/v1",
		API:     "openai-chat",
	})
	if resolved.Vendor != "deepseek" {
		t.Fatalf("Vendor = %q, want deepseek", resolved.Vendor)
	}
	if resolved.ThinkingFormat != "deepseek" {
		t.Fatalf("ThinkingFormat = %q, want deepseek", resolved.ThinkingFormat)
	}
}

func TestResolveAdapterConfigExplicitVendorDefaultAPI(t *testing.T) {
	resolved := ResolveAdapterConfig(&config.ProviderConfig{
		Vendor: "Anthropic",
	})
	if resolved.Vendor != "anthropic" {
		t.Fatalf("Vendor = %q, want anthropic", resolved.Vendor)
	}
	if resolved.API != "anthropic-messages" {
		t.Fatalf("API = %q, want anthropic-messages", resolved.API)
	}
}

func TestResolveAdapterConfigBaseURLDetect(t *testing.T) {
	resolved := ResolveAdapterConfig(&config.ProviderConfig{
		BaseURL: "https://api.deepseek.com/anthropic",
		API:     "anthropic-messages",
	})
	if resolved.Vendor != "deepseek" {
		t.Fatalf("Vendor = %q, want deepseek", resolved.Vendor)
	}
	if resolved.ThinkingFormat != "deepseek" {
		t.Fatalf("ThinkingFormat = %q, want deepseek", resolved.ThinkingFormat)
	}
}

func TestResolveAdapterConfigPreservesExplicitThinkingFormat(t *testing.T) {
	resolved := ResolveAdapterConfig(&config.ProviderConfig{
		Vendor:         "deepseek",
		BaseURL:        "https://api.deepseek.com",
		API:            "openai-chat",
		ThinkingFormat: "openai",
	})
	if resolved.ThinkingFormat != "openai" {
		t.Fatalf("ThinkingFormat = %q, want explicit openai", resolved.ThinkingFormat)
	}
}

func TestResolveAdapterConfigGenericFallback(t *testing.T) {
	resolved := ResolveAdapterConfig(&config.ProviderConfig{
		BaseURL: "https://unknown.example.com/v1",
	})
	if resolved.Vendor != "" {
		t.Fatalf("Vendor = %q, want empty", resolved.Vendor)
	}
	if resolved.API != "openai-chat" {
		t.Fatalf("API = %q, want openai-chat", resolved.API)
	}
}

func TestVendorFromBaseURLDetectsXiaomiTokenPlan(t *testing.T) {
	got := VendorFromBaseURL("https://token-plan-cn.xiaomimimo.com/v1")
	if got != "xiaomi-token-plan-cn" {
		t.Fatalf("VendorFromBaseURL = %q, want xiaomi-token-plan-cn", got)
	}
}
