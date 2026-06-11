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

	if len(s.Providers) != 7 {
		t.Errorf("expected 7 providers, got %d", len(s.Providers))
	}

	if s.Providers["openai"] == nil {
		t.Fatal("expected default openai provider")
	}
	if s.Providers["anthropic"] == nil {
		t.Fatal("expected default anthropic provider")
	}
	if s.Providers["xiaomi"] == nil {
		t.Fatal("expected default xiaomi provider")
	}
	if s.Providers["google-gemini"] == nil {
		t.Fatal("expected default google-gemini provider")
	}
	if s.Providers["google-vertex"] == nil {
		t.Fatal("expected default google-vertex provider")
	}

	if s.DefaultThinkingLevel != "medium" {
		t.Errorf("expected thinking level 'medium', got '%s'", s.DefaultThinkingLevel)
	}
	if s.WebSearch.Enabled == nil || *s.WebSearch.Enabled {
		t.Fatalf("expected web search to be disabled by default, got %#v", s.WebSearch.Enabled)
	}
	if s.WebSearch.Provider != "openai" || s.WebSearch.ProviderType != "responses" {
		t.Fatalf("unexpected web search defaults: %#v", s.WebSearch)
	}
	if s.WebSearch.Model != "" {
		t.Fatalf("expected empty web search model by default, got %q", s.WebSearch.Model)
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
	if s.WebSearch.Model != "" {
		t.Errorf("expected empty webSearch.model, got '%s'", s.WebSearch.Model)
	}
}

func TestLoadSettingsAppliesProjectOverridesAndEnv(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	configDir := filepath.Join(tmpDir, "config")
	if err := os.Setenv("VIBECODING_DIR", configDir); err != nil {
		t.Fatalf("set VIBECODING_DIR: %v", err)
	}
	if err := os.Setenv("VIBECODING_PROVIDER", "env-provider"); err != nil {
		t.Fatalf("set VIBECODING_PROVIDER: %v", err)
	}
	if err := os.Setenv("VIBECODING_MODEL", "env-model"); err != nil {
		t.Fatalf("set VIBECODING_MODEL: %v", err)
	}
	if err := os.Setenv("VIBECODING_MODE", "plan"); err != nil {
		t.Fatalf("set VIBECODING_MODE: %v", err)
	}
	if err := os.Setenv("VIBECODING_THINKING", "high"); err != nil {
		t.Fatalf("set VIBECODING_THINKING: %v", err)
	}
	defer func() {
		_ = os.Unsetenv("VIBECODING_DIR")
		_ = os.Unsetenv("VIBECODING_PROVIDER")
		_ = os.Unsetenv("VIBECODING_MODEL")
		_ = os.Unsetenv("VIBECODING_MODE")
		_ = os.Unsetenv("VIBECODING_THINKING")
	}()

	if err := os.MkdirAll(".vibe", 0700); err != nil {
		t.Fatalf("mkdir .vibe: %v", err)
	}
	projectSettings := `{
		"sessionDir": "./sessions",
		"providers": {
			"project-provider": {
				"baseUrl": "https://example.test",
				"api": "openai-chat",
				"models": [{"id": "project-model", "name": "Project Model"}]
			}
		},
		"contextFiles": {"enabled": false, "extraFiles": ["extra.md"]},
		"approval": {"bashWhitelist": ["go test "]}
	}`
	if err := os.WriteFile(ProjectSettingsPath(), []byte(projectSettings), 0600); err != nil {
		t.Fatalf("write project settings: %v", err)
	}

	s, err := LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if s.DefaultProvider != "env-provider" {
		t.Fatalf("DefaultProvider = %q, want env-provider", s.DefaultProvider)
	}
	if s.DefaultModel != "env-model" {
		t.Fatalf("DefaultModel = %q, want env-model", s.DefaultModel)
	}
	if s.DefaultMode != "plan" {
		t.Fatalf("DefaultMode = %q, want plan", s.DefaultMode)
	}
	if s.DefaultThinkingLevel != "high" {
		t.Fatalf("DefaultThinkingLevel = %q, want high", s.DefaultThinkingLevel)
	}
	if s.SessionDir != "./sessions" {
		t.Fatalf("SessionDir = %q, want ./sessions", s.SessionDir)
	}
	if s.GetProviderConfig("project-provider") == nil {
		t.Fatal("expected merged project provider")
	}
	if s.GetProviderConfig("deepseek-openai") == nil {
		t.Fatal("expected default provider to remain after project merge")
	}
	if s.ContextFiles.Enabled {
		t.Fatal("expected project contextFiles override to disable context files")
	}
	if len(s.ContextFiles.ExtraFiles) != 1 || s.ContextFiles.ExtraFiles[0] != "extra.md" {
		t.Fatalf("ExtraFiles = %#v, want extra.md", s.ContextFiles.ExtraFiles)
	}
	if len(s.Approval.BashWhitelist) != 1 || s.Approval.BashWhitelist[0] != "go test " {
		t.Fatalf("BashWhitelist = %#v, want go test", s.Approval.BashWhitelist)
	}
}

