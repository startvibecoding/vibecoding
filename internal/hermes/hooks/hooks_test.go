package hooks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("", "")
	if m.HasPreHook() {
		t.Error("expected no pre hook")
	}
	if m.HasPostHook() {
		t.Error("expected no post hook")
	}

	m2 := NewManager("/path/pre", "/path/post")
	if !m2.HasPreHook() {
		t.Error("expected pre hook")
	}
	if !m2.HasPostHook() {
		t.Error("expected post hook")
	}
}

func TestPreToolCallNoHook(t *testing.T) {
	m := NewManager("", "")
	allowed, reason, err := m.PreToolCall(context.Background(), "bash", map[string]any{"command": "ls"}, "ws", "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed when no hook")
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %s", reason)
	}
}

func TestPreToolCallAllow(t *testing.T) {
	script := createTestScript(t, `#!/bin/sh
echo '{"action": "allow"}'
`)
	defer os.Remove(script)

	m := NewManager(script, "")
	allowed, reason, err := m.PreToolCall(context.Background(), "bash", map[string]any{"command": "ls"}, "ws", "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed")
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %s", reason)
	}
}

func TestPreToolCallBlock(t *testing.T) {
	script := createTestScript(t, `#!/bin/sh
echo '{"action": "block", "reason": "destructive command"}'
`)
	defer os.Remove(script)

	m := NewManager(script, "")
	allowed, reason, err := m.PreToolCall(context.Background(), "bash", map[string]any{"command": "rm -rf /"}, "ws", "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected blocked")
	}
	if reason != "destructive command" {
		t.Errorf("expected 'destructive command', got %s", reason)
	}
}

func TestPreToolCallScriptNotFound(t *testing.T) {
	m := NewManager("/nonexistent/script", "")
	allowed, _, err := m.PreToolCall(context.Background(), "bash", map[string]any{}, "ws", "user1")
	if err == nil {
		t.Error("expected error for missing script")
	}
	// Fail-open: should allow even on error
	if !allowed {
		t.Error("expected fail-open (allowed)")
	}
}

func TestPreToolCallInvalidJSON(t *testing.T) {
	script := createTestScript(t, `#!/bin/sh
echo 'not json'
`)
	defer os.Remove(script)

	m := NewManager(script, "")
	allowed, _, err := m.PreToolCall(context.Background(), "bash", map[string]any{}, "ws", "user1")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	// Fail-open
	if !allowed {
		t.Error("expected fail-open (allowed)")
	}
}

func TestPostToolCallNoHook(t *testing.T) {
	m := NewManager("", "")
	// Should not panic
	m.PostToolCall(context.Background(), "bash", map[string]any{}, "result", "", "ws", "user1")
}

func TestPostToolCallWithHook(t *testing.T) {
	script := createTestScript(t, `#!/bin/sh
# Read stdin and log it
cat > /dev/null
echo "logged"
`)
	defer os.Remove(script)

	m := NewManager("", script)
	// Should not panic
	m.PostToolCall(context.Background(), "bash", map[string]any{"command": "ls"}, "result", "", "ws", "user1")
}

func createTestScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		t.Fatalf("create script: %v", err)
	}
	return path
}
