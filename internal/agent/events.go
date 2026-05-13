package agent

import (
	"github.com/fuckvibecoding/vibecoding/internal/provider"
)

// EventType identifies the type of agent event.
type EventType int

const (
	// Agent lifecycle events
	EventAgentStart EventType = iota
	EventAgentEnd

	// Turn lifecycle events (a turn = one assistant response + tool calls/results)
	EventTurnStart
	EventTurnEnd

	// Message lifecycle events
	EventMessageStart
	EventMessageUpdate
	EventMessageEnd

	// Streaming events
	EventTextDelta
	EventThinkDelta

	// Tool execution events
	EventToolCall
	EventToolExecutionStart
	EventToolExecutionUpdate
	EventToolExecutionEnd
	EventToolResult

	// Status events
	EventStatus
	EventDone
	EventError
	EventUsage
)

// Event represents an event from the agent to the UI.
type Event struct {
	Type EventType

	// Agent lifecycle
	Messages []provider.Message

	// Turn lifecycle
	TurnMessage     provider.Message
	TurnToolResults []provider.Message

	// Message lifecycle
	Message provider.Message

	// Stream events
	TextDelta  string
	ThinkDelta string

	// Tool events
	ToolCall      *provider.ToolCallBlock
	ToolCallID    string
	ToolName      string
	ToolArgs      map[string]any
	ToolResult    string
	ToolError     error
	PartialResult any

	// Status
	StatusMessage string

	// Completion
	Done       bool
	StopReason string
	Error      error

	// Usage
	Usage *provider.Usage
}
