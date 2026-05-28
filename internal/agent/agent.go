package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

// contextKey is an unexported type for context keys defined in this package.
type contextKey int

const (
	// agentIDKey is the context key for the current agent's ID.
	agentIDKey contextKey = iota
	// agentEventChanKey is the context key for the current agent's event channel.
	agentEventChanKey
)

// ContextWithAgentID returns a new context with the agent ID attached.
func ContextWithAgentID(ctx context.Context, id agentpkg.AgentID) context.Context {
	return context.WithValue(ctx, agentIDKey, id)
}

// AgentIDFromContext extracts the agent ID from the context.
func AgentIDFromContext(ctx context.Context) (agentpkg.AgentID, bool) {
	id, ok := ctx.Value(agentIDKey).(agentpkg.AgentID)
	return id, ok
}

// ContextWithEventChan returns a new context with the event channel attached.
func ContextWithEventChan(ctx context.Context, ch chan<- Event) context.Context {
	return context.WithValue(ctx, agentEventChanKey, ch)
}

// EventChanFromContext extracts the event channel from the context.
func EventChanFromContext(ctx context.Context) (chan<- Event, bool) {
	ch, ok := ctx.Value(agentEventChanKey).(chan<- Event)
	return ch, ok
}

// Config holds the agent configuration.
type Config struct {
	ID                 agentpkg.AgentID
	ParentID           agentpkg.AgentID
	Provider           provider.Provider
	Model              *provider.Model
	Mode               string // "plan", "agent", "yolo"
	ThinkingLevel      provider.ThinkingLevel
	MaxTokens          int
	SandboxMgr         *sandbox.Manager
	Settings           *config.Settings
	Session            *session.Manager
	ExtraContext       string // extra context from files and skills
	CompactionSettings ctxpkg.CompactionSettings
	ApprovalHandler    func(toolCallID, toolName string, args map[string]any) bool
	MultiAgent         bool // Decision 8: multi-agent mode
}

// AgentLoopConfig extends Config with loop-specific settings.
type AgentLoopConfig struct {
	Config

	// ToolExecutionMode determines how tool calls are executed.
	// "sequential": execute one by one
	// "parallel": execute concurrently (default)
	ToolExecutionMode string

	// MaxIterations is the safety limit for agent loop iterations.
	MaxIterations int

	// GetSteeringMessages returns messages to inject mid-run.
	GetSteeringMessages func() []provider.Message

	// GetFollowUpMessages returns messages to process after agent would stop.
	GetFollowUpMessages func() []provider.Message

	// ShouldStopAfterTurn is called after each turn to check if we should stop.
	ShouldStopAfterTurn func(ctx ShouldStopAfterTurnContext) bool

	// PrepareNextTurn is called before the next turn to update context/model.
	PrepareNextTurn func(ctx PrepareNextTurnContext) *TurnUpdate

	// BeforeToolCall is called before a tool is executed.
	BeforeToolCall func(ctx BeforeToolCallContext) *ToolCallBlockResult

	// AfterToolCall is called after a tool finishes executing.
	AfterToolCall func(ctx AfterToolCallContext) *ToolCallResult
}

// ShouldStopAfterTurnContext is passed to ShouldStopAfterTurn.
type ShouldStopAfterTurnContext struct {
	Message     provider.Message
	ToolResults []provider.Message
	Context     *AgentContext
	NewMessages []provider.Message
}

// PrepareNextTurnContext is passed to PrepareNextTurn.
type PrepareNextTurnContext struct {
	ShouldStopAfterTurnContext
}

// TurnUpdate is returned from PrepareNextTurn.
type TurnUpdate struct {
	Context       *AgentContext
	Model         *provider.Model
	ThinkingLevel provider.ThinkingLevel
}

// BeforeToolCallContext is passed to BeforeToolCall.
type BeforeToolCallContext struct {
	AssistantMessage provider.Message
	ToolCall         provider.ToolCallBlock
	Args             any
	Context          *AgentContext
}

// ToolCallBlockResult is returned from BeforeToolCall.
type ToolCallBlockResult struct {
	Block  bool
	Reason string
}

// AfterToolCallContext is passed to AfterToolCall.
type AfterToolCallContext struct {
	AssistantMessage provider.Message
	ToolCall         provider.ToolCallBlock
	Args             any
	Result           ToolCallResult
	IsError          bool
	Context          *AgentContext
}

// ToolCallResult represents the result of a tool call.
type ToolCallResult struct {
	Content   string
	IsError   bool
	Terminate bool
}

// AgentContext holds the current agent context.
type AgentContext struct {
	SystemPrompt string
	Messages     []provider.Message
	Tools        []provider.ToolDefinition
}

