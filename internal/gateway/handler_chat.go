package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/startvibecoding/vibecoding/internal/agent"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body", "invalid_request_error")
		return
	}
	defer r.Body.Close()

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error(), "invalid_request_error")
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages array is required and must not be empty", "invalid_request_error")
		return
	}

	// Validate x_working_dir
	workDir := s.cfg.GetWorkDir()
	if req.XWorkingDir != "" {
		if err := s.cfg.ValidateWorkDir(req.XWorkingDir); err != nil {
			writeError(w, http.StatusForbidden, err.Error(), "permission_error")
			return
		}
		workDir = req.XWorkingDir
	}

	// Resolve model
	s.mu.RLock()
	currentModel := s.model
	currentProvider := s.provider
	s.mu.RUnlock()

	if req.Model != "" {
		if m := currentProvider.GetModel(req.Model); m != nil {
			currentModel = m
		}
	}

	// Extract last user message
	lastUserMsg, systemMsgs, historyMsgs := parseMessages(req.Messages)
	if lastUserMsg == "" {
		writeError(w, http.StatusBadRequest, "no user message found", "invalid_request_error")
		return
	}

	// Get or create session
	sessionID := req.XSessionID
	sess := s.getOrCreateSession(sessionID, workDir)
	if sess == nil {
		writeError(w, http.StatusServiceUnavailable, "session pool is at capacity", "server_error")
		return
	}

	// Check for slash command
	if cmdResult := s.handleCommand(sess, lastUserMsg); cmdResult != nil {
		// If /clear, we need to reset agent state on the session
		if strings.HasPrefix(strings.TrimSpace(lastUserMsg), "/clear") {
			// Create a fresh session manager but keep the session slot
			newMgr := session.New(sess.WorkDir, s.settings.GetSessionDir())
			if err := newMgr.Init(); err == nil {
				sess.Manager = newMgr
			}
		}
		if req.Stream {
			s.writeCommandResponseStreaming(w, cmdResult, currentModel.ID, sess.ID, lastUserMsg)
		} else {
			s.writeCommandResponse(w, cmdResult, currentModel.ID, sess.ID, lastUserMsg)
		}
		return
	}

	// Lock session for serial processing
	sess.Lock()
	defer sess.Unlock()
	sess.Touch()

	// Determine mode
	mode := s.cfg.DefaultMode
	if sess.Mode != "" {
		mode = sess.Mode
	}
	if req.XMode != "" {
		mode = req.XMode
	}

	// Build extra context: system prompt handling
	extraContext := s.extraContext
	if s.cfg.SystemPromptMode == "append" && len(systemMsgs) > 0 {
		extraContext += "\n## Client Instructions\n" + strings.Join(systemMsgs, "\n") + "\n"
	}

	// Build compaction settings
	compactionSettings := ctxpkg.CompactionSettings{
		Enabled:          s.settings.Compaction.Enabled,
		ReserveTokens:    s.settings.Compaction.ReserveTokens,
		KeepRecentTokens: s.settings.Compaction.KeepRecentTokens,
	}
	if compactionSettings.ReserveTokens == 0 {
		compactionSettings.ReserveTokens = 16384
	}
	if compactionSettings.KeepRecentTokens == 0 {
		compactionSettings.KeepRecentTokens = 20000
	}

	// Build agent config
	thinkingLevel := provider.ThinkingLevel(s.cfg.DefaultThinkingLevel)
	if thinkingLevel == "" {
		thinkingLevel = provider.ThinkingLevel(s.settings.DefaultThinkingLevel)
	}

	maxTokens := s.settings.MaxOutputTokens
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	agentCfg := agent.Config{
		Provider:           currentProvider,
		Model:              currentModel,
		Mode:               mode,
		ThinkingLevel:      thinkingLevel,
		MaxTokens:          maxTokens,
		SandboxMgr:         s.sandboxMgr,
		Settings:           s.settings,
		Session:            sess.Manager,
		ExtraContext:        extraContext,
		CompactionSettings: compactionSettings,
		MultiAgent:         s.cfg.EnableSubAgents,
	}

	a := agent.New(agentCfg, sess.Registry)

	// Apply force compact flag from /compact command
	if sess.ForceCompact {
		a.SetForceCompact()
		sess.ForceCompact = false
	}

	// Load history if this is a new session with client-provided history
	if len(historyMsgs) > 0 && len(sess.Manager.GetMessages()) == 0 {
		internalMsgs := convertHistoryMessages(historyMsgs)
		a.LoadHistoryMessages(internalMsgs)
	}

	// Register sub-agent tools if enabled
	if s.cfg.EnableSubAgents && sess.AgentMgr != nil {
		sess.Registry.Register(agent.NewSubAgentSpawnTool(sess.AgentMgr))
		sess.Registry.Register(agent.NewSubAgentStatusTool(sess.AgentMgr))
		sess.Registry.Register(agent.NewSubAgentSendTool(sess.AgentMgr))
		sess.Registry.Register(agent.NewSubAgentDestroyTool(sess.AgentMgr))
	}

	// Setup request timeout
	timeout := time.Duration(s.cfg.RequestTimeoutSecs) * time.Second
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Run agent
	eventCh := a.Run(ctx, lastUserMsg)

	if req.Stream {
		s.handleStreamingResponse(w, r, eventCh, currentModel.ID, sess.ID)
	} else {
		s.handleNonStreamingResponse(w, eventCh, currentModel.ID, sess.ID)
	}
}

