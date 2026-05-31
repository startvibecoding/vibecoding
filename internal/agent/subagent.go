package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

// SubAgentSpawnTool creates and starts a sub-agent.
type SubAgentSpawnTool struct {
	manager *AgentManager
}

// NewSubAgentSpawnTool creates a new subagent_spawn tool.
func NewSubAgentSpawnTool(m *AgentManager) *SubAgentSpawnTool {
	return &SubAgentSpawnTool{manager: m}
}

func (t *SubAgentSpawnTool) Name() string         { return "subagent_spawn" }
func (t *SubAgentSpawnTool) Description() string   { return "Create and start a bounded sub-agent task. Returns a handle for status/result polling." }
func (t *SubAgentSpawnTool) PromptSnippet() string { return "Create a bounded sub-agent task for independent work" }
func (t *SubAgentSpawnTool) PromptGuidelines() []string {
	return []string{
		"Use subagent_spawn only for independent subtasks with clear scope, expected output, and stop conditions",
		"Spawn multiple sub-agents in parallel for independent investigation or review work, then reconcile their results in the main agent",
		"Use subagent_status to poll results and verify important claims before acting on them",
		"Use subagent_destroy to clean up finished sub-agents",
	}
}

func (t *SubAgentSpawnTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task": {"type": "string", "description": "Focused task for the sub-agent, including scope, relevant paths/context, expected artifact, and stop conditions"},
			"mode": {"type": "string", "enum": ["plan", "agent", "yolo"], "default": "agent", "description": "Agent mode"},
			"work_dir": {"type": "string", "description": "Working directory for the sub-agent (defaults to current)"},
			"tools": {"type": "array", "items": {"type": "string"}, "description": "Allowed tools (empty = all)"},
			"max_iterations": {"type": "integer", "default": 50, "description": "Maximum iterations"},
			"system_prompt_extra": {"type": "string", "description": "Extra context for the sub-agent"}
		},
		"required": ["task"]
	}`)
}

func (t *SubAgentSpawnTool) Execute(ctx context.Context, params map[string]any) (tools.ToolResult, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return tools.ToolResult{}, fmt.Errorf("task is required")
	}

	mode, _ := params["mode"].(string)
	if mode == "" {
		mode = "agent"
	}

	workDir, _ := params["work_dir"].(string)

	maxIter := 50
	if v, ok := params["max_iterations"].(float64); ok && v > 0 {
		maxIter = int(v)
	}

	extra, _ := params["system_prompt_extra"].(string)

	var toolFilter []string
	if ts, ok := params["tools"].([]any); ok {
		for _, tt := range ts {
			if s, ok := tt.(string); ok {
				toolFilter = append(toolFilter, s)
			}
		}
	}

	// Extract parent agent ID from context (injected by executeTool)
	parentID, _ := AgentIDFromContext(ctx)

	// Extract parent's event channel from context (injected by executeTool)
	parentEventCh, _ := EventChanFromContext(ctx)

	// Create approval forwarder that bridges sub-agent approval to parent
	var approvalHandler func(toolCallID, toolName string, args map[string]any) bool
	if parentEventCh != nil {
		approvalHandler = newApprovalForwarder(parentID, parentEventCh)
	}

	a, err := t.manager.Create(AgentOptions{
		ParentID:          parentID,
		Mode:              mode,
		WorkDir:           workDir,
		Tools:             toolFilter,
		SystemPromptExtra: extra,
		MaxIterations:     maxIter,
		ApprovalHandler:   approvalHandler,
	})
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("create sub-agent: %w", err)
	}
	t.manager.MarkRunning(a.ID())

	// Apply per-agent timeout from default policy
	policy := DefaultSubAgentPolicy()
	runCtx, cancel := context.WithTimeout(context.Background(), policy.TimeoutPerAgent)

	// Start the sub-agent asynchronously, forward events to parent
	go func() {
		defer cancel()
		ch := a.Run(runCtx, buildSubAgentTask(task))
		for e := range ch {
			// Forward approval events to parent so the UI can handle them
			if e.Type == agentpkg.EventToolApprovalRequest && parentEventCh != nil {
				parentEventCh <- Event{
					Type:         EventToolApprovalRequest,
					AgentID:      a.ID(),
					ApprovalID:   e.ApprovalID,
					ApprovalTool: e.ApprovalTool,
					ApprovalArgs: e.ApprovalArgs,
				}
			}
			switch e.Type {
			case agentpkg.EventDone:
				t.manager.MarkDone(a.ID(), lastAssistantResponse(a))
			case agentpkg.EventError:
				t.manager.MarkError(a.ID(), e.Error)
			}
		}
		if runCtx.Err() != nil {
			t.manager.MarkError(a.ID(), runCtx.Err())
		}
	}()

	result := map[string]any{
		"handle":  string(a.ID()),
		"status":  "running",
		"timeout": policy.TimeoutPerAgent.String(),
	}
	data, _ := json.Marshal(result)
	return tools.NewTextToolResult(string(data)), nil
}

// newApprovalForwarder creates an ApprovalHandler that forwards sub-agent approval
// requests to the parent agent's event channel and waits for a response.
func newApprovalForwarder(parentID agentpkg.AgentID, parentEventCh chan<- Event) func(toolCallID, toolName string, args map[string]any) bool {
	var mu sync.Mutex
	counter := int64(0)
	pending := make(map[string]chan bool)

	return func(toolCallID, toolName string, args map[string]any) bool {
		mu.Lock()
		counter++
		approvalID := fmt.Sprintf("sub-approval-%d", counter)
		responseCh := make(chan bool, 1)
		pending[approvalID] = responseCh
		mu.Unlock()

		// Forward approval request to parent's event channel
		parentEventCh <- Event{
			Type:         EventToolApprovalRequest,
			AgentID:      parentID,
			ApprovalID:   approvalID,
			ApprovalTool: toolName,
			ApprovalArgs: args,
		}

		// Wait for response (the parent TUI should call HandleSubAgentApprovalResponse)
		approved := <-responseCh

		mu.Lock()
		delete(pending, approvalID)
		mu.Unlock()

		return approved
	}
}

// SubAgentStatusTool queries sub-agent status and results.
type SubAgentStatusTool struct {
	manager *AgentManager
}

func NewSubAgentStatusTool(m *AgentManager) *SubAgentStatusTool {
	return &SubAgentStatusTool{manager: m}
}

func (t *SubAgentStatusTool) Name() string         { return "subagent_status" }
func (t *SubAgentStatusTool) Description() string   { return "Query the status and results of a sub-agent." }
func (t *SubAgentStatusTool) PromptSnippet() string { return "Check sub-agent status and get results" }
func (t *SubAgentStatusTool) PromptGuidelines() []string { return nil }

func (t *SubAgentStatusTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"handle": {"type": "string", "description": "The sub-agent handle ID"}
		},
		"required": ["handle"]
	}`)
}

