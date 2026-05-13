package contextfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadContextFiles(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	globalDir := filepath.Join(tmpDir, "global")

	os.MkdirAll(projectDir, 0755)
	os.MkdirAll(globalDir, 0755)

	// Create test files
	os.WriteFile(filepath.Join(projectDir, "AGENTS.md"), []byte("# Project Agent"), 0644)
	os.WriteFile(filepath.Join(globalDir, "AGENTS.md"), []byte("# Global Config"), 0644)

	// Load context files
	result := LoadContextFiles(projectDir, globalDir, nil)

	// Check results
	if len(result.ProjectFiles) != 1 {
		t.Errorf("expected 1 project file, got %d", len(result.ProjectFiles))
	}

	if len(result.GlobalFiles) != 1 {
		t.Errorf("expected 1 global file, got %d", len(result.GlobalFiles))
	}

	// Check file names
	if result.ProjectFiles[0].Name != "AGENTS.md" {
		t.Errorf("expected 'AGENTS.md', got '%s'", result.ProjectFiles[0].Name)
	}
}

func TestBuildContextString(t *testing.T) {
	result := &LoadResult{
		ProjectFiles: []FileContent{
			{Name: "AGENTS.md", Path: "/test/AGENTS.md", Content: "# Test Content"},
		},
	}

	context := BuildContextString(result)

	if context == "" {
		t.Fatal("expected non-empty context string")
	}

	if !contains(context, "AGENTS.md") {
		t.Error("expected context to contain 'AGENTS.md'")
	}

	if !contains(context, "# Test Content") {
		t.Error("expected context to contain file content")
	}
}

func TestBuildContextStringEmpty(t *testing.T) {
	result := &LoadResult{}
	context := BuildContextString(result)

	if context != "" {
		t.Errorf("expected empty context string, got '%s'", context)
	}
}

func TestExtraFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create extra file
	os.WriteFile(filepath.Join(tmpDir, "CUSTOM.md"), []byte("# Custom"), 0644)

	extraFiles := []string{"CUSTOM.md"}
	result := LoadContextFiles(tmpDir, "", extraFiles)

	if len(result.ProjectFiles) != 1 {
		t.Errorf("expected 1 project file, got %d", len(result.ProjectFiles))
	}

	if result.ProjectFiles[0].Name != "CUSTOM.md" {
		t.Errorf("expected 'CUSTOM.md', got '%s'", result.ProjectFiles[0].Name)
	}
}

func TestParentFiles(t *testing.T) {
	// Create nested directory structure
	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "parent")
	childDir := filepath.Join(parentDir, "child")

	os.MkdirAll(childDir, 0755)

	// Create file in parent directory
	os.WriteFile(filepath.Join(parentDir, "AGENTS.md"), []byte("# Parent Config"), 0644)

	// Load from child directory
	result := LoadContextFiles(childDir, "", nil)

	if len(result.ParentFiles) != 1 {
		t.Errorf("expected 1 parent file, got %d", len(result.ParentFiles))
	}

	if result.ParentFiles[0].Name != "AGENTS.md" {
		t.Errorf("expected 'AGENTS.md', got '%s'", result.ParentFiles[0].Name)
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
