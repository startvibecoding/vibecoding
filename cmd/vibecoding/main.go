package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"

	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/contextfiles"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args, runOptions{
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

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
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
	// Enable debug logging if requested
	debugEnabled = opts.debug
	if debugEnabled {
		fmt.Fprintf(os.Stderr, "[DEBUG] Debug logging enabled\n")
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
	skillsMgr := skills.NewManager(settings.GetGlobalSkillsDir(), cwd+"/.skills")
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
		switch mode {
		case "plan":
			sbMgr.SetLevel(sandbox.LevelStrict)
		case "yolo":
			sbMgr.SetLevel(sandbox.LevelNone)
		default:
			sbMgr.SetLevel(sandbox.LevelStandard)
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
		sess, err = session.Open(opts.session)
		if err != nil {
			return fmt.Errorf("open session: %w", err)
		}
		sessionInfo = fmt.Sprintf("📂 Opened session: %s", sess.GetFile())
	} else {
		sess = session.New(cwd, settings.GetSessionDir())
		if err := sess.Init(); err != nil {
			return fmt.Errorf("init session: %w", err)
		}
	}

	// Setup tools
	registry := tools.NewRegistry(cwd, sbMgr.GetActive())
	registry.RegisterDefaults()

	// Register skill reference tool if skills are available
	if skillsMgr != nil {
		registry.Register(tools.NewSkillRefTool(skillsMgr))
	}

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
	p2 := tea.NewProgram(app, tea.WithInputTTY(), tea.WithReportFocus())
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
			p = anthropic.NewProviderWithModels(apiKey, pc.BaseURL, models)
		case "openai-chat", "openai":
			p = openai.NewProviderWithModels(apiKey, pc.BaseURL, models)
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
	// Use a goroutine with timeout to read any pending input
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 128)
		for {
			select {
			case <-done:
				return
			default:
				n, _ := os.Stdin.Read(buf)
				if n == 0 {
					return
				}
			}
		}
	}()
	// Wait a short time for any pending input to be read
	time.Sleep(50 * time.Millisecond)
	close(done)
}

func runPrint(args []string, p provider.Provider, model *provider.Model, mode string, thinkingLevel provider.ThinkingLevel, settings *config.Settings, registry *tools.Registry, sess *session.Manager, extraContext string) error {
	input := strings.Join(args, " ")
	if input == "" {
		data, err := os.ReadFile("/dev/stdin")
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
		glamour.WithAutoStyle(),
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

	for event := range eventCh {
		switch event.Type {
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
				fmt.Fprintf(os.Stderr, "Tokens: %d in / %d out | Cost: $%.4f\n",
					event.Usage.Input, event.Usage.Output, event.Usage.Cost.Total)
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
	}

	return nil
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
