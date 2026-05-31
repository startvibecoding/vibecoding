package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
)

// --- AgentManager tests ---

func newTestManager() *AgentManager {
	factory := &AgentFactory{}
	return NewAgentManager(factory)
}

func TestAgentManagerCreate(t *testing.T) {
	m := newTestManager()

	a, err := m.Create(AgentOptions{ID: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil agent")
	}
	if a.ID() != "main" {
		t.Errorf("expected ID 'main', got %q", a.ID())
	}
}

func TestAgentManagerCreateAutoID(t *testing.T) {
	m := newTestManager()

	a, err := m.Create(AgentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID() == "" {
		t.Error("expected non-empty auto-generated ID")
	}
}

func TestAgentManagerCreateWithParent(t *testing.T) {
	m := newTestManager()

	parent, _ := m.Create(AgentOptions{ID: "main"})
	child, err := m.Create(AgentOptions{ID: "sub-1", ParentID: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child.ParentID() != "main" {
		t.Errorf("expected parent 'main', got %q", child.ParentID())
	}

	children := m.Children("main")
	if len(children) != 1 || children[0] != "sub-1" {
		t.Errorf("expected [sub-1], got %v", children)
	}

	pid, ok := m.Parent("sub-1")
	if !ok || pid != "main" {
		t.Errorf("expected parent 'main', got %q (ok=%v)", pid, ok)
	}

	_ = parent
}

func TestAgentManagerCreateNestedSubAgentRejected(t *testing.T) {
	m := newTestManager()

	// Create a sub-agent
	m.Create(AgentOptions{ID: "main"})
	m.Create(AgentOptions{ID: "sub-1", ParentID: "main"})

	// Try to create a sub-sub-agent (should fail - Decision 5)
	_, err := m.Create(AgentOptions{ID: "sub-sub-1", ParentID: "sub-1"})
	if err == nil {
		t.Fatal("expected error for nested sub-agent, got nil")
	}
}

func TestAgentManagerCreateMissingParent(t *testing.T) {
	m := newTestManager()

	_, err := m.Create(AgentOptions{ID: "orphan", ParentID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing parent, got nil")
	}
}

func TestAgentManagerGet(t *testing.T) {
	m := newTestManager()
	m.Create(AgentOptions{ID: "main"})

	a, ok := m.Get("main")
	if !ok || a == nil {
		t.Fatal("expected to find agent 'main'")
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("expected not to find agent 'nonexistent'")
	}
}

func TestAgentManagerDestroy(t *testing.T) {
	m := newTestManager()
	m.Create(AgentOptions{ID: "main"})
	m.Create(AgentOptions{ID: "sub-1", ParentID: "main"})
	m.Create(AgentOptions{ID: "sub-2", ParentID: "main"})

	if m.Count() != 3 {
		t.Errorf("expected 3 agents, got %d", m.Count())
	}

	err := m.Destroy("main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All should be destroyed (children recursively)
	if m.Count() != 0 {
		t.Errorf("expected 0 agents after destroy, got %d", m.Count())
	}
}

func TestAgentManagerDestroyChild(t *testing.T) {
	m := newTestManager()
	m.Create(AgentOptions{ID: "main"})
	m.Create(AgentOptions{ID: "sub-1", ParentID: "main"})
	m.Create(AgentOptions{ID: "sub-2", ParentID: "main"})

	err := m.Destroy("sub-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parent should still exist with one child
	if m.Count() != 2 {
		t.Errorf("expected 2 agents, got %d", m.Count())
	}
	children := m.Children("main")
	if len(children) != 1 || children[0] != "sub-2" {
		t.Errorf("expected [sub-2], got %v", children)
	}
}

func TestAgentManagerDestroyNotFound(t *testing.T) {
	m := newTestManager()
	err := m.Destroy("nonexistent")
	if err == nil {
		t.Fatal("expected error for destroying nonexistent agent")
	}
}

func TestAgentManagerList(t *testing.T) {
	m := newTestManager()
	m.Create(AgentOptions{ID: "a"})
	m.Create(AgentOptions{ID: "b"})
	m.Create(AgentOptions{ID: "c"})

	ids := m.List()
	if len(ids) != 3 {
		t.Errorf("expected 3 IDs, got %d", len(ids))
	}
}

func TestAgentManagerChildrenEmpty(t *testing.T) {
	m := newTestManager()
	m.Create(AgentOptions{ID: "main"})

	children := m.Children("main")
	if children != nil {
		t.Errorf("expected nil children, got %v", children)
	}
}

func TestAgentManagerParentNotFound(t *testing.T) {
	m := newTestManager()
	_, ok := m.Parent("nonexistent")
	if ok {
		t.Error("expected false for nonexistent agent")
	}
}

func TestAgentManagerConcurrent(t *testing.T) {
	m := newTestManager()
	m.Create(AgentOptions{ID: "main"})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Create(AgentOptions{ID: agentpkg.AgentID("sub"), ParentID: "main"})
		}()
	}
	wg.Wait()

	// Some will fail due to duplicate IDs, but no panic
	if m.Count() < 2 {
		t.Errorf("expected at least 2 agents, got %d", m.Count())
	}
}

// --- EventRouter tests ---

func TestEventRouterDispatch(t *testing.T) {
	r := NewEventRouter()

	var received []agentpkg.Event
	r.RegisterAgent("agent-1", RouterEventHandlerFunc(func(e agentpkg.Event) error {
		received = append(received, e)
		return nil
	}))

	r.Dispatch(agentpkg.Event{AgentID: "agent-1", Type: agentpkg.EventTextDelta, TextDelta: "hello"})
	r.Dispatch(agentpkg.Event{AgentID: "agent-2", Type: agentpkg.EventTextDelta, TextDelta: "world"})

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].TextDelta != "hello" {
		t.Errorf("expected 'hello', got %q", received[0].TextDelta)
	}
}

func TestEventRouterGlobal(t *testing.T) {
	r := NewEventRouter()

	var received []agentpkg.Event
	r.RegisterGlobal(RouterEventHandlerFunc(func(e agentpkg.Event) error {
		received = append(received, e)
		return nil
	}))

	r.Dispatch(agentpkg.Event{AgentID: "a1", Type: agentpkg.EventDone})
	r.Dispatch(agentpkg.Event{AgentID: "a2", Type: agentpkg.EventDone})

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
}

func TestEventRouterUnregisterAgent(t *testing.T) {
	r := NewEventRouter()

	count := 0
	r.RegisterAgent("a1", RouterEventHandlerFunc(func(e agentpkg.Event) error {
		count++
		return nil
	}))

	r.Dispatch(agentpkg.Event{AgentID: "a1"})
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}

	r.UnregisterAgent("a1")
	r.Dispatch(agentpkg.Event{AgentID: "a1"})
	if count != 1 {
		t.Errorf("expected still 1 after unregister, got %d", count)
	}
}

func TestEventRouterError(t *testing.T) {
	r := NewEventRouter()
	testErr := errors.New("test error")

	r.RegisterAgent("a1", RouterEventHandlerFunc(func(e agentpkg.Event) error {
		return testErr
	}))

	err := r.Dispatch(agentpkg.Event{AgentID: "a1"})
	if err != testErr {
		t.Errorf("expected test error, got %v", err)
	}
}

func TestEventRouterHandlerCount(t *testing.T) {
	r := NewEventRouter()
	r.RegisterAgent("a1", RouterEventHandlerFunc(func(e agentpkg.Event) error { return nil }))
	r.RegisterAgent("a1", RouterEventHandlerFunc(func(e agentpkg.Event) error { return nil }))
	r.RegisterGlobal(RouterEventHandlerFunc(func(e agentpkg.Event) error { return nil }))

	if r.HandlerCount("a1") != 2 {
		t.Errorf("expected 2 handlers for a1, got %d", r.HandlerCount("a1"))
	}
	if r.HandlerCount("a2") != 0 {
		t.Errorf("expected 0 handlers for a2, got %d", r.HandlerCount("a2"))
	}
	if r.GlobalHandlerCount() != 1 {
		t.Errorf("expected 1 global handler, got %d", r.GlobalHandlerCount())
	}
}

func TestEventRouterMultipleAgents(t *testing.T) {
	r := NewEventRouter()

	var mu sync.Mutex
	received := map[agentpkg.AgentID][]string{}

	r.RegisterGlobal(RouterEventHandlerFunc(func(e agentpkg.Event) error {
		mu.Lock()
		received[e.AgentID] = append(received[e.AgentID], e.TextDelta)
		mu.Unlock()
		return nil
	}))

	r.Dispatch(agentpkg.Event{AgentID: "a1", TextDelta: "from-a1"})
	r.Dispatch(agentpkg.Event{AgentID: "a2", TextDelta: "from-a2"})
	r.Dispatch(agentpkg.Event{AgentID: "a1", TextDelta: "from-a1-again"})

	if len(received["a1"]) != 2 {
		t.Errorf("expected 2 events for a1, got %d", len(received["a1"]))
	}
	if len(received["a2"]) != 1 {
		t.Errorf("expected 1 event for a2, got %d", len(received["a2"]))
	}
}

// --- AgentAdapter tests ---

func TestAgentAdapterImplementsInterface(t *testing.T) {
	// Verify AgentAdapter satisfies agent.Agent interface at compile time
	var _ agentpkg.Agent = (*AgentAdapter)(nil)
}

func TestEventToPublic(t *testing.T) {
	e := Event{
		AgentID:       "test-agent",
		Type:          EventTextDelta,
		TextDelta:     "hello",
		ToolCallID:    "tc1",
		ToolName:      "bash",
		ToolArgs:      map[string]any{"cmd": "ls"},
		StatusMessage: "running",
		Done:          true,
		StopReason:    "end_turn",
		Error:         context.Canceled,
		ApprovalID:    "ap1",
		ApprovalTool:  "write",
		ApprovalResult: true,
	}

	pub := EventToPublic(e)
	if pub.AgentID != "test-agent" {
		t.Errorf("expected agent ID 'test-agent', got %q", pub.AgentID)
	}
	if pub.Type != agentpkg.EventTextDelta {
		t.Errorf("expected EventTextDelta, got %d", pub.Type)
	}
	if pub.TextDelta != "hello" {
		t.Errorf("expected 'hello', got %q", pub.TextDelta)
	}
	if pub.Error != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", pub.Error)
	}
	if !pub.ApprovalResult {
		t.Error("expected ApprovalResult=true")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	original := agentpkg.Message{
		Role:    agentpkg.RoleAssistant,
		Content: "test content",
		Contents: []agentpkg.ContentBlock{
			{Type: "text", Text: "hello"},
			{Type: "toolCall", ToolCall: &agentpkg.ToolCallBlock{ID: "tc1", Name: "bash"}},
		},
		Usage: &agentpkg.Usage{InputTokens: 100, OutputTokens: 50},
	}

	internal := MessageFromPublic(original)
	back := MessageToPublic(internal)

	if back.Role != original.Role {
		t.Errorf("role mismatch: %q vs %q", back.Role, original.Role)
	}
	if back.Content != original.Content {
		t.Errorf("content mismatch: %q vs %q", back.Content, original.Content)
	}
	if len(back.Contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(back.Contents))
	}
	if back.Contents[1].ToolCall.Name != "bash" {
		t.Errorf("tool call name mismatch: %q", back.Contents[1].ToolCall.Name)
	}
	if back.Usage.InputTokens != 100 {
		t.Errorf("usage mismatch: %d", back.Usage.InputTokens)
	}
}

func TestContextUsageToPublicNil(t *testing.T) {
	if ContextUsageToPublic(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestWrapEventChan(t *testing.T) {
	in := make(chan Event, 2)
	in <- Event{AgentID: "a1", Type: EventTextDelta, TextDelta: "hi"}
	in <- Event{AgentID: "a1", Type: EventDone}
	close(in)

	out := WrapEventChan(in)
	var events []agentpkg.Event
	for e := range out {
		events = append(events, e)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].TextDelta != "hi" {
		t.Errorf("expected 'hi', got %q", events[0].TextDelta)
	}
}
