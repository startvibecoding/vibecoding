package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SSEWriter helps write Server-Sent Events to an HTTP response.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	model   string
	id      string
	created int64
	sessID  string
}

// NewSSEWriter creates an SSE writer and sets the appropriate headers.
func NewSSEWriter(w http.ResponseWriter, model, sessionID string) *SSEWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	flusher, _ := w.(http.Flusher)

	id := newCompletionID()
	return &SSEWriter{
		w:       w,
		flusher: flusher,
		model:   model,
		id:      id,
		created: time.Now().Unix(),
		sessID:  sessionID,
	}
}

// WriteContentDelta sends a text content delta chunk.
func (s *SSEWriter) WriteContentDelta(content string) {
	chunk := ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Delta: &ResponseMessage{Content: content},
			},
		},
		XSessionID: s.sessID,
	}
	s.writeData(chunk)
}

// WriteRoleDelta sends the initial role delta.
func (s *SSEWriter) WriteRoleDelta() {
	chunk := ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Delta: &ResponseMessage{Role: "assistant"},
			},
		},
		XSessionID: s.sessID,
	}
	s.writeData(chunk)
}

// WriteToolStatusContent sends a tool status in content mode (text in content delta).
// Uses a compact title like "read: path=main.go" rather than dumping full args.
func (s *SSEWriter) WriteToolStatusContent(title, status string) {
	text := fmt.Sprintf("[%s] %s\n", status, title)
	s.WriteContentDelta(text)
}

// WriteToolResult sends formatted tool output based on detail level.
func (s *SSEWriter) WriteToolResult(tc *toolCallInfo, detail string) {
	text := formatToolResult(tc, detail)
	s.WriteContentDelta(text)
}

// WriteToolStatusEvent sends a tool status as an SSE event (sse_event mode).
func (s *SSEWriter) WriteToolStatusEvent(toolName, status string, args map[string]any) {
	evt := ToolStatusEvent{
		Tool:   toolName,
		Status: status,
		Args:   args,
	}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(s.w, "event: tool_status\ndata: %s\n\n", data)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// WriteDone sends the final chunk with finish_reason and usage, then [DONE].
func (s *SSEWriter) WriteDone(usage *CompletionUsage) {
	finishReason := "stop"
	chunk := ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.model,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Delta:        &ResponseMessage{},
				FinishReason: &finishReason,
			},
		},
		Usage:      usage,
		XSessionID: s.sessID,
	}
	s.writeData(chunk)

	// Send [DONE] sentinel
	fmt.Fprintf(s.w, "data: [DONE]\n\n")
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// WriteError sends an error as a final chunk.
func (s *SSEWriter) WriteError(errMsg string) {
	finishReason := "stop"
	chunk := ChatCompletionChunk{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.model,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Delta:        &ResponseMessage{Content: "\n\n[Error: " + errMsg + "]"},
				FinishReason: &finishReason,
			},
		},
		XSessionID: s.sessID,
	}
	s.writeData(chunk)
	fmt.Fprintf(s.w, "data: [DONE]\n\n")
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *SSEWriter) writeData(v any) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}
