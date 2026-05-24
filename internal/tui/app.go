package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/skills"
	"github.com/startvibecoding/vibecoding/internal/tools"
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

	toolModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

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
	toolCallID  string // Unique tool call ID for precise matching
	toolName    string
	toolArgs    map[string]any // Tool call arguments
	summary     string         // Short summary for collapsed view
	fullContent string         // Full content for expanded view
	msgIndex    int            // Index in a.messages where this tool message lives
}

// App is the main TUI application.
type App struct {
	provider     provider.Provider
	model        *provider.Model
	settings     *config.Settings
	session      *session.Manager
	registry     *tools.Registry
	sandboxInfo  string
	mode         string
	extraContext string
	skillsMgr    *skills.Manager

	// Skills state: base extraContext (without skills) and active skill names
	baseExtraContext string            // extraContext without skill content
	activeSkills     map[string]string // skill name -> skill context string

	// UI Components
	input textinput.Model
	timer stopwatch.Model

	// State
	messages    []string
	toolResults []toolResult // Store tool results for expansion
	isThinking  bool
	agent       *agent.Agent
	eventCh     <-chan agent.Event
	width       int
	height      int
	ready       bool

	// Paste markers storage
	pasteCounter int
	pastes       map[int]string

	// Input queue for batching
	inputQueue     []InputEvent
	inputQueueMu   sync.Mutex
	lastInputTime  time.Time
	inputBatchSize int
	inputDelay     time.Duration

	// Live content stays in the managed Bubble Tea view while it is streaming.
	// Completed transcript entries are printed through Bubble Tea's unmanaged
	// print path so the terminal's native scrollback owns history.
	liveContent   string
	pendingPrints []string

	// Initial message to display
	initialMessage string

	// Tool output modal
	toolModalOpen         bool
	toolModalOffset       int
	toolModalPinnedBottom bool

	// Context usage
	contextUsage *ctxpkg.ContextUsage

	// Cache usage tracking (cumulative)
	totalInputTokens int
	totalCacheRead   int
	totalCacheWrite  int

	// Spinner state
	spinnerIndex int
	requestStart time.Time
	lastDuration time.Duration

	// Session history
	sessionMu          sync.Mutex
	historyLoaded      bool
	agentHistoryLoaded bool

	// Render throttling
	lastRender     time.Time
	renderPending  bool
	renderMu       sync.Mutex
	renderInterval time.Duration

	// Approval state
	waitingForApproval bool
	pendingApprovalID  string
	approvalQueue      []pendingApproval

	// Current streaming message indices (-1 = none)
	currentAssistantIdx int
	currentThinkIdx     int
	printedMessageIdx   map[int]bool

	// Markdown rendering for assistant messages
	mdRenderer        *glamour.TermRenderer
	assistantRaw      map[int]string // message index -> raw markdown content
	assistantRendered map[int]string // message index -> glamour-rendered content
	assistantDirty    map[int]bool   // message index -> needs re-render

	// Bubble Tea program used to marshal deferred renders back onto the UI goroutine.
	program *tea.Program
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

	// Determine initial mode: use provided mode, fall back to settings default
	mode := initialMode
	if mode == "" {
		mode = settings.DefaultMode
	}
	if mode == "" {
		mode = "agent"
	}

	app := &App{
		provider:            p,
		model:               model,
		settings:            settings,
		session:             sess,
		registry:            registry,
		sandboxInfo:         sandboxInfo,
		mode:                mode,
		extraContext:        extraContext,
		baseExtraContext:    extraContext,
		activeSkills:        make(map[string]string),
		skillsMgr:           skillsMgr,
		input:               input,
		timer:               stopwatch.NewWithInterval(time.Second),
		pastes:              make(map[int]string),
		inputQueue:          make([]InputEvent, 0, 100),
		inputBatchSize:      10,
		inputDelay:          16 * time.Millisecond, // ~60fps
		renderInterval:      16 * time.Millisecond, // ~60fps
		currentAssistantIdx: -1,
		currentThinkIdx:     -1,
		printedMessageIdx:   make(map[int]bool),
		assistantRaw:        make(map[int]string),
		assistantRendered:   make(map[int]string),
		assistantDirty:      make(map[int]bool),
	}

	// Initialize markdown renderer (best-effort; may fail in test/headless env)
	if r, err := glamour.NewTermRenderer(glamour.WithAutoStyle()); err == nil {
		app.mdRenderer = r
	}

	return app
}

// SetInitialMessage sets an initial message to display when the TUI starts.
func (a *App) SetInitialMessage(msg string) {
	a.initialMessage = msg
}

