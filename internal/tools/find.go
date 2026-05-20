package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/vendored"
)

// FindTool searches for files by name pattern using fd.
type FindTool struct {
	registry *Registry
}

// NewFindTool creates a new find tool.
func NewFindTool(r *Registry) *FindTool {
	return &FindTool{registry: r}
}

func (t *FindTool) Name() string { return "find" }

func (t *FindTool) Description() string {
	return "Search for files by name pattern. Supports glob patterns. Use for finding files by name, extension, or path pattern."
}

func (t *FindTool) PromptSnippet() string {
	return "Find files by glob pattern (preferred for locating files, respects .gitignore)"
}

func (t *FindTool) PromptGuidelines() []string {
	return nil
}

func (t *FindTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match file names (e.g. '*.go', '*.test.*')"
			},
			"path": {
				"type": "string",
				"description": "Directory to search in (default: current directory)"
			},
			"maxDepth": {
				"type": "integer",
				"description": "Maximum directory depth (default: unlimited)"
			},
			"maxResults": {
				"type": "integer",
				"description": "Maximum number of results (default 100)"
			}
		},
		"required": ["pattern"]
	}`)
}

// globToRegex 将 glob 模式转换为正则表达式
// 例如: *.go → \.go$, *.test.* → \.test\..*
func globToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.':
			result.WriteString("\\.")
		case '{':
			// 处理 {a,b} 这种模式
			result.WriteString("(?:")
		case '}':
			result.WriteString(")")
		case ',':
			// 在 {a,b} 内部的逗号
			result.WriteString("|")
		default:
			// 转义特殊正则字符
			if strings.ContainsRune(`\+^${}|[]()`, rune(c)) {
				result.WriteByte('\\')
			}
			result.WriteByte(c)
		}
	}

	result.WriteString("$")
	return result.String()
}

func (t *FindTool) Execute(ctx context.Context, params map[string]any) (ToolResult, error) {
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

	maxDepth := -1
	if v, ok := params["maxDepth"].(float64); ok && v > 0 {
		maxDepth = int(v)
	}

	maxResults := 100
	if v, ok := params["maxResults"].(float64); ok && v > 0 {
		maxResults = int(v)
	}

	// 获取 fd 路径
	fdPath := vendored.FdPath()
	if fdPath == "" {
		return ToolResult{}, fmt.Errorf("fd 未安装，请先运行 make prepare-vendored")
	}

	// 将 glob 模式转为正则
	regexPattern := globToRegex(pattern)
	// 验证正则是否有效
	if _, err := regexp.Compile(regexPattern); err != nil {
		return ToolResult{}, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	// 构建 fd 命令参数
	args := []string{
		"--color=never",
		fmt.Sprintf("--max-results=%d", maxResults),
	}

	if maxDepth >= 0 {
		args = append(args, fmt.Sprintf("--max-depth=%d", maxDepth))
	}

	// fd 使用正则匹配
	args = append(args, "--", regexPattern, searchPath)

	// 执行 fd
	cmd := exec.CommandContext(ctx, fdPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// fd 返回 1 表示没有匹配
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return NewTextToolResult("(no files found)"), nil
		}
		// 其他错误
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return ToolResult{}, fmt.Errorf("fd 执行失败: %s", errMsg)
		}
		return ToolResult{}, fmt.Errorf("fd 执行失败: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return NewTextToolResult("(no files found)"), nil
	}

	// fd 输出就是每行一个路径，与原实现格式一致
	return NewTextToolResult(output), nil
}
