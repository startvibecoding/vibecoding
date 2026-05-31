package hermes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/startvibecoding/vibecoding/internal/a2a"
	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/cron"
	"github.com/startvibecoding/vibecoding/internal/hermes/webhook"
	"github.com/startvibecoding/vibecoding/internal/hermes/ws"
	"github.com/startvibecoding/vibecoding/internal/memory"
	"github.com/startvibecoding/vibecoding/internal/messaging"
	"github.com/startvibecoding/vibecoding/internal/messaging/feishu"
	"github.com/startvibecoding/vibecoding/internal/messaging/wechat"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

// RunOptions holds CLI flags for the hermes start command.
type RunOptions struct {
	ConfigPath string
	Port       int
	WorkDir    string
	Provider   string
	Model      string
	MultiAgent bool
	Sandbox    bool
	Daemon     bool
	Verbose    bool
	Debug      bool
}

// Server is the Hermes daemon.
type Server struct {
	cfg        *HermesConfig
	settings   *config.Settings
	version    string
	gateway    *ws.Gateway
	dispatcher *Dispatcher
	platforms  []messaging.Platform
	scheduler  *cron.Scheduler
}

// PIDFilePath returns the path to the hermes PID file.
func PIDFilePath() string {
	return filepath.Join(config.ConfigDir(), "hermes.pid")
}

// writePIDFile writes the current process PID to the PID file.
func writePIDFile() error {
	path := PIDFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600)
}

// removePIDFile removes the PID file if it exists.
func removePIDFile() {
	os.Remove(PIDFilePath())
}

// ReadPIDFile reads the PID from the PID file. Returns 0 if not found.
func ReadPIDFile() (int, error) {
	data, err := os.ReadFile(PIDFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)
	return pid, nil
}