// SetProgram stores the Bubble Tea program used for deferred UI updates.
func (a *App) SetProgram(p *tea.Program) {
	a.program = p
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
	var cmds []tea.Cmd

	// Show initial message if set
	if a.initialMessage != "" {
		a.messages = append(a.messages, statusStyle.Render(a.initialMessage))
		a.printHistory(a.messages[len(a.messages)-1])
	}

	// Load history messages from session
	a.LoadHistoryMessages()
	a.updateViewportContent()

	cmds = append(cmds, a.flushPendingPrints(), textinput.Blink, a.processInputQueue())
	return tea.Batch(cmds...)
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

	case stopwatch.TickMsg, stopwatch.StartStopMsg, stopwatch.ResetMsg:
		var timerCmd tea.Cmd
		a.timer, timerCmd = a.timer.Update(msg)
		if timerCmd != nil {
			cmds = append(cmds, timerCmd)
		}
		return a, tea.Batch(cmds...)

	case renderRequestMsg:
		a.updateViewportContent()
		return a, nil

	case tea.KeyMsg:
		if a.toolModalOpen {
			switch msg.String() {
			case "esc", "ctrl+o", "q":
				a.closeToolModal()
				return a, nil
			case "up":
				a.scrollToolModal(-1)
				return a, nil
			case "down":
				a.scrollToolModal(1)
				return a, nil
			case "pgup":
				a.scrollToolModal(-a.toolModalPageSize())
				return a, nil
			case "pgdown":
				a.scrollToolModal(a.toolModalPageSize())
				return a, nil
			case "home":
				a.toolModalOffset = 0
				a.toolModalPinnedBottom = false
				return a, nil
			case "end":
				a.toolModalOffset = a.maxToolModalOffset()
				a.toolModalPinnedBottom = true
				return a, nil
			}
			return a, nil
		}

		// Special keys are processed immediately; regular text input is batched.
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "esc":
			if a.isThinking {
				if a.agent != nil {
					a.agent.Abort()
					a.agent = nil // Reset agent so next request creates a fresh one with new abort channel
					a.agentHistoryLoaded = false
				}
				a.inputQueueMu.Lock()
				a.inputQueue = a.inputQueue[:0]
				a.lastInputTime = time.Time{}
				a.inputQueueMu.Unlock()
				a.input.Reset()
				a.isThinking = false
				a.finishRequestTimer()
				a.addMessage(statusStyle.Render("⏹ Aborted"))
				return a, a.timer.Stop()
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
			return a, nil
		case "pgdown":
			return a, nil
		case "home":
			return a, nil
		case "end":
			return a, nil
		case "ctrl+o":
			a.openLatestToolModal()
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

		a.queueInput(msg)
		return a, nil

	case agentStartMsg:
		a.isThinking = true
		a.spinnerIndex = 0
		a.requestStart = time.Now()
		a.lastDuration = 0
		a.addMessage(userStyle.Render("You: ") + msg.input)
		return a, tea.Batch(a.listenAgentEvents(), a.tickSpinner(), a.timer.Reset(), a.timer.Start())

	case agentEventMsg:
		return a, a.handleAgentEvent(msg.event)

	case agentDoneMsg:
		a.isThinking = false
		a.finishRequestTimer()
		if msg.err != nil {
			a.addMessage(errorStyle.Render("Error: ") + msg.err.Error())
		}
		return a, a.timer.Stop()
	}

	// Update components
	var inputCmd tea.Cmd
	a.input, inputCmd = a.input.Update(msg)

	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
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

// scheduleRender schedules a render update with throttling.
// If called inside the throttle window, a delayed render is scheduled
// so the pending update is not lost.
func (a *App) scheduleRender() {
	a.renderMu.Lock()
	defer a.renderMu.Unlock()

	now := time.Now()
	if now.Sub(a.lastRender) < a.renderInterval {
		if !a.renderPending {
			a.renderPending = true
			remaining := a.renderInterval - now.Sub(a.lastRender)
			time.AfterFunc(remaining, func() {
				a.renderMu.Lock()
				wasPending := a.renderPending
				if wasPending {
					a.lastRender = time.Now()
					a.renderPending = false
				}
				a.renderMu.Unlock()
				if wasPending {
					if a.program != nil {
						a.program.Send(renderRequestMsg{})
					}
				}
			})
		}
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
	if a.toolModalOpen {
		return lipgloss.JoinVertical(lipgloss.Left, a.renderToolModal(), footer)
	}

	parts := []string{a.input.View(), footer}
	if a.liveContent != "" {
		parts = append([]string{a.liveContent}, parts...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
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
	used := make(map[int]bool)
	for pasteId, content := range a.pastes {
		// Match markers like [paste #1 +15 lines] or [paste #2 1234 chars]
		markerLine := fmt.Sprintf("+%d lines", strings.Count(content, "\n")+1)
		markerChar := fmt.Sprintf("%d chars", len(content))

		// Try line marker
		marker1 := fmt.Sprintf("[paste #%d %s]", pasteId, markerLine)
		if strings.Contains(result, marker1) {
			result = strings.ReplaceAll(result, marker1, content)
			used[pasteId] = true
			continue
		}

		// Try char marker
		marker2 := fmt.Sprintf("[paste #%d %s]", pasteId, markerChar)
		if strings.Contains(result, marker2) {
			result = strings.ReplaceAll(result, marker2, content)
			used[pasteId] = true
		}
	}

	// Clean up only used pastes
	for id := range used {
		delete(a.pastes, id)
	}

	return result
}

func (a *App) updateViewportContent() {
	a.liveContent = ""
	if a.currentThinkIdx >= 0 && a.currentThinkIdx < len(a.messages) {
		a.liveContent = a.messages[a.currentThinkIdx]
	}
	if a.currentAssistantIdx >= 0 {
		assistant := a.renderAssistantMessage(a.currentAssistantIdx)
		if assistant != "" {
			if a.liveContent != "" {
				a.liveContent += "\n\n"
			}
			a.liveContent += assistant
		}
	}
}

func (a *App) renderMessageAt(idx int) string {
	for i, tr := range a.toolResults {
		if tr.msgIndex == idx {
			return a.renderToolResult(a.toolResults[i])
		}
	}
	if _, ok := a.assistantRaw[idx]; ok {
		return a.renderAssistantMessage(idx)
	}
	if idx >= 0 && idx < len(a.messages) {
		return a.messages[idx]
	}
	return ""
}

func (a *App) renderToolResult(result toolResult) string {
	summary := result.summary
	if summary == "" {
		summary = "..."
	}
	return toolStyle.Render(fmt.Sprintf("%s %s", formatToolHeader(result), summary))
}

func (a *App) renderAssistantMessage(idx int) string {
	raw := a.assistantRaw[idx]
	if raw == "" {
		return ""
	}
	if a.assistantDirty[idx] && a.mdRenderer != nil {
		rendered, err := a.mdRenderer.Render(raw)
		if err == nil {
			a.assistantRendered[idx] = rendered
		}
		a.assistantDirty[idx] = false
	}
	prefix := assistantStyle.Render("Assistant: ")
	if rendered, ok := a.assistantRendered[idx]; ok && rendered != "" {
		return prefix + rendered
	}
	return prefix + raw
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

func formatToolHeader(result toolResult) string {
	path := toolPath(result.toolArgs)
	if path == "" {
		return fmt.Sprintf("🔧 [%s]", result.toolName)
	}
	return fmt.Sprintf("🔧 [%s] %s", result.toolName, path)
}

func toolPath(args map[string]any) string {
	if args == nil {
		return ""
	}
	path, _ := args["path"].(string)
	return path
}

func summarizeWriteToolResult(result string) string {
	lines := strings.Split(result, "\n")
	diff := ""
	deleted := ""
	added := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "Diff: ") {
			diff = strings.TrimPrefix(line, "Diff: ")
			continue
		}
		if strings.HasPrefix(line, "- lines: ") {
			deleted = strings.TrimPrefix(line, "- lines: ")
			continue
		}
		if strings.HasPrefix(line, "+ lines: ") {
			added = strings.TrimPrefix(line, "+ lines: ")
		}
	}
	if diff != "" && (deleted != "" || added != "") {
		return fmt.Sprintf("%s (-%s +%s)", diff, deleted, added)
	}
	if diff != "" {
		return diff
	}
	return "Written"
}

func (a *App) openLatestToolModal() {
	a.toolModalOpen = true
	a.toolModalPinnedBottom = true
	a.toolModalOffset = a.maxToolModalOffset()
}

func (a *App) closeToolModal() {
	a.toolModalOpen = false
	a.toolModalOffset = 0
	a.toolModalPinnedBottom = false
}

func formatToolModalContent(result toolResult) string {
	var parts []string
	if result.toolArgs != nil {
		if args := formatToolArgs(result.toolName, result.toolArgs); strings.TrimSpace(args) != "" {
			parts = append(parts, args)
		}
	}
	if result.fullContent != "" {
		parts = append(parts, "---", result.fullContent)
	}
	if len(parts) == 0 {
		return "(no output)"
	}
	return strings.Join(parts, "\n")
}

func (a *App) renderExpandedTranscript() string {
	var parts []string
	for i := range a.messages {
		msg := a.renderExpandedMessageAt(i)
		if strings.TrimSpace(msg) != "" {
			parts = append(parts, msg)
		}
	}
	if len(parts) == 0 {
		return "(no conversation yet)"
	}
	return strings.Join(parts, "\n\n")
}

func (a *App) renderExpandedMessageAt(idx int) string {
	for i, tr := range a.toolResults {
		if tr.msgIndex == idx {
			return a.renderExpandedToolResult(a.toolResults[i])
		}
	}
	if _, ok := a.assistantRaw[idx]; ok {
		return a.renderAssistantMessage(idx)
	}
	if idx >= 0 && idx < len(a.messages) {
		return a.messages[idx]
	}
	return ""
}

func (a *App) renderExpandedToolResult(result toolResult) string {
	content := formatToolHeader(result)
	details := formatToolModalContent(result)
	if strings.TrimSpace(details) != "" {
		content += "\n" + details
	}
	return toolStyle.Render(content)
}

func (a *App) renderToolModal() string {
	width := a.width - 4
	if width < 20 {
		width = 20
	}
	height := a.toolModalPageSize()
	contentText := a.renderExpandedTranscript()
	lines := strings.Split(contentText, "\n")
	maxOffset := a.maxToolModalOffset()
	if a.toolModalPinnedBottom {
		a.toolModalOffset = maxOffset
	}
	if a.toolModalOffset > maxOffset {
		a.toolModalOffset = maxOffset
	}
	end := a.toolModalOffset + height
	if end > len(lines) {
		end = len(lines)
	}
	visible := strings.Join(lines[a.toolModalOffset:end], "\n")
	if visible == "" {
		visible = " "
	}
	position := fmt.Sprintf("lines %d-%d/%d", a.toolModalOffset+1, end, len(lines))
	if len(lines) == 0 {
		position = "lines 0-0/0"
	}
	title := fmt.Sprintf("Expanded transcript  %s  PgUp/PgDn Up/Down Esc", position)
	content := title + "\n" + strings.Repeat("─", minInt(width-2, len(title))) + "\n" + visible
	return toolModalStyle.Width(width).Height(height + 3).Render(content)
}

func (a *App) scrollToolModal(delta int) {
	a.toolModalOffset += delta
	if a.toolModalOffset < 0 {
		a.toolModalOffset = 0
	}
	if maxOffset := a.maxToolModalOffset(); a.toolModalOffset > maxOffset {
		a.toolModalOffset = maxOffset
	}
	a.toolModalPinnedBottom = a.toolModalOffset == a.maxToolModalOffset()
}

func (a *App) toolModalPageSize() int {
	pageSize := a.height - 6
	if pageSize < 3 {
		return 3
	}
	return pageSize
}

func (a *App) maxToolModalOffset() int {
	lines := strings.Split(a.renderExpandedTranscript(), "\n")
	maxOffset := len(lines) - a.toolModalPageSize()
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatCachePercent calculates and returns the cache hit rate string, or empty string if no data.
// The denominator uses the full input footprint so OpenAI and Anthropic can share the same
// cache ratio display after their provider-specific usage fields are normalized.
func (a *App) formatCachePercent() string {
	switch {
	case a.totalInputTokens > 0:
		pct := float64(a.totalCacheRead) / float64(a.totalInputTokens) * 100
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("Cache: %.0f%%", pct)
	case a.totalCacheRead > 0:
		return fmt.Sprintf("CacheRead: %d", a.totalCacheRead)
	case a.totalCacheWrite > 0:
		return fmt.Sprintf("CacheWrite: %d", a.totalCacheWrite)
	default:
		return ""
	}
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

	// Build cache hit rate string, highlighting when hit rate >= 50%
	cacheStr := ""
	if cachePercentStr := a.formatCachePercent(); cachePercentStr != "" {
		if a.totalInputTokens > 0 && float64(a.totalCacheRead)/float64(a.totalInputTokens)*100 >= 50 {
			cacheStr = " | " + statusStyle.Render(cachePercentStr)
		} else {
			cacheStr = " | " + cachePercentStr
		}
	}

	status := fmt.Sprintf(" %s | %s | %s%s%s", modeStr, modelName, cwd, contextStr, cacheStr)
	if a.isThinking {
		status += " | " + spinnerChars[a.spinnerIndex] + " " + formatDuration(a.timer.Elapsed())
	} else {
		if a.lastDuration > 0 {
			status += " | last " + formatDuration(a.lastDuration)
		}
		if a.toolModalOpen {
			status += " | Esc/Ctrl+O:close PgUp/PgDn Up/Down:scroll"
		} else {
			status += " | Tab:mode Esc:abort Ctrl+O:details"
		}
	}

	return footerStyle.Width(a.width).Render(status)
}

func (a *App) addMessage(msg string) {
	a.messages = append(a.messages, msg)
	a.printHistory(msg)
}

func (a *App) printHistory(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	if a.program != nil {
		go a.program.Println(msg)
		return
	}
	a.pendingPrints = append(a.pendingPrints, msg)
}

func (a *App) printMessageOnce(idx int) {
	if idx < 0 || a.printedMessageIdx[idx] {
		return
	}
	if a.printedMessageIdx == nil {
		a.printedMessageIdx = make(map[int]bool)
	}
	msg := a.renderMessageAt(idx)
	if strings.TrimSpace(msg) == "" {
		return
	}
	a.printedMessageIdx[idx] = true
	a.printHistory(msg)
}

func (a *App) commitActiveStream() {
	hadActive := a.currentThinkIdx >= 0 || a.currentAssistantIdx >= 0
	if a.currentThinkIdx >= 0 {
		a.printMessageOnce(a.currentThinkIdx)
	}
	if a.currentAssistantIdx >= 0 {
		a.printMessageOnce(a.currentAssistantIdx)
	}
	if hadActive {
		a.currentThinkIdx = -1
		a.currentAssistantIdx = -1
		a.updateViewportContent()
	}
}

func (a *App) flushPendingPrints() tea.Cmd {
	if len(a.pendingPrints) == 0 {
		return nil
	}
	prints := append([]string(nil), a.pendingPrints...)
	a.pendingPrints = nil

	cmds := make([]tea.Cmd, 0, len(prints))
	for _, msg := range prints {
		cmds = append(cmds, tea.Println(msg))
	}
	return tea.Batch(cmds...)
}

func (a *App) finishRequestTimer() {
	if !a.requestStart.IsZero() {
		a.lastDuration = time.Since(a.requestStart)
		a.requestStart = time.Time{}
		return
	}
	if elapsed := a.timer.Elapsed(); elapsed > 0 {
		a.lastDuration = elapsed
	}
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
		a.agentHistoryLoaded = false
		a.inputQueueMu.Lock()
		a.inputQueue = a.inputQueue[:0]
		a.lastInputTime = time.Time{}
		a.inputQueueMu.Unlock()
		a.isThinking = false
		a.finishRequestTimer()
		a.addMessage(statusStyle.Render("⏹ Aborted (mode change)"))
	} else {
		a.agent = nil
		a.agentHistoryLoaded = false
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
		a.sessionMu.Lock()
		agentHistoryLoaded := a.agentHistoryLoaded
		a.sessionMu.Unlock()
		if a.session != nil && !agentHistoryLoaded {
			a.sessionMu.Lock()
			historyMessages := a.session.GetMessages()
			a.sessionMu.Unlock()

			if len(historyMessages) > 0 {
				a.agent.LoadHistoryMessages(historyMessages)
				a.sessionMu.Lock()
				a.agentHistoryLoaded = true
				a.sessionMu.Unlock()
			}
		}
	}

	ctx := context.Background()
	a.eventCh = a.agent.Run(ctx, input)

	return tea.Batch(
		func() tea.Msg { return agentStartMsg{input: input} },
		a.listenAgentEvents(),
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
					a.agentHistoryLoaded = false
					a.inputQueueMu.Lock()
					a.inputQueue = a.inputQueue[:0]
					a.lastInputTime = time.Time{}
					a.inputQueueMu.Unlock()
					a.isThinking = false
					a.finishRequestTimer()
					a.addMessage(statusStyle.Render("⏹ Aborted (mode change)"))
				} else {
					a.agent = nil
					a.agentHistoryLoaded = false
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
		if len(parts) > 1 {
			// Switch model
			modelID := parts[1]
			newModel := a.provider.GetModel(modelID)
			if newModel == nil {
				a.addMessage(errorStyle.Render(fmt.Sprintf("Model not found: %s", modelID)))
				// List available models
				models := a.provider.Models()
				if len(models) > 0 {
					var sb strings.Builder
					sb.WriteString("Available models:\n")
					for _, m := range models {
						marker := " "
						if m.ID == a.model.ID {
							marker = "*"
						}
						sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", marker, m.Name, m.ID))
					}
					a.addMessage(statusStyle.Render(sb.String()))
				}
				return nil
			}
			a.model = newModel
			// Reset agent so next message uses the new model
			a.agent = nil
			a.agentHistoryLoaded = false
			a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Model switched to: %s (%s)", newModel.Name, newModel.ID)))
		} else {
			// Show current model and available models
			a.addMessage(statusStyle.Render(fmt.Sprintf("Current model: %s (%s)", a.model.Name, a.model.ID)))
			models := a.provider.Models()
			if len(models) > 0 {
				var sb strings.Builder
				sb.WriteString("Available models (use /model <id> to switch):\n")
				for _, m := range models {
					marker := " "
					if m.ID == a.model.ID {
						marker = "*"
					}
					sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", marker, m.Name, m.ID))
				}
				a.addMessage(statusStyle.Render(sb.String()))
			}
		}
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
		a.agentHistoryLoaded = false
		a.contextUsage = nil
		a.totalInputTokens = 0
		a.totalCacheRead = 0
		a.totalCacheWrite = 0
		a.pastes = make(map[int]string)
		a.pasteCounter = 0
		a.activeSkills = make(map[string]string)
		a.extraContext = a.baseExtraContext
		a.updateViewportContent()
		a.addMessage(statusStyle.Render("✅ Conversation cleared"))
	case "/quit":
		return tea.Quit
	case "/sessions":
		a.handleSessionsCommand(parts)
	case "/help":
		a.addMessage(statusStyle.Render("Commands:"))
		a.addMessage(statusStyle.Render("  /mode [plan|agent|yolo] - Switch or show mode"))
		a.addMessage(statusStyle.Render("  /model [model_id]       - Switch or show model"))
		a.addMessage(statusStyle.Render("  /skills                 - List available skills"))
		a.addMessage(statusStyle.Render("  /skill <name>           - Activate a skill"))
		a.addMessage(statusStyle.Render("  /clear                  - Clear conversation"))
		a.addMessage(statusStyle.Render("  /sessions               - List sessions for this project"))
		a.addMessage(statusStyle.Render("  /sessions ls            - List sessions"))
		a.addMessage(statusStyle.Render("  /sessions set <id>      - Switch to session"))
		a.addMessage(statusStyle.Render("  /sessions clear         - Create a new session"))
		a.addMessage(statusStyle.Render("  /sessions del <id>      - Delete a session"))
		a.addMessage(statusStyle.Render("  /quit                   - Exit"))
		a.addMessage(statusStyle.Render("  /help                   - Show this help"))
		a.addMessage(statusStyle.Render(""))
		a.addMessage(statusStyle.Render("Keyboard shortcuts:"))
		a.addMessage(statusStyle.Render("  Tab       - Cycle mode (plan/agent/yolo)"))
		a.addMessage(statusStyle.Render("  Esc       - Abort current operation"))
		a.addMessage(statusStyle.Render("  Ctrl+O    - Open latest tool details"))
		a.addMessage(statusStyle.Render("  PgUp/PgDn - Page tool details when open"))
		a.addMessage(statusStyle.Render("  Mouse wheel - Scroll terminal history"))
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
	a.agentHistoryLoaded = false

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

// getSessionDir returns the session directory path.
func (a *App) getSessionDir() string {
	if a.settings != nil {
		return a.settings.GetSessionDir()
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".vibecoding", "sessions")
}

// getCurrentSessionID returns the current session's short ID (first 8 chars).
func (a *App) getCurrentSessionID() string {
	if a.session == nil {
		return ""
	}
	file := a.session.GetFile()
	if file == "" {
		return ""
	}
	base := filepath.Base(file)
	base = strings.TrimSuffix(base, ".jsonl")
	if idx := strings.Index(base, "_"); idx >= 0 {
		return base[idx+1:]
	}
	return ""
}

// handleSessionsCommand handles the /sessions command and its subcommands.
func (a *App) handleSessionsCommand(parts []string) {
	sub := "ls"
	if len(parts) > 1 {
		sub = strings.ToLower(parts[1])
	}

	switch sub {
	case "ls", "list":
		a.sessionsList()
	case "set", "switch", "use":
		if len(parts) < 3 {
			a.addMessage(errorStyle.Render("Usage: /sessions set <id>"))
			return
		}
		a.sessionsSet(parts[2])
	case "clear", "new":
		a.sessionsClear()
	case "del", "delete", "rm":
		if len(parts) < 3 {
			a.addMessage(errorStyle.Render("Usage: /sessions del <id>"))
			return
		}
		a.sessionsDel(parts[2])
	default:
		a.addMessage(errorStyle.Render(fmt.Sprintf("Unknown subcommand: %s. Use ls, set, clear, del.", sub)))
	}
}

// sessionsList lists all sessions for the current project directory.
func (a *App) sessionsList() {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	sessionDir := a.getSessionDir()
	details, err := session.ListForDirDetailed(cwd, sessionDir)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error listing sessions: %v", err)))
		return
	}

	if len(details) == 0 {
		a.addMessage(statusStyle.Render("No sessions found for this project."))
		return
	}

	currentID := a.getCurrentSessionID()

	var sb strings.Builder
	sb.WriteString("Sessions for this project:\n\n")
	for _, d := range details {
		marker := " "
		if d.ID == currentID {
			marker = "*"
		}
		age := formatAge(d.ModTime)
		preview := ""
		if d.Preview != "" {
			preview = " - " + d.Preview
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s  %d msgs  %s%s\n",
			marker, d.ID, d.MessageCount, age, preview))
	}
	sb.WriteString("\nUse /sessions set <id> to switch. * = current session.")
	a.addMessage(statusStyle.Render(sb.String()))
}

// sessionsSet switches to a different session by ID prefix.
func (a *App) sessionsSet(id string) {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	// Don't switch to the same session
	if id == a.getCurrentSessionID() {
		a.addMessage(statusStyle.Render("Already on this session."))
		return
	}

	sessionDir := a.getSessionDir()
	details, err := session.ListForDirDetailed(cwd, sessionDir)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		return
	}

	// Find matching session by ID prefix
	var match *session.SessionDetail
	for i, d := range details {
		if strings.HasPrefix(d.ID, id) {
			if match != nil {
				a.addMessage(errorStyle.Render(fmt.Sprintf("Ambiguous ID '%s'. Be more specific.", id)))
				return
			}
			match = &details[i]
		}
	}

	if match == nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("No session found matching '%s'.", id)))
		return
	}

	// Open the session
	newSess, err := session.Open(match.Path)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error opening session: %v", err)))
		return
	}

	// Switch session
	a.session = newSess
	a.historyLoaded = false
	a.agentHistoryLoaded = false

	// Reset agent and UI state
	a.agent = nil
	a.messages = nil
	a.toolResults = nil
	a.contextUsage = nil
	a.totalInputTokens = 0
	a.totalCacheRead = 0
	a.totalCacheWrite = 0
	a.assistantRaw = make(map[int]string)
	a.assistantRendered = make(map[int]string)
	a.assistantDirty = make(map[int]bool)
	a.printedMessageIdx = make(map[int]bool)
	a.currentAssistantIdx = -1
	a.currentThinkIdx = -1

	// Load history messages from the new session
	a.LoadHistoryMessages()
	a.updateViewportContent()

	a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Switched to session %s (%d msgs)",
		match.ID, match.MessageCount)))
}