// Agent is the core agent loop.
type Agent struct {
	id          agentpkg.AgentID
	parentID    agentpkg.AgentID
	config      AgentLoopConfig
	registry    *tools.Registry
	mu          sync.RWMutex
	context     *AgentContext
	abort       chan struct{}
	abortOnce   sync.Once
	messages    []provider.Message
	isStreaming bool

	// Frozen system prompt and tools (built once, never change during session)
	// This is critical for prompt cache optimization - see LLM_Agent_Cache.md
	frozenSystemPrompt string
	frozenToolDefs     []provider.ToolDefinition
	frozenToolNames    []string

	// Approval mechanism for agent mode
	pendingApprovals map[string]chan bool // approvalID -> response channel
	approvalMu       sync.Mutex
	approvalCounter  int64

	// Force compaction flag — set by /compact command, consumed by ShouldCompact
	forceCompact int32 // atomic: 0=false, 1=true
}

// buildFrozenPrompt builds the system prompt and tools once at construction time.
// These values are frozen for the entire session lifetime to maximize prompt cache hits.
// This implements Rule R2.1 from LLM_Agent_Cache.md: System prompt must be built once and never modified.
func (a *Agent) buildFrozenPrompt() {
	toolNames := make([]string, 0)
	for _, t := range a.registry.ModeTools(a.config.Mode) {
		toolNames = append(toolNames, t.Name)
	}
	toolSnippets := a.registry.ToolSnippets(toolNames)
	toolGuidelines := a.registry.ToolGuidelines(toolNames)
	a.frozenSystemPrompt = BuildSystemPrompt(
		a.config.Mode,
		toolNames,
		a.registry.GetWorkDir(),
		a.config.ExtraContext,
		toolSnippets,
		toolGuidelines,
		a.config.MultiAgent,
	)
	a.frozenToolDefs = a.registry.ModeTools(a.config.Mode)
	a.frozenToolNames = toolNames
}

// buildSessionContextMessage builds the [session context] message with dynamic information.
// This implements Rule R2.3 from LLM_Agent_Cache.md: dynamic info goes into a separate message.
// The message is marked as SystemInjected so cache markers skip it.
func (a *Agent) buildSessionContextMessage() provider.Message {
	modelID := "unknown"
	modelName := "unknown"
	if a.config.Model != nil {
		modelID = a.config.Model.ID
		modelName = a.config.Model.Name
	}

	context := fmt.Sprintf(`[session context]
- Current date: %s
- Model: %s (%s)
- Working directory: %s
- Mode: %s
`,
		time.Now().Format("2006-01-02"),
		modelName,
		modelID,
		a.registry.GetWorkDir(),
		a.config.Mode,
	)

	return provider.NewSystemInjectedUserMessage(context)
}

// selectCacheMarkers selects the last 2 non-injected messages for cache control markers.
// This implements Rule R3.2 from LLM_Agent_Cache.md: dual-marker selection algorithm.
// Returns the indices of the two messages to mark.
func selectCacheMarkers(messages []provider.Message) [2]int {
	var markers [2]int
	markers[0] = -1
	markers[1] = -1

	count := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].SystemInjected {
			continue
		}
		if count == 0 {
			markers[1] = i // newest marker
		} else if count == 1 {
			markers[0] = i // second newest marker
			break
		}
		count++
	}
	return markers
}

// applyCacheMarkers applies cache_control markers to messages for prompt caching.
// This implements the dual-marker rolling buffer from Rule R3.1-R3.3.
// Returns a new slice with markers applied (does not modify original).
func applyCacheMarkers(messages []provider.Message, markers [2]int) []provider.Message {
	if markers[0] == -1 && markers[1] == -1 {
		return messages
	}

	// Create a deep copy to avoid modifying the original messages
	result := make([]provider.Message, len(messages))
	for i, msg := range messages {
		result[i] = msg
		// Deep copy Contents slice and pointer fields
		if len(msg.Contents) > 0 {
			result[i].Contents = make([]provider.ContentBlock, len(msg.Contents))
			for j, cb := range msg.Contents {
				result[i].Contents[j] = cb
				if cb.Image != nil {
					imgCopy := *cb.Image
					result[i].Contents[j].Image = &imgCopy
				}
				if cb.ToolCall != nil {
					tcCopy := *cb.ToolCall
					result[i].Contents[j].ToolCall = &tcCopy
				}
				if cb.CacheControl != nil {
					ccCopy := *cb.CacheControl
					result[i].Contents[j].CacheControl = &ccCopy
				}
			}
		}
	}

	for _, idx := range markers {
		if idx < 0 || idx >= len(result) {
			continue
		}
		msg := &result[idx]
		if len(msg.Contents) > 0 {
			// Add cache_control to the last content block
			lastIdx := len(msg.Contents) - 1
			msg.Contents[lastIdx].CacheControl = &provider.CacheControl{Type: "ephemeral"}
		} else if msg.Content != "" {
			// Convert simple text to content blocks with cache_control
			msg.Contents = []provider.ContentBlock{
				{
					Type:         "text",
					Text:         msg.Content,
					CacheControl: &provider.CacheControl{Type: "ephemeral"},
				},
			}
			msg.Content = ""
		}
	}

	return result
}

