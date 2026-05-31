package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewGateway(t *testing.T) {
	gw := NewGateway("localhost:8090", "test-token", "0.1.27")
	if gw == nil {
		t.Fatal("expected gateway")
	}
	if gw.version != "0.1.27" {
		t.Errorf("expected version 0.1.27, got %s", gw.version)
	}
	if gw.authToken != "test-token" {
		t.Errorf("expected token test-token, got %s", gw.authToken)
	}
}

func TestGatewayConnectionCount(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")
	if gw.ConnectionCount() != 0 {
		t.Errorf("expected 0 connections, got %d", gw.ConnectionCount())
	}
}

func TestHandleHealth(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	gw.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("expected ok, got %v", result["status"])
	}
	if result["version"] != "0.1.27" {
		t.Errorf("expected 0.1.27, got %v", result["version"])
	}
}

func TestHandleHealthMethodNotAllowed(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	req := httptest.NewRequest("POST", "/api/health", nil)
	w := httptest.NewRecorder()
	gw.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleStatus(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	gw.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["version"] != "0.1.27" {
		t.Errorf("expected 0.1.27, got %v", result["version"])
	}
}

func TestHandleSessions(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	// No dispatcher set
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	gw.handleSessions(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleMemoryNoStore(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	req := httptest.NewRequest("GET", "/api/memory", nil)
	w := httptest.NewRecorder()
	gw.handleMemory(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandlePlatforms(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	req := httptest.NewRequest("GET", "/api/platforms", nil)
	w := httptest.NewRecorder()
	gw.handlePlatforms(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	platforms, ok := result["platforms"].([]any)
	if !ok {
		t.Fatal("expected platforms array")
	}
	if len(platforms) != 0 {
		t.Errorf("expected 0 platforms, got %d", len(platforms))
	}
}

func TestWithAuthNoToken(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	called := false
	handler := gw.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("expected handler to be called (no auth configured)")
	}
}

func TestWithAuthValidToken(t *testing.T) {
	gw := NewGateway("localhost:8090", "secret", "0.1.27")

	called := false
	handler := gw.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("expected handler to be called (valid token)")
	}
}

func TestWithAuthInvalidToken(t *testing.T) {
	gw := NewGateway("localhost:8090", "secret", "0.1.27")

	called := false
	handler := gw.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Error("expected handler NOT to be called (invalid token)")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWithAuthQueryToken(t *testing.T) {
	gw := NewGateway("localhost:8090", "secret", "0.1.27")

	called := false
	handler := gw.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test?token=secret", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("expected handler to be called (query token)")
	}
}

func TestWithAuthNoAuthHeader(t *testing.T) {
	gw := NewGateway("localhost:8090", "secret", "0.1.27")

	called := false
	handler := gw.withAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Error("expected handler NOT to be called (no auth)")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSessionInfo(t *testing.T) {
	info := SessionInfo{
		ID:           "test-session",
		Platform:     "ws",
		UserID:       "user1",
		WorkDir:      "/tmp",
		Mode:         "yolo",
		MessageCount: 5,
		LastActive:   time.Now(),
		Preview:      "hello",
	}

	if info.ID != "test-session" {
		t.Errorf("expected test-session, got %s", info.ID)
	}
	if info.Platform != "ws" {
		t.Errorf("expected ws, got %s", info.Platform)
	}
}

func TestPlatformStatus(t *testing.T) {
	status := PlatformStatus{
		Name:        "wechat",
		Enabled:     true,
		Connected:   true,
		WorkDir:     "/tmp",
		ActiveUsers: []string{"user1", "user2"},
		LoginStatus: "logged_in",
	}

	if status.Name != "wechat" {
		t.Errorf("expected wechat, got %s", status.Name)
	}
	if len(status.ActiveUsers) != 2 {
		t.Errorf("expected 2 users, got %d", len(status.ActiveUsers))
	}
}

func TestWSEventSerialization(t *testing.T) {
	ev := WSEvent{
		Type:    "text_delta",
		Content: "hello",
		Tool:    "read",
		CallID:  "tc_123",
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got WSEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if got.Type != "text_delta" {
		t.Errorf("expected text_delta, got %s", got.Type)
	}
	if got.Content != "hello" {
		t.Errorf("expected hello, got %s", got.Content)
	}
	if got.Tool != "read" {
		t.Errorf("expected read, got %s", got.Tool)
	}
}

func TestClientMessageSerialization(t *testing.T) {
	msg := ClientMessage{
		Type:       "approval",
		ApprovalID: "ap_123",
		Approved:   true,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got ClientMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if got.Type != "approval" {
		t.Errorf("expected approval, got %s", got.Type)
	}
	if !got.Approved {
		t.Error("expected approved=true")
	}
}

func TestPlanDataSerialization(t *testing.T) {
	plan := PlanData{
		Title: "Test Plan",
		Steps: []PlanStep{
			{Title: "Step 1", Status: "done"},
			{Title: "Step 2", Status: "running"},
		},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got PlanData
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if got.Title != "Test Plan" {
		t.Errorf("expected Test Plan, got %s", got.Title)
	}
	if len(got.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(got.Steps))
	}
}

func TestGatewayRoutesRegistered(t *testing.T) {
	gw := NewGateway("localhost:8090", "", "0.1.27")

	// Check that HTTP routes are registered (skip /ws which requires Hijack)
	routes := []string{
		"/api/health",
		"/api/status",
		"/api/sessions",
		"/api/memory",
		"/api/platforms",
	}

	for _, route := range routes {
		req := httptest.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()
		gw.mux.ServeHTTP(w, req)
		// We just want to verify the route exists (not 404 from mux)
		if w.Code == http.StatusNotFound && w.Body.String() == "404 page not found\n" {
			t.Errorf("route %s not registered", route)
		}
	}
}
