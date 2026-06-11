package remotetui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) addMessage(msg string) {
	a.messages = append(a.messages, msg)
	a.printHistory(msg)
}

func (a *App) printHistory(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	// Route through the single drain goroutine set up in SetProgram so that
	// program.Println calls reach Bubble Tea's message channel in the order
	// printHistory was invoked. Sending from many ad-hoc goroutines races and
	// can interleave messages — visually that looks like a missing line break
	// between two log lines.
	if a.printCh != nil {
		select {
		case a.printCh <- msg:
			return
		default:
			if a.program != nil {
				go a.program.Println(msg)
				return
			}
		}
	}
	a.pendingPrints = append(a.pendingPrints, msg)
}

func (a *App) printMessageOnce(idx int) {
	if idx < 0 || a.printedMessageIdx[idx] {
		return
	}
	a.printedMessageIdx[idx] = true
	rendered := a.renderMessageAt(idx)
	a.printHistory(rendered)
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
	// Sequence (not Batch) keeps prints in their queued order — Batch runs each
	// cmd in its own goroutine, which would re-introduce the very interleaving
	// issue we're trying to avoid here.
	return tea.Sequence(cmds...)
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

func (a *App) cycleMode() {
	switch a.mode {
	case "plan":
		a.mode = "agent"
	case "agent":
		a.mode = "yolo"
	case "yolo":
		a.mode = "plan"
	default:
		a.mode = "agent"
	}
	_ = a.sendWS(wsMessage{Type: "command", Content: "/mode " + a.mode})

	var modeLabel string
	switch a.mode {
	case "plan":
		modeLabel = "🗒 PLAN - Read-only mode"
	case "agent":
		modeLabel = "🔧 AGENT - File edits, bash with approval"
	case "yolo":
		modeLabel = "🚀 YOLO - Full access"
	}
	a.addMessage(statusStyle.Render(fmt.Sprintf("Mode: %s", modeLabel)))
}

func (a *App) recordInputHistory(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}
	if len(a.inputHistory) > 0 && a.inputHistory[len(a.inputHistory)-1] == input {
		a.resetInputHistoryNavigation()
		return
	}
	a.inputHistory = append(a.inputHistory, input)
	const maxInputHistory = 200
	if len(a.inputHistory) > maxInputHistory {
		a.inputHistory = a.inputHistory[len(a.inputHistory)-maxInputHistory:]
	}
	a.resetInputHistoryNavigation()
}

func (a *App) navigateInputHistory(direction int) bool {
	if a.waitingForApproval || len(a.inputHistory) == 0 {
		return false
	}

	switch {
	case direction < 0:
		if !a.inputHistoryBrowsing {
			a.inputHistoryDraft = a.input.Value()
			a.inputHistoryIndex = len(a.inputHistory) - 1
			a.inputHistoryBrowsing = true
		} else if a.inputHistoryIndex > 0 {
			a.inputHistoryIndex--
		}
	case direction > 0:
		if !a.inputHistoryBrowsing {
			return false
		}
		if a.inputHistoryIndex < len(a.inputHistory)-1 {
			a.inputHistoryIndex++
		} else {
			a.inputHistoryBrowsing = false
			a.inputHistoryIndex = 0
			a.input.SetValue(a.inputHistoryDraft)
			a.input.CursorEnd()
			a.inputHistoryDraft = ""
			a.scheduleRender()
			return true
		}
	default:
		return false
	}

	if a.inputHistoryIndex >= 0 && a.inputHistoryIndex < len(a.inputHistory) {
		a.input.SetValue(a.inputHistory[a.inputHistoryIndex])
		a.input.CursorEnd()
		a.scheduleRender()
		return true
	}
	return false
}

func (a *App) resetInputHistoryNavigation() {
	a.inputHistoryBrowsing = false
	a.inputHistoryIndex = 0
	a.inputHistoryDraft = ""
}

func (a *App) processInput(input string) tea.Cmd {
	if strings.HasPrefix(input, "/") {
		return a.handleCommand(input)
	}

	a.commitActiveStream()
	if err := a.sendWS(wsMessage{Type: "message", Content: input}); err != nil {
		a.addMessage(errorStyle.Render("Error: ") + err.Error())
		return nil
	}

	return func() tea.Msg { return agentStartMsg{input: input} }
}
