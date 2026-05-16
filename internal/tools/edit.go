package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// EditTool performs precise text replacements in files.
type EditTool struct {
	registry *Registry
}

// NewEditTool creates a new edit tool.
func NewEditTool(r *Registry) *EditTool {
	return &EditTool{registry: r}
}

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Edit a file using exact text replacement. Each edit must match a unique, non-overlapping region of the file. For multiple changes to the same file, use multiple edits in one call."
}

func (t *EditTool) PromptSnippet() string {
	return "Make precise file edits with exact text replacement, including multiple disjoint edits in one call"
}

func (t *EditTool) PromptGuidelines() []string {
	return []string{
		"Use edit for precise changes (edits[].oldText must match exactly)",
		"When changing multiple separate locations in one file, use one edit call with multiple entries in edits[] instead of multiple edit calls",
		"Each edits[].oldText is matched against the original file, not after earlier edits are applied. Do not emit overlapping or nested edits. Merge nearby changes into one edit.",
		"Keep edits[].oldText as small as possible while still being unique in the file. Do not pad with large unchanged regions.",
	}
}

func (t *EditTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file to edit"
			},
			"edits": {
				"type": "array",
				"description": "Array of edits. Each edit has oldText (exact match) and newText (replacement).",
				"items": {
					"type": "object",
					"properties": {
						"oldText": {
							"type": "string",
							"description": "Exact text to find and replace"
						},
						"newText": {
							"type": "string",
							"description": "Replacement text"
						}
					},
					"required": ["oldText", "newText"]
				}
			}
		},
		"required": ["path", "edits"]
	}`)
}

func (t *EditTool) Execute(ctx context.Context, params map[string]any) (ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return ToolResult{}, fmt.Errorf("path is required")
	}

	path, err := t.registry.ResolvePath(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("invalid path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("read file: %w", err)
	}
	content := string(data)

	editsRaw, ok := params["edits"].([]any)
	if !ok || len(editsRaw) == 0 {
		return ToolResult{}, fmt.Errorf("edits array is required and must not be empty")
	}

	type edit struct {
		OldText string
		NewText string
	}

	var edits []edit
	for _, e := range editsRaw {
		editMap, ok := e.(map[string]any)
		if !ok {
			return ToolResult{}, fmt.Errorf("invalid edit format")
		}
		oldText, _ := editMap["oldText"].(string)
		newText, _ := editMap["newText"].(string)
		if oldText == "" {
			return ToolResult{}, fmt.Errorf("oldText is required for each edit")
		}
		edits = append(edits, edit{OldText: oldText, NewText: newText})
	}

	// Validate all edits before applying and record their positions in the original content
	type editPos struct {
		edit  edit
		start int
		end   int
	}
	positions := make([]editPos, 0, len(edits))
	for i, e := range edits {
		count := strings.Count(content, e.OldText)
		if count == 0 {
			return ToolResult{}, fmt.Errorf("edit %d: oldText not found in file", i)
		}
		if count > 1 {
			return ToolResult{}, fmt.Errorf("edit %d: oldText matches %d times (must be unique). Make the match text more specific", i, count)
		}
		start := strings.Index(content, e.OldText)
		positions = append(positions, editPos{edit: e, start: start, end: start + len(e.OldText)})
	}

	// Sort by position to ensure non-overlapping order
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].start < positions[j].start
	})

	// Check for overlapping edits
	for i := 1; i < len(positions); i++ {
		if positions[i].start < positions[i-1].end {
			return ToolResult{}, fmt.Errorf("edit %d and edit %d overlap", i-1, i)
		}
	}

	// Apply edits in sorted order based on original content positions
	var sb strings.Builder
	lastEnd := 0
	for _, pos := range positions {
		sb.WriteString(content[lastEnd:pos.start])
		sb.WriteString(pos.edit.NewText)
		lastEnd = pos.end
	}
	sb.WriteString(content[lastEnd:])
	content = sb.String()

	if err := writeFileAtomic(path, []byte(content)); err != nil {
		return ToolResult{}, fmt.Errorf("write file: %w", err)
	}

	return NewTextToolResult(fmt.Sprintf("Applied %d edit(s) to %s", len(edits), path)), nil
}

