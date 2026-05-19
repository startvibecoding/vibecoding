package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/startvibecoding/vibecoding/internal/tools"
)

const mcpProtocolVersion = "2025-11-25"

type mcpServerConfig struct {
	Type    string   `json:"type,omitempty"`
	Name    string   `json:"name"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args"`
	Env     []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env,omitempty"`
}

type mcpClient struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	pending map[string]chan mcpResponse
	mu      sync.Mutex
	wmu     sync.Mutex
	nextID  int64
}

type mcpResponse struct {
	Result json.RawMessage
	Error  *rpcError
}

type mcpToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type mcpListToolsResult struct {
	Tools []mcpToolInfo `json:"tools"`
}

type mcpCallToolResult struct {
	Content []mcpContentBlock `json:"content,omitempty"`
	IsError bool              `json:"isError,omitempty"`
}

type mcpContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Data     string          `json:"data,omitempty"`
	MimeType string          `json:"mimeType,omitempty"`
	JSON     json.RawMessage `json:"json,omitempty"`
}

func connectMCPServers(ctx context.Context, configs []mcpServerConfig, registry *tools.Registry) ([]*mcpClient, error) {
	var clients []*mcpClient
	for _, cfg := range configs {
		client, err := newMCPClient(ctx, cfg)
		if err != nil {
			closeMCPClients(clients)
			return nil, err
		}
		clients = append(clients, client)
		toolInfos, err := client.listTools(ctx)
		if err != nil {
			closeMCPClients(clients)
			return nil, err
		}
		for _, info := range toolInfos {
			registry.Register(newMCPTool(client, info))
		}
	}
	return clients, nil
}

func closeMCPClients(clients []*mcpClient) {
	for _, client := range clients {
		client.Close()
	}
}

func newMCPClient(ctx context.Context, cfg mcpServerConfig) (*mcpClient, error) {
	if cfg.Type != "" && cfg.Type != "stdio" {
		return nil, fmt.Errorf("unsupported MCP transport %q for server %q", cfg.Type, cfg.Name)
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, fmt.Errorf("MCP server name is required")
	}
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("MCP server %q command is required", cfg.Name)
	}
	if !filepath.IsAbs(cfg.Command) {
		return nil, fmt.Errorf("MCP server %q command must be an absolute path", cfg.Name)
	}

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = os.Environ()
	for _, env := range cfg.Env {
		cmd.Env = append(cmd.Env, env.Name+"="+env.Value)
	}
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open MCP stdin for %q: %w", cfg.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open MCP stdout for %q: %w", cfg.Name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start MCP server %q: %w", cfg.Name, err)
	}

	client := &mcpClient{
		name:    cfg.Name,
		cmd:     cmd,
		stdin:   stdin,
		pending: make(map[string]chan mcpResponse),
	}
	go client.readLoop(stdout)
	go func() {
		_ = cmd.Wait()
		client.closePending(fmt.Errorf("MCP server %q exited", cfg.Name))
	}()

	if _, err := client.call(ctx, "initialize", map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "vibecoding",
			"title":   "VibeCoding",
			"version": "dev",
		},
	}); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize MCP server %q: %w", cfg.Name, err)
	}
	if err := client.notify("notifications/initialized", nil); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize MCP server %q: %w", cfg.Name, err)
	}
	return client, nil
}

func (c *mcpClient) listTools(ctx context.Context) ([]mcpToolInfo, error) {
	result, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("list MCP tools for %q: %w", c.name, err)
	}
	var out mcpListToolsResult
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("decode MCP tools for %q: %w", c.name, err)
	}
	return out.Tools, nil
}

func (c *mcpClient) callTool(ctx context.Context, name string, args map[string]any) (mcpCallToolResult, error) {
	result, err := c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return mcpCallToolResult{}, err
	}
	var out mcpCallToolResult
	if err := json.Unmarshal(result, &out); err != nil {
		return mcpCallToolResult{}, err
	}
	if out.IsError {
		return out, fmt.Errorf("%s", mcpContentToText(out.Content))
	}
	return out, nil
}

