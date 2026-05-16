package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindTool searches for files by name pattern.
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
	return "Find files by glob pattern (respects .gitignore)"
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

	var results []string
	count := 0

	depth := 0
	filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || count >= maxResults {
			return nil
		}

		// Track depth for directories
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".vibe" {
				return filepath.SkipDir
			}
			relPath, _ := filepath.Rel(searchPath, path)
			if relPath == "." {
				depth = 0
			} else {
				depth = strings.Count(relPath, string(os.PathSeparator)) + 1
			}
			if maxDepth >= 0 && depth > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}

		matched, _ := filepath.Match(pattern, info.Name())
		if !matched {
			// Also try matching the full relative path
			relPath, _ := filepath.Rel(searchPath, path)
			matched, _ = filepath.Match(pattern, relPath)
		}

		if matched {
			relPath, _ := filepath.Rel(searchPath, path)
			results = append(results, relPath)
			count++
		}

		return nil
	})

	if len(results) == 0 {
		return NewTextToolResult("(no files found)"), nil
	}

	return NewTextToolResult(strings.Join(results, "\n")), nil
}

