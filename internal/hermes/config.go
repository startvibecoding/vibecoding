package hermes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/config"
)

// HermesConfig holds all configuration for hermes mode.
type HermesConfig struct {
	Server          ServerConfig   `json:"server"`
	DefaultProvider string         `json:"default_provider,omitempty"`
	DefaultModel    string         `json:"default_model,omitempty"`
	MultiAgent      bool           `json:"multi_agent,omitempty"`
	Sandbox         bool           `json:"sandbox,omitempty"`
	Wechat          WechatConfig   `json:"wechat"`
	Feishu          FeishuConfig   `json:"feishu"`
	Webhooks        WebhookConfig  `json:"webhooks"`
	A2A             A2AConfig      `json:"a2a"`
	Cron            CronConfig     `json:"cron"`
	Memory          MemoryConfig   `json:"memory"`
	Security        SecurityConfig `json:"security"`
	Hooks           HooksConfig    `json:"hooks"`
	Agent           AgentConfig    `json:"agent"`
	WorkDir         string         `json:"work_dir"`
}

// ServerConfig defines the WebSocket + HTTP gateway settings.
type ServerConfig struct {
	Port      int    `json:"port"`
	Host      string `json:"host"`
	AuthToken string `json:"auth_token"`
}

// WechatConfig defines WeChat iLink platform settings.
type WechatConfig struct {
	Enabled      bool     `json:"enabled"`
	CredPath     string   `json:"cred_path"`
	WorkDir      string   `json:"work_dir"`
	AllowedUsers []string `json:"allowed_users"`
	AutoTyping   bool     `json:"auto_typing"`
}

// FeishuConfig defines Feishu (Lark) platform settings.
type FeishuConfig struct {
	Enabled      bool     `json:"enabled"`
	AppID        string   `json:"app_id"`
	AppSecret    string   `json:"app_secret"`
	WorkDir      string   `json:"work_dir"`
	AllowedUsers []string `json:"allowed_users"`
}

// WebhookConfig defines inbound webhook settings.
type WebhookConfig struct {
	Enabled bool           `json:"enabled"`
	Secret  string         `json:"secret"`
	Routes  []WebhookRoute `json:"routes"`
}

// WebhookRoute maps an inbound webhook path to an agent skill + delivery.
type WebhookRoute struct {
	Path     string   `json:"path"`
	Events   []string `json:"events"`
	Skill    string   `json:"skill"`
	Delivery string   `json:"delivery"`
}

// A2AConfig defines A2A protocol settings.
type A2AConfig struct {
	Enabled bool `json:"enabled"`
}

// CronConfig defines cron scheduler settings.
type CronConfig struct {
	Enabled  bool   `json:"enabled"`
	StorePath string `json:"store_path,omitempty"` // empty = <sessionDir>/hermes/cron.json
	Interval int    `json:"interval,omitempty"`    // seconds between checks (default 30)
}

// MemoryConfig defines persistent memory settings.
type MemoryConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"` // empty = auto-discover .vibe/memory.md → <GLOBAL_DIR>/memory.md
}

// SecurityConfig defines security settings.
type SecurityConfig struct {
	SmartApprovals  bool     `json:"smart_approvals"`
	AllowedWorkDirs []string `json:"allowed_work_dirs"`
}

// HooksConfig defines shell hook scripts.
type HooksConfig struct {
	PreToolCall  string `json:"pre_tool_call"`
	PostToolCall string `json:"post_tool_call"`
}

// AgentConfig defines agent behavior settings.
type AgentConfig struct {
	MaxTurns        int  `json:"max_turns"`
	BudgetPressure  bool `json:"budget_pressure"`
	ContextPressure bool `json:"context_pressure"`
}

// DefaultHermesConfig returns the default configuration.
func DefaultHermesConfig() *HermesConfig {
	return &HermesConfig{
		Server: ServerConfig{
			Port: 8090,
			Host: "0.0.0.0",
		},
		Wechat: WechatConfig{
			AutoTyping: true,
		},
		Cron: CronConfig{
			Enabled: true,
		},
		Memory: MemoryConfig{
			Enabled: true,
		},
		Security: SecurityConfig{
			SmartApprovals: true,
		},
		Agent: AgentConfig{
			MaxTurns:        90,
			BudgetPressure:  true,
			ContextPressure: true,
		},
		WorkDir: ".",
	}
}

// HermesConfigPath returns the path to the global hermes.json.
func HermesConfigPath() string {
	return filepath.Join(config.ConfigDir(), "hermes.json")
}

// ProjectHermesConfigPath returns the path to the project-level hermes.json.
func ProjectHermesConfigPath() string {
	return filepath.Join(".vibe", "hermes.json")
}

// LoadHermesConfig loads the hermes configuration, merging global + project.
// Priority: defaults → <GLOBAL_DIR>/hermes.json → .vibe/hermes.json
func LoadHermesConfig() (*HermesConfig, error) {
	cfg := DefaultHermesConfig()

	// 1. Load global config
	globalPath := HermesConfigPath()
	if data, err := os.ReadFile(globalPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse global hermes config %s: %w", globalPath, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read global hermes config %s: %w", globalPath, err)
	}

	// 2. Overlay project-level config
	projectPath := ProjectHermesConfigPath()
	if data, err := os.ReadFile(projectPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse project hermes config %s: %w", projectPath, err)
		}
	}

	// Resolve environment variable references
	cfg.resolveEnvVars()

	return cfg, nil
}

