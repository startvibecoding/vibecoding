package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/cron"
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
	diff        *tools.FileDiff
	msgIndex    int // Index in a.messages where this tool message lives
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
	currentPlan  *tools.TaskPlan

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

	// Prompt input history
	inputHistory         []string
	inputHistoryBrowsing bool
	inputHistoryIndex    int
	inputHistoryDraft    string

	// Render throttling
	lastRender     time.Time
	renderPending  bool
	renderMu       sync.Mutex
	renderInterval time.Duration

	// Approval state
	waitingForApproval bool
	pendingApprovalID  string
	approvalQueue      []pendingApproval

	// Question state
	waitingForQuestion bool
	pendingQuestionID  string
	questionQueue      []pendingQuestion

	// Multi-agent state (Decision 8: default off)
	multiAgent  bool
	activeAgent agentpkg.AgentID
	agentMgr    *agent.AgentManager

	// Cron state
	cronStore cron.CronStore
	scheduler *cron.Scheduler

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
	// printCh feeds a single drain goroutine that calls program.Println in FIFO
	// order. Using one goroutine instead of `go program.Println(...)` per call
	// prevents racing sends on Bubble Tea's unbuffered message channel, which
	// would reorder messages and visually drop newlines between them.
	printCh   chan string
	printOnce sync.Once
}

// pendingApproval holds a queued approval request.
type pendingApproval struct {
	approvalID string
	toolName   string
	args       map[string]any
}

// pendingQuestion holds a queued question request.
type pendingQuestion struct {
	questionID string
	question   string
	options    []string
	context    string
}

// NewApp creates a new TUI application.
func NewApp(p provider.Provider, model *provider.Model, settings *config.Settings, sess *session.Manager, registry *tools.Registry, sandboxInfo string, extraContext string, skillsMgr *skills.Manager, initialMode string, multiAgent bool, agentMgr *agent.AgentManager, cronStore cron.CronStore, scheduler *cron.Scheduler) *App {
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
		multiAgent:          multiAgent,
		agentMgr:            agentMgr,
		cronStore:           cronStore,
		scheduler:           scheduler,
	}

	app.configureMarkdownRenderer()

	return app
}

// SetInitialMessage sets an initial message to display when the TUI starts.
func (a *App) SetInitialMessage(msg string) {
	a.initialMessage = msg
}

