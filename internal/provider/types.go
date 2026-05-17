package provider

import (
	"encoding/json"
	"fmt"
	"time"
)

// CacheControl represents cache control hints for prompt caching.
type CacheControl struct {
	Type string `json:"type"` // "ephemeral" for breakpoint markers
}

// ContentBlock represents a block of content in a message.
type ContentBlock struct {
	Type         string         `json:"type"` // "text", "image", "thinking", "toolCall"
	Text         string         `json:"text,omitempty"`
	Thinking     string         `json:"thinking,omitempty"`
	Image        *ImageContent  `json:"image,omitempty"`
	ToolCall     *ToolCallBlock `json:"toolCall,omitempty"`
	CacheControl *CacheControl  `json:"cache_control,omitempty"` // cache breakpoint marker
}

// ImageContent represents an image in a message.
type ImageContent struct {
	Data     string `json:"data"`     // base64 encoded
	MimeType string `json:"mimeType"` // e.g. "image/png"
}

// ToolCallBlock represents a tool call in an assistant message.
type ToolCallBlock struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Message represents a conversation message.
type Message struct {
	Role           string         `json:"role"`                 // "user", "assistant", "toolResult"
	Content        string         `json:"content,omitempty"`    // simple text content
	Contents       []ContentBlock `json:"contents,omitempty"`   // rich content blocks
	ToolCallID     string         `json:"toolCallId,omitempty"` // for toolResult
	ToolName       string         `json:"toolName,omitempty"`   // for toolResult
	IsError        bool           `json:"isError,omitempty"`    // for toolResult
	Timestamp      time.Time      `json:"timestamp"`
	Usage          *Usage         `json:"usage,omitempty"`          // token usage from API response
	SystemInjected bool           `json:"systemInjected,omitempty"` // true for injected messages (session context, compression instructions) - skipped by cache markers
}

// NewUserMessage creates a simple user text message.
func NewUserMessage(text string) Message {
	return Message{
		Role:      "user",
		Content:   text,
		Timestamp: time.Now(),
	}
}

// NewSystemInjectedUserMessage creates a system-injected user message (skipped by cache markers).
func NewSystemInjectedUserMessage(text string) Message {
	return Message{
		Role:           "user",
		Content:        text,
		Timestamp:      time.Now(),
		SystemInjected: true,
	}
}

// NewAssistantMessage creates an assistant message with content blocks.
func NewAssistantMessage(contents []ContentBlock) Message {
	return Message{
		Role:      "assistant",
		Contents:  contents,
		Timestamp: time.Now(),
	}
}

// NewToolResultMessage creates a tool result message.
func NewToolResultMessage(toolCallID, toolName, content string, isError bool) Message {
	return Message{
		Role:       "toolResult",
		Content:    content,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		IsError:    isError,
		Timestamp:  time.Now(),
	}
}

// NewToolResultMessageWithContents creates a tool result message with rich content blocks.
// If contents is nil or empty, it falls back to using the text parameter.
func NewToolResultMessageWithContents(toolCallID, toolName, text string, contents []ContentBlock, isError bool) Message {
	msg := Message{
		Role:       "toolResult",
		ToolCallID: toolCallID,
		ToolName:   toolName,
		IsError:    isError,
		Timestamp:  time.Now(),
	}
	if len(contents) > 0 {
		msg.Contents = contents
		// Also set Content for backward compatibility (display/logging)
		msg.Content = text
	} else {
		msg.Content = text
	}
	return msg
}

// Usage represents token usage and cost information.
type Usage struct {
	Input       int  `json:"input"`
	Output      int  `json:"output"`
	CacheRead   int  `json:"cacheRead"`
	CacheWrite  int  `json:"cacheWrite"`
	TotalTokens int  `json:"totalTokens"`
	Cost        Cost `json:"cost"`
}

