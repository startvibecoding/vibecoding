package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/startvibecoding/vibecoding/internal/acp"
	"github.com/startvibecoding/vibecoding/internal/a2a"
	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/contextfiles"
	"github.com/startvibecoding/vibecoding/internal/cron"
	"github.com/startvibecoding/vibecoding/internal/gateway"
	"github.com/startvibecoding/vibecoding/internal/mcp"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/skills"
	"github.com/startvibecoding/vibecoding/internal/tools"
	"github.com/startvibecoding/vibecoding/internal/tui"
)

var version = "dev"

func main() {
	rootCmd := newRootCommand(run, acp.Run)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand(runFn func([]string, runOptions) error, acpRunFn func(acp.RunOptions) error) *cobra.Command {
	var (
		flagProvider          string
		flagModel             string
		flagMode              string
		flagThinking          string
		flagContinue          bool
		flagResume            string
		flagSession           string
		flagSandbox           bool
		flagPrint             bool
		flagVerbose           bool
		flagDebug             bool
		flagMultiAgent        bool
		flagInitGateway       bool
		flagForce             bool
		flagEnableA2AMaster   bool
		flagInitA2AMaster     bool
	)

	rootCmd := &cobra.Command{
		Use:     "vibecoding [message...]",
		Aliases: []string{"vc"},
		Short:   "VibeCoding - AI coding assistant",
		Long:    "VibeCoding is an AI-powered coding assistant that runs in your terminal.\nSupports OpenAI and Anthropic APIs with sandboxed execution.",
		Version: version,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagInitA2AMaster {
				path, err := a2a.InitA2AMasterConfig(flagForce)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Created a2a master config: %s\n", path)
				return nil
			}
			if flagInitGateway {
				path, err := gateway.InitGatewayConfig(flagForce)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Created gateway config: %s\n", path)
				return nil
			}
			return runFn(args, runOptions{
				provider:        flagProvider,
				model:           flagModel,
				mode:            flagMode,
				thinking:        flagThinking,
				continue_:       flagContinue,
				resume:          flagResume,
				session:         flagSession,
				sandbox:         flagSandbox,
				print:           flagPrint,
				verbose:         flagVerbose,
				debug:           flagDebug,
				multiAgent:      flagMultiAgent,
				enableA2AMaster: flagEnableA2AMaster,
			})
		},
	}

	acpCmd := &cobra.Command{
		Use:   "acp",
		Short: "Run the Agent Client Protocol server",
		Long:  "Run vibecoding as an ACP-compliant stdio agent.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return acpRunFn(acp.RunOptions{
				Provider:   flagProvider,
				Model:      flagModel,
				Mode:       flagMode,
				Thinking:   flagThinking,
				Sandbox:    flagSandbox,
				Verbose:    flagVerbose,
				Debug:      flagDebug,
				MultiAgent: flagMultiAgent,
			})
		},
	}

	flags := rootCmd.Flags()
	flags.StringVarP(&flagProvider, "provider", "p", "", "Provider (openai, anthropic, or custom provider name)")
	flags.StringVarP(&flagModel, "model", "m", "", "Model ID")
	flags.StringVarP(&flagMode, "mode", "M", "", "Mode (plan, agent, yolo)")
	flags.StringVarP(&flagThinking, "thinking", "t", "", "Thinking level (off, minimal, low, medium, high, xhigh)")
	flags.BoolVarP(&flagContinue, "continue", "c", false, "Continue most recent session")
	flags.StringVarP(&flagResume, "resume", "r", "", "Resume session by ID or path")
	flags.StringVar(&flagSession, "session", "", "Use specific session file or ID")
	flags.BoolVar(&flagSandbox, "sandbox", false, "Enable sandbox (bwrap) for secure execution")
	flags.BoolVarP(&flagPrint, "print", "P", false, "Print response and exit (non-interactive)")
	flags.BoolVar(&flagVerbose, "verbose", false, "Verbose output")
	flags.BoolVar(&flagDebug, "debug", false, "Enable debug logging")
	flags.BoolVar(&flagMultiAgent, "multi-agent", false, "Enable multi-agent mode (sub-agent tools)")
	flags.BoolVar(&flagInitGateway, "init-gateway", false, "Create gateway.json config template")
	flags.BoolVar(&flagForce, "force", false, "Force overwrite existing files (used with --init-*)")
	flags.BoolVar(&flagEnableA2AMaster, "enable-a2a-master", false, "Enable A2A master mode (dispatch tasks to remote agents)")
	flags.BoolVar(&flagInitA2AMaster, "init-a2a-master-config", false, "Create a2a-list.json config template")

	acpFlags := acpCmd.Flags()
	acpFlags.StringVarP(&flagProvider, "provider", "p", "", "Provider (openai, anthropic, or custom provider name)")
	acpFlags.StringVarP(&flagModel, "model", "m", "", "Model ID")
	acpFlags.StringVarP(&flagMode, "mode", "M", "", "Mode (plan, agent, yolo)")
	acpFlags.StringVarP(&flagThinking, "thinking", "t", "", "Thinking level (off, minimal, low, medium, high, xhigh)")
	acpFlags.BoolVar(&flagSandbox, "sandbox", false, "Enable sandbox (bwrap) for secure execution")
	acpFlags.BoolVar(&flagVerbose, "verbose", false, "Verbose output")
	acpFlags.BoolVar(&flagDebug, "debug", false, "Enable debug logging")
	acpFlags.BoolVar(&flagMultiAgent, "multi-agent", false, "Enable multi-agent mode (sub-agent tools)")

	var (
		flagGatewayPort    string
		flagGatewayConfig  string
		flagGatewayWorkDir string
	)

	gatewayCmd := &cobra.Command{
		Use:   "gateway",
		Short: "Run the OpenAI-compatible HTTP gateway",
		Long:  "Start VibeCoding as an HTTP server exposing a standard OpenAI Chat Completions API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return gateway.Run(gateway.RunOptions{
				ConfigPath: flagGatewayConfig,
				Port:       flagGatewayPort,
				Provider:   flagProvider,
				Model:      flagModel,
				WorkDir:    flagGatewayWorkDir,
				Sandbox:    flagSandbox,
				MultiAgent: flagMultiAgent,
				Verbose:    flagVerbose,
				Debug:      flagDebug,
			}, version)
		},
	}

	gatewayFlags := gatewayCmd.Flags()
	gatewayFlags.StringVar(&flagGatewayPort, "port", "", "Listen port (default: from gateway.json or 8080)")
	gatewayFlags.StringVar(&flagGatewayConfig, "config", "", "Path to gateway.json")
	gatewayFlags.StringVar(&flagGatewayWorkDir, "work-dir", "", "Default working directory")
	gatewayFlags.StringVarP(&flagProvider, "provider", "p", "", "Provider (openai, anthropic, or custom provider name)")
	gatewayFlags.StringVarP(&flagModel, "model", "m", "", "Model ID")
	gatewayFlags.BoolVar(&flagSandbox, "sandbox", false, "Enable sandbox (bwrap) for secure execution")
	gatewayFlags.BoolVar(&flagMultiAgent, "multi-agent", false, "Enable multi-agent mode (sub-agent tools)")
	gatewayFlags.BoolVar(&flagVerbose, "verbose", false, "Verbose output")
	gatewayFlags.BoolVar(&flagDebug, "debug", false, "Enable debug logging")

	rootCmd.AddCommand(acpCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(newHermesCommand())
	rootCmd.AddCommand(newA2ACommand())
	return rootCmd
}

