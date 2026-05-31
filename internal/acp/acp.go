package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/contextfiles"
	"github.com/startvibecoding/vibecoding/internal/mcp"
	"github.com/startvibecoding/vibecoding/internal/provider"
	providerfactory "github.com/startvibecoding/vibecoding/internal/provider/factory"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/skills"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

const protocolVersion = 1

type RunOptions struct {
	Provider   string
	Model      string
	Mode       string
	Thinking   string
	Sandbox    bool
	Verbose    bool
	Debug      bool
	MultiAgent bool
}

type server struct {
	mu  sync.Mutex
	wmu sync.Mutex

	settings *config.Settings
	cwd      string

	p provider.Provider
	m *provider.Model

	mode          string
	thinkingLevel provider.ThinkingLevel
	sbMgr         *sandbox.Manager
	skillsMgr     *skills.Manager
	extraContext  string
	contextFiles  string

	multiAgent bool
	factory    *agent.AgentFactory
	agentMgr   *agent.AgentManager

	sessions map[string]*sessionRuntime
	pending  map[string]chan json.RawMessage

	toolTitles map[string]string
	mcpNotify  map[string]bool

	nextID int64
	r      *bufio.Reader
	w      io.Writer
}

type sessionRuntime struct {
	id       string
	mgr      *session.Manager
	agent    agentpkg.Agent
	registry *tools.Registry
	cancel   context.CancelFunc
	promptID string
	cancelMu sync.Mutex
	mcp      []*mcp.Client
	agentMgr *agent.AgentManager
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcp.RPCError   `json:"error,omitempty"`
}

type clientInfo struct {
	Name    string `json:"name,omitempty"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type initializeRequest struct {
	ProtocolVersion    int            `json:"protocolVersion"`
	ClientCapabilities map[string]any `json:"clientCapabilities,omitempty"`
	ClientInfo         clientInfo     `json:"clientInfo,omitempty"`
}

type initializeResult struct {
	ProtocolVersion   int        `json:"protocolVersion"`
	AgentCapabilities agentCaps  `json:"agentCapabilities"`
	AgentInfo         clientInfo `json:"agentInfo"`
	AuthMethods       []string   `json:"authMethods"`
}

type agentCaps struct {
	LoadSession         bool            `json:"loadSession"`
	PromptCapabilities  promptCaps      `json:"promptCapabilities"`
	SessionCapabilities sessionCaps     `json:"sessionCapabilities"`
	McPCapabilities     map[string]bool `json:"mcpCapabilities,omitempty"`
}

type promptCaps struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type sessionCaps struct {
	Cancel bool `json:"cancel"`
	Close  bool `json:"close,omitempty"`
	List   bool `json:"list,omitempty"`
}

type newSessionRequest struct {
	Cwd        string             `json:"cwd"`
	McpServers []mcp.ServerConfig `json:"mcpServers,omitempty"`
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

type loadSessionRequest struct {
	SessionID  string             `json:"sessionId"`
	Cwd        string             `json:"cwd"`
	McpServers []mcp.ServerConfig `json:"mcpServers,omitempty"`
}

type promptRequest struct {
	SessionID string         `json:"sessionId"`
	MessageID string         `json:"messageId,omitempty"`
	Prompt    []contentBlock `json:"prompt"`
}

type promptResult struct {
	StopReason    string `json:"stopReason"`
	UserMessageID string `json:"userMessageId,omitempty"`
}

type cancelRequest struct {
	SessionID string `json:"sessionId"`
}

type requestPermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  permissionToolCall `json:"toolCall"`
	Options   []permissionOption `json:"options"`
}

type permissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type contentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

type sessionUpdate struct {
	SessionUpdate string         `json:"sessionUpdate"`
	Content       *contentBlock  `json:"content,omitempty"`
	ToolCallID    string         `json:"toolCallId,omitempty"`
	Title         string         `json:"title,omitempty"`
	Kind          string         `json:"kind,omitempty"`
	Status        string         `json:"status,omitempty"`
	RawInput      map[string]any `json:"rawInput,omitempty"`
	RawOutput     map[string]any `json:"rawOutput,omitempty"`
}

type permissionToolCall struct {
	ToolCallID string         `json:"toolCallId"`
	Title      string         `json:"title,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	Status     string         `json:"status,omitempty"`
	RawInput   map[string]any `json:"rawInput,omitempty"`
}

