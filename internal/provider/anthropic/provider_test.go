package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/provider"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func chatAndCollect(t *testing.T, p *Provider, params provider.ChatParams) []provider.StreamEvent {
	t.Helper()
	var events []provider.StreamEvent
	for e := range p.Chat(context.Background(), params) {
		events = append(events, e)
	}
	return events
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newMockAnthropicProvider(t *testing.T, models []*provider.Model, sse string, bodyCh chan<- string, check func(*http.Request)) *Provider {
	t.Helper()
	p := NewProviderWithModels("fake-key", "https://api.anthropic.com", models)
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if check != nil {
			check(r)
		}
		if bodyCh != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			bodyCh <- string(body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(sse)),
			Request:    r,
		}, nil
	})}
	return p
}

func TestAnthropicProviderHTTPProxy(t *testing.T) {
	p, err := NewProviderWithModelsAndProxy("fake-key", "https://api.anthropic.com", "http://127.0.0.1:7890", []*provider.Model{{ID: "m1"}})
	if err != nil {
		t.Fatalf("provider with proxy: %v", err)
	}
	transport, ok := p.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", p.client.Transport)
	}
	proxyURL, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.anthropic.com"}})
	if err != nil {
		t.Fatalf("proxy lookup: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("proxy = %v, want http://127.0.0.1:7890", proxyURL)
	}
}

func TestAnthropicCustomHeaders(t *testing.T) {
	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "claude-test"}}, "data: {\"type\":\"message_stop\"}\n", nil, func(r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Fatalf("X-Custom-Header = %q, want custom-value", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("anthropic-version") != "2024-01-01" {
			t.Fatalf("anthropic-version = %q, want 2024-01-01", r.Header.Get("anthropic-version"))
		}
	})
	p.SetHeaders(map[string]string{
		"X-Custom-Header":   "custom-value",
		"anthropic-version": "2024-01-01",
	})

	params := provider.ChatParams{
		ModelID:  "claude-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}
}

func mustUsage(t *testing.T, events []provider.StreamEvent) *provider.Usage {
	t.Helper()
	for _, e := range events {
		if e.Type == provider.StreamUsage && e.Usage != nil {
			return e.Usage
		}
	}
	t.Fatal("no StreamUsage event received")
	return nil
}

func boolPtr(v bool) *bool {
	return &v
}

// ─── standard Anthropic SSE scenarios ────────────────────────────────────────

func TestConvertMessagesPreservesCacheControlOnSingleTextBlock(t *testing.T) {
	p := NewProvider("fake-key", "https://api.anthropic.com")
	p.SetCacheControlEnabled(boolPtr(true))
	msgs := p.convertMessages(provider.ChatParams{
		Messages: []provider.Message{
			{
				Role: "user",
				Contents: []provider.ContentBlock{
					{
						Type:         "text",
						Text:         "cached text",
						CacheControl: &provider.CacheControl{Type: "ephemeral"},
					},
				},
			},
		},
	})

	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	blocks, ok := msgs[0].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatalf("content type = %T, want []anthropicContentBlock", msgs[0].Content)
	}
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(blocks))
	}
	if blocks[0].CacheControl == nil || blocks[0].CacheControl.Type != "ephemeral" {
		t.Fatalf("cache_control = %#v, want ephemeral", blocks[0].CacheControl)
	}
}

func TestConvertMessagesOmitsCacheControlWhenDisabled(t *testing.T) {
	p := NewProvider("fake-key", "https://api.anthropic.com")
	p.SetCacheControlEnabled(boolPtr(false))
	msgs := p.convertMessages(provider.ChatParams{
		Messages: []provider.Message{
			{
				Role: "user",
				Contents: []provider.ContentBlock{
					{
						Type:         "text",
						Text:         "cached text",
						CacheControl: &provider.CacheControl{Type: "ephemeral"},
					},
				},
			},
		},
	})

	if got, ok := msgs[0].Content.(string); !ok || got != "cached text" {
		t.Fatalf("content = %#v (%T), want simple text", msgs[0].Content, msgs[0].Content)
	}
}

