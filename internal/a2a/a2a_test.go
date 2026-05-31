package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Port != 8093 {
		t.Errorf("expected port 8093, got %d", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Host)
	}
	if cfg.Enabled {
		t.Error("expected disabled by default")
	}
}

func TestGetListenAddr(t *testing.T) {
	cfg := &Config{Host: "127.0.0.1", Port: 9090}
	if addr := cfg.GetListenAddr(); addr != "127.0.0.1:9090" {
		t.Errorf("expected 127.0.0.1:9090, got %s", addr)
	}
}

func TestGetWorkDir(t *testing.T) {
	cfg := &Config{WorkDir: "/tmp/test"}
	if wd := cfg.GetWorkDir(); wd != "/tmp/test" {
		t.Errorf("expected /tmp/test, got %s", wd)
	}

	cfg2 := &Config{WorkDir: ""}
	wd := cfg2.GetWorkDir()
	if wd == "" {
		t.Error("expected non-empty work dir")
	}
}

func TestTaskStore(t *testing.T) {
	store := NewTaskStore()

	// Create
	task := store.Create("task_1")
	if task.ID != "task_1" {
		t.Errorf("expected task_1, got %s", task.ID)
	}
	if task.State != TaskStateSubmitted {
		t.Errorf("expected submitted, got %s", task.State)
	}

	// Get
	got := store.Get("task_1")
	if got == nil {
		t.Fatal("expected task, got nil")
	}
	if got.ID != "task_1" {
		t.Errorf("expected task_1, got %s", got.ID)
	}

	// Get non-existent
	if store.Get("nonexistent") != nil {
		t.Error("expected nil for non-existent task")
	}

	// Update state
	store.SetState("task_1", TaskStateWorking)
	task = store.Get("task_1")
	if task.State != TaskStateWorking {
		t.Errorf("expected working, got %s", task.State)
	}

	// Update
	task.State = TaskStateCompleted
	store.Update(task)
	task = store.Get("task_1")
	if task.State != TaskStateCompleted {
		t.Errorf("expected completed, got %s", task.State)
	}
}

func TestTaskStateTransitions(t *testing.T) {
	states := []TaskState{
		TaskStateSubmitted,
		TaskStateWorking,
		TaskStateCompleted,
		TaskStateFailed,
		TaskStateCanceled,
	}

	for _, state := range states {
		if string(state) == "" {
			t.Errorf("empty state in list")
		}
	}
}

func TestDefaultAgentCard(t *testing.T) {
	card := DefaultAgentCard("0.1.27", "http://localhost:8093")

	if card.Name != "VibeCoding" {
		t.Errorf("expected VibeCoding, got %s", card.Name)
	}
	if card.Version != "0.1.27" {
		t.Errorf("expected 0.1.27, got %s", card.Version)
	}
	if card.URL != "http://localhost:8093/a2a" {
		t.Errorf("expected http://localhost:8093/a2a, got %s", card.URL)
	}
	if !card.Capabilities.Streaming {
		t.Error("expected streaming=true")
	}
	if len(card.Skills) != 3 {
		t.Errorf("expected 3 skills, got %d", len(card.Skills))
	}
}

func TestHandleAgentCard(t *testing.T) {
	card := DefaultAgentCard("0.1.27", "http://localhost:8093")
	handler := HandleAgentCard(card)

	// GET request
	req := httptest.NewRequest("GET", "/.well-known/agent.json", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got AgentCard
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Name != "VibeCoding" {
		t.Errorf("expected VibeCoding, got %s", got.Name)
	}

	// POST should be rejected
	req2 := httptest.NewRequest("POST", "/.well-known/agent.json", nil)
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if w2.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w2.Code)
	}
}

func TestHandlerMessageSend(t *testing.T) {
	executor := &mockExecutor{
		response: "Hello from agent",
	}
	handler := NewHandler(executor)

	// Create a message/send request
	params := SendMessageParams{
		Message: &Message{
			Role:  "user",
			Parts: []MessagePart{{Type: "text", Text: "hello"}},
		},
	}
	paramsJSON, _ := json.Marshal(params)

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params:  paramsJSON,
		ID:      1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/a2a", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
}

func TestHandlerGetTask(t *testing.T) {
	executor := &mockExecutor{response: "done"}
	handler := NewHandler(executor)

	// Create a task first
	task := handler.GetTaskStore().Create("test_task")
	task.State = TaskStateCompleted
	handler.GetTaskStore().Update(task)

	// Get task via JSON-RPC
	params, _ := json.Marshal(map[string]string{"task_id": "test_task"})
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "task/get",
		Params:  params,
		ID:      2,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/a2a", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}
}

