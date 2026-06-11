package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	if result.toolName == "edit" {
		if result.summary == "" && result.fullContent == "" && result.diff == nil {
			return toolStyle.Render(fmt.Sprintf("%s ...", formatToolHeader(result)))
		}
		return toolStyle.Render(formatEditedToolResult(result))
	}
	summary := result.summary
	if summary == "" {
		summary = "..."
	}
	sep := " "
	if strings.Contains(summary, "\n") {
		sep = "\n"
	}
	return toolStyle.Render(fmt.Sprintf("%s%s%s", formatToolHeader(result), sep, summary))
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

func (a *App) renderLiveAssistantMessage(idx int) string {
	raw := a.assistantRaw[idx]
	if raw == "" {
		return ""
	}
	return assistantStyle.Render("Assistant: ") + wrapPlainText(raw, a.assistantMarkdownWidth())
}

func wrapPlainText(s string, width int) string {
	if width <= 0 {
		return s
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		out = append(out, wrapPlainLine(line, width)...)
	}
	return strings.Join(out, "\n")
}

func wrapPlainLine(line string, width int) []string {
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	var lines []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range line {
		rw := lipgloss.Width(string(r))
		if currentWidth > 0 && currentWidth+rw > width {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	lines = append(lines, current.String())
	return lines
}

func (a *App) renderPlanPanel() string {
	if a.currentPlan == nil || len(a.currentPlan.Steps) == 0 {
		return ""
	}
	var lines []string
	title := a.currentPlan.Title
	if title == "" {
		title = "Plan"
	}
	lines = append(lines, statusStyle.Render(title))
	for _, step := range a.currentPlan.Steps {
		lines = append(lines, statusStyle.Render(fmt.Sprintf("%s %s", planStatusMarker(step.Status), step.Title)))
	}
	if a.currentPlan.Note != "" {
		lines = append(lines, statusStyle.Render("note: "+a.currentPlan.Note))
	}
	return strings.Join(lines, "\n")
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
