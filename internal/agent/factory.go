package agent

import (
	"os"
	"path/filepath"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

// AgentFactory creates Agent instances with consistent configuration.
type AgentFactory struct {
	provider           provider.Provider
	model              *provider.Model
	settings           *config.Settings
	sandboxMgr         *sandbox.Manager
	extraContext        string
	compactionSettings ctxpkg.CompactionSettings
	approvalHandler    func(toolCallID, toolName string, args map[string]any) bool
}

// NewAgentFactory creates a factory with shared configuration.
func NewAgentFactory(
	provider provider.Provider,
	model *provider.Model,
	settings *config.Settings,
	sandboxMgr *sandbox.Manager,
	extraContext string,
	compactionSettings ctxpkg.CompactionSettings,
	approvalHandler func(toolCallID, toolName string, args map[string]any) bool,
) *AgentFactory {
	return &AgentFactory{
		provider:           provider,
		model:              model,
		settings:           settings,
		sandboxMgr:         sandboxMgr,
		extraContext:        extraContext,
		compactionSettings: compactionSettings,
		approvalHandler:    approvalHandler,
	}
}

// AgentOptions specifies per-agent overrides.
type AgentOptions struct {
	ID                agentpkg.AgentID
	ParentID          agentpkg.AgentID
	Mode              string
	Model             *provider.Model
	WorkDir           string
	Tools             []string // optional: tool filter
	SystemPromptExtra string   // extra context for this agent
	MaxIterations     int
	ToolExecutionMode string
	Session           *session.Manager
	ApprovalHandler   func(toolCallID, toolName string, args map[string]any) bool // per-agent approval override
}

// Create creates a new Agent with per-agent Registry.
// Each agent gets its own Registry (with its own workDir, sandbox, JobManager).
func (f *AgentFactory) Create(opts AgentOptions) agentpkg.Agent {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	mode := opts.Mode
	if mode == "" {
		mode = "agent"
	}

	model := opts.Model
	if model == nil {
		model = f.model
	}

	maxIterations := opts.MaxIterations
	if maxIterations == 0 {
		maxIterations = 200
	}

	toolExecMode := opts.ToolExecutionMode
	if toolExecMode == "" {
		toolExecMode = "parallel"
	}

	// Create per-agent Registry with isolated workDir/sandbox/JobManager
	sb := f.sandboxForMode(mode)
	registry := tools.NewRegistryWithConfig(tools.RegistryConfig{
		WorkDir:    workDir,
		Sandbox:    sb,
		ToolFilter: opts.Tools,
	})

	// Decision 5: Sub-agents cannot spawn sub-agents
	// Remove subagent_* tools from sub-agent registries
	if opts.ParentID != "" {
		registry.Remove("subagent_spawn")
		registry.Remove("subagent_status")
		registry.Remove("subagent_send")
		registry.Remove("subagent_destroy")
	}

	// Build extra context: factory-level + per-agent
	extraContext := f.extraContext
	if opts.ParentID != "" {
		extraContext += "\n" + BuildSubAgentContext()
	}
	if opts.SystemPromptExtra != "" {
		extraContext += "\n" + opts.SystemPromptExtra
	}

	// Determine session
	sess := opts.Session
	if sess == nil {
		sess = f.defaultSession(workDir)
	}

	cfg := Config{
		ID:       opts.ID,
		ParentID: opts.ParentID,
		Provider: f.provider,
		Model:    model,
		Mode:     mode,
		ThinkingLevel: func() provider.ThinkingLevel {
			if f.settings != nil {
				return provider.ThinkingLevel(f.settings.DefaultThinkingLevel)
			}
			return provider.ThinkingLevel(agentpkg.ThinkingMedium)
		}(),
		MaxTokens: func() int {
			if f.settings != nil && f.settings.MaxOutputTokens > 0 {
				return f.settings.MaxOutputTokens
			}
			return 16384
		}(),
		SandboxMgr:         f.sandboxMgr,
		Settings:           f.settings,
		Session:            sess,
		ExtraContext:        extraContext,
		CompactionSettings: f.compactionSettings,
		ApprovalHandler: func() func(toolCallID, toolName string, args map[string]any) bool {
			if opts.ApprovalHandler != nil {
				return opts.ApprovalHandler
			}
			return f.approvalHandler
		}(),
		MultiAgent: opts.ParentID == "",
	}

	loopCfg := AgentLoopConfig{
		Config:            cfg,
		ToolExecutionMode: toolExecMode,
		MaxIterations:     maxIterations,
	}

	a := NewWithLoopConfig(loopCfg, registry)
	return NewAgentAdapter(a)
}

// CreateFromPublicOptions creates an agent from public Builder options.
func (f *AgentFactory) CreateFromPublicOptions(b *agentpkg.Builder) agentpkg.Agent {
	// This is called by the public Builder's Build() method via buildInternal.
	// Extract options from Builder and delegate to Create.
	// For now, use defaults — the Builder fields are accessed via the builder's internal state.
	return f.Create(AgentOptions{})
}

// sandboxForMode returns the appropriate sandbox for the given mode.
func (f *AgentFactory) sandboxForMode(mode string) sandbox.Sandbox {
	if f.sandboxMgr == nil {
		return sandbox.NewNoneSandbox()
	}
	switch mode {
	case "plan":
		return f.sandboxMgr.GetActive()
	case "agent":
		return f.sandboxMgr.GetActive()
	case "yolo":
		return sandbox.NewNoneSandbox()
	default:
		return f.sandboxMgr.GetActive()
	}
}

// defaultSession creates a default session manager for the given work directory.
func (f *AgentFactory) defaultSession(workDir string) *session.Manager {
	sessionDir := ""
	if f.settings != nil {
		sessionDir = f.settings.GetSessionDir()
	}
	if sessionDir == "" {
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "."
		}
		sessionDir = filepath.Join(home, ".vibecoding", "sessions")
	}
	return session.New(workDir, sessionDir)
}

