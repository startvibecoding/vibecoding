package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

type loopingToolProvider struct {
	models    []*provider.Model
	callCount int
}

func newLoopingToolProvider() *loopingToolProvider {
	return &loopingToolProvider{
		models: []*provider.Model{{ID: "model1", Name: "Model 1"}},
	}
}

func (p *loopingToolProvider) Chat(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
	ch := make(chan provider.StreamEvent, 3)
	p.callCount++
	toolCall := &provider.ToolCallBlock{
		ID:        fmt.Sprintf("call_%d", p.callCount),
		Name:      "unknown_tool",
		Arguments: []byte(`{}`),
	}
	go func() {
		defer close(ch)
		ch <- provider.StreamEvent{Type: provider.StreamStart}
		ch <- provider.StreamEvent{Type: provider.StreamToolCall, ToolCall: toolCall}
		ch <- provider.StreamEvent{Type: provider.StreamDone}
	}()
	return ch
}

func (p *loopingToolProvider) Name() string {
	return "looping"
}

func (p *loopingToolProvider) Models() []*provider.Model {
	return p.models
}

func (p *loopingToolProvider) GetModel(id string) *provider.Model {
	for _, m := range p.models {
		if m.ID == id {
			return m
		}
	}
	return nil
}

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

func TestToolOnlyWarningAppendedAfterToolResults(t *testing.T) {
	mockProvider := newLoopingToolProvider()

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry(t.TempDir(), sb)

	var stopped bool
	cfg := AgentLoopConfig{
		Config: Config{
			Provider: mockProvider,
			Model:    mockProvider.Models()[0],
			Mode:     "agent",
		},
		ToolExecutionMode: "sequential",
		MaxIterations:     95,
		ShouldStopAfterTurn: func(ctx ShouldStopAfterTurnContext) bool {
			for _, msg := range ctx.NewMessages {
				if msg.Role == "user" && contains(msg.Content, "You have been making tool calls") {
					stopped = true
					return true
				}
			}
			return false
		},
	}

	a := NewWithLoopConfig(cfg, registry)
	ch := a.Run(context.Background(), "keep using tools")

	for range ch {
	}

	if !stopped {
		t.Fatal("expected warning-triggered stop")
	}

	messages := a.GetMessages()
	warningIndex := -1
	for i, msg := range messages {
		if msg.Role == "user" && contains(msg.Content, "You have been making tool calls") {
			warningIndex = i
			break
		}
	}
	if warningIndex < 2 {
		t.Fatalf("warning index = %d, want at least 2", warningIndex)
	}
	if messages[warningIndex-1].Role != "toolResult" {
		t.Fatalf("message before warning role = %q, want toolResult", messages[warningIndex-1].Role)
	}
	if messages[warningIndex-2].Role != "assistant" {
		t.Fatalf("message before tool result role = %q, want assistant", messages[warningIndex-2].Role)
	}
}

func TestCallbackSnapshotDoesNotExposeInternalSlices(t *testing.T) {
	mockProvider := newMockProvider()
	a := New(Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}, tools.NewRegistry(t.TempDir(), sandbox.NewNoneSandbox()))

	a.messages = []provider.Message{
		provider.NewAssistantMessage([]provider.ContentBlock{{
			Type: "toolCall",
			ToolCall: &provider.ToolCallBlock{
				ID:        "call-1",
				Name:      "read",
				Arguments: json.RawMessage(`{"path":"a"}`),
			},
		}}),
	}
	a.context.Messages = a.messages

	messages, ctx := a.callbackSnapshot()
	messages[0].Contents[0].ToolCall.Name = "mutated"
	ctx.Messages[0].Contents[0].ToolCall.Arguments[0] = '{'

	if a.messages[0].Contents[0].ToolCall.Name != "read" {
		t.Fatalf("internal tool name mutated: %s", a.messages[0].Contents[0].ToolCall.Name)
	}
	if string(a.context.Messages[0].Contents[0].ToolCall.Arguments) != `{"path":"a"}` {
		t.Fatalf("internal arguments mutated: %s", string(a.context.Messages[0].Contents[0].ToolCall.Arguments))
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

func TestWebSearchToolDefinitionCarriesModelMetadata(t *testing.T) {
	settings := &config.Settings{
		WebSearch: config.WebSearchSettings{
			Enabled:      config.BoolPtr(true),
			Provider:     "anthropic",
			ProviderType: "messages",
			Model:        "claude-sonnet-4-20250514",
		},
	}
	def, ok := webSearchToolDefinition(settings)
	if !ok {
		t.Fatal("expected web search tool definition")
	}
	if def.Name != "web_search" {
		t.Fatalf("name = %q, want web_search", def.Name)
	}
	if def.Provider != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", def.Provider)
	}
	if def.ProviderType != "messages" {
		t.Fatalf("providerType = %q, want messages", def.ProviderType)
	}
	if def.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("model = %q, want claude-sonnet-4-20250514", def.Model)
	}
}