func (s *Server) handleStreamingResponse(w http.ResponseWriter, r *http.Request, eventCh <-chan agent.Event, modelID, sessionID string) {
	sse := NewSSEWriter(w, modelID, sessionID)
	sse.WriteRoleDelta()

	toolMode := s.cfg.ToolVisibility.Mode
	toolDetail := s.cfg.GetToolDetail()
	var totalUsage CompletionUsage
	var xToolCalls []XToolCall
	// Track in-flight tool calls by callID so we can attach result/diff on end.
	pendingTools := make(map[string]*toolCallInfo)

	for ev := range eventCh {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		switch ev.Type {
		case agent.EventTextDelta:
			sse.WriteContentDelta(ev.TextDelta)

		case agent.EventToolCall:
			name, callID := resolveToolEvent(ev)
			tc := &toolCallInfo{Name: name, Args: ev.ToolArgs, Status: "running"}
			if callID != "" {
				pendingTools[callID] = tc
			}
			xToolCalls = append(xToolCalls, XToolCall{Name: name, Args: ev.ToolArgs, Status: "running"})
			switch toolMode {
			case "content":
				sse.WriteContentDelta(formatToolRunning(name, ev.ToolArgs))
			case "sse_event":
				sse.WriteToolStatusEvent(name, "running", ev.ToolArgs)
			}

		case agent.EventToolExecutionEnd:
			status := "completed"
			if ev.ToolError != nil {
				status = "failed"
			}
			// Update xToolCalls status
			for i := len(xToolCalls) - 1; i >= 0; i-- {
				if xToolCalls[i].Name == ev.ToolName && xToolCalls[i].Status == "running" {
					xToolCalls[i].Status = status
					break
				}
			}
			// Build expanded output
			tc := pendingTools[ev.ToolCallID]
			if tc == nil {
				tc = &toolCallInfo{Name: ev.ToolName, Args: ev.ToolArgs}
			}
			tc.Status = status
			tc.Result = ev.ToolResult
			tc.Diff = ev.ToolDiff
			tc.Error = ev.ToolError
			delete(pendingTools, ev.ToolCallID)

			switch toolMode {
			case "content":
				sse.WriteToolResult(tc, toolDetail)
			case "sse_event":
				sse.WriteToolStatusEvent(ev.ToolName, status, nil)
			}

		case agent.EventUsage:
			if ev.Usage != nil {
				totalUsage.PromptTokens += ev.Usage.TotalInputTokens()
				totalUsage.CompletionTokens += ev.Usage.Output
				totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
			}

		case agent.EventDone:
			sse.WriteDone(&totalUsage)
			return

		case agent.EventError:
			if ev.Error != nil {
				sse.WriteError(ev.Error.Error())
			} else {
				sse.WriteDone(&totalUsage)
			}
			return
		}
	}
	// Channel closed without EventDone
	sse.WriteDone(&totalUsage)
}

func (s *Server) handleNonStreamingResponse(w http.ResponseWriter, eventCh <-chan agent.Event, modelID, sessionID string) {
	var sb strings.Builder
	var totalUsage CompletionUsage
	var xToolCalls []XToolCall
	toolMode := s.cfg.ToolVisibility.Mode
	toolDetail := s.cfg.GetToolDetail()
	pendingTools := make(map[string]*toolCallInfo)

	for ev := range eventCh {
		switch ev.Type {
		case agent.EventTextDelta:
			sb.WriteString(ev.TextDelta)

		case agent.EventToolCall:
			name, callID := resolveToolEvent(ev)
			tc := &toolCallInfo{Name: name, Args: ev.ToolArgs, Status: "running"}
			if callID != "" {
				pendingTools[callID] = tc
			}
			xToolCalls = append(xToolCalls, XToolCall{Name: name, Args: ev.ToolArgs, Status: "running"})

		case agent.EventToolExecutionEnd:
			status := "completed"
			if ev.ToolError != nil {
				status = "failed"
			}
			for i := len(xToolCalls) - 1; i >= 0; i-- {
				if xToolCalls[i].Name == ev.ToolName && xToolCalls[i].Status == "running" {
					xToolCalls[i].Status = status
					break
				}
			}
			// Build expanded output for content/none mode
			tc := pendingTools[ev.ToolCallID]
			if tc == nil {
				tc = &toolCallInfo{Name: ev.ToolName, Args: ev.ToolArgs}
			}
			tc.Status = status
			tc.Result = ev.ToolResult
			tc.Diff = ev.ToolDiff
			tc.Error = ev.ToolError
			delete(pendingTools, ev.ToolCallID)

			if toolMode == "content" {
				sb.WriteString(formatToolResult(tc, toolDetail))
			}

		case agent.EventUsage:
			if ev.Usage != nil {
				totalUsage.PromptTokens += ev.Usage.TotalInputTokens()
				totalUsage.CompletionTokens += ev.Usage.Output
				totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
			}

		case agent.EventError:
			if ev.Error != nil {
				writeError(w, http.StatusInternalServerError, ev.Error.Error(), "server_error")
				return
			}
		}
	}

	finishReason := "stop"
	resp := ChatCompletionResponse{
		ID:      newCompletionID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelID,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Message:      &ResponseMessage{Role: "assistant", Content: sb.String()},
				FinishReason: &finishReason,
			},
		},
		Usage:      &totalUsage,
		XSessionID: sessionID,
		XToolCalls: xToolCalls,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) writeCommandResponse(w http.ResponseWriter, result *CommandResult, modelID, sessionID, cmd string) {
	finishReason := "stop"
	resp := ChatCompletionResponse{
		ID:      newCommandCompletionID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelID,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Message:      &ResponseMessage{Role: "assistant", Content: result.Message},
				FinishReason: &finishReason,
			},
		},
		Usage:      &CompletionUsage{},
		XSessionID: sessionID,
		XCommand:   strings.Fields(cmd)[0],
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) writeCommandResponseStreaming(w http.ResponseWriter, result *CommandResult, modelID, sessionID, cmd string) {
	sse := NewSSEWriter(w, modelID, sessionID)
	sse.WriteRoleDelta()
	sse.WriteContentDelta(result.Message)
	sse.WriteDone(&CompletionUsage{})
}