type permissionResult struct {
	Outcome *permissionOutcome `json:"outcome,omitempty"`
}

type permissionOutcome struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId,omitempty"`
}

// Run starts the ACP stdio server.
func Run(opts RunOptions) error {
	config.Verbose = opts.Verbose || opts.Debug
	if opts.Debug {
		_ = os.Setenv("VIBECODING_DEBUG", "1")
	}

	settings, err := config.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	srv := &server{
		settings:   settings,
		cwd:        cwd,
		multiAgent: opts.MultiAgent,
		sessions:   make(map[string]*sessionRuntime),
		pending:    make(map[string]chan json.RawMessage),
		toolTitles: make(map[string]string),
		mcpNotify:  make(map[string]bool),
		r:          bufio.NewReader(os.Stdin),
		w:          os.Stdout,
	}

	p, model, err := createProvider(settings, opts.Provider, opts.Model)
	if err != nil {
		return err
	}
	srv.p = p
	srv.m = model

	mode := opts.Mode
	if mode == "" {
		mode = settings.DefaultMode
	}
	if mode == "" {
		mode = "agent"
	}
	srv.mode = mode

	thinkingLevel := opts.Thinking
	if thinkingLevel == "" {
		thinkingLevel = settings.DefaultThinkingLevel
	}
	srv.thinkingLevel = provider.ThinkingLevel(thinkingLevel)

	sbMgr := sandbox.NewManager(cwd)
	sbEnabled := opts.Sandbox || settings.Sandbox.Enabled
	if !sbEnabled {
		sbMgr.SetLevel(sandbox.LevelNone)
	} else {
		level := sandbox.LevelStandard
		if mode == "plan" {
			level = sandbox.LevelStrict
		} else if mode == "yolo" {
			level = sandbox.LevelNone
		}
		if err := sbMgr.SetLevel(level); err != nil {
			if opts.Sandbox {
				return fmt.Errorf("sandbox requested but unavailable: %w", err)
			}
			sbMgr.SetLevel(sandbox.LevelNone)
		}
	}
	srv.sbMgr = sbMgr

	skillsMgr := skills.NewManager(settings.GetGlobalSkillsDir(), filepath.Join(cwd, ".skills"))
	_ = skillsMgr.Load()
	srv.skillsMgr = skillsMgr

	cfResult := contextfiles.LoadContextFiles(cwd, config.ConfigDir(), settings.ContextFiles.ExtraFiles)
	if ctx := contextfiles.BuildContextString(cfResult); ctx != "" {
		srv.extraContext = ctx + skillsMgr.BuildAllSkillsContext()
	}

	// Multi-agent mode: create AgentFactory and AgentManager
	if opts.MultiAgent {
		compactionSettings := ctxpkg.CompactionSettings{
			Enabled:          settings.Compaction.Enabled,
			ReserveTokens:    settings.Compaction.ReserveTokens,
			KeepRecentTokens: settings.Compaction.KeepRecentTokens,
		}
		if compactionSettings.ReserveTokens == 0 {
			compactionSettings.ReserveTokens = 16384
		}
		if compactionSettings.KeepRecentTokens == 0 {
			compactionSettings.KeepRecentTokens = 20000
		}

		srv.factory = agent.NewAgentFactory(p, model, settings, sbMgr, srv.extraContext, compactionSettings, nil)
		srv.agentMgr = agent.NewAgentManager(srv.factory)
	}

	for {
		req, err := srv.readRequest()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			srv.writeMessage(map[string]any{
				"jsonrpc": "2.0",
				"error":   &mcp.RPCError{Code: -32700, Message: err.Error()},
			})
			continue
		}

		if len(req.Method) == 0 && len(req.ID) > 0 {
			srv.deliverResponse(req.ID, req.Result, req.Error)
			continue
		}

		switch req.Method {
		case "initialize":
			srv.handleInitialize(req)
		case "session/new":
			srv.handleNewSession(req)
		case "session/load":
			srv.handleLoadSession(req)
		case "session/prompt":
			srv.handlePrompt(req)
		case "session/cancel":
			srv.handleCancel(req)
		default:
			if len(req.ID) > 0 {
				srv.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32601, Message: "method not found"})
			}
		}
	}
}

func createProvider(settings *config.Settings, providerName, modelID string) (provider.Provider, *provider.Model, error) {
	enabled := true
	return providerfactory.CreateWithOptions(settings, providerName, modelID, providerfactory.Options{
		BuiltinAnthropicCacheControl: &enabled,
	})
}

