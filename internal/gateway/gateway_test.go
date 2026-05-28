package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/skills"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

// --- Config tests ---

func TestDefaultGatewayConfig(t *testing.T) {
	cfg := DefaultGatewayConfig()
	if cfg.Listen != ":8080" {
		t.Errorf("default listen = %q, want :8080", cfg.Listen)
	}
	if cfg.DefaultMode != "yolo" {
		t.Errorf("default mode = %q, want yolo", cfg.DefaultMode)
	}
	if cfg.ToolVisibility.Mode != "content" {
		t.Errorf("default tool visibility = %q, want content", cfg.ToolVisibility.Mode)
	}
	if cfg.SystemPromptMode != "append" {
		t.Errorf("default system prompt mode = %q, want append", cfg.SystemPromptMode)
	}
	if cfg.RequestTimeoutSecs != 1800 {
		t.Errorf("default timeout = %d, want 1800", cfg.RequestTimeoutSecs)
	}
	if cfg.Auth.Enabled {
		t.Error("auth should be disabled by default")
	}
}

func TestLoadGatewayConfig_Missing(t *testing.T) {
	cfg, err := LoadGatewayConfigFrom("/nonexistent/path/gateway.json")
	if err != nil {
		t.Fatalf("unexpected error for missing config: %v", err)
	}
	if cfg.Listen != ":8080" {
		t.Errorf("fallback listen = %q, want :8080", cfg.Listen)
	}
}

func TestLoadGatewayConfig_Custom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.json")
	data := `{
		"listen": ":9090",
		"auth": {"enabled": true, "tokens": ["sk-test"]},
		"defaultMode": "agent",
		"toolVisibility": {"mode": "none"},
		"systemPromptMode": "ignore",
		"requestTimeoutSeconds": 600,
		"maxConcurrentRequests": 10,
		"allowedWorkDirs": ["/home/test"]
	}`
	os.WriteFile(path, []byte(data), 0644)

	cfg, err := LoadGatewayConfigFrom(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if cfg.Listen != ":9090" {
		t.Errorf("listen = %q, want :9090", cfg.Listen)
	}
	if !cfg.Auth.Enabled {
		t.Error("auth should be enabled")
	}
	if len(cfg.Auth.Tokens) != 1 || cfg.Auth.Tokens[0] != "sk-test" {
		t.Errorf("tokens = %v, want [sk-test]", cfg.Auth.Tokens)
	}
	if cfg.DefaultMode != "agent" {
		t.Errorf("mode = %q, want agent", cfg.DefaultMode)
	}
	if cfg.ToolVisibility.Mode != "none" {
		t.Errorf("tool vis = %q, want none", cfg.ToolVisibility.Mode)
	}
	if cfg.SystemPromptMode != "ignore" {
		t.Errorf("sys prompt mode = %q, want ignore", cfg.SystemPromptMode)
	}
	if cfg.RequestTimeoutSecs != 600 {
		t.Errorf("timeout = %d, want 600", cfg.RequestTimeoutSecs)
	}
	if cfg.MaxConcurrentReqs != 10 {
		t.Errorf("max concurrent = %d, want 10", cfg.MaxConcurrentReqs)
	}
	if cfg.AllowedWorkDirs == nil || len(*cfg.AllowedWorkDirs) != 1 {
		t.Error("expected 1 allowed work dir")
	}
}

func TestValidateWorkDir(t *testing.T) {
	tests := []struct {
		name    string
		allowed *[]string
		dir     string
		wantErr bool
	}{
		{"nil=no check", nil, "/any/path", false},
		{"empty=deny all", &[]string{}, "/any/path", true},
		{"exact match", &[]string{"/home/user/projects"}, "/home/user/projects", false},
		{"prefix match", &[]string{"/home/user/projects"}, "/home/user/projects/foo", false},
		{"evil prefix", &[]string{"/home/user/projects"}, "/home/user/projects-evil", true},
		{"no match", &[]string{"/opt/repos"}, "/home/user/projects", true},
		{"multi allowed", &[]string{"/opt/repos", "/home/user"}, "/home/user/foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &GatewayConfig{AllowedWorkDirs: tt.allowed}
			err := cfg.ValidateWorkDir(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkDir(%q) error = %v, wantErr = %v", tt.dir, err, tt.wantErr)
			}
		})
	}
}

func TestSaveAndLoadGatewayConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.json")
	cfg := DefaultGatewayConfig()
	if err := SaveGatewayConfig(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadGatewayConfigFrom(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Listen != ":8080" {
		t.Errorf("reloaded listen = %q", loaded.Listen)
	}
}

// --- Auth middleware tests ---

func TestAuthMiddleware_Disabled(t *testing.T) {
	handler := AuthMiddleware(AuthConfig{Enabled: false}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	handler := AuthMiddleware(AuthConfig{Enabled: true, Tokens: []string{"sk-test"}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer sk-test")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	handler := AuthMiddleware(AuthConfig{Enabled: true, Tokens: []string{"sk-test"}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	handler := AuthMiddleware(AuthConfig{Enabled: true, Tokens: []string{"sk-test"}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// --- CORS middleware tests ---

func TestCORSMiddleware_Enabled(t *testing.T) {
	handler := CORSMiddleware(CORSConfig{Enabled: true, AllowOrigins: []string{"http://example.com"}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://example.com" {
		t.Errorf("CORS origin = %q, want http://example.com", got)
	}
}

func TestCORSMiddleware_Preflight(t *testing.T) {
	handler := CORSMiddleware(CORSConfig{Enabled: true}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

// --- Concurrency middleware tests ---

func TestConcurrencyMiddleware_NoLimit(t *testing.T) {
	handler := ConcurrencyMiddleware(0, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// --- SessionPool tests ---

func TestSessionPool_PutGet(t *testing.T) {
	pool := NewSessionPool(0, 0)
	defer pool.Stop()

	sess := &GatewaySession{ID: "sess-1", WorkDir: "/tmp", LastUsed: time.Now()}
	if err := pool.Put(sess); err != nil {
		t.Fatalf("put: %v", err)
	}
	got := pool.Get("sess-1")
	if got == nil || got.ID != "sess-1" {
		t.Error("expected to get session back")
	}
	if pool.Count() != 1 {
		t.Errorf("count = %d, want 1", pool.Count())
	}
}

func TestSessionPool_MaxSessions(t *testing.T) {
	pool := NewSessionPool(1, 0)
	defer pool.Stop()

	sess1 := &GatewaySession{ID: "sess-1", LastUsed: time.Now()}
	if err := pool.Put(sess1); err != nil {
		t.Fatalf("put 1: %v", err)
	}
	sess2 := &GatewaySession{ID: "sess-2", LastUsed: time.Now()}
	if err := pool.Put(sess2); err == nil {
		t.Error("expected pool full error")
	}
}

func TestSessionPool_Remove(t *testing.T) {
	pool := NewSessionPool(0, 0)
	defer pool.Stop()

	pool.Put(&GatewaySession{ID: "sess-1", LastUsed: time.Now()})
	pool.Remove("sess-1")
	if pool.Get("sess-1") != nil {
		t.Error("session should be removed")
	}
}

func TestSessionPool_List(t *testing.T) {
	pool := NewSessionPool(0, 0)
	defer pool.Stop()

	pool.Put(&GatewaySession{ID: "a", LastUsed: time.Now()})
	pool.Put(&GatewaySession{ID: "b", LastUsed: time.Now()})
	ids := pool.List()
	if len(ids) != 2 {
		t.Errorf("list len = %d, want 2", len(ids))
	}
}

// --- parseMessages tests ---

func TestParseMessages(t *testing.T) {
	msgs := []RequestMessage{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "explain main.go"},
	}
	lastUser, sysMsgs, history := parseMessages(msgs)
	if lastUser != "explain main.go" {
		t.Errorf("lastUser = %q", lastUser)
	}
	if len(sysMsgs) != 1 || sysMsgs[0] != "you are helpful" {
		t.Errorf("sysMsgs = %v", sysMsgs)
	}
	if len(history) != 2 { // "hello" and "hi there"
		t.Errorf("history len = %d, want 2", len(history))
	}
}

func TestParseMessages_NoUser(t *testing.T) {
	msgs := []RequestMessage{
		{Role: "system", Content: "test"},
	}
	lastUser, _, _ := parseMessages(msgs)
	if lastUser != "" {
		t.Errorf("expected empty lastUser, got %q", lastUser)
	}
}

// --- SSE writer tests ---

func TestSSEWriter_ContentDelta(t *testing.T) {
	w := httptest.NewRecorder()
	sse := NewSSEWriter(w, "test-model", "sess-1")
	sse.WriteContentDelta("hello")
	body := w.Body.String()
	if !strings.Contains(body, `"content":"hello"`) {
		t.Errorf("body doesn't contain content delta: %s", body)
	}
	if !strings.HasPrefix(body, "data: ") {
		t.Error("SSE data should start with 'data: '")
	}
}

func TestSSEWriter_Done(t *testing.T) {
	w := httptest.NewRecorder()
	sse := NewSSEWriter(w, "test-model", "sess-1")
	sse.WriteDone(&CompletionUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150})
	body := w.Body.String()
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Errorf("missing finish_reason: %s", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Error("missing [DONE] sentinel")
	}
}

func TestSSEWriter_ToolStatusContent(t *testing.T) {
	w := httptest.NewRecorder()
	sse := NewSSEWriter(w, "test-model", "")
	sse.WriteToolStatusContent("🔧 [read] main.go", "running")
	body := w.Body.String()
	if !strings.Contains(body, "[running]") {
		t.Errorf("missing status in content: %s", body)
	}
	if !strings.Contains(body, "read") {
		t.Errorf("missing tool name in content: %s", body)
	}
}

func TestSSEWriter_ToolStatusEvent(t *testing.T) {
	w := httptest.NewRecorder()
	sse := NewSSEWriter(w, "test-model", "")
	sse.WriteToolStatusEvent("bash", "running", map[string]any{"command": "ls"})
	body := w.Body.String()
	if !strings.Contains(body, "event: tool_status") {
		t.Errorf("missing tool_status event: %s", body)
	}
	if !strings.Contains(body, `"tool":"bash"`) {
		t.Errorf("missing tool name: %s", body)
	}
}

// --- writeError / writeJSON tests ---

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "bad input", "invalid_request_error")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Message != "bad input" {
		t.Errorf("error message = %q", resp.Error.Message)
	}
}

// --- Health handler test ---

func TestHealthHandler(t *testing.T) {
	srv := &Server{
		version: "test",
		pool:    NewSessionPool(0, 0),
	}
	defer srv.pool.Stop()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp HealthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "ok" {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.Version != "test" {
		t.Errorf("version = %q", resp.Version)
	}
}

// --- Models handler test ---

func TestModelsHandler(t *testing.T) {
	mockP := provider.NewMockProvider("test", []*provider.Model{
		{ID: "m1", Name: "Model 1"},
		{ID: "m2", Name: "Model 2"},
	}, nil)
	srv := &Server{
		provider: mockP,
	}
	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()
	srv.handleModels(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp ModelListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Object != "list" {
		t.Errorf("object = %q", resp.Object)
	}
	if len(resp.Data) != 2 {
		t.Errorf("models = %d, want 2", len(resp.Data))
	}
}

// --- Chat handler slash command test ---

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cwd := t.TempDir()
	models := []*provider.Model{
		{ID: "m1", Name: "Model 1"},
	}
	mockP := provider.NewMockProvider("test", models, nil)

	settings := config.DefaultSettings()
	settings.SessionDir = filepath.Join(cwd, "sessions")

	sbMgr := sandbox.NewManager(cwd)
	sbMgr.SetLevel(sandbox.LevelNone)

	skillsMgr := skills.NewManager(filepath.Join(cwd, "skills-global"), filepath.Join(cwd, "skills-project"))

	pool := NewSessionPool(0, 0)

	return &Server{
		cfg:        DefaultGatewayConfig(),
		settings:   settings,
		version:    "test",
		provider:   mockP,
		model:      models[0],
		sandboxMgr: sbMgr,
		skillsMgr:  skillsMgr,
		pool:       pool,
	}
}

func TestChatHandler_SlashHelp(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	body := `{"messages":[{"role":"user","content":"/help"}],"stream":false}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.XCommand != "/help" {
		t.Errorf("x_command = %q, want /help", resp.XCommand)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
		t.Fatal("missing choice")
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "/clear") {
		t.Error("help output should mention /clear")
	}
}

func TestChatHandler_SlashClear(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	body := `{"messages":[{"role":"user","content":"/clear"}],"stream":false,"x_session_id":"test-sess"}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.XCommand != "/clear" {
		t.Errorf("x_command = %q, want /clear", resp.XCommand)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "Conversation cleared") {
		t.Errorf("expected clear confirmation, got %q", resp.Choices[0].Message.Content)
	}
}

func TestChatHandler_SlashMode(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	body := `{"messages":[{"role":"user","content":"/mode plan"}],"stream":false,"x_session_id":"mode-sess"}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp.Choices[0].Message.Content, "PLAN") {
		t.Errorf("expected PLAN in response, got %q", resp.Choices[0].Message.Content)
	}
}

func TestChatHandler_EmptyMessages(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	body := `{"messages":[]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestChatHandler_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestChatHandler_WorkDirForbidden(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	// Set restrictive allowedWorkDirs
	allowed := []string{"/opt/allowed"}
	srv.cfg.AllowedWorkDirs = &allowed

	body := `{"messages":[{"role":"user","content":"hi"}],"x_working_dir":"/etc/evil"}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

// --- Commands tests ---

func TestCommands_UnknownCommand(t *testing.T) {
	srv := newTestServer(t)
	result := srv.handleCommand(nil, "/foobar")
	if result == nil {
		t.Fatal("expected result for unknown command")
	}
	if !result.Error {
		t.Error("expected error=true for unknown command")
	}
}

func TestCommands_NotACommand(t *testing.T) {
	srv := newTestServer(t)
	result := srv.handleCommand(nil, "hello world")
	if result != nil {
		t.Error("non-command should return nil")
	}
}

func TestCommands_Status(t *testing.T) {
	srv := newTestServer(t)
	sess := &GatewaySession{ID: "test-sess", WorkDir: "/tmp", Mode: "agent"}
	result := srv.cmdStatus(sess)
	if result == nil {
		t.Fatal("expected result")
	}
	if !strings.Contains(result.Message, "AGENT") {
		t.Errorf("status should show mode, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "test-sess") {
		t.Errorf("status should show session ID, got %q", result.Message)
	}
}

func TestCommands_CompactNoSession(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdCompact(nil)
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.Error {
		t.Error("expected error for nil session")
	}
}

func TestCommands_CompactTooShort(t *testing.T) {
	srv := newTestServer(t)
	// Create a session with less than 2 messages
	sess := &GatewaySession{ID: "test-sess", WorkDir: "/tmp"}
	mgr := session.New(t.TempDir(), t.TempDir())
	mgr.Init()
	sess.Manager = mgr
	result := srv.cmdCompact(sess)
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.Error {
		t.Error("expected error for too-short conversation")
	}
	if !strings.Contains(result.Message, "too short") {
		t.Errorf("expected 'too short' message, got %q", result.Message)
	}
}

func TestCommands_CompactSetsFlag(t *testing.T) {
	srv := newTestServer(t)
	sess := &GatewaySession{ID: "test-sess", WorkDir: t.TempDir()}
	mgr := session.New(sess.WorkDir, t.TempDir())
	mgr.Init()
	// Append 2 messages so conversation is long enough
	mgr.AppendMessage(provider.NewUserMessage("hello"))
	mgr.AppendMessage(provider.NewAssistantMessage([]provider.ContentBlock{{Type: "text", Text: "hi"}}))
	sess.Manager = mgr

	result := srv.cmdCompact(sess)
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Error {
		t.Errorf("unexpected error: %s", result.Message)
	}
	if !sess.ForceCompact {
		t.Error("expected ForceCompact to be set")
	}
	if !strings.Contains(result.Message, "compaction") {
		t.Errorf("expected compaction confirmation, got %q", result.Message)
	}
}

func TestChatHandler_SlashCompact(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	body := `{"messages":[{"role":"user","content":"/compact"}],"stream":false,"x_session_id":"compact-sess"}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp ChatCompletionResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.XCommand != "/compact" {
		t.Errorf("x_command = %q, want /compact", resp.XCommand)
	}
}

// --- Tool format tests ---

func TestFormatToolExpanded_Read(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "read",
		Args:   map[string]any{"path": "main.go"},
		Status: "completed",
		Result: "package main\n\nfunc main() {}\n",
	}
	text := formatToolExpanded(tc)
	// Markdown header
	if !strings.Contains(text, "🔧 read: main.go") {
		t.Errorf("missing markdown header: %s", text)
	}
	// Code fence with language
	if !strings.Contains(text, "```go\n") {
		t.Errorf("missing go code fence: %s", text)
	}
	if !strings.Contains(text, "package main") {
		t.Errorf("missing result content: %s", text)
	}
	// Closing fence
	if !strings.Contains(text, "\n```") {
		t.Errorf("missing closing fence: %s", text)
	}
}

func TestFormatToolExpanded_Bash(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "bash",
		Args:   map[string]any{"command": "go test ./..."},
		Status: "completed",
		Result: "ok  pkg 0.5s\n",
	}
	text := formatToolExpanded(tc)
	if !strings.Contains(text, "🔧 bash: go test ./...") {
		t.Errorf("missing markdown header: %s", text)
	}
	if !strings.Contains(text, "```bash\n") {
		t.Errorf("missing bash code fence: %s", text)
	}
	if !strings.Contains(text, "ok  pkg") {
		t.Errorf("missing stdout: %s", text)
	}
}

func TestFormatToolExpanded_EditWithDiff(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "edit",
		Args:   map[string]any{"path": "main.go"},
		Status: "completed",
		Diff:   &tools.FileDiff{Path: "main.go", Added: 2, Deleted: 1, Unified: "+func new1() {}\n-func old() {}\n"},
	}
	text := formatToolExpanded(tc)
	if !strings.Contains(text, "```diff\n") {
		t.Errorf("missing diff code fence: %s", text)
	}
	if !strings.Contains(text, "+func new1") {
		t.Errorf("missing diff content: %s", text)
	}
}

func TestFormatToolExpanded_Error(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "bash",
		Args:   map[string]any{"command": "false"},
		Status: "failed",
		Error:  fmt.Errorf("exit code 1"),
	}
	text := formatToolExpanded(tc)
	if !strings.Contains(text, "Error: exit code 1") {
		t.Errorf("missing error: %s", text)
	}
}

func TestFormatToolExpanded_ReadJSON(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "read",
		Args:   map[string]any{"path": "package.json"},
		Status: "completed",
		Result: `{"name": "test"}`,
	}
	text := formatToolExpanded(tc)
	if !strings.Contains(text, "```json\n") {
		t.Errorf("should use json fence for .json file: %s", text)
	}
}

func TestFormatToolExpanded_GrepPlain(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "grep",
		Args:   map[string]any{"pattern": "TODO", "path": "./src"},
		Status: "completed",
		Result: "src/main.go:10: // TODO fix this\n",
	}
	text := formatToolExpanded(tc)
	// grep should use plain text fence (no language)
	if !strings.Contains(text, "```\n") {
		t.Errorf("grep should use plain code fence: %s", text)
	}
}

func TestFormatToolRunning(t *testing.T) {
	text := formatToolRunning("read", map[string]any{"path": "main.go"})
	if !strings.Contains(text, "\u23f3") {
		t.Errorf("missing hourglass: %s", text)
	}
	if !strings.Contains(text, "read") {
		t.Errorf("missing tool name: %s", text)
	}
}

func TestInferCodeLang(t *testing.T) {
	tests := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"bash", nil, "bash"},
		{"read", map[string]any{"path": "main.go"}, "go"},
		{"read", map[string]any{"path": "app.py"}, "python"},
		{"read", map[string]any{"path": "style.css"}, "css"},
		{"read", map[string]any{"path": "Makefile"}, "makefile"},
		{"read", map[string]any{"path": "Dockerfile"}, "dockerfile"},
		{"read", map[string]any{"path": "data.json"}, "json"},
		{"grep", map[string]any{"pattern": "x"}, ""},
		{"ls", nil, ""},
	}
	for _, tt := range tests {
		got := inferCodeLang(tt.tool, tt.args)
		if got != tt.want {
			t.Errorf("inferCodeLang(%q, %v) = %q, want %q", tt.tool, tt.args, got, tt.want)
		}
	}
}

func TestToolKeyArg(t *testing.T) {
	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"read path", "read", map[string]any{"path": "main.go"}, "main.go"},
		{"bash command", "bash", map[string]any{"command": "ls -la"}, "ls -la"},
		{"grep", "grep", map[string]any{"pattern": "TODO", "path": "src/"}, "TODO src/"},
		{"nil args", "read", nil, ""},
		{"unknown tool", "foo", map[string]any{"name": "bar"}, "bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolKeyArg(tt.tool, tt.args)
			if got != tt.want {
				t.Errorf("toolKeyArg(%q) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}

func TestChatHandler_SlashHelp_Streaming(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	body := `{"messages":[{"role":"user","content":"/help"}],"stream":true}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resBody := w.Body.String()
	if !strings.Contains(resBody, "data: ") {
		t.Error("streaming response should contain SSE data lines")
	}
	if !strings.Contains(resBody, "[DONE]") {
		t.Error("streaming response should end with [DONE]")
	}
	if !strings.Contains(resBody, "/clear") {
		t.Error("help content should mention /clear")
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

// --- Collapsed mode tests ---

func TestFormatToolCollapsed_Read(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "read",
		Args:   map[string]any{"path": "main.go"},
		Status: "completed",
		Result: "package main\n\nfunc main() {}\n",
	}
	text := formatToolCollapsed(tc)
	if !strings.Contains(text, "read") {
		t.Errorf("missing tool name: %s", text)
	}
	if !strings.Contains(text, "main.go") {
		t.Errorf("missing path: %s", text)
	}
	if !strings.Contains(text, "✅") {
		t.Errorf("missing success marker: %s", text)
	}
	// Should NOT contain the file content
	if strings.Contains(text, "package main") {
		t.Errorf("collapsed should not contain file content: %s", text)
	}
	if strings.Contains(text, "```") {
		t.Errorf("collapsed should not contain code fences: %s", text)
	}
}

func TestFormatToolCollapsed_EditShowsDiff(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "edit",
		Args:   map[string]any{"path": "main.go"},
		Status: "completed",
		Diff:   &tools.FileDiff{Path: "main.go", Added: 1, Deleted: 1, Unified: "+new line\n-old line\n"},
	}
	text := formatToolCollapsed(tc)
	// edit with diff should always show the diff even in collapsed mode
	if !strings.Contains(text, "```diff") {
		t.Errorf("collapsed edit should show diff fence: %s", text)
	}
	if !strings.Contains(text, "+new line") {
		t.Errorf("collapsed edit should show diff content: %s", text)
	}
}

func TestFormatToolCollapsed_ErrorAlwaysShown(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "bash",
		Args:   map[string]any{"command": "false"},
		Status: "failed",
		Error:  fmt.Errorf("exit code 1"),
	}
	text := formatToolCollapsed(tc)
	if !strings.Contains(text, "Error: exit code 1") {
		t.Errorf("collapsed error should always show: %s", text)
	}
}

func TestFormatToolCollapsed_BashNoOutput(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "bash",
		Args:   map[string]any{"command": "go test ./..."},
		Status: "completed",
		Result: "ok  pkg 0.5s\n",
	}
	text := formatToolCollapsed(tc)
	if !strings.Contains(text, "✅") {
		t.Errorf("missing success marker: %s", text)
	}
	if strings.Contains(text, "ok  pkg") {
		t.Errorf("collapsed bash should not show stdout: %s", text)
	}
}

