package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

func newTestFactoryAndManager(t testing.TB) (*AgentFactory, *AgentManager) {
	t.Helper()

	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, nil)

	sandboxMgr := sandbox.NewManager(t.TempDir())
	sandboxMgr.SetLevel(sandbox.LevelNone)
	settings := &config.Settings{SessionDir: t.TempDir()}

	factory := NewAgentFactory(
		mockProvider,
		mockProvider.Models()[0],
		settings,
		sandboxMgr,
		"",
		ctxpkg.CompactionSettings{},
		nil,
	)
	return factory, NewAgentManager(factory)
}

func TestSubAgentSpawnTool(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentSpawnTool(mgr)

	if tool.Name() != "subagent_spawn" {
		t.Errorf("expected 'subagent_spawn', got %q", tool.Name())
	}

	result, err := tool.Execute(context.Background(), map[string]any{
		"task": "list files",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Text), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["handle"] == nil || parsed["handle"] == "" {
		t.Error("expected non-empty handle")
	}
	if parsed["status"] != "running" {
		t.Errorf("expected 'running', got %q", parsed["status"])
	}
	handle, _ := parsed["handle"].(string)
	waitForManagedAgentToStop(t, mgr, agentpkg.AgentID(handle))
	if err := mgr.Destroy(agentpkg.AgentID(handle)); err != nil {
		t.Fatalf("destroy spawned agent: %v", err)
	}
}

func waitForManagedAgentToStop(t testing.TB, mgr *AgentManager, id agentpkg.AgentID) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		st, ok := mgr.Status(id)
		if ok && (st.State == "done" || st.State == "error") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for agent %s to stop", id)
}

func TestSubAgentSpawnToolMissingTask(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentSpawnTool(mgr)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing task")
	}
}

func TestSubAgentStatusTool(t *testing.T) {
	factory, mgr := newTestFactoryAndManager(t)
	_ = factory

	// Create an agent manually
	a, _ := mgr.Create(AgentOptions{ID: "test-agent"})

	tool := NewSubAgentStatusTool(mgr)
	result, err := tool.Execute(context.Background(), map[string]any{
		"handle": string(a.ID()),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result.Text), &parsed)
	if parsed["handle"] != "test-agent" {
		t.Errorf("expected 'test-agent', got %q", parsed["handle"])
	}
}

