package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/startvibecoding/vibecoding/internal/sandbox"
)

func TestNewRegistry(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)

	if r.GetWorkDir() != "/tmp" {
		t.Errorf("expected workdir '/tmp', got '%s'", r.GetWorkDir())
	}
}

func TestRegisterAndGet(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)

	tool := NewReadTool(r)
	r.Register(tool)

	// Get existing tool
	got, ok := r.Get("read")
	if !ok {
		t.Fatal("expected to get 'read' tool")
	}

	if got.Name() != "read" {
		t.Errorf("expected name 'read', got '%s'", got.Name())
	}

	// Get non-existing tool
	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("expected not to get 'nonexistent' tool")
	}
}

func TestRegisterDefaults(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	r.RegisterDefaults()

	expectedTools := []string{"read", "write", "edit", "bash", "jobs", "kill", "grep", "find", "ls"}

	for _, name := range expectedTools {
		_, ok := r.Get(name)
		if !ok {
			t.Errorf("expected to get '%s' tool", name)
		}
	}
}

func TestModeTools(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	r.RegisterDefaults()

	// Plan mode - only read-only tools
	planTools := r.ModeTools("plan")
	planToolNames := make(map[string]bool)
	for _, tool := range planTools {
		planToolNames[tool.Name] = true
	}

	if !planToolNames["read"] {
		t.Error("expected 'read' in plan mode")
	}

	if !planToolNames["grep"] {
		t.Error("expected 'grep' in plan mode")
	}

	if planToolNames["write"] {
		t.Error("expected no 'write' in plan mode")
	}

	if planToolNames["bash"] {
		t.Error("expected no 'bash' in plan mode")
	}

	// Agent mode - all tools
	agentTools := r.ModeTools("agent")
	if len(agentTools) != 9 {
		t.Errorf("expected 9 tools in agent mode, got %d", len(agentTools))
	}
}

func TestReadTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewReadTool(r)

	if tool.Name() != "read" {
		t.Errorf("expected name 'read', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}

	if tool.Parameters() == nil {
		t.Error("expected non-nil parameters")
	}
}

func TestReadToolExecute(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("Hello, World!"), 0644)

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewReadTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "test.txt",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestWriteTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewWriteTool(r)

	if tool.Name() != "write" {
		t.Errorf("expected name 'write', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestWriteToolExecute(t *testing.T) {
	tmpDir := t.TempDir()

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewWriteTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path":    "test.txt",
		"content": "Hello, World!",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}

	// Verify file was written
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(content) != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", string(content))
	}
}

func TestEditTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewEditTool(r)

	if tool.Name() != "edit" {
		t.Errorf("expected name 'edit', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestEditToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("Hello, World!"), 0644)

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewEditTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "test.txt",
		"edits": []any{
			map[string]any{
				"oldText": "World",
				"newText": "Go",
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}

	// Verify edit was applied
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(content) != "Hello, Go!" {
		t.Errorf("expected 'Hello, Go!', got '%s'", string(content))
	}
}

func TestBashTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewBashTool(r)

	if tool.Name() != "bash" {
		t.Errorf("expected name 'bash', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestBashToolExecute(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewBashTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBashToolAsync(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewBashTool(r)

	// Start async command
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "sleep 1",
		"async":  true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}

	// Check job was created
	jm := tool.GetJobManager()
	jobs := jm.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].ID != 1 {
		t.Errorf("expected job ID 1, got %d", jobs[0].ID)
	}

	// Wait for job to finish
	time.Sleep(2 * time.Second)

	if !jobs[0].IsDone() {
		t.Error("expected job to be done")
	}
}

func TestJobsTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	bashTool := NewBashTool(r)
	jobsTool := NewJobsTool(r, bashTool)

	if jobsTool.Name() != "jobs" {
		t.Errorf("expected name 'jobs', got '%s'", jobsTool.Name())
	}

	// List jobs - should be empty
	result, err := jobsTool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "No background jobs." {
		t.Errorf("expected 'No background jobs.', got '%s'", result)
	}
}

func TestKillTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	bashTool := NewBashTool(r)
	killTool := NewKillTool(r, bashTool)

	if killTool.Name() != "kill" {
		t.Errorf("expected name 'kill', got '%s'", killTool.Name())
	}

	// Try to kill non-existent job
	_, err := killTool.Execute(context.Background(), map[string]any{
		"jobId": float64(999),
	})
	if err == nil {
		t.Error("expected error for non-existent job")
	}
}

func TestGrepTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewGrepTool(r)

	if tool.Name() != "grep" {
		t.Errorf("expected name 'grep', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestGrepToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("Hello, World!\nFoo bar\nHello again"), 0644)

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewGrepTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "Hello",
		"path":    ".",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestFindTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewFindTool(r)

	if tool.Name() != "find" {
		t.Errorf("expected name 'find', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestFindToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("Hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main"), 0644)

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewFindTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    ".",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestLsTool(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewLsTool(r)

	if tool.Name() != "ls" {
		t.Errorf("expected name 'ls', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestLsToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("Hello"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewLsTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": ".",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestToolDefinition(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewReadTool(r)

	def := ToolDefinition(tool)

	if def.Name != "read" {
		t.Errorf("expected name 'read', got '%s'", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	if def.Parameters == nil {
		t.Error("expected non-nil parameters")
	}
}

func TestDefinitions(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	r.RegisterDefaults()

	defs := r.Definitions()

	if len(defs) != 9 {
		t.Errorf("expected 9 definitions, got %d", len(defs))
	}
}

func TestAll(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	r.RegisterDefaults()

	all := r.All()

	if len(all) != 9 {
		t.Errorf("expected 9 tools, got %d", len(all))
	}
}
