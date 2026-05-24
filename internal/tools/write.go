package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// WriteTool writes content to files.
type WriteTool struct {
	registry *Registry
}

// NewWriteTool creates a new write tool.
func NewWriteTool(r *Registry) *WriteTool {
	return &WriteTool{registry: r}
}

func (t *WriteTool) Name() string { return "write" }

func (t *WriteTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories."
}

func (t *WriteTool) PromptSnippet() string {
	return "Create or overwrite files"
}

func (t *WriteTool) PromptGuidelines() []string {
	return []string{"Use write only for new files or complete rewrites."}
}

func (t *WriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "Content to write to the file"
			}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteTool) Execute(ctx context.Context, params map[string]any) (ToolResult, error) {
	path, _ := params["path"].(string)
	content, ok := params["content"].(string)
	if !ok {
		return ToolResult{}, fmt.Errorf("content is required")
	}

	if path == "" {
		return ToolResult{}, fmt.Errorf("path is required")
	}

	path, err := t.registry.ResolvePath(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("invalid path: %w", err)
	}

	oldContent := ""
	if data, err := os.ReadFile(path); err == nil {
		oldContent = string(data)
	}

	// Write file atomically, preserving existing permissions
	if err := writeFileAtomic(path, []byte(content)); err != nil {
		return ToolResult{}, fmt.Errorf("write file: %w", err)
	}

	return NewTextToolResult(fmt.Sprintf("File written: %s (%d bytes)\n%s", path, len(content), formatWriteDiffSummary(oldContent, content))), nil
}

func formatWriteDiffSummary(oldContent, newContent string) string {
	deleted, added := diffLineChanges(splitDiffLines(oldContent), splitDiffLines(newContent))
	return fmt.Sprintf("Diff: +%d -%d\n- lines: %s\n+ lines: %s",
		len(added),
		len(deleted),
		formatLineRanges(deleted),
		formatLineRanges(added),
	)
}

func splitDiffLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func diffLineChanges(oldLines, newLines []string) ([]int, []int) {
	if len(oldLines) == 0 && len(newLines) == 0 {
		return nil, nil
	}
	if len(oldLines)*len(newLines) > 200000 {
		return allLineNumbers(len(oldLines)), allLineNumbers(len(newLines))
	}

	lcs := make([][]int, len(oldLines)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var deleted, added []int
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		switch {
		case oldLines[i] == newLines[j]:
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			deleted = append(deleted, i+1)
			i++
		default:
			added = append(added, j+1)
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		deleted = append(deleted, i+1)
	}
	for ; j < len(newLines); j++ {
		added = append(added, j+1)
	}
	return deleted, added
}

func allLineNumbers(count int) []int {
	lines := make([]int, count)
	for i := range lines {
		lines[i] = i + 1
	}
	return lines
}

func formatLineRanges(lines []int) string {
	if len(lines) == 0 {
		return "none"
	}
	var ranges []string
	start, prev := lines[0], lines[0]
	for _, line := range lines[1:] {
		if line == prev+1 {
			prev = line
			continue
		}
		ranges = append(ranges, formatLineRange(start, prev))
		start, prev = line, line
	}
	ranges = append(ranges, formatLineRange(start, prev))
	return strings.Join(ranges, ",")
}

func formatLineRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}