func TestChatRequestPreservesCacheControlOnSingleTextBlock(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "claude-test"}}, "data: {\"type\":\"message_stop\"}\n", bodyCh, nil)
	p.SetCacheControlEnabled(boolPtr(true))
	params := provider.ChatParams{
		ModelID: "claude-test",
		Messages: []provider.Message{
			{
				Role: "user",
				Contents: []provider.ContentBlock{
					{
						Type:         "text",
						Text:         "cached text",
						CacheControl: &provider.CacheControl{Type: "ephemeral"},
					},
				},
			},
		},
		Abort: make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req anthropicRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}

	if len(req.Messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(req.Messages))
	}
	rawContent, err := json.Marshal(req.Messages[0].Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	var blocks []anthropicContentBlock
	if err := json.Unmarshal(rawContent, &blocks); err != nil {
		t.Fatalf("content is not a block array: %v\ncontent: %s", err, rawContent)
	}
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(blocks))
	}
	if blocks[0].CacheControl == nil || blocks[0].CacheControl.Type != "ephemeral" {
		t.Fatalf("cache_control = %#v, want ephemeral", blocks[0].CacheControl)
	}
}

func TestChatRequestHostedWebSearchTool(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "claude-test"}}, "data: {\"type\":\"message_stop\"}\n", bodyCh, nil)
	params := provider.ChatParams{
		ModelID: "claude-test",
		Messages: []provider.Message{
			provider.NewUserMessage("search the web"),
		},
		Tools: []provider.ToolDefinition{
			{Name: "web_search", Kind: "hosted", Provider: "anthropic", ProviderType: "messages"},
		},
		Abort: make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req anthropicRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if len(req.Tools) != 1 {
		t.Fatalf("len(tools) = %d, want 1", len(req.Tools))
	}
	if req.Tools[0].Type != "web_search_20250305" {
		t.Fatalf("tool.type = %q, want web_search_20250305", req.Tools[0].Type)
	}
	if req.Tools[0].Name != "" {
		t.Fatalf("hosted tool should not include name: %#v", req.Tools[0])
	}
}

func TestConvertMessagesAnthropicToolResultEmptyContentFallback(t *testing.T) {
	p := NewProvider("fake-key", "https://api.anthropic.com")
	msgs := p.convertMessages(provider.ChatParams{
		Messages: []provider.Message{
			provider.NewToolResultMessage("toolu_1", "bash", "", false),
		},
	})

	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Fatalf("role = %q, want user", msgs[0].Role)
	}
	blocks, ok := msgs[0].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatalf("content type = %T, want []anthropicContentBlock", msgs[0].Content)
	}
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(blocks))
	}
	if blocks[0].Type != "tool_result" {
		t.Fatalf("block type = %q, want tool_result", blocks[0].Type)
	}
	if blocks[0].ToolUseID != "toolu_1" {
		t.Fatalf("tool_use_id = %q, want toolu_1", blocks[0].ToolUseID)
	}
	if blocks[0].Content != "Tool completed with no output." {
		t.Fatalf("content = %#v, want fallback text", blocks[0].Content)
	}
}

func TestConvertMessagesAnthropicGroupsConsecutiveToolResults(t *testing.T) {
	p := NewProvider("fake-key", "https://api.anthropic.com")
	msgs := p.convertMessages(provider.ChatParams{
		Messages: []provider.Message{
			provider.NewToolResultMessage("toolu_1", "read", "first", false),
			provider.NewToolResultMessageWithContents("toolu_2", "screenshot", "image result", []provider.ContentBlock{
				{Type: "text", Text: "second"},
				{Type: "image", Image: &provider.ImageContent{MimeType: "image/png", Data: "abc123"}},
			}, false),
			provider.NewAssistantMessage([]provider.ContentBlock{{Type: "text", Text: "done"}}),
		},
	})

	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Fatalf("role = %q, want user", msgs[0].Role)
	}
	blocks, ok := msgs[0].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatalf("content type = %T, want []anthropicContentBlock", msgs[0].Content)
	}
	if len(blocks) != 3 {
		t.Fatalf("len(blocks) = %d, want 3", len(blocks))
	}
	if blocks[0].Type != "tool_result" || blocks[0].ToolUseID != "toolu_1" || blocks[0].Content != "first" {
		t.Fatalf("first block = %#v, want first tool_result", blocks[0])
	}
	if blocks[1].Type != "tool_result" || blocks[1].ToolUseID != "toolu_2" || blocks[1].Content != "second" {
		t.Fatalf("second block = %#v, want second tool_result", blocks[1])
	}
	if blocks[2].Type != "image" || blocks[2].Source == nil || blocks[2].Source.Data != "abc123" {
		t.Fatalf("third block = %#v, want image block after tool results", blocks[2])
	}
}

