package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRouter(t *testing.T) {
	routes := []RouteConfig{
		{Path: "/github", Events: []string{"push", "pull_request"}, Skill: "code-review", Delivery: "wechat"},
	}
	handler := &mockHandler{}
	router := NewRouter(routes, "secret123", handler)

	if router == nil {
		t.Fatal("expected router")
	}
}

func TestRouterServeHTTPNoRoute(t *testing.T) {
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{}, "", handler)

	req := httptest.NewRequest("POST", "/webhook/unknown", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestRouterServeHTTPMethodNotAllowed(t *testing.T) {
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/github", Events: []string{"push"}},
	}, "", handler)

	req := httptest.NewRequest("GET", "/webhook/github", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestRouterServeHTTPMatchRoute(t *testing.T) {
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/github", Events: []string{"push", "pull_request"}},
	}, "", handler)

	body := `{"action": "push"}`
	req := httptest.NewRequest("POST", "/webhook/github", bytes.NewReader([]byte(body)))
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	time.Sleep(100 * time.Millisecond)
	if !handler.called {
		t.Error("expected handler to be called")
	}
}

func TestRouterServeHTTPEventFilter(t *testing.T) {
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/github", Events: []string{"push"}},
	}, "", handler)

	body := `{"action": "issues"}`
	req := httptest.NewRequest("POST", "/webhook/github", bytes.NewReader([]byte(body)))
	req.Header.Set("X-GitHub-Event", "issues")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if handler.called {
		t.Error("expected handler NOT to be called (event filtered)")
	}
}

func TestRouterServeHTTPWildcardEvent(t *testing.T) {
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/ci", Events: []string{"*"}},
	}, "", handler)

	body := `{"type": "build"}`
	req := httptest.NewRequest("POST", "/webhook/ci", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	time.Sleep(100 * time.Millisecond)
	if !handler.called {
		t.Error("expected handler to be called (wildcard)")
	}
}

func TestRouterSignatureVerification(t *testing.T) {
	secret := "test-secret"
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/github", Events: []string{"*"}},
	}, secret, handler)

	body := []byte(`{"action": "push"}`)

	// Compute correct signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	time.Sleep(100 * time.Millisecond)
	if !handler.called {
		t.Error("expected handler to be called with valid signature")
	}
}

func TestRouterSignatureVerificationInvalid(t *testing.T) {
	secret := "test-secret"
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/github", Events: []string{"*"}},
	}, secret, handler)

	body := []byte(`{"action": "push"}`)

	req := httptest.NewRequest("POST", "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if handler.called {
		t.Error("expected handler NOT to be called with invalid signature")
	}
}

func TestRouterSignatureVerificationMissing(t *testing.T) {
	secret := "test-secret"
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/github", Events: []string{"*"}},
	}, secret, handler)

	body := []byte(`{"action": "push"}`)

	req := httptest.NewRequest("POST", "/webhook/github", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRouterNoSecret(t *testing.T) {
	handler := &mockHandler{}
	router := NewRouter([]RouteConfig{
		{Path: "/github", Events: []string{"*"}},
	}, "", handler)

	body := []byte(`{"action": "push"}`)

	req := httptest.NewRequest("POST", "/webhook/github", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	time.Sleep(100 * time.Millisecond)
	if !handler.called {
		t.Error("expected handler to be called (no secret)")
	}
}

func TestVerifySignature(t *testing.T) {
	router := &Router{secret: "test"}

	body := []byte("hello")
	mac := hmac.New(sha256.New, []byte("test"))
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !router.verifySignature(body, validSig) {
		t.Error("expected valid signature")
	}

	if router.verifySignature(body, "sha256=invalid") {
		t.Error("expected invalid signature")
	}

	if router.verifySignature(body, "") {
		t.Error("expected empty signature to fail")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected application/json, got %s", contentType)
	}

	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("expected ok, got %s", result["status"])
	}
}

type mockHandler struct {
	called    bool
	lastRoute RouteConfig
}

func (h *mockHandler) HandleWebhookEvent(ctx context.Context, route RouteConfig, payload []byte) error {
	h.called = true
	h.lastRoute = route
	return nil
}
