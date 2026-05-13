package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/fuckvibecoding/vibecoding/internal/config"
	"github.com/fuckvibecoding/vibecoding/internal/provider"
	"github.com/fuckvibecoding/vibecoding/internal/sandbox"
	"github.com/fuckvibecoding/vibecoding/internal/session"
	"github.com/fuckvibecoding/vibecoding/internal/tools"
)

// Config holds the agent configuration.
type Config struct {
	Provider      provider.Provider
	Model         *provider.Model
	Mode          string // "plan", "agent", "yolo"
	ThinkingLevel provider.ThinkingLevel
	MaxTokens     int
	SandboxMgr    *sandbox.Manager
	Settings      *config.Settings
	Session       *session.Manager
	ExtraContext  string // extra context from files and skills
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
	config      AgentLoopConfig
	registry    *tools.Registry
	context     *AgentContext
	abort       chan struct{}
	abortOnce   sync.Once
	messages    []provider.Message
	isStreaming bool
}

// New creates a new agent.
func New(cfg Config, registry *tools.Registry) *Agent {
	loopConfig := AgentLoopConfig{
		Config:            cfg,
		ToolExecutionMode: "parallel",
		MaxIterations:     50,
	}

	return &Agent{
		config:   loopConfig,
		registry: registry,
		abort:    make(chan struct{}),
		context: &AgentContext{
			Messages: make([]provider.Message, 0),
		},
	}
}

// NewWithLoopConfig creates a new agent with custom loop configuration.
func NewWithLoopConfig(cfg AgentLoopConfig, registry *tools.Registry) *Agent {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 50
	}
	if cfg.ToolExecutionMode == "" {
		cfg.ToolExecutionMode = "parallel"
	}

	return &Agent{
		config:   cfg,
		registry: registry,
		abort:    make(chan struct{}),
		context: &AgentContext{
			Messages: make([]provider.Message, 0),
		},
	}
}

// Abort signals the agent to stop processing.
func (a *Agent) Abort() {
	a.abortOnce.Do(func() {
		close(a.abort)
		a.abort = make(chan struct{})
	})
}