// sessionsClear creates a new session, starting fresh.
func (a *App) sessionsClear() {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	sessionDir := a.getSessionDir()
	newSess := session.New(cwd, sessionDir)
	if err := newSess.Init(); err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error creating session: %v", err)))
		return
	}

	a.session = newSess
	a.historyLoaded = false
	a.agentHistoryLoaded = false

	// Reset agent and UI state
	a.agent = nil
	a.messages = nil
	a.toolResults = nil
	a.contextUsage = nil
	a.totalInputTokens = 0
	a.totalCacheRead = 0
	a.totalCacheWrite = 0
	a.assistantRaw = make(map[int]string)
	a.assistantRendered = make(map[int]string)
	a.assistantDirty = make(map[int]bool)
	a.printedMessageIdx = make(map[int]bool)
	a.currentAssistantIdx = -1
	a.currentThinkIdx = -1
	a.updateViewportContent()

	a.addMessage(statusStyle.Render("✅ New session created."))
}

// sessionsDel deletes a session by ID prefix.
func (a *App) sessionsDel(id string) {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	// Don't delete the current session
	if id == a.getCurrentSessionID() {
		a.addMessage(errorStyle.Render("Cannot delete the current session. Switch to another session first, or use /sessions clear to start fresh."))
		return
	}

	sessionDir := a.getSessionDir()
	details, err := session.ListForDirDetailed(cwd, sessionDir)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		return
	}

	// Find matching session by ID prefix
	var match *session.SessionDetail
	for i, d := range details {
		if strings.HasPrefix(d.ID, id) {
			if match != nil {
				a.addMessage(errorStyle.Render(fmt.Sprintf("Ambiguous ID '%s'. Be more specific.", id)))
				return
			}
			match = &details[i]
		}
	}

	if match == nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("No session found matching '%s'.", id)))
		return
	}

	if err := session.DeleteSession(match.Path); err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error deleting session: %v", err)))
		return
	}

	a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Deleted session %s.", match.ID)))
}

