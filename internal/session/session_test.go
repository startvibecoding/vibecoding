package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/startvibecoding/vibecoding/internal/provider"
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

func TestAppendMessageAutoInitializesSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	id, err := m.AppendMessage(provider.NewUserMessage("Hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty message ID")
	}
	if m.GetHeader() == nil {
		t.Fatal("expected session header to be initialized")
	}
	if m.GetFile() == "" {
		t.Fatal("expected session file to be initialized")
	}
	if _, err := os.Stat(m.GetFile()); err != nil {
		t.Fatalf("expected session file to exist: %v", err)
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

	if m.file == "" {
		t.Fatal("expected new session file")
	}
	if m.header == nil {
		t.Fatal("expected new session header")
	}
	if _, err := os.Stat(m.file); err != nil {
		t.Fatalf("expected session file to exist: %v", err)
	}
	if _, err := m.AppendMessage(provider.NewUserMessage("Hello")); err != nil {
		t.Fatalf("append message to new continued session: %v", err)
	}
}

func TestContinueRecentDefaultDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Test with empty session dir (should use default)
	m, err := ContinueRecent("/tmp/test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestOpenByPathOrID(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m1 := New("/tmp/test", sessionDir)
	if err := m1.InitWithID("session-test-id"); err != nil {
		t.Fatalf("init session: %v", err)
	}

	byPath, err := OpenByPathOrID("/tmp/test", sessionDir, m1.file)
	if err != nil {
		t.Fatalf("open by path: %v", err)
	}
	if byPath.file != m1.file {
		t.Errorf("expected file %q, got %q", m1.file, byPath.file)
	}

	byID, err := OpenByPathOrID("/tmp/test", sessionDir, "session-test-id")
	if err != nil {
		t.Fatalf("open by id: %v", err)
	}
	if byID.file != m1.file {
		t.Errorf("expected file %q, got %q", m1.file, byID.file)
	}

	shortID := sessionFileID(m1.file)
	byShortID, err := OpenByPathOrID("/tmp/test", sessionDir, shortID)
	if err != nil {
		t.Fatalf("open by short id: %v", err)
	}
	if byShortID.file != m1.file {
		t.Errorf("expected file %q, got %q", m1.file, byShortID.file)
	}
}

func TestOpenByPathOrIDAmbiguousPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	ids := []string{"abcdef01", "abcdef02"}
	for _, id := range ids {
		m := New("/tmp/test", sessionDir)
		if err := m.InitWithID(id); err != nil {
			t.Fatalf("init session %s: %v", id, err)
		}
	}

	_, err := OpenByPathOrID("/tmp/test", sessionDir, "abc")
	if err == nil {
		t.Fatal("expected ambiguous prefix error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %q, want ambiguous", err)
	}
}

func TestLoadRejectsCorruptSessionLine(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.jsonl")
	data := fmt.Sprintf(
		"{\"type\":\"%s\",\"version\":%d,\"id\":\"session-id\",\"timestamp\":\"%s\",\"cwd\":\"/tmp/test\"}\nnot-json\n",
		EntrySession,
		CurrentVersion,
		time.Now().Format(time.RFC3339Nano),
	)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write session: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Fatal("expected corrupt session error")
	}
	if !strings.Contains(err.Error(), "corrupt line") {
		t.Fatalf("err = %q, want corrupt line", err)
	}
}

func TestAppendEntriesMaintainParentChain(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	if err := m.Init(); err != nil {
		t.Fatalf("init session: %v", err)
	}

	firstID, err := m.AppendMessage(provider.NewUserMessage("first"))
	if err != nil {
		t.Fatalf("append first: %v", err)
	}
	secondID, err := m.AppendModelChange("openai", "model")
	if err != nil {
		t.Fatalf("append second: %v", err)
	}

	if len(m.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(m.entries))
	}
	second, ok := m.entries[1].(ModelChangeEntry)
	if !ok {
		t.Fatalf("entry type = %T, want ModelChangeEntry", m.entries[1])
	}
	if second.ParentID == nil || *second.ParentID != firstID {
		t.Fatalf("second parent = %#v, want %s", second.ParentID, firstID)
	}
	if leaf := m.GetLeafID(); leaf == nil || *leaf != secondID {
		t.Fatalf("leaf = %#v, want %s", leaf, secondID)
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

func TestDeleteSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	path := m.GetFile()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file should exist: %v", err)
	}

	err := DeleteSession(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected session file to be deleted")
	}
}