func TestSubAgentStatusToolNotFound(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentStatusTool(mgr)

	_, err := tool.Execute(context.Background(), map[string]any{
		"handle": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestSubAgentStatusToolAfterParentFinish(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	mgr.Create(AgentOptions{ID: "main"})
	mgr.Create(AgentOptions{ID: "sub-1", ParentID: "main"})
	mgr.MarkDone("sub-1", "finished work")
	mgr.Finish("main", nil)

	tool := NewSubAgentStatusTool(mgr)
	result, err := tool.Execute(context.Background(), map[string]any{
		"handle": "sub-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Text), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["status"] != "done" {
		t.Fatalf("expected done status, got %q", parsed["status"])
	}
	if parsed["last_response"] != "finished work" {
		t.Fatalf("expected retained response, got %q", parsed["last_response"])
	}
}

func TestSubAgentStatusToolMissingHandle(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentStatusTool(mgr)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing handle")
	}
}

func TestSubAgentSendTool(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	a, _ := mgr.Create(AgentOptions{ID: "test-agent"})

	tool := NewSubAgentSendTool(mgr)
	result, err := tool.Execute(context.Background(), map[string]any{
		"handle":  string(a.ID()),
		"message": "do something",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result.Text), &parsed)
	if parsed["status"] != "message_sent" {
		t.Errorf("expected 'message_sent', got %q", parsed["status"])
	}
}

func TestSubAgentSendToolNotFound(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentSendTool(mgr)

	_, err := tool.Execute(context.Background(), map[string]any{
		"handle":  "nonexistent",
		"message": "test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSubAgentSendToolMissingParams(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentSendTool(mgr)

	_, err := tool.Execute(context.Background(), map[string]any{
		"handle": "x",
	})
	if err == nil {
		t.Fatal("expected error for missing message")
	}
}

func TestSubAgentDestroyTool(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	a, _ := mgr.Create(AgentOptions{ID: "to-destroy"})

	tool := NewSubAgentDestroyTool(mgr)
	result, err := tool.Execute(context.Background(), map[string]any{
		"handle": string(a.ID()),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result.Text), &parsed)
	if parsed["status"] != "destroyed" {
		t.Errorf("expected 'destroyed', got %q", parsed["status"])
	}

	// Verify it's gone
	if _, ok := mgr.Get("to-destroy"); ok {
		t.Error("expected agent to be destroyed")
	}
}

func TestSubAgentDestroyToolNotFound(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentDestroyTool(mgr)

	_, err := tool.Execute(context.Background(), map[string]any{
		"handle": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSubAgentDestroyToolMissingHandle(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	tool := NewSubAgentDestroyTool(mgr)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing handle")
	}
}

// --- SubAgentPolicy tests ---

func TestSubAgentPolicyDefault(t *testing.T) {
	p := DefaultSubAgentPolicy()
	if p.MaxChildren != 5 {
		t.Errorf("expected MaxChildren=5, got %d", p.MaxChildren)
	}
	if len(p.AllowedModes) != 1 || p.AllowedModes[0] != "agent" {
		t.Errorf("expected AllowedModes=[agent], got %v", p.AllowedModes)
	}
}

func TestSubAgentPolicyValidateTopLevel(t *testing.T) {
	p := DefaultSubAgentPolicy()
	// Top-level agents (no parent) are always allowed
	if err := p.Validate("", "yolo", 0); err != nil {
		t.Errorf("expected no error for top-level, got %v", err)
	}
}

func TestSubAgentPolicyValidateAllowed(t *testing.T) {
	p := DefaultSubAgentPolicy()
	if err := p.Validate("parent", "agent", 0); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSubAgentPolicyValidateMaxChildren(t *testing.T) {
	p := DefaultSubAgentPolicy()
	err := p.Validate("parent", "agent", 5)
	if err == nil {
		t.Fatal("expected error for max children")
	}
}

func TestSubAgentPolicyValidateDisallowedMode(t *testing.T) {
	p := DefaultSubAgentPolicy()
	err := p.Validate("parent", "yolo", 0)
	if err == nil {
		t.Fatal("expected error for disallowed mode")
	}
}

func TestSubAgentPolicyValidateCustom(t *testing.T) {
	p := SubAgentPolicy{
		MaxChildren:  3,
		AllowedModes: []string{"agent", "plan"},
	}
	if err := p.Validate("parent", "plan", 1); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if err := p.Validate("parent", "yolo", 0); err == nil {
		t.Error("expected error for yolo")
	}
	if err := p.Validate("parent", "agent", 3); err == nil {
		t.Error("expected error for max children")
	}
}

func TestSubAgentPromptContractOnlyForChild(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	parent, err := mgr.Create(AgentOptions{ID: "main"})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := mgr.Create(AgentOptions{ID: "sub-1", ParentID: parent.ID()})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	parentCtx := parent.GetContext()
	if parentCtx == nil || !contains(parentCtx.SystemPrompt, "Sub-Agent Tools") {
		t.Fatal("expected top-level multi-agent prompt to include orchestration guidance")
	}
	if contains(parentCtx.SystemPrompt, "Sub-Agent Operating Contract") {
		t.Error("expected top-level prompt to omit worker contract")
	}

	childCtx := child.GetContext()
	if childCtx == nil || !contains(childCtx.SystemPrompt, "Sub-Agent Operating Contract") {
		t.Fatal("expected child prompt to include worker contract")
	}
	if contains(childCtx.SystemPrompt, "Sub-Agent Tools") {
		t.Error("expected child prompt to omit sub-agent tools guidance")
	}
}

func TestAgentManagerEnforcesSubAgentPolicy(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)
	parent, err := mgr.Create(AgentOptions{ID: "main"})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	for i := 0; i < DefaultSubAgentPolicy().MaxChildren; i++ {
		_, err := mgr.Create(AgentOptions{
			ID:       agentpkg.AgentID(fmt.Sprintf("sub-%d", i)),
			ParentID: parent.ID(),
			Mode:     "agent",
		})
		if err != nil {
			t.Fatalf("create child %d: %v", i, err)
		}
	}

	_, err = mgr.Create(AgentOptions{ID: "sub-overflow", ParentID: parent.ID(), Mode: "agent"})
	if err == nil {
		t.Fatal("expected max-children error")
	}

	_, mgr = newTestFactoryAndManager(t)
	parent, _ = mgr.Create(AgentOptions{ID: "main"})
	_, err = mgr.Create(AgentOptions{ID: "sub-yolo", ParentID: parent.ID(), Mode: "yolo"})
	if err == nil {
		t.Fatal("expected disallowed mode error")
	}
}

// --- Tool interface compliance ---

func TestSubAgentToolsImplementToolInterface(t *testing.T) {
	var _ tools.Tool = (*SubAgentSpawnTool)(nil)
	var _ tools.Tool = (*SubAgentStatusTool)(nil)
	var _ tools.Tool = (*SubAgentSendTool)(nil)
	var _ tools.Tool = (*SubAgentDestroyTool)(nil)
}

func TestSubAgentToolsDescriptions(t *testing.T) {
	_, mgr := newTestFactoryAndManager(t)

	tools := []tools.Tool{
		NewSubAgentSpawnTool(mgr),
		NewSubAgentStatusTool(mgr),
		NewSubAgentSendTool(mgr),
		NewSubAgentDestroyTool(mgr),
	}

	for _, tool := range tools {
		if tool.Name() == "" {
			t.Errorf("tool %T has empty name", tool)
		}
		if tool.Description() == "" {
			t.Errorf("tool %s has empty description", tool.Name())
		}
		if tool.Parameters() == nil {
			t.Errorf("tool %s has nil parameters", tool.Name())
		}
	}
}

// TestSendParentEvent_ClosedChannel verifies sendParentEvent does not panic
// when the channel is closed (recover logs and returns false).
func TestSendParentEvent_ClosedChannel(t *testing.T) {
	ch := make(chan Event, 1)
	close(ch)

	ev := Event{Type: EventStatus, StatusMessage: "test"}
	ok := sendParentEvent(context.Background(), ch, ev)
	if ok {
		t.Error("expected sendParentEvent to return false on closed channel")
	}
}

// TestSendParentEvent_ContextCanceled verifies sendParentEvent returns false
// when the context is canceled and the channel is full (unbuffered, never read).
func TestSendParentEvent_ContextCanceled(t *testing.T) {
	ch := make(chan Event) // unbuffered — will block until context cancels
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ev := Event{Type: EventStatus, StatusMessage: "test"}
	ok := sendParentEvent(ctx, ch, ev)
	if ok {
		t.Error("expected sendParentEvent to return false on canceled context")
	}
}

// TestSendParentEvent_Success verifies sendParentEvent succeeds normally.
func TestSendParentEvent_Success(t *testing.T) {
	ch := make(chan Event, 1)
	ev := Event{Type: EventStatus, StatusMessage: "test"}
	ok := sendParentEvent(context.Background(), ch, ev)
	if !ok {
		t.Error("expected sendParentEvent to return true on success")
	}
	received := <-ch
	if received.StatusMessage != "test" {
		t.Errorf("expected 'test', got %q", received.StatusMessage)
	}
}
