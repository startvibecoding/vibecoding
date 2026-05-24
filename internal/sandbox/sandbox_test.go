package sandbox

import (
	"context"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelStrict, "strict"},
		{LevelStandard, "standard"},
		{LevelNone, "none"},
		{Level(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.level.String()
		if result != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, result)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		hasError bool
	}{
		{"strict", LevelStrict, false},
		{"standard", LevelStandard, false},
		{"none", LevelNone, false},
		{"invalid", LevelNone, true},
	}

	for _, tt := range tests {
		result, err := ParseLevel(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error for input '%s'", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for input '%s': %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		}
	}
}

func TestNewNoneSandbox(t *testing.T) {
	sb := NewNoneSandbox()

	if sb == nil {
		t.Fatal("expected non-nil sandbox")
	}

	if sb.Name() != "none" {
		t.Errorf("expected name 'none', got '%s'", sb.Name())
	}

	if sb.Level() != LevelNone {
		t.Errorf("expected level %d, got %d", LevelNone, sb.Level())
	}

	if !sb.IsAvailable() {
		t.Error("expected none sandbox to be available")
	}
}

func TestNoneSandboxWrapCommand(t *testing.T) {
	sb := NewNoneSandbox()

	cmd := sb.WrapCommand(context.Background(), "/bin/bash", "echo hello", ExecOpts{
		WorkDir: "/tmp",
	})

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
}

func TestNoneSandboxWrapCommandUsesPlatformShellArgs(t *testing.T) {
	sb := NewNoneSandbox()

	cmd := sb.WrapCommand(context.Background(), "cmd.exe", "echo hello", ExecOpts{})
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "/c" || cmd.Args[2] != "echo hello" {
		t.Fatalf("expected cmd.exe arguments to use /c, got %#v", cmd.Args)
	}

	cmd = sb.WrapCommand(context.Background(), "PowerShell.exe", "echo hello", ExecOpts{})
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if len(cmd.Args) != 5 ||
		cmd.Args[1] != "-NoProfile" ||
		cmd.Args[2] != "-NonInteractive" ||
		cmd.Args[3] != "-Command" ||
		cmd.Args[4] != "echo hello" {
		t.Fatalf("expected PowerShell arguments, got %#v", cmd.Args)
	}
}

func TestNewBwrapSandbox(t *testing.T) {
	sb := NewBwrapSandbox("/tmp", LevelStandard)

	if sb == nil {
		t.Fatal("expected non-nil sandbox")
	}

	if sb.Name() != "bwrap" {
		t.Errorf("expected name 'bwrap', got '%s'", sb.Name())
	}

	if sb.Level() != LevelStandard {
		t.Errorf("expected level %d, got %d", LevelStandard, sb.Level())
	}
}

func TestBwrapSandboxName(t *testing.T) {
	sb := NewBwrapSandbox("/tmp", LevelStrict)

	if sb.Name() != "bwrap" {
		t.Errorf("expected name 'bwrap', got '%s'", sb.Name())
	}
}

func TestBwrapSandboxLevel(t *testing.T) {
	sb := NewBwrapSandbox("/tmp", LevelStrict)

	if sb.Level() != LevelStrict {
		t.Errorf("expected level %d, got %d", LevelStrict, sb.Level())
	}

	sb = NewBwrapSandbox("/tmp", LevelStandard)
	if sb.Level() != LevelStandard {
		t.Errorf("expected level %d, got %d", LevelStandard, sb.Level())
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp")

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	// Should have 3 sandboxes
	if len(m.sandboxes) != 3 {
		t.Errorf("expected 3 sandboxes, got %d", len(m.sandboxes))
	}
}

func TestManagerSetLevel(t *testing.T) {
	m := NewManager("/tmp")

	// Set to none (should always work)
	err := m.SetLevel(LevelNone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sb := m.GetActive()
	if sb.Level() != LevelNone {
		t.Errorf("expected level %d, got %d", LevelNone, sb.Level())
	}
}

func TestManagerGetActive(t *testing.T) {
	m := NewManager("/tmp")

	// Default should be none
	sb := m.GetActive()
	if sb == nil {
		t.Fatal("expected non-nil sandbox")
	}

	if sb.Level() != LevelNone {
		t.Errorf("expected level %d, got %d", LevelNone, sb.Level())
	}
}

func TestManagerGetForLevel(t *testing.T) {
	m := NewManager("/tmp")

	// Get none sandbox
	sb, err := m.GetForLevel(LevelNone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb == nil {
		t.Fatal("expected non-nil sandbox")
	}

	if sb.Level() != LevelNone {
		t.Errorf("expected level %d, got %d", LevelNone, sb.Level())
	}
}

func TestFormatSandboxInfo(t *testing.T) {
	sb := NewNoneSandbox()
	info := FormatSandboxInfo(sb)

	if info == "" {
		t.Error("expected non-empty info")
	}

	if !contains(info, "No sandbox") {
		t.Error("expected info to contain 'No sandbox'")
	}
}

func TestFormatSandboxInfoNil(t *testing.T) {
	info := FormatSandboxInfo(nil)

	if info == "" {
		t.Error("expected non-empty info")
	}

	if !contains(info, "No sandbox") {
		t.Error("expected info to contain 'No sandbox'")
	}
}

func TestExecOpts(t *testing.T) {
	opts := ExecOpts{
		WritablePaths: []string{"/tmp"},
		ReadOnlyPaths: []string{"/usr"},
		NetworkAccess: true,
		EnvVars:       map[string]string{"FOO": "bar"},
		WorkDir:       "/tmp",
		Timeout:       30,
	}

	if len(opts.WritablePaths) != 1 {
		t.Errorf("expected 1 writable path, got %d", len(opts.WritablePaths))
	}

	if !opts.NetworkAccess {
		t.Error("expected network access to be true")
	}

	if opts.EnvVars["FOO"] != "bar" {
		t.Errorf("expected 'bar', got '%s'", opts.EnvVars["FOO"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
