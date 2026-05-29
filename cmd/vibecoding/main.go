package main

import (
	"encoding/json"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"

	"github.com/startvibecoding/vibecoding/internal/acp"
	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/contextfiles"
	"github.com/startvibecoding/vibecoding/internal/cron"
	"github.com/startvibecoding/vibecoding/internal/gateway"
	"github.com/startvibecoding/vibecoding/internal/hermes"
	"github.com/startvibecoding/vibecoding/internal/messaging/wechat"
	"github.com/startvibecoding/vibecoding/internal/mcp"
	"github.com/startvibecoding/vibecoding/internal/provider"
	providerfactory "github.com/startvibecoding/vibecoding/internal/provider/factory"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/skills"
	"github.com/startvibecoding/vibecoding/internal/tools"
	"github.com/startvibecoding/vibecoding/internal/tui"
)

var version = "dev"
var debugEnabled bool

// debugLog prints debug messages to stderr if debug mode is enabled.
func debugLog(format string, args ...interface{}) {
	if debugEnabled {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

func main() {
	rootCmd := newRootCommand(run, acp.Run)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand(runFn func([]string, runOptions) error, acpRunFn func(acp.RunOptions) error) *cobra.Command {
	var (
		flagProvider     string
		flagModel        string
		flagMode         string
		flagThinking     string
		flagContinue     bool
		flagResume       string
		flagSession      string
		flagSandbox      bool
		flagPrint        bool
		flagVerbose      bool
		flagDebug        bool
		flagMultiAgent   bool
		flagInitGateway  bool
		flagForce        bool
	)

	rootCmd := &cobra.Command{
		Use:     "vibecoding [message...]",
		Aliases: []string{"vc"},
		Short:   "VibeCoding - AI coding assistant",
		Long:    "VibeCoding is an AI-powered coding assistant that runs in your terminal.\nSupports OpenAI and Anthropic APIs with sandboxed execution.",
		Version: version,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagInitGateway {
				path, err := gateway.InitGatewayConfig(flagForce)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Created gateway config: %s\n", path)
				return nil
			}
			return runFn(args, runOptions{
				provider:   flagProvider,
				model:      flagModel,
				mode:       flagMode,
				thinking:   flagThinking,
				continue_:  flagContinue,
				resume:     flagResume,
				session:    flagSession,
				sandbox:    flagSandbox,
				print:      flagPrint,
				verbose:    flagVerbose,
				debug:      flagDebug,
				multiAgent: flagMultiAgent,
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
	flags.BoolVar(&flagForce, "force", false, "Force overwrite existing files (used with --init-gateway)")

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
	return rootCmd
}

type runOptions struct {
	provider   string
	model      string
	mode       string
	thinking   string
	continue_  bool
	resume     string
	session    string
	sandbox    bool
	print      bool
	verbose    bool
	debug      bool
	multiAgent bool
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

// createProvider creates a provider from config based on provider name.
func createProvider(settings *config.Settings, providerName, modelID string) (provider.Provider, *provider.Model, error) {
	return providerfactory.Create(settings, providerName, modelID)
}

// clearStdin reads and discards any pending input from stdin.
// This is needed because some terminals send color query sequences on startup.
func clearStdin() {
	// Set a short read deadline so pending reads time out cleanly.
	// Some stdin types (pipes, certain PTYs) don't support deadlines;
	// if SetReadDeadline fails we skip clearing to avoid blocking forever.
	if err := os.Stdin.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		return
	}
	defer os.Stdin.SetReadDeadline(time.Time{}) // Clear deadline
	buf := make([]byte, 128)
	for {
		n, err := os.Stdin.Read(buf)
		if n == 0 || err != nil {
			return
		}
	}
}

func runPrint(args []string, p provider.Provider, model *provider.Model, mode string, thinkingLevel provider.ThinkingLevel, settings *config.Settings, registry *tools.Registry, sess *session.Manager, extraContext string, multiAgent bool, agentMgr *agent.AgentManager) error {
	input := strings.Join(args, " ")
	if input == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("no input provided")
		}
		input = string(data)
	}

	fmt.Fprintf(os.Stderr, "Using %s/%s in %s mode\n", p.Name(), model.ID, mode)

	// Create glamour renderer for markdown
	wordWrap := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		wordWrap = w
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(wordWrap),
	)
	if err != nil {
		debugLog("Failed to create glamour renderer: %v", err)
		renderer = nil
	}

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

	agentCfg := agent.Config{
		Provider:           p,
		Model:              model,
		Mode:               mode,
		ThinkingLevel:      thinkingLevel,
		MaxTokens:          settings.MaxOutputTokens,
		Settings:           settings,
		Session:            sess,
		ExtraContext:       extraContext,
		CompactionSettings: compactionSettings,
		MultiAgent:         multiAgent,
	}

	a := agent.New(agentCfg, registry)
	if multiAgent && agentMgr != nil {
		agentMgr.Register(agent.NewAgentAdapter(a))
	}

	ctx := context.Background()
	eventCh := a.Run(ctx, input)

	var textBuffer strings.Builder

	err = agent.ConsumeEvents(ctx, eventCh, agent.EventHandlerFunc(func(_ context.Context, event agent.Event) error {
		switch event.Type {
		case agent.EventToolApprovalRequest:
			return fmt.Errorf("tool approval required in print mode for %s; rerun interactively, use --mode yolo, or whitelist the command", event.ApprovalTool)
		case agent.EventTextDelta:
			textBuffer.WriteString(event.TextDelta)
		case agent.EventToolCall:
			// Flush text buffer before tool call
			if textBuffer.Len() > 0 {
				flushTextBuffer(&textBuffer, renderer)
			}
			fmt.Fprintf(os.Stderr, "\n[tool: %s]\n", event.ToolCall.Name)
		case agent.EventToolExecutionStart:
			fmt.Fprintf(os.Stderr, "[running: %s] ", event.ToolName)
		case agent.EventToolExecutionEnd:
			if event.ToolError != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", event.ToolError)
			} else {
				fmt.Fprintf(os.Stderr, "done\n")
			}
		case agent.EventToolResult:
			// Show full tool result for bash commands
			if event.ToolName == "bash" {
				fmt.Fprintf(os.Stderr, "\n%s\n", event.ToolResult)
			} else if event.ToolDiff != nil {
				fmt.Fprintf(os.Stderr, "\n[change: %s] +%d -%d (-%s +%s)\n",
					event.ToolDiff.Path,
					event.ToolDiff.Added,
					event.ToolDiff.Deleted,
					formatLineRanges(event.ToolDiff.DeletedLines),
					formatLineRanges(event.ToolDiff.AddedLines),
				)
			}
		case agent.EventPlanUpdate:
			if event.Plan != nil {
				fmt.Fprintf(os.Stderr, "\n%s\n", formatTaskPlan(event.Plan))
			}
		case agent.EventDone:
			// Flush remaining text buffer
			if textBuffer.Len() > 0 {
				flushTextBuffer(&textBuffer, renderer)
			}
			// Show context usage
			if event.ContextUsage != nil && event.ContextUsage.Percent != nil {
				fmt.Fprintf(os.Stderr, "\nContext: %.1f%%/%s\n",
					*event.ContextUsage.Percent,
					formatTokenCount(event.ContextUsage.ContextWindow))
			}
		case agent.EventError:
			// Flush text buffer before error
			if textBuffer.Len() > 0 {
				flushTextBuffer(&textBuffer, renderer)
			}
			if event.Error != nil {
				return event.Error
			}
		case agent.EventUsage:
			if event.ContextUsage != nil && event.ContextUsage.Percent != nil {
				fmt.Fprintf(os.Stderr, "Context: %.1f%%/%s | ",
					*event.ContextUsage.Percent,
					formatTokenCount(event.ContextUsage.ContextWindow))
			}
			if event.Usage != nil {
				cacheInfo := ""
				if info := event.Usage.CacheInfo(); info != "" {
					cacheInfo = " | " + info
				}
				fmt.Fprintf(os.Stderr, "Tokens: %d↓/%d↑ $%.4f%s\n",
					event.Usage.TotalInputTokens(), event.Usage.Output, event.Usage.Cost.Total, cacheInfo)
			}
		case agent.EventCompactionStart:
			fmt.Fprintf(os.Stderr, "\n⏳ Compacting context...\n")
		case agent.EventCompactionEnd:
			if event.Error != nil {
				fmt.Fprintf(os.Stderr, "Compaction failed: %v\n", event.Error)
			} else if event.StatusMessage != "" {
				fmt.Fprintf(os.Stderr, "✅ %s\n", event.StatusMessage)
			} else {
				fmt.Fprintf(os.Stderr, "✅ Context compacted\n")
			}
		}
		return nil
	}))
	if err != nil {
		return err
	}

	return nil
}

func formatTaskPlan(plan *tools.TaskPlan) string {
	if plan == nil || len(plan.Steps) == 0 {
		return "Plan updated."
	}
	var sb strings.Builder
	title := plan.Title
	if title == "" {
		title = "Plan"
	}
	sb.WriteString(title)
	for _, step := range plan.Steps {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("%s %s", planStatusMarker(step.Status), step.Title))
	}
	if plan.Note != "" {
		sb.WriteString("\nnote: " + plan.Note)
	}
	return sb.String()
}

func planStatusMarker(status string) string {
	switch status {
	case "running":
		return ">"
	case "done":
		return "x"
	case "failed":
		return "!"
	default:
		return "-"
	}
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

// flushTextBuffer renders and prints the accumulated text buffer.
func flushTextBuffer(buffer *strings.Builder, renderer *glamour.TermRenderer) {
	text := buffer.String()
	buffer.Reset()

	if renderer != nil {
		rendered, err := renderer.Render(text)
		if err != nil {
			// Fallback to plain text
			fmt.Print(text)
		} else {
			fmt.Print(rendered)
		}
	} else {
		fmt.Print(text)
	}
}

// formatTokenCount formats a token count for display.
func formatTokenCount(count int) string {
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	}
	if count < 10000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	}
	if count < 1000000 {
		return fmt.Sprintf("%dk", count/1000)
	}
	if count < 10000000 {
		return fmt.Sprintf("%.1fM", float64(count)/1000000)
	}
	return fmt.Sprintf("%dM", count/1000000)
}

