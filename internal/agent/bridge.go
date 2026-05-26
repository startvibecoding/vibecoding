package agent

import (
	"context"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
)

// init registers the internal builder function with the public agent package.
func init() {
	agentpkg.SetBuilderFunc(buildFromPublicBuilder)
}

// buildFromPublicBuilder converts a public Builder to an internal Agent.
func buildFromPublicBuilder(b *agentpkg.Builder) (agentpkg.Agent, error) {
	// This will be fully implemented in Phase 3 (AgentFactory).
	// For now, it's a placeholder that enables the Builder pattern.
	return nil, nil
}

// --- Type conversion helpers ---

// MessageToPublic converts an internal provider.Message to a public agent.Message.
func MessageToPublic(m provider.Message) agentpkg.Message {
	msg := agentpkg.Message{
		Role:            agentpkg.Role(m.Role),
		Content:         m.Content,
		IsError:         m.IsError,
		SystemInjected: m.SystemInjected,
		ToolCallID:      m.ToolCallID,
		ToolName:        m.ToolName,
	}
	if m.Usage != nil {
		msg.Usage = &agentpkg.Usage{
			InputTokens:  m.Usage.Input,
			OutputTokens: m.Usage.Output,
			CacheRead:    m.Usage.CacheRead,
			CacheWrite:   m.Usage.CacheWrite,
			TotalTokens:  m.Usage.TotalTokens,
		}
	}
	for _, cb := range m.Contents {
		msg.Contents = append(msg.Contents, ContentBlockToPublic(cb))
	}
	return msg
}

// MessageFromPublic converts a public agent.Message to an internal provider.Message.
func MessageFromPublic(m agentpkg.Message) provider.Message {
	msg := provider.Message{
		Role:            string(m.Role),
		Content:         m.Content,
		IsError:         m.IsError,
		SystemInjected: m.SystemInjected,
		ToolCallID:      m.ToolCallID,
		ToolName:        m.ToolName,
	}
	if m.Usage != nil {
		msg.Usage = &provider.Usage{
			Input:       m.Usage.InputTokens,
			Output:      m.Usage.OutputTokens,
			CacheRead:   m.Usage.CacheRead,
			CacheWrite:  m.Usage.CacheWrite,
			TotalTokens: m.Usage.TotalTokens,
		}
	}
	for _, cb := range m.Contents {
		msg.Contents = append(msg.Contents, ContentBlockFromPublic(cb))
	}
	return msg
}

// ContentBlockToPublic converts an internal provider.ContentBlock to public.
func ContentBlockToPublic(cb provider.ContentBlock) agentpkg.ContentBlock {
	pub := agentpkg.ContentBlock{
		Type:      cb.Type,
		Text:      cb.Text,
		Thinking:  cb.Thinking,
		Signature: cb.Signature,
	}
	if cb.ToolCall != nil {
		pub.ToolCall = &agentpkg.ToolCallBlock{
			ID:        cb.ToolCall.ID,
			Name:      cb.ToolCall.Name,
			Arguments: cb.ToolCall.Arguments,
		}
	}
	if cb.Image != nil {
		pub.Image = &agentpkg.ImageContent{
			MimeType: cb.Image.MimeType,
			Data:     cb.Image.Data,
		}
	}
	if cb.CacheControl != nil {
		pub.CacheControl = &agentpkg.CacheControl{Type: cb.CacheControl.Type}
	}
	return pub
}

// ContentBlockFromPublic converts a public agent.ContentBlock to internal.
func ContentBlockFromPublic(cb agentpkg.ContentBlock) provider.ContentBlock {
	internal := provider.ContentBlock{
		Type:      cb.Type,
		Text:      cb.Text,
		Thinking:  cb.Thinking,
		Signature: cb.Signature,
	}
	if cb.ToolCall != nil {
		internal.ToolCall = &provider.ToolCallBlock{
			ID:        cb.ToolCall.ID,
			Name:      cb.ToolCall.Name,
			Arguments: cb.ToolCall.Arguments,
		}
	}
	if cb.Image != nil {
		internal.Image = &provider.ImageContent{
			MimeType: cb.Image.MimeType,
			Data:     cb.Image.Data,
		}
	}
	if cb.CacheControl != nil {
		internal.CacheControl = &provider.CacheControl{Type: cb.CacheControl.Type}
	}
	return internal
}

// MessagesToPublic converts a slice of internal messages to public.
func MessagesToPublic(msgs []provider.Message) []agentpkg.Message {
	result := make([]agentpkg.Message, len(msgs))
	for i, m := range msgs {
		result[i] = MessageToPublic(m)
	}
	return result
}