func (s *server) newToolRegistry() *tools.Registry {
	registry := tools.NewRegistry(s.cwd, s.sbMgr.GetActive())
	registry.RegisterDefaultsWithPlanTool(s.settings.IsPlanToolEnabled())
	if s.skillsMgr != nil {
		registry.Register(tools.NewSkillRefTool(s.skillsMgr))
	}
	// Register subagent tools when multi-agent mode is enabled
	if s.agentMgr != nil {
		registry.Register(agent.NewSubAgentSpawnTool(s.agentMgr))
		registry.Register(agent.NewSubAgentStatusTool(s.agentMgr))
		registry.Register(agent.NewSubAgentSendTool(s.agentMgr))
		registry.Register(agent.NewSubAgentDestroyTool(s.agentMgr))
	}
	return registry
}

func (s *server) handleInitialize(req rpcRequest) {
	var in initializeRequest
	_ = json.Unmarshal(req.Params, &in)
	result := initializeResult{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: agentCaps{
			LoadSession: true,
			PromptCapabilities: promptCaps{
				Image:           false,
				Audio:           false,
				EmbeddedContext: false,
			},
			SessionCapabilities: sessionCaps{
				Cancel: true,
			},
			McPCapabilities: map[string]bool{"stdio": true, "http": true, "sse": true},
		},
		AgentInfo: clientInfo{
			Name:    "vibecoding",
			Title:   "VibeCoding",
			Version: "dev",
		},
		AuthMethods: []string{},
	}
	s.writeResponse(req.ID, result, nil)
}

func (s *server) handleNewSession(req rpcRequest) {
	var in newSessionRequest
	if err := json.Unmarshal(req.Params, &in); err != nil {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32602, Message: "invalid params"})
		return
	}
	if strings.TrimSpace(in.Cwd) == "" {
		in.Cwd = s.cwd
	}
	if !filepath.IsAbs(in.Cwd) {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32602, Message: "cwd must be an absolute path"})
		return
	}
	mgr := session.New(in.Cwd, s.settings.GetSessionDir())
	if err := mgr.InitWithID(""); err != nil {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: err.Error()})
		return
	}
	id := mgr.GetHeader().ID
	registry := s.newToolRegistry()
	mcpClients, err := mcp.ConnectServers(context.Background(), in.McpServers, registry, s.buildMCPCallbacks(id))
	if err != nil {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: err.Error()})
		return
	}
	s.mu.Lock()
	if old := s.sessions[id]; old != nil {
		mcp.CloseClients(old.mcp)
	}
	s.sessions[id] = &sessionRuntime{id: id, mgr: mgr, registry: registry, mcp: mcpClients}
	s.mu.Unlock()
	s.writeResponse(req.ID, newSessionResult{SessionID: id}, nil)
}

func (s *server) handleLoadSession(req rpcRequest) {
	var in loadSessionRequest
	if err := json.Unmarshal(req.Params, &in); err != nil {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32602, Message: "invalid params"})
		return
	}
	if strings.TrimSpace(in.Cwd) == "" {
		in.Cwd = s.cwd
	}
	if !filepath.IsAbs(in.Cwd) {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32602, Message: "cwd must be an absolute path"})
		return
	}
	registry := s.newToolRegistry()
	mcpClients, err := mcp.ConnectServers(context.Background(), in.McpServers, registry, s.buildMCPCallbacks(in.SessionID))
	if err != nil {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: err.Error()})
		return
	}
	mgr, err := session.OpenByID(in.Cwd, s.settings.GetSessionDir(), in.SessionID)
	if err != nil {
		mcp.CloseClients(mcpClients)
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: err.Error()})
		return
	}
	s.mu.Lock()
	if old := s.sessions[in.SessionID]; old != nil {
		mcp.CloseClients(old.mcp)
	}
	s.sessions[in.SessionID] = &sessionRuntime{id: in.SessionID, mgr: mgr, registry: registry, mcp: mcpClients}
	s.mu.Unlock()
	for _, msg := range mgr.GetMessages() {
		s.emitMessage(in.SessionID, msg)
	}
	s.writeResponse(req.ID, nil, nil)
}

