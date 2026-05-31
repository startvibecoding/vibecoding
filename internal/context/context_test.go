package context

import (
	"strings"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/provider"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		message  provider.Message
		expected int
	}{
		{
			name: "simple text message",
			message: provider.Message{
				Role:    "user",
				Content: "Hello, world!",
			},
			expected: 4, // 13 chars / 4 = 3.25, ceil = 4
		},
		{
			name: "empty message",
			message: provider.Message{
				Role:    "user",
				Content: "",
			},
			expected: 0,
		},
		{
			name: "assistant with text content block",
			message: provider.Message{
				Role: "assistant",
				Contents: []provider.ContentBlock{
					{Type: "text", Text: "This is a test message with some content"},
				},
			},
			expected: 10, // 39 chars / 4 = 9.75, ceil = 10
		},
		{
			name: "assistant with tool call",
			message: provider.Message{
				Role: "assistant",
				Contents: []provider.ContentBlock{
					{
						Type: "toolCall",
						ToolCall: &provider.ToolCallBlock{
							Name:      "bash",
							Arguments: []byte(`{"command":"ls -la"}`),
						},
					},
				},
			},
			expected: 6, // name=4 chars, args=19 chars, total=23, ceil(23/4)=6
		},
		{
			name: "tool result message",
			message: provider.Message{
				Role:    "toolResult",
				Content: "file1.txt\nfile2.txt\nfile3.txt",
			},
			expected: 8, // 28 chars / 4 = 7, ceil = 7... actually 29 chars, ceil(29/4)=8
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokens(tt.message)
			if result != tt.expected {
				t.Errorf("EstimateTokens() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestCalculateContextTokens(t *testing.T) {
	tests := []struct {
		name     string
		usage    *provider.Usage
		expected int
	}{
		{
			name:     "nil usage",
			usage:    nil,
			expected: 0,
		},
		{
			name: "with totalTokens",
			usage: &provider.Usage{
				Input:       100,
				Output:      50,
				CacheRead:   20,
				CacheWrite:  10,
				TotalTokens: 180,
			},
			expected: 180,
		},
		{
			name: "with cache aware totalTokens",
			usage: &provider.Usage{
				Input:       100,
				Output:      50,
				CacheRead:   20,
				CacheWrite:  10,
				TotalTokens: 180,
			},
			expected: 180,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateContextTokens(tt.usage)
			if result != tt.expected {
				t.Errorf("CalculateContextTokens() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestEstimateContextTokens(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there", Usage: &provider.Usage{Input: 100, Output: 50, TotalTokens: 150}},
		{Role: "user", Content: "How are you?"},
	}

	tokens, lastUsageIndex := EstimateContextTokens(messages)
	if lastUsageIndex != 1 {
		t.Errorf("lastUsageIndex = %d, want 1", lastUsageIndex)
	}
	// 150 (from usage) + estimate of "How are you?" (12 chars / 4 = 3)
	if tokens != 153 {
		t.Errorf("tokens = %d, want 153", tokens)
	}
}

func TestShouldCompact(t *testing.T) {
	tests := []struct {
		name          string
		contextTokens int
		contextWindow int
		reserveTokens int
		expected      bool
	}{
		{
			name:          "should compact - over threshold",
			contextTokens: 190000,
			contextWindow: 200000,
			reserveTokens: 16384,
			expected:      true,
		},
		{
			name:          "should not compact - under threshold",
			contextTokens: 100000,
			contextWindow: 200000,
			reserveTokens: 16384,
			expected:      false,
		},
		{
			name:          "no context window",
			contextTokens: 100000,
			contextWindow: 0,
			reserveTokens: 16384,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldCompact(tt.contextTokens, tt.contextWindow, tt.reserveTokens)
			if result != tt.expected {
				t.Errorf("ShouldCompact() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindCutPoint(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Message 2"},
		{Role: "assistant", Content: "Response 2"},
		{Role: "user", Content: "Message 3"},
		{Role: "assistant", Content: "Response 3"},
	}

	// Try to keep only ~10 tokens (should cut near the end)
	cutPoint := FindCutPoint(messages, 0, len(messages), 10)
	if cutPoint.FirstKeptIndex < 0 || cutPoint.FirstKeptIndex >= len(messages) {
		t.Errorf("FirstKeptIndex out of range: %d", cutPoint.FirstKeptIndex)
	}
}

func TestEstimateTokensImage(t *testing.T) {
	msg := provider.Message{
		Role: "user",
		Contents: []provider.ContentBlock{
			{Type: "image", Image: &provider.ImageContent{MimeType: "image/png", Data: "base64data"}},
		},
	}
	result := EstimateTokens(msg)
	if result != 1200 { // 4800 chars / 4 = 1200
		t.Errorf("EstimateTokens(image) = %d, want 1200", result)
	}
}

func TestEstimateTokensThinking(t *testing.T) {
	msg := provider.Message{
		Role: "assistant",
		Contents: []provider.ContentBlock{
			{Type: "thinking", Thinking: "Let me think about this..."},
		},
	}
	result := EstimateTokens(msg)
	expected := (len("Let me think about this...") + 3) / 4
	if result != expected {
		t.Errorf("EstimateTokens(thinking) = %d, want %d", result, expected)
	}
}

func TestEstimateTokensContentBlocksTakePrecedence(t *testing.T) {
	// When Contents is non-empty, Content should be ignored
	msg := provider.Message{
		Role:    "assistant",
		Content: "This should be ignored because Contents is set",
		Contents: []provider.ContentBlock{
			{Type: "text", Text: "Short"},
		},
	}
	result := EstimateTokens(msg)
	expected := (len("Short") + 3) / 4
	if result != expected {
		t.Errorf("EstimateTokens() = %d, want %d (should use Contents, not Content)", result, expected)
	}
}

func TestEstimateTokensToolCallNilBlock(t *testing.T) {
	msg := provider.Message{
		Role: "assistant",
		Contents: []provider.ContentBlock{
			{Type: "toolCall", ToolCall: nil},
		},
	}
	result := EstimateTokens(msg)
	if result != 0 { // 0 chars -> (0+3)/4 = 0
		t.Errorf("EstimateTokens(nil toolCall) = %d, want 0", result)
	}
}

func TestCalculateContextTokensFallback(t *testing.T) {
	// When TotalTokens is 0, should sum components
	usage := &provider.Usage{
		Input:       100,
		Output:      50,
		CacheRead:   20,
		CacheWrite:  10,
		TotalTokens: 0,
	}
	result := CalculateContextTokens(usage)
	if result != 180 {
		t.Errorf("CalculateContextTokens() = %d, want 180", result)
	}
}

func TestEstimateContextTokensNoUsage(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	tokens, lastUsageIndex := EstimateContextTokens(messages)
	if lastUsageIndex != -1 {
		t.Errorf("lastUsageIndex = %d, want -1", lastUsageIndex)
	}
	// Should estimate all messages
	expected := EstimateTokens(messages[0]) + EstimateTokens(messages[1])
	if tokens != expected {
		t.Errorf("tokens = %d, want %d", tokens, expected)
	}
}

func TestEstimateContextTokensEmptyMessages(t *testing.T) {
	tokens, lastUsageIndex := EstimateContextTokens(nil)
	if tokens != 0 {
		t.Errorf("tokens = %d, want 0", tokens)
	}
	if lastUsageIndex != -1 {
		t.Errorf("lastUsageIndex = %d, want -1", lastUsageIndex)
	}
}

func TestEstimateContextTokensUsageWithZeroTotal(t *testing.T) {
	// Usage present but TotalTokens=0 → should skip and estimate manually
	messages := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi", Usage: &provider.Usage{TotalTokens: 0}},
	}
	_, lastUsageIndex := EstimateContextTokens(messages)
	// Usage TotalTokens=0 means we skip it
	if lastUsageIndex != -1 {
		t.Errorf("lastUsageIndex = %d, want -1 (zero TotalTokens should be skipped)", lastUsageIndex)
	}
}

func TestFindValidCutPoints(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "resp1"},
		{Role: "toolResult", Content: "result1"},
		{Role: "user", Content: "msg2"},
		{Role: "assistant", Content: "resp2"},
	}

	cuts := FindValidCutPoints(messages, 0, len(messages))
	// Should include indices 0,1,3,4 but NOT 2 (toolResult)
	expected := []int{0, 1, 3, 4}
	if len(cuts) != len(expected) {
		t.Fatalf("FindValidCutPoints() = %v, want %v", cuts, expected)
	}
	for i, c := range cuts {
		if c != expected[i] {
			t.Errorf("cuts[%d] = %d, want %d", i, c, expected[i])
		}
	}
}

func TestFindValidCutPointsSubrange(t *testing.T) {
	messages := []provider.Message{
		{Role: "user"},
		{Role: "assistant"},
		{Role: "user"},
		{Role: "assistant"},
	}

	cuts := FindValidCutPoints(messages, 1, 3)
	expected := []int{1, 2}
	if len(cuts) != len(expected) {
		t.Fatalf("FindValidCutPoints(1,3) = %v, want %v", cuts, expected)
	}
}

func TestFindValidCutPointsEmpty(t *testing.T) {
	cuts := FindValidCutPoints(nil, 0, 0)
	if len(cuts) != 0 {
		t.Errorf("FindValidCutPoints(nil) = %v, want empty", cuts)
	}
}

func TestFindTurnStartIndex(t *testing.T) {
	messages := []provider.Message{
		{Role: "user"},
		{Role: "assistant"},
		{Role: "toolResult"},
		{Role: "assistant"},
	}

	// From index 3, should find user at index 0
	idx := FindTurnStartIndex(messages, 3, 0)
	if idx != 0 {
		t.Errorf("FindTurnStartIndex(3) = %d, want 0", idx)
	}

	// From index 1, should find user at index 0
	idx = FindTurnStartIndex(messages, 1, 0)
	if idx != 0 {
		t.Errorf("FindTurnStartIndex(1) = %d, want 0", idx)
	}

	// No user message found
	noUserMsgs := []provider.Message{
		{Role: "assistant"},
		{Role: "toolResult"},
	}
	idx = FindTurnStartIndex(noUserMsgs, 1, 0)
	if idx != -1 {
		t.Errorf("FindTurnStartIndex(no user) = %d, want -1", idx)
	}
}

func TestFindCutPointNoCutPoints(t *testing.T) {
	// All toolResult messages → no valid cut points
	messages := []provider.Message{
		{Role: "toolResult", Content: "result1"},
		{Role: "toolResult", Content: "result2"},
	}

	result := FindCutPoint(messages, 0, len(messages), 10)
	if result.FirstKeptIndex != 0 {
		t.Errorf("FirstKeptIndex = %d, want 0", result.FirstKeptIndex)
	}
	if result.TurnStartIndex != -1 {
		t.Errorf("TurnStartIndex = %d, want -1", result.TurnStartIndex)
	}
}

func TestFindCutPointSplitTurn(t *testing.T) {
	// Create messages where cut lands on an assistant message (not user)
	messages := []provider.Message{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
		{Role: "assistant", Content: strings.Repeat("x", 200)}, // large
		{Role: "user", Content: "third question"},
		{Role: "assistant", Content: strings.Repeat("y", 200)}, // large
	}

	// keepRecentTokens small enough to trigger cut in the middle
	result := FindCutPoint(messages, 0, len(messages), 20)
	if result.FirstKeptIndex < 0 || result.FirstKeptIndex >= len(messages) {
		t.Errorf("FirstKeptIndex = %d, out of range", result.FirstKeptIndex)
	}
}

func TestFindCutPointKeepAll(t *testing.T) {
	// keepRecentTokens very large → keep all messages
	messages := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	result := FindCutPoint(messages, 0, len(messages), 999999)
	if result.FirstKeptIndex != 0 {
		t.Errorf("FirstKeptIndex = %d, want 0 (should keep all)", result.FirstKeptIndex)
	}
}

func TestSerializeConversation(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	result := SerializeConversation(messages)
	if result == "" {
		t.Error("SerializeConversation() returned empty string")
	}
	if !contains(result, "User: Hello") {
		t.Error("SerializeConversation() missing user message")
	}
	if !contains(result, "Assistant: Hi there") {
		t.Error("SerializeConversation() missing assistant message")
	}
}

func TestSerializeConversationToolResult(t *testing.T) {
	messages := []provider.Message{
		{Role: "toolResult", ToolName: "bash", Content: "output here"},
	}

	result := SerializeConversation(messages)
	if !contains(result, "Tool Result [bash]") {
		t.Error("SerializeConversation() missing tool result")
	}
	if !contains(result, "output here") {
		t.Error("SerializeConversation() missing tool output")
	}
}

func TestSerializeConversationThinking(t *testing.T) {
	messages := []provider.Message{
		{Role: "assistant", Contents: []provider.ContentBlock{
			{Type: "thinking", Thinking: "hmm let me think"},
			{Type: "text", Text: "Here is my answer"},
		}},
	}

	result := SerializeConversation(messages)
	if !contains(result, "[thinking: hmm let me think]") {
		t.Error("SerializeConversation() missing thinking block")
	}
	if !contains(result, "Here is my answer") {
		t.Error("SerializeConversation() missing text content")
	}
}

func TestSerializeConversationToolCall(t *testing.T) {
	messages := []provider.Message{
		{Role: "assistant", Contents: []provider.ContentBlock{
			{Type: "toolCall", ToolCall: &provider.ToolCallBlock{Name: "read", Arguments: []byte(`{"path":"foo.go"}`)}},
		}},
	}

	result := SerializeConversation(messages)
	if !contains(result, "[tool_call: read(") {
		t.Errorf("SerializeConversation() missing tool call, got: %s", result)
	}
}

func TestSerializeConversationSystemInjectedSkipped(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "Hello", SystemInjected: true},
		{Role: "user", Content: "World"},
	}

	result := SerializeConversation(messages)
	if contains(result, "Hello") {
		t.Error("SerializeConversation() should skip system injected messages")
	}
	if !contains(result, "World") {
		t.Error("SerializeConversation() should include normal messages")
	}
}

func TestSerializeConversationUserContentBlocks(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Contents: []provider.ContentBlock{
			{Type: "text", Text: "block content"},
		}},
	}

	result := SerializeConversation(messages)
	if !contains(result, "User: block content") {
		t.Errorf("SerializeConversation() missing user content block, got: %s", result)
	}
}

