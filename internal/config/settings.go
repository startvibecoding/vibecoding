package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/platform"
)

// Settings holds all configuration for vibecoding.
type Settings struct {
	Providers            map[string]ProviderConfig `json:"providers,omitempty"`
	DefaultProvider      string                    `json:"defaultProvider,omitempty"`
	DefaultModel         string                    `json:"defaultModel,omitempty"`
	DefaultThinkingLevel string                    `json:"defaultThinkingLevel,omitempty"`
	DefaultMode          string                    `json:"defaultMode,omitempty"`
	MaxContextTokens     int                       `json:"maxContextTokens,omitempty"`
	MaxOutputTokens      int                       `json:"maxOutputTokens,omitempty"`
	ContextFiles         ContextFilesSettings      `json:"contextFiles"`
	SkillsDir            string                    `json:"skillsDir,omitempty"`
	Compaction           CompactionSettings        `json:"compaction"`
	Sandbox              SandboxSettings           `json:"sandbox"`
	SessionDir           string                    `json:"sessionDir,omitempty"`
	ShellPath            string                    `json:"shellPath,omitempty"`
	ShellCommandPrefix   string                    `json:"shellCommandPrefix,omitempty"`
	Theme                string                    `json:"theme,omitempty"`
	Retry                RetrySettings             `json:"retry"`
	Approval             ApprovalSettings          `json:"approval"`
}

type ProviderConfig struct {
	APIKey         string        `json:"apiKey,omitempty"`
	BaseURL        string        `json:"baseUrl,omitempty"`
	API            string        `json:"api,omitempty"`
	ThinkingFormat string        `json:"thinkingFormat,omitempty"` // "", "openai", "anthropic", "xiaomi"
	Models         []ModelConfig `json:"models"`
}

type ModelConfig struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Reasoning     bool        `json:"reasoning,omitempty"`
	ContextWindow int         `json:"contextWindow,omitempty"`
	MaxTokens     int         `json:"maxTokens,omitempty"`
	Cost          *CostConfig `json:"cost,omitempty"`
	Input         []string    `json:"input,omitempty"`
}

type CostConfig struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead,omitempty"`
	CacheWrite float64 `json:"cacheWrite,omitempty"`
}

type ContextFilesSettings struct {
	Enabled    bool     `json:"enabled"`
	ExtraFiles []string `json:"extraFiles,omitempty"`
}

type CompactionSettings struct {
	Enabled          bool `json:"enabled"`
	ReserveTokens    int  `json:"reserveTokens"`
	KeepRecentTokens int  `json:"keepRecentTokens"`
}

type SandboxSettings struct {
	Enabled      bool     `json:"enabled"`
	Level        string   `json:"level"`
	BwrapPath    string   `json:"bwrapPath,omitempty"`
	AllowNetwork bool     `json:"allowNetwork"`
	AllowedRead  []string `json:"allowedRead,omitempty"`
	AllowedWrite []string `json:"allowedWrite,omitempty"`
	DeniedPaths  []string `json:"deniedPaths,omitempty"`
	PassEnv      []string `json:"passEnv,omitempty"`
	TmpSize      string   `json:"tmpSize,omitempty"`
}

type RetrySettings struct {
	Enabled     bool `json:"enabled"`
	MaxRetries  int  `json:"maxRetries"`
	BaseDelayMs int  `json:"baseDelayMs"`
}

type ApprovalSettings struct {
	// BashWhitelist is a list of command prefixes that auto-approve in agent mode
	BashWhitelist []string `json:"bashWhitelist,omitempty"`
	// BashBlacklist is a list of command prefixes that always require approval (even in yolo mode if configured)
	BashBlacklist []string `json:"bashBlacklist,omitempty"`
}