// MessagesFromPublic converts a slice of public messages to internal.
func MessagesFromPublic(msgs []agentpkg.Message) []provider.Message {
	result := make([]provider.Message, len(msgs))
	for i, m := range msgs {
		result[i] = MessageFromPublic(m)
	}
	return result
}

// ContextUsageToPublic converts internal context usage to public.
func ContextUsageToPublic(u *ctxpkg.ContextUsage) *agentpkg.ContextUsage {
	if u == nil {
		return nil
	}
	return &agentpkg.ContextUsage{
		Tokens:        u.Tokens,
		ContextWindow: u.ContextWindow,
		Percent:       u.Percent,
	}
}

// EventToPublic converts an internal Event to a public agent.Event.
func EventToPublic(e Event) agentpkg.Event {
	return agentpkg.Event{
		AgentID:        agentpkg.AgentID(e.AgentID),
		Type:           agentpkg.EventType(e.Type),
		TextDelta:      e.TextDelta,
		ThinkDelta:     e.ThinkDelta,
		ToolCallID:     e.ToolCallID,
		ToolName:       e.ToolName,
		ToolArgs:       e.ToolArgs,
		ToolResult:     e.ToolResult,
		StatusMessage:  e.StatusMessage,
		Done:           e.Done,
		StopReason:     e.StopReason,
		Error:          e.Error,
		ApprovalID:     e.ApprovalID,
		ApprovalTool:   e.ApprovalTool,
		ApprovalArgs:   e.ApprovalArgs,
		ApprovalResult: e.ApprovalResult,
	}
}

// WrapEventChan wraps an internal event channel into a public event channel.
func WrapEventChan(in <-chan Event) <-chan agentpkg.Event {
	out := make(chan agentpkg.Event, 100)
	go func() {
		defer close(out)
		for e := range in {
			out <- EventToPublic(e)
		}
	}()
	return out
}

// --- AgentAdapter wraps internal Agent to satisfy public agent.Agent interface ---

// AgentAdapter wraps an internal *Agent and satisfies the public agent.Agent interface.
type AgentAdapter struct {
	inner *Agent
}

// NewAgentAdapter creates an adapter that wraps an internal Agent.
func NewAgentAdapter(a *Agent) *AgentAdapter {
	return &AgentAdapter{inner: a}
}

func (a *AgentAdapter) ID() agentpkg.AgentID                                    { return a.inner.id }
func (a *AgentAdapter) ParentID() agentpkg.AgentID                              { return a.inner.parentID }
func (a *AgentAdapter) Abort()                                                  { a.inner.Abort() }
func (a *AgentAdapter) HandleApprovalResponse(id string, approved bool)         { a.inner.HandleApprovalResponse(id, approved) }
func (a *AgentAdapter) Run(ctx context.Context, userMsg string) <-chan agentpkg.Event { return WrapEventChan(a.inner.Run(ctx, userMsg)) }
func (a *AgentAdapter) RunWithMessages(ctx context.Context, msgs []agentpkg.Message) <-chan agentpkg.Event { return WrapEventChan(a.inner.RunWithMessages(ctx, MessagesFromPublic(msgs))) }
func (a *AgentAdapter) GetMessages() []agentpkg.Message                         { return MessagesToPublic(a.inner.GetMessages()) }
func (a *AgentAdapter) SetMessages(msgs []agentpkg.Message)                     { a.inner.SetMessages(MessagesFromPublic(msgs)) }
func (a *AgentAdapter) GetContextUsage() *agentpkg.ContextUsage                 { return ContextUsageToPublic(a.inner.GetContextUsage()) }
func (a *AgentAdapter) LoadHistoryMessages(msgs []agentpkg.Message)             { a.inner.LoadHistoryMessages(MessagesFromPublic(msgs)) }

func (a *AgentAdapter) GetContext() *agentpkg.AgentContext {
	x := a.inner.GetContext()
	if x == nil {
		return nil
	}
	return &agentpkg.AgentContext{
		SystemPrompt: x.SystemPrompt,
		Messages:     MessagesToPublic(x.Messages),
	}
}

func (a *AgentAdapter) SetContext(ctx *agentpkg.AgentContext) {
	a.inner.SetContext(&AgentContext{
		SystemPrompt: ctx.SystemPrompt,
		Messages:     MessagesFromPublic(ctx.Messages),
	})
}
