package main

import (
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
	"github.com/startvibecoding/vibecoding/internal/mcp"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/provider/anthropic"
	"github.com/startvibecoding/vibecoding/internal/provider/openai"
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
		flagProvider string
		flagModel    string
		flagMode     string
		flagThinking string
		flagContinue bool
		flagResume   string
		flagSession  string
		flagSandbox  bool
		flagPrint    bool
		flagVerbose  bool
		flagDebug    bool
	)

	rootCmd := &cobra.Command{
		Use:     "vibecoding [message...]",
		Aliases: []string{"vc"},
		Short:   "VibeCoding - AI coding assistant",
		Long:    "VibeCoding is an AI-powered coding assistant that runs in your terminal.\nSupports OpenAI and Anthropic APIs with sandboxed execution.",
		Version: version,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFn(args, runOptions{
				provider:  flagProvider,
				model:     flagModel,
				mode:      flagMode,
				thinking:  flagThinking,
				continue_: flagContinue,
				resume:    flagResume,
				session:   flagSession,
				sandbox:   flagSandbox,
				print:     flagPrint,
				verbose:   flagVerbose,
				debug:     flagDebug,
			})
		},
	}

	acpCmd := &cobra.Command{
		Use:   "acp",
		Short: "Run the Agent Client Protocol server",
		Long:  "Run vibecoding as an ACP-compliant stdio agent.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return acpRunFn(acp.RunOptions{
				Provider: flagProvider,
				Model:    flagModel,
				Mode:     flagMode,
				Thinking: flagThinking,
				Sandbox:  flagSandbox,
				Verbose:  flagVerbose,
				Debug:    flagDebug,
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

	acpFlags := acpCmd.Flags()
	acpFlags.StringVarP(&flagProvider, "provider", "p", "", "Provider (openai, anthropic, or custom provider name)")
	acpFlags.StringVarP(&flagModel, "model", "m", "", "Model ID")
	acpFlags.StringVarP(&flagMode, "mode", "M", "", "Mode (plan, agent, yolo)")
	acpFlags.StringVarP(&flagThinking, "thinking", "t", "", "Thinking level (off, minimal, low, medium, high, xhigh)")
	acpFlags.BoolVar(&flagSandbox, "sandbox", false, "Enable sandbox (bwrap) for secure execution")
	acpFlags.BoolVar(&flagVerbose, "verbose", false, "Verbose output")
	acpFlags.BoolVar(&flagDebug, "debug", false, "Enable debug logging")

	rootCmd.AddCommand(acpCmd)
	return rootCmd
}

type runOptions struct {
	provider  string
	model     string
	mode      string
	thinking  string
	continue_ bool
	resume    string
	session   string
	sandbox   bool
	print     bool
	verbose   bool
	debug     bool
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

	// Print mode: non-interactive
	if opts.print {
		return runPrint(args, p, model, mode, provider.ThinkingLevel(thinkingLevel), settings, registry, sess, extraContext)
	}

	// Interactive mode
	// Clear any pending stdin input (e.g., terminal color queries)
	clearStdin()

	app := tui.NewApp(p, model, settings, sess, registry, sbInfo, extraContext, skillsMgr, mode)
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
	// Check if provider is in config
	pc := settings.GetProviderConfig(providerName)

	if pc != nil {
		// Custom provider from config
		apiKey := settings.ResolveKey(providerName)
		models := convertModelConfigs(providerName, pc.Models)

		api := pc.API
		if api == "" {
			// Auto-detect: if baseUrl contains "anthropic", use anthropic-messages
			if strings.Contains(strings.ToLower(pc.BaseURL), "anthropic") {
				api = "anthropic-messages"
			} else {
				api = "openai-chat"
			}
		}

		var p provider.Provider
		switch api {
		case "anthropic-messages":
			ap := anthropic.NewProviderWithModels(apiKey, pc.BaseURL, models)
			if pc.ThinkingFormat != "" {
				ap.SetThinkingFormat(pc.ThinkingFormat)
			}
			if pc.CacheControl != nil {
				ap.SetCacheControlEnabled(pc.CacheControl)
			}
			configureRetry(ap, settings)
			p = ap
		case "openai-chat", "openai":
			op := openai.NewProviderWithModels(apiKey, pc.BaseURL, models)
			if pc.ThinkingFormat != "" {
				op.SetThinkingFormat(pc.ThinkingFormat)
			}
			configureRetry(op, settings)
			p = op
		default:
			return nil, nil, fmt.Errorf("unsupported API type: %s (use 'openai-chat' or 'anthropic-messages')", api)
		}

		// Find model
		model := p.GetModel(modelID)
		if model == nil {
			if len(models) > 0 {
				model = models[0]
			} else {
				return nil, nil, fmt.Errorf("no models configured for provider %s", providerName)
			}
		}

		return p, model, nil
	}

	// Built-in providers (fallback)
	var p provider.Provider
	switch strings.ToLower(providerName) {
	case "openai":
		apiKey := settings.ResolveKey(providerName)
		p = openai.NewProvider(apiKey, "")
	case "anthropic":
		apiKey := settings.ResolveKey(providerName)
		p = anthropic.NewProvider(apiKey, "")
	default:
		return nil, nil, fmt.Errorf("unknown provider: %s (add it to settings.json providers section)", providerName)
	}

	model := p.GetModel(modelID)
	if model == nil {
		models := p.Models()
		if len(models) > 0 {
			model = models[0]
		} else {
			return nil, nil, fmt.Errorf("no models available for provider %s", providerName)
		}
	}

	return p, model, nil
}

// retryConfigurable is implemented by providers that support retry configuration.
type retryConfigurable interface {
	SetRetryConfig(cfg *provider.RetryConfig)
}

// configureRetry sets retry config on a provider if it supports it.
func configureRetry(p provider.Provider, settings *config.Settings) {
	if rc, ok := p.(retryConfigurable); ok {
		rc.SetRetryConfig(&provider.RetryConfig{
			Enabled:     settings.Retry.Enabled,
			MaxRetries:  settings.Retry.MaxRetries,
			BaseDelayMs: settings.Retry.BaseDelayMs,
		})
	}
}

// convertModelConfigs converts config.ModelConfig to provider.Model.
func convertModelConfigs(providerName string, models []config.ModelConfig) []*provider.Model {
	var result []*provider.Model
	for _, m := range models {
		input := m.Input
		if len(input) == 0 {
			input = []string{"text"}
		}
		var cost provider.ModelPricing
		if m.Cost != nil {
			cost = provider.ModelPricing{
				Input:      m.Cost.Input,
				Output:     m.Cost.Output,
				CacheRead:  m.Cost.CacheRead,
				CacheWrite: m.Cost.CacheWrite,
			}
		}
		result = append(result, &provider.Model{
			ID:            m.ID,
			Name:          m.Name,
			Provider:      providerName,
			Reasoning:     m.Reasoning,
			Input:         input,
			Cost:          cost,
			ContextWindow: m.ContextWindow,
			MaxTokens:     m.MaxTokens,
		})
	}
	return result
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

func runPrint(args []string, p provider.Provider, model *provider.Model, mode string, thinkingLevel provider.ThinkingLevel, settings *config.Settings, registry *tools.Registry, sess *session.Manager, extraContext string) error {
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
	}

	a := agent.New(agentCfg, registry)

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
