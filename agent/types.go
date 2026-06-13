// Package agent defines the public Agent interface and related types.
// External Go developers can import this package to create custom Agent implementations
// or use the Builder to instantiate the built-in Agent.
//
// Import path: github.com/startvibecoding/vibecoding/agent
package agent

import "context"

// AgentID uniquely identifies an agent instance.
type AgentID string

// Agent is the interface that all agent implementations must satisfy.
type Agent interface {
	// ID returns the unique identifier for this agent.
	ID() AgentID

	// ParentID returns the ID of the parent agent, or empty if top-level.
	ParentID() AgentID

	// Run processes a user message and streams events back.
	Run(ctx context.Context, userMsg string) <-chan Event

	// RunWithMessages processes with explicit message history.
	RunWithMessages(ctx context.Context, messages []Message) <-chan Event

	// Abort signals the agent to stop processing.
	Abort()

	// GetMessages returns a copy of the current message history.
	GetMessages() []Message

	// SetMessages replaces the message history.
	SetMessages(msgs []Message)

	// GetContext returns a copy of the current agent context.
	GetContext() *AgentContext

	// SetContext replaces the agent context.
	SetContext(ctx *AgentContext)

	// GetContextUsage returns the current context window usage, or nil if unavailable.
	GetContextUsage() *ContextUsage

	// LoadHistoryMessages loads historical messages into agent context.
	LoadHistoryMessages(messages []Message)

	// HandleApprovalResponse processes the user's approval response for a pending tool call.
	HandleApprovalResponse(approvalID string, approved bool)
}

// QuestionHandler is an optional extension of Agent that supports interactive questions.
// Only implemented by agents in TUI plan mode. Use type assertion to check support.
type QuestionHandler interface {
	Agent
	HandleQuestionResponse(questionID string, answer string)
}

// AgentConfigView is a read-only view of agent configuration for external inspection.
type AgentConfigView struct {
	ID       AgentID
	ParentID AgentID
	Mode     string
	ModelID  string
}

// ContextUsage reports how much of the context window is consumed.
type ContextUsage struct {
	Tokens        int
	ContextWindow int
	Percent       *float64
}

// AgentContext holds the current agent conversation context.
type AgentContext struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
}

// Role identifies who produced a message.
type Role string

const (
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleToolResult Role = "toolResult"
	RoleSystem     Role = "system"
)

// Message represents a single message in the conversation.
type Message struct {
	Role           Role
	Content        string
	Contents       []ContentBlock
	IsError        bool
	SystemInjected bool
	ToolCallID     string
	ToolName       string
	Usage          *Usage
}

// ContentBlock represents a typed block within a message.
type ContentBlock struct {
	Type         string // "text", "toolCall", "thinking", "image"
	Text         string
	ToolCall     *ToolCallBlock
	Thinking     string
	Signature    string
	Image        *ImageContent
	CacheControl *CacheControl
}

// ToolCallBlock represents a tool call requested by the LLM.
type ToolCallBlock struct {
	ID               string
	Name             string
	Arguments        []byte
	ThoughtSignature string
}

// ImageContent represents an image in a content block.
type ImageContent struct {
	MimeType string
	Data     string // base64-encoded
}

// CacheControl represents cache control metadata on a content block.
type CacheControl struct {
	Type string // "ephemeral"
}

// ToolDefinition describes a tool available to the LLM.
type ToolDefinition struct {
	Name         string
	Description  string
	Parameters   []byte // JSON Schema
	Kind         string // "function" (default) or "hosted"
	Provider     string
	ProviderType string
	Model        string
}

// Usage tracks token consumption for a single LLM response.
type Usage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	TotalTokens  int
	Cost         CostBreakdown
}

// CostBreakdown itemizes the cost of an LLM call.
type CostBreakdown struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
	Total      float64
}

// CalculateCost computes cost based on model pricing.
func (u *Usage) CalculateCost(inputPrice, outputPrice, cacheReadPrice, cacheWritePrice float64) {
	u.Cost.Input = float64(u.InputTokens) * inputPrice / 1_000_000
	u.Cost.Output = float64(u.OutputTokens) * outputPrice / 1_000_000
	u.Cost.CacheRead = float64(u.CacheRead) * cacheReadPrice / 1_000_000
	u.Cost.CacheWrite = float64(u.CacheWrite) * cacheWritePrice / 1_000_000
	u.Cost.Total = u.Cost.Input + u.Cost.Output + u.Cost.CacheRead + u.Cost.CacheWrite
}

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
	EventQuestionRequest      // Ask user a multiple-choice question
	EventQuestionResponse     // User response to question
	EventPlanUpdate           // Structured task plan update

	// Status events
	EventStatus
	EventDone
	EventError
	EventUsage

	// Compaction events
	EventCompactionStart
	EventCompactionEnd
)

// Event represents an event from the agent to the consumer.
type Event struct {
	AgentID AgentID
	Type    EventType

	// Agent lifecycle
	Messages []Message

	// Turn lifecycle
	TurnMessage     Message
	TurnToolResults []Message

	// Message lifecycle
	Message Message

	// Stream events
	TextDelta  string
	ThinkDelta string

	// Tool events
	ToolCall      *ToolCallBlock
	ToolCallID    string
	ToolName      string
	ToolArgs      map[string]any
	ToolResult    string
	ToolDiff      *FileDiff
	ToolError     error
	PartialResult any

	// Plan events
	Plan *TaskPlan

	// Approval events
	ApprovalID     string
	ApprovalTool   string
	ApprovalArgs   map[string]any
	ApprovalResult bool

	// Question events
	QuestionID      string
	QuestionText    string
	QuestionOptions []string
	QuestionContext string
	QuestionAnswer  string

	// Status
	StatusMessage string

	// Completion
	Done       bool
	StopReason string
	Error      error

	// Usage
	Usage *Usage

	// Context usage
	ContextUsage *ContextUsage
}

// FileDiff describes a file change produced by a write-like tool.
type FileDiff struct {
	Path         string
	Added        int
	Deleted      int
	AddedLines   []int
	DeletedLines []int
	Unified      string
	Truncated    bool
}

// TaskPlan describes a structured task plan emitted by the plan tool.
type TaskPlan struct {
	Title string
	Steps []PlanStep
	Note  string
}

// PlanStep describes one step in a task plan.
type PlanStep struct {
	Title  string
	Status string
}

// --- Helper constructors ---

// NewUserMessage creates a user message with plain text content.
func NewUserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

// NewAssistantMessage creates an assistant message with content blocks.
func NewAssistantMessage(contents []ContentBlock) Message {
	return Message{Role: RoleAssistant, Contents: contents}
}

// NewAssistantTextMessage creates an assistant message with plain text.
func NewAssistantTextMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}

// NewToolResultMessage creates a tool result message with plain text.
func NewToolResultMessage(toolCallID, toolName, content string, isError bool) Message {
	return Message{
		Role:       RoleToolResult,
		Content:    content,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		IsError:    isError,
	}
}

// NewToolResultMessageWithContents creates a tool result message with rich content blocks.
func NewToolResultMessageWithContents(toolCallID, toolName, text string, contents []ContentBlock, isError bool) Message {
	return Message{
		Role:       RoleToolResult,
		Content:    text,
		Contents:   contents,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		IsError:    isError,
	}
}

// NewSystemInjectedUserMessage creates a user message marked as system-injected
// (skipped by cache markers).
func NewSystemInjectedUserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content, SystemInjected: true}
}
