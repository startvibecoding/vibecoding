package agent

import (
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
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
	EventToolApprovalRequest // Request user approval for tool execution
	EventToolApprovalResponse // User response to approval request

	// Status events
	EventStatus
	EventDone
	EventError
	EventUsage

	// Compaction events
	EventCompactionStart
	EventCompactionEnd
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

	// Approval events
	ApprovalID     string // Unique ID for approval request
	ApprovalTool   string // Tool name requiring approval
	ApprovalArgs   map[string]any // Tool arguments
	ApprovalResult bool // true = approved, false = denied

	// Status
	StatusMessage string

	// Completion
	Done       bool
	StopReason string
	Error      error

	// Usage
	Usage *provider.Usage

	// Context usage
	ContextUsage *ctxpkg.ContextUsage
}
