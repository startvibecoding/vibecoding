package agent

import (
	agentpkg "github.com/startvibecoding/vibecoding/agent"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/tools"
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
	EventToolApprovalRequest  // Request user approval for tool execution
	EventToolApprovalResponse // User response to approval request
	EventPlanUpdate           // Structured task plan update

	// Status events
	EventStatus
	EventDone
	EventError
	EventUsage

	// Compaction events
	EventCompactionStart
	EventCompactionEnd

	// Pressure events
	EventContextPressure // Context usage exceeded threshold (one-shot)
	EventBudgetPressure  // Remaining iterations below threshold (one-shot)
)

// Event represents an event from the agent to the UI.
type Event struct {
	Type    EventType
	AgentID agentpkg.AgentID

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
	ToolDiff      *tools.FileDiff
	ToolError     error
	PartialResult any

	// Plan events
	Plan *tools.TaskPlan

	// Approval events
	ApprovalID     string         // Unique ID for approval request
	ApprovalTool   string         // Tool name requiring approval
	ApprovalArgs   map[string]any // Tool arguments
	ApprovalResult bool           // true = approved, false = denied

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

	// Pressure info (for EventContextPressure / EventBudgetPressure)
	PressureMessage string // Human-readable warning message
	PressureType    string // "context" or "budget"
	PressurePercent float64 // Usage percentage that triggered the event
}