func TestHandlerCancelTask(t *testing.T) {
	executor := &mockExecutor{response: "done"}
	handler := NewHandler(executor)

	// Create a working task
	task := handler.GetTaskStore().Create("cancel_task")
	task.State = TaskStateWorking
	handler.GetTaskStore().Update(task)

	// Cancel via JSON-RPC
	params, _ := json.Marshal(map[string]string{"task_id": "cancel_task"})
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "task/cancel",
		Params:  params,
		ID:      3,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/a2a", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Verify task is canceled
	task = handler.GetTaskStore().Get("cancel_task")
	if task.State != TaskStateCanceled {
		t.Errorf("expected canceled, got %s", task.State)
	}
}

func TestHandlerInvalidJSON(t *testing.T) {
	executor := &mockExecutor{}
	handler := NewHandler(executor)

	req := httptest.NewRequest("POST", "/a2a", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp JSONRPCResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil {
		t.Error("expected error for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected error code -32700, got %d", resp.Error.Code)
	}
}

func TestHandlerInvalidMethod(t *testing.T) {
	executor := &mockExecutor{}
	handler := NewHandler(executor)

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "unknown/method",
		ID:      1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/a2a", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp JSONRPCResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestHandlerInvalidJSONRPCVersion(t *testing.T) {
	executor := &mockExecutor{}
	handler := NewHandler(executor)

	reqBody := JSONRPCRequest{
		JSONRPC: "1.0",
		Method:  "message/send",
		ID:      1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/a2a", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp JSONRPCResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil {
		t.Error("expected error for invalid jsonrpc version")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected error code -32600, got %d", resp.Error.Code)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	executor := &mockExecutor{}
	handler := NewHandler(executor)

	req := httptest.NewRequest("GET", "/a2a", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	executor := &mockExecutor{}
	handler := NewHandler(executor)

	ch := handler.Subscribe("task_1")
	if ch == nil {
		t.Fatal("expected channel")
	}

	// Send event
	handler.broadcast("task_1", TaskEvent{
		TaskID: "task_1",
		State:  TaskStateWorking,
	})

	select {
	case ev := <-ch:
		if ev.TaskID != "task_1" {
			t.Errorf("expected task_1, got %s", ev.TaskID)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}

	// Unsubscribe
	handler.Unsubscribe("task_1", ch)
}

func TestClientSendMessage(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a2a" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		task := &Task{
			ID:    "task_123",
			State: TaskStateCompleted,
			Artifacts: []Artifact{
				{Name: "response", Parts: []MessagePart{{Type: "text", Text: "Hello!"}}},
			},
		}

		json.NewEncoder(w).Encode(JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  task,
			ID:      req.ID,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	task, err := client.SendMessage(context.Background(), "", &Message{
		Role:  "user",
		Parts: []MessagePart{{Type: "text", Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID != "task_123" {
		t.Errorf("expected task_123, got %s", task.ID)
	}
	if task.State != TaskStateCompleted {
		t.Errorf("expected completed, got %s", task.State)
	}
}

func TestClientGetAgentCard(t *testing.T) {
	card := DefaultAgentCard("0.1.27", "http://localhost:8093")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/agent.json" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(card)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	got, err := client.GetAgentCard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "VibeCoding" {
		t.Errorf("expected VibeCoding, got %s", got.Name)
	}
}

func TestClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32000, Message: "task not found"},
			ID:      1,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.SendMessage(context.Background(), "", &Message{
		Role:  "user",
		Parts: []MessagePart{{Type: "text", Text: "hello"}},
	})
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Errorf("expected 'task not found' in error, got: %v", err)
	}
}

func TestClientWithAuth(t *testing.T) {
	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Authorization")
		task := &Task{ID: "t1", State: TaskStateCompleted}
		json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: task, ID: 1})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.SendMessage(context.Background(), "", &Message{
		Role:  "user",
		Parts: []MessagePart{{Type: "text", Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotToken != "Bearer test-token" {
		t.Errorf("expected 'Bearer test-token', got '%s'", gotToken)
	}
}

// mockExecutor implements AgentExecutor for testing.
type mockExecutor struct {
	response string
	err      error
}

func (m *mockExecutor) ExecuteTask(ctx context.Context, task *Task, msg *Message) (<-chan TaskEvent, error) {
	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan TaskEvent, 10)
	go func() {
		defer close(ch)
		ch <- TaskEvent{
			TaskID:    task.ID,
			State:     TaskStateWorking,
			Message:   &Message{Role: "agent", Parts: []MessagePart{{Type: "text", Text: m.response}}},
			Timestamp: time.Now(),
		}
		ch <- TaskEvent{
			TaskID: task.ID,
			State:  TaskStateCompleted,
			Artifact: &Artifact{
				Name:  "response",
				Parts: []MessagePart{{Type: "text", Text: m.response}},
			},
			Timestamp: time.Now(),
		}
	}()

	return ch, nil
}