// --- Dispatcher test ---

func TestFormatToolResult_Dispatches(t *testing.T) {
	tc := &toolCallInfo{
		Name:   "read",
		Args:   map[string]any{"path": "main.go"},
		Status: "completed",
		Result: "package main\n",
	}

	collapsed := formatToolResult(tc, "collapsed")
	expanded := formatToolResult(tc, "expanded")

	if strings.Contains(collapsed, "```go") {
		t.Error("collapsed should not have code fence")
	}
	if !strings.Contains(expanded, "```go") {
		t.Error("expanded should have code fence")
	}
}

// --- Project-level config test ---

func TestLoadGatewayConfig_ProjectOverlay(t *testing.T) {
	dir := t.TempDir()

	// Create global config
	globalDir := filepath.Join(dir, "global")
	globalPath := filepath.Join(globalDir, "gateway.json")
	globalCfg := DefaultGatewayConfig()
	globalCfg.Listen = ":9090"
	globalCfg.DefaultMode = "agent"
	SaveGatewayConfig(globalPath, globalCfg)

	// Create project config that overrides some fields
	projectDir := filepath.Join(dir, "project", ".vibe")
	os.MkdirAll(projectDir, 0755)
	projectPath := filepath.Join(projectDir, "gateway.json")
	os.WriteFile(projectPath, []byte(`{"defaultMode":"yolo","toolVisibility":{"detail":"expanded"}}`), 0644)

	// Load global
	cfg, err := LoadGatewayConfigFrom(globalPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DefaultMode != "agent" {
		t.Errorf("global mode = %q", cfg.DefaultMode)
	}

	// Overlay project (simulating what LoadGatewayConfig does)
	data, _ := os.ReadFile(projectPath)
	json.Unmarshal(data, cfg)
	normalizeConfig(cfg)

	if cfg.DefaultMode != "yolo" {
		t.Errorf("project should override mode to yolo, got %q", cfg.DefaultMode)
	}
	if cfg.Listen != ":9090" {
		t.Errorf("listen should be preserved from global, got %q", cfg.Listen)
	}
	if cfg.ToolVisibility.Detail != "expanded" {
		t.Errorf("detail should be overridden to expanded, got %q", cfg.ToolVisibility.Detail)
	}
}

func TestToolVisibility_DefaultDetail(t *testing.T) {
	cfg := DefaultGatewayConfig()
	if cfg.GetToolDetail() != "collapsed" {
		t.Errorf("default detail = %q, want collapsed", cfg.GetToolDetail())
	}
}

// --- CORS middleware disabled test ---

func TestCORSMiddleware_Disabled(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CORSMiddleware(CORSConfig{Enabled: false}, inner)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	// CORS headers should NOT be set
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS origin should be empty, got %q", got)
	}
}

