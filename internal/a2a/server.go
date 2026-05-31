package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server is the A2A HTTP server.
type Server struct {
	cfg     *Config
	version string
	handler *Handler
	mux     *http.ServeMux
	httpSrv *http.Server
	card    *AgentCard
}

// NewServer creates a new A2A server.
func NewServer(cfg *Config, version string, executor AgentExecutor) *Server {
	handler := NewHandler(executor)
	mux := http.NewServeMux()

	serverURL := fmt.Sprintf("http://%s", cfg.GetListenAddr())
	card := DefaultAgentCard(version, serverURL)
	if cfg.AgentCard != nil {
		if cfg.AgentCard.Name != "" {
			card.Name = cfg.AgentCard.Name
		}
		if cfg.AgentCard.Description != "" {
			card.Description = cfg.AgentCard.Description
		}
		if cfg.AgentCard.Version != "" {
			card.Version = cfg.AgentCard.Version
		}
	}

	s := &Server{
		cfg:     cfg,
		version: version,
		handler: handler,
		mux:     mux,
		card:    card,
	}

	s.registerRoutes()
	return s
}

// GetHandler returns the A2A handler (for integration mode).
func (s *Server) GetHandler() *Handler {
	return s.handler
}

// GetCard returns the Agent Card.
func (s *Server) GetCard() *AgentCard {
	return s.card
}

// registerRoutes registers all A2A HTTP routes.
func (s *Server) registerRoutes() {
	// Agent Card
	s.mux.HandleFunc("/.well-known/agent.json", HandleAgentCard(s.card))

	// JSON-RPC endpoint
	s.mux.Handle("/a2a", s.handler)

	// REST-style endpoints (alternative to JSON-RPC)
	s.mux.HandleFunc("/a2a/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		isSSE := r.Header.Get("Accept") == "text/event-stream"
		var req struct {
			TaskID  string   `json:"task_id,omitempty"`
			Message *Message `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Message == nil {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}
		var task *Task
		if req.TaskID != "" {
			task = s.handler.taskStore.Get(req.TaskID)
			if task == nil {
				http.Error(w, "task not found", http.StatusNotFound)
				return
			}
		} else {
			taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
			task = s.handler.taskStore.Create(taskID)
		}
		task.Message = req.Message
		s.handler.taskStore.SetState(task.ID, TaskStateWorking)
		if isSSE {
			s.handler.streamResponse(w, r, task, req.Message)
		} else {
			s.handler.syncResponse(w, r, task, req.Message, nil)
		}
	})

	s.mux.HandleFunc("/a2a/task", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		taskID := r.URL.Query().Get("task_id")
		if taskID == "" {
			http.Error(w, "task_id required", http.StatusBadRequest)
			return
		}
		task := s.handler.taskStore.Get(taskID)
		if task == nil {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	})

	s.mux.HandleFunc("/a2a/task/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			TaskID string `json:"task_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		task := s.handler.taskStore.Get(req.TaskID)
		if task == nil {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		if task.State != TaskStateWorking && task.State != TaskStateSubmitted {
			http.Error(w, "cannot cancel task in state: "+string(task.State), http.StatusConflict)
			return
		}
		task.State = TaskStateCanceled
		s.handler.taskStore.Update(task)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	})

	// SSE event stream
	s.mux.HandleFunc("/a2a/events", s.handler.SubscribeSSE)
}

// RegisterRoutes registers A2A routes on an external mux (for integration mode).
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/.well-known/agent.json", HandleAgentCard(s.card))
	mux.Handle("/a2a", s.handler)
	mux.HandleFunc("/a2a/events", s.handler.SubscribeSSE)
}

// Start starts the A2A server in standalone mode. Blocks until stopped.
func (s *Server) Start() error {
	s.httpSrv = &http.Server{
		Addr:         s.cfg.GetListenAddr(),
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("A2A server listening on %s", s.cfg.GetListenAddr())
	return s.httpSrv.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(timeout time.Duration) error {
	if s.httpSrv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}

// Run starts the A2A server in standalone mode with signal handling.
func Run(cfg *Config, version string, executor AgentExecutor) error {
	srv := NewServer(cfg, version, executor)

	// Start server
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Fprintf(os.Stderr, "VibeCoding A2A Server v%s starting\n", version)
	fmt.Fprintf(os.Stderr, "  Endpoint: http://%s/a2a\n", cfg.GetListenAddr())
	fmt.Fprintf(os.Stderr, "  Agent Card: http://%s/.well-known/agent.json\n", cfg.GetListenAddr())
	fmt.Fprintf(os.Stderr, "  WorkDir: %s\n", cfg.GetWorkDir())
	fmt.Fprintf(os.Stderr, "\nReady to serve.\n")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("a2a server error: %w", err)
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\nReceived %s, shutting down...\n", sig)
		if err := srv.Stop(10 * time.Second); err != nil {
			log.Printf("A2A server shutdown error: %v", err)
		}
	}

	return nil
}
