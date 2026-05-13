package provider

import (
	"context"
	"testing"
)

func TestNewBaseProvider(t *testing.T) {
	models := []*Model{
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
}

func TestGetModel(t *testing.T) {
	models := []*Model{
		{ID: "model1", Name: "Model 1"},
		{ID: "model2", Name: "Model 2"},
	}

	p := NewBaseProvider("test", models)

	// Test existing model
	m := p.GetModel("model1")
	if m == nil {
		t.Fatal("expected model, got nil")
	}
	if m.Name != "Model 1" {
		t.Errorf("expected name 'Model 1', got '%s'", m.Name)
	}

	// Test non-existing model
	m = p.GetModel("model3")
	if m != nil {
		t.Errorf("expected nil, got model '%s'", m.Name)
	}
}

func TestMockProvider(t *testing.T) {
	models := []*Model{
		{ID: "model1", Name: "Model 1"},
	}

	responses := []StreamEvent{
		{Type: StreamStart},
		{Type: StreamTextDelta, TextDelta: "Hello"},
		{Type: StreamDone},
	}

	p := NewMockProvider("mock", models, responses)

	if p.Name() != "mock" {
		t.Errorf("expected name 'mock', got '%s'", p.Name())
	}

	if p.GetCallCount() != 0 {
		t.Errorf("expected call count 0, got %d", p.GetCallCount())
	}

	// Test Models
	if len(p.Models()) != 1 {
		t.Errorf("expected 1 model, got %d", len(p.Models()))
	}

	// Test GetModel
	m := p.GetModel("model1")
	if m == nil {
		t.Fatal("expected model, got nil")
	}

	m = p.GetModel("nonexistent")
	if m != nil {
		t.Error("expected nil for nonexistent model")
	}

	// Test Chat
	ch := p.Chat(context.Background(), ChatParams{})

	var events []StreamEvent
	for event := range ch {
		events = append(events, event)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	if p.GetCallCount() != 1 {
		t.Errorf("expected call count 1, got %d", p.GetCallCount())
	}
}

func TestMockProviderWithContext(t *testing.T) {
	models := []*Model{
		{ID: "model1", Name: "Model 1"},
	}

	responses := []StreamEvent{
		{Type: StreamStart},
		{Type: StreamTextDelta, TextDelta: "Hello"},
		{Type: StreamDone},
	}

	p := NewMockProvider("mock", models, responses)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ch := p.Chat(ctx, ChatParams{})

	var events []StreamEvent
	for event := range ch {
		events = append(events, event)
	}

	// Should have error event due to cancelled context
	hasError := false
	for _, event := range events {
		if event.Type == StreamError {
			hasError = true
		}
	}

	if !hasError {
		t.Error("expected error event due to cancelled context")
	}
}

func TestModelPricing(t *testing.T) {
	usage := &Usage{
		Input:       1000,
		Output:      500,
		CacheRead:   100,
		CacheWrite:  50,
		TotalTokens: 1650,
	}

	model := &Model{
		Cost: ModelPricing{
			Input:      3.0,
			Output:     15.0,
			CacheRead:  0.3,
			CacheWrite: 3.75,
		},
	}

	usage.CalculateCost(model)

	// Input: 1000 * 3.0 / 1_000_000 = 0.003
	if usage.Cost.Input != 0.003 {
		t.Errorf("expected input cost 0.003, got %f", usage.Cost.Input)
	}

	// Output: 500 * 15.0 / 1_000_000 = 0.0075
	if usage.Cost.Output != 0.0075 {
		t.Errorf("expected output cost 0.0075, got %f", usage.Cost.Output)
	}

	// Total
	expectedTotal := 0.003 + 0.0075 + 0.00003 + 0.0001875
	if usage.Cost.Total != expectedTotal {
		t.Errorf("expected total cost %f, got %f", expectedTotal, usage.Cost.Total)
	}
}

func TestModelPricingNilModel(t *testing.T) {
	usage := &Usage{
		Input:  1000,
		Output: 500,
	}

	usage.CalculateCost(nil)

	if usage.Cost.Total != 0 {
		t.Errorf("expected 0 cost for nil model, got %f", usage.Cost.Total)
	}
}

func TestNewUserMessage(t *testing.T) {
	msg := NewUserMessage("Hello")

	if msg.Role != "user" {
		t.Errorf("expected role 'user', got '%s'", msg.Role)
	}

	if msg.Content != "Hello" {
		t.Errorf("expected content 'Hello', got '%s'", msg.Content)
	}

	if msg.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestNewAssistantMessage(t *testing.T) {
	contents := []ContentBlock{
		{Type: "text", Text: "Hello"},
		{Type: "thinking", Thinking: "Let me think..."},
	}

	msg := NewAssistantMessage(contents)

	if msg.Role != "assistant" {
		t.Errorf("expected role 'assistant', got '%s'", msg.Role)
	}

	if len(msg.Contents) != 2 {
		t.Errorf("expected 2 contents, got %d", len(msg.Contents))
	}

	if msg.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestNewToolResultMessage(t *testing.T) {
	msg := NewToolResultMessage("call_1", "ls", "file1.txt\nfile2.txt", false)

	if msg.Role != "toolResult" {
		t.Errorf("expected role 'toolResult', got '%s'", msg.Role)
	}

	if msg.ToolCallID != "call_1" {
		t.Errorf("expected toolCallID 'call_1', got '%s'", msg.ToolCallID)
	}

	if msg.ToolName != "ls" {
		t.Errorf("expected toolName 'ls', got '%s'", msg.ToolName)
	}

	if msg.Content != "file1.txt\nfile2.txt" {
		t.Errorf("expected content 'file1.txt\\nfile2.txt', got '%s'", msg.Content)
	}

	if msg.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestNewToolResultMessageError(t *testing.T) {
	msg := NewToolResultMessage("call_1", "bash", "command not found", true)

	if !msg.IsError {
		t.Error("expected IsError to be true")
	}
}

func TestStreamEventTypes(t *testing.T) {
	events := []StreamEvent{
		{Type: StreamStart},
		{Type: StreamTextDelta, TextDelta: "Hello"},
		{Type: StreamThinkDelta, ThinkDelta: "Thinking..."},
		{Type: StreamToolCall, ToolCall: &ToolCallBlock{ID: "1", Name: "ls"}},
		{Type: StreamUsage, Usage: &Usage{Input: 100, Output: 50}},
		{Type: StreamDone, StopReason: "end_turn"},
		{Type: StreamError, Error: nil},
	}

	for _, event := range events {
		if event.Type == 0 && event != events[0] {
			t.Error("expected non-zero event type")
		}
	}
}

func TestThinkingLevels(t *testing.T) {
	levels := []ThinkingLevel{
		ThinkingOff,
		ThinkingMinimal,
		ThinkingLow,
		ThinkingMedium,
		ThinkingHigh,
		ThinkingXHigh,
	}

	expected := []string{"off", "minimal", "low", "medium", "high", "xhigh"}

	for i, level := range levels {
		if string(level) != expected[i] {
			t.Errorf("expected '%s', got '%s'", expected[i], string(level))
		}
	}
}

func TestModel(t *testing.T) {
	model := &Model{
		ID:            "gpt-4o",
		Name:          "GPT-4o",
		Provider:      "openai",
		Reasoning:     false,
		Input:         []string{"text", "image"},
		Cost:          ModelPricing{Input: 2.5, Output: 10.0},
		ContextWindow: 128000,
		MaxTokens:     16384,
	}

	if model.ID != "gpt-4o" {
		t.Errorf("expected ID 'gpt-4o', got '%s'", model.ID)
	}

	if model.Name != "GPT-4o" {
		t.Errorf("expected Name 'GPT-4o', got '%s'", model.Name)
	}

	if model.Provider != "openai" {
		t.Errorf("expected Provider 'openai', got '%s'", model.Provider)
	}

	if model.Reasoning {
		t.Error("expected Reasoning to be false")
	}

	if len(model.Input) != 2 {
		t.Errorf("expected 2 input types, got %d", len(model.Input))
	}

	if model.ContextWindow != 128000 {
		t.Errorf("expected ContextWindow 128000, got %d", model.ContextWindow)
	}

	if model.MaxTokens != 16384 {
		t.Errorf("expected MaxTokens 16384, got %d", model.MaxTokens)
	}
}

func TestContentBlock(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Hello",
	}

	if block.Type != "text" {
		t.Errorf("expected Type 'text', got '%s'", block.Type)
	}

	if block.Text != "Hello" {
		t.Errorf("expected Text 'Hello', got '%s'", block.Text)
	}
}

func TestToolCallBlock(t *testing.T) {
	block := ToolCallBlock{
		ID:        "call_1",
		Name:      "ls",
		Arguments: []byte(`{"path": "."}`),
	}

	if block.ID != "call_1" {
		t.Errorf("expected ID 'call_1', got '%s'", block.ID)
	}

	if block.Name != "ls" {
		t.Errorf("expected Name 'ls', got '%s'", block.Name)
	}

	if string(block.Arguments) != `{"path": "."}` {
		t.Errorf("expected Arguments '{\"path\": \".\"}', got '%s'", string(block.Arguments))
	}
}

func TestChatParams(t *testing.T) {
	params := ChatParams{
		Messages: []Message{
			NewUserMessage("Hello"),
		},
		Tools: []ToolDefinition{
			{Name: "ls", Description: "List files"},
		},
		SystemPrompt:  "You are a helpful assistant",
		ThinkingLevel: ThinkingMedium,
		MaxTokens:     1000,
		Abort:         make(chan struct{}),
	}

	if len(params.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(params.Messages))
	}

	if len(params.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(params.Tools))
	}

	if params.SystemPrompt != "You are a helpful assistant" {
		t.Errorf("expected system prompt 'You are a helpful assistant', got '%s'", params.SystemPrompt)
	}

	if params.ThinkingLevel != ThinkingMedium {
		t.Errorf("expected thinking level 'medium', got '%s'", params.ThinkingLevel)
	}

	if params.MaxTokens != 1000 {
		t.Errorf("expected max tokens 1000, got %d", params.MaxTokens)
	}
}