// --- Hermes subcommand ---

func newHermesCommand() *cobra.Command {
	var (
		flagPort       int
		flagWorkDir    string
		flagConfig     string
		flagProvider   string
		flagModel      string
		flagMultiAgent bool
		flagSandbox    bool
		flagDaemon     bool
		flagVerbose    bool
		flagDebug      bool
		flagForce      bool
		flagProject    bool
		flagGlobal     bool
		flagWebhook    bool
		flagSchedule   string
		flagOneShot    bool
	)

	hermesCmd := &cobra.Command{
		Use:   "hermes",
		Short: "Run the Hermes messaging gateway",
		Long:  "Start VibeCoding Hermes — a messaging gateway with WebSocket/HTTP API, WeChat, Feishu, and more.",
	}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Hermes daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hermes.Run(hermes.RunOptions{
				ConfigPath: flagConfig,
				Port:       flagPort,
				WorkDir:    flagWorkDir,
				Provider:   flagProvider,
				Model:      flagModel,
				MultiAgent: flagMultiAgent,
				Sandbox:    flagSandbox,
				Daemon:     flagDaemon,
				Verbose:    flagVerbose,
				Debug:      flagDebug,
			}, version)
		},
	}

	startFlags := startCmd.Flags()
	startFlags.IntVar(&flagPort, "port", 0, "Listen port (default: from hermes.json or 8090)")
	startFlags.StringVar(&flagWorkDir, "work-dir", "", "Default working directory")
	startFlags.StringVar(&flagConfig, "config", "", "Path to hermes.json")
	startFlags.StringVarP(&flagProvider, "provider", "p", "", "Default provider name (overrides hermes.json)")
	startFlags.StringVarP(&flagModel, "model", "m", "", "Default model ID (overrides hermes.json)")
	startFlags.BoolVar(&flagMultiAgent, "multi-agent", false, "Enable multi-agent mode (sub-agent tools)")
	startFlags.BoolVar(&flagSandbox, "sandbox", false, "Enable sandbox mode (bwrap)")
	startFlags.BoolVarP(&flagDaemon, "daemon", "d", false, "Run in background")
	startFlags.BoolVar(&flagVerbose, "verbose", false, "Verbose output")
	startFlags.BoolVar(&flagDebug, "debug", false, "Enable debug logging")

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Hermes daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "hermes stop: not yet implemented")
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show Hermes daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "hermes status: not yet implemented")
			return nil
		},
	}

	// config subcommand
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Hermes configuration",
	}

	configInitCmd := &cobra.Command{
		Use:   "init",
		Short: "Create hermes.json config template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagProject && flagGlobal {
				return fmt.Errorf("--project and --global are mutually exclusive")
			}
			if flagWebhook {
				path, err := hermes.InitWebhookConfig(flagProject, flagForce)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Created webhook config: %s\n", path)
				fmt.Fprintf(os.Stderr, "\nSample routes:\n")
				fmt.Fprintf(os.Stderr, "  POST /webhook/github  — GitHub events (push, pull_request, issues)\n")
				fmt.Fprintf(os.Stderr, "  POST /webhook/ci      — CI events (all types)\n")
				fmt.Fprintf(os.Stderr, "\nSet WEBHOOK_SECRET env var or replace ${WEBHOOK_SECRET} in config.\n")
				return nil
			}
			path, err := hermes.InitHermesConfig(flagProject, flagForce)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Created hermes config: %s\n", path)
			return nil
		},
	}
	configInitCmd.Flags().BoolVar(&flagProject, "project", false, "Write to .vibe/hermes.json")
	configInitCmd.Flags().BoolVar(&flagGlobal, "global", false, "Write to global hermes.json (default)")
	configInitCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing file")
	configInitCmd.Flags().BoolVar(&flagWebhook, "webhook", false, "Include sample webhook routes (GitHub, CI)")

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current effective configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}

	configCmd.AddCommand(configInitCmd, configShowCmd)

	// client subcommand
	var flagURL string
	var flagSession string

	clientCmd := &cobra.Command{
		Use:   "client",
		Short: "Connect to a running Hermes instance via WebSocket",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "hermes client: not yet implemented")
			return nil
		},
	}
	clientCmd.Flags().StringVar(&flagURL, "url", "ws://localhost:8090/ws", "WebSocket URL to connect to")
	clientCmd.Flags().StringVar(&flagSession, "session", "", "Session ID to resume")

	// wechat subcommand
	wechatCmd := &cobra.Command{
		Use:   "wechat",
		Short: "Manage WeChat iLink connection",
	}

	wechatLoginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to WeChat via QR code",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			credPath := cfg.GetWechatCredPath()
			client := wechat.NewClient()
			_, err = wechat.Login(cmd.Context(), client, wechat.LoginOptions{
				CredPath: credPath,
				Force:    flagForce,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "WeChat credentials saved to %s\n", credPath)
			return nil
		},
	}
	wechatLoginCmd.Flags().BoolVar(&flagForce, "force", false, "Force re-login even if credentials exist")

	wechatStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show WeChat connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			credPath := cfg.GetWechatCredPath()
			creds, err := wechat.LoadCredentials(credPath)
			if err != nil || creds == nil {
				fmt.Fprintln(os.Stderr, "WeChat: not logged in")
				fmt.Fprintf(os.Stderr, "  Run: vibecoding hermes wechat login\n")
				return nil
			}
			fmt.Fprintf(os.Stderr, "WeChat: logged in\n")
			fmt.Fprintf(os.Stderr, "  UserID: %s\n", creds.UserID)
			fmt.Fprintf(os.Stderr, "  AccountID: %s\n", creds.AccountID)
			fmt.Fprintf(os.Stderr, "  SavedAt: %s\n", creds.SavedAt)
			fmt.Fprintf(os.Stderr, "  CredPath: %s\n", credPath)
			return nil
		},
	}

	wechatCmd.AddCommand(wechatLoginCmd, wechatStatusCmd)

	// feishu subcommand
	feishuCmd := &cobra.Command{
		Use:   "feishu",
		Short: "Manage Feishu (Lark) connection",
	}

	feishuSetupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure Feishu app credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "Configure Feishu app credentials in hermes.json:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, `  "feishu": {`)
			fmt.Fprintln(os.Stderr, `    "enabled": true,`)
			fmt.Fprintln(os.Stderr, `    "app_id": "cli_xxxx",`)
			fmt.Fprintln(os.Stderr, `    "app_secret": "xxxx"`)
			fmt.Fprintln(os.Stderr, `  }`)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Or set environment variables: FEISHU_APP_ID, FEISHU_APP_SECRET")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Steps:")
			fmt.Fprintln(os.Stderr, "  1. Go to https://open.feishu.cn → Create App")
			fmt.Fprintln(os.Stderr, "  2. Enable Bot capability")
			fmt.Fprintln(os.Stderr, "  3. Subscribe to im.message.receive_v1 event")
			fmt.Fprintln(os.Stderr, "  4. Copy App ID and App Secret to hermes.json")
			return nil
		},
	}

	feishuStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show Feishu connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := hermes.LoadHermesConfig()
			if err != nil {
				return err
			}
			if !cfg.Feishu.Enabled {
				fmt.Fprintln(os.Stderr, "Feishu: disabled")
				return nil
			}
			if cfg.Feishu.AppID == "" || cfg.Feishu.AppSecret == "" {
				fmt.Fprintln(os.Stderr, "Feishu: enabled but not configured")
				fmt.Fprintln(os.Stderr, "  Run: vibecoding hermes feishu setup")
				return nil
			}
			fmt.Fprintln(os.Stderr, "Feishu: configured")
			fmt.Fprintf(os.Stderr, "  AppID: %s\n", cfg.Feishu.AppID)
			fmt.Fprintf(os.Stderr, "  WorkDir: %s\n", cfg.GetPlatformWorkDir("feishu"))
			return nil
		},
	}

	feishuCmd.AddCommand(feishuSetupCmd, feishuStatusCmd)

	// cron subcommand
	cronCmd := &cobra.Command{
		Use:   "cron",
		Short: "Manage cron scheduled tasks",
	}

	cronListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := openCronStore()
			jobs, err := store.List()
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				fmt.Println("No cron jobs.")
				return nil
			}
			for _, j := range jobs {
				enabled := "✅"
				if !j.Enabled {
					enabled = "⏸"
				}
				kind := "periodic"
				if j.OneShot {
					kind = "one-shot"
				}
				fmt.Printf("%s [%s] %s (%s, %s, runs: %d)\n", enabled, j.ID, j.Name, kind, j.Schedule, j.RunCount)
			}
			return nil
		},
	}

	cronAddCmd := &cobra.Command{
		Use:   "add <name> <prompt>",
		Short: "Add a cron job",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := openCronStore()
			name := args[0]
			prompt := args[1]
			job, err := store.Create(cron.CronJob{
				Name:     name,
				Prompt:   prompt,
				Schedule: flagSchedule,
				OneShot:  flagOneShot,
				Enabled:  true,
				Mode:     "yolo",
			})
			if err != nil {
				return err
			}
			fmt.Printf("✅ Created: [%s] %s\n", job.ID, job.Name)
			return nil
		},
	}
	cronAddCmd.Flags().StringVar(&flagSchedule, "schedule", "", "Schedule: @daily, @weekly, @every 30m, etc.")
	cronAddCmd.Flags().BoolVar(&flagOneShot, "oneshot", false, "One-shot task (auto-disable after first run)")

	cronRemoveCmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := openCronStore()
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("🗑 Removed: %s\n", args[0])
			return nil
		},
	}

	cronEnableCmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setCronEnabled(args[0], true)
		},
	}

	cronDisableCmd := &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setCronEnabled(args[0], false)
		},
	}

	cronCmd.AddCommand(cronListCmd, cronAddCmd, cronRemoveCmd, cronEnableCmd, cronDisableCmd)

	hermesCmd.AddCommand(startCmd, stopCmd, statusCmd, configCmd, clientCmd, wechatCmd, feishuCmd, cronCmd)
	return hermesCmd
}

func openCronStore() *cron.FileCronStore {
	path := filepath.Join(config.ConfigDir(), "hermes-cron.json")
	return cron.NewFileCronStore(path)
}

func setCronEnabled(id string, enabled bool) error {
	store := openCronStore()
	job, err := store.Get(id)
	if err != nil {
		return err
	}
	job.Enabled = enabled
	if err := store.Update(*job); err != nil {
		return err
	}
	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	fmt.Printf("✅ %s: [%s] %s\n", state, job.ID, job.Name)
	return nil
}
