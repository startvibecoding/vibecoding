package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/platform"
)

// Verbose controls whether config loading prints diagnostic messages to stderr.
var Verbose bool

// Settings holds all configuration for vibecoding.
type Settings struct {
	Providers            map[string]*ProviderConfig `json:"providers,omitempty"`
	DefaultProvider      string                     `json:"defaultProvider,omitempty"`
	DefaultModel         string                     `json:"defaultModel,omitempty"`
	DefaultThinkingLevel string                     `json:"defaultThinkingLevel,omitempty"`
	DefaultMode          string                     `json:"defaultMode,omitempty"`
	EnablePlanTool       *bool                      `json:"enablePlanTool,omitempty"`
	MaxContextTokens     int                        `json:"maxContextTokens,omitempty"`
	MaxOutputTokens      int                        `json:"maxOutputTokens,omitempty"`
	ContextFiles         ContextFilesSettings       `json:"contextFiles"`
	SkillsDir            string                     `json:"skillsDir,omitempty"`
	Compaction           CompactionSettings         `json:"compaction"`
	Sandbox              SandboxSettings            `json:"sandbox"`
	SessionDir           string                     `json:"sessionDir,omitempty"`
	ShellPath            string                     `json:"shellPath,omitempty"`
	ShellCommandPrefix   string                     `json:"shellCommandPrefix,omitempty"`
	Theme                string                     `json:"theme,omitempty"`
	Retry                RetrySettings              `json:"retry"`
	Approval             ApprovalSettings           `json:"approval"`
}

type ProviderConfig struct {
	APIKey         string        `json:"apiKey,omitempty"`
	BaseURL        string        `json:"baseUrl,omitempty"`
	API            string        `json:"api,omitempty"`
	ThinkingFormat string        `json:"thinkingFormat,omitempty"` // "", "openai", "anthropic", "xiaomi"
	CacheControl   *bool         `json:"cacheControl,omitempty"`   // enable cache_control markers (nil=auto, true=force on, false=force off)
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

	// Idle compression settings (R5.1-R5.5)
	IdleCompressionEnabled   bool `json:"idleCompressionEnabled,omitempty"`   // R5.1: off by default
	IdleTimeoutSeconds       int  `json:"idleTimeoutSeconds,omitempty"`       // seconds of inactivity (default: 90)
	IdleMinTokensForCompress int  `json:"idleMinTokensForCompress,omitempty"` // minimum tokens to trigger (default: 150000)
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
	// ConfirmBeforeWrite requires user approval before write/edit tools run in agent mode.
	ConfirmBeforeWrite *bool `json:"confirmBeforeWrite,omitempty"`
}

func DefaultSettings() *Settings {
	return &Settings{
		Providers: map[string]*ProviderConfig{
			"deepseek-anthropic": &ProviderConfig{
				BaseURL: "https://api.deepseek.com/anthropic",
				APIKey:  "${DEEPSEEK_API_KEY}",
				API:     "anthropic-messages",
				Models: []ModelConfig{
					{ID: "deepseek-v4-flash", Name: "DeepSeek-V4-Flash", ContextWindow: 1000000, MaxTokens: 384000, Cost: &CostConfig{Input: 0.5, Output: 2}, Input: []string{"text"}},
					{ID: "deepseek-v4-pro", Name: "DeepSeek-V4-Pro", Reasoning: true, ContextWindow: 1000000, MaxTokens: 384000, Cost: &CostConfig{Input: 1, Output: 4}, Input: []string{"text"}},
				},
			},
			"deepseek-openai": &ProviderConfig{
				BaseURL: "https://api.deepseek.com",
				APIKey:  "${DEEPSEEK_API_KEY}",
				API:     "openai-chat",
				Models: []ModelConfig{
					{ID: "deepseek-v4-flash", Name: "DeepSeek-V4-Flash", ContextWindow: 1000000, MaxTokens: 384000, Cost: &CostConfig{Input: 0.5, Output: 2}, Input: []string{"text"}},
					{ID: "deepseek-v4-pro", Name: "DeepSeek-V4-Pro", Reasoning: true, ContextWindow: 1000000, MaxTokens: 384000, Cost: &CostConfig{Input: 1, Output: 4}, Input: []string{"text"}},
				},
			},
		},
		DefaultProvider:      "deepseek-openai",
		DefaultModel:         "deepseek-v4-flash",
		DefaultThinkingLevel: "medium",
		DefaultMode:          "agent",
		EnablePlanTool:       boolPtr(true),
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
			BashWhitelist:      []string{"go ", "make ", "git ", "npm ", "yarn ", "node ", "python ", "pip "},
			ConfirmBeforeWrite: boolPtr(true),
		},
	}
}