func TestCORSMiddleware_DefaultOrigins(t *testing.T) {
	handler := CORSMiddleware(CORSConfig{Enabled: true}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS origin = %q, want *", got)
	}
}

// --- Concurrency middleware at capacity test ---

func TestConcurrencyMiddleware_AtCapacity(t *testing.T) {
	blocking := make(chan struct{})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocking // block until released
		w.WriteHeader(http.StatusOK)
	})
	handler := ConcurrencyMiddleware(1, inner)

	// Fill the single slot
	go func() {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}()

	// Give goroutine time to start
	time.Sleep(20 * time.Millisecond)

	// Second request should be rejected
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}

	// Release the blocking goroutine
	close(blocking)
}

// --- Auth with non-Bearer prefix ---

func TestAuthMiddleware_NonBearerPrefix(t *testing.T) {
	handler := AuthMiddleware(AuthConfig{Enabled: true, Tokens: []string{"sk-test"}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// --- extractBearerToken tests ---

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name string
		auth string
		want string
	}{
		{"empty", "", ""},
		{"bearer", "Bearer sk-test", "sk-test"},
		{"bearer with spaces", "Bearer  sk-test ", "sk-test"},
		{"basic", "Basic dXNlcjpwYXNz", ""},
		{"no prefix", "sk-test", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			got := extractBearerToken(req)
			if got != tt.want {
				t.Errorf("extractBearerToken(%q) = %q, want %q", tt.auth, got, tt.want)
			}
		})
	}
}