// New creates a new agent.
func New(cfg Config, registry *tools.Registry) *Agent {
	loopConfig := AgentLoopConfig{
		Config:            cfg,
		ToolExecutionMode: "parallel",
		MaxIterations:     200,
	}

	id := cfg.ID
	if id == "" {
		id = agentpkg.AgentID(fmt.Sprintf("agent-%d", time.Now().UnixNano()))
	}

	agent := &Agent{
		id:               id,
		parentID:         cfg.ParentID,
		config:           loopConfig,
		registry:         registry,
		abort:            make(chan struct{}),
		pendingApprovals: make(map[string]chan bool),
		context: &AgentContext{
			Messages: make([]provider.Message, 0),
		},
	}
	// Build frozen system prompt once at construction time (R2.1)
	agent.buildFrozenPrompt()
	agent.context.SystemPrompt = agent.frozenSystemPrompt
	agent.context.Tools = agent.frozenToolDefs
	return agent
}

// NewWithLoopConfig creates a new agent with custom loop configuration.
func NewWithLoopConfig(cfg AgentLoopConfig, registry *tools.Registry) *Agent {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 200
	}
	if cfg.ToolExecutionMode == "" {
		cfg.ToolExecutionMode = "parallel"
	}

	id := cfg.ID
	if id == "" {
		id = agentpkg.AgentID(fmt.Sprintf("agent-%d", time.Now().UnixNano()))
	}

	agent := &Agent{
		id:               id,
		parentID:         cfg.ParentID,
		config:           cfg,
		registry:         registry,
		abort:            make(chan struct{}),
		pendingApprovals: make(map[string]chan bool),
		context: &AgentContext{
			Messages: make([]provider.Message, 0),
		},
	}
	// Build frozen system prompt once at construction time (R2.1)
	agent.buildFrozenPrompt()
	agent.context.SystemPrompt = agent.frozenSystemPrompt
	agent.context.Tools = agent.frozenToolDefs
	return agent
}

// LoadHistoryMessages loads historical messages from session into agent context.
func (a *Agent) LoadHistoryMessages(messages []provider.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, messages...)
	a.context.Messages = append(a.context.Messages, messages...)
}

// Abort signals the agent to stop processing.
// Satisfies both internal and public agent.Agent interface.
func (a *Agent) Abort() {
	a.abortOnce.Do(func() {
		close(a.abort)
	})
}

// emit sends an event with this agent's ID stamped on it.
func (a *Agent) emit(ch chan<- Event, event Event) {
	event.AgentID = a.id
	ch <- event
}

// --- Public agent.Agent interface methods ---

// ID returns the agent's unique identifier.
func (a *Agent) ID() agentpkg.AgentID { return a.id }

// ParentID returns the parent agent's ID, or empty if top-level.
func (a *Agent) ParentID() agentpkg.AgentID { return a.parentID }

// Run processes a user message and streams events back.
func (a *Agent) Run(ctx context.Context, userMsg string) <-chan Event {
	ch := make(chan Event, 100)

	go func() {
		defer close(ch)

		// Add user message to conversation
		msg := provider.NewUserMessage(userMsg)
		a.mu.Lock()
		a.messages = append(a.messages, msg)
		a.context.Messages = append(a.context.Messages, msg)
		a.mu.Unlock()

		// Save to session
		if a.config.Session != nil {
			if _, err := a.config.Session.AppendMessage(msg); err != nil {
				ch <- Event{Type: EventError, Error: fmt.Errorf("save user message to session: %w", err)}
				return
			}
		}

		// Run agent loop
		a.loop(ctx, ch)
	}()

	return ch
}

// RunWithMessages processes with explicit message history.
func (a *Agent) RunWithMessages(ctx context.Context, messages []provider.Message) <-chan Event {
	ch := make(chan Event, 100)

	go func() {
		defer close(ch)
		a.mu.Lock()
		a.messages = messages
		a.context.Messages = messages
		a.mu.Unlock()
		a.loop(ctx, ch)
	}()

	return ch
}

