package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/contextfiles"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/provider/anthropic"
	"github.com/startvibecoding/vibecoding/internal/provider/openai"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/skills"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

const protocolVersion = 1

type RunOptions struct {
	Provider string
	Model    string
	Mode     string
	Thinking string
	Sandbox  bool
	Verbose  bool
	Debug    bool
}

type server struct {
	mu sync.Mutex
	wmu sync.Mutex

	settings *config.Settings
	cwd      string

	p provider.Provider
	m *provider.Model

	mode          string
	thinkingLevel  provider.ThinkingLevel
	sbMgr         *sandbox.Manager
	registry      *tools.Registry
	skillsMgr     *skills.Manager
	extraContext  string
	contextFiles  string

	sessions map[string]*sessionRuntime
	pending  map[string]chan json.RawMessage

	nextID int64
	r      *bufio.Reader
	w      io.Writer
}

type sessionRuntime struct {
	id     string
	mgr    *session.Manager
	agent  *agent.Agent
	cancel context.CancelFunc
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
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type clientInfo struct {
	Name    string `json:"name,omitempty"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type initializeRequest struct {
	ProtocolVersion   int                    `json:"protocolVersion"`
	ClientCapabilities map[string]any        `json:"clientCapabilities,omitempty"`
	ClientInfo        clientInfo             `json:"clientInfo,omitempty"`
}

type initializeResult struct {
	ProtocolVersion  int            `json:"protocolVersion"`
	AgentCapabilities agentCaps      `json:"agentCapabilities"`
	AgentInfo        clientInfo     `json:"agentInfo"`
	AuthMethods      []string       `json:"authMethods"`
}

type agentCaps struct {
	LoadSession      bool                   `json:"loadSession"`
	PromptCapabilities promptCaps           `json:"promptCapabilities"`
	SessionCapabilities sessionCaps         `json:"sessionCapabilities"`
	McPCapabilities   map[string]bool       `json:"mcpCapabilities,omitempty"`
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
	Cwd       string `json:"cwd"`
	McpServers []any  `json:"mcpServers,omitempty"`
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

type loadSessionRequest struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	McpServers []any  `json:"mcpServers,omitempty"`
}

type promptRequest struct {
	SessionID string            `json:"sessionId"`
	MessageID string            `json:"messageId,omitempty"`
	Prompt    []contentBlock     `json:"prompt"`
}

type promptResult struct {
	StopReason  string `json:"stopReason"`
	UserMessageID string `json:"userMessageId,omitempty"`
}

type cancelRequest struct {
	SessionID string `json:"sessionId"`
}

type requestPermissionRequest struct {
	SessionID string         `json:"sessionId"`
	ToolCall   toolCallUpdate `json:"toolCall"`
	Options    []permissionOption `json:"options"`
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

type toolCallUpdate struct {
	ToolCallID string `json:"toolCallId"`
}

// Run starts the ACP stdio server.
func Run(opts RunOptions) error {
	config.Verbose = opts.Verbose || opts.Debug

	settings, err := config.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	srv := &server{
		settings: settings,
		cwd:      cwd,
		sessions: make(map[string]*sessionRuntime),
		pending:  make(map[string]chan json.RawMessage),
		r:        bufio.NewReader(os.Stdin),
		w:        os.Stdout,
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

	skillsMgr := skills.NewManager(settings.GetGlobalSkillsDir(), cwd+"/.skills")
	_ = skillsMgr.Load()
	srv.skillsMgr = skillsMgr

	registry := tools.NewRegistry(cwd, sbMgr.GetActive())
	registry.RegisterDefaults()
	if skillsMgr != nil {
		registry.Register(tools.NewSkillRefTool(skillsMgr))
	}
	srv.registry = registry

	cfResult := contextfiles.LoadContextFiles(cwd, config.ConfigDir(), settings.ContextFiles.ExtraFiles)
	if ctx := contextfiles.BuildContextString(cfResult); ctx != "" {
		srv.extraContext = ctx + skillsMgr.BuildAllSkillsContext()
	}

	for {
		req, err := srv.readRequest()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
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
				srv.writeResponse(req.ID, nil, &rpcError{Code: -32601, Message: "method not found"})
			}
		}
	}
}

func createProvider(settings *config.Settings, providerName, modelID string) (provider.Provider, *provider.Model, error) {
	if providerName == "" {
		providerName = settings.DefaultProvider
	}
	if modelID == "" {
		modelID = settings.DefaultModel
	}
	pc := settings.GetProviderConfig(providerName)
	if pc != nil {
		apiKey := settings.ResolveKey(providerName)
		models := convertModelConfigs(providerName, pc.Models)
		api := pc.API
		if api == "" {
			if strings.Contains(strings.ToLower(pc.BaseURL), "anthropic") {
				api = "anthropic-messages"
			} else {
				api = "openai-chat"
			}
		}
		var p provider.Provider
		switch api {
		case "anthropic-messages":
			ap := anthropic.NewProviderWithModels(apiKey, pc.BaseURL, models)
			if pc.ThinkingFormat != "" {
				ap.SetThinkingFormat(pc.ThinkingFormat)
			}
			if pc.CacheControl != nil {
				ap.SetCacheControlEnabled(pc.CacheControl)
			}
			p = ap
		case "openai-chat", "openai":
			op := openai.NewProviderWithModels(apiKey, pc.BaseURL, models)
			if pc.ThinkingFormat != "" {
				op.SetThinkingFormat(pc.ThinkingFormat)
			}
			p = op
		default:
			return nil, nil, fmt.Errorf("unsupported API type: %s", api)
		}
		model := p.GetModel(modelID)
		if model == nil {
			if len(models) > 0 {
				model = models[0]
			} else {
				return nil, nil, fmt.Errorf("no models configured for provider %s", providerName)
			}
		}
		return p, model, nil
	}
	var p provider.Provider
	switch strings.ToLower(providerName) {
	case "openai":
		p = openai.NewProvider(settings.ResolveKey(providerName), "")
	case "anthropic":
		p = anthropic.NewProvider(settings.ResolveKey(providerName), "")
	default:
		return nil, nil, fmt.Errorf("unknown provider: %s", providerName)
	}
	model := p.GetModel(modelID)
	if model == nil {
		models := p.Models()
		if len(models) > 0 {
			model = models[0]
		} else {
			return nil, nil, fmt.Errorf("no models available for provider %s", providerName)
		}
	}
	return p, model, nil
}

func convertModelConfigs(providerName string, models []config.ModelConfig) []*provider.Model {
	var result []*provider.Model
	for _, m := range models {
		input := m.Input
		if len(input) == 0 {
			input = []string{"text"}
		}
		var cost provider.ModelPricing
		if m.Cost != nil {
			cost = provider.ModelPricing{
				Input:      m.Cost.Input,
				Output:     m.Cost.Output,
				CacheRead:  m.Cost.CacheRead,
				CacheWrite: m.Cost.CacheWrite,
			}
		}
		result = append(result, &provider.Model{
			ID:            m.ID,
			Name:          m.Name,
			Provider:      providerName,
			Reasoning:     m.Reasoning,
			Input:         input,
			Cost:          cost,
			ContextWindow: m.ContextWindow,
			MaxTokens:     m.MaxTokens,
		})
	}
	return result
}

func (s *server) handleInitialize(req rpcRequest) {
	var in initializeRequest
	_ = json.Unmarshal(req.Params, &in)
	result := initializeResult{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: agentCaps{
			LoadSession: true,
			PromptCapabilities: promptCaps{
				Image:           true,
				Audio:           false,
				EmbeddedContext: true,
			},
			SessionCapabilities: sessionCaps{
				Cancel: true,
			},
			McPCapabilities: map[string]bool{"http": false, "sse": false},
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
	_ = json.Unmarshal(req.Params, &in)
	mgr := session.New(in.Cwd, s.settings.GetSessionDir())
	if err := mgr.InitWithID(""); err != nil {
		s.writeResponse(req.ID, nil, &rpcError{Code: -32000, Message: err.Error()})
		return
	}
	id := mgr.GetHeader().ID
	s.mu.Lock()
	s.sessions[id] = &sessionRuntime{id: id, mgr: mgr}
	s.mu.Unlock()
	s.writeResponse(req.ID, newSessionResult{SessionID: id}, nil)
}

func (s *server) handleLoadSession(req rpcRequest) {
	var in loadSessionRequest
	_ = json.Unmarshal(req.Params, &in)
	mgr, err := session.OpenByID(in.Cwd, s.settings.GetSessionDir(), in.SessionID)
	if err != nil {
		s.writeResponse(req.ID, nil, &rpcError{Code: -32000, Message: err.Error()})
		return
	}
	s.mu.Lock()
	s.sessions[in.SessionID] = &sessionRuntime{id: in.SessionID, mgr: mgr}
	s.mu.Unlock()
	for _, msg := range mgr.GetMessages() {
		s.emitMessage(in.SessionID, msg)
	}
	s.writeResponse(req.ID, map[string]any{}, nil)
}

func (s *server) handlePrompt(req rpcRequest) {
	var in promptRequest
	_ = json.Unmarshal(req.Params, &in)
	rt := s.sessionForPrompt(in.SessionID)
	if rt == nil {
		s.writeResponse(req.ID, nil, &rpcError{Code: -32000, Message: "unknown session"})
		return
	}
	userText := promptToText(in.Prompt)
	if userText == "" {
		s.writeResponse(req.ID, nil, &rpcError{Code: -32602, Message: "empty prompt"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt.cancel = cancel
	rt.agent = agent.New(agent.Config{
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
		ApprovalHandler: func(toolName string, args map[string]any) bool {
			return s.requestPermission(rt.id, toolName, args)
		},
	}, s.registry)
	go func() {
		events := rt.agent.Run(ctx, userText)
		for ev := range events {
			s.handleAgentEvent(rt.id, ev)
		}
	}()
	s.writeResponse(req.ID, promptResult{StopReason: "started", UserMessageID: in.MessageID}, nil)
}

func (s *server) handleCancel(req rpcRequest) {
	var in cancelRequest
	_ = json.Unmarshal(req.Params, &in)
	s.mu.Lock()
	rt := s.sessions[in.SessionID]
	s.mu.Unlock()
	if rt != nil && rt.cancel != nil {
		rt.cancel()
	}
	s.writeResponse(req.ID, map[string]any{}, nil)
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
	rt := &sessionRuntime{id: sessionID, mgr: mgr}
	s.sessions[sessionID] = rt
	return rt
}

func (s *server) handleAgentEvent(sessionID string, ev agent.Event) {
	switch ev.Type {
	case agent.EventTextDelta:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "agent_message_chunk",
			Content: &contentBlock{Type: "text", Text: ev.TextDelta},
		})
	case agent.EventThinkDelta:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "agent_thought_chunk",
			Content: &contentBlock{Type: "text", Text: ev.ThinkDelta},
		})
	case agent.EventToolCall:
		if ev.ToolCall != nil {
			s.notify(sessionID, sessionUpdate{
				SessionUpdate: "tool_call",
				ToolCallID:    ev.ToolCall.ID,
				Title:         ev.ToolCall.Name,
				Kind:          "other",
				Status:        "pending",
			})
		}
	case agent.EventToolExecutionStart:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    ev.ToolCallID,
			Title:         ev.ToolName,
			Status:        "running",
			RawInput:      map[string]any{"args": ev.ToolArgs},
		})
	case agent.EventToolResult:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    ev.ToolCallID,
			Title:         ev.ToolName,
			Status:        "completed",
			RawOutput:     map[string]any{"content": ev.ToolResult},
		})
	case agent.EventUsage:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "tool_call_update",
		})
	case agent.EventDone:
		s.notify(sessionID, sessionUpdate{
			SessionUpdate: "agent_message_chunk",
		})
	}
}

func (s *server) requestPermission(sessionID, toolName string, args map[string]any) bool {
	id := s.nextRequestID()
	ch := make(chan json.RawMessage, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()
	s.notifyRequest(id, "session/request_permission", requestPermissionRequest{
		SessionID: sessionID,
		ToolCall:  toolCallUpdate{ToolCallID: id},
		Options: []permissionOption{
			{OptionID: "allow-once", Name: "Allow once", Kind: "allow_once"},
			{OptionID: "reject-once", Name: "Reject", Kind: "reject_once"},
		},
	})
	select {
	case <-time.After(30 * time.Second):
		return false
	case resp := <-ch:
		var out struct {
			OptionID string `json:"optionId"`
			Approved bool   `json:"approved"`
		}
		_ = json.Unmarshal(resp, &out)
		return out.Approved || out.OptionID == "allow-once"
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

func (s *server) nextRequestID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return fmt.Sprintf("acp-%d", s.nextID)
}

func (s *server) readRequest() (rpcRequest, error) {
	var req rpcRequest
	headers := make(map[string]string)
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			return req, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			headers[strings.ToLower(strings.TrimSpace(line[:idx]))] = strings.TrimSpace(line[idx+1:])
		}
	}
	length := 0
	fmt.Sscanf(headers["content-length"], "%d", &length)
	if length <= 0 {
		return req, fmt.Errorf("missing content-length")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(s.r, payload); err != nil {
		return req, err
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s *server) writeResponse(id json.RawMessage, result any, errResp *rpcError) {
	resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: errResp}
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
	fmt.Fprintf(s.w, "Content-Length: %d\r\n\r\n", len(data))
	_, _ = s.w.Write(data)
	if f, ok := s.w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}