func (s *server) handlePrompt(req rpcRequest) {
	var in promptRequest
	if err := json.Unmarshal(req.Params, &in); err != nil {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32602, Message: "invalid params"})
		return
	}
	rt := s.sessionForPrompt(in.SessionID)
	if rt == nil {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: "unknown session"})
		return
	}
	userText := promptToText(in.Prompt)
	if userText == "" {
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32602, Message: "empty prompt"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	promptKey := mcp.RawIDKey(req.ID)
	rt.cancelMu.Lock()
	if rt.cancel != nil {
		rt.cancelMu.Unlock()
		cancel()
		s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: "session already has an active prompt"})
		return
	}
	rt.cancel = cancel
	rt.promptID = promptKey
	rt.cancelMu.Unlock()

	var a agentpkg.Agent
	if s.agentMgr != nil {
		var err error
		a, err = s.agentMgr.Create(agent.AgentOptions{
			Mode:    s.mode,
			Model:   s.m,
			Session: rt.mgr,
		})
		if err != nil {
			cancel()
			s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: err.Error()})
			return
		}
	} else {
		inner := agent.New(agent.Config{
			Provider:      s.p,
			Model:         s.m,
			Mode:          s.mode,
			ThinkingLevel: s.thinkingLevel,
			MaxTokens:     s.settings.MaxOutputTokens,
			SandboxMgr:    s.sbMgr,
			Settings:      s.settings,
			Session:       rt.mgr,
			ExtraContext:  s.extraContext,
			CompactionSettings: ctxpkg.CompactionSettings{
				Enabled:          s.settings.Compaction.Enabled,
				ReserveTokens:    s.settings.Compaction.ReserveTokens,
				KeepRecentTokens: s.settings.Compaction.KeepRecentTokens,
			},
			ApprovalHandler: func(toolCallID, toolName string, args map[string]any) bool {
				return s.requestPermission(rt.id, toolCallID, toolName, args)
			},
		}, rt.registry)
		a = agent.NewAgentAdapter(inner)
	}
	rt.agent = a
	go func() {
		defer func() {
			rt.cancelMu.Lock()
			if rt.promptID == promptKey {
				rt.cancel = nil
				rt.promptID = ""
			}
			rt.cancelMu.Unlock()
			cancel()
		}()
		stopReason := "end_turn"
		var runErr error
		events := rt.agent.Run(ctx, userText)
		for ev := range events {
			s.handleAgentEvent(rt.id, ev)
			switch ev.Type {
			case agentpkg.EventDone:
				stopReason = normalizeStopReason(ev.StopReason)
			case agentpkg.EventError:
				if ev.Error != nil {
					runErr = ev.Error
				}
				stopReason = normalizeStopReason(ev.StopReason)
			}
		}
		if runErr != nil && stopReason != "cancelled" {
			s.writeResponse(req.ID, nil, &mcp.RPCError{Code: -32000, Message: runErr.Error()})
			return
		}
		s.writeResponse(req.ID, promptResult{StopReason: stopReason, UserMessageID: in.MessageID}, nil)
	}()
}

func (s *server) handleCancel(req rpcRequest) {
	var in cancelRequest
	_ = json.Unmarshal(req.Params, &in)
	s.mu.Lock()
	rt := s.sessions[in.SessionID]
	s.mu.Unlock()
	if rt != nil {
		rt.cancelMu.Lock()
		if rt.cancel != nil {
			rt.cancel()
		}
		rt.cancelMu.Unlock()
	}
	if len(req.ID) > 0 {
		s.writeResponse(req.ID, map[string]any{}, nil)
	}
}

func (s *server) sessionForPrompt(sessionID string) *sessionRuntime {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rt, ok := s.sessions[sessionID]; ok {
		return rt
	}
	mgr := session.New(s.cwd, s.settings.GetSessionDir())
	if err := mgr.InitWithID(sessionID); err != nil {
		return nil
	}
	rt := &sessionRuntime{id: sessionID, mgr: mgr, registry: s.newToolRegistry()}
	s.sessions[sessionID] = rt
	return rt
}

