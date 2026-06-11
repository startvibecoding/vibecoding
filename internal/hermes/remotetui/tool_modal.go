package remotetui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	if result.diff != nil && result.diff.Unified != "" {
		parts = append(parts, "--- diff", result.diff.Unified)
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
	if result.toolName == "edit" {
		content = formatEditedToolResult(result)
	}
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
	content := title + "\n" + strings.Repeat("─", minInt(width-2, lipgloss.Width(title))) + "\n" + visible
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