func TestAnthropicThinkingFormatDeepSeek(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockAnthropicProvider(t, []*provider.Model{
		{ID: "deepseek-test", Reasoning: true},
	}, "data: {\"type\":\"message_stop\"}\n", bodyCh, nil)
	p.SetThinkingFormat("deepseek")
	params := provider.ChatParams{
		ModelID:       "deepseek-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingXHigh,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req anthropicRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}

	if req.Thinking == nil || req.Thinking.Type != "enabled" || req.Thinking.BudgetTokens != nil {
		t.Fatalf("thinking = %#v, want enabled without budget_tokens", req.Thinking)
	}
	if req.OutputConfig == nil || req.OutputConfig.Effort != "max" {
		t.Fatalf("output_config = %#v, want effort max", req.OutputConfig)
	}
}

func TestAnthropicThinkingOmittedForNonReasoningModel(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockAnthropicProvider(t, []*provider.Model{
		{ID: "claude-opus-test", Reasoning: false},
	}, "data: {\"type\":\"message_stop\"}\n", bodyCh, nil)
	params := provider.ChatParams{
		ModelID:       "claude-opus-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingMedium,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req anthropicRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}

	if req.Thinking != nil {
		t.Fatalf("thinking = %#v, want nil for non-reasoning model", req.Thinking)
	}
	if req.OutputConfig != nil {
		t.Fatalf("output_config = %#v, want nil for non-reasoning model", req.OutputConfig)
	}
}

func TestAnthropicThinkingAdaptiveForOpus47(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockAnthropicProvider(t, []*provider.Model{
		{ID: "claude-opus-4-7", Reasoning: true},
	}, "data: {\"type\":\"message_stop\"}\n", bodyCh, nil)
	params := provider.ChatParams{
		ModelID:       "claude-opus-4-7",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingHigh,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req anthropicRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}

	if req.Thinking == nil || req.Thinking.Type != "adaptive" || req.Thinking.BudgetTokens != nil {
		t.Fatalf("thinking = %#v, want adaptive without budget_tokens", req.Thinking)
	}
	if req.OutputConfig == nil || req.OutputConfig.Effort != "high" {
		t.Fatalf("output_config = %#v, want effort high", req.OutputConfig)
	}
}

func TestAnthropicThinkingAdaptiveFromModelCompat(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockAnthropicProvider(t, []*provider.Model{
		{ID: "custom-adaptive", Reasoning: true, Compat: &provider.ModelCompat{ForceAdaptiveThinking: true}},
	}, "data: {\"type\":\"message_stop\"}\n", bodyCh, nil)
	params := provider.ChatParams{
		ModelID:       "custom-adaptive",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingMedium,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req anthropicRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if req.Thinking == nil || req.Thinking.Type != "adaptive" {
		t.Fatalf("thinking = %#v, want adaptive", req.Thinking)
	}
	if req.OutputConfig == nil || req.OutputConfig.Effort != "medium" {
		t.Fatalf("output_config = %#v, want effort medium", req.OutputConfig)
	}
}

// TestAnthropicCache_FirstTurn: cache is created for the first time.
// message_start carries cache_creation_input_tokens; no cache_read yet.
func TestAnthropicCache_FirstTurn(t *testing.T) {
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":1000,\"output_tokens\":0,\"cache_creation_input_tokens\":5000,\"cache_read_input_tokens\":0}}}\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":10}}\n" +
		"data: {\"type\":\"message_stop\"}\n"

	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 1000 {
		t.Errorf("Input = %d, want 1000", u.Input)
	}
	if u.Output != 10 {
		t.Errorf("Output = %d, want 10", u.Output)
	}
	if u.CacheRead != 0 {
		t.Errorf("CacheRead = %d, want 0", u.CacheRead)
	}
	if u.CacheWrite != 5000 {
		t.Errorf("CacheWrite = %d, want 5000", u.CacheWrite)
	}
	if u.TotalTokens != 6010 {
		t.Errorf("TotalTokens = %d, want 6010", u.TotalTokens)
	}
	if got, want := u.CacheInfo(), "CacheWrite: 5000"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestAnthropicCache_CachedTurn: subsequent turn where the cache is hit.
// message_start carries cache_read_input_tokens; no cache_creation.
func TestAnthropicCache_CachedTurn(t *testing.T) {
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_2\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":1000,\"output_tokens\":0,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":750}}}\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"World\"}}\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":15}}\n" +
		"data: {\"type\":\"message_stop\"}\n"

	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 1000 {
		t.Errorf("Input = %d, want 1000", u.Input)
	}
	if u.Output != 15 {
		t.Errorf("Output = %d, want 15", u.Output)
	}
	if u.CacheRead != 750 {
		t.Errorf("CacheRead = %d, want 750", u.CacheRead)
	}
	if u.CacheWrite != 0 {
		t.Errorf("CacheWrite = %d, want 0", u.CacheWrite)
	}
	if u.TotalTokens != 1765 {
		t.Errorf("TotalTokens = %d, want 1765", u.TotalTokens)
	}
	if got, want := u.CacheInfo(), "Cache: 43%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestAnthropicCache_NoCache: turn with no cache activity at all.