func (t *SubAgentStatusTool) Execute(ctx context.Context, params map[string]any) (tools.ToolResult, error) {
	handle, _ := params["handle"].(string)
	if handle == "" {
		return tools.ToolResult{}, fmt.Errorf("handle is required")
	}

	a, ok := t.manager.Get(agentpkg.AgentID(handle))
	if !ok {
		return tools.ToolResult{}, fmt.Errorf("sub-agent %q not found", handle)
	}

	messages := a.GetMessages()
	st, _ := t.manager.Status(agentpkg.AgentID(handle))
	status := st.State
	if status == "" {
		status = "unknown"
	}
	lastResponse := st.Result
	if lastResponse == "" {
		lastResponse = lastAssistantResponse(a)
	}

	result := map[string]any{
		"handle":        handle,
		"status":        status,
		"message_count": len(messages),
	}
	if lastResponse != "" {
		result["last_response"] = lastResponse
	}
	if st.Error != "" {
		result["error"] = st.Error
	}
	if !st.UpdatedAt.IsZero() {
		result["updated_at"] = st.UpdatedAt.Format(time.RFC3339)
	}

	data, _ := json.Marshal(result)
	return tools.NewTextToolResult(string(data)), nil
}

// SubAgentSendTool sends a follow-up message to a running sub-agent.
type SubAgentSendTool struct {
	manager *AgentManager
}

func NewSubAgentSendTool(m *AgentManager) *SubAgentSendTool {
	return &SubAgentSendTool{manager: m}
}

func (t *SubAgentSendTool) Name() string         { return "subagent_send" }
func (t *SubAgentSendTool) Description() string   { return "Send a follow-up message to a running sub-agent." }
func (t *SubAgentSendTool) PromptSnippet() string { return "Send follow-up instructions to a sub-agent" }
func (t *SubAgentSendTool) PromptGuidelines() []string { return nil }