func TestWebSearchToolDefinitionResolvesProviderReference(t *testing.T) {
	settings := &config.Settings{
		DefaultProvider: "gpt",
		WebSearch: config.WebSearchSettings{
			Enabled:      config.BoolPtr(true),
			Provider:     "gpt",
			ProviderType: "responses",
		},
		Providers: map[string]*config.ProviderConfig{
			"gpt": {
				BaseURL: "https://co.yes.vg/v1",
				API:     "openai-responses",
			},
		},
	}
	def, ok := webSearchToolDefinition(settings)
	if !ok {
		t.Fatal("expected web search tool definition")
	}
	if def.Provider != "gpt" {
		t.Fatalf("provider = %q, want gpt", def.Provider)
	}
	if def.ProviderType != "responses" {
		t.Fatalf("providerType = %q, want responses", def.ProviderType)
	}
	if def.Provider == "" {
		t.Fatal("expected hosted provider to be resolved")
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

	prompt := BuildSystemPrompt("agent", toolNames, cwd, extraContext, toolSnippets, toolGuidelines, false)

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
	planPrompt := BuildSystemPrompt("plan", nil, "/tmp", "", nil, nil, false)
	if !contains(planPrompt, "PLAN") {
		t.Error("expected plan prompt to contain 'PLAN'")
	}

	if !contains(planPrompt, "READ-ONLY") {
		t.Error("expected plan prompt to contain 'READ-ONLY'")
	}

	// Test agent mode
	agentPrompt := BuildSystemPrompt("agent", nil, "/tmp", "", nil, nil, false)
	if !contains(agentPrompt, "AGENT") {
		t.Error("expected agent prompt to contain 'AGENT'")
	}

	// Test yolo mode
	yoloPrompt := BuildSystemPrompt("yolo", nil, "/tmp", "", nil, nil, false)
	if !contains(yoloPrompt, "YOLO") {
		t.Error("expected yolo prompt to contain 'YOLO'")
	}

	// Test unknown mode
	unknownPrompt := BuildSystemPrompt("custom", nil, "/tmp", "", nil, nil, false)
	if !contains(unknownPrompt, "CUSTOM") {
		t.Error("expected unknown prompt to contain mode name")
	}
}

func TestBuildSystemPromptMultiAgentGated(t *testing.T) {
	defaultPrompt := BuildSystemPrompt("agent", nil, "/tmp", "", nil, nil, false)
	if contains(defaultPrompt, "Sub-Agent Tools") {
		t.Error("expected default prompt to omit sub-agent instructions")
	}

	multiPrompt := BuildSystemPrompt("agent", []string{"subagent_spawn"}, "/tmp", "", nil, nil, true)
	if !contains(multiPrompt, "Sub-Agent Tools") {
		t.Error("expected multi-agent prompt to include sub-agent instructions")
	}
	if !contains(multiPrompt, "Act as the orchestrator") {
		t.Error("expected multi-agent prompt to include orchestration guidance")
	}
}

// --- stripImageContent tests ---

func TestStripImageContent(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "toolResult", ToolName: "read", Contents: []provider.ContentBlock{
			{Type: "text", Text: "[Image file: test.png]"},
			{Type: "image", Image: &provider.ImageContent{MimeType: "image/png", Data: "base64data"}},
		}},
		{Role: "assistant", Contents: []provider.ContentBlock{
			{Type: "text", Text: "I see the image"},
		}},
	}

	result := stripImageContent(messages)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// Second message should have image stripped
	if len(result[1].Contents) != 1 {
		t.Errorf("expected 1 content block after stripping, got %d", len(result[1].Contents))
	}
	if result[1].Contents[0].Type == "image" {
		t.Error("image content should have been stripped")
	}
}

func TestStripImageContentOnlyImage(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "toolResult", ToolName: "read", Contents: []provider.ContentBlock{
			{Type: "image", Image: &provider.ImageContent{MimeType: "image/png", Data: "base64data"}},
		}},
	}

	result := stripImageContent(messages)
	// Message with only image and no text should be skipped
	if len(result) != 1 {
		t.Fatalf("expected 1 message (image-only skipped), got %d", len(result))
	}
}