func TestAnthropicCache_NoCache(t *testing.T) {
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_3\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":200,\"output_tokens\":0,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":0}}}\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n" +
		"data: {\"type\":\"message_stop\"}\n"

	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 200 {
		t.Errorf("Input = %d, want 200", u.Input)
	}
	if u.CacheRead != 0 {
		t.Errorf("CacheRead = %d, want 0", u.CacheRead)
	}
	if u.CacheWrite != 0 {
		t.Errorf("CacheWrite = %d, want 0", u.CacheWrite)
	}
	if u.TotalTokens != 205 {
		t.Errorf("TotalTokens = %d, want 205", u.TotalTokens)
	}
	if got, want := u.CacheInfo(), "Cache: 0%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// ─── proxy-compatibility scenarios ───────────────────────────────────────────

// TestAnthropicCache_ProxyAllUsageInMessageDelta: some proxies send the full
// usage (including input and cache tokens) in message_delta instead of
// message_start. The parser must pick up those values.
func TestAnthropicCache_ProxyAllUsageInMessageDelta(t *testing.T) {
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_4\",\"content\":[],\"stop_reason\":null}}\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hey\"}}\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":800,\"output_tokens\":20,\"cache_read_input_tokens\":600,\"cache_creation_input_tokens\":0}}\n" +
		"data: {\"type\":\"message_stop\"}\n"

	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 800 {
		t.Errorf("Input = %d, want 800", u.Input)
	}
	if u.Output != 20 {
		t.Errorf("Output = %d, want 20", u.Output)
	}
	if u.CacheRead != 600 {
		t.Errorf("CacheRead = %d, want 600", u.CacheRead)
	}
	if u.TotalTokens != 1420 {
		t.Errorf("TotalTokens = %d, want 1420", u.TotalTokens)
	}
	if got, want := u.CacheInfo(), "Cache: 43%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestAnthropicCache_ProxySplitUsage: message_start sets input+cache fields,
// message_delta adds output_tokens. Both contributions must merge correctly.
func TestAnthropicCache_ProxySplitUsage(t *testing.T) {
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_5\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":500,\"output_tokens\":0,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":500}}}\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":8}}\n" +
		"data: {\"type\":\"message_stop\"}\n"

	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 500 {
		t.Errorf("Input = %d, want 500", u.Input)
	}
	if u.Output != 8 {
		t.Errorf("Output = %d, want 8", u.Output)
	}
	if u.CacheRead != 500 {
		t.Errorf("CacheRead = %d, want 500", u.CacheRead)
	}
	if u.TotalTokens != 1008 {
		t.Errorf("TotalTokens = %d, want 1008", u.TotalTokens)
	}
	// 500/(500+500) = 50%
	if got, want := u.CacheInfo(), "Cache: 50%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestAnthropicCache_FirstWinsOnConflict: if a proxy sends usage in both
// message_start and message_delta with conflicting values, the message_start
// values (first seen) must be preserved.
func TestAnthropicCache_FirstWinsOnConflict(t *testing.T) {
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_6\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":1000,\"output_tokens\":0,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":750}}}\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Done\"}}\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":999,\"output_tokens\":12,\"cache_read_input_tokens\":800}}\n" +
		"data: {\"type\":\"message_stop\"}\n"

	p := newMockAnthropicProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	// message_start values win
	if u.Input != 1000 {
		t.Errorf("Input = %d, want 1000 (message_start wins)", u.Input)
	}
	if u.CacheRead != 750 {
		t.Errorf("CacheRead = %d, want 750 (message_start wins)", u.CacheRead)
	}
	// output_tokens was 0 in message_start, so message_delta fills it in
	if u.Output != 12 {
		t.Errorf("Output = %d, want 12 (message_delta fills zero)", u.Output)
	}
}
