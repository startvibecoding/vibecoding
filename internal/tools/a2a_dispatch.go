package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// A2ADispatcher is the interface needed by the a2a_dispatch tool.
// It is satisfied by a2a.A2AManager.
type A2ADispatcher interface {
	List() []AgentEntry
	Dispatch(ctx context.Context, name, message string) (string, error)
}

// AgentEntry is a minimal view of a remote A2A agent.
type AgentEntry struct {
	Name string
	URL  string
}

// A2ADispatchTool sends tasks to registered remote A2A agents.
type A2ADispatchTool struct {
	dispatcher A2ADispatcher
}

// NewA2ADispatchTool creates a new A2A dispatch tool.
func NewA2ADispatchTool(dispatcher A2ADispatcher) *A2ADispatchTool {
	return &A2ADispatchTool{dispatcher: dispatcher}
}

func (t *A2ADispatchTool) Name() string {
	return "a2a_dispatch"
}

func (t *A2ADispatchTool) Description() string {
	return "Send a task to a registered remote A2A agent. The agent will execute the task and return the result."
}

func (t *A2ADispatchTool) PromptSnippet() string {
	return "Dispatch tasks to remote A2A agents"
}

func (t *A2ADispatchTool) PromptGuidelines() []string {
	return []string{
		"Use a2a_dispatch to delegate tasks to specialized remote agents.",
		"Each agent has specific capabilities described in its Agent Card.",
		"Long-running tasks may take up to 5 minutes to complete.",
	}
}

func (t *A2ADispatchTool) Parameters() json.RawMessage {
	// Build enum from registered agents
	agents := t.dispatcher.List()
	agentNames := make([]string, 0, len(agents))
	for _, a := range agents {
		agentNames = append(agentNames, a.Name)
	}

	// Build agent descriptions for the LLM
	agentDesc := "Available agents:\n"
	for _, a := range agents {
		agentDesc += fmt.Sprintf("  - %s (%s)\n", a.Name, a.URL)
	}

	return json.RawMessage(fmt.Sprintf(`{
		"type": "object",
		"properties": {
			"agent_name": {
				"type": "string",
				"description": %q,
				"enum": %s
			},
			"message": {
				"type": "string",
				"description": "The task message to send to the agent"
			}
		},
		"required": ["agent_name", "message"]
	}`, agentDesc, mustMarshalJSON(agentNames)))
}

func (t *A2ADispatchTool) Execute(ctx context.Context, params map[string]any) (ToolResult, error) {
	agentName, ok := params["agent_name"].(string)
	if !ok || agentName == "" {
		return ToolResult{}, fmt.Errorf("missing required parameter: agent_name")
	}

	message, ok := params["message"].(string)
	if !ok || message == "" {
		return ToolResult{}, fmt.Errorf("missing required parameter: message")
	}

	result, err := t.dispatcher.Dispatch(ctx, agentName, message)
	if err != nil {
		return ToolResult{}, err
	}

	return NewTextToolResult(result), nil
}

func mustMarshalJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
