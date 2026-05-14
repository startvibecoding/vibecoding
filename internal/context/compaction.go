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
}

// DefaultCompactionSettings returns default compaction settings.
func DefaultCompactionSettings() CompactionSettings {
	return CompactionSettings{
		Enabled:          true,
		ReserveTokens:    16384,
		KeepRecentTokens: 20000,
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

const summarizationPrompt = `The messages above are a conversation to summarize. Create a structured context checkpoint summary that another LLM will use to continue the work.

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

const updateSummarizationPrompt = `The messages above are NEW conversation messages to incorporate into the existing summary provided in <previous-summary> tags.

Update the existing structured summary with new information. RULES:
- PRESERVE all existing information from the previous summary
- ADD new progress, decisions, and context from the new messages
- UPDATE the Progress section: move items from "In Progress" to "Done" when completed
- UPDATE "Next Steps" based on what was accomplished
- PRESERVE exact file paths, function names, and error messages
- If something is no longer relevant, you may remove it

Use this EXACT format:

## Goal
[Preserve existing goals, add new ones if the task expanded]

## Constraints & Preferences
- [Preserve existing, add new ones discovered]

## Progress
### Done
- [x] [Include previously done items AND newly completed items]

### In Progress
- [ ] [Current work - update based on progress]

### Blocked
- [Current blockers - remove if resolved]

## Key Decisions
- **[Decision]**: [Brief rationale] (preserve all previous, add new)

## Next Steps
1. [Update based on current state]

## Critical Context
- [Preserve important context, add new if needed]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

// GenerateSummary generates a summary of the conversation using the LLM.
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

	// Serialize conversation to text
	conversationText := SerializeConversation(messages)

	// Build the prompt
	var promptText string
	if previousSummary != "" {
		promptText = fmt.Sprintf("<conversation>\n%s\n</conversation>\n\n<previous-summary>\n%s\n</previous-summary>\n\n%s",
			conversationText, previousSummary, updateSummarizationPrompt)
	} else {
		promptText = fmt.Sprintf("<conversation>\n%s\n</conversation>\n\n%s",
			conversationText, summarizationPrompt)
	}

	// Create summarization request
	summarizeMsg := provider.NewUserMessage(promptText)
	summarizeParams := provider.ChatParams{
		Messages:     []provider.Message{summarizeMsg},
		SystemPrompt: "You are a conversation summarizer. Create concise, structured summaries that preserve critical context for continuing work.",
		MaxTokens:    maxTokens,
		ModelID:      model.ID,
	}

	// Call LLM to generate summary
	streamCh := p.Chat(ctx, summarizeParams)

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

// Compact performs context compaction on the messages.
func Compact(
	ctx context.Context,
	messages []provider.Message,
	p provider.Provider,
	model *provider.Model,
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

	// Find cut point
	cutPoint := FindCutPoint(messages, 0, len(messages), settings.KeepRecentTokens)

	// Messages to summarize (will be discarded after summary)
	messagesToSummarize := messages[:cutPoint.FirstKeptIndex]
	if cutPoint.IsSplitTurn && cutPoint.TurnStartIndex >= 0 {
		messagesToSummarize = messages[:cutPoint.TurnStartIndex]
	}

	if len(messagesToSummarize) == 0 {
		return nil, fmt.Errorf("nothing to compact")
	}

	// Generate summary
	summary, err := GenerateSummary(ctx, messagesToSummarize, p, model, settings.ReserveTokens, previousSummary)
	if err != nil {
		return nil, fmt.Errorf("generate summary: %w", err)
	}

	return &CompactionResult{
		Summary:        summary,
		FirstKeptIndex: cutPoint.FirstKeptIndex,
		TokensBefore:   tokensBefore,
	}, nil
}