func (s *server) handleAgentEvent(sessionID string, ev agentpkg.Event) {
	switch ev.Type {
	case agentpkg.EventTextDelta:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "agent_message_chunk",
			Content:       &contentBlock{Type: "text", Text: ev.TextDelta},
		})
	case agentpkg.EventThinkDelta:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "agent_thought_chunk",
			Content:       &contentBlock{Type: "text", Text: ev.ThinkDelta},
		})
	case agentpkg.EventToolCall:
		if ev.ToolCall != nil {
			title := s.rememberToolTitle(ev.ToolCall.ID, ev.ToolCall.Name, ev.ToolArgs)
			s.notify(sessionID, sessionUpdate{
				SessionUpdate: "tool_call",
				ToolCallID:    ev.ToolCall.ID,
				Title:         title,
				Kind:          "other",
				Status:        "pending",
				RawInput:      toolRawInput(ev.ToolArgs),
			})
		}
	case agentpkg.EventToolExecutionStart:
		title := s.rememberToolTitle(ev.ToolCallID, ev.ToolName, ev.ToolArgs)
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    ev.ToolCallID,
			Title:         title,
			Status:        "in_progress",
			RawInput:      toolRawInput(ev.ToolArgs),
		})
	case agentpkg.EventToolExecutionEnd:
		status := "completed"
		if ev.ToolError != nil {
			status = "failed"
		}
		rawOutput := map[string]any{"content": ev.ToolResult}
		if ev.ToolDiff != nil {
			rawOutput["diff"] = ev.ToolDiff
		}
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    ev.ToolCallID,
			Title:         s.toolTitleFor(ev.ToolCallID, ev.ToolName),
			Status:        status,
			RawOutput:     rawOutput,
		})
	case agentpkg.EventToolResult:
	case agentpkg.EventPlanUpdate:
		if ev.Plan != nil {
			s.notify(sessionID, sessionUpdate{
				SessionUpdate: "agent_message_chunk",
				Content:       &contentBlock{Type: "text", Text: formatACPPlan(ev.Plan)},
			})
		}
	case agentpkg.EventUsage:
	case agentpkg.EventDone:
	}
}

func formatACPPlan(plan *agentpkg.TaskPlan) string {
	if plan == nil || len(plan.Steps) == 0 {
		return "Plan updated."
	}
	var b strings.Builder
	title := plan.Title
	if title == "" {
		title = "Plan"
	}
	b.WriteString(title)
	for _, step := range plan.Steps {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%s %s", planStatusMarker(step.Status), step.Title))
	}
	if plan.Note != "" {
		b.WriteString("\nnote: " + plan.Note)
	}
	return b.String()
}

func planStatusMarker(status string) string {
	switch status {
	case "running":
		return ">"
	case "done":
		return "x"
	case "failed":
		return "!"
	default:
		return "-"
	}
}

func (s *server) buildMCPCallbacks(sessionID string) mcp.Callbacks {
	return mcp.Callbacks{
		OnNotification: func(serverName, method string, params json.RawMessage) {
			s.handleMCPNotification(sessionID, serverName, method, params)
		},
		OnSamplingCreateMessage: func(ctx context.Context, serverName string, params json.RawMessage) (json.RawMessage, *mcp.RPCError) {
			return s.handleMCPSamplingCreateMessage(ctx, sessionID, serverName, params)
		},
	}
}

func (s *server) handleMCPNotification(sessionID, serverName, method string, params json.RawMessage) {
	callID := "mcp-notify-" + mcp.SanitizeToolName(serverName)
	title := "mcp_notification: " + serverName
	s.mu.Lock()
	if !s.mcpNotify[callID] {
		s.mcpNotify[callID] = true
		s.mu.Unlock()
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "tool_call",
			ToolCallID:    callID,
			Title:         title,
			Kind:          "other",
			Status:        "pending",
		})
	} else {
		s.mu.Unlock()
	}

	rawOut := map[string]any{
		"method": method,
	}
	if parsed := parseJSONRawToMap(params); parsed != nil {
		rawOut["params"] = parsed
	} else if trimmed := strings.TrimSpace(string(params)); trimmed != "" && trimmed != "null" {
		rawOut["paramsText"] = trimmed
	}

	switch method {
	case "notifications/progress", "notifications/message", "logging/message", "notifications/cancelled":
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    callID,
			Title:         title,
			Status:        "in_progress",
			RawOutput:     rawOut,
		})
	}
}