// Run processes a user message and streams events back.
func (a *Agent) Run(ctx context.Context, userMsg string) <-chan Event {
	ch := make(chan Event, 100)

	go func() {
		defer close(ch)

		// Add user message to conversation
		msg := provider.NewUserMessage(userMsg)
		a.messages = append(a.messages, msg)
		a.context.Messages = append(a.context.Messages, msg)

		// Save to session
		if a.config.Session != nil {
			a.config.Session.AppendMessage(msg)
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
		a.messages = messages
		a.context.Messages = messages
		a.loop(ctx, ch)
	}()

	return ch
}

// loop runs the main agent loop: send message -> receive response -> execute tools -> repeat.
func (a *Agent) loop(ctx context.Context, ch chan<- Event) {
	ch <- Event{Type: EventAgentStart}

	for i := 0; i < a.config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			ch <- Event{Type: EventError, Error: ctx.Err(), StopReason: "aborted"}
			ch <- Event{Type: EventAgentEnd, Messages: a.messages}
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

		// Build system prompt
		toolNames := make([]string, 0)
		for _, t := range a.registry.ModeTools(a.config.Mode) {
			toolNames = append(toolNames, t.Name)
		}
		systemPrompt := BuildSystemPrompt(a.config.Mode, toolNames, a.registry.GetWorkDir(), a.config.ExtraContext)
		a.context.SystemPrompt = systemPrompt

		// Get tool definitions for current mode
		toolDefs := a.registry.ModeTools(a.config.Mode)
		a.context.Tools = toolDefs

		// Chat request
		params := provider.ChatParams{
			Messages:      a.context.Messages,
			Tools:         toolDefs,
			SystemPrompt:  systemPrompt,
			ThinkingLevel: a.config.ThinkingLevel,
			MaxTokens:     a.config.MaxTokens,
			Abort:         a.abort,
		}

		streamCh := a.config.Provider.Chat(ctx, params)

		var (
			textContent  string
			thinkContent string
			toolCalls    []provider.ToolCallBlock
			usage        *provider.Usage
			stopReason   string
			streamErr    error
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
			case provider.StreamToolCall:
				if event.ToolCall != nil {
					toolCalls = append(toolCalls, *event.ToolCall)
					ch <- Event{Type: EventToolCall, ToolCall: event.ToolCall}
				}
			case provider.StreamUsage:
				usage = event.Usage
				ch <- Event{Type: EventUsage, Usage: event.Usage}
			case provider.StreamDone:
				stopReason = event.StopReason
			case provider.StreamError:
				streamErr = event.Error
				stopReason = event.StopReason
			}
		}

		if streamErr != nil {
			ch <- Event{Type: EventError, Error: streamErr, StopReason: stopReason}
			ch <- Event{Type: EventAgentEnd, Messages: a.messages}
			return
		}

		// Build assistant message
		var contents []provider.ContentBlock
		if thinkContent != "" {
			contents = append(contents, provider.ContentBlock{
				Type:     "thinking",
				Thinking: thinkContent,
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
		a.messages = append(a.messages, assistantMsg)
		a.context.Messages = append(a.context.Messages, assistantMsg)

		// Save to session
		if a.config.Session != nil {
			a.config.Session.AppendMessage(assistantMsg)
		}

		// Calculate cost
		if usage != nil && a.config.Model != nil {
			usage.CalculateCost(a.config.Model)
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			ch <- Event{Type: EventTurnEnd, TurnMessage: assistantMsg}
			ch <- Event{Type: EventDone, StopReason: stopReason}
			ch <- Event{Type: EventAgentEnd, Messages: a.messages}
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
		for _, result := range toolResults {
			a.messages = append(a.messages, result)
			a.context.Messages = append(a.context.Messages, result)
		}

		ch <- Event{Type: EventTurnEnd, TurnMessage: assistantMsg, TurnToolResults: toolResults}

		// Check if we should stop after this turn
		if a.config.ShouldStopAfterTurn != nil {
			ctx := ShouldStopAfterTurnContext{
				Message:     assistantMsg,
				ToolResults: toolResults,
				Context:     a.context,
				NewMessages: a.messages,
			}
			if a.config.ShouldStopAfterTurn(ctx) {
				ch <- Event{Type: EventDone, StopReason: "should_stop"}
				ch <- Event{Type: EventAgentEnd, Messages: a.messages}
				return
			}
		}

		// Prepare next turn
		if a.config.PrepareNextTurn != nil {
			ctx := PrepareNextTurnContext{
				ShouldStopAfterTurnContext: ShouldStopAfterTurnContext{
					Message:     assistantMsg,
					ToolResults: toolResults,
					Context:     a.context,
					NewMessages: a.messages,
				},
			}
			update := a.config.PrepareNextTurn(ctx)
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
					a.messages = append(a.messages, msg)
					a.context.Messages = append(a.context.Messages, msg)
				}
			}
		}

		// Continue loop - LLM will see tool results and decide next action
		// The loop will only exit when LLM returns a response without tool calls
		continue
	}

	ch <- Event{Type: EventError, Error: fmt.Errorf("max iterations (%d) exceeded", a.config.MaxIterations), StopReason: "max_iterations"}
	ch <- Event{Type: EventAgentEnd, Messages: a.messages}
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
	ch <- Event{
		Type:       EventToolExecutionStart,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		ToolArgs:   map[string]any{},
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

	// Execute tool with timeout
	toolCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	result, err := tool.Execute(toolCtx, params)
	isError := err != nil
	resultContent := result
	if err != nil {
		resultContent = err.Error()
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
		}
	}

	ch <- Event{
		Type:       EventToolExecutionEnd,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		ToolResult: resultContent,
		ToolError:  err,
	}
	ch <- Event{
		Type:       EventToolResult,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		ToolResult: resultContent,
		ToolError:  err,
	}

	return provider.NewToolResultMessage(tc.ID, tc.Name, resultContent, isError)
}

// GetMessages returns the current message history.
func (a *Agent) GetMessages() []provider.Message {
	return a.messages
}

// SetMessages replaces the message history.
func (a *Agent) SetMessages(msgs []provider.Message) {
	a.messages = msgs
	a.context.Messages = msgs
}

// GetContext returns the current agent context.
func (a *Agent) GetContext() *AgentContext {
	return a.context
}

// SetContext replaces the agent context.
func (a *Agent) SetContext(ctx *AgentContext) {
	a.context = ctx
}
