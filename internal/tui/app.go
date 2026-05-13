package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"

	"github.com/fuckvibecoding/vibecoding/internal/agent"
	"github.com/fuckvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/fuckvibecoding/vibecoding/internal/context"
	"github.com/fuckvibecoding/vibecoding/internal/provider"
	"github.com/fuckvibecoding/vibecoding/internal/session"
	"github.com/fuckvibecoding/vibecoding/internal/skills"
	"github.com/fuckvibecoding/vibecoding/internal/tools"
)

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	thinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			BorderTop(true).
			BorderForeground(lipgloss.Color("240"))

	pasteMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
)

// InputEvent represents a queued input event
type InputEvent struct {
	msg     tea.Msg
	arrived time.Time
}

// toolResult stores tool result information
type toolResult struct {
	toolName    string
	toolArgs    map[string]any // Tool call arguments
	summary     string         // Short summary for collapsed view
	fullContent string         // Full content for expanded view
	msgIndex    int            // Index in a.messages where this tool message lives
}

// App is the main TUI application.
type App struct {
	provider    provider.Provider
	model       *provider.Model
	settings    *config.Settings
	session     *session.Manager
	registry    *tools.Registry
	sandboxInfo string
	mode        string
	extraContext string
	skillsMgr   *skills.Manager

	// Skills state: base extraContext (without skills) and active skill names
	baseExtraContext string            // extraContext without skill content
	activeSkills     map[string]string // skill name -> skill context string

	// UI Components
	viewport viewport.Model
	input    textinput.Model

	// State
	messages   []string
	toolResults []toolResult // Store tool results for expansion
	isThinking bool
	agent      *agent.Agent
	eventCh    <-chan agent.Event
	width      int
	height     int
	ready      bool
	autoScroll bool

	// Paste markers storage
	pasteCounter int
	pastes       map[int]string

	// Input queue for batching
	inputQueue     []InputEvent
	inputQueueMu   sync.Mutex
	lastInputTime  time.Time
	inputBatchSize int
	inputDelay     time.Duration

	// Full content for native scrollbar support
	fullContent string

	// Initial message to display
	initialMessage string

	// Tool output expansion
	toolOutputExpanded bool

	// Context usage
	contextUsage *ctxpkg.ContextUsage
	
	// Spinner state
	spinnerIndex int

	// Session history
	sessionMu      sync.Mutex
	historyLoaded  bool

	// Render throttling
	lastRender     time.Time
	renderPending  bool
	renderMu       sync.Mutex
	renderInterval time.Duration

	// Approval state
	waitingForApproval bool
	pendingApprovalID  string
	approvalQueue      []pendingApproval
}

// pendingApproval holds a queued approval request.
type pendingApproval struct {
	approvalID string
	toolName   string
	args       map[string]any
}

// NewApp creates a new TUI application.
func NewApp(p provider.Provider, model *provider.Model, settings *config.Settings, sess *session.Manager, registry *tools.Registry, sandboxInfo string, extraContext string, skillsMgr *skills.Manager, initialMode string) *App {
	input := textinput.New()
	input.Placeholder = "Type a message..."
	input.Focus()
	input.CharLimit = 0

	vp := viewport.New(80, 20)

	// Determine initial mode: use provided mode, fall back to settings default
	mode := initialMode
	if mode == "" {
		mode = settings.DefaultMode
	}
	if mode == "" {
		mode = "agent"
	}

	return &App{
		provider:       p,
		model:          model,
		settings:       settings,
		session:        sess,
		registry:       registry,
		sandboxInfo:    sandboxInfo,
		mode:           mode,
		extraContext:   extraContext,
		baseExtraContext: extraContext,
		activeSkills:    make(map[string]string),
		skillsMgr:      skillsMgr,
		input:          input,
		viewport:       vp,
		autoScroll:     true,
		pastes:         make(map[int]string),
		inputQueue:     make([]InputEvent, 0, 100),
		inputBatchSize: 10,
		inputDelay:     16 * time.Millisecond, // ~60fps
		renderInterval: 16 * time.Millisecond, // ~60fps
	}
}

// SetInitialMessage sets an initial message to display when the TUI starts.
func (a *App) SetInitialMessage(msg string) {
	a.initialMessage = msg
}