func (s *server) handleMCPSamplingCreateMessage(ctx context.Context, sessionID, serverName string, params json.RawMessage) (json.RawMessage, *mcp.RPCError) {
	prompt, systemPrompt, maxTokens := extractSamplingInput(params)
	if strings.TrimSpace(prompt) == "" {
		return nil, &mcp.RPCError{Code: -32602, Message: "sampling/createMessage requires non-empty messages"}
	}
	if maxTokens <= 0 {
		maxTokens = s.settings.MaxOutputTokens
	}
	modelID := ""
	if s.m != nil {
		modelID = s.m.ID
	}
	chatCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	events := s.p.Chat(chatCtx, provider.ChatParams{
		Messages:      []provider.Message{provider.NewUserMessage(prompt)},
		SystemPrompt:  systemPrompt,
		ThinkingLevel: s.thinkingLevel,
		MaxTokens:     maxTokens,
		ModelID:       modelID,
	})
	var outText strings.Builder
	for ev := range events {
		switch ev.Type {
		case provider.StreamTextDelta:
			outText.WriteString(ev.TextDelta)
		case provider.StreamDone:
			// noop
		case provider.StreamError:
			if ev.Error != nil {
				return nil, &mcp.RPCError{Code: -32000, Message: ev.Error.Error()}
			}
		}
	}
	text := strings.TrimSpace(outText.String())
	if text == "" {
		text = "(empty response)"
	}
	result := map[string]any{
		"model": modelID,
		"role":  "assistant",
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, &mcp.RPCError{Code: -32000, Message: err.Error()}
	}
	s.notify(sessionID, sessionUpdate{
		SessionUpdate: "agent_message_chunk",
		Content:       &contentBlock{Type: "text", Text: "MCP[" + serverName + "] sampling/createMessage completed"},
	})
	return data, nil
}

func extractSamplingPrompt(params json.RawMessage) string {
	prompt, _, _ := extractSamplingInput(params)
	return prompt
}

func extractSamplingInput(params json.RawMessage) (prompt string, systemPrompt string, maxTokens int) {
	maxTokens = 0
	if len(params) == 0 {
		return "", "", maxTokens
	}
	var raw map[string]any
	if err := json.Unmarshal(params, &raw); err != nil {
		return strings.TrimSpace(string(params)), "", maxTokens
	}
	if v, ok := raw["maxTokens"].(float64); ok && int(v) > 0 {
		maxTokens = int(v)
	}
	msgs, _ := raw["messages"].([]any)
	var parts []string
	for _, m := range msgs {
		msgMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content := msgMap["content"]
		role, _ := msgMap["role"].(string)
		switch v := content.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				if role == "system" {
					if systemPrompt == "" {
						systemPrompt = v
					}
					continue
				}
				parts = append(parts, v)
			}
		case []any:
			var blockTexts []string
			for _, item := range v {
				block, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if t, _ := block["type"].(string); t == "text" {
					if txt, _ := block["text"].(string); strings.TrimSpace(txt) != "" {
						blockTexts = append(blockTexts, txt)
					}
				}
			}
			if len(blockTexts) == 0 {
				continue
			}
			joined := strings.Join(blockTexts, "\n")
			if role == "system" {
				if systemPrompt == "" {
					systemPrompt = joined
				}
				continue
			}
			parts = append(parts, joined)
		}
	}
	return strings.Join(parts, "\n"), systemPrompt, maxTokens
}

func parseJSONRawToMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

