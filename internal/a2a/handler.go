package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      any             `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      any           `json:"id"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SendMessageParams represents the params for message/send.
type SendMessageParams struct {
	TaskID  string   `json:"task_id,omitempty"`
	Message *Message `json:"message"`
}

// AgentExecutor processes A2A tasks by running them through the agent loop.
type AgentExecutor interface {
	ExecuteTask(ctx context.Context, task *Task, msg *Message) (<-chan TaskEvent, error)
}

// Handler handles A2A JSON-RPC requests.
type Handler struct {
	taskStore   *TaskStore
	executor    AgentExecutor
	mu          sync.RWMutex
	subscribers map[string][]chan TaskEvent
}

// NewHandler creates a new A2A handler.
func NewHandler(executor AgentExecutor) *Handler {
	return &Handler{
		taskStore:   NewTaskStore(),
		executor:    executor,
		subscribers: make(map[string][]chan TaskEvent),
	}
}

// GetTaskStore returns the task store.
func (h *Handler) GetTaskStore() *TaskStore {
	return h.taskStore
}

// ServeHTTP handles A2A JSON-RPC requests at /a2a.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, nil, -32700, "Parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		h.writeError(w, req.ID, -32600, "Invalid Request: jsonrpc must be \"2.0\"")
		return
	}

	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")

	switch req.Method {
	case "message/send":
		h.handleSendMessage(w, r, &req, isSSE)
	case "task/get":
		h.handleGetTask(w, &req)
	case "task/cancel":
		h.handleCancelTask(w, &req)
	default:
		h.writeError(w, req.ID, -32601, "Method not found: "+req.Method)
	}
}

// handleSendMessage processes message/send.
func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request, req *JSONRPCRequest, isSSE bool) {
	var params SendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		h.writeError(w, req.ID, -32602, "Invalid params: "+err.Error())
		return
	}
	if params.Message == nil {
		h.writeError(w, req.ID, -32602, "Invalid params: message is required")
		return
	}

	// Create or get task
	var task *Task
	if params.TaskID != "" {
		task = h.taskStore.Get(params.TaskID)
		if task == nil {
			h.writeError(w, req.ID, -32000, "Task not found: "+params.TaskID)
			return
		}
	} else {
		taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
		task = h.taskStore.Create(taskID)
	}

	task.Message = params.Message
	h.taskStore.SetState(task.ID, TaskStateWorking)

	if isSSE {
		h.streamResponse(w, r, task, params.Message)
	} else {
		h.syncResponse(w, r, task, params.Message, req.ID)
	}
}

// syncResponse processes the task synchronously.
func (h *Handler) syncResponse(w http.ResponseWriter, r *http.Request, task *Task, msg *Message, reqID any) {
	eventCh, err := h.executor.ExecuteTask(r.Context(), task, msg)
	if err != nil {
		task.State = TaskStateFailed
		task.Error = &TaskError{Code: -32000, Message: err.Error()}
		h.taskStore.Update(task)
		h.writeError(w, reqID, -32000, err.Error())
		return
	}

	var lastEvent TaskEvent
	for ev := range eventCh {
		lastEvent = ev
		h.broadcast(task.ID, ev)
	}

	task.State = lastEvent.State
	if lastEvent.Error != nil {
		task.Error = lastEvent.Error
	}
	if lastEvent.Artifact != nil {
		task.Artifacts = append(task.Artifacts, *lastEvent.Artifact)
	}
	h.taskStore.Update(task)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: task, ID: reqID})
}

// streamResponse processes the task with SSE streaming.
func (h *Handler) streamResponse(w http.ResponseWriter, r *http.Request, task *Task, msg *Message) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventCh, err := h.executor.ExecuteTask(r.Context(), task, msg)
	if err != nil {
		task.State = TaskStateFailed
		task.Error = &TaskError{Code: -32000, Message: err.Error()}
		h.taskStore.Update(task)
		h.writeSSE(w, flusher, TaskEvent{TaskID: task.ID, State: TaskStateFailed, Error: task.Error, Timestamp: time.Now()})
		return
	}

	for ev := range eventCh {
		h.writeSSE(w, flusher, ev)
		h.broadcast(task.ID, ev)
		if ev.State == TaskStateCompleted || ev.State == TaskStateFailed {
			task.State = ev.State
			if ev.Error != nil {
				task.Error = ev.Error
			}
			if ev.Artifact != nil {
				task.Artifacts = append(task.Artifacts, *ev.Artifact)
			}
			h.taskStore.Update(task)
		}
	}
}

// handleGetTask returns the current state of a task.
func (h *Handler) handleGetTask(w http.ResponseWriter, req *JSONRPCRequest) {
	var params struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		h.writeError(w, req.ID, -32602, "Invalid params: "+err.Error())
		return
	}
	task := h.taskStore.Get(params.TaskID)
	if task == nil {
		h.writeError(w, req.ID, -32000, "Task not found: "+params.TaskID)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: task, ID: req.ID})
}

// handleCancelTask cancels a running task.
func (h *Handler) handleCancelTask(w http.ResponseWriter, req *JSONRPCRequest) {
	var params struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		h.writeError(w, req.ID, -32602, "Invalid params: "+err.Error())
		return
	}
	task := h.taskStore.Get(params.TaskID)
	if task == nil {
		h.writeError(w, req.ID, -32000, "Task not found: "+params.TaskID)
		return
	}
	if task.State != TaskStateWorking && task.State != TaskStateSubmitted {
		h.writeError(w, req.ID, -32000, "Task cannot be canceled in state: "+string(task.State))
		return
	}
	task.State = TaskStateCanceled
	h.taskStore.Update(task)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: task, ID: req.ID})
}

// Subscribe adds an SSE subscriber for task events.
func (h *Handler) Subscribe(taskID string) chan TaskEvent {
	ch := make(chan TaskEvent, 100)
	h.mu.Lock()
	h.subscribers[taskID] = append(h.subscribers[taskID], ch)
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes an SSE subscriber.
func (h *Handler) Unsubscribe(taskID string, ch chan TaskEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			h.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}

// broadcast sends an event to all subscribers of a task.
func (h *Handler) broadcast(taskID string, event TaskEvent) {
	h.mu.RLock()
	subs := h.subscribers[taskID]
	h.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

// writeError writes a JSON-RPC error response.
func (h *Handler) writeError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: msg},
		ID:      id,
	})
}

// writeSSE writes an SSE event.
func (h *Handler) writeSSE(w http.ResponseWriter, flusher http.Flusher, event TaskEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// SubscribeSSE handles SSE subscription for task events at /a2a/events.
func (h *Handler) SubscribeSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.Subscribe(taskID)
	defer h.Unsubscribe(taskID, ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if event.State == TaskStateCompleted || event.State == TaskStateFailed || event.State == TaskStateCanceled {
				return
			}
		}
	}
}
