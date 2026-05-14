package context

import (
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
			name: "without totalTokens",
			usage: &provider.Usage{
				Input:       100,
				Output:      50,
				CacheRead:   20,
				CacheWrite:  10,
				TotalTokens: 0,
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
		name           string
		contextTokens  int
		contextWindow  int
		reserveTokens  int
		expected       bool
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
