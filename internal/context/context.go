package context

import (
	"github.com/startvibecoding/vibecoding/internal/provider"
)

// ContextUsage holds the current context usage information.
type ContextUsage struct {
	Tokens       int      // Current estimated context tokens
	ContextWindow int     // Maximum context window size
	Percent      *float64 // Usage percentage, nil if unknown
}

// EstimateTokens estimates token count for a message using chars/4 heuristic.
// This is conservative (overestimates tokens).
func EstimateTokens(msg provider.Message) int {
	chars := 0

	if msg.Content != "" {
		chars += len(msg.Content)
	}

	for _, block := range msg.Contents {
		switch block.Type {
		case "text":
			chars += len(block.Text)
		case "thinking":
			chars += len(block.Thinking)
		case "toolCall":
			if block.ToolCall != nil {
				chars += len(block.ToolCall.Name)
				chars += len(block.ToolCall.Arguments)
			}
		case "image":
			// Estimate images as ~4800 chars (~1200 tokens)
			chars += 4800
		}
	}

	return (chars + 3) / 4 // ceil(chars/4)
}

// CalculateContextTokens calculates total context tokens from usage.
// Uses the totalTokens field when available, falls back to computing from components.
func CalculateContextTokens(usage *provider.Usage) int {
	if usage == nil {
		return 0
	}
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	return usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
}

// EstimateContextTokens estimates context tokens from messages.
// Uses the last assistant's usage when available, then estimates trailing messages.
func EstimateContextTokens(messages []provider.Message) (tokens int, lastUsageIndex int) {
	lastUsageIndex = -1
	usageTokens := 0

	// Find last assistant message with usage
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" && msg.Usage != nil {
			totalTokens := CalculateContextTokens(msg.Usage)
			if totalTokens > 0 {
				usageTokens = totalTokens
				lastUsageIndex = i
				break
			}
		}
	}

	// If we found usage, estimate trailing messages
	if lastUsageIndex >= 0 {
		trailingTokens := 0
		for i := lastUsageIndex + 1; i < len(messages); i++ {
			trailingTokens += EstimateTokens(messages[i])
		}
		return usageTokens + trailingTokens, lastUsageIndex
	}

	// No usage data, estimate all messages
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg)
	}
	return total, -1
}

// ShouldCompact checks if compaction should trigger based on context usage.
func ShouldCompact(contextTokens int, contextWindow int, reserveTokens int) bool {
	if contextWindow <= 0 {
		return false
	}
	return contextTokens > contextWindow-reserveTokens
}
