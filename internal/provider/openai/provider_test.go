package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/config"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newMockOpenAIProvider(t *testing.T, models []*provider.Model, sse string, bodyCh chan<- string, check func(*http.Request)) *Provider {
	t.Helper()
	p := NewProviderWithModels("fake-key", "https://api.test/v1", models)
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

func TestOpenAIProviderHTTPProxy(t *testing.T) {
	p, err := NewProviderWithModelsAndProxy("fake-key", "https://api.test/v1", "http://127.0.0.1:7890", []*provider.Model{{ID: "m1"}})
	if err != nil {
		t.Fatalf("provider with proxy: %v", err)
	}
	transport, ok := p.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", p.client.Transport)
	}
	proxyURL, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.test"}})
	if err != nil {
		t.Fatalf("proxy lookup: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("proxy = %v, want http://127.0.0.1:7890", proxyURL)
	}
}

func TestOpenAICustomHeaders(t *testing.T) {
	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "gpt-test"}}, "data: [DONE]\n", nil, func(r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Fatalf("X-Custom-Header = %q, want custom-value", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("Authorization") != "Bearer override-key" {
			t.Fatalf("Authorization = %q, want Bearer override-key", r.Header.Get("Authorization"))
		}
	})
	p.SetHeaders(map[string]string{
		"X-Custom-Header": "custom-value",
		"Authorization":   "Bearer override-key",
	})

	params := provider.ChatParams{
		ModelID:  "gpt-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}
}

func TestOpenAIResponsesCustomHeaders(t *testing.T) {
	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "gpt-test"}}, "data: [DONE]\n", nil, func(r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q, want /v1/responses", r.URL.Path)
		}
		if r.Header.Get("X-Responses-Header") != "responses-value" {
			t.Fatalf("X-Responses-Header = %q, want responses-value", r.Header.Get("X-Responses-Header"))
		}
	})
	p.SetUseResponsesAPI(true)
	p.SetHeaders(map[string]string{"X-Responses-Header": "responses-value"})

	params := provider.ChatParams{
		ModelID:  "gpt-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}
}