// SetProgram stores the Bubble Tea program used for deferred UI updates and
// starts the single drain goroutine that serializes program.Println calls.
func (a *App) SetProgram(p *tea.Program) {
	a.program = p
	if p == nil {
		return
	}
	a.printOnce.Do(func() {
		a.printCh = make(chan string, 1024)
		go func(ch <-chan string, prog *tea.Program) {
			for msg := range ch {
				prog.Println(msg)
			}
		}(a.printCh, p)
	})
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
		oldWidth := a.width
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		a.input.Width = msg.Width - 4
		if oldWidth != a.width {
			a.configureMarkdownRenderer()
			a.markAssistantRenderedDirty()
		}

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
			switch {
			case msg.Type == tea.KeyEsc || msg.Type == tea.KeyCtrlO || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q"):
				a.closeToolModal()
				return a, nil
			case msg.Type == tea.KeyUp:
				a.scrollToolModal(-1)
				return a, nil
			case msg.Type == tea.KeyDown:
				a.scrollToolModal(1)
				return a, nil
			case msg.Type == tea.KeyPgUp:
				a.scrollToolModal(-a.toolModalPageSize())
				return a, nil
			case msg.Type == tea.KeyPgDown:
				a.scrollToolModal(a.toolModalPageSize())
				return a, nil
			case msg.Type == tea.KeyHome:
				a.toolModalOffset = 0
				a.toolModalPinnedBottom = false
				return a, nil
			case msg.Type == tea.KeyEnd:
				a.toolModalOffset = a.maxToolModalOffset()
				a.toolModalPinnedBottom = true
				return a, nil
			}
			return a, nil
		}

		// Special keys are processed immediately; regular text input is batched.
		switch msg.Type {
		case tea.KeyCtrlC:
			return a, tea.Quit
		case tea.KeyEsc:
			if a.isThinking || a.waitingForApproval || a.waitingForQuestion {
				if a.agent != nil {
					a.agent.Abort()
					a.agent = nil // Reset agent so next request creates a fresh one with new abort channel
					a.agentHistoryLoaded = false
				}
				a.clearApprovalState()
				a.clearQuestionState()
				a.inputQueueMu.Lock()
				a.inputQueue = a.inputQueue[:0]
				a.lastInputTime = time.Time{}
				a.inputQueueMu.Unlock()
				a.input.Reset()
				a.resetInputHistoryNavigation()
				a.isThinking = false
				a.finishRequestTimer()
				a.addMessage(statusStyle.Render("⏹ Aborted"))
				return a, a.timer.Stop()
			} else {
				a.input.Reset()
				a.resetInputHistoryNavigation()
			}
			return a, nil
		case tea.KeyEnter:
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
				a.resetInputHistoryNavigation()
				a.scheduleRender()
				return a, nil
			}

			// Check if waiting for a question
			if a.waitingForQuestion {
				if a.agent != nil {
					answer := strings.TrimSpace(input)
					// Check if it's a number selection
					var num int
					if _, err := fmt.Sscanf(answer, "%d", &num); err == nil && num > 0 {
						// Find the question to resolve options
						// Options are already shown; just pass the number as the answer
						a.agent.HandleQuestionResponse(a.pendingQuestionID, answer)
						a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Selected: [%s]", answer)))
					} else if answer != "" {
						// Custom text input
						a.agent.HandleQuestionResponse(a.pendingQuestionID, answer)
						a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Answer: %s", answer)))
					} else {
						// Empty input — re-prompt
						a.input.Reset()
						a.resetInputHistoryNavigation()
						a.scheduleRender()
						return a, nil
					}
				}
				// Show next queued question or clear waiting state
				if len(a.questionQueue) > 0 {
					a.showNextQuestion()
				} else {
					a.waitingForQuestion = false
					a.pendingQuestionID = ""
				}
				a.input.Reset()
				a.resetInputHistoryNavigation()
				a.scheduleRender()
				return a, nil
			}

			if input != "" {
				a.input.Reset()
				a.recordInputHistory(input)
				expandedInput := a.expandPasteMarkers(input)
				return a, a.processInput(expandedInput)
			}
			return a, nil
		case tea.KeyTab:
			a.cycleMode()
			return a, nil
		case tea.KeyPgUp:
			return a, nil
		case tea.KeyPgDown:
			return a, nil
		case tea.KeyUp:
			a.flushInputQueue()
			if a.navigateInputHistory(-1) {
				return a, nil
			}
		case tea.KeyDown:
			a.flushInputQueue()
			if a.navigateInputHistory(1) {
				return a, nil
			}
		case tea.KeyCtrlO:
			a.openLatestToolModal()
			return a, nil
		case tea.KeyCtrlP:
			a.toggleMultiAgent()
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
		a.resetInputHistoryNavigation()
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
		parts = append([]string{a.clampedLiveContent(footer)}, parts...)
	}
	if planPanel := a.renderPlanPanel(); planPanel != "" {
		parts = append([]string{planPanel}, parts...)
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
		assistant := a.renderLiveAssistantMessage(a.currentAssistantIdx)
		if assistant != "" {
			if a.liveContent != "" {
				a.liveContent += "\n\n"
			}
			a.liveContent += assistant
		}
	}
}

func (a *App) configureMarkdownRenderer() {
	width := a.assistantMarkdownWidth()
	if r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	); err == nil {
		a.mdRenderer = r
	}
}

func (a *App) assistantMarkdownWidth() int {
	width := a.width
	if width <= 0 {
		width = 80
	}
	width -= lipgloss.Width("Assistant: ")
	if width < 20 {
		return 20
	}
	return width
}

func (a *App) liveContentHeight(footer string) int {
	height := a.height
	if height <= 0 {
		return 0
	}
	used := lipgloss.Height(a.input.View()) + lipgloss.Height(footer)
	if panel := a.renderPlanPanel(); panel != "" {
		used += lipgloss.Height(panel)
	}
	available := height - used
	if available < 1 {
		return 1
	}
	return available
}

func (a *App) clampedLiveContent(footer string) string {
	maxLines := a.liveContentHeight(footer)
	if maxLines <= 0 {
		return a.liveContent
	}
	lines := strings.Split(strings.TrimRight(a.liveContent, "\n"), "\n")
	if len(lines) <= maxLines {
		return a.liveContent
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func (a *App) markAssistantRenderedDirty() {
	if a.assistantDirty == nil {
		a.assistantDirty = make(map[int]bool)
	}
	for idx := range a.assistantRendered {
		a.assistantDirty[idx] = true
	}
}

// Message types
type agentStartMsg struct{ input string }
type renderRequestMsg struct{}