// --- SessionPool advanced tests ---

func TestSessionPool_ReplaceSameID(t *testing.T) {
	pool := NewSessionPool(1, 0)
	defer pool.Stop()

	sess1 := &GatewaySession{ID: "sess-1", WorkDir: "/tmp/a", LastUsed: time.Now()}
	if err := pool.Put(sess1); err != nil {
		t.Fatalf("put 1: %v", err)
	}

	// Replace same ID should succeed even at max capacity
	sess1v2 := &GatewaySession{ID: "sess-1", WorkDir: "/tmp/b", LastUsed: time.Now()}
	if err := pool.Put(sess1v2); err != nil {
		t.Fatalf("replace same ID should succeed: %v", err)
	}

	got := pool.Get("sess-1")
	if got.WorkDir != "/tmp/b" {
		t.Errorf("workdir = %q, want /tmp/b", got.WorkDir)
	}
}

func TestSessionPool_EvictIdle(t *testing.T) {
	pool := NewSessionPool(0, 50*time.Millisecond)
	defer pool.Stop()

	sess := &GatewaySession{ID: "sess-1", LastUsed: time.Now()}
	pool.Put(sess)
	// Manually backdate LastUsed after Put (which calls Touch)
	sess.LastUsed = time.Now().Add(-time.Hour)

	pool.evictIdle()

	if pool.Get("sess-1") != nil {
		t.Error("idle session should be evicted")
	}
}