func TestOpenAIThinkingFormatDeepSeekAutoDetect(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockOpenAIProvider(t, []*provider.Model{
		{ID: "deepseek-test", Reasoning: true},
	}, "data: [DONE]\n", bodyCh, nil)
	p.baseURL = p.baseURL + "/deepseek"
	params := provider.ChatParams{
		ModelID:       "deepseek-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingXHigh,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req openAIRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}

	if req.Thinking == nil || req.Thinking.Type != "enabled" {
		t.Fatalf("thinking = %#v, want enabled", req.Thinking)
	}
	if req.ReasoningEffort != "max" {
		t.Fatalf("reasoning_effort = %q, want max", req.ReasoningEffort)
	}
}

func TestOpenAIThinkingFormatFromModelCompat(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockOpenAIProvider(t, []*provider.Model{
		{ID: "compat-test", Reasoning: true, Compat: &provider.ModelCompat{ThinkingFormat: "deepseek"}},
	}, "data: [DONE]\n", bodyCh, nil)
	params := provider.ChatParams{
		ModelID:       "compat-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingHigh,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req openAIRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if req.Thinking == nil || req.Thinking.Type != "enabled" {
		t.Fatalf("thinking = %#v, want enabled", req.Thinking)
	}
	if req.ReasoningEffort != "high" {
		t.Fatalf("reasoning_effort = %q, want high", req.ReasoningEffort)
	}
}

func TestOpenAIModelCompatRequestFields(t *testing.T) {
	bodyCh := make(chan string, 1)
	supportsReasoningEffort := false
	p := newMockOpenAIProvider(t, []*provider.Model{
		{
			ID:        "compat-fields",
			Reasoning: true,
			Compat: &provider.ModelCompat{
				MaxTokensField:          "max_completion_tokens",
				SupportsReasoningEffort: &supportsReasoningEffort,
			},
		},
	}, "data: [DONE]\n", bodyCh, nil)
	params := provider.ChatParams{
		ModelID:       "compat-fields",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingHigh,
		MaxTokens:     1234,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if _, ok := raw["max_tokens"]; ok {
		t.Fatalf("max_tokens present, want max_completion_tokens only: %#v", raw)
	}
	if got := raw["max_completion_tokens"]; got != float64(1234) {
		t.Fatalf("max_completion_tokens = %#v, want 1234", got)
	}
	if _, ok := raw["reasoning_effort"]; ok {
		t.Fatalf("reasoning_effort present despite compat flag: %#v", raw)
	}
}

func TestOpenAIRequiresReasoningContentOnAssistant(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockOpenAIProvider(t, []*provider.Model{
		{
			ID: "compat-reasoning",
			Compat: &provider.ModelCompat{
				RequiresReasoningContentOnAssistant: true,
			},
		},
	}, "data: [DONE]\n", bodyCh, nil)
	params := provider.ChatParams{
		ModelID: "compat-reasoning",
		Messages: []provider.Message{
			provider.NewAssistantMessage([]provider.ContentBlock{
				{Type: "text", Text: "previous answer"},
			}),
			provider.NewUserMessage("continue"),
		},
		Abort: make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	messages, ok := raw["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Fatalf("messages = %#v, want non-empty array", raw["messages"])
	}
	assistant, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("first message = %#v, want object", messages[0])
	}
	value, ok := assistant["reasoning_content"]
	if !ok {
		t.Fatalf("reasoning_content missing from assistant message: %#v", assistant)
	}
	if value != "" {
		t.Fatalf("reasoning_content = %#v, want empty string", value)
	}
}

func TestOpenAIResponsesAPIRequest(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockOpenAIProvider(t, []*provider.Model{
		{ID: "responses-test", Reasoning: true},
	}, "data: [DONE]\n", bodyCh, func(r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q, want /v1/responses", r.URL.Path)
		}
	})
	p.SetUseResponsesAPI(true)

	params := provider.ChatParams{
		ModelID:       "responses-test",
		SystemPrompt:  "You are a helper.",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingXHigh,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if raw["model"] != "responses-test" {
		t.Fatalf("model = %#v, want responses-test", raw["model"])
	}
	if raw["instructions"] != "You are a helper." {
		t.Fatalf("instructions = %#v, want system prompt", raw["instructions"])
	}
	if raw["stream"] != true {
		t.Fatalf("stream = %#v, want true", raw["stream"])
	}
	if _, ok := raw["max_output_tokens"]; !ok {
		t.Fatalf("max_output_tokens missing: %#v", raw)
	}
	if _, ok := raw["input"].([]any); !ok {
		t.Fatalf("input = %#v, want array", raw["input"])
	}
	if _, ok := raw["reasoning"].(map[string]any); !ok {
		t.Fatalf("reasoning = %#v, want object", raw["reasoning"])
	}
	reasoning := raw["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" {
		t.Fatalf("reasoning.effort = %#v, want high", reasoning["effort"])
	}
	if reasoning["summary"] != "auto" {
		t.Fatalf("reasoning.summary = %#v, want auto", reasoning["summary"])
	}
	if raw["prompt_cache_key"] == "" {
		t.Fatalf("prompt_cache_key missing: %#v", raw)
	}
}

func TestOpenAIResponsesAPIConfigOverrides(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockOpenAIProvider(t, []*provider.Model{
		{ID: "responses-test", Reasoning: true},
	}, "data: [DONE]\n", bodyCh, nil)
	p.SetUseResponsesAPI(true)
	p.SetResponsesConfig(config.ResponsesConfig{
		ReasoningSummary:     "concise",
		PromptCacheKey:       "custom-cache-key",
		PromptCacheRetention: "24h",
	})

	params := provider.ChatParams{
		ModelID:       "responses-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingMinimal,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	reasoning, ok := raw["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("reasoning = %#v, want object", raw["reasoning"])
	}
	if reasoning["effort"] != "minimal" {
		t.Fatalf("reasoning.effort = %#v, want minimal", reasoning["effort"])
	}
	if reasoning["summary"] != "concise" {
		t.Fatalf("reasoning.summary = %#v, want concise", reasoning["summary"])
	}
	if raw["prompt_cache_key"] != "custom-cache-key" {
		t.Fatalf("prompt_cache_key = %#v, want custom-cache-key", raw["prompt_cache_key"])
	}
	if raw["prompt_cache_retention"] != "24h" {
		t.Fatalf("prompt_cache_retention = %#v, want 24h", raw["prompt_cache_retention"])
	}
}

func TestOpenAIResponsesAPIHostedWebSearchTool(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "responses-test"}}, "data: [DONE]\n", bodyCh, nil)
	p.SetUseResponsesAPI(true)

	params := provider.ChatParams{
		ModelID:  "responses-test",
		Messages: []provider.Message{provider.NewUserMessage("latest news?")},
		Tools: []provider.ToolDefinition{
			{Name: "web_search", Kind: "hosted", Provider: "gpt", ProviderType: "responses"},
		},
		Abort: make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	tools, ok := raw["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want one hosted tool", raw["tools"])
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("tool = %#v, want object", tools[0])
	}
	if tool["type"] != "web_search" {
		t.Fatalf("tool.type = %#v, want web_search", tool["type"])
	}
	if _, ok := tool["name"]; ok {
		t.Fatalf("hosted web search should not include function name: %#v", tool)
	}
}

