package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/provider"
)

// CompactionSettings holds compaction configuration.
type CompactionSettings struct {
	Enabled          bool `json:"enabled"`
	ReserveTokens    int  `json:"reserveTokens"`
	KeepRecentTokens int  `json:"keepRecentTokens"`

	// Idle compression settings (R5.1-R5.5)
	// When enabled, triggers compression during idle periods to maintain cache warmth.
	IdleCompressionEnabled   bool `json:"idleCompressionEnabled,omitempty"`   // R5.1: default off
	IdleTimeoutSeconds       int  `json:"idleTimeoutSeconds,omitempty"`       // seconds of inactivity before triggering (default: 90)
	IdleMinTokensForCompress int  `json:"idleMinTokensForCompress,omitempty"` // minimum tokens to trigger idle compression (default: 150000)
}

// DefaultCompactionSettings returns default compaction settings.
func DefaultCompactionSettings() CompactionSettings {
	return CompactionSettings{
		Enabled:                  true,
		ReserveTokens:            16384,
		KeepRecentTokens:         20000,
		IdleCompressionEnabled:   false, // R5.1: off by default
		IdleTimeoutSeconds:       90,    // R5.2: 90 seconds
		IdleMinTokensForCompress: 150000, // R5.4: 150k tokens minimum
	}
}

// CompactionResult holds the result of a compaction operation.
type CompactionResult struct {
	Summary        string
	FirstKeptIndex int
	TokensBefore   int
}

// CutPointResult holds information about where to cut the conversation.
type CutPointResult struct {
	FirstKeptIndex int
	TurnStartIndex int
	IsSplitTurn    bool
}

// FindValidCutPoints finds valid cut points in messages.
// Valid cut points are user, assistant messages (never tool results).
func FindValidCutPoints(messages []provider.Message, startIndex, endIndex int) []int {
	var cutPoints []int
	for i := startIndex; i < endIndex && i < len(messages); i++ {
		msg := messages[i]
		switch msg.Role {
		case "user", "assistant":
			cutPoints = append(cutPoints, i)
		case "toolResult":
			// Never cut at tool results
			continue
		}
	}
	return cutPoints
}

// FindTurnStartIndex finds the user message that starts the turn containing the given index.
func FindTurnStartIndex(messages []provider.Message, entryIndex, startIndex int) int {
	for i := entryIndex; i >= startIndex; i-- {
		if messages[i].Role == "user" {
			return i
		}
	}
	return -1
}

// FindCutPoint finds the cut point that keeps approximately keepRecentTokens.
func FindCutPoint(messages []provider.Message, startIndex, endIndex, keepRecentTokens int) CutPointResult {
	cutPoints := FindValidCutPoints(messages, startIndex, endIndex)

	if len(cutPoints) == 0 {
		return CutPointResult{FirstKeptIndex: startIndex, TurnStartIndex: -1, IsSplitTurn: false}
	}

	// Walk backwards from newest, accumulating estimated message sizes
	accumulatedTokens := 0
	cutIndex := cutPoints[0] // Default: keep from first message

	for i := endIndex - 1; i >= startIndex; i-- {
		messageTokens := EstimateTokens(messages[i])
		accumulatedTokens += messageTokens

		if accumulatedTokens >= keepRecentTokens {
			// Find the closest valid cut point at or after this entry
			for _, c := range cutPoints {
				if c >= i {
					cutIndex = c
					break
				}
			}
			break
		}
	}

	// Determine if this is a split turn
	isUserMessage := messages[cutIndex].Role == "user"
	turnStartIndex := -1
	if !isUserMessage {
		turnStartIndex = FindTurnStartIndex(messages, cutIndex, startIndex)
	}

	return CutPointResult{
		FirstKeptIndex: cutIndex,
		TurnStartIndex: turnStartIndex,
		IsSplitTurn:    !isUserMessage && turnStartIndex != -1,
	}
}