func TestSupportsImages(t *testing.T) {
	a := &Agent{config: AgentLoopConfig{}}
	a.config.Model = &provider.Model{Input: []string{"text"}}
	if a.supportsImages() {
		t.Error("expected false for text-only model")
	}

	a.config.Model = &provider.Model{Input: []string{"text", "image"}}
	if !a.supportsImages() {
		t.Error("expected true for text+image model")
	}

	a.config.Model = nil
	if a.supportsImages() {
		t.Error("expected false for nil model")
	}
}

func TestFormatToolListWithSnippets(t *testing.T) {
	// Test with tools and snippets
	tools := []string{"read", "write", "bash"}
	snippets := map[string]string{"read": "Read a file", "write": "Write a file"}
	list := formatToolListWithSnippets(tools, snippets)

	if !contains(list, "read") {
		t.Error("expected list to contain 'read'")
	}

	if !contains(list, "Read a file") {
		t.Error("expected list to contain snippet")
	}

	// Test empty
	emptyList := formatToolListWithSnippets(nil, nil)
	if emptyList != "(none)" {
		t.Errorf("expected empty list to say '(none)', got %q", emptyList)
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

// --- ContextWithAgentID tests ---

func TestContextWithAgentID(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithAgentID(ctx, "test-agent")

	id, ok := AgentIDFromContext(ctx)
	if !ok {
		t.Fatal("expected agent ID in context")
	}
	if id != "test-agent" {
		t.Errorf("agent ID = %q, want 'test-agent'", id)
	}

	// Missing from context
	_, ok = AgentIDFromContext(context.Background())
	if ok {
		t.Error("expected no agent ID in empty context")
	}
}

func TestContextWithEventChan(t *testing.T) {
	ch := make(chan Event, 1)
	ctx := ContextWithEventChan(context.Background(), ch)

	got, ok := EventChanFromContext(ctx)
	if !ok {
		t.Fatal("expected event chan in context")
	}
	if got == nil {
		t.Fatal("expected non-nil event chan")
	}

	_, ok = EventChanFromContext(context.Background())
	if ok {
		t.Error("expected no event chan in empty context")
	}
}

func TestContextWithParentRunContext(t *testing.T) {
	parent := context.Background()
	ctx := ContextWithParentRunContext(context.Background(), parent)

	got, ok := ParentRunContextFromContext(ctx)
	if !ok {
		t.Fatal("expected parent run context")
	}
	if got != parent {
		t.Fatal("unexpected parent run context")
	}

	_, ok = ParentRunContextFromContext(context.Background())
	if ok {
		t.Error("expected no parent run context in empty context")
	}
}

// --- Manager status tests ---

func TestAgentManagerMarkRunning(t *testing.T) {
	m := NewAgentManager(&AgentFactory{})
	m.Create(AgentOptions{ID: "a1"})
	m.MarkRunning("a1")
	st, ok := m.Status("a1")
	if !ok {
		t.Fatal("expected status")
	}
	if st.State != "running" {
		t.Errorf("state = %q, want running", st.State)
	}
}

func TestAgentManagerMarkDone(t *testing.T) {
	m := NewAgentManager(&AgentFactory{})
	m.Create(AgentOptions{ID: "a1"})
	m.MarkDone("a1", "completed")
	st, _ := m.Status("a1")
	if st.State != "done" {
		t.Errorf("state = %q, want done", st.State)
	}
	if st.Result != "completed" {
		t.Errorf("result = %q, want completed", st.Result)
	}
}

func TestAgentManagerMarkError(t *testing.T) {
	m := NewAgentManager(&AgentFactory{})
	m.Create(AgentOptions{ID: "a1"})
	m.MarkError("a1", fmt.Errorf("test error"))
	st, _ := m.Status("a1")
	if st.State != "error" {
		t.Errorf("state = %q, want error", st.State)
	}
	if st.Error != "test error" {
		t.Errorf("error = %q, want 'test error'", st.Error)
	}
}

func TestAgentManagerMarkErrorNil(t *testing.T) {
	m := NewAgentManager(&AgentFactory{})
	m.Create(AgentOptions{ID: "a1"})
	m.MarkError("a1", nil)
	st, _ := m.Status("a1")
	if st.Error != "" {
		t.Errorf("error = %q, want empty", st.Error)
	}
}

func TestAgentManagerRegister(t *testing.T) {
	m := NewAgentManager(&AgentFactory{})
	// Create an agent through factory to get a valid agentpkg.Agent
	a, _ := m.Create(AgentOptions{ID: "parent"})
	m.Destroy("parent")
	// Re-register
	m.Register(a)
	if m.Count() != 1 {
		t.Errorf("count = %d, want 1", m.Count())
	}
}

func TestAgentManagerRegisterNil(t *testing.T) {
	m := NewAgentManager(&AgentFactory{})
	m.Register(nil) // Should not panic
	if m.Count() != 0 {
		t.Errorf("count = %d, want 0", m.Count())
	}
}

func TestAgentManagerStatusNotFound(t *testing.T) {
	m := NewAgentManager(&AgentFactory{})
	_, ok := m.Status("nonexistent")
	if ok {
		t.Error("expected not found")
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

// --- ForceCompact tests ---

func TestSetForceCompact_ShouldCompactReturnsTrue(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1", ContextWindow: 100000},
	}, nil)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry(t.TempDir(), sb)

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)

	// Load some messages so there's something to compact
	a.LoadHistoryMessages([]provider.Message{
		provider.NewUserMessage("Hello"),
		provider.NewAssistantMessage([]provider.ContentBlock{{Type: "text", Text: "Hi there"}}),
	})

	// Without force, ShouldCompact should be false (context is tiny)
	if a.ShouldCompact() {
		t.Fatal("ShouldCompact should be false without force and small context")
	}

	// Set force flag
	a.SetForceCompact()

	// Now ShouldCompact should return true (force flag set)
	if !a.ShouldCompact() {
		t.Fatal("ShouldCompact should be true after SetForceCompact")
	}

	// Force flag is consumed — second call should return false
	if a.ShouldCompact() {
		t.Fatal("ShouldCompact should be false after force flag was consumed")
	}
}

func TestSetForceCompact_NoMessagesDoesNotForce(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1", ContextWindow: 100000},
	}, nil)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry(t.TempDir(), sb)

	cfg := Config{
		Provider: mockProvider,
		Model:    mockProvider.Models()[0],
		Mode:     "agent",
	}

	a := New(cfg, registry)

	// No messages loaded — force should not trigger (nothing to compact)
	a.SetForceCompact()
	if a.ShouldCompact() {
		t.Fatal("ShouldCompact should be false with force but no messages")
	}
}

