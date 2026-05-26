package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// Builder provides a fluent API for creating Agent instances.
// External developers use this to instantiate the built-in Agent without
// depending on internal packages.
//
// Usage:
//
//	a, err := agent.NewBuilder().
//	    WithProvider(myProvider).
//	    WithModel("gpt-4").
//	    WithMode("yolo").
//	    WithWorkDir("/home/user/project").
//	    Build()
type Builder struct {
	provider          Provider
	modelID           string
	mode              string
	workDir           string
	thinkingLevel     ThinkingLevel
	maxTokens         int
	systemPromptExtra string
	maxIterations     int
	toolExecutionMode string
	tools             []string
	sandboxEnabled    bool
	sessionDir        string
	compactionEnabled bool
	compactionReserve int
	multiAgent        bool
	approvalHandler   func(toolCallID, toolName string, args map[string]any) bool
}

// NewBuilder creates a new Builder with sensible defaults.
func NewBuilder() *Builder {
	return &Builder{
		mode:              "agent",
		thinkingLevel:     ThinkingMedium,
		maxTokens:         16384,
		maxIterations:     200,
		toolExecutionMode: "parallel",
		compactionEnabled: true,
		compactionReserve: 16384,
	}
}

// WithProvider sets the LLM provider.
func (b *Builder) WithProvider(p Provider) *Builder {
	b.provider = p
	return b
}

// WithModel sets the model ID.
func (b *Builder) WithModel(modelID string) *Builder {
	b.modelID = modelID
	return b
}

// WithMode sets the agent mode: "plan", "agent", or "yolo".
func (b *Builder) WithMode(mode string) *Builder {
	b.mode = mode
	return b
}

// WithWorkDir sets the working directory.
func (b *Builder) WithWorkDir(dir string) *Builder {
	b.workDir = dir
	return b
}

// WithThinkingLevel sets the thinking/reasoning level.
func (b *Builder) WithThinkingLevel(level ThinkingLevel) *Builder {
	b.thinkingLevel = level
	return b
}

// WithMaxTokens sets the maximum output tokens.
func (b *Builder) WithMaxTokens(n int) *Builder {
	b.maxTokens = n
	return b
}

// WithSystemPromptExtra adds extra context to the system prompt.
func (b *Builder) WithSystemPromptExtra(extra string) *Builder {
	b.systemPromptExtra = extra
	return b
}

// WithMaxIterations sets the safety limit for agent loop iterations.
func (b *Builder) WithMaxIterations(n int) *Builder {
	b.maxIterations = n
	return b
}

// WithToolExecutionMode sets how tool calls are executed: "sequential" or "parallel".
func (b *Builder) WithToolExecutionMode(mode string) *Builder {
	b.toolExecutionMode = mode
	return b
}

// WithTools sets a filter for available tools. Empty means all tools.
func (b *Builder) WithTools(tools []string) *Builder {
	b.tools = tools
	return b
}

// WithSandbox enables or disables sandboxing.
func (b *Builder) WithSandbox(enabled bool) *Builder {
	b.sandboxEnabled = enabled
	return b
}

// WithSessionDir sets the session persistence directory.
func (b *Builder) WithSessionDir(dir string) *Builder {
	b.sessionDir = dir
	return b
}

// WithCompaction configures context compaction.
func (b *Builder) WithCompaction(enabled bool, reserveTokens int) *Builder {
	b.compactionEnabled = enabled
	b.compactionReserve = reserveTokens
	return b
}

// WithMultiAgent enables multi-agent mode.
func (b *Builder) WithMultiAgent(enabled bool) *Builder {
	b.multiAgent = enabled
	return b
}

// WithApprovalHandler sets a custom approval handler for tool calls.
func (b *Builder) WithApprovalHandler(h func(toolCallID, toolName string, args map[string]any) bool) *Builder {
	b.approvalHandler = h
	return b
}

// Build creates and returns an Agent instance.
// Returns an error if required fields are missing.
func (b *Builder) Build() (Agent, error) {
	if b.provider == nil {
		return nil, fmt.Errorf("agent: provider is required (use WithProvider)")
	}
	if b.workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("agent: get working directory: %w", err)
		}
		b.workDir = wd
	}
	if b.modelID == "" {
		models := b.provider.Models()
		if len(models) == 0 {
			return nil, fmt.Errorf("agent: no models available from provider %q", b.provider.Name())
		}
		b.modelID = models[0].ID
	}
	if b.sessionDir == "" {
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "."
		}
		b.sessionDir = filepath.Join(home, ".vibecoding", "sessions")
	}

	// Delegate to internal builder
	return buildInternal(b)
}

// buildInternal is set by internal/agent/init.go to avoid import cycles.
// The internal package calls agent.SetBuilderFunc() at init time.
var buildInternal func(b *Builder) (Agent, error)

// SetBuilderFunc registers the internal builder function.
// Called by internal/agent package at init time.
func SetBuilderFunc(fn func(b *Builder) (Agent, error)) {
	buildInternal = fn
}