func TestOpenAIResponsesAPIStreamToolCall(t *testing.T) {
	lines := []string{
		`{"type":"response.output_text.delta","delta":"Working"}`,
		`{"type":"response.function_call_arguments.delta","item_id":"call_1","delta":"{\"command\":"}`,
		`{"type":"response.function_call_arguments.delta","item_id":"call_1","delta":"\"echo hi\"}"}`,
		`{"type":"response.output_item.done","item":{"id":"call_1","type":"function_call","call_id":"call_1","name":"bash"}}`,
		`{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105,"input_tokens_details":{"cached_tokens":75},"output_tokens_details":{"reasoning_tokens":3}}}}`,
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("data: [DONE]\n")

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock", Reasoning: true}}, b.String(), nil, nil)
	p.SetUseResponsesAPI(true)

	params := provider.ChatParams{
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}
	var events []provider.StreamEvent
	for e := range p.Chat(context.Background(), params) {
		events = append(events, e)
	}
	if len(events) == 0 {
		t.Fatal("no events returned")
	}

	var (
		gotText  string
		gotTool  *provider.ToolCallBlock
		gotUsage *provider.Usage
		gotDone  bool
	)
	for _, e := range events {
		switch e.Type {
		case provider.StreamTextDelta:
			gotText += e.TextDelta
		case provider.StreamToolCall:
			gotTool = e.ToolCall
		case provider.StreamUsage:
			gotUsage = e.Usage
		case provider.StreamDone:
			gotDone = true
		}
	}
	if gotText != "Working" {
		t.Fatalf("text = %q, want Working", gotText)
	}
	if gotTool == nil {
		t.Fatal("missing StreamToolCall event")
	}
	if gotTool.ID != "call_1" {
		t.Fatalf("tool ID = %q, want call_1", gotTool.ID)
	}
	if gotTool.Name != "bash" {
		t.Fatalf("tool name = %q, want bash", gotTool.Name)
	}
	if string(gotTool.Arguments) != "{\"command\":\"echo hi\"}" {
		t.Fatalf("tool args = %q, want %q", string(gotTool.Arguments), "{\"command\":\"echo hi\"}")
	}
	if gotUsage == nil || gotUsage.CacheRead != 75 {
		t.Fatalf("usage = %#v, want cacheRead 75", gotUsage)
	}
	if gotUsage.Reasoning != 3 {
		t.Fatalf("usage reasoning = %d, want 3", gotUsage.Reasoning)
	}
	if !gotDone {
		t.Fatal("missing StreamDone event")
	}
}

// ─── standard OpenAI SSE scenarios ───────────────────────────────────────────

// TestOpenAICache_CacheHit: final SSE chunk carries full usage with cached tokens.
func TestOpenAICache_CacheHit(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":5,\"total_tokens\":1005,\"prompt_tokens_details\":{\"cached_tokens\":750}}}\n" +
		"data: [DONE]\n"

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 1000 {
		t.Errorf("Input = %d, want 1000", u.Input)
	}
	if u.Output != 5 {
		t.Errorf("Output = %d, want 5", u.Output)
	}
	if u.CacheRead != 750 {
		t.Errorf("CacheRead = %d, want 750", u.CacheRead)
	}
	if got, want := u.CacheInfo(), "Cache: 75%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestOpenAICache_NoCache: usage chunk present but no cached tokens.
func TestOpenAICache_NoCache(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-2\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-2\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":200,\"completion_tokens\":8,\"total_tokens\":208}}\n" +
		"data: [DONE]\n"

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 200 {
		t.Errorf("Input = %d, want 200", u.Input)
	}
	if u.CacheRead != 0 {
		t.Errorf("CacheRead = %d, want 0", u.CacheRead)
	}
	if got, want := u.CacheInfo(), "Cache: 0%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestOpenAICache_100Pct: all input tokens are cached.
func TestOpenAICache_100Pct(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-3\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Full\"},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-3\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":500,\"completion_tokens\":4,\"total_tokens\":504,\"prompt_tokens_details\":{\"cached_tokens\":500}}}\n" +
		"data: [DONE]\n"

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.CacheRead != 500 {
		t.Errorf("CacheRead = %d, want 500", u.CacheRead)
	}
	if got, want := u.CacheInfo(), "Cache: 100%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// ─── proxy-compatibility scenarios ───────────────────────────────────────────

// TestOpenAICache_ProxyFirstChunkHasUsage: some proxies send usage in an early
// chunk rather than the final one. The first-seen values must be kept.
func TestOpenAICache_ProxyFirstChunkHasUsage(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-4\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hey\"},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":800,\"completion_tokens\":3,\"total_tokens\":803,\"prompt_tokens_details\":{\"cached_tokens\":600}}}\n" +
		"data: {\"id\":\"chatcmpl-4\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n" +
		"data: [DONE]\n"

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 800 {
		t.Errorf("Input = %d, want 800", u.Input)
	}
	if u.CacheRead != 600 {
		t.Errorf("CacheRead = %d, want 600", u.CacheRead)
	}
	if got, want := u.CacheInfo(), "Cache: 75%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestOpenAICache_ProxyFirstWinsOnConflict: if two chunks carry usage with
// different values for the same field, the first chunk's value must win.
func TestOpenAICache_ProxyFirstWinsOnConflict(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-5\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"A\"},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":6,\"total_tokens\":1006,\"prompt_tokens_details\":{\"cached_tokens\":750}}}\n" +
		// Second chunk has different (wrong) values — must be ignored
		"data: {\"id\":\"chatcmpl-5\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":999,\"completion_tokens\":99,\"total_tokens\":1098,\"prompt_tokens_details\":{\"cached_tokens\":800}}}\n" +
		"data: [DONE]\n"

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 1000 {
		t.Errorf("Input = %d, want 1000 (first chunk wins)", u.Input)
	}
	if u.Output != 6 {
		t.Errorf("Output = %d, want 6 (first chunk wins)", u.Output)
	}
	if u.CacheRead != 750 {
		t.Errorf("CacheRead = %d, want 750 (first chunk wins)", u.CacheRead)
	}
	if got, want := u.CacheInfo(), "Cache: 75%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

// TestOpenAICache_ProxySplitUsage: first chunk has prompt/completion counts
// but no cache details; a later chunk fills in the cache details.
// The first-wins rule applies per-field: the later chunk's cache value fills
// the zero CacheRead.
func TestOpenAICache_ProxySplitUsage(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-6\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"B\"},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":400,\"completion_tokens\":7,\"total_tokens\":407}}\n" +
		// Second chunk has only cache details (no prompt/completion override since those are non-zero)
		"data: {\"id\":\"chatcmpl-6\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0,\"prompt_tokens_details\":{\"cached_tokens\":300}}}\n" +
		"data: [DONE]\n"

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	u := mustUsage(t, chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})}))

	if u.Input != 400 {
		t.Errorf("Input = %d, want 400 (first chunk)", u.Input)
	}
	if u.Output != 7 {
		t.Errorf("Output = %d, want 7 (first chunk)", u.Output)
	}
	if u.CacheRead != 300 {
		t.Errorf("CacheRead = %d, want 300 (second chunk fills zero)", u.CacheRead)
	}
	// 300/400 = 75%
	if got, want := u.CacheInfo(), "Cache: 75%"; got != want {
		t.Errorf("CacheInfo() = %q, want %q", got, want)
	}
}

func TestOpenAIToolCall_MissingIDGetsFallback(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-tool-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"type\":\"function\",\"function\":{\"name\":\"bash\",\"arguments\":\"{\\\"command\\\":\"}}]},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-tool-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"type\":\"function\",\"function\":{\"arguments\":\"\\\"echo hi\\\"}\"}}]},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-tool-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n" +
		"data: [DONE]\n"

	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	events := chatAndCollect(t, p, provider.ChatParams{Messages: []provider.Message{provider.NewUserMessage("hi")}, Abort: make(chan struct{})})

	var got *provider.ToolCallBlock
	for _, e := range events {
		if e.Type == provider.StreamToolCall && e.ToolCall != nil {
			got = e.ToolCall
			break
		}
	}
	if got == nil {
		t.Fatal("expected StreamToolCall event")
	}
	if got.ID != "toolcall_0" {
		t.Fatalf("ToolCall.ID = %q, want %q", got.ID, "toolcall_0")
	}
	if got.Name != "bash" {
		t.Fatalf("ToolCall.Name = %q, want %q", got.Name, "bash")
	}
	if string(got.Arguments) != "{\"command\":\"echo hi\"}" {
		t.Fatalf("ToolCall.Arguments = %q, want %q", string(got.Arguments), "{\"command\":\"echo hi\"}")
	}
}

func TestOpenAIResponsesAPICompatDisablesOptionalParams(t *testing.T) {
	bodyCh := make(chan string, 1)
	no := false
	p := newMockOpenAIProvider(t, []*provider.Model{{
		ID:        "responses-test",
		Reasoning: true,
		Compat: &provider.ModelCompat{
			SupportsPromptCacheKey:   &no,
			SupportsReasoningSummary: &no,
		},
	}}, "data: [DONE]\n", bodyCh, nil)
	p.SetUseResponsesAPI(true)

	for range p.Chat(context.Background(), provider.ChatParams{
		ModelID:       "responses-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingHigh,
		Abort:         make(chan struct{}),
	}) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if _, ok := raw["prompt_cache_key"]; ok {
		t.Fatalf("prompt_cache_key present despite compat flag: %#v", raw)
	}
	reasoning, ok := raw["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("reasoning = %#v, want object", raw["reasoning"])
	}
	if _, ok := reasoning["summary"]; ok {
		t.Fatalf("reasoning.summary present despite compat flag: %#v", reasoning)
	}
}

func TestOpenAIResponsesAPILongCacheRetentionCompat(t *testing.T) {
	bodyCh := make(chan string, 1)
	no := false
	p := newMockOpenAIProvider(t, []*provider.Model{{
		ID: "responses-test",
		Compat: &provider.ModelCompat{
			SupportsLongCacheRetention: &no,
		},
	}}, "data: [DONE]\n", bodyCh, nil)
	p.SetUseResponsesAPI(true)
	p.SetResponsesConfig(config.ResponsesConfig{PromptCacheRetention: "24h"})

	for range p.Chat(context.Background(), provider.ChatParams{
		ModelID:  "responses-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if raw["prompt_cache_key"] == "" {
		t.Fatalf("prompt_cache_key missing: %#v", raw)
	}
	if _, ok := raw["prompt_cache_retention"]; ok {
		t.Fatalf("prompt_cache_retention present despite compat flag: %#v", raw)
	}
}

func TestOpenAIResponsesAPIPromptCacheCanBeDisabled(t *testing.T) {
	bodyCh := make(chan string, 1)
	no := false
	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "responses-test", Reasoning: true}}, "data: [DONE]\n", bodyCh, nil)
	p.SetUseResponsesAPI(true)
	p.SetResponsesConfig(config.ResponsesConfig{PromptCacheEnabled: &no})

	for range p.Chat(context.Background(), provider.ChatParams{
		ModelID:       "responses-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingHigh,
		Abort:         make(chan struct{}),
	}) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if _, ok := raw["prompt_cache_key"]; ok {
		t.Fatalf("prompt_cache_key present despite disabled cache: %#v", raw)
	}
}

