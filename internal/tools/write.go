package tools

import (
	"context"
	"encoding/json"
	"fmt"
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
	content, _ := params["content"].(string)

	if path == "" {
		return ToolResult{}, fmt.Errorf("path is required")
	}

	path, err := t.registry.ResolvePath(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("invalid path: %w", err)
	}

	// Write file atomically, preserving existing permissions
	if err := writeFileAtomic(path, []byte(content)); err != nil {
		return ToolResult{}, fmt.Errorf("write file: %w", err)
	}

	return NewTextToolResult(fmt.Sprintf("File written: %s (%d bytes)", path, len(content))), nil
}

