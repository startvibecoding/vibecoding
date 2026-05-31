package tui

import (
	"encoding/json"
	"fmt"
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
	if len(a.approvalQueue) > 0 {
		a.addMessage(warningStyle.Render(fmt.Sprintf("⚠️  Approval required for [%s] (%d more pending)", next.toolName, len(a.approvalQueue))))
	} else {
		a.addMessage(warningStyle.Render(fmt.Sprintf("⚠️  Approval required for [%s]", next.toolName)))
	}
	if len(next.args) > 0 {
		a.addMessage(warningStyle.Render(formatApprovalArgs(next.toolName, next.args)))
	}
	a.addMessage(warningStyle.Render("Approve? (y/n): "))
}

func formatApprovalArgs(toolName string, args map[string]any) string {
	if toolName == "edit" {
		return formatEditApprovalArgs(args)
	}

	safeArgs := make(map[string]any, len(args))
	for k, v := range args {
		if k == "content" {
			text := fmt.Sprintf("%v", v)
			safeArgs[k] = fmt.Sprintf("(%d bytes)", len(text))
			continue
		}
		safeArgs[k] = v
	}
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(safeArgs); err != nil {
		return fmt.Sprintf("%v", safeArgs)
	}
	return strings.TrimRight(buf.String(), "\n")
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
