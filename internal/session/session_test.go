package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fuckvibecoding/vibecoding/internal/provider"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	if m.cwd != "/tmp/test" {
		t.Errorf("expected cwd '/tmp/test', got '%s'", m.cwd)
	}

	if m.sessionDir != sessionDir {
		t.Errorf("expected sessionDir '%s', got '%s'", sessionDir, m.sessionDir)
	}
}

func TestNewDefaultDir(t *testing.T) {
	m := New("/tmp/test", "")

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	if m.sessionDir == "" {
		t.Error("expected non-empty default session dir")
	}
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)

	if err := m.Init(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.header == nil {
		t.Fatal("expected non-nil header")
	}

	if m.header.Version != CurrentVersion {
		t.Errorf("expected version %d, got %d", CurrentVersion, m.header.Version)
	}

	if m.header.Cwd != "/tmp/test" {
		t.Errorf("expected cwd '/tmp/test', got '%s'", m.header.Cwd)
	}

	if m.header.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Check file was created
	if _, err := os.Stat(m.file); os.IsNotExist(err) {
		t.Error("expected session file to exist")
	}
}

func TestAppendMessage(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	msg := provider.NewUserMessage("Hello")
	id, err := m.AppendMessage(msg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty ID")
	}

	if len(m.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m.entries))
	}

	// Append another message
	msg2 := provider.NewAssistantMessage([]provider.ContentBlock{
		{Type: "text", Text: "Hi there"},
	})
	id2, err := m.AppendMessage(msg2)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id2 == "" {
		t.Error("expected non-empty ID")
	}

	if len(m.entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m.entries))
	}
}

func TestAppendModelChange(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	id, err := m.AppendModelChange("anthropic", "claude-sonnet-4-20250514")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty ID")
	}

	if len(m.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m.entries))
	}
}

func TestAppendThinkingLevelChange(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	id, err := m.AppendThinkingLevelChange("high")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty ID")
	}

	if len(m.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m.entries))
	}
}

func TestAppendCompaction(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	id, err := m.AppendCompaction("Compacted 10 messages", "entry-1", 1000)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty ID")
	}

	if len(m.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m.entries))
	}
}

func TestGetHeader(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	header := m.GetHeader()

	if header == nil {
		t.Fatal("expected non-nil header")
	}

	if header.Cwd != "/tmp/test" {
		t.Errorf("expected cwd '/tmp/test', got '%s'", header.Cwd)
	}
}

func TestGetLeafID(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	// Initially nil
	leafID := m.GetLeafID()
	if leafID != nil {
		t.Error("expected nil leaf ID initially")
	}

	// After append
	m.AppendMessage(provider.NewUserMessage("Hello"))
	leafID = m.GetLeafID()
	if leafID == nil {
		t.Error("expected non-nil leaf ID after append")
	}
}

func TestGetFile(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	file := m.GetFile()
	if file == "" {
		t.Error("expected non-empty file path")
	}
}

func TestGetMessages(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	m.AppendMessage(provider.NewUserMessage("Hello"))
	m.AppendMessage(provider.NewAssistantMessage([]provider.ContentBlock{
		{Type: "text", Text: "Hi"},
	}))

	messages := m.GetMessages()
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	// Create a session
	m1 := New("/tmp/test", sessionDir)
	m1.Init()
	m1.AppendMessage(provider.NewUserMessage("Hello"))

	// Open the session
	m2, err := Open(m1.file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m2 == nil {
		t.Fatal("expected non-nil manager")
	}

	if m2.header.Cwd != "/tmp/test" {
		t.Errorf("expected cwd '/tmp/test', got '%s'", m2.header.Cwd)
	}

	if len(m2.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m2.entries))
	}
}

func TestOpenNonExistent(t *testing.T) {
	_, err := Open("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestListForDir(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	// Create sessions
	m1 := New("/tmp/test1", sessionDir)
	m1.Init()

	time.Sleep(10 * time.Millisecond)

	m2 := New("/tmp/test1", sessionDir)
	m2.Init()

	m3 := New("/tmp/test2", sessionDir)
	m3.Init()

	// List for test1
	sessions, err := ListForDir("/tmp/test1", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// List for test2
	sessions, err = ListForDir("/tmp/test2", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}

	// List for non-existent
	sessions, err = ListForDir("/tmp/nonexistent", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestContinueRecent(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	// Create a session
	m1 := New("/tmp/test", sessionDir)
	m1.Init()

	// Continue recent
	m2, err := ContinueRecent("/tmp/test", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m2 == nil {
		t.Fatal("expected non-nil manager")
	}

	if m2.file != m1.file {
		t.Errorf("expected file '%s', got '%s'", m1.file, m2.file)
	}
}

func TestContinueRecentNew(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	// Continue recent for non-existing dir
	m, err := ContinueRecent("/tmp/nonexistent", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	// Should be a new session (no file)
	if m.file != "" {
		t.Errorf("expected empty file for new session, got '%s'", m.file)
	}
}

func TestContinueRecentDefaultDir(t *testing.T) {
	// Test with empty session dir (should use default)
	m, err := ContinueRecent("/tmp/test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("expected non-empty ID")
	}

	if id2 == "" {
		t.Error("expected non-empty ID")
	}

	if id1 == id2 {
		t.Error("expected unique IDs")
	}
}

func TestSessionInfo(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	// Create sessions
	m1 := New("/tmp/test", sessionDir)
	m1.Init()

	time.Sleep(10 * time.Millisecond)

	m2 := New("/tmp/test", sessionDir)
	m2.Init()

	// List and check info
	sessions, _ := ListForDir("/tmp/test", sessionDir)

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Check that sessions have required fields
	for _, s := range sessions {
		if s.Path == "" {
			t.Error("expected non-empty path")
		}
		if s.ModTime.IsZero() {
			t.Error("expected non-zero mod time")
		}
	}
}
