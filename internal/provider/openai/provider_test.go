package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/provider"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T, sse string) *httptest.Server {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if strings.Contains(fmt.Sprint(r), "httptest: failed to listen on a port") {
				t.Skipf("local httptest listener unavailable: %v", r)
			}
			panic(r)
		}
	}()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sse))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func chatAndCollect(t *testing.T, srv *httptest.Server) []provider.StreamEvent {
	t.Helper()
	p := NewProvider("fake-key", srv.URL)
	params := provider.ChatParams{
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}
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

func TestOpenAIThinkingFormatDeepSeekAutoDetect(t *testing.T) {
	bodyCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		bodyCh <- string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: [DONE]\n"))
	}))
	t.Cleanup(srv.Close)

	p := NewProviderWithModels("fake-key", srv.URL+"/deepseek", []*provider.Model{
		{ID: "deepseek-test", Reasoning: true},
	})
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		bodyCh <- string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: [DONE]\n"))
	}))
	t.Cleanup(srv.Close)

	p := NewProviderWithModels("fake-key", srv.URL, []*provider.Model{
		{ID: "compat-test", Reasoning: true, Compat: &provider.ModelCompat{ThinkingFormat: "deepseek"}},
	})
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		bodyCh <- string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: [DONE]\n"))
	}))
	t.Cleanup(srv.Close)

	supportsReasoningEffort := false
	p := NewProviderWithModels("fake-key", srv.URL, []*provider.Model{
		{
			ID:        "compat-fields",
			Reasoning: true,
			Compat: &provider.ModelCompat{
				MaxTokensField:          "max_completion_tokens",
				SupportsReasoningEffort: &supportsReasoningEffort,
			},
		},
	})
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		bodyCh <- string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: [DONE]\n"))
	}))
	t.Cleanup(srv.Close)

	p := NewProviderWithModels("fake-key", srv.URL, []*provider.Model{
		{
			ID: "compat-reasoning",
			Compat: &provider.ModelCompat{
				RequiresReasoningContentOnAssistant: true,
			},
		},
	})
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

// ─── standard OpenAI SSE scenarios ───────────────────────────────────────────

// TestOpenAICache_CacheHit: final SSE chunk carries full usage with cached tokens.
func TestOpenAICache_CacheHit(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":5,\"total_tokens\":1005,\"prompt_tokens_details\":{\"cached_tokens\":750}}}\n" +
		"data: [DONE]\n"

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	events := chatAndCollect(t, newTestServer(t, sse))

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
