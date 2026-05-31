// Package ws implements the WebSocket + HTTP gateway for Hermes mode.
package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Gateway is the WebSocket + HTTP gateway server.
type Gateway struct {
	mu         sync.RWMutex
	mux        *http.ServeMux
	httpServer *http.Server
	dispatcher Dispatcher
	platforms  PlatformStatusProvider
	memoryStore MemoryStore
	version    string
	authToken  string
	startTime  time.Time

	// Active WebSocket connections
	connMu sync.RWMutex
	conns  map[string]*WSConn
}

// Dispatcher is the interface the gateway uses to dispatch messages.
type Dispatcher interface {
	HandleWSMessage(ctx context.Context, connID, text string, eventCh chan<- WSEvent) error
	ListSessions() []SessionInfo
	RemoveSession(key string)
	ResolveApproval(approvalID string, approved bool) bool
}

// SessionInfo is a simplified session view for API responses.
type SessionInfo struct {
	ID           string    `json:"id"`
	Platform     string    `json:"platform"`
	UserID       string    `json:"user_id"`
	WorkDir      string    `json:"work_dir"`
	Mode         string    `json:"mode,omitempty"`
	Model        string    `json:"model,omitempty"`
	MessageCount int       `json:"message_count"`
	LastActive   time.Time `json:"last_active"`
	Preview      string    `json:"preview,omitempty"`
}

// PlatformStatus represents a messaging platform's connection status.
type PlatformStatus struct {
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	Connected   bool     `json:"connected"`
	WorkDir     string   `json:"work_dir,omitempty"`
	ActiveUsers []string `json:"active_users,omitempty"`
	LoginStatus string   `json:"login_status,omitempty"`
}

// PlatformStatusProvider supplies platform connection status.
type PlatformStatusProvider interface {
	GetPlatformStatuses() []PlatformStatus
}

// NewGateway creates a new gateway server.
func NewGateway(listenAddr, authToken, version string) *Gateway {
	gw := &Gateway{
		mux:       http.NewServeMux(),
		version:   version,
		authToken: authToken,
		startTime: time.Now(),
		conns:     make(map[string]*WSConn),
	}

	gw.httpServer = &http.Server{
		Addr:         listenAddr,
		Handler:      gw.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Register routes
	gw.mux.HandleFunc("/ws", gw.handleWebSocket)
	gw.mux.HandleFunc("/api/health", gw.handleHealth)
	gw.mux.HandleFunc("/api/status", gw.withAuth(gw.handleStatus))
	gw.mux.HandleFunc("/api/sessions", gw.withAuth(gw.handleSessions))
	gw.mux.HandleFunc("/api/sessions/", gw.withAuth(gw.handleSessionByID))
	gw.mux.HandleFunc("/api/memory", gw.withAuth(gw.handleMemory))
	gw.mux.HandleFunc("/api/platforms", gw.withAuth(gw.handlePlatforms))

	return gw
}

// RegisterHandler registers an additional HTTP handler on the gateway mux.
func (gw *Gateway) RegisterHandler(pattern string, handler http.Handler) {
	gw.mux.Handle(pattern, handler)
}

// SetDispatcher sets the message dispatcher.
func (gw *Gateway) SetDispatcher(d Dispatcher) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.dispatcher = d
}

// SetPlatformStatusProvider sets the platform status provider.
func (gw *Gateway) SetPlatformStatusProvider(p PlatformStatusProvider) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.platforms = p
}

// MemoryStore provides read/write access to memory.md.
type MemoryStore interface {
	Read() (content string, path string, source string, err error)
	WriteAll(content string) error
}

// SetMemoryStore sets the memory store for the /api/memory endpoint.
func (gw *Gateway) SetMemoryStore(s MemoryStore) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.memoryStore = s
}

// GetMux returns the HTTP mux for registering additional routes.
func (gw *Gateway) GetMux() *http.ServeMux {
	return gw.mux
}

// Start starts the HTTP server. Blocks until stopped.
func (gw *Gateway) Start() error {
	log.Printf("Hermes gateway listening on %s", gw.httpServer.Addr)
	return gw.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the gateway.
func (gw *Gateway) Stop(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Close all WebSocket connections
	gw.connMu.Lock()
	for _, conn := range gw.conns {
		conn.Close()
	}
	gw.connMu.Unlock()

	return gw.httpServer.Shutdown(ctx)
}

// ConnectionCount returns the number of active WebSocket connections.
func (gw *Gateway) ConnectionCount() int {
	gw.connMu.RLock()
	defer gw.connMu.RUnlock()
	return len(gw.conns)
}

// --- Auth middleware ---

func (gw *Gateway) withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if gw.authToken != "" {
			token := r.Header.Get("Authorization")
			if token == "" {
				token = r.URL.Query().Get("token")
			} else if len(token) > 7 && token[:7] == "Bearer " {
				token = token[7:]
			}
			if token != gw.authToken {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		handler(w, r)
	}
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
