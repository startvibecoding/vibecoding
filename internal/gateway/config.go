package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/config"
)

// GatewayConfig holds all gateway-specific configuration.
type GatewayConfig struct {
	Listen               string               `json:"listen,omitempty"`
	Auth                 AuthConfig            `json:"auth"`
	DefaultMode          string                `json:"defaultMode,omitempty"`
	DefaultThinkingLevel string                `json:"defaultThinkingLevel,omitempty"`
	EnableSubAgents      bool                  `json:"enableSubAgents,omitempty"`
	Sandbox              GatewaySandboxConfig  `json:"sandbox"`
	AllowedWorkDirs      *[]string             `json:"allowedWorkDirs,omitempty"` // nil=no check, []=deny all overrides
	Session              SessionConfig         `json:"session"`
	WorkingDir           string                `json:"workingDir,omitempty"`
	CORS                 CORSConfig            `json:"cors"`
	Provider             string                `json:"provider,omitempty"`
	Model                string                `json:"model,omitempty"`
	ToolVisibility       ToolVisibilityConfig  `json:"toolVisibility"`
	SystemPromptMode     string                `json:"systemPromptMode,omitempty"` // "append" (default), "ignore"
	RequestTimeoutSecs   int                   `json:"requestTimeoutSeconds,omitempty"`
	MaxConcurrentReqs    int                   `json:"maxConcurrentRequests,omitempty"`
	LogLevel             string                `json:"logLevel,omitempty"`
}

// AuthConfig controls bearer token authentication.
type AuthConfig struct {
	Enabled bool     `json:"enabled"`
	Tokens  []string `json:"tokens,omitempty"`
}

// GatewaySandboxConfig controls sandbox for gateway mode.
type GatewaySandboxConfig struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level,omitempty"` // "none", "standard", "strict"; empty=auto from mode
}

// SessionConfig controls session pool behavior.
type SessionConfig struct {
	IdleTimeoutSeconds int `json:"idleTimeoutSeconds,omitempty"`
	MaxSessions        int `json:"maxSessions,omitempty"`
}

// CORSConfig controls cross-origin resource sharing.
type CORSConfig struct {
	Enabled      bool     `json:"enabled"`
	AllowOrigins []string `json:"allowOrigins,omitempty"`
}

// ToolVisibilityConfig controls how tool calls are exposed to the client.
type ToolVisibilityConfig struct {
	// Mode controls the transport for tool status:
	//   "content" (default) — tool output mixed into content stream
	//   "sse_event" — tool output via separate SSE events
	//   "none" — no tool output
	Mode string `json:"mode,omitempty"`

	// Detail controls the verbosity of tool output in content mode:
	//   "collapsed" (default) — one-line summary: 🔧 `read` main.go
	//                           edit always shows path + diff
	//   "expanded" — full output with code fences (Ctrl+O style)
	Detail string `json:"detail,omitempty"`
}

// DefaultGatewayConfig returns the default gateway configuration.
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		Listen:               ":8080",
		Auth:                 AuthConfig{Enabled: false},
		DefaultMode:          "yolo",
		DefaultThinkingLevel: "medium",
		EnableSubAgents:      false,
		Sandbox:              GatewaySandboxConfig{Enabled: false},
		Session:              SessionConfig{IdleTimeoutSeconds: 1800},
		CORS:                 CORSConfig{Enabled: false, AllowOrigins: []string{"*"}},
		ToolVisibility:       ToolVisibilityConfig{Mode: "content", Detail: "collapsed"},
		SystemPromptMode:     "append",
		RequestTimeoutSecs:   1800,
		LogLevel:             "info",
	}
}

// GatewayConfigPath returns the path to the global gateway.json.
func GatewayConfigPath() string {
	return filepath.Join(config.ConfigDir(), "gateway.json")
}

// ProjectGatewayConfigPath returns the path to the project-level gateway.json.
func ProjectGatewayConfigPath() string {
	return filepath.Join(".vibe", "gateway.json")
}

