package agent

import (
	"context"
	"testing"
	"time"

	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

func TestNewAgent(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, nil)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)
	registry.RegisterDefaults()

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)

	if a == nil {
		t.Fatal("expected non-nil agent")
	}
}

func TestNewWithLoopConfig(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, nil)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)
	registry.RegisterDefaults()

	cfg := AgentLoopConfig{
		Config: Config{
			Provider: mockProvider,
			Model:    mockProvider.Models()[0],
			Mode:     "agent",
		},
		ToolExecutionMode: "sequential",
		MaxIterations:     100,
	}

	a := NewWithLoopConfig(cfg, registry)

	if a == nil {
		t.Fatal("expected non-nil agent")
	}
}

func TestAgentAbort(t *testing.T) {
	// Use a slow provider that gives us time to abort
	responses := []provider.StreamEvent{
		{Type: provider.StreamStart},
		{Type: provider.StreamTextDelta, TextDelta: "Hello"},
		{Type: provider.StreamDone},
	}

	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, responses)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)

	// Run and collect events
	ch := a.Run(context.Background(), "test")

	// Abort after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		a.Abort()
	}()

	var events []Event
	for event := range ch {
		events = append(events, event)
	}

	// Should have events (abort may or may not cause error depending on timing)
	if len(events) == 0 {
		t.Error("expected at least one event")
	}
}

func TestAgentRun(t *testing.T) {
	responses := []provider.StreamEvent{
		{Type: provider.StreamStart},
		{Type: provider.StreamTextDelta, TextDelta: "Hello"},
		{Type: provider.StreamTextDelta, TextDelta: " World"},
		{Type: provider.StreamDone},
	}

	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, responses)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)
	registry.RegisterDefaults()

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)
	ch := a.Run(context.Background(), "test")

	var events []Event
	for event := range ch {
		events = append(events, event)
	}

	// Should have: AgentStart, TurnStart, TextDelta, TextDelta, TurnEnd, Done, AgentEnd
	if len(events) < 5 {
		t.Errorf("expected at least 5 events, got %d", len(events))
	}

	// Check first event is AgentStart
	if events[0].Type != EventAgentStart {
		t.Errorf("expected first event to be AgentStart, got %d", events[0].Type)
	}

	// Check last event is AgentEnd
	lastEvent := events[len(events)-1]
	if lastEvent.Type != EventAgentEnd {
		t.Errorf("expected last event to be AgentEnd, got %d", lastEvent.Type)
	}
}

func TestRunWithMessages(t *testing.T) {
	responses := []provider.StreamEvent{
		{Type: provider.StreamStart},
		{Type: provider.StreamTextDelta, TextDelta: "Response"},
		{Type: provider.StreamDone},
	}

	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, responses)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)
	registry.RegisterDefaults()

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)

	messages := []provider.Message{
		provider.NewUserMessage("Hello"),
	}

	ch := a.RunWithMessages(context.Background(), messages)

	var events []Event
	for event := range ch {
		events = append(events, event)
	}

	if len(events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(events))
	}
}

func TestGetSetMessages(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, nil)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)

	messages := []provider.Message{
		provider.NewUserMessage("Hello"),
		provider.NewAssistantMessage([]provider.ContentBlock{
			{Type: "text", Text: "Hi"},
		}),
	}

	a.SetMessages(messages)

	got := a.GetMessages()
	if len(got) != 2 {
		t.Errorf("expected 2 messages, got %d", len(got))
	}
}

func TestGetSetContext(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, nil)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)

	ctx := &AgentContext{
		SystemPrompt: "test prompt",
		Messages:     []provider.Message{provider.NewUserMessage("Hello")},
	}

	a.SetContext(ctx)

	got := a.GetContext()
	if got.SystemPrompt != "test prompt" {
		t.Errorf("expected system prompt 'test prompt', got '%s'", got.SystemPrompt)
	}
}

func TestAgentRunWithToolCall(t *testing.T) {
	toolCall := &provider.ToolCallBlock{
		ID:        "call_1",
		Name:      "ls",
		Arguments: []byte(`{"path": "."}`),
	}

	responses := []provider.StreamEvent{
		{Type: provider.StreamStart},
		{Type: provider.StreamToolCall, ToolCall: toolCall},
		{Type: provider.StreamDone},
	}

	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, responses)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)
	registry.RegisterDefaults()

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)
	ch := a.Run(context.Background(), "list files")

	var events []Event
	for event := range ch {
		events = append(events, event)
	}

	// Check that tool events are present
	hasToolCall := false
	hasToolExecution := false
	for _, event := range events {
		if event.Type == EventToolCall {
			hasToolCall = true
		}
		if event.Type == EventToolExecutionStart {
			hasToolExecution = true
		}
	}

	if !hasToolCall {
		t.Error("expected tool call event")
	}

	if !hasToolExecution {
		t.Error("expected tool execution event")
	}
}

