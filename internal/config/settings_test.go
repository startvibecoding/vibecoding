package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()

	if s.DefaultProvider != "deepseek-openai" {
		t.Errorf("expected default provider 'deepseek-openai', got '%s'", s.DefaultProvider)
	}

	if s.DefaultModel != "deepseek-v4-flash" {
		t.Errorf("expected default model 'deepseek-v4-flash', got '%s'", s.DefaultModel)
	}

	if s.DefaultMode != "agent" {
		t.Errorf("expected default mode 'agent', got '%s'", s.DefaultMode)
	}

	if len(s.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(s.Providers))
	}

	if s.DefaultThinkingLevel != "medium" {
		t.Errorf("expected thinking level 'medium', got '%s'", s.DefaultThinkingLevel)
	}
}

func TestGetProviderConfig(t *testing.T) {
	s := DefaultSettings()

	// Test existing provider (openai format)
	pc := s.GetProviderConfig("deepseek-openai")
	if pc == nil {
		t.Fatal("expected provider config, got nil")
	}

	if pc.API != "openai-chat" {
		t.Errorf("expected API 'openai-chat', got '%s'", pc.API)
	}

	// Test non-existing provider
	pc = s.GetProviderConfig("nonexistent")
	if pc != nil {
		t.Errorf("expected nil, got provider config")
	}
}

func TestGetModelConfig(t *testing.T) {
	s := DefaultSettings()

	// Test existing model
	mc := s.GetModelConfig("deepseek-openai", "deepseek-v4-flash")
	if mc == nil {
		t.Fatal("expected model config, got nil")
	}

	if mc.Name != "DeepSeek-V4-Flash" {
		t.Errorf("expected name 'DeepSeek-V4-Flash', got '%s'", mc.Name)
	}

	// Test non-existing model
	mc = s.GetModelConfig("deepseek-openai", "nonexistent")
	if mc != nil {
		t.Errorf("expected nil, got model config")
	}

	// Test non-existing provider
	mc = s.GetModelConfig("nonexistent", "model")
	if mc != nil {
		t.Errorf("expected nil, got model config")
	}
}

func TestConfigDir(t *testing.T) {
	// Test with env var
	os.Setenv("VIBECODING_DIR", "/tmp/test-vibecoding")
	dir := ConfigDir()
	if dir != "/tmp/test-vibecoding" {
		t.Errorf("expected '/tmp/test-vibecoding', got '%s'", dir)
	}
	os.Unsetenv("VIBECODING_DIR")

	// Test default
	dir = ConfigDir()
	if dir == "" {
		t.Error("expected non-empty config dir")
	}
}

func TestGlobalSettingsPath(t *testing.T) {
	path := GlobalSettingsPath()
	if path == "" {
		t.Error("expected non-empty path")
	}

	if !contains(path, "settings.json") {
		t.Error("expected path to contain 'settings.json'")
	}
}

func TestProjectSettingsPath(t *testing.T) {
	path := ProjectSettingsPath()
	if path != ".vibe/settings.json" {
		t.Errorf("expected '.vibe/settings.json', got '%s'", path)
	}
}

func TestLoadSettings(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Write test settings
	settingsJSON := `{
		"providers": {
			"test": {
				"baseUrl": "https://api.test.com",
				"apiKey": "test-key",
				"api": "openai-chat",
				"models": [
					{
						"id": "test-model",
						"name": "Test Model",
						"contextWindow": 100000,
						"maxTokens": 4096
					}
				]
			}
		},
		"defaultProvider": "test",
		"defaultModel": "test-model"
	}`

	if err := os.WriteFile(settingsPath, []byte(settingsJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Load settings
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	s := DefaultSettings()
	if err := json.Unmarshal(data, s); err != nil {
		t.Fatal(err)
	}

	if s.DefaultProvider != "test" {
		t.Errorf("expected provider 'test', got '%s'", s.DefaultProvider)
	}
}

func TestResolveKey(t *testing.T) {
	s := &Settings{
		Providers: map[string]*ProviderConfig{
			"test": {
				APIKey: "test-api-key",
			},
		},
	}

	// Test direct key
	key := s.ResolveKey("test")
	if key != "test-api-key" {
		t.Errorf("expected 'test-api-key', got '%s'", key)
	}

	// Test env var
	os.Setenv("TEST_API_KEY", "env-key")
	s.Providers["env"] = &ProviderConfig{
		APIKey: "TEST_API_KEY",
	}
	key = s.ResolveKey("env")
	if key != "env-key" {
		t.Errorf("expected 'env-key', got '%s'", key)
	}
	os.Unsetenv("TEST_API_KEY")

	// Test missing key
	key = s.ResolveKey("nonexistent")
	if key != "" {
		t.Errorf("expected empty string, got '%s'", key)
	}
}

func TestGetShell(t *testing.T) {
	s := &Settings{}

	// Test default
	shell := s.GetShell()
	if shell == "" {
		t.Error("expected non-empty shell")
	}

	// Test custom
	s.ShellPath = "/bin/zsh"
	shell = s.GetShell()
	if shell != "/bin/zsh" {
		t.Errorf("expected '/bin/zsh', got '%s'", shell)
	}
}

func TestGetSessionDir(t *testing.T) {
	s := &Settings{}

	// Test default
	dir := s.GetSessionDir()
	if dir == "" {
		t.Error("expected non-empty session dir")
	}

	// Test custom
	s.SessionDir = "/tmp/sessions"
	dir = s.GetSessionDir()
	if dir != "/tmp/sessions" {
		t.Errorf("expected '/tmp/sessions', got '%s'", dir)
	}

	// Test with tilde
	s.SessionDir = "~/sessions"
	dir = s.GetSessionDir()
	if dir == "" {
		t.Error("expected non-empty session dir")
	}
}

func TestGetGlobalSkillsDir(t *testing.T) {
	s := &Settings{}

	// Test default
	dir := s.GetGlobalSkillsDir()
	if dir == "" {
		t.Error("expected non-empty skills dir")
	}

	// Test custom
	s.SkillsDir = "/tmp/skills"
	dir = s.GetGlobalSkillsDir()
	if dir != "/tmp/skills" {
		t.Errorf("expected '/tmp/skills', got '%s'", dir)
	}
}

func TestSaveGlobalSettings(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	os.Setenv("VIBECODING_DIR", tmpDir)
	defer os.Unsetenv("VIBECODING_DIR")

	s := DefaultSettings()
	s.DefaultProvider = "test"

	err := SaveGlobalSettings(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	settingsPath := filepath.Join(tmpDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("expected settings file to exist")
	}

	// Load and verify
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	loaded := &Settings{}
	if err := json.Unmarshal(data, loaded); err != nil {
		t.Fatal(err)
	}

	if loaded.DefaultProvider != "test" {
		t.Errorf("expected provider 'test', got '%s'", loaded.DefaultProvider)
	}
}

func TestResolveKeyValue(t *testing.T) {
	// Test direct value
	key := resolveKeyValue("direct-key")
	if key != "direct-key" {
		t.Errorf("expected 'direct-key', got '%s'", key)
	}

	// Test env var
	os.Setenv("TEST_ENV_KEY", "env-value")
	key = resolveKeyValue("TEST_ENV_KEY")
	if key != "env-value" {
		t.Errorf("expected 'env-value', got '%s'", key)
	}
	os.Unsetenv("TEST_ENV_KEY")
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