// LoadGatewayConfig loads the gateway configuration, merging global + project.
// Priority: .vibe/gateway.json > ~/.config/vibecoding/gateway.json > defaults
func LoadGatewayConfig() (*GatewayConfig, error) {
	cfg, err := LoadGatewayConfigFrom(GatewayConfigPath())
	if err != nil {
		return nil, err
	}
	// Overlay project-level config
	projectPath := ProjectGatewayConfigPath()
	if data, err := os.ReadFile(projectPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse project gateway config %s: %w", projectPath, err)
		}
	}
	normalizeConfig(cfg)
	return cfg, nil
}

// LoadGatewayConfigFrom loads gateway configuration from a specific path (no project merge).
func LoadGatewayConfigFrom(path string) (*GatewayConfig, error) {
	cfg := DefaultGatewayConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // use defaults
		}
		return nil, fmt.Errorf("read gateway config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse gateway config: %w", err)
	}

	normalizeConfig(cfg)
	return cfg, nil
}

// normalizeConfig fills in defaults for empty fields.
func normalizeConfig(cfg *GatewayConfig) {
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = "yolo"
	}
	if cfg.ToolVisibility.Mode == "" {
		cfg.ToolVisibility.Mode = "content"
	}
	if cfg.ToolVisibility.Detail == "" {
		cfg.ToolVisibility.Detail = "collapsed"
	}
	if cfg.SystemPromptMode == "" {
		cfg.SystemPromptMode = "append"
	}
	if cfg.RequestTimeoutSecs <= 0 {
		cfg.RequestTimeoutSecs = 1800
	}
}

// SaveGatewayConfig writes the configuration to the given path.
func SaveGatewayConfig(path string, cfg *GatewayConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gateway config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// InitGatewayConfig creates the gateway.json template at the default location.
// Returns the file path. If force is false and the file already exists, returns an error.
func InitGatewayConfig(force bool) (string, error) {
	path := GatewayConfigPath()
	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, fmt.Errorf("gateway.json already exists: %s", path)
		}
	}
	cfg := DefaultGatewayConfig()
	// Add example tokens and allowedWorkDirs for the template
	cfg.Auth.Tokens = []string{"sk-change-me-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/home/user"
	}
	exampleDirs := []string{filepath.Join(home, "projects")}
	cfg.AllowedWorkDirs = &exampleDirs
	cfg.WorkingDir = filepath.Join(home, "projects")

	if err := SaveGatewayConfig(path, cfg); err != nil {
		return "", err
	}
	return path, nil
}

// GetListenAddr returns the effective listen address.
func (c *GatewayConfig) GetListenAddr() string {
	if c.Listen != "" {
		return c.Listen
	}
	return ":8080"
}

// GetWorkDir returns the effective working directory.
func (c *GatewayConfig) GetWorkDir() string {
	if c.WorkingDir != "" {
		if strings.HasPrefix(c.WorkingDir, "~") {
			home, _ := os.UserHomeDir()
			if home != "" {
				return filepath.Join(home, c.WorkingDir[1:])
			}
		}
		return c.WorkingDir
	}
	cwd, _ := os.Getwd()
	return cwd
}

// GetToolDetail returns the effective tool detail level.
func (c *GatewayConfig) GetToolDetail() string {
	if c.ToolVisibility.Detail != "" {
		return c.ToolVisibility.Detail
	}
	return "collapsed"
}

// ValidateWorkDir checks if the given directory is allowed by the allowedWorkDirs whitelist.
// Returns nil if allowed, an error describing the violation otherwise.
func (c *GatewayConfig) ValidateWorkDir(dir string) error {
	// nil AllowedWorkDirs = no restriction
	if c.AllowedWorkDirs == nil {
		return nil
	}
	allowed := *c.AllowedWorkDirs
	// empty list = deny all overrides
	if len(allowed) == 0 {
		return fmt.Errorf("x_working_dir overrides are disabled")
	}

	cleanDir := filepath.Clean(dir)
	for _, a := range allowed {
		cleanAllowed := filepath.Clean(a)
		if cleanDir == cleanAllowed {
			return nil
		}
		// Prefix match with path separator boundary
		prefix := cleanAllowed + string(filepath.Separator)
		if strings.HasPrefix(cleanDir, prefix) {
			return nil
		}
	}
	return fmt.Errorf("directory %q is not in allowedWorkDirs", dir)
}
