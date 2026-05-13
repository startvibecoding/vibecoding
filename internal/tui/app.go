package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fuckvibecoding/vibecoding/internal/agent"
	"github.com/fuckvibecoding/vibecoding/internal/config"
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
)

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

	// UI Components
	viewport viewport.Model
	input    textinput.Model

	// State
	messages   []string
	isThinking bool
	agent      *agent.Agent
	eventCh    <-chan agent.Event
	width      int
	height     int
	ready      bool
	autoScroll bool

	// Paste markers storage
	pasteCounter int
	pastes       map[int]string // pasteId -> original content

	// Initial message to display
	initialMessage string
}

// NewApp creates a new TUI application.
func NewApp(p provider.Provider, model *provider.Model, settings *config.Settings, sess *session.Manager, registry *tools.Registry, sandboxInfo string, extraContext string, skillsMgr *skills.Manager) *App {
	input := textinput.New()
	input.Placeholder = "Type a message..."
	input.Focus()
	input.CharLimit = 0

	vp := viewport.New(80, 20)

	return &App{
		provider:     p,
		model:        model,
		settings:     settings,
		session:      sess,
		registry:     registry,
		sandboxInfo:  sandboxInfo,
		mode:         settings.DefaultMode,
		extraContext: extraContext,
		skillsMgr:    skillsMgr,
		input:        input,
		viewport:     vp,
		autoScroll:   true,
		pastes:       make(map[int]string),
	}
}

// SetInitialMessage sets an initial message to display when the TUI starts.
func (a *App) SetInitialMessage(msg string) {
	a.initialMessage = msg
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	// Show initial message if set
	if a.initialMessage != "" {
		a.messages = append(a.messages, statusStyle.Render(a.initialMessage))
	}
	return textinput.Blink
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		inputCmd tea.Cmd
		vpCmd    tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		chatHeight := msg.Height - 5
		if chatHeight < 3 {
			chatHeight = 3
		}

		a.viewport.Width = msg.Width
		a.viewport.Height = chatHeight
		a.input.Width = msg.Width - 4

		a.updateViewportContent()
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit

		case "esc":
			if a.isThinking {
				if a.agent != nil {
					a.agent.Abort()
				}
				a.isThinking = false
				a.addMessage(statusStyle.Render("⏹ Aborted"))
			} else {
				a.input.Reset()
			}
			return a, nil

		case "tab":
			a.cycleMode()
			return a, nil

		case "enter":
			input := strings.TrimSpace(a.input.Value())
			if input != "" {
				a.input.Reset()
				// Expand paste markers before processing
				expandedInput := a.expandPasteMarkers(input)
				return a, a.processInput(expandedInput)
			}
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
		}

		// Check for paste (multi-line input in a single key event)
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			input := string(msg.Runes)
			if strings.Contains(input, "\n") {
				a.handlePaste(input)
				return a, nil
			}
		}

	case agentStartMsg:
		a.isThinking = true
		a.addMessage(userStyle.Render("You: ") + msg.input)
		return a, listenEvents(a.eventCh)

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
	a.input, inputCmd = a.input.Update(msg)
	a.viewport, vpCmd = a.viewport.Update(msg)

	return a, tea.Batch(inputCmd, vpCmd)
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

func (a *App) updateViewportContent() {
	a.viewport.SetContent(strings.Join(a.messages, "\n\n"))
	if a.autoScroll {
		a.viewport.GotoBottom()
	}
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

	status := fmt.Sprintf(" %s | %s | %s", modeStr, modelName, cwd)
	if a.isThinking {
		status += " | ⏳"
	} else {
		status += " | Tab:mode Esc:abort"
	}

	return footerStyle.Width(a.width).Render(status)
}

func (a *App) addMessage(msg string) {
	a.messages = append(a.messages, msg)
	a.updateViewportContent()
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
	a.agent = nil

	var modeLabel string
	switch a.mode {
	case "plan":
		modeLabel = "🗒️ PLAN - Read-only"
	case "agent":
		modeLabel = "🔧 AGENT - Standard"
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
		agentCfg := agent.Config{
			Provider:      a.provider,
			Model:         a.model,
			Mode:          a.mode,
			ThinkingLevel: provider.ThinkingLevel(a.settings.DefaultThinkingLevel),
			MaxTokens:     a.settings.MaxOutputTokens,
			Settings:      a.settings,
			Session:       a.session,
			ExtraContext:  a.extraContext,
		}
		a.agent = agent.New(agentCfg, a.registry)
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
				a.agent = nil
				a.addMessage(statusStyle.Render(fmt.Sprintf("Mode: %s", strings.ToUpper(a.mode))))
			default:
				a.addMessage(errorStyle.Render("Invalid mode"))
			}
		} else {
			a.addMessage(statusStyle.Render(fmt.Sprintf("Current mode: %s", strings.ToUpper(a.mode))))
		}
	case "/model":
		a.addMessage(statusStyle.Render(fmt.Sprintf("Model: %s (%s)", a.model.Name, a.model.Provider)))
	case "/clear":
		a.messages = nil
		a.agent = nil
		a.pastes = make(map[int]string)
		a.pasteCounter = 0
		a.updateViewportContent()
	case "/quit":
		return tea.Quit
	case "/help":
		a.addMessage(statusStyle.Render("Commands: /mode, /model, /clear, /quit, /help"))
	default:
		a.addMessage(errorStyle.Render(fmt.Sprintf("Unknown: %s", command)))
	}

	return nil
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
		a.updateViewportContent()
		return listenEvents(a.eventCh)

	case agent.EventThinkDelta:
		lastIdx := len(a.messages) - 1
		if lastIdx >= 0 && strings.HasPrefix(a.messages[lastIdx], thinkStyle.Render("💭 ")) {
			a.messages[lastIdx] += event.ThinkDelta
		} else {
			a.messages = append(a.messages, thinkStyle.Render("💭 ")+event.ThinkDelta)
		}
		a.updateViewportContent()
		return listenEvents(a.eventCh)

	case agent.EventToolCall:
		if event.ToolCall != nil {
			a.addMessage(toolStyle.Render(fmt.Sprintf("🔧 [%s] ...", event.ToolCall.Name)))
		}
		return listenEvents(a.eventCh)

	case agent.EventToolResult:
		for i := len(a.messages) - 1; i >= 0; i-- {
			if strings.Contains(a.messages[i], "🔧 [") {
				a.messages[i] = toolStyle.Render(fmt.Sprintf("🔧 [%s] %s", event.ToolName, truncate(event.ToolResult, 80)))
				break
			}
		}
		a.updateViewportContent()
		return listenEvents(a.eventCh)

	case agent.EventDone:
		a.isThinking = false
		a.autoScroll = true
		return listenEvents(a.eventCh)

	case agent.EventError:
		a.isThinking = false
		if event.Error != nil {
			a.addMessage(errorStyle.Render("Error: ") + event.Error.Error())
		}
		return listenEvents(a.eventCh)

	case agent.EventUsage:
		if event.Usage != nil {
			costStr := fmt.Sprintf("Tokens: %d↓/%d↑ $%.4f",
				event.Usage.Input, event.Usage.Output, event.Usage.Cost.Total)
			a.addMessage(statusStyle.Render(costStr))
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