func TestDefaultSettingsConfirmBeforeWrite(t *testing.T) {
	s := DefaultSettings()
	if s.Approval.ConfirmBeforeWrite == nil || !*s.Approval.ConfirmBeforeWrite {
		t.Fatal("expected confirmBeforeWrite to be enabled by default")
	}
}

func TestDefaultSettingsEnablePlanTool(t *testing.T) {
	s := DefaultSettings()
	if s.EnablePlanTool == nil || !*s.EnablePlanTool {
		t.Fatal("expected enablePlanTool to be enabled by default")
	}
	if !s.IsPlanToolEnabled() {
		t.Fatal("expected IsPlanToolEnabled to return true by default")
	}
}

func TestMergeSettingsIgnoresNilProviderAndKeepsExistingProviders(t *testing.T) {
	base := &Settings{
		Providers: map[string]*ProviderConfig{
			"base": {API: "openai-chat"},
		},
		DefaultProvider: "base",
	}
	project := &Settings{
		Providers: map[string]*ProviderConfig{
			"base": nil,
			"new":  {API: "anthropic"},
		},
		DefaultProvider: "project",
	}

	mergeSettings(base, project)

	if base.DefaultProvider != "project" {
		t.Fatalf("DefaultProvider = %q, want project", base.DefaultProvider)
	}
	if base.Providers["base"] == nil {
		t.Fatal("expected nil provider override to be ignored")
	}
	if base.Providers["new"] == nil || base.Providers["new"].API != "anthropic" {
		t.Fatalf("new provider = %#v, want anthropic provider", base.Providers["new"])
	}
}

func TestMergeSettingsEnablePlanToolOverride(t *testing.T) {
	base := DefaultSettings()
	disabled := false
	project := &Settings{EnablePlanTool: &disabled}

	mergeSettings(base, project)

	if base.IsPlanToolEnabled() {
		t.Fatal("expected enablePlanTool=false override to be applied")
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

func TestResolveProviderHeaders(t *testing.T) {
	t.Setenv("CUSTOM_HEADER_VALUE", "env-header-value")
	s := &Settings{
		Providers: map[string]*ProviderConfig{
			"test": {
				Headers: map[string]string{
					"X-Static": "static-value",
					"X-Env":    "${CUSTOM_HEADER_VALUE}",
					" ":        "ignored",
				},
			},
		},
	}

	headers := s.ResolveProviderHeaders("test")
	if headers["X-Static"] != "static-value" {
		t.Fatalf("X-Static = %q, want static-value", headers["X-Static"])
	}
	if headers["X-Env"] != "env-header-value" {
		t.Fatalf("X-Env = %q, want env-header-value", headers["X-Env"])
	}
	if _, ok := headers[""]; ok {
		t.Fatal("expected empty header name to be ignored")
	}
	if got := s.ResolveProviderHeaders("missing"); got != nil {
		t.Fatalf("missing headers = %#v, want nil", got)
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

func TestResolveKeyValueShellCommandRequiresOptIn(t *testing.T) {
	t.Setenv("VIBECODING_ALLOW_SHELL_CONFIG", "")
	if got := resolveKeyValue("!printf secret"); got != "!printf secret" {
		t.Fatalf("resolveKeyValue without opt-in = %q, want literal", got)
	}

	t.Setenv("VIBECODING_ALLOW_SHELL_CONFIG", "1")
	if got := resolveKeyValue("!printf secret"); got != "secret" {
		t.Fatalf("resolveKeyValue with opt-in = %q, want secret", got)
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
