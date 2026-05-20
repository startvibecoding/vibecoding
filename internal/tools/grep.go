package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/vendored"
)

// GrepTool searches file contents using ripgrep (rg).
type GrepTool struct {
	registry *Registry
}

// NewGrepTool creates a new grep tool.
func NewGrepTool(r *Registry) *GrepTool {
	return &GrepTool{registry: r}
}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search file contents using regex patterns. Returns matching lines with file paths and line numbers. Use for finding code patterns, function definitions, etc."
}

func (t *GrepTool) PromptSnippet() string {
	return "Search file contents for patterns (preferred for code search, respects .gitignore)"
}

func (t *GrepTool) PromptGuidelines() []string {
	return nil
}

func (t *GrepTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Regex pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "Directory or file to search in (default: current directory)"
			},
			"include": {
				"type": "string",
				"description": "File pattern to include (e.g. '*.go')"
			},
			"maxResults": {
				"type": "integer",
				"description": "Maximum number of results (default 100)"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GrepTool) Execute(ctx context.Context, params map[string]any) (ToolResult, error) {
	pattern, _ := params["pattern"].(string)
	if pattern == "" {
		return ToolResult{}, fmt.Errorf("pattern is required")
	}

	searchPath := t.registry.GetWorkDir()
	if v, ok := params["path"].(string); ok && v != "" {
		var err error
		searchPath, err = t.registry.ResolvePath(v)
		if err != nil {
			return ToolResult{}, fmt.Errorf("invalid path: %w", err)
		}
	}

	include, _ := params["include"].(string)
	maxResults := 100
	if v, ok := params["maxResults"].(float64); ok && v > 0 {
		maxResults = int(v)
	}

	// 获取 rg 路径
	rgPath := vendored.RgPath()
	if rgPath == "" {
		return ToolResult{}, fmt.Errorf("ripgrep (rg) 未安装，请先运行 make prepare-vendored")
	}

	// 构建 rg 命令参数
	args := []string{
		"--no-heading",
		"--line-number",
		"--color=never",
		fmt.Sprintf("--max-count=%d", maxResults),
	}

	if include != "" {
		args = append(args, "-g", include)
	}

	args = append(args, "--", pattern, searchPath)

	// 执行 rg
	cmd := exec.CommandContext(ctx, rgPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// rg 返回 1 表示没有匹配，这不是错误
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return NewTextToolResult("(no matches found)"), nil
		}
		// 其他错误
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return ToolResult{}, fmt.Errorf("rg 执行失败: %s", errMsg)
		}
		return ToolResult{}, fmt.Errorf("rg 执行失败: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return NewTextToolResult("(no matches found)"), nil
	}

	// rg 默认输出格式: file:line:content
	// 与原实现格式一致: file:line: content
	return NewTextToolResult(output), nil
}