func TestSetForceCompact_NoModelDoesNotForce(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{
		{ID: "model1", Name: "Model 1"},
	}, nil)

	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry(t.TempDir(), sb)

	cfg := Config{
		Provider: mockProvider,
		Model:    nil, // no model
		Mode:     "agent",
	}

	a := New(cfg, registry)
	a.LoadHistoryMessages([]provider.Message{
		provider.NewUserMessage("Hello"),
		provider.NewAssistantMessage([]provider.ContentBlock{{Type: "text", Text: "Hi"}}),
	})

	a.SetForceCompact()
	if a.ShouldCompact() {
		t.Fatal("ShouldCompact should be false with force but no model")
	}
}

// --- MaxConsecutiveNoText tests ---

func TestMaxConsecutiveNoText_Default(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{{ID: "m1", Name: "M1"}}, nil)
	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry(t.TempDir(), sb)

	a := NewWithLoopConfig(AgentLoopConfig{
		Config: Config{
			Provider: mockProvider,
			Model:    mockProvider.Models()[0],
			Mode:     "agent",
		},
	}, registry)

	// Default MaxConsecutiveNoText should be 200 (MaxIterations default)
	// but the threshold is 95. Verify the config field is 0 (uses default).
	if a.config.MaxConsecutiveNoText != 0 {
		t.Fatalf("expected default MaxConsecutiveNoText=0, got %d", a.config.MaxConsecutiveNoText)
	}
}

func TestMaxConsecutiveNoText_Custom(t *testing.T) {
	mockProvider := provider.NewMockProvider("mock", []*provider.Model{{ID: "m1", Name: "M1"}}, nil)
	sb := sandbox.NewNoneSandbox()
	registry := tools.NewRegistry(t.TempDir(), sb)

	a := NewWithLoopConfig(AgentLoopConfig{
		Config: Config{
			Provider: mockProvider,
			Model:    mockProvider.Models()[0],
			Mode:     "agent",
		},
		MaxConsecutiveNoText: 10,
	}, registry)

	if a.config.MaxConsecutiveNoText != 10 {
		t.Fatalf("expected MaxConsecutiveNoText=10, got %d", a.config.MaxConsecutiveNoText)
	}
}