func TestSessionPool_EvictIdleKeepsFresh(t *testing.T) {
	pool := NewSessionPool(0, time.Hour)
	defer pool.Stop()

	sess := &GatewaySession{ID: "sess-1", LastUsed: time.Now()}
	pool.Put(sess)

	pool.evictIdle()

	if pool.Get("sess-1") == nil {
		t.Error("fresh session should not be evicted")
	}
}

func TestPoolFullError_Error(t *testing.T) {
	e := &PoolFullError{Max: 5}
	if e.Error() != "session pool is at capacity" {
		t.Errorf("error = %q", e.Error())
	}
}

// --- parseMessages advanced tests ---

func TestParseMessages_MultipleSystem(t *testing.T) {
	msgs := []RequestMessage{
		{Role: "system", Content: "sys1"},
		{Role: "system", Content: "sys2"},
		{Role: "user", Content: "hello"},
	}
	lastUser, sysMsgs, history := parseMessages(msgs)
	if lastUser != "hello" {
		t.Errorf("lastUser = %q", lastUser)
	}
	if len(sysMsgs) != 2 {
		t.Errorf("sysMsgs len = %d, want 2", len(sysMsgs))
	}
	if len(history) != 0 {
		t.Errorf("history len = %d, want 0", len(history))
	}
}

func TestParseMessages_SingleUser(t *testing.T) {
	msgs := []RequestMessage{
		{Role: "user", Content: "only message"},
	}
	lastUser, sysMsgs, history := parseMessages(msgs)
	if lastUser != "only message" {
		t.Errorf("lastUser = %q", lastUser)
	}
	if len(sysMsgs) != 0 {
		t.Errorf("sysMsgs len = %d", len(sysMsgs))
	}
	if len(history) != 0 {
		t.Errorf("history len = %d", len(history))
	}
}

