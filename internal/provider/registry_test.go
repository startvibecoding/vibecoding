package provider

import (
	"testing"

	"github.com/startvibecoding/vibecoding/internal/config"
)

func TestProviderRegistryRegisterAndCreate(t *testing.T) {
	r := NewProviderRegistry()

	r.Register("test", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("test", []*Model{
			{ID: "m1", Name: "Model 1"},
		}, nil), nil
	})

	if !r.Has("test") {
		t.Error("expected 'test' to be registered")
	}
	if r.Has("nonexistent") {
		t.Error("expected 'nonexistent' to not be registered")
	}

	p, err := r.Create("test", &config.ProviderConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "test" {
		t.Errorf("expected 'test', got %q", p.Name())
	}
}

func TestProviderRegistryCreateNotFound(t *testing.T) {
	r := NewProviderRegistry()
	_, err := r.Create("nonexistent", &config.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProviderRegistryList(t *testing.T) {
	r := NewProviderRegistry()
	r.Register("a", func(cfg *config.ProviderConfig) (Provider, error) { return nil, nil })
	r.Register("b", func(cfg *config.ProviderConfig) (Provider, error) { return nil, nil })

	names := r.List()
	if len(names) != 2 {
		t.Errorf("expected 2, got %d", len(names))
	}
}

func TestVendorFromBaseURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://api.deepseek.com", "deepseek"},
		{"https://api.deepseek.com/anthropic", "deepseek"},
		{"https://api.xiaomimimo.com/v1", "xiaomi"},
		{"https://api.moonshot.cn/v1", "kimi"},
		{"https://api.minimax.chat/v1", "minimax"},
		{"https://ark.cn-beijing.volces.com/api", "seed"},
		{"https://aip.baidubce.com/rpc", "qianfan"},
		{"https://dashscope.aliyuncs.com/api", "bailian"},
		{"https://ai.gitee.com/v1", "gitee"},
		{"https://openrouter.ai/api/v1", "openrouter"},
		{"https://api.together.xyz/v1", "together"},
		{"https://api.groq.com/openai", "groq"},
		{"https://api.fireworks.ai/inference", "fireworks"},
		{"https://unknown.example.com/v1", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := VendorFromBaseURL(tt.url)
		if got != tt.expected {
			t.Errorf("VendorFromBaseURL(%q) = %q, want %q", tt.url, got, tt.expected)
		}
	}
}

func TestResolveProviderExplicitVendor(t *testing.T) {
	r := NewProviderRegistry()
	r.Register("myvendor", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("myvendor", nil, nil), nil
	})
	orig := globalRegistry
	globalRegistry = r
	defer func() { globalRegistry = orig }()

	p, err := ResolveProvider(&config.ProviderConfig{
		Vendor: "myvendor",
		API:    "openai-chat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "myvendor" {
		t.Errorf("expected 'myvendor', got %q", p.Name())
	}
}

func TestResolveProviderAutoDetect(t *testing.T) {
	r := NewProviderRegistry()
	r.Register("deepseek", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("deepseek", nil, nil), nil
	})
	r.Register("openai_compatible", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("openai_compatible", nil, nil), nil
	})
	r.Register("anthropic_compatible", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("anthropic_compatible", nil, nil), nil
	})
	orig := globalRegistry
	globalRegistry = r
	defer func() { globalRegistry = orig }()

	p, err := ResolveProvider(&config.ProviderConfig{
		BaseURL: "https://api.deepseek.com",
		API:     "openai-chat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "deepseek" {
		t.Errorf("expected 'deepseek', got %q", p.Name())
	}
}

func TestResolveProviderFallback(t *testing.T) {
	r := NewProviderRegistry()
	r.Register("openai_compatible", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("openai_compatible", nil, nil), nil
	})
	r.Register("anthropic_compatible", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("anthropic_compatible", nil, nil), nil
	})
	orig := globalRegistry
	globalRegistry = r
	defer func() { globalRegistry = orig }()

	p, err := ResolveProvider(&config.ProviderConfig{
		BaseURL: "https://unknown.example.com/v1",
		API:     "openai-chat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai_compatible" {
		t.Errorf("expected 'openai_compatible', got %q", p.Name())
	}

	p, err = ResolveProvider(&config.ProviderConfig{
		BaseURL: "https://unknown.example.com/v1",
		API:     "anthropic-messages",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic_compatible" {
		t.Errorf("expected 'anthropic_compatible', got %q", p.Name())
	}
}

func TestGlobalRegistry(t *testing.T) {
	Register("global_test", func(cfg *config.ProviderConfig) (Provider, error) {
		return NewMockProvider("global_test", nil, nil), nil
	})

	names := ListProviders()
	found := false
	for _, n := range names {
		if n == "global_test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'global_test' in list")
	}

	p, err := CreateProvider("global_test", &config.ProviderConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "global_test" {
		t.Errorf("expected 'global_test', got %q", p.Name())
	}
}