// LoadHistoryMessages loads messages from session history into TUI display.
func (a *App) LoadHistoryMessages() {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()

	if a.session == nil {
		return
	}

	historyMessages := a.session.GetMessages()
	if len(historyMessages) == 0 {
		return
	}

	a.historyLoaded = true

	// Display history messages in TUI
	for _, msg := range historyMessages {
		switch msg.Role {
		case "user":
			a.addMessage(userStyle.Render("You: ") + msg.Content)
		case "assistant":
			// Extract text content from assistant message
			var textContent string
			if msg.Content != "" {
				textContent = msg.Content
			} else if len(msg.Contents) > 0 {
				for _, block := range msg.Contents {
					if block.Type == "text" && block.Text != "" {
						textContent += block.Text
					}
				}
			}
			if textContent != "" {
				a.addMessage(assistantStyle.Render(textContent))
			}
		}
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	// Show initial message if set
	if a.initialMessage != "" {
		a.messages = append(a.messages, statusStyle.Render(a.initialMessage))
	}

	// Load history messages from session
	a.LoadHistoryMessages()

	return tea.Batch(textinput.Blink, a.processInputQueue())
}

// processInputQueue returns a command that processes queued input events
func (a *App) processInputQueue() tea.Cmd {
	return tea.Tick(a.inputDelay, func(t time.Time) tea.Msg {
		return inputQueueTickMsg(t)
	})
}

// inputQueueTickMsg is sent when the input queue should be processed
type inputQueueTickMsg time.Time

// spinnerTickMsg is sent to update the spinner animation
type spinnerTickMsg time.Time

// Spinner characters for the thinking animation
var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
const spinnerInterval = 100 * time.Millisecond

// tickSpinner returns a command that updates the spinner
func (a *App) tickSpinner() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		// Calculate heights: input (1 line) + footer (1 line) + some padding
		heightUsed := 3 // input + footer + padding
		chatHeight := msg.Height - heightUsed
		if chatHeight < 3 {
			chatHeight = 3
		}

		a.viewport.Width = msg.Width
		a.viewport.Height = chatHeight
		a.input.Width = msg.Width - 4

		a.updateViewportContent()
		return a, nil

	case inputQueueTickMsg:
		// Process queued input events
		cmd := a.flushInputQueue()
		cmds = append(cmds, cmd)
		// Schedule next tick
		cmds = append(cmds, a.processInputQueue())
		return a, tea.Batch(cmds...)

	case spinnerTickMsg:
		// Update spinner animation if still thinking
		if a.isThinking {
			a.spinnerIndex = (a.spinnerIndex + 1) % len(spinnerChars)
			cmds = append(cmds, a.tickSpinner())
		}
		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Queue the key event
		a.queueInput(msg)

		// For special keys, process immediately
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "esc":
			if a.isThinking {
				if a.agent != nil {
					a.agent.Abort()
					a.agent = nil // Reset agent so next request creates a fresh one with new abort channel
				}
				a.isThinking = false
				a.addMessage(statusStyle.Render("⏹ Aborted"))
			} else {
				a.input.Reset()
			}
			return a, nil
		case "enter":
			// Process enter immediately
			a.flushInputQueue()
			input := strings.TrimSpace(a.input.Value())

			// Check if waiting for approval
			if a.waitingForApproval {
				if a.agent != nil {
					approved := strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
					a.agent.HandleApprovalResponse(a.pendingApprovalID, approved)
					if approved {
						a.addMessage(statusStyle.Render("✅ Approved"))
					} else {
						a.addMessage(statusStyle.Render("❌ Denied"))
					}
				}
				// Show next queued approval or clear waiting state
				if len(a.approvalQueue) > 0 {
					a.showNextApproval()
				} else {
					a.waitingForApproval = false
					a.pendingApprovalID = ""
				}
				a.input.Reset()
				a.scheduleRender()
				return a, nil
			}

			if input != "" {
				a.input.Reset()
				expandedInput := a.expandPasteMarkers(input)
				return a, a.processInput(expandedInput)
			}
			return a, nil
		case "tab":
			a.cycleMode()
			return a, nil
		case "pgup":
			a.viewport.HalfViewUp()
			a.autoScroll = false
			return a, nil
		case "pgdown":
			a.viewport.HalfViewDown()
			if a.viewport.AtBottom() {
				a.autoScroll = true
			}
			return a, nil
		case "home":
			a.viewport.GotoTop()
			a.autoScroll = false
			return a, nil
		case "end":
			a.viewport.GotoBottom()
			a.autoScroll = true
			return a, nil
		case "ctrl+o":
			// Toggle tool output expansion
			a.toolOutputExpanded = !a.toolOutputExpanded
			a.updateViewportContent()
			return a, nil
		}

		// Check for paste (multi-line input in a single key event)
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			input := string(msg.Runes)
			if strings.Contains(input, "\n") {
				a.handlePaste(input)
				return a, nil
			}
		}

		return a, nil

	case agentStartMsg:
		a.isThinking = true
		a.spinnerIndex = 0
		a.addMessage(userStyle.Render("You: ") + msg.input)
		return a, tea.Batch(listenEvents(a.eventCh), a.tickSpinner())

	case agentEventMsg:
		return a, a.handleAgentEvent(msg.event)

	case agentDoneMsg:
		a.isThinking = false
		if msg.err != nil {
			a.addMessage(errorStyle.Render("Error: ") + msg.err.Error())
		}
		return a, nil
	}

	// Update components
	var inputCmd, vpCmd tea.Cmd
	a.input, inputCmd = a.input.Update(msg)
	a.viewport, vpCmd = a.viewport.Update(msg)

	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}

	return a, tea.Batch(cmds...)
}

