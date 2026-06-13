package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/tools"
)

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

	a.addMessage(a.renderApprovalRequest(next, len(a.approvalQueue)))
	a.scheduleRender()
}

func (a *App) clearApprovalState() {
	a.waitingForApproval = false
	a.pendingApprovalID = ""
	a.approvalQueue = a.approvalQueue[:0]
}

// showNextQuestion pops the next question request from the queue and displays it.
func (a *App) showNextQuestion() {
	if len(a.questionQueue) == 0 {
		a.waitingForQuestion = false
		a.pendingQuestionID = ""
		return
	}
	next := a.questionQueue[0]
	a.questionQueue = a.questionQueue[1:]
	a.currentQuestion = next
	a.pendingQuestionID = next.questionID
	a.waitingForQuestion = true

	// Build all lines into one message to preserve order (addMessage uses
	// async goroutines, so multiple calls can interleave).
	var sb strings.Builder
	if next.context != "" {
		sb.WriteString(warningStyle.Render("💬 " + next.context))
		sb.WriteByte('\n')
	}
	sb.WriteString(warningStyle.Render("❓ " + next.question))
	sb.WriteByte('\n')
	for i, opt := range next.options {
		sb.WriteString(statusStyle.Render(fmt.Sprintf("  [%d] %s", i+1, opt)))
		sb.WriteByte('\n')
	}
	sb.WriteString(statusStyle.Render(fmt.Sprintf("  [%d] ✍️  Custom input", len(next.options)+1)))
	sb.WriteByte('\n')
	sb.WriteString(warningStyle.Render("Enter number or custom text: "))
	a.addMessage(sb.String())
}

func (a *App) clearQuestionState() {
	a.waitingForQuestion = false
	a.pendingQuestionID = ""
	a.currentQuestion = pendingQuestion{}
	a.questionQueue = a.questionQueue[:0]
}

func (a *App) renderApprovalRequest(next pendingApproval, remaining int) string {
	var sb strings.Builder
	title := fmt.Sprintf("! Approval required: %s", next.toolName)
	if remaining > 0 {
		title += fmt.Sprintf(" (%d more pending)", remaining)
	}
	sb.WriteString(warningStyle.Render(title))
	sb.WriteByte('\n')

	if detail := formatApprovalArgs(next.toolName, next.args); strings.TrimSpace(detail) != "" {
		sb.WriteString(detail)
		sb.WriteByte('\n')
	}

	sb.WriteString(warningStyle.Render("Approve? "))
	sb.WriteString(statusStyle.Render("y = approve, n = deny"))
	return sb.String()
}

func formatApprovalArgs(toolName string, args map[string]any) string {
	switch toolName {
	case "bash":
		return formatBashApprovalArgs(args)
	case "edit":
		return formatEditApprovalArgs(args)
	case "write":
		return formatWriteApprovalArgs(args)
	}

	return formatGenericApprovalArgs(args)
}

func formatBashApprovalArgs(args map[string]any) string {
	var lines []string
	if command, ok := args["command"].(string); ok && command != "" {
		lines = append(lines, statusStyle.Render("command:"))
		lines = append(lines, indentLines(command, "  "))
	}
	if timeout, ok := args["timeout"]; ok {
		lines = append(lines, fmt.Sprintf("timeout: %v", timeout))
	}
	if async, ok := args["async"]; ok {
		lines = append(lines, fmt.Sprintf("async: %v", async))
	}
	return strings.Join(lines, "\n")
}

func formatWriteApprovalArgs(args map[string]any) string {
	var lines []string
	if path, ok := args["path"].(string); ok && path != "" {
		lines = append(lines, fmt.Sprintf("path: %s", path))
	}
	if content, ok := args["content"]; ok {
		text := fmt.Sprintf("%v", content)
		lines = append(lines, fmt.Sprintf("content: (%d bytes)", len(text)))
	}
	if len(lines) == 0 {
		return formatGenericApprovalArgs(args)
	}
	return strings.Join(lines, "\n")
}

func formatGenericApprovalArgs(args map[string]any) string {
	safeArgs := make(map[string]any, len(args))
	for k, v := range args {
		if k == "content" {
			text := fmt.Sprintf("%v", v)
			safeArgs[k] = fmt.Sprintf("(%d bytes)", len(text))
			continue
		}
		safeArgs[k] = v
	}

	keys := make([]string, 0, len(safeArgs))
	for k := range safeArgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s: %s", k, formatApprovalValue(safeArgs[k])))
	}
	return strings.Join(lines, "\n")
}

func formatApprovalValue(v any) string {
	switch val := v.(type) {
	case string:
		if strings.Contains(val, "\n") {
			return "\n" + indentLines(val, "  ")
		}
		return val
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func formatEditApprovalArgs(args map[string]any) string {
	path, _ := args["path"].(string)
	if path == "" {
		path = "<unknown path>"
	}

	var diffs []string
	editList, ok := args["edits"].([]any)
	if ok {
		for _, e := range editList {
			editMap, ok := e.(map[string]any)
			if !ok {
				continue
			}
			oldText, _ := editMap["oldText"].(string)
			newText, _ := editMap["newText"].(string)
			diff := tools.BuildFileDiff(path, oldText, newText)
			if diff == nil || strings.TrimSpace(diff.Unified) == "" {
				continue
			}
			diffs = append(diffs, strings.TrimRight(diff.Unified, "\n"))
		}
	}

	if len(diffs) == 0 {
		return fmt.Sprintf("path: %s\ndiff: (empty)", path)
	}
	return fmt.Sprintf("path: %s\n%s", path, strings.Join(diffs, "\n"))
}
