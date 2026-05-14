package tools

import (
	"context"
	"encoding/json"

	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
)

// Tool is the interface that all tools must implement.
type Tool interface {
	// Name returns the tool's name.
	Name() string

	// Description returns a description of what the tool does.
	Description() string

	// PromptSnippet returns a short one-line description for the system prompt's Available tools section.
	PromptSnippet() string

	// PromptGuidelines returns guideline bullets for the system prompt's Guidelines section.
	PromptGuidelines() []string

	// Parameters returns the JSON Schema for the tool's parameters.
	Parameters() json.RawMessage

	// Execute runs the tool with the given parameters.
	Execute(ctx context.Context, params map[string]any) (string, error)
}

// ToolDefinition converts a Tool to a provider.ToolDefinition.
func ToolDefinition(t Tool) provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.Parameters(),
	}
}

// Registry manages available tools.
type Registry struct {
	tools   map[string]Tool
	order   []string
	sandbox sandbox.Sandbox
	workDir string
}

// NewRegistry creates a new tool registry.
func NewRegistry(workDir string, sb sandbox.Sandbox) *Registry {
	return &Registry{
		tools:   make(map[string]Tool),
		workDir: workDir,
		sandbox: sb,
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	name := t.Name()
	r.tools[name] = t
	r.order = append(r.order, name)
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools in order.
func (r *Registry) All() []Tool {
	var result []Tool
	for _, name := range r.order {
		if t, ok := r.tools[name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// Definitions returns tool definitions for all registered tools.
func (r *Registry) Definitions() []provider.ToolDefinition {
	var defs []provider.ToolDefinition
	for _, t := range r.All() {
		defs = append(defs, ToolDefinition(t))
	}
	return defs
}

// GetSandbox returns the registry's sandbox.
func (r *Registry) GetSandbox() sandbox.Sandbox {
	return r.sandbox
}

// GetWorkDir returns the registry's working directory.
func (r *Registry) GetWorkDir() string {
	return r.workDir
}

// SetSandbox updates the sandbox used by tools.
func (r *Registry) SetSandbox(sb sandbox.Sandbox) {
	r.sandbox = sb
}

// RegisterDefaults registers all default tools.
func (r *Registry) RegisterDefaults() {
	r.Register(NewReadTool(r))
	r.Register(NewWriteTool(r))
	r.Register(NewEditTool(r))
	bashTool := NewBashTool(r)
	r.Register(bashTool)
	r.Register(NewJobsTool(r, bashTool))
	r.Register(NewKillTool(r, bashTool))
	r.Register(NewGrepTool(r))
	r.Register(NewFindTool(r))
	r.Register(NewLsTool(r))
}

// ModeTools returns tool definitions appropriate for the given mode.
func (r *Registry) ModeTools(mode string) []provider.ToolDefinition {
	switch mode {
	case "plan":
		// Plan mode: read-only tools
		var defs []provider.ToolDefinition
		for _, t := range r.All() {
			switch t.Name() {
			case "read", "grep", "find", "ls":
				defs = append(defs, ToolDefinition(t))
			}
		}
		return defs
	default:
		// Agent/YOLO: all tools
		return r.Definitions()
	}
}

// ToolSnippets returns prompt snippets for the given tool names.
func (r *Registry) ToolSnippets(toolNames []string) map[string]string {
	snippets := make(map[string]string)
	for _, name := range toolNames {
		if t, ok := r.tools[name]; ok {
			if snippet := t.PromptSnippet(); snippet != "" {
				snippets[name] = snippet
			}
		}
	}
	return snippets
}

// ToolGuidelines returns prompt guidelines for the given tool names.
func (r *Registry) ToolGuidelines(toolNames []string) []string {
	var guidelines []string
	seen := make(map[string]bool)
	for _, name := range toolNames {
		if t, ok := r.tools[name]; ok {
			for _, g := range t.PromptGuidelines() {
				if !seen[g] {
					seen[g] = true
					guidelines = append(guidelines, g)
				}
			}
		}
	}
	return guidelines
}
