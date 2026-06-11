package remotetui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/startvibecoding/vibecoding/internal/agent"
)

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
				a.toolResults[j].diff = event.ToolDiff

				// Create summary based on tool type
				switch event.ToolName {
				case "bash":
					a.toolResults[j].summary = compactBashOutput(event.ToolResult)
				case "read":
					lines := strings.Split(event.ToolResult, "\n")
					a.toolResults[j].summary = fmt.Sprintf("%d lines", len(lines))
				case "ls":
					a.toolResults[j].summary = compactBashOutput(event.ToolResult)
				case "write":
					if summary := summarizeFileDiff(event.ToolDiff); summary != "" {
						a.toolResults[j].summary = summary
					} else {
						a.toolResults[j].summary = summarizeWriteToolResult(event.ToolResult)
					}
				case "edit":
					if summary := summarizeFileDiff(event.ToolDiff); summary != "" {
						a.toolResults[j].summary = summary
					} else {
						a.toolResults[j].summary = "Applied"
					}
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

	case agent.EventPlanUpdate:
		a.currentPlan = event.Plan
		a.addMessage(statusStyle.Render(formatPlanForDisplay(event.Plan)))
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

	case agent.EventQuestionRequest:
		a.commitActiveStream()
		// Queue the question request
		a.questionQueue = append(a.questionQueue, pendingQuestion{
			questionID: event.QuestionID,
			question:   event.QuestionText,
			options:    event.QuestionOptions,
			context:    event.QuestionContext,
		})
		// If not currently waiting for a question, show the next one
		if !a.waitingForQuestion {
			a.showNextQuestion()
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