func (c *mcpClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	key := fmt.Sprintf("%d", id)
	ch := make(chan mcpResponse, 1)

	c.mu.Lock()
	c.pending[key] = ch
	c.mu.Unlock()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	if err := c.writeMessage(msg); err != nil {
		c.removePending(key)
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.removePending(key)
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("%s", resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *mcpClient) notify(method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	return c.writeMessage(msg)
}

func (c *mcpClient) writeMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := c.stdin.Write(data); err != nil {
		return err
	}
	_, err = c.stdin.Write([]byte("\n"))
	return err
}

func (c *mcpClient) readLoop(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var msg rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if len(msg.ID) == 0 || len(msg.Method) > 0 {
			continue
		}
		key := rawIDKey(msg.ID)
		c.mu.Lock()
		ch, ok := c.pending[key]
		if ok {
			delete(c.pending, key)
		}
		c.mu.Unlock()
		if ok {
			resp := mcpResponse{Result: msg.Result}
			if len(msg.Error) > 0 {
				var rpcErr rpcError
				if err := json.Unmarshal(msg.Error, &rpcErr); err == nil {
					resp.Error = &rpcErr
				} else {
					resp.Error = &rpcError{Code: -32000, Message: string(msg.Error)}
				}
			}
			ch <- resp
		}
	}
	c.closePending(fmt.Errorf("MCP server %q output closed", c.name))
}

func (c *mcpClient) removePending(key string) {
	c.mu.Lock()
	delete(c.pending, key)
	c.mu.Unlock()
}

func (c *mcpClient) closePending(err error) {
	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[string]chan mcpResponse)
	c.mu.Unlock()
	for _, ch := range pending {
		ch <- mcpResponse{Error: &rpcError{Code: -32000, Message: err.Error()}}
	}
}

func (c *mcpClient) Close() {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

func rawIDKey(id json.RawMessage) string {
	return strings.Trim(string(id), "\"")
}

type mcpTool struct {
	client *mcpClient
	info   mcpToolInfo
	name   string
}

func newMCPTool(client *mcpClient, info mcpToolInfo) tools.Tool {
	return &mcpTool{
		client: client,
		info:   info,
		name:   "mcp_" + sanitizeToolName(client.name) + "_" + sanitizeToolName(info.Name),
	}
}

func (t *mcpTool) Name() string {
	return t.name
}

func (t *mcpTool) Description() string {
	if t.info.Description != "" {
		return t.info.Description
	}
	return "Tool provided by MCP server " + t.client.name
}

func (t *mcpTool) PromptSnippet() string {
	return fmt.Sprintf("%s: MCP tool %q from server %q", t.name, t.info.Name, t.client.name)
}

func (t *mcpTool) PromptGuidelines() []string {
	return nil
}

func (t *mcpTool) Parameters() json.RawMessage {
	if len(t.info.InputSchema) == 0 {
		return json.RawMessage(`{"type":"object"}`)
	}
	return t.info.InputSchema
}

func (t *mcpTool) Execute(ctx context.Context, params map[string]any) (tools.ToolResult, error) {
	result, err := t.client.callTool(ctx, t.info.Name, params)
	text := mcpContentToText(result.Content)
	if text == "" && err != nil {
		text = err.Error()
	}
	return tools.NewTextToolResult(text), err
}

func sanitizeToolName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "tool"
	}
	return out
}

func mcpContentToText(blocks []mcpContentBlock) string {
	var parts []string
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "image", "audio":
			parts = append(parts, fmt.Sprintf("[%s content: %s]", block.Type, block.MimeType))
		default:
			data, _ := json.Marshal(block)
			if len(data) > 0 {
				parts = append(parts, string(data))
			}
		}
	}
	return strings.Join(parts, "\n")
}