// loop runs the main agent loop: send message -> receive response -> execute tools -> repeat.
func (a *Agent) loop(ctx context.Context, ch chan<- Event) {
	ch <- Event{Type: EventAgentStart}

	// Track consecutive iterations without text output for loop detection
	consecutiveNoText := 0
	const maxConsecutiveNoText = 95            // Threshold to trigger stuck detection
	const maxConsecutiveNoTextAfterWarning = 5 // After warning, allow 5 more turns before stopping
	warningIssued := false

	for i := 0; i < a.config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			ch <- Event{Type: EventError, Error: ctx.Err(), StopReason: "aborted"}
			ch <- Event{Type: EventAgentEnd, Messages: func() []provider.Message {
				a.mu.RLock()
				defer a.mu.RUnlock()
				m := make([]provider.Message, len(a.messages))
				copy(m, a.messages)
				return m
			}()}
			return
		default:
		}

		ch <- Event{Type: EventTurnStart}

		// Process pending steering messages
		if a.config.GetSteeringMessages != nil {
			steeringMessages := a.config.GetSteeringMessages()
			for _, msg := range steeringMessages {
				ch <- Event{Type: EventMessageStart, Message: msg}
				ch <- Event{Type: EventMessageEnd, Message: msg}
				a.messages = append(a.messages, msg)
				a.context.Messages = append(a.context.Messages, msg)
			}
		}

		// Use frozen system prompt and tools (R2.1: built once, never change during session)
		a.context.SystemPrompt = a.frozenSystemPrompt
		a.context.Tools = a.frozenToolDefs

		// Build session context message with dynamic info (R2.3)
		sessionContextMsg := a.buildSessionContextMessage()

		// Build message list: session context + history messages
		// Session context is marked as system_injected, so cache markers skip it
		a.mu.RLock()
		allMessages := make([]provider.Message, 0, len(a.messages)+1)
		allMessages = append(allMessages, sessionContextMsg)
		allMessages = append(allMessages, a.messages...)
		a.mu.RUnlock()

		// Select cache markers (dual-marker rolling buffer, R3.1-R3.3)
		markers := selectCacheMarkers(allMessages)
		messagesWithMarkers := applyCacheMarkers(allMessages, markers)

		// Chat request with frozen system prompt and cache markers
		params := provider.ChatParams{
			Messages:      messagesWithMarkers,
			Tools:         a.frozenToolDefs,
			SystemPrompt:  a.frozenSystemPrompt,
			ThinkingLevel: a.config.ThinkingLevel,
			MaxTokens:     a.config.MaxTokens,
			Abort:         a.abort,
		}

		streamCh := a.config.Provider.Chat(ctx, params)

		var (
			textContent    string
			thinkContent   string
			thinkSignature string
			toolCalls      []provider.ToolCallBlock
			usage          *provider.Usage
			stopReason     string
			streamErr      error
		)

		// Process stream events
		for event := range streamCh {
			switch event.Type {
			case provider.StreamStart:
				// Stream started
			case provider.StreamTextDelta:
				textContent += event.TextDelta
				ch <- Event{Type: EventTextDelta, TextDelta: event.TextDelta}
			case provider.StreamThinkDelta:
				thinkContent += event.ThinkDelta
				ch <- Event{Type: EventThinkDelta, ThinkDelta: event.ThinkDelta}
			case provider.StreamThinkSignature:
				thinkSignature = event.ThinkSignature
			case provider.StreamToolCall:
				if event.ToolCall != nil {
					if event.ToolCall.ID == "" {
						event.ToolCall.ID = fmt.Sprintf("toolcall_%d", len(toolCalls))
					}
					toolCalls = append(toolCalls, *event.ToolCall)
					// Parse arguments for the event
					var args map[string]any
					if len(event.ToolCall.Arguments) > 0 {
						if err := json.Unmarshal(event.ToolCall.Arguments, &args); err != nil {
							// Log parse error but continue - tool execution will handle invalid args
							ch <- Event{Type: EventStatus, StatusMessage: fmt.Sprintf("Warning: failed to parse tool arguments: %v", err)}
						}
					}
					ch <- Event{Type: EventToolCall, ToolCall: event.ToolCall, ToolArgs: args}
				}
			case provider.StreamUsage:
				usage = event.Usage
				ch <- Event{Type: EventUsage, Usage: event.Usage, ContextUsage: a.GetContextUsage()}
			case provider.StreamDone:
				stopReason = event.StopReason
			case provider.StreamError:
				streamErr = event.Error
				stopReason = event.StopReason
			case provider.StreamRetry:
				if event.Error != nil {
					ch <- Event{Type: EventStatus, StatusMessage: event.Error.Error()}
				}
			}
		}

		if streamErr != nil {
			ch <- Event{Type: EventError, Error: streamErr, StopReason: stopReason}
			ch <- Event{Type: EventAgentEnd, Messages: func() []provider.Message {
				a.mu.RLock()
				defer a.mu.RUnlock()
				m := make([]provider.Message, len(a.messages))
				copy(m, a.messages)
				return m
			}()}
			return
		}

		// Build assistant message
		var contents []provider.ContentBlock
		if thinkContent != "" {
			contents = append(contents, provider.ContentBlock{
				Type:      "thinking",
				Thinking:  thinkContent,
				Signature: thinkSignature,
			})
		}
		if textContent != "" {
			contents = append(contents, provider.ContentBlock{
				Type: "text",
				Text: textContent,
			})
		}
		for _, tc := range toolCalls {
			tc := tc
			contents = append(contents, provider.ContentBlock{
				Type:     "toolCall",
				ToolCall: &tc,
			})
		}

		assistantMsg := provider.NewAssistantMessage(contents)
		// Store usage in the message for context tracking
		if usage != nil {
			assistantMsg.Usage = usage
		}
		a.mu.Lock()
		a.messages = append(a.messages, assistantMsg)
		a.context.Messages = append(a.context.Messages, assistantMsg)
		a.mu.Unlock()

		// Save to session
		if a.config.Session != nil {
			if _, err := a.config.Session.AppendMessage(assistantMsg); err != nil {
				ch <- Event{Type: EventError, Error: fmt.Errorf("save assistant message to session: %w", err)}
				return
			}
		}

		// Calculate cost
		if usage != nil && a.config.Model != nil {
			usage.CalculateCost(a.config.Model)
		}

		// Track progress for loop detection. Tool-only warnings are injected
		// after tool results are recorded so provider message ordering stays valid.
		if textContent != "" {
			consecutiveNoText = 0
			warningIssued = false // AI responded with text, reset warning state
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			contextUsage := a.GetContextUsage()
			ch <- Event{Type: EventTurnEnd, TurnMessage: assistantMsg, ContextUsage: contextUsage}
			ch <- Event{Type: EventDone, StopReason: stopReason, Usage: usage, ContextUsage: contextUsage}
			ch <- Event{Type: EventAgentEnd, Messages: func() []provider.Message {
				a.mu.RLock()
				defer a.mu.RUnlock()
				m := make([]provider.Message, len(a.messages))
				copy(m, a.messages)
				return m
			}()}
			return
		}

		// Execute tool calls
		var toolResults []provider.Message
		if a.config.ToolExecutionMode == "sequential" {
			toolResults = a.executeToolCallsSequential(ctx, toolCalls, ch)
		} else {
			toolResults = a.executeToolCallsParallel(ctx, toolCalls, ch)
		}

		// Add tool results to context
		a.mu.Lock()
		for _, result := range toolResults {
			a.messages = append(a.messages, result)
			a.context.Messages = append(a.context.Messages, result)
		}
		a.mu.Unlock()
		for _, result := range toolResults {
			if a.config.Session != nil {
				if _, err := a.config.Session.AppendMessage(result); err != nil {
					ch <- Event{Type: EventError, Error: fmt.Errorf("save tool result to session: %w", err)}
					return
				}
			}
		}

		if textContent == "" {
			consecutiveNoText++
			threshold := maxConsecutiveNoText
			if warningIssued {
				threshold = maxConsecutiveNoTextAfterWarning
			}
			if consecutiveNoText >= threshold {
				if !warningIssued {
					// Inject a warning message to let the AI explain itself.
					warningMsg := provider.NewUserMessage("[System] You have been making tool calls for " + fmt.Sprintf("%d", consecutiveNoText) + " consecutive turns without any text response. Please explain what you are doing and whether you are stuck. If you are making progress, briefly describe your current task and continue. If you are truly stuck, please stop and explain the issue.")
					ch <- Event{Type: EventMessageStart, Message: warningMsg}
					ch <- Event{Type: EventMessageEnd, Message: warningMsg}
					a.mu.Lock()
					a.messages = append(a.messages, warningMsg)
					a.context.Messages = append(a.context.Messages, warningMsg)
					a.mu.Unlock()
					if a.config.Session != nil {
						if _, err := a.config.Session.AppendMessage(warningMsg); err != nil {
							ch <- Event{Type: EventError, Error: fmt.Errorf("save warning message to session: %w", err)}
							return
						}
					}
					warningIssued = true
					consecutiveNoText = 0 // Reset counter for post-warning phase
				} else {
					// Already warned, now truly stuck. Tool results have already been
					// appended, so the saved transcript remains provider-valid.
					ch <- Event{Type: EventError, Error: fmt.Errorf("agent appears stuck: %d consecutive turns without text output after warning", consecutiveNoText+maxConsecutiveNoText), StopReason: "stuck"}
					ch <- Event{Type: EventAgentEnd, Messages: func() []provider.Message {
						a.mu.RLock()
						defer a.mu.RUnlock()
						m := make([]provider.Message, len(a.messages))
						copy(m, a.messages)
						return m
					}()}
					return
				}
			}
		}

		ch <- Event{Type: EventTurnEnd, TurnMessage: assistantMsg, TurnToolResults: toolResults, ContextUsage: a.GetContextUsage()}

		// Check if compaction should trigger
		if a.ShouldCompact() {
			if err := a.Compact(ctx, ch); err != nil {
				// Log error but continue
				ch <- Event{Type: EventStatus, StatusMessage: fmt.Sprintf("Compaction failed: %v", err)}
			}
		}

		// Check if we should stop after this turn
		if a.config.ShouldStopAfterTurn != nil {
			stopCtx := ShouldStopAfterTurnContext{
				Message:     assistantMsg,
				ToolResults: toolResults,
				Context:     a.context,
				NewMessages: a.messages,
			}
			if a.config.ShouldStopAfterTurn(stopCtx) {
				ch <- Event{Type: EventDone, StopReason: "should_stop"}
				ch <- Event{Type: EventAgentEnd, Messages: func() []provider.Message {
					a.mu.RLock()
					defer a.mu.RUnlock()
					m := make([]provider.Message, len(a.messages))
					copy(m, a.messages)
					return m
				}()}
				return
			}
		}

		// Prepare next turn
		if a.config.PrepareNextTurn != nil {
			prepCtx := PrepareNextTurnContext{
				ShouldStopAfterTurnContext: ShouldStopAfterTurnContext{
					Message:     assistantMsg,
					ToolResults: toolResults,
					Context:     a.context,
					NewMessages: a.messages,
				},
			}
			update := a.config.PrepareNextTurn(prepCtx)
			if update != nil {
				if update.Context != nil {
					a.context = update.Context
				}
				if update.Model != nil {
					a.config.Model = update.Model
				}
				if update.ThinkingLevel != "" {
					a.config.ThinkingLevel = update.ThinkingLevel
				}
			}
		}

		// Check for steering messages (for mid-run injection)
		if a.config.GetSteeringMessages != nil {
			steeringMessages := a.config.GetSteeringMessages()
			if len(steeringMessages) > 0 {
				for _, msg := range steeringMessages {
					ch <- Event{Type: EventMessageStart, Message: msg}
					ch <- Event{Type: EventMessageEnd, Message: msg}
					a.mu.Lock()
					a.messages = append(a.messages, msg)
					a.context.Messages = append(a.context.Messages, msg)
					a.mu.Unlock()
				}
			}
		}

		// Continue loop - LLM will see tool results and decide next action
		// The loop will only exit when LLM returns a response without tool calls
		continue
	}

	ch <- Event{Type: EventError, Error: fmt.Errorf("max iterations (%d) exceeded", a.config.MaxIterations), StopReason: "max_iterations"}
	ch <- Event{Type: EventAgentEnd, Messages: func() []provider.Message {
		a.mu.RLock()
		defer a.mu.RUnlock()
		m := make([]provider.Message, len(a.messages))
		copy(m, a.messages)
		return m
	}()}
}