func DefaultSettings() *Settings {
	return &Settings{
		Providers: map[string]ProviderConfig{
			"anthropic": {
				BaseURL: "https://api.anthropic.com",
				APIKey:  "${ANTHROPIC_API_KEY}",
				API:     "anthropic-messages",
				Models: []ModelConfig{
					{ID: "claude-sonnet-4-20250514", Name: "Claude 4 Sonnet", Reasoning: true, ContextWindow: 200000, MaxTokens: 16384, Cost: &CostConfig{Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75}, Input: []string{"text", "image"}},
					{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextWindow: 200000, MaxTokens: 8192, Cost: &CostConfig{Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75}, Input: []string{"text", "image"}},
					{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ContextWindow: 200000, MaxTokens: 8192, Cost: &CostConfig{Input: 0.8, Output: 4, CacheRead: 0.08, CacheWrite: 1}, Input: []string{"text", "image"}},
					{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", ContextWindow: 200000, MaxTokens: 4096, Cost: &CostConfig{Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75}, Input: []string{"text", "image"}},
				},
			},
			"openai": {
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "${OPENAI_API_KEY}",
				API:     "openai-chat",
				Models: []ModelConfig{
					{ID: "gpt-4o", Name: "GPT-4o", ContextWindow: 128000, MaxTokens: 16384, Cost: &CostConfig{Input: 2.5, Output: 10, CacheRead: 1.25, CacheWrite: 2.5}, Input: []string{"text", "image"}},
					{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ContextWindow: 128000, MaxTokens: 16384, Cost: &CostConfig{Input: 0.15, Output: 0.6, CacheRead: 0.075, CacheWrite: 0.15}, Input: []string{"text", "image"}},
					{ID: "o1", Name: "o1", Reasoning: true, ContextWindow: 200000, MaxTokens: 100000, Cost: &CostConfig{Input: 15, Output: 60, CacheRead: 7.5, CacheWrite: 15}, Input: []string{"text", "image"}},
					{ID: "o3-mini", Name: "o3-mini", Reasoning: true, ContextWindow: 200000, MaxTokens: 100000, Cost: &CostConfig{Input: 1.1, Output: 4.4, CacheRead: 0.55, CacheWrite: 1.1}, Input: []string{"text", "image"}},
				},
			},
		},
		DefaultProvider:      "anthropic",
		DefaultModel:         "claude-sonnet-4-20250514",
		DefaultThinkingLevel: "medium",
		DefaultMode:          "agent",
		ContextFiles:         ContextFilesSettings{Enabled: true},
		SkillsDir:            platform.SkillsDir(),
		Compaction:           CompactionSettings{Enabled: true, ReserveTokens: 16384, KeepRecentTokens: 20000},
		Sandbox: SandboxSettings{
			Enabled:     false,
			Level:       "none",
			AllowedRead: platform.SandboxPaths(),
			DeniedPaths: platform.DeniedPaths(),
			PassEnv:     platform.DefaultEnvVars(),
			TmpSize:     "100m",
		},
		SessionDir: platform.SessionDir(),
		Theme:      "dark",
		Retry:      RetrySettings{Enabled: true, MaxRetries: 3, BaseDelayMs: 2000},
		Approval: ApprovalSettings{
			BashWhitelist: []string{"go ", "make ", "git ", "npm ", "yarn ", "node ", "python ", "pip "},
		},
	}
}

func ConfigDir() string {
	return platform.ConfigDir()
}

func GlobalSettingsPath() string {
	return filepath.Join(ConfigDir(), "settings.json")
}

func ProjectSettingsPath() string {
	return filepath.Join(".vibe", "settings.json")
}

func LoadSettings() (*Settings, error) {
	s := DefaultSettings()

	if err := ensureConfigExists(s); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create config: %v\n", err)
	}

	if data, err := os.ReadFile(GlobalSettingsPath()); err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			return nil, fmt.Errorf("parse global settings: %w", err)
		}
	}

	if data, err := os.ReadFile(ProjectSettingsPath()); err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			return nil, fmt.Errorf("parse project settings: %w", err)
		}
	}

	if v := os.Getenv("VIBECODING_PROVIDER"); v != "" {
		s.DefaultProvider = v
	}
	if v := os.Getenv("VIBECODING_MODEL"); v != "" {
		s.DefaultModel = v
	}
	if v := os.Getenv("VIBECODING_MODE"); v != "" {
		s.DefaultMode = v
	}
	if v := os.Getenv("VIBECODING_THINKING"); v != "" {
		s.DefaultThinkingLevel = v
	}

	return s, nil
}

func ensureConfigExists(defaults *Settings) error {
	configDir := ConfigDir()
	settingsPath := GlobalSettingsPath()

	if _, err := os.Stat(settingsPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Created default config: %s\n", settingsPath)
	return nil
}

type AuthData struct {
	Entries map[string]AuthEntry `json:"entries"`
}

type AuthEntry struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

func AuthFilePath() string {
	return filepath.Join(ConfigDir(), "auth.json")
}

func LoadAuth() (*AuthData, error) {
	data := &AuthData{Entries: make(map[string]AuthEntry)}
	raw, err := os.ReadFile(AuthFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &data.Entries); err != nil {
		return nil, fmt.Errorf("parse auth.json: %w", err)
	}
	return data, nil
}

func (s *Settings) ResolveKey(providerName string) string {
	if pc, ok := s.Providers[providerName]; ok && pc.APIKey != "" {
		return resolveKeyValue(pc.APIKey)
	}
	auth, err := LoadAuth()
	if err == nil {
		if entry, ok := auth.Entries[providerName]; ok && entry.Key != "" {
			return resolveKeyValue(entry.Key)
		}
	}
	envMap := map[string]string{
		"anthropic": "ANTHROPIC_API_KEY",
		"openai":    "OPENAI_API_KEY",
	}
	if envVar, ok := envMap[providerName]; ok {
		return os.Getenv(envVar)
	}
	return ""
}

func resolveKeyValue(key string) string {
	if strings.HasPrefix(key, "!") {
		return resolveShellCommand(key[1:])
	}
	if v := os.Getenv(key); v != "" && !strings.Contains(key, " ") {
		return v
	}
	return key
}

func (s *Settings) GetProviderConfig(name string) *ProviderConfig {
	if pc, ok := s.Providers[name]; ok {
		return &pc
	}
	return nil
}

func (s *Settings) GetModelConfig(providerName, modelID string) *ModelConfig {
	pc := s.GetProviderConfig(providerName)
	if pc == nil {
		return nil
	}
	for _, m := range pc.Models {
		if m.ID == modelID {
			return &m
		}
	}
	return nil
}

func resolveShellCommand(cmd string) string {
	return ""
}

func (s *Settings) GetShell() string {
	if s.ShellPath != "" {
		return s.ShellPath
	}
	return platform.DefaultShell()
}

func (s *Settings) GetSessionDir() string {
	if s.SessionDir != "" {
		if strings.HasPrefix(s.SessionDir, "~") {
			return platform.ExpandHome(s.SessionDir)
		}
		return s.SessionDir
	}
	return platform.SessionDir()
}

func (s *Settings) GetGlobalSkillsDir() string {
	if s.SkillsDir != "" {
		if strings.HasPrefix(s.SkillsDir, "~") {
			return platform.ExpandHome(s.SkillsDir)
		}
		return s.SkillsDir
	}
	return platform.SkillsDir()
}

func SaveGlobalSettings(s *Settings) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GlobalSettingsPath(), data, 0600)
}
