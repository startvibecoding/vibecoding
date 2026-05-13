package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/global", "/project")

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	if m.globalDir != "/global" {
		t.Errorf("expected globalDir '/global', got '%s'", m.globalDir)
	}

	if m.projectDir != "/project" {
		t.Errorf("expected projectDir '/project', got '%s'", m.projectDir)
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")

	// Create global skill
	globalSkillDir := filepath.Join(globalDir, "test-skill")
	os.MkdirAll(globalSkillDir, 0755)
	os.WriteFile(filepath.Join(globalSkillDir, "SKILL.md"), []byte("# Test Skill\nThis is a test skill."), 0644)

	// Create project skill
	projectSkillDir := filepath.Join(projectDir, "project-skill")
	os.MkdirAll(projectSkillDir, 0755)
	os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte("# Project Skill\nProject specific skill."), 0644)

	m := NewManager(globalDir, projectDir)
	err := m.Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skills := m.List()
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestLoadOverride(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")

	// Create global skill
	globalSkillDir := filepath.Join(globalDir, "test-skill")
	os.MkdirAll(globalSkillDir, 0755)
	os.WriteFile(filepath.Join(globalSkillDir, "SKILL.md"), []byte("# Global Skill"), 0644)

	// Create project skill with same name
	projectSkillDir := filepath.Join(projectDir, "test-skill")
	os.MkdirAll(projectSkillDir, 0755)
	os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte("# Project Skill"), 0644)

	m := NewManager(globalDir, projectDir)
	m.Load()

	skills := m.List()
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}

	// Should be project version
	skill := skills[0]
	if skill.Content != "# Project Skill" {
		t.Errorf("expected project skill content, got '%s'", skill.Content)
	}

	if skill.Source != "project" {
		t.Errorf("expected source 'project', got '%s'", skill.Source)
	}
}

func TestLoadNonExistentDir(t *testing.T) {
	m := NewManager("/nonexistent", "/nonexistent")
	err := m.Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skills := m.List()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewManager(tmpDir, "")
	err := m.Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skills := m.List()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkillMd(t *testing.T) {
	tmpDir := t.TempDir()

	// Test lowercase skill.md
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte("# Lowercase Skill"), 0644)

	m := NewManager(tmpDir, "")
	m.Load()

	skills := m.List()
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill"), 0644)

	m := NewManager(tmpDir, "")
	m.Load()

	// Get existing skill
	skill := m.Get("test-skill")
	if skill == nil {
		t.Fatal("expected non-nil skill")
	}

	if skill.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", skill.Name)
	}

	// Get non-existing skill
	skill = m.Get("nonexistent")
	if skill != nil {
		t.Error("expected nil skill")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple skills
	for _, name := range []string{"skill1", "skill2", "skill3"} {
		skillDir := filepath.Join(tmpDir, name)
		os.MkdirAll(skillDir, 0755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name), 0644)
	}

	m := NewManager(tmpDir, "")
	m.Load()

	skills := m.List()
	if len(skills) != 3 {
		t.Errorf("expected 3 skills, got %d", len(skills))
	}
}

func TestListBySource(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")

	// Create global skill
	globalSkillDir := filepath.Join(globalDir, "global-skill")
	os.MkdirAll(globalSkillDir, 0755)
	os.WriteFile(filepath.Join(globalSkillDir, "SKILL.md"), []byte("# Global"), 0644)

	// Create project skill
	projectSkillDir := filepath.Join(projectDir, "project-skill")
	os.MkdirAll(projectSkillDir, 0755)
	os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte("# Project"), 0644)

	m := NewManager(globalDir, projectDir)
	m.Load()

	// List global
	globalSkills := m.ListBySource("global")
	if len(globalSkills) != 1 {
		t.Errorf("expected 1 global skill, got %d", len(globalSkills))
	}

	// List project
	projectSkills := m.ListBySource("project")
	if len(projectSkills) != 1 {
		t.Errorf("expected 1 project skill, got %d", len(projectSkills))
	}

	// List unknown
	unknownSkills := m.ListBySource("unknown")
	if len(unknownSkills) != 0 {
		t.Errorf("expected 0 unknown skills, got %d", len(unknownSkills))
	}
}

func TestNames(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple skills
	for _, name := range []string{"skill1", "skill2", "skill3"} {
		skillDir := filepath.Join(tmpDir, name)
		os.MkdirAll(skillDir, 0755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name), 0644)
	}

	m := NewManager(tmpDir, "")
	m.Load()

	names := m.Names()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}
}

func TestBuildSkillContext(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nThis is a test."), 0644)

	m := NewManager(tmpDir, "")
	m.Load()

	context := m.BuildSkillContext("test-skill")

	if context == "" {
		t.Error("expected non-empty context")
	}

	if !contains(context, "test-skill") {
		t.Error("expected context to contain 'test-skill'")
	}

	// Test non-existing skill
	context = m.BuildSkillContext("nonexistent")
	if context != "" {
		t.Error("expected empty context for non-existing skill")
	}
}

func TestBuildAllSkillsContext(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nThis is a test."), 0644)

	m := NewManager(tmpDir, "")
	m.Load()

	context := m.BuildAllSkillsContext()

	if context == "" {
		t.Error("expected non-empty context")
	}

	if !contains(context, "test-skill") {
		t.Error("expected context to contain 'test-skill'")
	}
}

func TestBuildAllSkillsContextEmpty(t *testing.T) {
	m := NewManager("", "")
	m.Load()

	context := m.BuildAllSkillsContext()

	if context != "" {
		t.Errorf("expected empty context, got '%s'", context)
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"# Test Skill\nDescription here", "Test Skill"},
		{"## Heading\nDescription", "Heading"},
		{"Description without heading", "Description without heading"},
		{"", "(no description)"},
	}

	for _, tt := range tests {
		result := extractDescription(tt.content)
		if result != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, result)
		}
	}
}

func TestCreateProjectSkillsDir(t *testing.T) {
	tmpDir := t.TempDir()

	err := CreateProjectSkillsDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check directory was created
	skillsDir := filepath.Join(tmpDir, ".skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Error("expected skills directory to exist")
	}
}

func TestSkill(t *testing.T) {
	skill := &Skill{
		Name:        "test",
		Path:        "/path/to/skill",
		Description: "Test skill",
		Content:     "# Test",
		Source:      "project",
	}

	if skill.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", skill.Name)
	}

	if skill.Source != "project" {
		t.Errorf("expected source 'project', got '%s'", skill.Source)
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