// --- convertHistoryMessages tests ---

func TestConvertHistoryMessages(t *testing.T) {
	msgs := []RequestMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "system", Content: "ignored"},
	}
	result := convertHistoryMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("result len = %d, want 2", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("result[0].Role = %q", result[0].Role)
	}
	if result[1].Role != "assistant" {
		t.Errorf("result[1].Role = %q", result[1].Role)
	}
}

func TestConvertHistoryMessages_Empty(t *testing.T) {
	result := convertHistoryMessages(nil)
	if len(result) != 0 {
		t.Errorf("result len = %d, want 0", len(result))
	}
}

// --- resolveToolEvent tests ---

func TestResolveToolEvent_FromTopLevel(t *testing.T) {
	ev := agent.Event{
		ToolName:   "read",
		ToolCallID: "call-1",
	}
	name, callID := resolveToolEvent(ev)
	if name != "read" {
		t.Errorf("name = %q", name)
	}
	if callID != "call-1" {
		t.Errorf("callID = %q", callID)
	}
}

func TestResolveToolEvent_FallbackToToolCall(t *testing.T) {
	ev := agent.Event{
		ToolCall: &provider.ToolCallBlock{
			ID:   "call-2",
			Name: "bash",
		},
	}
	name, callID := resolveToolEvent(ev)
	if name != "bash" {
		t.Errorf("name = %q", name)
	}
	if callID != "call-2" {
		t.Errorf("callID = %q", callID)
	}
}

func TestResolveToolEvent_TopLevelTakesPrecedence(t *testing.T) {
	ev := agent.Event{
		ToolName:   "read",
		ToolCallID: "call-1",
		ToolCall: &provider.ToolCallBlock{
			ID:   "call-2",
			Name: "bash",
		},
	}
	name, callID := resolveToolEvent(ev)
	if name != "read" {
		t.Errorf("name = %q, want read", name)
	}
	if callID != "call-1" {
		t.Errorf("callID = %q, want call-1", callID)
	}
}

// --- Commands: mode/model/sessions edge cases ---

func TestCommands_ModeInvalid(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdMode(nil, []string{"/mode", "invalid"})
	if !result.Error {
		t.Error("expected error for invalid mode")
	}
}

func TestCommands_ModeShowCurrent(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdMode(nil, []string{"/mode"})
	if result.Error {
		t.Error("unexpected error")
	}
	if !strings.Contains(result.Message, "YOLO") {
		t.Errorf("expected current mode YOLO, got %q", result.Message)
	}
}

func TestCommands_ModeShowSessionOverride(t *testing.T) {
	srv := newTestServer(t)
	sess := &GatewaySession{ID: "s1", Mode: "plan"}
	result := srv.cmdMode(sess, []string{"/mode"})
	if !strings.Contains(result.Message, "PLAN") {
		t.Errorf("expected PLAN, got %q", result.Message)
	}
}

func TestCommands_ModelNotFound(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdModel([]string{"/model", "nonexistent"})
	if !result.Error {
		t.Error("expected error for unknown model")
	}
}

func TestCommands_ModelShowCurrent(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdModel([]string{"/model"})
	if result.Error {
		t.Error("unexpected error")
	}
	if !strings.Contains(result.Message, "Model 1") {
		t.Errorf("expected Model 1, got %q", result.Message)
	}
}

func TestCommands_SessionsList(t *testing.T) {
	srv := newTestServer(t)
	srv.pool.Put(&GatewaySession{ID: "s1", LastUsed: time.Now()})
	srv.pool.Put(&GatewaySession{ID: "s2", LastUsed: time.Now()})

	result := srv.cmdSessions([]string{"/sessions"})
	if result.Error {
		t.Error("unexpected error")
	}
	if !strings.Contains(result.Message, "s1") || !strings.Contains(result.Message, "s2") {
		t.Errorf("expected both session IDs, got %q", result.Message)
	}
}

func TestCommands_SessionsEmpty(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdSessions([]string{"/sessions"})
	if !strings.Contains(result.Message, "No active sessions") {
		t.Errorf("expected no sessions message, got %q", result.Message)
	}
}

func TestCommands_SessionsDelete(t *testing.T) {
	srv := newTestServer(t)
	srv.pool.Put(&GatewaySession{ID: "s1", LastUsed: time.Now()})
	result := srv.cmdSessions([]string{"/sessions", "del", "s1"})
	if result.Error {
		t.Error("unexpected error")
	}
	if srv.pool.Get("s1") != nil {
		t.Error("session should be deleted")
	}
}

func TestCommands_SessionsDeleteNotFound(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdSessions([]string{"/sessions", "del", "nonexistent"})
	if !result.Error {
		t.Error("expected error for missing session")
	}
}

func TestCommands_SessionsDeleteMissingID(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdSessions([]string{"/sessions", "del"})
	if !result.Error {
		t.Error("expected error for missing ID")
	}
}

func TestCommands_SessionsUnknownSubcmd(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdSessions([]string{"/sessions", "badcmd"})
	if !result.Error {
		t.Error("expected error for unknown subcmd")
	}
}

func TestCommands_StatusNoSession(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdStatus(nil)
	if !result.Error {
		t.Error("expected error for nil session")
	}
}