// Provider returns the factory's provider (for Builder integration).
func (f *AgentFactory) Provider() provider.Provider { return f.provider }

// Settings returns the factory's settings.
func (f *AgentFactory) Settings() *config.Settings { return f.settings }

// --- Register the internal builder with the public agent package ---

func init() {
	agentpkg.SetBuilderFunc(buildFromPublicBuilder)
}

// buildFromPublicBuilder converts a public Builder into an internal Agent.
// This bridges the public agent.Builder API to the internal Agent implementation.
func buildFromPublicBuilder(b *agentpkg.Builder) (agentpkg.Agent, error) {
	cfg := b.Config()

	// Adapt the public Provider to the internal provider.Provider interface
	internalProvider := NewProviderAdapter(cfg.Provider)

	// Resolve the model from the provider
	model := internalProvider.GetModel(cfg.ModelID)
	if model == nil {
		// If the model is not found, create a minimal model entry
		model = &provider.Model{
			ID:   cfg.ModelID,
			Name: cfg.ModelID,
		}
	}

	// Build compaction settings
	compactionSettings := ctxpkg.CompactionSettings{
		Enabled:       cfg.CompactionEnabled,
		ReserveTokens: cfg.CompactionReserve,
	}
	if compactionSettings.ReserveTokens == 0 {
		compactionSettings.ReserveTokens = 16384
	}

	// Build sandbox
	var sandboxMgr *sandbox.Manager
	if cfg.SandboxEnabled {
		sandboxMgr = sandbox.NewManager(cfg.WorkDir)
	}

	// Build session
	var sess *session.Manager
	if cfg.SessionDir != "" {
		sess = session.New(cfg.WorkDir, cfg.SessionDir)
	}

	// Build the tool registry
	var sb sandbox.Sandbox
	if sandboxMgr != nil {
		sb = sandboxMgr.GetActive()
	} else {
		sb = sandbox.NewNoneSandbox()
	}
	registry := tools.NewRegistryWithConfig(tools.RegistryConfig{
		WorkDir:    cfg.WorkDir,
		Sandbox:    sb,
		ToolFilter: cfg.Tools,
	})

	agentCfg := Config{
		Provider:           internalProvider,
		Model:              model,
		Mode:               cfg.Mode,
		ThinkingLevel:      provider.ThinkingLevel(cfg.ThinkingLevel),
		MaxTokens:          cfg.MaxTokens,
		SandboxMgr:         sandboxMgr,
		Session:            sess,
		ExtraContext:        cfg.SystemPromptExtra,
		CompactionSettings: compactionSettings,
		ApprovalHandler:    cfg.ApprovalHandler,
		MultiAgent:         cfg.MultiAgent,
	}

	loopCfg := AgentLoopConfig{
		Config:            agentCfg,
		ToolExecutionMode: cfg.ToolExecutionMode,
		MaxIterations:     cfg.MaxIterations,
	}

	a := NewWithLoopConfig(loopCfg, registry)
	return NewAgentAdapter(a), nil
}