// executeToolCallsSequential executes tool calls one by one.
func (a *Agent) executeToolCallsSequential(ctx context.Context, toolCalls []provider.ToolCallBlock, ch chan<- Event) []provider.Message {
	var results []provider.Message

	for _, tc := range toolCalls {
		result := a.executeSingleToolCall(ctx, tc, ch)
		results = append(results, result)

		// Check for early termination
		if result.IsError {
			// Continue with other tools even if one fails
		}
	}

	return results
}

// executeToolCallsParallel executes tool calls concurrently.
func (a *Agent) executeToolCallsParallel(ctx context.Context, toolCalls []provider.ToolCallBlock, ch chan<- Event) []provider.Message {
	type toolResult struct {
		index  int
		result provider.Message
	}

	results := make([]provider.Message, len(toolCalls))
	resultCh := make(chan toolResult, len(toolCalls))

	// Start all tool calls concurrently
	for i, tc := range toolCalls {
		go func(index int, toolCall provider.ToolCallBlock) {
			result := a.executeSingleToolCall(ctx, toolCall, ch)
			resultCh <- toolResult{index: index, result: result}
		}(i, tc)
	}

	// Collect results
	for i := 0; i < len(toolCalls); i++ {
		tr := <-resultCh
		results[tr.index] = tr.result
	}

	return results
}