func TestCommands_SkillNoManager(t *testing.T) {
	srv := newTestServer(t)
	srv.skillsMgr = nil
	result := srv.cmdSkill([]string{"/skill", "test"})
	if !result.Error {
		t.Error("expected error when no skills manager")
	}
}

func TestCommands_SkillNotFound(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdSkill([]string{"/skill", "nonexistent"})
	if !result.Error {
		t.Error("expected error for unknown skill")
	}
}

func TestCommands_SkillsEmpty(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdSkills()
	if !strings.Contains(result.Message, "No skills found") {
		t.Errorf("expected no skills message, got %q", result.Message)
	}
}

func TestCommands_Help(t *testing.T) {
	srv := newTestServer(t)
	result := srv.cmdHelp()
	for _, cmd := range []string{"/clear", "/mode", "/model", "/compact", "/help"} {
		if !strings.Contains(result.Message, cmd) {
			t.Errorf("help missing %s", cmd)
		}
	}
}

// --- Chat handler method-not-allowed test ---

func TestChatHandler_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	defer srv.pool.Stop()

	req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

// --- Type helper tests ---

func TestNewCompletionID(t *testing.T) {
	id := newCompletionID()
	if !strings.HasPrefix(id, "chatcmpl-") {
		t.Errorf("id = %q, want chatcmpl- prefix", id)
	}
}

func TestNewCommandCompletionID(t *testing.T) {
	id := newCommandCompletionID()
	if !strings.HasPrefix(id, "chatcmpl-cmd-") {
		t.Errorf("id = %q, want chatcmpl-cmd- prefix", id)
	}
}

func TestStringPtr(t *testing.T) {
	p := stringPtr("test")
	if *p != "test" {
		t.Errorf("*p = %q", *p)
	}
}

func TestMarshalJSON(t *testing.T) {
	data := marshalJSON(map[string]string{"key": "val"})
	if !strings.Contains(string(data), "key") {
		t.Errorf("data = %s", data)
	}
}

// --- langFromPath extended tests ---

func TestLangFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"index.js", "javascript"},
		{"app.ts", "typescript"},
		{"comp.tsx", "tsx"},
		{"comp.jsx", "jsx"},
		{"main.rs", "rust"},
		{"app.rb", "ruby"},
		{"Main.java", "java"},
		{"main.c", "c"},
		{"main.h", "c"},
		{"main.cpp", "cpp"},
		{"main.cc", "cpp"},
		{"main.cs", "csharp"},
		{"main.swift", "swift"},
		{"main.kt", "kotlin"},
		{"script.sh", "bash"},
		{"script.bash", "bash"},
		{"script.zsh", "zsh"},
		{"script.ps1", "powershell"},
		{"query.sql", "sql"},
		{"index.html", "html"},
		{"style.css", "css"},
		{"style.scss", "scss"},
		{"data.json", "json"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"config.toml", "toml"},
		{"data.xml", "xml"},
		{"README.md", "markdown"},
		{"main.tf", "hcl"},
		{"main.lua", "lua"},
		{"main.php", "php"},
		{"main.pl", "perl"},
		{"main.ex", "elixir"},
		{"main.erl", "erlang"},
		{"main.hs", "haskell"},
		{"main.scala", "scala"},
		{"main.clj", "clojure"},
		{"main.vim", "vim"},
		{"schema.proto", "protobuf"},
		{"schema.graphql", "graphql"},
		{"config.ini", "ini"},
		{".env", "bash"},
		{"Makefile", "makefile"},
		{"Dockerfile", "dockerfile"},
		{"Gemfile", "ruby"},
		{"unknown.xyz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := langFromPath(tt.path)
			if got != tt.want {
				t.Errorf("langFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// --- formatToolHeaderMD tests ---

func TestFormatToolHeaderMD(t *testing.T) {
	got := formatToolHeaderMD("read", map[string]any{"path": "main.go"})
	if got != "🔧 read: main.go" {
		t.Errorf("got %q", got)
	}
	got2 := formatToolHeaderMD("plan", nil)
	if got2 != "🔧 plan" {
		t.Errorf("got %q", got2)
	}
}

// --- formatToolHeader tests ---

func TestFormatToolHeader(t *testing.T) {
	got := formatToolHeader("bash", map[string]any{"command": "ls"})
	if got != "🔧 [bash] ls" {
		t.Errorf("got %q", got)
	}
	got2 := formatToolHeader("plan", nil)
	if got2 != "🔧 [plan]" {
		t.Errorf("got %q", got2)
	}
}

// --- toolKeyArg: bash long command truncation ---

func TestToolKeyArg_BashLongCommand(t *testing.T) {
	longCmd := strings.Repeat("a", 200)
	got := toolKeyArg("bash", map[string]any{"command": longCmd})
	if len(got) > 124 { // 120 + "..."
		t.Errorf("expected truncated, got len %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected ... suffix")
	}
}

// --- GatewaySession Touch/Lock ---

func TestGatewaySession_Touch(t *testing.T) {
	sess := &GatewaySession{ID: "s1"}
	sess.Touch()
	if sess.LastUsed.IsZero() {
		t.Error("expected non-zero LastUsed after Touch")
	}
}

func TestGatewaySession_LockUnlock(t *testing.T) {
	sess := &GatewaySession{ID: "s1"}
	sess.Lock()
	sess.Unlock()
	// No panic = pass
}