func (s *server) requestPermission(sessionID, toolCallID, toolName string, args map[string]any) bool {
	id := s.nextRequestID()
	ch := make(chan json.RawMessage, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()
	s.notifyRequest(id, "session/request_permission", requestPermissionRequest{
		SessionID: sessionID,
		ToolCall: permissionToolCall{
			ToolCallID: toolCallID,
			Title:      toolName,
			Kind:       "execute",
			Status:     "pending",
			RawInput:   toolRawInput(args),
		},
		Options: []permissionOption{
			{OptionID: "allow-once", Name: "Allow once", Kind: "allow_once"},
			{OptionID: "reject-once", Name: "Reject", Kind: "reject_once"},
		},
	})
	select {
	case <-time.After(30 * time.Second):
		return false
	case resp := <-ch:
		var out permissionResult
		_ = json.Unmarshal(resp, &out)
		return out.Outcome != nil && out.Outcome.Outcome == "selected" && out.Outcome.OptionID == "allow-once"
	}
}

func (s *server) deliverResponse(id json.RawMessage, result json.RawMessage, errMsg json.RawMessage) {
	key := strings.Trim(string(id), "\"")
	s.mu.Lock()
	ch, ok := s.pending[key]
	if ok {
		delete(s.pending, key)
	}
	s.mu.Unlock()
	if ok {
		if len(errMsg) > 0 {
			ch <- errMsg
			return
		}
		ch <- result
	}
}

func (s *server) emitMessage(sessionID string, msg provider.Message) {
	if msg.Role == "assistant" {
		for _, c := range msg.Contents {
			if c.Type == "thinking" && c.Thinking != "" {
				s.notify(sessionID, sessionUpdate{SessionUpdate: "agent_thought_chunk", Content: &contentBlock{Type: "text", Text: c.Thinking}})
			} else if c.Type == "text" && c.Text != "" {
				s.notify(sessionID, sessionUpdate{SessionUpdate: "agent_message_chunk", Content: &contentBlock{Type: "text", Text: c.Text}})
			}
		}
		return
	}
	if msg.Role == "user" {
		text := msg.Content
		if text == "" {
			for _, c := range msg.Contents {
				if c.Type == "text" && c.Text != "" {
					text = c.Text
					break
				}
			}
		}
		if text != "" {
			s.notify(sessionID, sessionUpdate{SessionUpdate: "user_message_chunk", Content: &contentBlock{Type: "text", Text: text}})
		}
	}
}

func promptToText(blocks []contentBlock) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case "resource_link":
			if b.URI != "" {
				parts = append(parts, b.URI)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func toolRawInput(args map[string]any) map[string]any {
	raw := map[string]any{"args": args}
	for key, value := range args {
		raw[key] = value
	}
	return raw
}

func (s *server) rememberToolTitle(toolCallID, name string, args map[string]any) string {
	title := toolTitle(name, args)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := s.toolTitles[toolCallID]; existing != "" && existing != name {
		return existing
	}
	s.toolTitles[toolCallID] = title
	return title
}

func (s *server) toolTitleFor(toolCallID, fallback string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if title := s.toolTitles[toolCallID]; title != "" {
		return title
	}
	return fallback
}

func toolTitle(name string, args map[string]any) string {
	if args == nil {
		return name
	}

	var details []string
	switch name {
	case "bash":
		details = appendStringArg(details, "command", args)
	case "read", "write", "edit", "ls":
		details = appendStringArg(details, "path", args)
	case "grep":
		details = appendStringArg(details, "pattern", args)
		details = appendStringArg(details, "path", args)
	case "find":
		details = appendStringArg(details, "pattern", args)
		details = appendStringArg(details, "path", args)
	default:
		for _, key := range []string{"command", "path", "pattern", "query", "name"} {
			details = appendStringArg(details, key, args)
			if len(details) > 0 {
				break
			}
		}
	}

	if len(details) == 0 {
		return name
	}
	return name + ": " + truncateTitle(strings.Join(details, " "))
}

func appendStringArg(details []string, key string, args map[string]any) []string {
	value, ok := args[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return details
	}
	if key == "command" {
		return append(details, value)
	}
	return append(details, key+"="+value)
}

func truncateTitle(title string) string {
	const maxTitleLength = 160
	title = strings.TrimSpace(strings.ReplaceAll(title, "\n", " "))
	if len(title) <= maxTitleLength {
		return title
	}
	return title[:maxTitleLength-3] + "..."
}

func normalizeStopReason(reason string) string {
	switch reason {
	case "", "stop", "end_turn", "tool_use":
		return "end_turn"
	case "max_tokens", "length":
		return "max_tokens"
	case "cancelled", "aborted":
		return "cancelled"
	default:
		return "refusal"
	}
}

func (s *server) nextRequestID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return fmt.Sprintf("acp-%d", s.nextID)
}

func (s *server) readRequest() (rpcRequest, error) {
	var req rpcRequest
	line, err := s.r.ReadBytes('\n')
	if err != nil {
		return req, err
	}
	payload := strings.TrimRight(string(line), "\r\n")
	if strings.TrimSpace(payload) == "" {
		return req, fmt.Errorf("empty message")
	}
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s *server) writeResponse(id json.RawMessage, result any, errResp *mcp.RPCError) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
	}
	if errResp != nil {
		resp["error"] = errResp
	} else {
		resp["result"] = result
	}
	s.writeMessage(resp)
}

func (s *server) notify(sessionID string, update sessionUpdate) {
	s.writeMessage(map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/update",
		"params": map[string]any{
			"sessionId": sessionID,
			"update":    update,
		},
	})
}

func (s *server) notifyRequest(id string, method string, params any) {
	s.writeMessage(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
}

func (s *server) writeMessage(v any) {
	data, _ := json.Marshal(v)
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_, _ = s.w.Write(data)
	_, _ = s.w.Write([]byte("\n"))
	if f, ok := s.w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}