// executeSingleToolCall executes a single tool call.
func (a *Agent) executeSingleToolCall(ctx context.Context, tc provider.ToolCallBlock, ch chan<- Event) provider.Message {
	// Parse arguments
	var params map[string]any
	if len(tc.Arguments) > 0 {
		if err := json.Unmarshal(tc.Arguments, &params); err != nil {
			errMsg := fmt.Sprintf("parse tool arguments: %v", err)
			ch <- Event{
				Type:       EventToolExecutionEnd,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				ToolResult: errMsg,
				ToolError:  err,
			}
			return provider.NewToolResultMessage(tc.ID, tc.Name, errMsg, true)
		}
	}
	if params == nil {
		params = map[string]any{}
	}

	ch <- Event{
		Type:       EventToolExecutionStart,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		ToolArgs:   params,
	}

	// Find tool
	tool, ok := a.registry.Get(tc.Name)
	if !ok {
		errMsg := fmt.Sprintf("unknown tool: %s", tc.Name)
		ch <- Event{
			Type:       EventToolExecutionEnd,
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			ToolResult: errMsg,
			ToolError:  fmt.Errorf("%s", errMsg),
		}
		return provider.NewToolResultMessage(tc.ID, tc.Name, errMsg, true)
	}

	// Check if tool call should be blocked
	if a.config.BeforeToolCall != nil {
		blockResult := a.config.BeforeToolCall(BeforeToolCallContext{
			ToolCall: tc,
			Args:     params,
			Context:  a.context,
		})
		if blockResult != nil && blockResult.Block {
			reason := blockResult.Reason
			if reason == "" {
				reason = "Tool execution was blocked"
			}
			ch <- Event{
				Type:       EventToolExecutionEnd,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				ToolResult: reason,
				ToolError:  fmt.Errorf("%s", reason),
			}
			return provider.NewToolResultMessage(tc.ID, tc.Name, reason, true)
		}
	}

	// Check if tool needs user approval based on mode
	if a.NeedsApproval(tc.Name, params) {
		approved := false
		if a.config.ApprovalHandler != nil {
			approved = a.config.ApprovalHandler(tc.ID, tc.Name, params)
		} else {
			approved = a.RequestApproval(ch, tc.Name, params)
		}
		if !approved {
			reason := "Tool execution denied by user"
			ch <- Event{
				Type:       EventToolExecutionEnd,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				ToolResult: reason,
				ToolError:  fmt.Errorf("%s", reason),
			}
			return provider.NewToolResultMessage(tc.ID, tc.Name, reason, true)
		}
	}

	// Execute tool with timeout
	toolCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Inject agent ID and event channel into context for sub-agent tools
	toolCtx = ContextWithAgentID(toolCtx, a.id)
	toolCtx = ContextWithEventChan(toolCtx, ch)

	result, err := tool.Execute(toolCtx, params)
	isError := err != nil
	resultContent := result.Text
	resultContents := result.Contents
	resultDiff := result.Diff
	resultPlan := result.Plan
	if err != nil {
		resultContent = err.Error()
		resultContents = nil
		resultDiff = nil
		resultPlan = nil
	}

	// Apply after-tool-call hook
	if a.config.AfterToolCall != nil {
		afterResult := a.config.AfterToolCall(AfterToolCallContext{
			ToolCall: tc,
			Args:     params,
			Result: ToolCallResult{
				Content: resultContent,
				IsError: isError,
			},
			IsError: isError,
			Context: a.context,
		})
		if afterResult != nil {
			if afterResult.Content != "" {
				resultContent = afterResult.Content
			}
			isError = afterResult.IsError
			resultContents = nil
			resultPlan = nil
		}
	}

	if resultPlan != nil {
		ch <- Event{
			Type:       EventPlanUpdate,
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Plan:       resultPlan,
		}
	}

	ch <- Event{
		Type:       EventToolExecutionEnd,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		ToolResult: resultContent,
		ToolDiff:   resultDiff,
		ToolError:  err,
	}
	ch <- Event{
		Type:       EventToolResult,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		ToolResult: resultContent,
		ToolDiff:   resultDiff,
		ToolError:  err,
	}

	return provider.NewToolResultMessageWithContents(tc.ID, tc.Name, resultContent, resultContents, isError)
}

