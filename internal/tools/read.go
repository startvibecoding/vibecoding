package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadTool reads file contents.
type ReadTool struct {
	registry *Registry
}

// NewReadTool creates a new read tool.
func NewReadTool(r *Registry) *ReadTool {
	return &ReadTool{registry: r}
}

func (t *ReadTool) Name() string { return "read" }

func (t *ReadTool) Description() string {
	return "Read the contents of a file. Supports text files and images (jpg, png, gif, webp). For text files, output is truncated at 2000 lines or 50KB. Use offset/limit for large files."
}

func (t *ReadTool) PromptSnippet() string {
	return "Read file contents"
}

func (t *ReadTool) PromptGuidelines() []string {
	return []string{"Use read to examine files instead of cat or sed."}
}

func (t *ReadTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file to read"
			},
			"offset": {
				"type": "integer",
				"description": "Line number to start reading from (1-indexed)"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of lines to read"
			}
		},
		"required": ["path"]
	}`)
}

// imageMimeType maps file extensions to MIME types.
var imageMimeType = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

func (t *ReadTool) Execute(ctx context.Context, params map[string]any) (ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return ToolResult{}, fmt.Errorf("path is required")
	}

	path, err := t.registry.ResolvePath(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("invalid path: %w", err)
	}

	// Check for image files
	ext := strings.ToLower(filepath.Ext(path))
	if mimeType, ok := imageMimeType[ext]; ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return ToolResult{}, fmt.Errorf("cannot read image file: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		desc := fmt.Sprintf("[Image file: %s, size: %d bytes, type: %s]", path, len(data), mimeType)
		return NewImageToolResult(desc, mimeType, b64), nil
	}

	// Read text file
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("cannot read file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	offset := 0
	if v, ok := params["offset"].(float64); ok && v > 0 {
		offset = int(v) - 1 // Convert to 0-indexed
	}

	limit := len(lines)
	if v, ok := params["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	// Clamp
	if offset >= len(lines) {
		return NewTextToolResult("(end of file)"), nil
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	selected := lines[offset:end]

	// Number lines
	var sb strings.Builder
	for i, line := range selected {
		lineNum := offset + i + 1
		sb.WriteString(fmt.Sprintf("%d\t%s\n", lineNum, line))
	}

	result := sb.String()

	// Truncate
	const maxBytes = 50000
	if len(result) > maxBytes {
		result = result[:maxBytes] + fmt.Sprintf("\n... (truncated, total %d lines)", len(lines))
	}

	return NewTextToolResult(result), nil
}

