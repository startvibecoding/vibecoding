package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMCPPathHelpers(t *testing.T) {
	if filepath.Base(GlobalMCPPath()) != "mcp.json" {
		t.Fatalf("unexpected global MCP path: %s", GlobalMCPPath())
	}
	if ProjectMCPPath() != filepath.Join(".vibe", "mcp.json") {
		t.Fatalf("unexpected project MCP path: %s", ProjectMCPPath())
	}
}

func TestSaveLoadMCPConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "mcp.json")
	cfg := &MCPConfig{
		MCPServers: []MCPServer{
			{Name: "s1", Type: "stdio", Command: "/tmp/mcp"},
		},
	}
	if err := SaveMCPConfig(path, cfg); err != nil {
		t.Fatalf("save MCP config: %v", err)
	}
	got, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("load MCP config: %v", err)
	}
	if len(got.MCPServers) != 1 || got.MCPServers[0].Name != "s1" {
		t.Fatalf("unexpected MCP config: %#v", got)
	}
}

func TestNormalizeMCPConfig(t *testing.T) {
	cfg := &MCPConfig{
		MCPServers: []MCPServer{
			{Name: " a ", Type: ""},
		},
	}
	NormalizeMCPConfig(cfg)
	if cfg.MCPServers[0].Name != "a" {
		t.Fatalf("name not trimmed: %q", cfg.MCPServers[0].Name)
	}
	if cfg.MCPServers[0].Type != "stdio" {
		t.Fatalf("type default not applied: %q", cfg.MCPServers[0].Type)
	}
}

func TestFullMCPConfigTemplate(t *testing.T) {
	cfg := FullMCPConfigTemplate()
	if cfg == nil || len(cfg.MCPServers) < 3 {
		t.Fatalf("expected full template with >=3 servers, got %#v", cfg)
	}
	var hasStdio, hasHTTP, hasSSE bool
	for _, s := range cfg.MCPServers {
		switch s.Type {
		case "stdio":
			hasStdio = true
		case "http":
			hasHTTP = true
		case "sse":
			hasSSE = true
		}
	}
	if !hasStdio || !hasHTTP || !hasSSE {
		t.Fatalf("missing transport in full template: stdio=%v http=%v sse=%v", hasStdio, hasHTTP, hasSSE)
	}
}

func TestLoadMCPConfigNotFound(t *testing.T) {
	_, err := LoadMCPConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not exists error, got: %v", err)
	}
}
