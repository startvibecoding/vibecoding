package remotetui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/startvibecoding/vibecoding/internal/tools"
)

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

func formatPlanForDisplay(plan *tools.TaskPlan) string {
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

// formatToolArgs formats tool arguments for display
func formatToolArgs(toolName string, args map[string]any) string {
	var parts []string

	switch toolName {
	case "write":
		if path, ok := args["path"]; ok {
			parts = append(parts, fmt.Sprintf("path: %v", path))
		}
		if content, ok := args["content"]; ok {
			contentStr := fmt.Sprintf("%v", content)
			parts = append(parts, fmt.Sprintf("content:\n%s", contentStr))
		}
	case "edit":
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

func formatEditedToolResult(result toolResult) string {
	path := toolPath(result.toolArgs)
	if result.diff != nil && result.diff.Path != "" {
		path = result.diff.Path
	}
	if path == "" {
		path = "(unknown)"
	}

	summary := result.summary
	if result.diff != nil {
		summary = fmt.Sprintf("(+%d -%d)", result.diff.Added, result.diff.Deleted)
	}

	header := fmt.Sprintf("• Edited %s", path)
	if summary != "" {
		header += " " + summary
	}

	if result.diff == nil || strings.TrimSpace(result.diff.Unified) == "" {
		return header
	}

	diffLines := formatUnifiedDiffExcerpt(result.diff.Unified)
	if diffLines == "" {
		return header
	}
	return header + "\n" + diffLines
}

var unifiedHunkRe = regexp.MustCompile(`^@@ -([0-9]+)(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@`)

func formatUnifiedDiffExcerpt(unified string) string {
	var lines []string
	oldLine, newLine := 0, 0
	for _, line := range strings.Split(strings.TrimRight(unified, "\n"), "\n") {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || line == "" {
			continue
		}
		if matches := unifiedHunkRe.FindStringSubmatch(line); matches != nil {
			oldLine, _ = strconv.Atoi(matches[1])
			newLine, _ = strconv.Atoi(matches[2])
			continue
		}
		if oldLine == 0 && newLine == 0 {
			continue
		}

		kind := line[0]
		text := ""
		if len(line) > 1 {
			text = line[1:]
		}

		switch kind {
		case ' ':
			lines = append(lines, formatDiffExcerptLine(newLine, ' ', text))
			oldLine++
			newLine++
		case '-':
			lines = append(lines, formatDiffExcerptLine(oldLine, '-', text))
			oldLine++
		case '+':
			lines = append(lines, formatDiffExcerptLine(newLine, '+', text))
			newLine++
		}
	}
	return strings.Join(lines, "\n")
}

func formatDiffExcerptLine(lineNo int, kind byte, text string) string {
	return fmt.Sprintf("    %-4d %c%s", lineNo, kind, text)
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

func summarizeFileDiff(diff *tools.FileDiff) string {
	if diff == nil {
		return ""
	}
	suffix := ""
	if diff.Truncated {
		suffix = " large"
	}
	return fmt.Sprintf("+%d -%d%s (-%s +%s)",
		diff.Added,
		diff.Deleted,
		suffix,
		formatLineRangesForDisplay(diff.DeletedLines),
		formatLineRangesForDisplay(diff.AddedLines),
	)
}

func formatLineRangesForDisplay(lines []int) string {
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
		ranges = append(ranges, formatLineRangeForDisplay(start, prev))
		start, prev = line, line
	}
	ranges = append(ranges, formatLineRangeForDisplay(start, prev))
	return strings.Join(ranges, ",")
}

func formatLineRangeForDisplay(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// compactBashOutput compresses bash tool output for summary display by removing blank lines.
func compactBashOutput(s string) string {
	var sb strings.Builder
	prevBlank := false
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevBlank {
				sb.WriteString("\n")
			}
			prevBlank = true
			continue
		}
		prevBlank = false
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

// truncate shortens s so its terminal display width does not exceed maxWidth,
// appending "..." when truncation occurs. Width is measured in display cells
// (CJK runes count as 2, ANSI escape sequences as 0) so the result lines up
// correctly in the TUI grid.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	const suffix = "..."
	target := maxWidth - lipgloss.Width(suffix)
	if target <= 0 {
		return suffix
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > target {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + suffix
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
