package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MCPServer defines one MCP server entry in mcp.json.
type MCPServer struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Command    string `json:"command,omitempty"`
	URL        string `json:"url,omitempty"`
	MessageURL string `json:"messageUrl,omitempty"`
	Args       []string `json:"args,omitempty"`
	Headers    []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers,omitempty"`
	Env []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env,omitempty"`
}

// MCPConfig is the standalone MCP configuration file schema.
type MCPConfig struct {
	MCPServers []MCPServer `json:"mcpServers,omitempty"`
}

// GlobalMCPPath returns the global mcp.json path.
func GlobalMCPPath() string {
	return filepath.Join(ConfigDir(), "mcp.json")
}

// ProjectMCPPath returns the project-local mcp.json path.
func ProjectMCPPath() string {
	return filepath.Join(".vibe", "mcp.json")
}

// LoadMCPConfig reads and parses mcp.json from path.
func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse MCP config: %w", err)
	}
	return &cfg, nil
}

// SaveMCPConfig writes mcp.json to path.
func SaveMCPConfig(path string, cfg *MCPConfig) error {
	if cfg == nil {
		cfg = &MCPConfig{}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create MCP config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal MCP config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}
	return nil
}

// DefaultMCPConfig returns a starter mcp.json template.
func DefaultMCPConfig() *MCPConfig {
	return &MCPConfig{
		MCPServers: []MCPServer{
			{
				Name:    "example-stdio",
				Type:    "stdio",
				Command: "/absolute/path/to/mcp-server",
			},
		},
	}
}

// FullMCPConfigTemplate returns a comprehensive multi-transport template.
func FullMCPConfigTemplate() *MCPConfig {
	return &MCPConfig{
		MCPServers: []MCPServer{
			{
				Name:    "local-stdio",
				Type:    "stdio",
				Command: "/absolute/path/to/mcp-server",
				Args:    []string{"--port", "8080"},
				Env: []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{
					{Name: "API_KEY", Value: "replace-me"},
				},
			},
			{
				Name: "remote-http",
				Type: "http",
				URL:  "https://mcp.example.com",
				Headers: []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{
					{Name: "Authorization", Value: "Bearer replace-me"},
				},
			},
			{
				Name:       "legacy-sse",
				Type:       "sse",
				URL:        "https://legacy.example.com/sse",
				MessageURL: "https://legacy.example.com/messages",
				Headers: []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{
					{Name: "Authorization", Value: "Bearer replace-me"},
				},
			},
		},
	}
}

// NormalizeMCPConfig applies basic defaults.
func NormalizeMCPConfig(cfg *MCPConfig) {
	if cfg == nil {
		return
	}
	for i := range cfg.MCPServers {
		cfg.MCPServers[i].Name = strings.TrimSpace(cfg.MCPServers[i].Name)
		cfg.MCPServers[i].Type = strings.TrimSpace(cfg.MCPServers[i].Type)
		if cfg.MCPServers[i].Type == "" {
			cfg.MCPServers[i].Type = "stdio"
		}
	}
}