// Run starts the Hermes server.
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

	// Load hermes.json
	var cfg *HermesConfig
	if opts.ConfigPath != "" {
		cfg, err = LoadHermesConfigFrom(opts.ConfigPath)
	} else {
		cfg, err = LoadHermesConfig()
	}
	if err != nil {
		return fmt.Errorf("load hermes config: %w", err)
	}

	// CLI flag overrides
	if opts.Port != 0 {
		cfg.Server.Port = opts.Port
	}
	if opts.WorkDir != "" {
		cfg.WorkDir = opts.WorkDir
	}
	if opts.Provider != "" {
		cfg.DefaultProvider = opts.Provider
	}
	if opts.Model != "" {
		cfg.DefaultModel = opts.Model
	}
	if opts.MultiAgent {
		cfg.MultiAgent = true
	}
	if opts.Sandbox {
		cfg.Sandbox = true
	}

	// Resolve working directory
	if cfg.WorkDir == "" || cfg.WorkDir == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		cfg.WorkDir = cwd
	}

	// Create cron store (always when cron enabled, for tool registration)
	var cronStore cron.CronStore
	var cronScheduler *cron.Scheduler
	if cfg.Cron.Enabled {
		storePath := cfg.Cron.StorePath
		if storePath == "" {
			storePath = filepath.Join(config.ConfigDir(), "hermes-cron.json")
		}
		cronStore = cron.NewFileCronStore(storePath)
	}

	// Create dispatcher
	dispatcher, err := NewDispatcher(cfg, settings, version, cronStore, cronScheduler)
	if err != nil {
		return fmt.Errorf("create dispatcher: %w", err)
	}

	// Create and start cron scheduler if multi-agent is available
	if cfg.Cron.Enabled && dispatcher.agentMgr != nil {
		interval := time.Duration(cfg.Cron.Interval) * time.Second
		if interval <= 0 {
			interval = 30 * time.Second
		}
		cronScheduler = cron.NewScheduler(cronStore, dispatcher.agentMgr, interval)
		cronScheduler.Start()
	}

	// Create gateway
	gw := ws.NewGateway(cfg.GetListenAddr(), cfg.Server.AuthToken, version)
	gw.SetDispatcher(newWSDispatcherAdapter(dispatcher))

	// Set memory store for /api/memory
	memStore := memory.NewStore(cfg.Memory.Path, cfg.GetWorkDir())
	gw.SetMemoryStore(memStore)

	// webhook handler is stored here so we can wire platforms after startPlatforms
	var webhookHandler *WebhookHandler

	// Register webhook routes if configured
	if cfg.Webhooks.Enabled && len(cfg.Webhooks.Routes) > 0 {
		var routes []webhook.RouteConfig
		for _, r := range cfg.Webhooks.Routes {
			routes = append(routes, webhook.RouteConfig{
				Path:     r.Path,
				Events:   r.Events,
				Skill:    r.Skill,
				Delivery: r.Delivery,
			})
		}
		webhookHandler = NewWebhookHandler(dispatcher, nil) // platforms wired after startPlatforms
		router := webhook.NewRouter(routes, cfg.Webhooks.Secret, webhookHandler)
		gw.RegisterHandler("/webhook/", router)
	}

	// Register A2A routes if enabled
	if cfg.A2A.Enabled {
		a2aCfg := &a2a.Config{
			Enabled: true,
			Port:    cfg.A2A.Port,
			Host:    cfg.Server.Host,
			WorkDir: cfg.GetWorkDir(),
		}
		if a2aCfg.Port == 0 {
			a2aCfg.Port = 8093
		}
		executor := a2a.NewDefaultExecutor(&hermesA2AFactory{dispatcher: dispatcher})
		a2aSrv := a2a.NewServer(a2aCfg, version, executor)
		a2aSrv.RegisterRoutes(gw.GetMux())
		log.Printf("[hermes] A2A routes registered on hermes gateway")
	}

	srv := &Server{
		cfg:        cfg,
		settings:   settings,
		version:    version,
		gateway:    gw,
		dispatcher: dispatcher,
		scheduler:  cronScheduler,
	}

	// Print startup info
	fmt.Fprintf(os.Stderr, "VibeCoding Hermes v%s starting\n", version)
	fmt.Fprintf(os.Stderr, "  Gateway: http://%s\n", cfg.GetListenAddr())
	fmt.Fprintf(os.Stderr, "  WebSocket: ws://%s/ws\n", cfg.GetListenAddr())
	fmt.Fprintf(os.Stderr, "  WorkDir: %s\n", cfg.GetWorkDir())
	fmt.Fprintf(os.Stderr, "  Provider: %s\n", cfg.GetDefaultProvider(settings.DefaultProvider))
	fmt.Fprintf(os.Stderr, "  Model: %s\n", cfg.GetDefaultModel(settings.DefaultModel))
	if cfg.Server.AuthToken != "" {
		fmt.Fprintf(os.Stderr, "  Auth: enabled\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Auth: disabled\n")
	}
	if cfg.MultiAgent {
		fmt.Fprintf(os.Stderr, "  Multi-agent: enabled\n")
	}
	if cfg.Sandbox {
		fmt.Fprintf(os.Stderr, "  Sandbox: enabled\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Sandbox: disabled\n")
	}

	if cfg.Cron.Enabled {
		if cronScheduler != nil {
			fmt.Fprintf(os.Stderr, "  Cron: enabled\n")
		} else {
			fmt.Fprintf(os.Stderr, "  Cron: disabled (requires --multi-agent)\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "  Cron: disabled\n")
	}

	if cfg.Webhooks.Enabled && len(cfg.Webhooks.Routes) > 0 {
		fmt.Fprintf(os.Stderr, "  Webhooks: %d routes\n", len(cfg.Webhooks.Routes))
	} else {
		fmt.Fprintf(os.Stderr, "  Webhooks: disabled\n")
	}

	// Start messaging platforms
	srv.startPlatforms()

	// Wire platform map into webhook handler now that platforms are started
	if webhookHandler != nil && len(srv.platforms) > 0 {
		pm := make(map[string]messaging.Platform, len(srv.platforms))
		for _, p := range srv.platforms {
			pm[p.Name()] = p
		}
		webhookHandler.SetPlatforms(pm)
	}

	// Start gateway (blocking)
	errCh := make(chan error, 1)
	go func() {
		if err := gw.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Fprintf(os.Stderr, "\nReady to serve.\n")

	// Write PID file for stop/status commands
	if err := writePIDFile(); err != nil {
		log.Printf("Warning: could not write PID file: %v", err)
	} else {
		defer removePIDFile()
	}

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("gateway error: %w", err)
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\nReceived %s, shutting down...\n", sig)
		srv.stop()
	}

	return nil
}

// startPlatforms connects to enabled messaging platforms.
func (srv *Server) startPlatforms() {
	if srv.cfg.Wechat.Enabled {
		credPath := srv.cfg.GetWechatCredPath()
		creds, err := wechat.LoadCredentials(credPath)
		if err != nil || creds == nil {
			fmt.Fprintf(os.Stderr, "  WeChat: enabled but not logged in — run 'vibecoding hermes wechat login'\n")
		} else {
			bot := wechat.NewBot(wechat.BotOptions{
				CredPath:   credPath,
				AutoTyping: srv.cfg.Wechat.AutoTyping,
			})
			srv.platforms = append(srv.platforms, bot)
			fmt.Fprintf(os.Stderr, "  WeChat: connected (user: %s, work_dir: %s)\n", creds.UserID, srv.cfg.GetPlatformWorkDir("wechat"))

			// Start in background
			go func() {
				if err := bot.Start(context.Background(), func(ctx context.Context, msg messaging.InboundMessage) (string, error) {
					return srv.dispatcher.HandleMessage(ctx, msg)
				}); err != nil {
					log.Printf("[wechat] Platform stopped: %v", err)
				}
			}()
		}
	} else {
		fmt.Fprintf(os.Stderr, "  WeChat: disabled\n")
	}

	if srv.cfg.Feishu.Enabled {
		if srv.cfg.Feishu.AppID == "" || srv.cfg.Feishu.AppSecret == "" {
			fmt.Fprintf(os.Stderr, "  Feishu: enabled but app_id/app_secret not configured\n")
		} else {
			bot := feishu.NewBot(feishu.BotOptions{
				AppID:     srv.cfg.Feishu.AppID,
				AppSecret: srv.cfg.Feishu.AppSecret,
			})
			srv.platforms = append(srv.platforms, bot)
			fmt.Fprintf(os.Stderr, "  Feishu: connecting (work_dir: %s)\n", srv.cfg.GetPlatformWorkDir("feishu"))

			go func() {
				if err := bot.Start(context.Background(), func(ctx context.Context, msg messaging.InboundMessage) (string, error) {
					return srv.dispatcher.HandleMessage(ctx, msg)
				}); err != nil {
					log.Printf("[feishu] Platform stopped: %v", err)
				}
			}()
		}
	} else {
		fmt.Fprintf(os.Stderr, "  Feishu: disabled\n")
	}

	if srv.cfg.Cron.Enabled {
		if srv.scheduler == nil {
			fmt.Fprintf(os.Stderr, "  Cron: disabled (requires --multi-agent)\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "  Cron: disabled\n")
	}

	if srv.cfg.A2A.Enabled {
		fmt.Fprintf(os.Stderr, "  A2A: enabled\n")
	}
}

// hermesA2AFactory creates agents for A2A task execution via hermes dispatcher.
type hermesA2AFactory struct {
	dispatcher *Dispatcher
}

func (f *hermesA2AFactory) CreateForA2A(workDir string, mode string) (*agent.Agent, error) {
	if workDir == "" {
		workDir = f.dispatcher.cfg.GetWorkDir()
	}
	// Create a new agent using the dispatcher's provider and settings
	a := agent.New(agent.Config{
		Provider:   f.dispatcher.provider,
		Model:      f.dispatcher.model,
		Mode:       mode,
		SandboxMgr: sandbox.NewManager(workDir),
		Settings:   f.dispatcher.settings,
	}, tools.NewRegistry(workDir, sandbox.NewManager(workDir).GetActive()))
	return a, nil
}

// stop gracefully shuts down all components.
func (srv *Server) stop() {
	// Stop cron scheduler
	if srv.scheduler != nil {
		srv.scheduler.Stop()
	}

	// Stop messaging platforms
	for _, p := range srv.platforms {
		log.Printf("Stopping platform: %s", p.Name())
		p.Stop()
	}

	// Stop gateway
	if err := srv.gateway.Stop(10 * time.Second); err != nil {
		log.Printf("Gateway shutdown error: %v", err)
	}
}

// --- WS Dispatcher adapter ---
// Bridges hermes.Dispatcher to ws.Dispatcher interface.

type wsDispatcherAdapter struct {
	d *Dispatcher
}

func newWSDispatcherAdapter(d *Dispatcher) *wsDispatcherAdapter {
	return &wsDispatcherAdapter{d: d}
}

func (a *wsDispatcherAdapter) HandleWSMessage(ctx context.Context, connID, text string, eventCh chan<- ws.WSEvent) error {
	// Command path
	if len(text) > 0 && text[0] == '/' {
		result := a.d.handleCommandForWS(connID, text)
		eventCh <- ws.WSEvent{
			Type:    "command_result",
			Command: text,
			Message: result,
		}
		eventCh <- ws.WSEvent{Type: "done", StopReason: "end_turn"}
		return nil
	}

	// Regular message — run agent with streaming
	sess, err := a.d.resolveSession("ws", connID)
	if err != nil {
		return err
	}

	sess.Lock()
	defer sess.Unlock()
	sess.Touch()

	// Run agent in goroutine, convert agent events to ws events
	agentCh := make(chan agent.Event, 100)
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.d.runAgentStreaming(ctx, sess, text, agentCh)
	}()

	for ev := range agentCh {
		wsev := agentEventToWSEvent(ev)
		eventCh <- wsev
	}

	if err := <-errCh; err != nil {
		eventCh <- ws.WSEvent{Type: "error", Message: err.Error()}
	}
	return nil
}

// agentEventToWSEvent converts an agent.Event to a ws.WSEvent.
func agentEventToWSEvent(ev agent.Event) ws.WSEvent {
	switch ev.Type {
	case agent.EventTextDelta:
		return ws.WSEvent{Type: "text_delta", Content: ev.TextDelta}
	case agent.EventThinkDelta:
		return ws.WSEvent{Type: "think_delta", Content: ev.ThinkDelta}
	case agent.EventToolCall:
		evTool := ws.WSEvent{
			Type:   "tool_call",
			Tool:   ev.ToolName,
			CallID: ev.ToolCallID,
			Args:   ev.ToolArgs,
		}
		if ev.ToolCall != nil {
			evTool.Tool = ev.ToolCall.Name
			evTool.CallID = ev.ToolCall.ID
		}
		return evTool
	case agent.EventToolExecutionEnd:
		name := ev.ToolName
		if name == "" && ev.ToolCall != nil {
			name = ev.ToolCall.Name
		}
		result := ws.WSEvent{
			Type:   "tool_result",
			Tool:   name,
			CallID: ev.ToolCallID,
			Result: ev.ToolResult,
		}
		if ev.ToolError != nil {
			result.Code = "error"
			result.Message = ev.ToolError.Error()
		}
		if ev.ToolDiff != nil {
			result.Type = "tool_diff"
			result.Path = ev.ToolDiff.Path
			result.Diff = ev.ToolDiff.Unified
		}
		return result
	case agent.EventContextPressure, agent.EventBudgetPressure:
		return ws.WSEvent{
			Type:    "status",
			Message: ev.PressureMessage,
		}
	case agent.EventToolApprovalRequest:
		return ws.WSEvent{
			Type:        "approval_request",
			ApprovalID:  ev.ApprovalID,
			Tool:        ev.ApprovalTool,
			Args:        ev.ApprovalArgs,
		}
	case agent.EventDone:
		return ws.WSEvent{Type: "done", StopReason: ev.StopReason}
	case agent.EventStatus:
		return ws.WSEvent{Type: "status", Message: ev.StatusMessage}
	case agent.EventError:
		msg := ""
		if ev.Error != nil {
			msg = ev.Error.Error()
		}
		return ws.WSEvent{Type: "error", Message: msg, Code: ev.StopReason}
	case agent.EventUsage:
		evWS := ws.WSEvent{Type: "usage"}
		if ev.Usage != nil {
			evWS.PromptTokens = ev.Usage.PromptTokens()
			evWS.CompletionTokens = ev.Usage.Output
			evWS.TotalTokens = ev.Usage.TotalTokens
			evWS.CacheReadTokens = ev.Usage.CacheRead
			evWS.CacheWriteTokens = ev.Usage.CacheWrite
		}
		return evWS
	default:
		// Skip lifecycle events (AgentStart, AgentEnd, TurnStart, TurnEnd, etc.)
		return ws.WSEvent{}
	}
}

func (a *wsDispatcherAdapter) ListSessions() []ws.SessionInfo {
	sessions := a.d.ListSessions()
	result := make([]ws.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		msgs := s.Manager.GetMessages()
		preview := ""
		for _, m := range msgs {
			if m.Role == "user" {
				preview = m.Content
				if len(preview) > 60 {
					preview = preview[:60] + "..."
				}
				break
			}
		}
		result = append(result, ws.SessionInfo{
			ID:           s.ID,
			Platform:     s.Platform,
			UserID:       s.UserID,
			WorkDir:      s.WorkDir,
			Mode:         s.Mode,
			MessageCount: len(msgs),
			LastActive:   s.LastUsed,
			Preview:      preview,
		})
	}
	return result
}

func (a *wsDispatcherAdapter) RemoveSession(key string) {
	a.d.RemoveSession(key)
}

func (a *wsDispatcherAdapter) ResolveApproval(approvalID string, approved bool) bool {
	return a.d.ResolveApproval(approvalID, approved)
}