func TestDeleteSessionNonExistent(t *testing.T) {
	err := DeleteSession("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestListForDirDetailed(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	// Create a session with messages
	m := New("/tmp/test", sessionDir)
	m.Init()
	m.AppendMessage(provider.NewUserMessage("Hello world"))
	m.AppendMessage(provider.NewAssistantMessage([]provider.ContentBlock{
		{Type: "text", Text: "Hi there"},
	}))
	m.AppendMessage(provider.NewUserMessage("Another message"))

	details, err := ListForDirDetailed("/tmp/test", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(details) != 1 {
		t.Fatalf("expected 1 session detail, got %d", len(details))
	}

	d := details[0]
	if d.MessageCount != 3 {
		t.Errorf("expected 3 messages, got %d", d.MessageCount)
	}
	if d.Preview != "Hello world" {
		t.Errorf("expected preview 'Hello world', got %q", d.Preview)
	}
	if d.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestListForDirDetailedLongPreview(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()
	// Message longer than 60 chars
	longMsg := strings.Repeat("a", 100)
	m.AppendMessage(provider.NewUserMessage(longMsg))

	details, err := ListForDirDetailed("/tmp/test", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(details) != 1 {
		t.Fatalf("expected 1 session, got %d", len(details))
	}

	if len(details[0].Preview) > 64 { // 60 + "..."
		t.Errorf("preview should be truncated, got length %d", len(details[0].Preview))
	}
	if !strings.HasSuffix(details[0].Preview, "...") {
		t.Error("expected truncated preview to end with '...'")
	}
}

func TestListForDirDetailedEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	details, err := ListForDirDetailed("/tmp/nonexistent", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(details) != 0 {
		t.Errorf("expected 0 details, got %d", len(details))
	}
}

func TestListForDirDetailedContentBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()
	// User message with content blocks (no Content field)
	m.AppendMessage(provider.Message{
		Role: "user",
		Contents: []provider.ContentBlock{
			{Type: "text", Text: "Block content"},
		},
	})

	details, err := ListForDirDetailed("/tmp/test", sessionDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(details) != 1 {
		t.Fatalf("expected 1 session, got %d", len(details))
	}
	if details[0].Preview != "Block content" {
		t.Errorf("expected preview 'Block content', got %q", details[0].Preview)
	}
}

func TestAppendSessionInfo(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	m.Init()

	id, err := m.AppendSessionInfo("My Session")
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

func TestEncodePath(t *testing.T) {
	// Same path should produce same encoding
	e1 := encodePath("/tmp/test")
	e2 := encodePath("/tmp/test")
	if e1 != e2 {
		t.Error("expected same encoding for same path")
	}

	// Different paths should produce different encodings
	e3 := encodePath("/tmp/test2")
	if e1 == e3 {
		t.Error("expected different encoding for different path")
	}

	// Paths that are similar but different should not collide
	e4 := encodePath("/tmp/test-1")
	e5 := encodePath("/tmp/test:1")
	if e4 == e5 {
		t.Error("expected different encoding for paths with different special chars")
	}
}

func TestInitWithID(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	m := New("/tmp/test", sessionDir)
	err := m.InitWithID("custom-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	header := m.GetHeader()
	if header.ID != "custom-id" {
		t.Errorf("expected ID 'custom-id', got %q", header.ID)
	}
}

func TestSessionFileID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/20240101-120000_abcd1234.jsonl", "abcd1234"},
		{"/path/to/session.jsonl", ""},
		{"simple_id.jsonl", "id"},
	}

	for _, tt := range tests {
		result := sessionFileID(tt.path)
		if result != tt.expected {
			t.Errorf("sessionFileID(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestOpenByPathOrIDEmptyValue(t *testing.T) {
	_, err := OpenByPathOrID("/tmp", "/tmp/sessions", "")
	if err == nil {
		t.Error("expected error for empty value")
	}
}

func TestSessionRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")

	// Create session with various entry types
	m1 := New("/tmp/test", sessionDir)
	m1.Init()
	m1.AppendMessage(provider.NewUserMessage("Hello"))
	m1.AppendMessage(provider.NewAssistantMessage([]provider.ContentBlock{
		{Type: "text", Text: "Hi"},
	}))
	m1.AppendModelChange("anthropic", "claude-sonnet-4-20250514")
	m1.AppendThinkingLevelChange("high")
	m1.AppendCompaction("Summary", "", 1000)
	m1.AppendSessionInfo("Test Session")

	// Re-open and verify all entries loaded
	m2, err := Open(m1.GetFile())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m2.entries) != 6 {
		t.Errorf("expected 6 entries, got %d", len(m2.entries))
	}

	msgs := m2.GetMessages()
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}
