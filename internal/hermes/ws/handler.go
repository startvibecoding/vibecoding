package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// WSEvent is the event type sent over WebSocket.
// Mapped from agent.Event by the dispatcher.
type WSEvent struct {
	Type    string         `json:"type"`
	Content string         `json:"content,omitempty"`

	// Connected event fields
	SessionID string `json:"session_id,omitempty"`
	Version   string `json:"version,omitempty"`
	Model     string `json:"model,omitempty"`
	WorkDir   string `json:"work_dir,omitempty"`

	// Tool event fields
	Tool   string         `json:"tool,omitempty"`
	CallID string         `json:"call_id,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Result string         `json:"result,omitempty"`

	// Diff fields
	Path string `json:"path,omitempty"`
	Diff string `json:"diff,omitempty"`

	// Approval fields
	ApprovalID string `json:"approval_id,omitempty"`
	RiskLevel  string `json:"risk_level,omitempty"`
	Approved   bool   `json:"approved,omitempty"`

	// Plan fields
	Plan *PlanData `json:"plan,omitempty"`

	// Usage fields
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`

	// Done/Error fields
	StopReason string `json:"stop_reason,omitempty"`
	Message    string `json:"message,omitempty"`
	Command    string `json:"command,omitempty"`
	Error      bool   `json:"error,omitempty"`
	Code       string `json:"code,omitempty"`
}

// PlanData represents a task plan for the plan_update event.
type PlanData struct {
	Title string     `json:"title"`
	Steps []PlanStep `json:"steps"`
}

// PlanStep is a single step in a task plan.
type PlanStep struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// ClientMessage represents a message from the WebSocket client.
type ClientMessage struct {
	Type       string `json:"type"`
	Content    string `json:"content,omitempty"`
	ApprovalID string `json:"approval_id,omitempty"`
	Approved   bool   `json:"approved,omitempty"`
}

// WSConn wraps a WebSocket connection with metadata.
type WSConn struct {
	ID     string
	ws     *websocket.Conn
	sendMu sync.Mutex
	closed bool
	mu     sync.Mutex
}

// Send sends a WSEvent to the client.
func (c *WSConn) Send(ev WSEvent) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	return websocket.JSON.Send(c.ws, ev)
}

// Close closes the WebSocket connection.
func (c *WSConn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.ws.Close()
	}
}

// handleWebSocket handles WebSocket upgrade and message loop.
func (gw *Gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Auth check
	if gw.authToken != "" {
		token := r.URL.Query().Get("token")
		if token != gw.authToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	handler := websocket.Handler(func(ws *websocket.Conn) {
		connID := generateConnID()
		conn := &WSConn{
			ID: connID,
			ws: ws,
		}

		// Register connection
		gw.connMu.Lock()
		gw.conns[connID] = conn
		gw.connMu.Unlock()

		defer func() {
			conn.Close()
			gw.connMu.Lock()
			delete(gw.conns, connID)
			gw.connMu.Unlock()
		}()

		// Send connected event
		conn.Send(WSEvent{
			Type:      "connected",
			SessionID: "hermes/ws/" + connID,
			Version:   gw.version,
		})

		log.Printf("WebSocket client connected: %s", connID)

		// Message loop
		for {
			var msg ClientMessage
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				log.Printf("WebSocket read error (%s): %v", connID, err)
				return
			}

			switch msg.Type {
			case "ping":
				conn.Send(WSEvent{Type: "pong"})

			case "message", "command":
				text := msg.Content
				if msg.Type == "command" && text != "" && text[0] != '/' {
					text = "/" + text
				}
				gw.handleWSChat(r.Context(), conn, connID, text)

			case "approval":
				if msg.ApprovalID != "" && gw.dispatcher != nil {
					gw.dispatcher.ResolveApproval(msg.ApprovalID, msg.Approved)
				}
				conn.Send(WSEvent{Type: "status", Message: fmt.Sprintf("Approval %s: %v", msg.ApprovalID, msg.Approved)})

			default:
				conn.Send(WSEvent{
					Type:    "error",
					Message: "unknown message type: " + msg.Type,
				})
			}
		}
	})

	handler.ServeHTTP(w, r)
}

// handleWSChat dispatches a chat message and streams events back.
func (gw *Gateway) handleWSChat(ctx context.Context, conn *WSConn, connID, text string) {
	gw.mu.RLock()
	dispatcher := gw.dispatcher
	gw.mu.RUnlock()

	if dispatcher == nil {
		conn.Send(WSEvent{Type: "error", Message: "dispatcher not ready"})
		return
	}

	eventCh := make(chan WSEvent, 100)
	go func() {
		defer close(eventCh)
		if err := dispatcher.HandleWSMessage(ctx, connID, text, eventCh); err != nil {
			eventCh <- WSEvent{Type: "error", Message: err.Error()}
		}
	}()

	for ev := range eventCh {
		if err := conn.Send(ev); err != nil {
			log.Printf("WebSocket send error (%s): %v", connID, err)
			return
		}
	}
}

// generateConnID generates a random connection ID.
func generateConnID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// keepAlive sends periodic pings to keep the connection alive.
func (c *WSConn) keepAlive(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return
		}
		c.Send(WSEvent{Type: "pong"})
	}
}