// GetMessages returns a copy of the current message history.
func (a *Agent) GetMessages() []provider.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]provider.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

// SetMessages replaces the message history.
func (a *Agent) SetMessages(msgs []provider.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = msgs
	a.context.Messages = msgs
}

// GetContext returns a copy of the current agent context.
func (a *Agent) GetContext() *AgentContext {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.context == nil {
		return nil
	}
	ctx := *a.context
	ctx.Messages = make([]provider.Message, len(a.context.Messages))
	copy(ctx.Messages, a.context.Messages)
	ctx.Tools = make([]provider.ToolDefinition, len(a.context.Tools))
	copy(ctx.Tools, a.context.Tools)
	return &ctx
}

// SetContext replaces the agent context.
func (a *Agent) SetContext(ctx *AgentContext) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = ctx
}

// GetContextUsage calculates and returns the current context usage.
func (a *Agent) GetContextUsage() *ctxpkg.ContextUsage {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.config.Model == nil {
		return nil
	}
	contextWindow := a.config.Model.ContextWindow
	if contextWindow <= 0 {
		return nil
	}

	tokens, _ := ctxpkg.EstimateContextTokens(a.messages)
	percent := float64(tokens) / float64(contextWindow) * 100

	return &ctxpkg.ContextUsage{
		Tokens:        tokens,
		ContextWindow: contextWindow,
		Percent:       &percent,
	}
}

// SetForceCompact marks the agent for forced compaction on the next turn.
// Called by /compact command in TUI and Gateway.
func (a *Agent) SetForceCompact() {
	atomic.StoreInt32(&a.forceCompact, 1)
}

