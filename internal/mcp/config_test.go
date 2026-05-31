package mcp

import (
	"testing"

	"github.com/startvibecoding/vibecoding/internal/config"
)

func TestIsTemplateServer(t *testing.T) {
	cases := []struct {
		name string
		srv  config.MCPServer
		want bool
	}{
		{
			name: "real stdio",
			srv:  config.MCPServer{Name: "local", Type: "stdio", Command: "/usr/local/bin/mcp-server"},
		},
		{
			name: "empty name",
			srv:  config.MCPServer{Type: "stdio", Command: "/usr/local/bin/mcp-server"},
			want: true,
		},
		{
			name: "placeholder command",
			srv:  config.MCPServer{Name: "example", Type: "stdio", Command: "/absolute/path/to/mcp-server"},
			want: true,
		},
		{
			name: "placeholder url",
			srv:  config.MCPServer{Name: "example", Type: "http", URL: "https://mcp.example.com"},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTemplateServer(tc.srv); got != tc.want {
				t.Fatalf("isTemplateServer() = %v, want %v", got, tc.want)
			}
		})
	}
}