// LoadHermesConfigFrom loads hermes config from a specific path.
func LoadHermesConfigFrom(path string) (*HermesConfig, error) {
	cfg := DefaultHermesConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read hermes config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse hermes config %s: %w", path, err)
	}
	cfg.resolveEnvVars()
	return cfg, nil
}

// GetListenAddr returns the listen address string.
func (c *HermesConfig) GetListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetWorkDir returns the resolved work directory.
// Falls back to current directory if not set.
func (c *HermesConfig) GetWorkDir() string {
	if c.WorkDir != "" && c.WorkDir != "." {
		return c.WorkDir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// GetPlatformWorkDir returns the work directory for a specific platform.
// Priority: platform work_dir → global work_dir → cwd
func (c *HermesConfig) GetPlatformWorkDir(platform string) string {
	switch platform {
	case "wechat":
		if c.Wechat.WorkDir != "" {
			return c.Wechat.WorkDir
		}
	case "feishu":
		if c.Feishu.WorkDir != "" {
			return c.Feishu.WorkDir
		}
	}
	return c.GetWorkDir()
}

// GetWechatCredPath returns the wechat credentials path.
func (c *HermesConfig) GetWechatCredPath() string {
	if c.Wechat.CredPath != "" {
		return c.Wechat.CredPath
	}
	return filepath.Join(config.ConfigDir(), "wechat-credentials.json")
}

// InitHermesConfig creates a hermes.json config template.
// If project is true, writes to .vibe/hermes.json; otherwise <GLOBAL_DIR>/hermes.json.
func InitHermesConfig(project, force bool) (string, error) {
	var path string
	if project {
		path = ProjectHermesConfigPath()
	} else {
		path = HermesConfigPath()
	}

	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, fmt.Errorf("hermes.json already exists: %s", path)
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create directory %s: %w", dir, err)
	}

	var cfg *HermesConfig
	if project {
		// Project template: only fields typically overridden per-project
		cfg = &HermesConfig{
			Memory: MemoryConfig{Enabled: true},
			Agent: AgentConfig{
				MaxTurns:        90,
				BudgetPressure:  true,
				ContextPressure: true,
			},
			WorkDir: ".",
		}
	} else {
		cfg = DefaultHermesConfig()
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return path, nil
}

// InitWebhookConfig adds sample webhook routes to the hermes config.
// If the config file already exists, it merges webhook routes into it.
// If not, it creates a new config with webhook routes included.
// The returned path is the config file that was written.
func InitWebhookConfig(project, force bool) (string, error) {
	var path string
	if project {
		path = ProjectHermesConfigPath()
	} else {
		path = HermesConfigPath()
	}

	// Load existing config or start from defaults
	cfg := DefaultHermesConfig()
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return "", fmt.Errorf("parse existing config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read config %s: %w", path, err)
	}

	// Check if webhook routes already exist
	if len(cfg.Webhooks.Routes) > 0 && !force {
		return path, fmt.Errorf("webhook routes already exist in %s (use --force to overwrite)", path)
	}

	// Add sample webhook configuration
	cfg.Webhooks = WebhookConfig{
		Enabled: true,
		Secret:  "${WEBHOOK_SECRET}",
		Routes: []WebhookRoute{
			{
				Path:     "/github",
				Events:   []string{"push", "pull_request", "issues"},
				Skill:    "code-review",
				Delivery: "",
			},
			{
				Path:     "/ci",
				Events:   []string{"*"},
				Skill:    "ci-monitor",
				Delivery: "",
			},
		},
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return path, nil
}

// resolveEnvVars resolves ${VAR} references in string fields.
func (c *HermesConfig) resolveEnvVars() {
	c.Server.AuthToken = resolveEnv(c.Server.AuthToken)
	c.Feishu.AppID = resolveEnv(c.Feishu.AppID)
	c.Feishu.AppSecret = resolveEnv(c.Feishu.AppSecret)
	c.Webhooks.Secret = resolveEnv(c.Webhooks.Secret)
}

// GetDefaultProvider returns the effective default provider.
// Priority: HermesConfig → Settings
func (c *HermesConfig) GetDefaultProvider(settingsProvider string) string {
	if c.DefaultProvider != "" {
		return c.DefaultProvider
	}
	return settingsProvider
}

// GetDefaultModel returns the effective default model.
// Priority: HermesConfig → Settings
func (c *HermesConfig) GetDefaultModel(settingsModel string) string {
	if c.DefaultModel != "" {
		return c.DefaultModel
	}
	return settingsModel
}

// resolveEnv resolves a single ${VAR} reference.
func resolveEnv(s string) string {
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		envName := s[2 : len(s)-1]
		if v := os.Getenv(envName); v != "" {
			return v
		}
	}
	return s
}