// getOrCreateSession returns an existing session or creates a new one.
func (s *Server) getOrCreateSession(sessionID, workDir string) *GatewaySession {
	if sessionID != "" {
		if sess := s.pool.Get(sessionID); sess != nil {
			return sess
		}
	}

	// Create new session
	mgr := session.New(workDir, s.settings.GetSessionDir())
	if sessionID != "" {
		if err := mgr.InitWithID(sessionID); err != nil {
			// Fallback to auto-generated ID
			if err := mgr.Init(); err != nil {
				return nil
			}
		}
	} else {
		if err := mgr.Init(); err != nil {
			return nil
		}
	}

	id := sessionID
	if id == "" && mgr.GetHeader() != nil {
		id = mgr.GetHeader().ID
	}

	registry := tools.NewRegistry(workDir, s.sandboxMgr.GetActive())
	registry.RegisterDefaultsWithPlanTool(s.settings.IsPlanToolEnabled())
	if s.skillsMgr != nil {
		registry.Register(tools.NewSkillRefTool(s.skillsMgr))
	}

	sess := &GatewaySession{
		ID:       id,
		WorkDir:  workDir,
		Manager:  mgr,
		Registry: registry,
		Mode:     "",
		LastUsed: time.Now(),
	}

	// Create sub-agent manager if enabled
	if s.cfg.EnableSubAgents {
		compactionSettings := ctxpkg.CompactionSettings{
			Enabled:          s.settings.Compaction.Enabled,
			ReserveTokens:    s.settings.Compaction.ReserveTokens,
			KeepRecentTokens: s.settings.Compaction.KeepRecentTokens,
		}
		factory := agent.NewAgentFactory(s.provider, s.model, s.settings, s.sandboxMgr, s.extraContext, compactionSettings, nil)
		sess.AgentMgr = agent.NewAgentManager(factory)
	}

	if err := s.pool.Put(sess); err != nil {
		return nil
	}
	return sess
}

// parseMessages extracts the last user message, system messages, and history messages.
func parseMessages(msgs []RequestMessage) (lastUser string, systemMsgs []string, history []RequestMessage) {
	for _, m := range msgs {
		switch m.Role {
		case "system":
			systemMsgs = append(systemMsgs, m.Content)
		}
	}

	// Find the last user message
	lastIdx := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastIdx = i
			break
		}
	}
	if lastIdx < 0 {
		return "", systemMsgs, nil
	}
	lastUser = msgs[lastIdx].Content

	// Everything before the last user message (excluding system) is history
	for i := 0; i < lastIdx; i++ {
		if msgs[i].Role != "system" {
			history = append(history, msgs[i])
		}
	}
	return lastUser, systemMsgs, history
}

// convertHistoryMessages converts OpenAI-format history to internal provider.Message.
func convertHistoryMessages(msgs []RequestMessage) []provider.Message {
	result := make([]provider.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			result = append(result, provider.NewUserMessage(m.Content))
		case "assistant":
			result = append(result, provider.NewAssistantMessage([]provider.ContentBlock{
				{Type: "text", Text: m.Content},
			}))
		}
	}
	return result
}

// resolveToolEvent extracts tool name and call ID from an agent event,
// falling back to ToolCall fields when top-level fields are empty.
func resolveToolEvent(ev agent.Event) (name string, callID string) {
	name = ev.ToolName
	callID = ev.ToolCallID
	if ev.ToolCall != nil {
		if name == "" {
			name = ev.ToolCall.Name
		}
		if callID == "" {
			callID = ev.ToolCall.ID
		}
	}
	return name, callID
}