// queueInput adds an input event to the queue
func (a *App) queueInput(msg tea.Msg) {
	a.inputQueueMu.Lock()
	defer a.inputQueueMu.Unlock()

	a.inputQueue = append(a.inputQueue, InputEvent{
		msg:     msg,
		arrived: time.Now(),
	})
	a.lastInputTime = time.Now()
}

// flushInputQueue processes all queued input events
func (a *App) flushInputQueue() tea.Cmd {
	a.inputQueueMu.Lock()
	events := make([]InputEvent, len(a.inputQueue))
	copy(events, a.inputQueue)
	a.inputQueue = a.inputQueue[:0]
	a.inputQueueMu.Unlock()

	if len(events) == 0 {
		return nil
	}

	// Process events in batch
	var cmds []tea.Cmd
	for _, event := range events {
		// Update input component
		if keyMsg, ok := event.msg.(tea.KeyMsg); ok {
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(keyMsg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	// Schedule render
	a.scheduleRender()

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// scheduleRender schedules a render update with throttling
func (a *App) scheduleRender() {
	a.renderMu.Lock()
	defer a.renderMu.Unlock()

	now := time.Now()
	if now.Sub(a.lastRender) < a.renderInterval {
		// Too soon, mark as pending
		a.renderPending = true
		return
	}

	// Render now
	a.lastRender = now
	a.renderPending = false
	a.updateViewportContent()
}

// View implements tea.Model.
func (a *App) View() string {
	if !a.ready {
		return "\n  Loading...\n"
	}

	footer := a.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		a.viewport.View(),
		a.input.View(),
		footer,
	)
}

// handlePaste handles large pastes by creating markers
func (a *App) handlePaste(text string) {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	totalChars := len(text)

	// Check if this is a large paste (> 5 lines or > 500 chars)
	if len(lines) > 5 || totalChars > 500 {
		a.pasteCounter++
		pasteId := a.pasteCounter
		a.pastes[pasteId] = text

		// Create marker
		var marker string
		if len(lines) > 5 {
			marker = fmt.Sprintf("[paste #%d +%d lines]", pasteId, len(lines))
		} else {
			marker = fmt.Sprintf("[paste #%d %d chars]", pasteId, totalChars)
		}

		// Insert marker into input
		current := a.input.Value()
		a.input.SetValue(current + marker)
	} else {
		// Small paste - insert directly
		current := a.input.Value()
		// Replace newlines with spaces for single-line input
		cleanText := strings.ReplaceAll(text, "\n", " ")
		a.input.SetValue(current + cleanText)
	}
}

// expandPasteMarkers expands paste markers to their original content
func (a *App) expandPasteMarkers(text string) string {
	result := text
	for pasteId, content := range a.pastes {
		// Match markers like [paste #1 +15 lines] or [paste #2 1234 chars]
		markerLine := fmt.Sprintf("+%d lines", strings.Count(content, "\n")+1)
		markerChar := fmt.Sprintf("%d chars", len(content))

		// Try line marker
		marker1 := fmt.Sprintf("[paste #%d %s]", pasteId, markerLine)
		if strings.Contains(result, marker1) {
			result = strings.ReplaceAll(result, marker1, content)
			continue
		}

		// Try char marker
		marker2 := fmt.Sprintf("[paste #%d %s]", pasteId, markerChar)
		if strings.Contains(result, marker2) {
			result = strings.ReplaceAll(result, marker2, content)
		}
	}

	// Clean up used pastes
	a.pastes = make(map[int]string)
	a.pasteCounter = 0

	return result
}

func (a *App) updateViewportContent() {
	// Rebuild messages based on expansion state
	var displayMessages []string

	// Build a set of message indices that are tool results
	toolMsgIndices := make(map[int]int) // msgIndex -> toolResults index
	for i, tr := range a.toolResults {
		toolMsgIndices[tr.msgIndex] = i
	}

	for idx, msg := range a.messages {
		if trIdx, ok := toolMsgIndices[idx]; ok {
			result := a.toolResults[trIdx]
			if a.toolOutputExpanded {
				// Show full content with arguments
				var content string
				if result.toolArgs != nil {
					argsStr := formatToolArgs(result.toolName, result.toolArgs)
					if result.fullContent != "" {
						content = fmt.Sprintf("🔧 [%s]\n%s\n---\n%s", result.toolName, argsStr, result.fullContent)
					} else {
						content = fmt.Sprintf("🔧 [%s]\n%s", result.toolName, argsStr)
					}
				} else if result.fullContent != "" {
					content = fmt.Sprintf("🔧 [%s]\n%s", result.toolName, result.fullContent)
				} else {
					content = fmt.Sprintf("🔧 [%s]", result.toolName)
				}
				displayMessages = append(displayMessages, toolStyle.Render(content))
			} else {
				// Show summary
				displayMessages = append(displayMessages, toolStyle.Render(fmt.Sprintf("🔧 [%s] %s", result.toolName, result.summary)))
			}
		} else {
			displayMessages = append(displayMessages, msg)
		}
	}
	
	a.fullContent = strings.Join(displayMessages, "\n\n")
	a.viewport.SetContent(a.wrapContent(a.fullContent))
	if a.autoScroll {
		a.viewport.GotoBottom()
	}
}

// wrapContent wraps content to fit within the viewport width.
// This ensures logical lines in the viewport match visual lines after wrapping.
func (a *App) wrapContent(content string) string {
	if a.width <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, cellbuf.Wrap(line, a.width, ""))
	}
	return strings.Join(wrapped, "\n")
}

// formatToolArgs formats tool arguments for display
func formatToolArgs(toolName string, args map[string]any) string {
	var parts []string
	
	switch toolName {
	case "write":
		// Show path and content for write tool
		if path, ok := args["path"]; ok {
			parts = append(parts, fmt.Sprintf("path: %v", path))
		}
		if content, ok := args["content"]; ok {
			contentStr := fmt.Sprintf("%v", content)
			// Truncate content if too long
			if len(contentStr) > 500 {
				contentStr = contentStr[:500] + "..."
			}
			parts = append(parts, fmt.Sprintf("content:\n%s", contentStr))
		}
	case "edit":
		// Show path and edits for edit tool
		if path, ok := args["path"]; ok {
			parts = append(parts, fmt.Sprintf("path: %v", path))
		}
		if editList, ok := args["edits"]; ok {
			if arr, ok := editList.([]any); ok {
				for idx, e := range arr {
					if m, ok := e.(map[string]any); ok {
						oldT, _ := m["oldText"].(string)
						newT, _ := m["newText"].(string)
						if len(oldT) > 100 {
							oldT = oldT[:100] + "..."
						}
						if len(newT) > 100 {
							newT = newT[:100] + "..."
						}
						parts = append(parts, fmt.Sprintf("edit[%d]:\n  old: %s\n  new: %s", idx+1, oldT, newT))
					}
				}
			}
		}
	case "read":
		if path, ok := args["path"]; ok {
			parts = append(parts, fmt.Sprintf("path: %v", path))
		}
	case "bash":
		if cmd, ok := args["command"]; ok {
			parts = append(parts, fmt.Sprintf("command: %v", cmd))
		}
	default:
		// Show all arguments for other tools
		for k, v := range args {
			vStr := fmt.Sprintf("%v", v)
			if len(vStr) > 100 {
				vStr = vStr[:100] + "..."
			}
			parts = append(parts, fmt.Sprintf("%s: %s", k, vStr))
		}
	}
	
	return strings.Join(parts, "\n")
}


func formatTokens(count int) string {
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

func (a *App) renderFooter() string {
	modelName := "unknown"
	if a.model != nil {
		modelName = a.model.Name
	}

	var modeStr string
	switch a.mode {
	case "plan":
		modeStr = "🗒 PLAN"
	case "agent":
		modeStr = "🔧 AGENT"
	case "yolo":
		modeStr = "🚀 YOLO"
	default:
		modeStr = strings.ToUpper(a.mode)
	}

	cwd := "."
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if len(cwd) > 30 {
		cwd = "..." + cwd[len(cwd)-27:]
	}

	// Build context usage string with color coding
	contextStr := ""
	if a.contextUsage != nil && a.contextUsage.ContextWindow > 0 {
		if a.contextUsage.Percent != nil {
			percent := *a.contextUsage.Percent
			contextDisplay := fmt.Sprintf("%.1f%%/%s",
				percent,
				formatTokens(a.contextUsage.ContextWindow))
			// Colorize based on usage
			if percent > 90 {
				contextStr = " | " + errorStyle.Render(contextDisplay)
			} else if percent > 70 {
				contextStr = " | " + userStyle.Render(contextDisplay)
			} else {
				contextStr = " | " + contextDisplay
			}
		} else {
			contextStr = fmt.Sprintf(" | ?/%s", formatTokens(a.contextUsage.ContextWindow))
		}
	}

	status := fmt.Sprintf(" %s | %s | %s%s", modeStr, modelName, cwd, contextStr)
	if a.isThinking {
		status += " | " + spinnerChars[a.spinnerIndex]
	} else {
		if a.toolOutputExpanded {
			status += " | Tab:mode Esc:abort Ctrl+O:collapse"
		} else {
			status += " | Tab:mode Esc:abort Ctrl+O:expand"
		}
	}

	return footerStyle.Width(a.width).Render(status)
}

func (a *App) addMessage(msg string) {
	a.messages = append(a.messages, msg)
	a.updateViewportContent()
}

// showNextApproval pops the next approval request from the queue and displays it.
func (a *App) showNextApproval() {
	if len(a.approvalQueue) == 0 {
		a.waitingForApproval = false
		a.pendingApprovalID = ""
		return
	}
	next := a.approvalQueue[0]
	a.approvalQueue = a.approvalQueue[1:]
	a.pendingApprovalID = next.approvalID
	a.waitingForApproval = true
	if len(a.approvalQueue) > 0 {
		a.addMessage(warningStyle.Render(fmt.Sprintf("⚠️  Approval required for [%s] (%d more pending)", next.toolName, len(a.approvalQueue))))
	} else {
		a.addMessage(warningStyle.Render(fmt.Sprintf("⚠️  Approval required for [%s]", next.toolName)))
	}
	if len(next.args) > 0 {
		var buf strings.Builder
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(next.args); err == nil {
			a.addMessage(warningStyle.Render(strings.TrimRight(buf.String(), "\n")))
		}
	}
	a.addMessage(warningStyle.Render("Approve? (y/n): "))
}

func (a *App) cycleMode() {
	modes := []string{"plan", "agent", "yolo"}
	current := 0
	for i, m := range modes {
		if m == a.mode {
			current = i
			break
		}
	}
	next := (current + 1) % len(modes)
	a.mode = modes[next]
	
	// If agent is currently running, abort it so the new mode takes effect immediately
	if a.isThinking && a.agent != nil {
		a.agent.Abort()
		a.agent = nil
		a.isThinking = false
		a.addMessage(statusStyle.Render("⏹ Aborted (mode change)"))
	} else {
		a.agent = nil
	}

	var modeLabel string
	switch a.mode {
	case "plan":
		modeLabel = "🗒️ PLAN - Read-only (no modifications)"
	case "agent":
		modeLabel = "🔧 AGENT - Bash requires approval"
	case "yolo":
		modeLabel = "🚀 YOLO - Full access"
	}
	a.addMessage(statusStyle.Render(fmt.Sprintf("Mode: %s", modeLabel)))
}

func (a *App) processInput(input string) tea.Cmd {
	if strings.HasPrefix(input, "/") {
		return a.handleCommand(input)
	}

	if a.agent == nil {
		compactionSettings := ctxpkg.CompactionSettings{
			Enabled:          a.settings.Compaction.Enabled,
			ReserveTokens:    a.settings.Compaction.ReserveTokens,
			KeepRecentTokens: a.settings.Compaction.KeepRecentTokens,
		}
		if compactionSettings.ReserveTokens == 0 {
			compactionSettings.ReserveTokens = 16384
		}
		if compactionSettings.KeepRecentTokens == 0 {
			compactionSettings.KeepRecentTokens = 20000
		}

		agentCfg := agent.Config{
			Provider:           a.provider,
			Model:              a.model,
			Mode:               a.mode,
			ThinkingLevel:      provider.ThinkingLevel(a.settings.DefaultThinkingLevel),
			MaxTokens:          a.settings.MaxOutputTokens,
			Settings:           a.settings,
			Session:            a.session,
			ExtraContext:       a.extraContext,
			CompactionSettings: compactionSettings,
		}
		a.agent = agent.New(agentCfg, a.registry)

		// Load history messages from session if available and not yet loaded
		if a.session != nil && !a.historyLoaded {
			a.sessionMu.Lock()
			historyMessages := a.session.GetMessages()
			a.sessionMu.Unlock()

			if len(historyMessages) > 0 {
				a.agent.LoadHistoryMessages(historyMessages)
			}
		}
	}

	ctx := context.Background()
	a.eventCh = a.agent.Run(ctx, input)

	return tea.Batch(
		func() tea.Msg { return agentStartMsg{input: input} },
		listenEvents(a.eventCh),
	)
}

func (a *App) handleCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	command := parts[0]

	switch command {
	case "/mode":
		if len(parts) > 1 {
			switch parts[1] {
			case "plan", "agent", "yolo":
				a.mode = parts[1]
				// If agent is currently running, abort it so the new mode takes effect immediately
				if a.isThinking && a.agent != nil {
					a.agent.Abort()
					a.agent = nil
					a.isThinking = false
					a.addMessage(statusStyle.Render("⏹ Aborted (mode change)"))
				} else {
					a.agent = nil
				}
				a.addMessage(statusStyle.Render(fmt.Sprintf("Mode: %s", strings.ToUpper(a.mode))))
			default:
				a.addMessage(errorStyle.Render("Invalid mode"))
			}
		} else {
			a.addMessage(statusStyle.Render(fmt.Sprintf("Current mode: %s", strings.ToUpper(a.mode))))
			switch a.mode {
			case "plan":
				a.addMessage(statusStyle.Render("  Permissions: READ only (no modifications)"))
			case "agent":
				a.addMessage(statusStyle.Render("  Permissions: READ/WRITE/EDIT auto | BASH requires approval"))
			case "yolo":
				a.addMessage(statusStyle.Render("  Permissions: ALL tools auto-execute"))
			}
		}
	case "/model":
		a.addMessage(statusStyle.Render(fmt.Sprintf("Model: %s (%s)", a.model.Name, a.model.Provider)))
	case "/skills":
		a.listSkills()
	case "/skill":
		if len(parts) > 1 {
			a.activateSkill(parts[1])
		} else {
			a.listSkills()
		}
	case "/clear":
		a.messages = nil
		a.agent = nil
		a.contextUsage = nil
		a.pastes = make(map[int]string)
		a.pasteCounter = 0
		a.activeSkills = make(map[string]string)
		a.extraContext = a.baseExtraContext
		a.updateViewportContent()
	case "/quit":
		return tea.Quit
	case "/help":
		a.addMessage(statusStyle.Render("Commands: /mode, /model, /skills, /skill <name>, /clear, /quit, /help"))
	default:
		// Handle /skill:<name> syntax (colon-separated)
		if strings.HasPrefix(command, "/skill:") {
			skillName := strings.TrimPrefix(command, "/skill:")
			if skillName != "" {
				a.activateSkill(skillName)
			} else {
				a.listSkills()
			}
		} else {
			a.addMessage(errorStyle.Render(fmt.Sprintf("Unknown: %s", command)))
		}
	}

	return nil
}

// listSkills displays all available skills.
func (a *App) listSkills() {
	if a.skillsMgr == nil {
		a.addMessage(statusStyle.Render("No skills manager available."))
		return
	}
	skillList := a.skillsMgr.List()
	if len(skillList) == 0 {
		a.addMessage(statusStyle.Render("No skills found."))
		return
	}

	var sb strings.Builder
	sb.WriteString("Available skills:\n")
	for _, s := range skillList {
		marker := " "
		if _, ok := a.activeSkills[s.Name]; ok {
			marker = "*"
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s (%s): %s\n", marker, s.Name, s.Source, s.Description))
	}
	sb.WriteString("\nUse /skill <name> or /skill:<name> to activate a skill.")
	a.addMessage(statusStyle.Render(sb.String()))
}

// activateSkill loads a skill's content into the extra context.
func (a *App) activateSkill(name string) {
	if a.skillsMgr == nil {
		a.addMessage(errorStyle.Render("No skills manager available."))
		return
	}
	skill := a.skillsMgr.Get(name)
	if skill == nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Skill not found: %s", name)))
		return
	}

	// Check if already active
	if _, ok := a.activeSkills[name]; ok {
		a.addMessage(statusStyle.Render(fmt.Sprintf("Skill '%s' is already active.", name)))
		return
	}

	// Add skill content to active skills
	skillCtx := a.skillsMgr.BuildSkillContext(name)
	a.activeSkills[name] = skillCtx

	// Rebuild extraContext from base + all active skills
	a.rebuildExtraContext()

	// Reset agent so next message uses the updated context
	a.agent = nil

	a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Skill '%s' activated (%s): %s", name, skill.Source, skill.Description)))
}

// rebuildExtraContext rebuilds extraContext from base context + all active skills.
func (a *App) rebuildExtraContext() {
	sb := strings.Builder{}
	sb.WriteString(a.baseExtraContext)
	for _, ctx := range a.activeSkills {
		sb.WriteString(ctx)
	}
	a.extraContext = sb.String()
}

func (a *App) handleAgentEvent(event agent.Event) tea.Cmd {
	switch event.Type {
	case agent.EventTextDelta:
		lastIdx := len(a.messages) - 1
		if lastIdx >= 0 && isAssistantMsg(a.messages[lastIdx]) {
			a.messages[lastIdx] += event.TextDelta
		} else {
			a.messages = append(a.messages, assistantStyle.Render("Assistant: ")+event.TextDelta)
		}
		a.scheduleRender()
		return listenEvents(a.eventCh)

	case agent.EventThinkDelta:
		lastIdx := len(a.messages) - 1
		if lastIdx >= 0 && strings.HasPrefix(a.messages[lastIdx], thinkStyle.Render("think: ")) {
			a.messages[lastIdx] += event.ThinkDelta
		} else {
			a.messages = append(a.messages, thinkStyle.Render("think: ")+event.ThinkDelta)
		}
		a.scheduleRender()
		return listenEvents(a.eventCh)

	case agent.EventToolCall:
		if event.ToolCall != nil {
			// Store tool args for later display
			msgIdx := len(a.messages) // Will be the index after append
			a.toolResults = append(a.toolResults, toolResult{
				toolName: event.ToolCall.Name,
				toolArgs: event.ToolArgs,
				msgIndex: msgIdx,
			})
			a.addMessage(toolStyle.Render(fmt.Sprintf("🔧 [%s] ...", event.ToolCall.Name)))
		}
		return listenEvents(a.eventCh)

	case agent.EventToolResult:
		// Find the matching tool result entry and update it
		for j := len(a.toolResults) - 1; j >= 0; j-- {
			if a.toolResults[j].toolName == event.ToolName && a.toolResults[j].fullContent == "" {
				a.toolResults[j].fullContent = event.ToolResult
				
				// Create summary based on tool type
				switch event.ToolName {
				case "bash":
					a.toolResults[j].summary = event.ToolResult
				case "read":
					lines := strings.Split(event.ToolResult, "\n")
					a.toolResults[j].summary = fmt.Sprintf("%d lines", len(lines))
				case "write":
					a.toolResults[j].summary = "Written"
				case "edit":
					a.toolResults[j].summary = "Applied"
				default:
					a.toolResults[j].summary = truncate(event.ToolResult, 50)
				}
				break
			}
		}
		
		// Update the message at the stored index
		for j := len(a.toolResults) - 1; j >= 0; j-- {
			if a.toolResults[j].toolName == event.ToolName && a.toolResults[j].fullContent == "" {
				idx := a.toolResults[j].msgIndex
				if idx >= 0 && idx < len(a.messages) {
					if event.ToolName == "bash" || a.toolOutputExpanded {
						a.messages[idx] = toolStyle.Render(fmt.Sprintf("🔧 [%s]\n%s", event.ToolName, event.ToolResult))
					} else {
						a.messages[idx] = toolStyle.Render(fmt.Sprintf("🔧 [%s] %s", event.ToolName, a.toolResults[j].summary))
					}
				}
				break
			}
		}
		a.scheduleRender()
		return listenEvents(a.eventCh)

	case agent.EventToolApprovalRequest:
		// Queue the approval request
		a.approvalQueue = append(a.approvalQueue, pendingApproval{
			approvalID: event.ApprovalID,
			toolName:   event.ApprovalTool,
			args:       event.ApprovalArgs,
		})
		// If not currently waiting, show the next one
		if !a.waitingForApproval {
			a.showNextApproval()
		}
		a.scheduleRender()
		return listenEvents(a.eventCh)

	case agent.EventTurnEnd:
		if event.ContextUsage != nil {
			a.contextUsage = event.ContextUsage
		}
		return listenEvents(a.eventCh)

	case agent.EventDone:
		a.isThinking = false
		a.autoScroll = true
		if event.ContextUsage != nil {
			a.contextUsage = event.ContextUsage
		}
		return listenEvents(a.eventCh)

	case agent.EventError:
		a.isThinking = false
		if event.Error != nil {
			a.addMessage(errorStyle.Render("Error: ") + event.Error.Error())
		}
		return listenEvents(a.eventCh)

	case agent.EventUsage:
		if event.ContextUsage != nil {
			a.contextUsage = event.ContextUsage
		}
		if event.Usage != nil {
			costStr := fmt.Sprintf("Tokens: %d↓/%d↑ $%.4f",
				event.Usage.Input, event.Usage.Output, event.Usage.Cost.Total)
			a.addMessage(statusStyle.Render(costStr))
		}
		return listenEvents(a.eventCh)

	case agent.EventCompactionStart:
		a.addMessage(statusStyle.Render("⏳ Compacting context..."))
		return listenEvents(a.eventCh)

	case agent.EventCompactionEnd:
		if event.Error != nil {
			a.addMessage(errorStyle.Render("Compaction failed: ") + event.Error.Error())
		} else if event.StatusMessage != "" {
			a.addMessage(statusStyle.Render("✅ " + event.StatusMessage))
		} else {
			a.addMessage(statusStyle.Render("✅ Context compacted"))
		}
		return listenEvents(a.eventCh)

	default:
		return listenEvents(a.eventCh)
	}
}

func isAssistantMsg(s string) bool {
	return strings.Contains(s, "Assistant: ")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Message types
type agentStartMsg struct{ input string }
type agentEventMsg struct{ event agent.Event }
type agentDoneMsg struct{ err error }

func listenEvents(eventCh <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-eventCh
		if !ok {
			return agentDoneMsg{}
		}
		if event.Type == agent.EventError {
			return agentDoneMsg{err: event.Error}
		}
		if event.Type == agent.EventDone {
			return agentDoneMsg{}
		}
		return agentEventMsg{event: event}
	}
}