// ShouldCompact checks if compaction should trigger.
// Returns true if context exceeds the threshold OR if forced via SetForceCompact.
func (a *Agent) ShouldCompact() bool {
	// Check force flag first (consumes it)
	if atomic.CompareAndSwapInt32(&a.forceCompact, 1, 0) {
		// Force compaction requested — still need a model and some messages
		a.mu.RLock()
		hasModel := a.config.Model != nil
		hasMsgs := len(a.messages) >= 2
		a.mu.RUnlock()
		if hasModel && hasMsgs {
			return true
		}
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.config.CompactionSettings.Enabled {
		return false
	}
	if a.config.Model == nil {
		return false
	}
	contextWindow := a.config.Model.ContextWindow
	if contextWindow <= 0 {
		return false
	}
	tokens, _ := ctxpkg.EstimateContextTokens(a.messages)
	return ctxpkg.ShouldCompact(tokens, contextWindow, a.config.CompactionSettings.ReserveTokens)
}

// Compact performs context compaction using Insert-then-Compress pattern (R4.1-R4.4).
// Uses the SAME system prompt and tools as the main conversation.
func (a *Agent) Compact(ctx context.Context, ch chan<- Event) error {
	if a.config.Model == nil {
		return fmt.Errorf("no model set for compaction")
	}

	ch <- Event{Type: EventCompactionStart}

	// Snapshot messages under lock
	a.mu.RLock()
	msgs := make([]provider.Message, len(a.messages))
	copy(msgs, a.messages)
	a.mu.RUnlock()

	// Get previous summary if exists
	previousSummary := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && strings.HasPrefix(msgs[i].Content, "## Goal") {
			previousSummary = msgs[i].Content
			break
		}
	}

	// Use Insert-then-Compress with the SAME system prompt and tools (R4.1)
	result, err := ctxpkg.Compact(ctx, msgs, a.config.Provider, a.config.Model,
		a.frozenSystemPrompt, a.frozenToolDefs,
		a.config.CompactionSettings, previousSummary)
	if err != nil {
		ch <- Event{Type: EventCompactionEnd, Error: err}
		return fmt.Errorf("compaction failed: %w", err)
	}

	// Replace messages with summary + kept messages
	// Mark summary as system_injected so cache markers skip it
	a.mu.Lock()
	summaryMsg := provider.NewSystemInjectedUserMessage(result.Summary)
	a.messages = append([]provider.Message{summaryMsg}, a.messages[result.FirstKeptIndex:]...)
	a.context.Messages = a.messages
	a.mu.Unlock()

	// Save compaction to session
	if a.config.Session != nil {
		if _, err := a.config.Session.AppendCompaction(result.Summary, "", result.TokensBefore); err != nil {
			ch <- Event{Type: EventCompactionEnd, Error: fmt.Errorf("save compaction to session: %w", err)}
			return fmt.Errorf("save compaction to session: %w", err)
		}
	}

	ch <- Event{
		Type: EventCompactionEnd,
		StatusMessage: func() string {
			usage := a.GetContextUsage()
			if usage != nil {
				return fmt.Sprintf("Context compacted: %d tokens -> %d tokens", result.TokensBefore, usage.Tokens)
			}
			return fmt.Sprintf("Context compacted: %d tokens", result.TokensBefore)
		}(),
	}

	return nil
}

// NeedsApproval checks if a tool call needs user approval based on the current mode.
func (a *Agent) NeedsApproval(toolName string, args map[string]any) bool {
	if (toolName == "write" || toolName == "edit") && a.config.Mode == "agent" {
		return a.config.Settings != nil &&
			a.config.Settings.Approval.ConfirmBeforeWrite != nil &&
			*a.config.Settings.Approval.ConfirmBeforeWrite
	}
	if toolName != "bash" {
		return false
	}
	if a.isBashBlacklisted(args) {
		return true
	}
	switch a.config.Mode {
	case "plan":
		// Plan mode: no tools should be executed (read-only tools don't need approval)
		return false
	case "agent":
		// Agent mode: only whitelisted bash can skip approval.
		return !a.isBashWhitelisted(args)
	case "yolo":
		// YOLO mode: allow bash unless explicitly blacklisted above.
		return false
	default:
		return false
	}
}

func (a *Agent) isBashWhitelisted(args map[string]any) bool {
	if a.config.Settings == nil {
		return false
	}
	command, ok := args["command"].(string)
	if !ok {
		return false
	}
	for _, prefix := range a.config.Settings.Approval.BashWhitelist {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

func (a *Agent) isBashBlacklisted(args map[string]any) bool {
	if a.config.Settings == nil {
		return false
	}
	command, ok := args["command"].(string)
	if !ok {
		return false
	}
	for _, prefix := range a.config.Settings.Approval.BashBlacklist {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

// RequestApproval sends an approval request and waits for the user's response.
func (a *Agent) RequestApproval(ch chan<- Event, toolName string, args map[string]any) bool {
	a.approvalMu.Lock()
	a.approvalCounter++
	approvalID := fmt.Sprintf("approval-%d", a.approvalCounter)
	responseCh := make(chan bool, 1)
	a.pendingApprovals[approvalID] = responseCh
	a.approvalMu.Unlock()

	// Send approval request event
	ch <- Event{
		Type:         EventToolApprovalRequest,
		ApprovalID:   approvalID,
		ApprovalTool: toolName,
		ApprovalArgs: args,
	}

	// Wait for response or abort
	select {
	case approved := <-responseCh:
		return approved
	case <-a.abort:
		a.approvalMu.Lock()
		delete(a.pendingApprovals, approvalID)
		a.approvalMu.Unlock()
		return false
	}
}

// HandleApprovalResponse processes the user's approval response.
func (a *Agent) HandleApprovalResponse(approvalID string, approved bool) {
	a.approvalMu.Lock()
	defer a.approvalMu.Unlock()

	if ch, ok := a.pendingApprovals[approvalID]; ok {
		ch <- approved
		delete(a.pendingApprovals, approvalID)
	}
}