func boolPtr(v bool) *bool {
	return &v
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

	globalPath := GlobalSettingsPath()
	if Verbose {
		fmt.Fprintf(os.Stderr, "[config] Loading global settings: %s\n", globalPath)
	}
	if data, err := os.ReadFile(globalPath); err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			return nil, fmt.Errorf("parse global settings: %w", err)
		}
		if Verbose {
			fmt.Fprintf(os.Stderr, "[config] Loaded global settings\n")
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: could not read global settings %s: %v\n", globalPath, err)
	} else if Verbose {
		fmt.Fprintf(os.Stderr, "[config] Global settings not found: %s\n", globalPath)
	}

	projectPath := ProjectSettingsPath()
	if Verbose {
		fmt.Fprintf(os.Stderr, "[config] Loading project settings: %s\n", projectPath)
	}
	if data, err := os.ReadFile(projectPath); err == nil {
		var proj Settings
		if err := json.Unmarshal(data, &proj); err != nil {
			return nil, fmt.Errorf("parse project settings: %w", err)
		}
		mergeSettings(s, &proj)
		if Verbose {
			fmt.Fprintf(os.Stderr, "[config] Loaded project settings\n")
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: could not read project settings %s: %v\n", projectPath, err)
	} else if Verbose {
		fmt.Fprintf(os.Stderr, "[config] Project settings not found: %s\n", projectPath)
		// Detect common typo: .vibe/setting.json (singular)
		if _, err2 := os.Stat(".vibe/setting.json"); err2 == nil {
			fmt.Fprintf(os.Stderr, "[config] Found .vibe/setting.json (singular) — expected .vibe/settings.json (plural). Please rename the file.\n")
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

// mergeSettings deep-merges project settings into global settings.
// Top-level scalar fields are overwritten if non-zero in proj.
// The Providers map is merged per-key rather than replaced.
func mergeSettings(s, proj *Settings) {
	if proj.DefaultProvider != "" {
		s.DefaultProvider = proj.DefaultProvider
	}
	if proj.DefaultModel != "" {
		s.DefaultModel = proj.DefaultModel
	}
	if proj.DefaultThinkingLevel != "" {
		s.DefaultThinkingLevel = proj.DefaultThinkingLevel
	}
	if proj.DefaultMode != "" {
		s.DefaultMode = proj.DefaultMode
	}
	if proj.EnablePlanTool != nil {
		s.EnablePlanTool = boolPtr(*proj.EnablePlanTool)
	}
	if proj.MaxContextTokens != 0 {
		s.MaxContextTokens = proj.MaxContextTokens
	}
	if proj.MaxOutputTokens != 0 {
		s.MaxOutputTokens = proj.MaxOutputTokens
	}
	if proj.SkillsDir != "" {
		s.SkillsDir = proj.SkillsDir
	}
	if proj.SessionDir != "" {
		s.SessionDir = proj.SessionDir
	}
	if proj.ShellPath != "" {
		s.ShellPath = proj.ShellPath
	}
	if proj.ShellCommandPrefix != "" {
		s.ShellCommandPrefix = proj.ShellCommandPrefix
	}
	if proj.Theme != "" {
		s.Theme = proj.Theme
	}

	// Merge nested structs only if they are non-zero
	if proj.ContextFiles.Enabled != s.ContextFiles.Enabled || len(proj.ContextFiles.ExtraFiles) > 0 {
		s.ContextFiles = proj.ContextFiles
	}
	if proj.Compaction.Enabled != s.Compaction.Enabled || proj.Compaction.ReserveTokens != 0 || proj.Compaction.KeepRecentTokens != 0 {
		s.Compaction = proj.Compaction
	}
	if proj.Sandbox.Enabled != s.Sandbox.Enabled || proj.Sandbox.Level != "" {
		s.Sandbox = proj.Sandbox
	}
	if proj.Retry.Enabled != s.Retry.Enabled || proj.Retry.MaxRetries != 0 || proj.Retry.BaseDelayMs != 0 {
		s.Retry = proj.Retry
	}
	if len(proj.Approval.BashWhitelist) > 0 || len(proj.Approval.BashBlacklist) > 0 || proj.Approval.ConfirmBeforeWrite != nil {
		s.Approval = proj.Approval
	}

	// Deep merge providers: add or override individual providers
	for name, pc := range proj.Providers {
		if pc == nil {
			continue
		}
		if s.Providers == nil {
			s.Providers = make(map[string]*ProviderConfig)
		}
		s.Providers[name] = pc
	}
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

func (s *Settings) ResolveKey(providerName string) string {
	// 1. Use apiKey from provider config (supports ${VAR} env references)
	if pc, ok := s.Providers[providerName]; ok && pc != nil && pc.APIKey != "" {
		return resolveKeyValue(pc.APIKey)
	}
	// 2. Fallback: derive env var from provider name, e.g. "deepseek-openai" → "DEEPSEEK_OPENAI_API_KEY"
	envVar := providerToEnvVar(providerName)
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return ""
}

// providerToEnvVar converts a provider name to a conventional environment variable name.
// e.g. "deepseek-openai" → "DEEPSEEK_OPENAI_API_KEY", "my-provider" → "MY_PROVIDER_API_KEY".
func providerToEnvVar(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_API_KEY"
}

func resolveKeyValue(key string) string {
	if strings.HasPrefix(key, "!") {
		return resolveShellCommand(key[1:])
	}

	// Handle ${VAR} syntax: look up the variable name inside ${}
	envName := key
	if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
		envName = key[2 : len(key)-1]
	}

	if !strings.Contains(envName, " ") {
		if v := os.Getenv(envName); v != "" {
			return v
		}
	}

	return key
}

func (s *Settings) GetProviderConfig(name string) *ProviderConfig {
	return s.Providers[name]
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
	if cmd == "" {
		return ""
	}
	var out []byte
	var err error
	if platform.IsWindows() {
		out, err = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmd).Output()
	} else {
		out, err = exec.Command("sh", "-c", cmd).Output()
	}
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

func (s *Settings) IsPlanToolEnabled() bool {
	if s.EnablePlanTool == nil {
		return true
	}
	return *s.EnablePlanTool
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