// PromptTokens returns the total prompt tokens sent for the turn, including
// cached tokens when the provider reports them separately.
func (u *Usage) PromptTokens() int {
	if u == nil {
		return 0
	}
	if u.TotalTokens > 0 {
		prompt := u.TotalTokens - u.Output
		if prompt > 0 {
			return prompt
		}
	}
	return u.Input
}

// TotalInputTokens returns the full input footprint for the turn, including
// cached reads and writes when the provider reports them separately.
func (u *Usage) TotalInputTokens() int {
	if u == nil {
		return 0
	}
	if u.TotalTokens > 0 {
		totalInput := u.TotalTokens - u.Output
		if totalInput > 0 {
			return totalInput
		}
	}
	return u.Input + u.CacheRead + u.CacheWrite
}

// CacheInfo returns a short display string for cache activity (e.g. "Cache: 75%"),
// or an empty string when there is no cache data to show.
func (u *Usage) CacheInfo() string {
	totalInputTokens := u.TotalInputTokens()
	switch {
	case totalInputTokens > 0 && u.CacheRead > 0:
		pct := float64(u.CacheRead) / float64(totalInputTokens) * 100
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("Cache: %.0f%%", pct)
	case u.CacheWrite > 0 && u.CacheRead == 0:
		return fmt.Sprintf("CacheWrite: %d", u.CacheWrite)
	case totalInputTokens > 0 && u.CacheRead == 0 && u.CacheWrite == 0:
		return "Cache: 0%"
	default:
		return ""
	}
}

// Cost represents the monetary cost of a request.
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

// CalculateCost computes the cost based on the model's pricing.
func (u *Usage) CalculateCost(model *Model) {
	if model == nil {
		return
	}
	c := Cost{
		Input:      float64(u.Input) / 1_000_000 * model.Cost.Input,
		Output:     float64(u.Output) / 1_000_000 * model.Cost.Output,
		CacheRead:  float64(u.CacheRead) / 1_000_000 * model.Cost.CacheRead,
		CacheWrite: float64(u.CacheWrite) / 1_000_000 * model.Cost.CacheWrite,
	}
	c.Total = c.Input + c.Output + c.CacheRead + c.CacheWrite
	u.Cost = c
}

// ModelPricing represents the cost per million tokens for a model.
type ModelPricing struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// Model represents a model available from a provider.
type Model struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Provider      string       `json:"provider"`
	Reasoning     bool         `json:"reasoning"` // supports extended thinking
	Input         []string     `json:"input"`     // "text", "image"
	Cost          ModelPricing `json:"cost"`
	ContextWindow int          `json:"contextWindow"` // max context tokens
	MaxTokens     int          `json:"maxTokens"`     // max output tokens
}

// ThinkingLevel represents the depth of reasoning.
type ThinkingLevel string

const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
)

// ToolDefinition describes a tool available to the model.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// StreamEventType identifies the type of a streaming event.
type StreamEventType int

const (
	StreamStart      StreamEventType = iota // Stream started
	StreamTextDelta                         // Text content delta
	StreamThinkDelta                        // Thinking content delta
	StreamToolCall                          // Tool call event
	StreamUsage                             // Usage statistics
	StreamDone                              // Stream completed
	StreamError                             // Error occurred
)

// StreamEvent represents a single event from a streaming response.
type StreamEvent struct {
	Type       StreamEventType
	TextDelta  string         // for StreamTextDelta
	ThinkDelta string         // for StreamThinkDelta
	ToolCall   *ToolCallBlock // for StreamToolCall
	Usage      *Usage         // for StreamUsage
	Error      error          // for StreamError
	StopReason string         // for StreamDone: "stop", "length", "toolUse", "error", "aborted"
}

// ChatParams contains all parameters for a chat request.
type ChatParams struct {
	Messages      []Message
	Tools         []ToolDefinition
	SystemPrompt  string
	ThinkingLevel ThinkingLevel
	MaxTokens     int
	ModelID       string          // which model to use
	Abort         <-chan struct{} // closed to abort the request
}
