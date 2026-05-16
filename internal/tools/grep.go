package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool searches file contents using regex patterns.
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
	return "Search file contents for patterns (respects .gitignore)"
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

	re, err := regexp.Compile(pattern)
	if err != nil {
		return ToolResult{}, fmt.Errorf("invalid regex: %w", err)
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

	var results []string
	count := 0

	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || count >= maxResults {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".vibe" {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by include pattern
		if include != "" {
			matched, _ := filepath.Match(include, info.Name())
			if !matched {
				return nil
			}
		}

		// Skip binary files
		if isBinary(path) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				relPath, _ := filepath.Rel(searchPath, path)
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, lineNum, strings.TrimSpace(line)))
				count++
				if count >= maxResults {
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		return ToolResult{}, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return NewTextToolResult("(no matches found)"), nil
	}

	return NewTextToolResult(strings.Join(results, "\n")), nil
}


func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return true
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}