// formatAge returns a human-readable age string for a time.
func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2006-01-02")
	}
}

func (a *App) handleAgentEvent(event agent.Event) tea.Cmd {
	switch event.Type {
	case agent.EventTextDelta:
		if a.currentAssistantIdx >= 0 && a.currentAssistantIdx < len(a.messages) {
			a.assistantRaw[a.currentAssistantIdx] += event.TextDelta
		} else {
			a.currentAssistantIdx = len(a.messages)
			a.assistantRaw[a.currentAssistantIdx] = event.TextDelta
			// placeholder; actual display is built in updateViewportContent
			a.messages = append(a.messages, "")
		}
		a.assistantDirty[a.currentAssistantIdx] = true
		a.scheduleRender()
		return a.listenAgentEvents()

	case agent.EventThinkDelta:
		if a.currentThinkIdx >= 0 && a.currentThinkIdx < len(a.messages) {
			a.messages[a.currentThinkIdx] += event.ThinkDelta
		} else {
			a.currentThinkIdx = len(a.messages)
			a.messages = append(a.messages, thinkStyle.Render("think: ")+event.ThinkDelta)
		}
		a.scheduleRender()
		return a.listenAgentEvents()

	case agent.EventTurnStart:
		// Reserve display slots before streaming deltas arrive so later tool output
		// cannot shift the assistant message index underneath us.
		a.currentAssistantIdx = len(a.messages)
		a.assistantRaw[a.currentAssistantIdx] = ""
		a.messages = append(a.messages, "")
		return a.listenAgentEvents()

	case agent.EventToolCall:
		if event.ToolCall != nil {
			a.commitActiveStream()
			// Store tool args for later display
			msgIdx := len(a.messages) // Will be the index after append
			a.toolResults = append(a.toolResults, toolResult{
				toolCallID: event.ToolCall.ID,
				toolName:   event.ToolCall.Name,
				toolArgs:   event.ToolArgs,
				msgIndex:   msgIdx,
			})
			a.messages = append(a.messages, "")
			a.printHistory(a.renderMessageAt(msgIdx))
		}
		return a.listenAgentEvents()

	case agent.EventToolResult:
		// Find the matching tool result entry and update it
		foundIdx := -1
		for j := len(a.toolResults) - 1; j >= 0; j-- {
			if a.toolResults[j].toolCallID == event.ToolCallID {
				foundIdx = j
				a.toolResults[j].fullContent = event.ToolResult

				// Create summary based on tool type
				switch event.ToolName {
				case "bash":
					a.toolResults[j].summary = event.ToolResult
				case "read":
					lines := strings.Split(event.ToolResult, "\n")
					a.toolResults[j].summary = fmt.Sprintf("%d lines", len(lines))
				case "write":
					a.toolResults[j].summary = summarizeWriteToolResult(event.ToolResult)
				case "edit":
					a.toolResults[j].summary = "Applied"
				default:
					a.toolResults[j].summary = truncate(event.ToolResult, 50)
				}
				break
			}
		}

		// Update the message at the stored index
		if foundIdx >= 0 {
			idx := a.toolResults[foundIdx].msgIndex
			if idx >= 0 && idx < len(a.messages) {
				a.messages[idx] = ""
				a.printHistory(a.renderMessageAt(idx))
			}
		}
		a.scheduleRender()
		return a.listenAgentEvents()

	case agent.EventToolApprovalRequest:
		a.commitActiveStream()
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
		return a.listenAgentEvents()

	case agent.EventTurnEnd:
		if event.ContextUsage != nil {
			a.contextUsage = event.ContextUsage
		}
		if a.currentThinkIdx >= 0 {
			a.printMessageOnce(a.currentThinkIdx)
		}
		if a.currentAssistantIdx >= 0 {
			a.printMessageOnce(a.currentAssistantIdx)
		}
		a.currentAssistantIdx = -1
		a.currentThinkIdx = -1
		a.updateViewportContent()
		return a.listenAgentEvents()

	case agent.EventDone:
		a.isThinking = false
		a.finishRequestTimer()
		if event.ContextUsage != nil {
			a.contextUsage = event.ContextUsage
		}
		if a.currentThinkIdx >= 0 {
			a.printMessageOnce(a.currentThinkIdx)
		}
		if a.currentAssistantIdx >= 0 {
			a.printMessageOnce(a.currentAssistantIdx)
		}
		a.currentAssistantIdx = -1
		a.currentThinkIdx = -1
		a.updateViewportContent()
		return tea.Batch(a.timer.Stop(), a.listenAgentEvents())

	case agent.EventError:
		a.isThinking = false
		a.finishRequestTimer()
		if event.Error != nil {
			a.addMessage(errorStyle.Render("Error: ") + event.Error.Error())
		}
		a.currentAssistantIdx = -1
		a.currentThinkIdx = -1
		a.updateViewportContent()
		return tea.Batch(a.timer.Stop(), a.listenAgentEvents())

	case agent.EventUsage:
		if event.ContextUsage != nil {
			a.contextUsage = event.ContextUsage
		}
		if event.Usage != nil {
			// Accumulate cache stats
			a.totalInputTokens += event.Usage.TotalInputTokens()
			a.totalCacheRead += event.Usage.CacheRead
			a.totalCacheWrite += event.Usage.CacheWrite

			// Per-turn cache info
			cacheInfo := ""
			if info := event.Usage.CacheInfo(); info != "" {
				cacheInfo = " | " + info
			}
			costStr := fmt.Sprintf("Tokens: %d↓/%d↑ $%.4f%s",
				event.Usage.TotalInputTokens(), event.Usage.Output, event.Usage.Cost.Total, cacheInfo)
			a.addMessage(statusStyle.Render(costStr))
		}
		a.scheduleRender()
		return a.listenAgentEvents()

	case agent.EventCompactionStart:
		a.addMessage(statusStyle.Render("⏳ Compacting context..."))
		return a.listenAgentEvents()

	case agent.EventCompactionEnd:
		if event.Error != nil {
			a.addMessage(errorStyle.Render("Compaction failed: ") + event.Error.Error())
		} else if event.StatusMessage != "" {
			a.addMessage(statusStyle.Render("✅ " + event.StatusMessage))
		} else {
			a.addMessage(statusStyle.Render("✅ Context compacted"))
		}
		return a.listenAgentEvents()

	case agent.EventStatus:
		if event.StatusMessage != "" {
			a.addMessage(statusStyle.Render(event.StatusMessage))
		}
		return a.listenAgentEvents()

	case agent.EventMessageStart:
		if event.Message.Role == "user" && event.Message.Content != "" {
			a.addMessage(userStyle.Render("You: ") + event.Message.Content)
		}
		return a.listenAgentEvents()

	default:
		return a.listenAgentEvents()
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// Message types
type agentStartMsg struct{ input string }
type renderRequestMsg struct{}
