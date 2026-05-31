package gateway

import (
	"encoding/json"
	"fmt"
	"time"
)

// --- OpenAI-compatible request types ---

// ChatCompletionRequest represents the OpenAI chat completions request.
type ChatCompletionRequest struct {
	Model       string           `json:"model,omitempty"`
	Messages    []RequestMessage `json:"messages"`
	Stream      bool             `json:"stream,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`

	// VibeCoding extensions
	XSessionID  string `json:"x_session_id,omitempty"`
	XMode       string `json:"x_mode,omitempty"`
	XWorkingDir string `json:"x_working_dir,omitempty"`
}

// RequestMessage represents a message in the OpenAI request.
type RequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// --- OpenAI-compatible response types ---

// ChatCompletionResponse is the non-streaming response.
type ChatCompletionResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []ChatCompletionChoice   `json:"choices"`
	Usage   *CompletionUsage         `json:"usage,omitempty"`

	// VibeCoding extensions
	XSessionID string         `json:"x_session_id,omitempty"`
	XCommand   string         `json:"x_command,omitempty"`
	XToolCalls []XToolCall    `json:"x_tool_calls,omitempty"`
}

// ChatCompletionChoice is a single choice in the response.
type ChatCompletionChoice struct {
	Index        int              `json:"index"`
	Message      *ResponseMessage `json:"message,omitempty"`
	Delta        *ResponseMessage `json:"delta,omitempty"`
	FinishReason *string          `json:"finish_reason"`
}

// ResponseMessage is the assistant's response message.
type ResponseMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// CompletionUsage tracks token counts.
type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// XToolCall is a VibeCoding extension for exposing tool call info.
type XToolCall struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args,omitempty"`
	Status string         `json:"status"` // "running", "completed", "failed"
}

// --- Streaming chunk types ---

// ChatCompletionChunk is the streaming chunk response.
type ChatCompletionChunk struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *CompletionUsage       `json:"usage,omitempty"`

	// VibeCoding extensions
	XSessionID string `json:"x_session_id,omitempty"`
}

// --- SSE tool_status event (for sse_event mode) ---

// ToolStatusEvent is sent via SSE event: tool_status.
type ToolStatusEvent struct {
	Tool   string         `json:"tool"`
	Status string         `json:"status"` // "running", "completed", "failed"
	Args   map[string]any `json:"args,omitempty"`
}

// --- Model list types ---

// ModelListResponse is the response for GET /v1/models.
type ModelListResponse struct {
	Object string      `json:"object"`
	Data   []ModelItem `json:"data"`
}

// ModelItem represents one model in the list.
type ModelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// --- Health ---

// HealthResponse is the response for GET /health.
type HealthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Sessions int    `json:"sessions"`
}

// --- Error response ---

// ErrorResponse is the standard OpenAI error format.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// --- Helpers ---

func newCompletionID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}

func newCommandCompletionID() string {
	return fmt.Sprintf("chatcmpl-cmd-%d", time.Now().UnixNano())
}

func stringPtr(s string) *string {
	return &s
}

func marshalJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
