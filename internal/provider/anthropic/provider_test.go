package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/provider"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T, sse string) *httptest.Server {
	t.Helper()
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

// ─── standard Anthropic SSE scenarios ────────────────────────────────────────

// TestAnthropicCache_FirstTurn: cache is created for the first time.
// message_start carries cache_creation_input_tokens; no cache_read yet.
func TestAnthropicCache_FirstTurn(t *testing.T) {
	sse := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":1000,\"output_tokens\":0,\"cache_creation_input_tokens\":5000,\"cache_read_input_tokens\":0}}}\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":10}}\n" +
		"data: {\"type\":\"message_stop\"}\n"

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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
	// 500/500 = 100%
	if got, want := u.CacheInfo(), "Cache: 100%"; got != want {
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

	srv := newTestServer(t, sse)
	u := mustUsage(t, chatAndCollect(t, srv))

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