type runOptions struct {
	provider        string
	model           string
	mode            string
	thinking        string
	continue_       bool
	resume          string
	session         string
	sandbox         bool
	print           bool
	verbose         bool
	debug           bool
	multiAgent      bool
	enableA2AMaster bool
}

func run(args []string, opts runOptions) error {
	// Set Windows console to UTF-8 so CJK IME works correctly.
	if err := initConsole(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: init console: %v\n", err)
	}

	// Enable debug logging if requested
	debugEnabled = opts.debug
	if debugEnabled {
		fmt.Fprintf(os.Stderr, "[DEBUG] Debug logging enabled\n")
	}

	// Enable config verbose logging
	config.Verbose = opts.verbose || opts.debug
	if opts.debug {
		_ = os.Setenv("VIBECODING_DEBUG", "1")
	}

	// Load settings
	settings, err := config.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Load context files (before provider creation so user always sees what's loaded)
	var contextStr string
	var contextFilesInfo string
	if settings.ContextFiles.Enabled {
		cfResult := contextfiles.LoadContextFiles(cwd, config.ConfigDir(), settings.ContextFiles.ExtraFiles)
		contextStr = contextfiles.BuildContextString(cfResult)
		if contextStr != "" {
			// Build context files info string for TUI
			var sb strings.Builder
			sb.WriteString("📄 Loaded context files:\n")
			for _, f := range cfResult.GlobalFiles {
				sb.WriteString(fmt.Sprintf("  ✓ %s (global)\n", f.Name))
			}
			for _, f := range cfResult.ParentFiles {
				sb.WriteString(fmt.Sprintf("  ✓ %s (parent: %s)\n", f.Name, filepath.Dir(f.Path)))
			}
			for _, f := range cfResult.ProjectFiles {
				sb.WriteString(fmt.Sprintf("  ✓ %s (project)\n", f.Name))
			}
			contextFilesInfo = sb.String()
		}
	}

	// Determine provider
	providerName := opts.provider
	if providerName == "" {
		providerName = settings.DefaultProvider
	}

	// Determine model
	modelID := opts.model
	if modelID == "" {
		modelID = settings.DefaultModel
	}

	// Create provider from config
	p, model, err := createProvider(settings, providerName, modelID)
	if err != nil {
		return err
	}

	// Determine mode
	mode := opts.mode
	if mode == "" {
		mode = settings.DefaultMode
	}
	if mode == "" {
		mode = "agent"
	}

	// Determine thinking level
	thinkingLevel := opts.thinking
	if thinkingLevel == "" {
		thinkingLevel = settings.DefaultThinkingLevel
	}

	// Load skills
	skillsMgr := skills.NewManager(settings.GetGlobalSkillsDir(), filepath.Join(cwd, ".skills"))
	if err := skillsMgr.Load(); err != nil && opts.verbose {
		fmt.Fprintf(os.Stderr, "Warning: load skills: %v\n", err)
	}
	skillsContext := skillsMgr.BuildAllSkillsContext()
	if opts.verbose && skillsContext != "" {
		fmt.Fprintf(os.Stderr, "Loaded %d skills\n", len(skillsMgr.List()))
	}

	// Setup sandbox
	sbMgr := sandbox.NewManager(cwd)

	// Sandbox is disabled by default, enabled via --sandbox flag or config
	sbEnabled := opts.sandbox || settings.Sandbox.Enabled

	if !sbEnabled {
		sbMgr.SetLevel(sandbox.LevelNone)
	} else {
		var targetLevel sandbox.Level
		switch mode {
		case "plan":
			targetLevel = sandbox.LevelStrict
		case "yolo":
			targetLevel = sandbox.LevelNone
		default:
			targetLevel = sandbox.LevelStandard
		}
		// When the user explicitly passed --sandbox, verify the requested level
		// is actually available before allowing silent fallback to none.
		if opts.sandbox && targetLevel != sandbox.LevelNone {
			if _, err := sbMgr.GetForLevel(targetLevel); err != nil {
				return fmt.Errorf("sandbox requested but unavailable: %w", err)
			}
		}
		if err := sbMgr.SetLevel(targetLevel); err != nil {
			if opts.sandbox {
				return fmt.Errorf("sandbox requested but unavailable: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Warning: sandbox unavailable, continuing without: %v\n", err)
			sbMgr.SetLevel(sandbox.LevelNone)
		}
	}
	sbInfo := sandbox.FormatSandboxInfo(sbMgr.GetActive())

	// Setup session
	var sess *session.Manager
	var sessionInfo string
	if opts.continue_ {
		sess, err = session.ContinueRecent(cwd, settings.GetSessionDir())
		if err != nil {
			return fmt.Errorf("continue session: %w", err)
		}
		if sess.GetHeader() != nil {
			sessionInfo = fmt.Sprintf("📂 Continuing session: %s", sess.GetFile())
			if messages := sess.GetMessages(); len(messages) > 0 {
				sessionInfo += fmt.Sprintf(" (%d messages)", len(messages))
			}
		}
	} else if opts.session != "" {
		sess, err = session.OpenByPathOrID(cwd, settings.GetSessionDir(), opts.session)
		if err != nil {
			return fmt.Errorf("open session: %w", err)
		}
		sessionInfo = fmt.Sprintf("📂 Opened session: %s", sess.GetFile())
	} else if opts.resume != "" {
		sess, err = session.OpenByPathOrID(cwd, settings.GetSessionDir(), opts.resume)
		if err != nil {
			return fmt.Errorf("resume session: %w", err)
		}
		sessionInfo = fmt.Sprintf("📂 Resumed session: %s", sess.GetFile())
	} else {
		sess = session.New(cwd, settings.GetSessionDir())
		if err := sess.Init(); err != nil {
			return fmt.Errorf("init session: %w", err)
		}
	}

	// Setup tools
	registry := tools.NewRegistry(cwd, sbMgr.GetActive())
	registry.RegisterDefaultsWithPlanTool(settings.IsPlanToolEnabled())

	// Register skill reference tool if skills are available
	if skillsMgr != nil {
		registry.Register(tools.NewSkillRefTool(skillsMgr))
	}

	mcpServers, err := mcp.LoadConfiguredServers(cwd)
	if err != nil {
		return err
	}
	mcpClients, err := mcp.ConnectServers(context.Background(), mcpServers, registry, mcp.Callbacks{})
	if err != nil {
		return fmt.Errorf("connect MCP servers: %w", err)
	}
	defer mcp.CloseClients(mcpClients)

	// Build extra system context
	extraContext := contextStr + skillsContext

	// A2A master mode: load agent list and register dispatch tool
	if opts.enableA2AMaster {
		// Try project-level first, then global
		a2aListPath := a2a.ProjectAgentListConfigPath()
		if _, err := os.Stat(a2aListPath); err != nil {
			a2aListPath = a2a.AgentListConfigPath()
		}
		a2aListCfg, err := a2a.LoadAgentList(a2aListPath)
		if err != nil {
			return fmt.Errorf("load a2a-list.json: %w", err)
		}
		a2aMgr := a2a.NewA2AManager(a2aListCfg)
		registry.Register(tools.NewA2ADispatchTool(&a2aDispatcherAdapter{mgr: a2aMgr}))
		if opts.verbose {
			fmt.Fprintf(os.Stderr, "A2A master mode enabled: %d agents loaded from %s\n", len(a2aMgr.List()), a2aListPath)
		}
	}

	// Multi-agent mode: create AgentFactory and AgentManager, register subagent tools
	var agentMgr *agent.AgentManager
	var cronStore cron.CronStore
	var cronScheduler *cron.Scheduler
	if opts.multiAgent {
		compactionSettings := ctxpkg.CompactionSettings{
			Enabled:          settings.Compaction.Enabled,
			ReserveTokens:    settings.Compaction.ReserveTokens,
			KeepRecentTokens: settings.Compaction.KeepRecentTokens,
		}
		if compactionSettings.ReserveTokens == 0 {
			compactionSettings.ReserveTokens = 16384
		}
		if compactionSettings.KeepRecentTokens == 0 {
			compactionSettings.KeepRecentTokens = 20000
		}

		factory := agent.NewAgentFactory(p, model, settings, sbMgr, extraContext, compactionSettings, nil)
		agentMgr = agent.NewAgentManager(factory)

		// Register subagent tools
		registry.Register(agent.NewSubAgentSpawnTool(agentMgr))
		registry.Register(agent.NewSubAgentStatusTool(agentMgr))
		registry.Register(agent.NewSubAgentSendTool(agentMgr))
		registry.Register(agent.NewSubAgentDestroyTool(agentMgr))

		// Create cron store, scheduler, and tool
		cronPath := filepath.Join(config.ConfigDir(), "cron.json")
		cronStore = cron.NewFileCronStore(cronPath)
		cronScheduler = cron.NewScheduler(cronStore, agentMgr, 30*time.Second)
		cronScheduler.Start()
		registry.Register(cron.NewCronTool(cronStore, cronScheduler))
		defer cronScheduler.Stop()

		if opts.verbose {
			fmt.Fprintf(os.Stderr, "Multi-agent mode enabled\n")
		}
	}

	// Print mode: non-interactive
	if opts.print {
		return runPrint(args, p, model, mode, provider.ThinkingLevel(thinkingLevel), settings, registry, sess, extraContext, opts.multiAgent, agentMgr)
	}

	// Interactive mode
	// Clear any pending stdin input (e.g., terminal color queries)
	clearStdin()

	app := tui.NewApp(p, model, settings, sess, registry, sbInfo, extraContext, skillsMgr, mode, opts.multiAgent, agentMgr, cronStore, cronScheduler)
	// Add context files info and session info as initial message
	var initialMsg string
	if contextFilesInfo != "" {
		initialMsg = contextFilesInfo
	}
	if sessionInfo != "" {
		if initialMsg != "" {
			initialMsg += "\n"
		}
		initialMsg += sessionInfo
	}
	if initialMsg != "" {
		app.SetInitialMessage(initialMsg)
	}
	p2 := tea.NewProgram(app, teaProgramOptions()...)
	app.SetProgram(p2)
	if _, err := p2.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	return nil
}

// a2aDispatcherAdapter adapts a2a.A2AManager to tools.A2ADispatcher.
type a2aDispatcherAdapter struct {
	mgr *a2a.A2AManager
}

func (a *a2aDispatcherAdapter) List() []tools.AgentEntry {
	entries := a.mgr.List()
	result := make([]tools.AgentEntry, len(entries))
	for i, e := range entries {
		result[i] = tools.AgentEntry{Name: e.Name, URL: e.URL}
	}
	return result
}

func (a *a2aDispatcherAdapter) Dispatch(ctx context.Context, name, message string) (string, error) {
	return a.mgr.Dispatch(ctx, name, message)
}
