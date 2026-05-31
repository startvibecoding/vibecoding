package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/vendored"
)

func TestMain(m *testing.M) {
	// 提取 vendored 二进制到 ~/.vibecoding/bin/
	if err := vendored.Ensure(); err != nil {
		// 如果提取失败，跳过需要 rg/fd 的测试
		os.Exit(m.Run())
	}
	os.Exit(m.Run())
}

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

	expectedTools := []string{"read", "write", "edit", "bash", "jobs", "kill", "grep", "find", "ls", "plan"}

	for _, name := range expectedTools {
		_, ok := r.Get(name)
		if !ok {
			t.Errorf("expected to get '%s' tool", name)
		}
	}
}

func TestRegisterDefaultsWithPlanToolDisabled(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	r.RegisterDefaultsWithPlanTool(false)

	if _, ok := r.Get("plan"); ok {
		t.Fatal("expected plan tool to be disabled")
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
	if !planToolNames["plan"] {
		t.Error("expected 'plan' in plan mode")
	}

	if planToolNames["bash"] {
		t.Error("expected no 'bash' in plan mode")
	}

	// Agent mode - all tools
	agentTools := r.ModeTools("agent")
	if len(agentTools) != 10 {
		t.Errorf("expected 10 tools in agent mode, got %d", len(agentTools))
	}
}

func TestPlanToolExecute(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewPlanTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"title": "Ship feature",
		"steps": []any{
			map[string]any{"title": "Read code", "status": "done"},
			map[string]any{"title": "Implement change", "status": "running"},
		},
		"note": "Keep scope small",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected structured plan")
	}
	if result.Plan.Title != "Ship feature" {
		t.Fatalf("plan title = %q, want Ship feature", result.Plan.Title)
	}
	if len(result.Plan.Steps) != 2 || result.Plan.Steps[1].Status != "running" {
		t.Fatalf("plan steps = %#v", result.Plan.Steps)
	}
	if !strings.Contains(result.Text, "[running] Implement change") {
		t.Fatalf("expected formatted plan text, got: %s", result.Text)
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

	if result.Text == "" {
		t.Error("expected non-empty result")
	}
}

func TestReadToolImage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal valid PNG (1x1 pixel, red)
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // 8-bit RGB
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, // IEND chunk
		0x44, 0xae, 0x42, 0x60, 0x82,
	}

	tmpFile := filepath.Join(tmpDir, "test.png")
	os.WriteFile(tmpFile, pngData, 0644)

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewReadTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "test.png",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have text description
	if result.Text == "" {
		t.Error("expected non-empty text result")
	}
	if !strings.Contains(result.Text, "Image file") {
		t.Errorf("expected 'Image file' in text, got '%s'", result.Text)
	}

	// Should have rich contents with image block
	if len(result.Contents) != 2 {
		t.Fatalf("expected 2 content blocks (text + image), got %d", len(result.Contents))
	}
	if result.Contents[0].Type != "text" {
		t.Errorf("expected first block type 'text', got '%s'", result.Contents[0].Type)
	}
	if result.Contents[1].Type != "image" {
		t.Errorf("expected second block type 'image', got '%s'", result.Contents[1].Type)
	}
	if result.Contents[1].Image == nil {
		t.Fatal("expected non-nil image content")
	}
	if result.Contents[1].Image.MimeType != "image/png" {
		t.Errorf("expected mime type 'image/png', got '%s'", result.Contents[1].Image.MimeType)
	}
	if result.Contents[1].Image.Data == "" {
		t.Error("expected non-empty base64 data")
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

	if result.Text == "" {
		t.Error("expected non-empty result")
	}
	if result.Diff == nil {
		t.Fatal("expected structured diff")
	}
	if result.Diff.Added != 1 || result.Diff.Deleted != 0 {
		t.Fatalf("diff = +%d -%d, want +1 -0", result.Diff.Added, result.Diff.Deleted)
	}
	if !strings.Contains(result.Diff.Unified, "+Hello, World!") {
		t.Fatalf("expected unified diff to include added content, got: %s", result.Diff.Unified)
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

	if result.Text == "" {
		t.Error("expected non-empty result")
	}
	if result.Diff == nil {
		t.Fatal("expected structured diff")
	}
	if result.Diff.Added != 1 || result.Diff.Deleted != 1 {
		t.Fatalf("diff = +%d -%d, want +1 -1", result.Diff.Added, result.Diff.Deleted)
	}
	if !strings.Contains(result.Text, "Diff: +1 -1") {
		t.Fatalf("expected diff summary in result text, got: %s", result.Text)
	}
	if !strings.Contains(result.Diff.Unified, "-Hello, World!") || !strings.Contains(result.Diff.Unified, "+Hello, Go!") {
		t.Fatalf("expected unified diff replacement, got: %s", result.Diff.Unified)
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

	if result.Text == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result.Text, "[command]\necho hello") {
		t.Fatalf("expected command section, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[stdout]\nhello") {
		t.Fatalf("expected stdout section with command output, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[stderr]\n(no output)") {
		t.Fatalf("expected empty stderr section, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[exit_code]\n0") {
		t.Fatalf("expected zero exit code, got: %s", result.Text)
	}
}

func TestBashToolExecuteStderrOnly(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewBashTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo problem >&2",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Text, "[stdout]\n(no output)") {
		t.Fatalf("expected empty stdout section, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[stderr]\nproblem") {
		t.Fatalf("expected stderr section with output, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[exit_code]\n0") {
		t.Fatalf("expected zero exit code, got: %s", result.Text)
	}
}

func TestBashToolExecuteNoOutput(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewBashTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "true",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Text, "[stdout]\n(no output)") {
		t.Fatalf("expected empty stdout section, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[stderr]\n(no output)") {
		t.Fatalf("expected empty stderr section, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[exit_code]\n0") {
		t.Fatalf("expected zero exit code, got: %s", result.Text)
	}
}

func TestBashToolExecuteNonZeroExitCode(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewBashTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo boom >&2; exit 7",
	})

	if err != nil {
		t.Fatalf("expected non-zero exit to be returned as tool output, got error: %v", err)
	}
	if !strings.Contains(result.Text, "[stderr]\nboom") {
		t.Fatalf("expected stderr section with output, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[exit_code]\n7") {
		t.Fatalf("expected exit code 7, got: %s", result.Text)
	}
}

func TestBashToolAsync(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	tool := NewBashTool(r)

	// Start async command
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "sleep 1",
		"async":   true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text == "" {
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

	if result.Text != "No background jobs." {
		t.Errorf("expected 'No background jobs.', got '%s'", result.Text)
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

	if result.Text == "" {
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

	if result.Text == "" {
		t.Error("expected non-empty result")
	}
}

func TestFindToolExecuteUsesNativeGlob(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "test.txt"), []byte("Hello"), 0644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("Hello"), 0644); err != nil {
		t.Fatalf("write root file: %v", err)
	}

	sb := sandbox.NewNoneSandbox()
	r := NewRegistry(tmpDir, sb)
	tool := NewFindTool(r)

	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern":  "**/*.txt",
		"path":     ".",
		"maxDepth": float64(2),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Text, filepath.Join("nested", "test.txt")) {
		t.Fatalf("result = %q, want nested/test.txt", result.Text)
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

	if result.Text == "" {
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

	if len(defs) != 10 {
		t.Errorf("expected 10 definitions, got %d", len(defs))
	}
}

func TestAll(t *testing.T) {
	sb := sandbox.NewNoneSandbox()
	r := NewRegistry("/tmp", sb)
	r.RegisterDefaults()

	all := r.All()

	if len(all) != 10 {
		t.Errorf("expected 10 tools, got %d", len(all))
	}
}