func TestAgentRunSequential(t *testing.T) {
	toolCall1 := &provider.ToolCallBlock{
		ID:        "call_1",
		Name:      "ls",
		Arguments: []byte(`{"path": "."}`),
	}

	// First call returns tool call, second call returns text
	callCount := 0
	responses := func() []provider.StreamEvent {
		callCount++
		if callCount == 1 {
			return []provider.StreamEvent{
				{Type: provider.StreamStart},
				{Type: provider.StreamToolCall, ToolCall: toolCall1},
				{Type: provider.StreamDone},
			}
		}
		return []provider.StreamEvent{
			{Type: provider.StreamStart},
			{Type: provider.StreamTextDelta, TextDelta: "Done"},
			{Type: provider.StreamDone},
		}
	}

	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, responses())

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry("/tmp", sb)
	registry.RegisterDefaults()

	cfg := AgentLoopConfig{
		Config: Config{
			Provider: mockProvider,
			Model:    mockProvider.Models()[0],
			Mode:     "agent",
		},
		ToolExecutionMode: "sequential",
		MaxIterations:     10,
	}

	a := NewWithLoopConfig(cfg, registry)
	ch := a.Run(context.Background(), "test")

	var events []Event
	for event := range ch {
		events = append(events, event)
	}

	// Should have tool execution and text events
	hasToolExecution := false
	for _, event := range events {
		if event.Type == EventToolExecutionStart {
			hasToolExecution = true
		}
	}

	if !hasToolExecution {
		t.Error("expected tool execution event")
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	toolNames := []string{"read", "write", "bash"}
	cwd := "/home/user/project"
	extraContext := "## Extra\nSome extra context"
	toolSnippets := map[string]string{
		"read":  "Read file contents",
		"write": "Create or overwrite files",
		"bash":  "Execute bash commands",
	}
	toolGuidelines := []string{"Use read to examine files instead of cat or sed."}

	prompt := BuildSystemPrompt("agent", toolNames, cwd, extraContext, toolSnippets, toolGuidelines)

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}

	// Check that prompt contains expected content
	if !contains(prompt, "VibeCoding") {
		t.Error("expected prompt to contain 'VibeCoding'")
	}

	if !contains(prompt, "/home/user/project") {
		t.Error("expected prompt to contain working directory")
	}

	if !contains(prompt, "read") {
		t.Error("expected prompt to contain tool names")
	}

	if !contains(prompt, "Extra") {
		t.Error("expected prompt to contain extra context")
	}
}

func TestBuildSystemPromptModes(t *testing.T) {
	// Test plan mode
	planPrompt := BuildSystemPrompt("plan", nil, "/tmp", "", nil, nil)
	if !contains(planPrompt, "PLAN") {
		t.Error("expected plan prompt to contain 'PLAN'")
	}

	if !contains(planPrompt, "READ-ONLY") {
		t.Error("expected plan prompt to contain 'READ-ONLY'")
	}

	// Test agent mode
	agentPrompt := BuildSystemPrompt("agent", nil, "/tmp", "", nil, nil)
	if !contains(agentPrompt, "AGENT") {
		t.Error("expected agent prompt to contain 'AGENT'")
	}

	// Test yolo mode
	yoloPrompt := BuildSystemPrompt("yolo", nil, "/tmp", "", nil, nil)
	if !contains(yoloPrompt, "YOLO") {
		t.Error("expected yolo prompt to contain 'YOLO'")
	}

	// Test unknown mode
	unknownPrompt := BuildSystemPrompt("custom", nil, "/tmp", "", nil, nil)
	if !contains(unknownPrompt, "CUSTOM") {
		t.Error("expected unknown prompt to contain mode name")
	}
}

func TestFormatToolList(t *testing.T) {
	// Test with tools
	tools := []string{"read", "write", "bash"}
	list := formatToolList(tools)

	if !contains(list, "read") {
		t.Error("expected list to contain 'read'")
	}

	if !contains(list, "write") {
		t.Error("expected list to contain 'write'")
	}

	// Test empty
	emptyList := formatToolList(nil)
	if !contains(emptyList, "No tools") {
		t.Error("expected empty list to say 'No tools'")
	}
}

func TestBuildSkillsContext(t *testing.T) {
	skills := []SkillInfo{
		{Name: "test", Description: "Test skill", Path: "/path/to/skill"},
	}

	context := BuildSkillsContext(skills)

	if context == "" {
		t.Fatal("expected non-empty context")
	}

	if !contains(context, "test") {
		t.Error("expected context to contain skill name")
	}

	// Test empty
	emptyContext := BuildSkillsContext(nil)
	if emptyContext != "" {
		t.Error("expected empty context for nil skills")
	}
}

func TestBuildContextFilesContext(t *testing.T) {
	files := []ContextFileInfo{
		{Name: "AGENTS.md", Path: "/path", Scope: "project", Content: "# Test"},
	}

	context := BuildContextFilesContext(files)

	if context == "" {
		t.Fatal("expected non-empty context")
	}

	if !contains(context, "AGENTS.md") {
		t.Error("expected context to contain file name")
	}

	// Test empty
	emptyContext := BuildContextFilesContext(nil)
	if emptyContext != "" {
		t.Error("expected empty context for nil files")
	}
}

func TestConvertMessages(t *testing.T) {
	messages := []provider.Message{
		provider.NewUserMessage("Hello"),
		provider.NewAssistantMessage([]provider.ContentBlock{
			{Type: "text", Text: "Hi"},
		}),
	}

	converted := ConvertToProviderMessages(messages)
	if len(converted) != 2 {
		t.Errorf("expected 2 messages, got %d", len(converted))
	}

	converted = ConvertFromProviderMessages(messages)
	if len(converted) != 2 {
		t.Errorf("expected 2 messages, got %d", len(converted))
	}
}

func TestBaseProvider(t *testing.T) {
	models := []*provider.Model{
		{ID: "model1", Name: "Model 1"},
		{ID: "model2", Name: "Model 2"},
	}

	p := NewBaseProvider("test", models)

	if p.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", p.Name())
	}

	if len(p.Models()) != 2 {
		t.Errorf("expected 2 models, got %d", len(p.Models()))
	}

	m := p.GetModel("model1")
	if m == nil {
		t.Fatal("expected model, got nil")
	}

	if m.Name != "Model 1" {
		t.Errorf("expected name 'Model 1', got '%s'", m.Name)
	}

	m = p.GetModel("nonexistent")
	if m != nil {
		t.Error("expected nil for nonexistent model")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
