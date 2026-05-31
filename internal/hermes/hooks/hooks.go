// Package hooks implements shell hook scripts for Hermes mode.
// Hooks are external scripts called before/after tool execution,
// communicating via JSON on stdin/stdout.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Manager manages pre/post tool call hooks.
type Manager struct {
	preToolCall  string // path to pre_tool_call script
	postToolCall string // path to post_tool_call script
	timeout      time.Duration
}

// NewManager creates a hooks manager.
func NewManager(preToolCall, postToolCall string) *Manager {
	return &Manager{
		preToolCall:  preToolCall,
		postToolCall: postToolCall,
		timeout:      10 * time.Second,
	}
}

// HasPreHook returns true if a pre_tool_call hook is configured.
func (m *Manager) HasPreHook() bool {
	return m.preToolCall != ""
}

// HasPostHook returns true if a post_tool_call hook is configured.
func (m *Manager) HasPostHook() bool {
	return m.postToolCall != ""
}

// PreToolCallRequest is sent to the pre_tool_call script via stdin.
type PreToolCallRequest struct {
	Hook     string         `json:"hook"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args"`
	Platform string         `json:"platform"`
	UserID   string         `json:"user_id"`
}

// PreToolCallResponse is read from the pre_tool_call script via stdout.
type PreToolCallResponse struct {
	Action string `json:"action"` // "allow" or "block"
	Reason string `json:"reason,omitempty"`
}

// PostToolCallRequest is sent to the post_tool_call script via stdin.
type PostToolCallRequest struct {
	Hook     string         `json:"hook"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args"`
	Result   string         `json:"result"`
	Error    string         `json:"error,omitempty"`
	Platform string         `json:"platform"`
	UserID   string         `json:"user_id"`
}

// PreToolCall runs the pre_tool_call hook.
// Returns (allow, reason, error).
// If no hook is configured, returns (true, "", nil).
func (m *Manager) PreToolCall(ctx context.Context, tool string, args map[string]any, platform, userID string) (bool, string, error) {
	if m.preToolCall == "" {
		return true, "", nil
	}

	req := PreToolCallRequest{
		Hook:     "pre_tool_call",
		Tool:     tool,
		Args:     args,
		Platform: platform,
		UserID:   userID,
	}

	output, err := m.runScript(ctx, m.preToolCall, req)
	if err != nil {
		// Hook failure = allow by default (fail open)
		return true, "", fmt.Errorf("pre_tool_call hook error: %w", err)
	}

	var resp PreToolCallResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return true, "", fmt.Errorf("pre_tool_call hook: invalid JSON response: %w", err)
	}

	switch strings.ToLower(resp.Action) {
	case "block":
		return false, resp.Reason, nil
	case "allow", "":
		return true, "", nil
	default:
		return true, "", fmt.Errorf("pre_tool_call hook: unknown action %q", resp.Action)
	}
}

// PostToolCall runs the post_tool_call hook (fire-and-forget).
func (m *Manager) PostToolCall(ctx context.Context, tool string, args map[string]any, result, errMsg, platform, userID string) {
	if m.postToolCall == "" {
		return
	}

	req := PostToolCallRequest{
		Hook:     "post_tool_call",
		Tool:     tool,
		Args:     args,
		Result:   result,
		Error:    errMsg,
		Platform: platform,
		UserID:   userID,
	}

	// Fire and forget — don't block the agent loop
	go func() {
		m.runScript(ctx, m.postToolCall, req)
	}()
}

// runScript executes a hook script with JSON input on stdin, returns stdout.
func (m *Manager) runScript(ctx context.Context, scriptPath string, input any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	// Check script exists
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("hook script not found: %s", scriptPath)
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal hook input: %w", err)
	}

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Stdin = strings.NewReader(string(inputJSON))

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("hook script exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, err
	}

	return output, nil
}
