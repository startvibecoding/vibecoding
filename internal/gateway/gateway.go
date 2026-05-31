package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/contextfiles"
	"github.com/startvibecoding/vibecoding/internal/provider"
	providerfactory "github.com/startvibecoding/vibecoding/internal/provider/factory"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/skills"
)

// RunOptions holds the CLI flags for the gateway command.
type RunOptions struct {
	ConfigPath string
	Port       string
	Provider   string
	Model      string
	WorkDir    string
	Sandbox    bool
	MultiAgent bool
	Verbose    bool
	Debug      bool
}

// Server is the gateway HTTP server.
type Server struct {
	mu sync.RWMutex

	cfg        *GatewayConfig
	settings   *config.Settings
	version    string

	provider   provider.Provider
	model      *provider.Model
	sandboxMgr *sandbox.Manager
	skillsMgr  *skills.Manager
	pool       *SessionPool

	extraContext     string
	defaultSessionID string // used when x_session_id is empty
}

// Run starts the gateway server.
func Run(opts RunOptions, version string) error {
	config.Verbose = opts.Verbose || opts.Debug
	if opts.Debug {
		_ = os.Setenv("VIBECODING_DEBUG", "1")
	}

	// Load settings.json
	settings, err := config.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	// Load gateway.json
	var gCfg *GatewayConfig
	if opts.ConfigPath != "" {
		gCfg, err = LoadGatewayConfigFrom(opts.ConfigPath)
	} else {
		gCfg, err = LoadGatewayConfig()
	}
	if err != nil {
		return fmt.Errorf("load gateway config: %w", err)
	}

	// CLI flag overrides
	if opts.Port != "" {
		gCfg.Listen = ":" + opts.Port
	}
	if opts.MultiAgent {
		gCfg.EnableSubAgents = true
	}
	if opts.Sandbox {
		gCfg.Sandbox.Enabled = true
	}
	if opts.WorkDir != "" {
		gCfg.WorkingDir = opts.WorkDir
	}

	// Resolve provider/model
	providerName := gCfg.Provider
	if opts.Provider != "" {
		providerName = opts.Provider
	}
	if providerName == "" {
		providerName = settings.DefaultProvider
	}

	modelID := gCfg.Model
	if opts.Model != "" {
		modelID = opts.Model
	}
	if modelID == "" {
		modelID = settings.DefaultModel
	}

	p, model, err := providerfactory.Create(settings, providerName, modelID)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	// Setup working directory
	cwd := gCfg.GetWorkDir()

	// Setup sandbox
	sbMgr := sandbox.NewManager(cwd)
	sbEnabled := gCfg.Sandbox.Enabled
	if !sbEnabled {
		sbMgr.SetLevel(sandbox.LevelNone)
	} else {
		level := sandbox.LevelStandard
		if gCfg.Sandbox.Level != "" {
			switch gCfg.Sandbox.Level {
			case "none":
				level = sandbox.LevelNone
			case "strict":
				level = sandbox.LevelStrict
			default:
				level = sandbox.LevelStandard
			}
		} else {
			switch gCfg.DefaultMode {
			case "plan":
				level = sandbox.LevelStrict
			case "yolo":
				level = sandbox.LevelNone
			}
		}
		if err := sbMgr.SetLevel(level); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: sandbox unavailable: %v\n", err)
			sbMgr.SetLevel(sandbox.LevelNone)
		}
	}

	// Load skills
	skillsMgr := skills.NewManager(settings.GetGlobalSkillsDir(), filepath.Join(cwd, ".skills"))
	_ = skillsMgr.Load()

	// Load context files
	var extraContext string
	if settings.ContextFiles.Enabled {
		cfResult := contextfiles.LoadContextFiles(cwd, config.ConfigDir(), settings.ContextFiles.ExtraFiles)
		if ctx := contextfiles.BuildContextString(cfResult); ctx != "" {
			extraContext = ctx
		}
	}
	extraContext += skillsMgr.BuildAllSkillsContext()

	// Build session pool
	idleTimeout := time.Duration(gCfg.Session.IdleTimeoutSeconds) * time.Second
	pool := NewSessionPool(gCfg.Session.MaxSessions, idleTimeout)

	srv := &Server{
		cfg:          gCfg,
		settings:     settings,
		version:      version,
		provider:     p,
		model:        model,
		sandboxMgr:   sbMgr,
		skillsMgr:    skillsMgr,
		pool:         pool,
		extraContext: extraContext,
	}

	// Build routes
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", srv.handleChatCompletions)
	mux.HandleFunc("/v1/models", srv.handleModels)
	mux.HandleFunc("/health", srv.handleHealth)

	// Apply middleware stack (inside-out)
	var handler http.Handler = mux
	handler = ConcurrencyMiddleware(gCfg.MaxConcurrentReqs, handler)
	handler = CORSMiddleware(gCfg.CORS, handler)
	handler = LoggingMiddleware(handler)

	// Auth middleware wraps everything except /health
	authMux := http.NewServeMux()
	authMux.Handle("/health", LoggingMiddleware(http.HandlerFunc(srv.handleHealth)))
	authMux.Handle("/", AuthMiddleware(gCfg.Auth, handler))

	httpServer := &http.Server{
		Addr:         gCfg.GetListenAddr(),
		Handler:      authMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: time.Duration(gCfg.RequestTimeoutSecs+10) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "VibeCoding Gateway v%s starting on %s\n", version, gCfg.GetListenAddr())
		fmt.Fprintf(os.Stderr, "  Provider: %s | Model: %s | Mode: %s\n", p.Name(), model.ID, gCfg.DefaultMode)
		fmt.Fprintf(os.Stderr, "  WorkDir: %s\n", cwd)
		if gCfg.Auth.Enabled {
			fmt.Fprintf(os.Stderr, "  Auth: enabled (%d tokens)\n", len(gCfg.Auth.Tokens))
		} else {
			fmt.Fprintf(os.Stderr, "  Auth: disabled\n")
		}
		if gCfg.Sandbox.Enabled {
			fmt.Fprintf(os.Stderr, "  Sandbox: enabled (level: %s)\n", gCfg.Sandbox.Level)
		}
		if gCfg.EnableSubAgents {
			fmt.Fprintf(os.Stderr, "  Sub-Agents: enabled\n")
		}
		fmt.Fprintf(os.Stderr, "  Tool visibility: %s | System prompt: %s\n", gCfg.ToolVisibility.Mode, gCfg.SystemPromptMode)
		fmt.Fprintf(os.Stderr, "\nReady to serve.\n")
		errCh <- httpServer.ListenAndServe()
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\nReceived %s, shutting down...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool.Stop()
		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
	}

	return nil
}

// LoggingMiddleware logs each request.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lw.statusCode, time.Since(start).Round(time.Millisecond))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

// Ensure loggingResponseWriter also satisfies http.Flusher for SSE.
func (lw *loggingResponseWriter) Flush() {
	if f, ok := lw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message, errType string) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    errType,
		},
	}
	writeJSON(w, status, resp)
}