func (t *SubAgentSendTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"handle": {"type": "string", "description": "The sub-agent handle ID"},
			"message": {"type": "string", "description": "The follow-up message"}
		},
		"required": ["handle", "message"]
	}`)
}

func (t *SubAgentSendTool) Execute(ctx context.Context, params map[string]any) (tools.ToolResult, error) {
	handle, _ := params["handle"].(string)
	message, _ := params["message"].(string)
	if handle == "" || message == "" {
		return tools.ToolResult{}, fmt.Errorf("handle and message are required")
	}

	a, ok := t.manager.Get(agentpkg.AgentID(handle))
	if !ok {
		return tools.ToolResult{}, fmt.Errorf("sub-agent %q not found", handle)
	}

	// Apply per-agent timeout for follow-up messages too
	policy := DefaultSubAgentPolicy()
	runCtx, cancel := context.WithTimeout(context.Background(), policy.TimeoutPerAgent)
	t.manager.MarkRunning(a.ID())

	// Extract parent's event channel for approval forwarding
	parentEventCh, _ := EventChanFromContext(ctx)

	go func() {
		defer cancel()
		ch := a.Run(runCtx, message)
		for e := range ch {
			// Forward approval events to parent
			if e.Type == agentpkg.EventToolApprovalRequest && parentEventCh != nil {
				parentEventCh <- Event{
					Type:         EventToolApprovalRequest,
					AgentID:      a.ID(),
					ApprovalID:   e.ApprovalID,
					ApprovalTool: e.ApprovalTool,
					ApprovalArgs: e.ApprovalArgs,
				}
			}
			switch e.Type {
			case agentpkg.EventDone:
				t.manager.MarkDone(a.ID(), lastAssistantResponse(a))
			case agentpkg.EventError:
				t.manager.MarkError(a.ID(), e.Error)
			}
		}
		if runCtx.Err() != nil {
			t.manager.MarkError(a.ID(), runCtx.Err())
		}
	}()

	return tools.NewTextToolResult(fmt.Sprintf(`{"handle":%q,"status":"message_sent"}`, handle)), nil
}

func buildSubAgentTask(task string) string {
	task = strings.TrimSpace(task)
	return fmt.Sprintf(`Delegated task:
%s

Return the artifact using this format:
Result:
Evidence:
Changes:
Risks:
`, task)
}

func lastAssistantResponse(a agentpkg.Agent) string {
	messages := a.GetMessages()
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == agentpkg.RoleAssistant {
			if messages[i].Content != "" {
				return messages[i].Content
			}
			var sb strings.Builder
			for _, block := range messages[i].Contents {
				if block.Type == "text" && block.Text != "" {
					sb.WriteString(block.Text)
				}
			}
			return sb.String()
		}
	}
	return ""
}

// SubAgentDestroyTool destroys a sub-agent and releases resources.
type SubAgentDestroyTool struct {
	manager *AgentManager
}

func NewSubAgentDestroyTool(m *AgentManager) *SubAgentDestroyTool {
	return &SubAgentDestroyTool{manager: m}
}

func (t *SubAgentDestroyTool) Name() string         { return "subagent_destroy" }
func (t *SubAgentDestroyTool) Description() string   { return "Destroy a sub-agent and release resources." }
func (t *SubAgentDestroyTool) PromptSnippet() string { return "Destroy a finished sub-agent" }
func (t *SubAgentDestroyTool) PromptGuidelines() []string { return nil }

func (t *SubAgentDestroyTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"handle": {"type": "string", "description": "The sub-agent handle ID"}
		},
		"required": ["handle"]
	}`)
}

func (t *SubAgentDestroyTool) Execute(ctx context.Context, params map[string]any) (tools.ToolResult, error) {
	handle, _ := params["handle"].(string)
	if handle == "" {
		return tools.ToolResult{}, fmt.Errorf("handle is required")
	}

	if err := t.manager.Destroy(agentpkg.AgentID(handle)); err != nil {
		return tools.ToolResult{}, fmt.Errorf("destroy sub-agent: %w", err)
	}

	return tools.NewTextToolResult(fmt.Sprintf(`{"handle":%q,"status":"destroyed"}`, handle)), nil
}

// SubAgentPolicy defines security constraints for sub-agents.
type SubAgentPolicy struct {
	MaxChildren     int           // Maximum number of sub-agents (default 5)
	AllowedModes    []string      // Allowed modes for sub-agents (default ["agent"])
	InheritSandbox  bool          // Inherit parent's sandbox (default true)
	TimeoutPerAgent time.Duration // Per-agent timeout (default 10min)
	TotalTimeout    time.Duration // Total timeout for all sub-agents (default 30min)
}

// DefaultSubAgentPolicy returns the default policy.
func DefaultSubAgentPolicy() SubAgentPolicy {
	return SubAgentPolicy{
		MaxChildren:     5,
		AllowedModes:    []string{"agent"},
		InheritSandbox:  true,
		TimeoutPerAgent: 10 * time.Minute,
		TotalTimeout:    30 * time.Minute,
	}
}

// Validate checks if a sub-agent creation request is allowed.
func (p *SubAgentPolicy) Validate(parentID string, mode string, currentChildCount int) error {
	if parentID == "" {
		return nil
	}
	if currentChildCount >= p.MaxChildren {
		return fmt.Errorf("maximum %d sub-agents allowed", p.MaxChildren)
	}
	allowed := false
	for _, m := range p.AllowedModes {
		if m == mode {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("mode %q is not allowed for sub-agents; allowed: %v", mode, p.AllowedModes)
	}
	return nil
}
