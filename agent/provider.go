package agent

import "context"

// Provider is the interface that all LLM provider implementations must satisfy.
// External developers implement this to integrate custom LLM backends.
type Provider interface {
	// Chat sends a chat request and returns a channel of streaming events.
	Chat(ctx context.Context, params ChatParams) <-chan StreamEvent

	// Name returns the provider's name (e.g. "openai", "anthropic").
	Name() string

	// Models returns the list of available models.
	Models() []ModelInfo

	// GetModel returns a model by ID, or nil if not found.
	GetModel(id string) *ModelInfo
}

// ChatParams holds parameters for a chat request.
type ChatParams struct {
	Messages      []Message
	Tools         []ToolDefinition
	SystemPrompt  string
	ThinkingLevel ThinkingLevel
	MaxTokens     int
	Abort         chan struct{}
}

// ThinkingLevel represents the thinking/reasoning level.
type ThinkingLevel string

const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
)

// StreamEventType identifies the type of stream event.
type StreamEventType int

const (
	StreamStart StreamEventType = iota
	StreamTextDelta
	StreamThinkDelta
	StreamToolCall
	StreamUsage
	StreamDone
	StreamError
)

// StreamEvent represents an event from the LLM stream.
type StreamEvent struct {
	Type       StreamEventType
	TextDelta  string
	ThinkDelta string
	ToolCall   *ToolCallBlock
	Usage      *Usage
	StopReason string
	Error      error
}

// ModelInfo describes a model available from a provider.
type ModelInfo struct {
	ID            string
	Name          string
	Provider      string
	Reasoning     bool
	Input         []string
	ContextWindow int
	MaxTokens     int
}

// ModelCompat defines per-model compatibility flags.
// These flags control how the provider adjusts requests/responses
// for vendor-specific differences.
// Reference: pi/packages/ai/src/models.generated.ts compat field
type ModelCompat struct {
	// Thinking/reasoning
	ThinkingFormat                      string `json:"thinkingFormat,omitempty"`          // "deepseek"|"openai"|"anthropic"|"together"|"zai"|"qwen"
	RequiresReasoningContentOnAssistant bool   `json:"requiresReasoningContentOnAssistant,omitempty"`
	ForceAdaptiveThinking               bool   `json:"forceAdaptiveThinking,omitempty"`

	// API parameter compatibility
	SupportsDeveloperRole   *bool  `json:"supportsDeveloperRole,omitempty"`   // nil = true
	SupportsStore           *bool  `json:"supportsStore,omitempty"`           // nil = true
	SupportsReasoningEffort *bool  `json:"supportsReasoningEffort,omitempty"` // nil = true
	SupportsStrictMode      *bool  `json:"supportsStrictMode,omitempty"`      // nil = true
	MaxTokensField          string `json:"maxTokensField,omitempty"`          // "max_tokens"|"max_completion_tokens"

	// Cache
	SupportsCacheControlOnTools *bool `json:"supportsCacheControlOnTools,omitempty"` // nil = true
	SupportsLongCacheRetention  *bool `json:"supportsLongCacheRetention,omitempty"`  // nil = true
	SendSessionAffinityHeaders  bool  `json:"sendSessionAffinityHeaders,omitempty"`

	// Streaming
	SupportsEagerToolInputStreaming *bool `json:"supportsEagerToolInputStreaming,omitempty"` // nil = true
}

// BoolPtr returns a pointer to the given bool value.
// Useful for setting optional bool fields in ModelCompat.
func BoolPtr(v bool) *bool {
	return &v
}

// BaseProvider provides common functionality for provider implementations.
// Embed this in your custom Provider to get Models/GetModel for free.
type BaseProvider struct {
	name   string
	models []ModelInfo
}

// NewBaseProvider creates a new BaseProvider.
func NewBaseProvider(name string, models []ModelInfo) BaseProvider {
	return BaseProvider{name: name, models: models}
}

// Name returns the provider's name.
func (p *BaseProvider) Name() string {
	return p.name
}

// Models returns the list of available models.
func (p *BaseProvider) Models() []ModelInfo {
	return p.models
}

// GetModel returns a model by ID, or nil if not found.
func (p *BaseProvider) GetModel(id string) *ModelInfo {
	for i := range p.models {
		if p.models[i].ID == id {
			return &p.models[i]
		}
	}
	return nil
}

// VendorFromBaseURL attempts to identify the vendor from a base URL.
// Returns empty string if no match.
func VendorFromBaseURL(baseURL string) string {
	vendorMap := map[string]string{
		"api.deepseek.com":          "deepseek",
		"api.xiaomimimo.com":        "xiaomi",
		"api.xiaomi.com":            "xiaomi",
		"api.moonshot.cn":           "kimi",
		"api.minimax.chat":          "minimax",
		"ark.cn-beijing.volces.com": "seed",
		"aip.baidubce.com":          "qianfan",
		"dashscope.aliyuncs.com":    "bailian",
		"ai.gitee.com":              "gitee",
		"openrouter.ai":             "openrouter",
		"api.together.xyz":          "together",
		"api.groq.com":              "groq",
		"api.fireworks.ai":          "fireworks",
	}
	for domain, vendor := range vendorMap {
		if contains(baseURL, domain) {
			return vendor
		}
	}
	return ""
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