// SerializeConversation serializes messages to text for summarization.
func SerializeConversation(messages []provider.Message) string {
	var sb strings.Builder

	for _, msg := range messages {
		// Skip system-injected messages
		if msg.SystemInjected {
			continue
		}

		switch msg.Role {
		case "user":
			content := msg.Content
			if content == "" {
				for _, block := range msg.Contents {
					if block.Type == "text" {
						content += block.Text
					}
				}
			}
			sb.WriteString(fmt.Sprintf("User: %s\n\n", content))

		case "assistant":
			sb.WriteString("Assistant: ")
			if msg.Content != "" {
				sb.WriteString(msg.Content)
			}
			for _, block := range msg.Contents {
				switch block.Type {
				case "text":
					sb.WriteString(block.Text)
				case "thinking":
					sb.WriteString(fmt.Sprintf("[thinking: %s]", block.Thinking))
				case "toolCall":
					if block.ToolCall != nil {
						sb.WriteString(fmt.Sprintf("[tool_call: %s(%s)]", block.ToolCall.Name, string(block.ToolCall.Arguments)))
					}
				}
			}
			sb.WriteString("\n\n")

		case "toolResult":
			sb.WriteString(fmt.Sprintf("Tool Result [%s]: %s\n\n", msg.ToolName, truncateString(msg.Content, 500)))
		}
	}

	return sb.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// compressionInstruction is the instruction injected into the conversation for Insert-then-Compress.
// This implements Rule R4.2: the compression instruction is a system_injected message.
const compressionInstruction = `Please create a structured context checkpoint summary of our conversation so far.

Use this EXACT format:

## Goal
[What is the user trying to accomplish?]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned by user]
- Or "(none)" if none were mentioned

## Progress
### Done
- [x] [Completed tasks/changes]

### In Progress
- [ ] [Current work]

### Blocked
- [Issues preventing progress, if any]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [Ordered list of what should happen next]

## Critical Context
- [Any data, examples, or references needed to continue]
- Or "(none)" if not applicable

Keep each section concise. Preserve exact file paths, function names, and error messages.`

// updateCompressionInstruction is used when there's an existing summary to update.
const updateCompressionInstruction = `Please update the existing summary with new information from our conversation.

<existing-summary>
%s
</existing-summary>

RULES:
- PRESERVE all existing information from the previous summary
- ADD new progress, decisions, and context from the new messages
- UPDATE the Progress section: move items from "In Progress" to "Done" when completed
- UPDATE "Next Steps" based on what was accomplished
- PRESERVE exact file paths, function names, and error messages
- If something is no longer relevant, you may remove it

Use the same EXACT format as the existing summary.`

// GenerateSummaryInsertThenCompress generates a summary using Insert-then-Compress pattern.
// This implements Rule R4.1-R4.2: use the SAME system prompt and tools, not a separate call.
// The compression instruction is injected as a system_injected user message at the end of the conversation.
func GenerateSummaryInsertThenCompress(
	ctx context.Context,
	messages []provider.Message,
	p provider.Provider,
	systemPrompt string,
	tools []provider.ToolDefinition,
	previousSummary string,
	maxTokens int,
) (string, error) {
	// Build compression instruction
	var instruction string
	if previousSummary != "" {
		instruction = fmt.Sprintf(updateCompressionInstruction, previousSummary)
	} else {
		instruction = compressionInstruction
	}

	// Create the compression instruction message (system_injected)
	compressionMsg := provider.NewSystemInjectedUserMessage(instruction)

	// Build messages: original conversation + compression instruction
	// The LLM sees the full conversation and responds with a summary
	compactionMessages := make([]provider.Message, 0, len(messages)+1)
	compactionMessages = append(compactionMessages, messages...)
	compactionMessages = append(compactionMessages, compressionMsg)

	// Use the SAME system prompt and tools (R4.1: no separate LLM call with different prompt)
	params := provider.ChatParams{
		Messages:     compactionMessages,
		Tools:        tools,
		SystemPrompt: systemPrompt,
		MaxTokens:    maxTokens,
	}

	// Call LLM to generate summary
	streamCh := p.Chat(ctx, params)

	var summary strings.Builder
	for event := range streamCh {
		switch event.Type {
		case provider.StreamTextDelta:
			summary.WriteString(event.TextDelta)
		case provider.StreamError:
			if event.Error != nil {
				return "", fmt.Errorf("summarization failed: %w", event.Error)
			}
		}
	}

	result := strings.TrimSpace(summary.String())
	if result == "" {
		return "", fmt.Errorf("summarization returned empty result")
	}

	return result, nil
}

// GenerateSummary is the legacy interface that delegates to Insert-then-Compress.
// Kept for backward compatibility but now uses the same system prompt.
// Deprecated: use GenerateSummaryInsertThenCompress directly.
func GenerateSummary(
	ctx context.Context,
	messages []provider.Message,
	p provider.Provider,
	model *provider.Model,
	reserveTokens int,
	previousSummary string,
) (string, error) {
	maxTokens := int(float64(reserveTokens) * 0.8)
	if model.MaxTokens > 0 && maxTokens > model.MaxTokens {
		maxTokens = model.MaxTokens
	}

	// Use empty system prompt and tools - this is the legacy path
	// The caller should migrate to GenerateSummaryInsertThenCompress
	return GenerateSummaryInsertThenCompress(ctx, messages, p, "", nil, previousSummary, maxTokens)
}

// Compact performs context compaction on the messages using Insert-then-Compress pattern.
// This implements Rule R4.1-R4.4.
func Compact(
	ctx context.Context,
	messages []provider.Message,
	p provider.Provider,
	model *provider.Model,
	systemPrompt string,
	tools []provider.ToolDefinition,
	settings CompactionSettings,
	previousSummary string,
) (*CompactionResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to compact")
	}

	tokensBefore := 0
	for _, msg := range messages {
		tokensBefore += EstimateTokens(msg)
	}

	// Find cut point - keep recent messages, summarize older ones
	cutPoint := FindCutPoint(messages, 0, len(messages), settings.KeepRecentTokens)

	// Messages to summarize (will be discarded after summary)
	messagesToSummarize := messages[:cutPoint.FirstKeptIndex]
	if cutPoint.IsSplitTurn && cutPoint.TurnStartIndex >= 0 {
		messagesToSummarize = messages[:cutPoint.TurnStartIndex]
	}

	if len(messagesToSummarize) == 0 {
		return nil, fmt.Errorf("nothing to compact")
	}

	// Calculate max tokens for summary
	maxTokens := int(float64(settings.ReserveTokens) * 0.8)
	if model != nil && model.MaxTokens > 0 && maxTokens > model.MaxTokens {
		maxTokens = model.MaxTokens
	}

	// Generate summary using Insert-then-Compress (R4.1-R4.2)
	summary, err := GenerateSummaryInsertThenCompress(
		ctx, messagesToSummarize, p,
		systemPrompt, tools,
		previousSummary, maxTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("generate summary: %w", err)
	}

	return &CompactionResult{
		Summary:        summary,
		FirstKeptIndex: cutPoint.FirstKeptIndex,
		TokensBefore:   tokensBefore,
	}, nil
}

// CompactWithLegacyInterface is a compatibility wrapper that calls the old Compact signature.
// Deprecated: use the new Compact with systemPrompt and tools parameters.
func CompactWithLegacyInterface(
	ctx context.Context,
	messages []provider.Message,
	p provider.Provider,
	model *provider.Model,
	settings CompactionSettings,
	previousSummary string,
) (*CompactionResult, error) {
	return Compact(ctx, messages, p, model, "", nil, settings, previousSummary)
}