func TestOpenAIResponsesAPINoReasoningWhenOff(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "responses-test", Reasoning: true}}, "data: [DONE]\n", bodyCh, nil)
	p.SetUseResponsesAPI(true)

	for range p.Chat(context.Background(), provider.ChatParams{
		ModelID:       "responses-test",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		ThinkingLevel: provider.ThinkingOff,
		Abort:         make(chan struct{}),
	}) {
	}

	var raw map[string]any
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if _, ok := raw["reasoning"]; ok {
		t.Fatalf("reasoning present despite thinking off: %#v", raw)
	}
}

func TestOpenAIResponsesAPIStreamFailure(t *testing.T) {
	sse := "data: {\"type\":\"response.failed\",\"error\":{\"message\":\"bad request\"}}\n"
	p := newMockOpenAIProvider(t, []*provider.Model{{ID: "mock"}}, sse, nil, nil)
	p.SetUseResponsesAPI(true)

	events := chatAndCollect(t, p, provider.ChatParams{
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	})
	for _, e := range events {
		if e.Type == provider.StreamError {
			if e.Error == nil || !strings.Contains(e.Error.Error(), "bad request") {
				t.Fatalf("error = %v, want bad request", e.Error)
			}
			return
		}
	}
	t.Fatal("missing StreamError event")
}