func TestSerializeConversationLongToolResult(t *testing.T) {
	longContent := strings.Repeat("x", 600)
	messages := []provider.Message{
		{Role: "toolResult", ToolName: "bash", Content: longContent},
	}

	result := SerializeConversation(messages)
	// Should be truncated to 500 chars + "..."
	if !contains(result, "...") {
		t.Error("SerializeConversation() should truncate long tool results")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"toolong", 4, "tool..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestDefaultCompactionSettings(t *testing.T) {
	s := DefaultCompactionSettings()
	if !s.Enabled {
		t.Error("expected Enabled=true")
	}
	if s.ReserveTokens != 16384 {
		t.Errorf("ReserveTokens = %d, want 16384", s.ReserveTokens)
	}
	if s.KeepRecentTokens != 20000 {
		t.Errorf("KeepRecentTokens = %d, want 20000", s.KeepRecentTokens)
	}
	if s.IdleCompressionEnabled {
		t.Error("expected IdleCompressionEnabled=false")
	}
	if s.IdleTimeoutSeconds != 90 {
		t.Errorf("IdleTimeoutSeconds = %d, want 90", s.IdleTimeoutSeconds)
	}
	if s.IdleMinTokensForCompress != 150000 {
		t.Errorf("IdleMinTokensForCompress = %d, want 150000", s.IdleMinTokensForCompress)
	}
}

func TestShouldCompactExact(t *testing.T) {
	// Exactly at threshold
	if ShouldCompact(183616, 200000, 16384) {
		t.Error("exactly at threshold should NOT compact")
	}
	// One token over
	if !ShouldCompact(183617, 200000, 16384) {
		t.Error("one over threshold should compact")
	}
}

func TestAbsHelper(t *testing.T) {
	if abs(-5) != 5 {
		t.Errorf("abs(-5) = %d, want 5", abs(-5))
	}
	if abs(5) != 5 {
		t.Errorf("abs(5) = %d, want 5", abs(5))
	}
	if abs(0) != 0 {
		t.Errorf("abs(0) = %d, want 0", abs(0))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
