package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/charmbracelet/glamour"

	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	ctxpkg "github.com/startvibecoding/vibecoding/internal/context"
	"github.com/startvibecoding/vibecoding/internal/provider"
	providerfactory "github.com/startvibecoding/vibecoding/internal/provider/factory"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

var debugEnabled bool

// clearStdin reads and discards any pending input from stdin.
// This is needed because some terminals send color query sequences on startup.
func clearStdin() {
	// Set a short read deadline so pending reads time out cleanly.
	// Some stdin types (pipes, certain PTYs) don't support deadlines;
	// if SetReadDeadline fails we skip clearing to avoid blocking forever.
	if err := os.Stdin.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		return
	}
	defer os.Stdin.SetReadDeadline(time.Time{}) // Clear deadline
	buf := make([]byte, 128)
	for {
		n, err := os.Stdin.Read(buf)
		if n == 0 || err != nil {
			return
		}
	}
}

// debugLog prints debug messages to stderr if debug mode is enabled.
func debugLog(format string, args ...interface{}) {
	if debugEnabled {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// createProvider creates a provider from config based on provider name.
func createProvider(settings *config.Settings, providerName, modelID string) (provider.Provider, *provider.Model, error) {
	return providerfactory.Create(settings, providerName, modelID)
}

func runPrint(args []string, p provider.Provider, model *provider.Model, mode string, thinkingLevel provider.ThinkingLevel, settings *config.Settings, registry *tools.Registry, sess *session.Manager, extraContext string, multiAgent bool, agentMgr *agent.AgentManager) error {
	input := strings.Join(args, " ")
	if input == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("no input provided")
		}
		input = string(data)
	}

	fmt.Fprintf(os.Stderr, "Using %s/%s in %s mode\n", p.Name(), model.ID, mode)

	// Create glamour renderer for markdown
	wordWrap := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		wordWrap = w
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(wordWrap),
	)
	if err != nil {
		debugLog("Failed to create glamour renderer: %v", err)
		renderer = nil
	}

	compactionSettings := ctxpkg.CompactionSettings{
		Enabled:          settings.Compaction.Enabled,
		ReserveTokens:    settings.Compaction.ReserveTokens,
		KeepRecentTokens: settings.Compaction.KeepRecentTokens,
	}
	if compactionSettings.ReserveTokens == 0 {
		compactionSettings.ReserveTokens = 16384
	}
	if compactionSettings.KeepRecentTokens == 0 {
		compactionSettings.KeepRecentTokens = 20000
	}

	agentCfg := agent.Config{
		Provider:           p,
		Model:              model,
		Mode:               mode,
		ThinkingLevel:      thinkingLevel,
		MaxTokens:          settings.MaxOutputTokens,
		Settings:           settings,
		Session:            sess,
		ExtraContext:       extraContext,
		CompactionSettings: compactionSettings,
		MultiAgent:         multiAgent,
	}

	a := agent.New(agentCfg, registry)
	if multiAgent && agentMgr != nil {
		agentMgr.Register(agent.NewAgentAdapter(a))
	}

	ctx := context.Background()
	eventCh := a.Run(ctx, input)

	var textBuffer strings.Builder

	err = agent.ConsumeEvents(ctx, eventCh, agent.EventHandlerFunc(func(_ context.Context, event agent.Event) error {
		switch event.Type {
		case agent.EventToolApprovalRequest:
			return fmt.Errorf("tool approval required in print mode for %s; rerun interactively, use --mode yolo, or whitelist the command", event.ApprovalTool)
		case agent.EventTextDelta:
			textBuffer.WriteString(event.TextDelta)
		case agent.EventToolCall:
			// Flush text buffer before tool call
			if textBuffer.Len() > 0 {
				flushTextBuffer(&textBuffer, renderer)
			}
			fmt.Fprintf(os.Stderr, "\n[tool: %s]\n", event.ToolCall.Name)
		case agent.EventToolExecutionStart:
			fmt.Fprintf(os.Stderr, "[running: %s] ", event.ToolName)
		case agent.EventToolExecutionEnd:
			if event.ToolError != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", event.ToolError)
			} else {
				fmt.Fprintf(os.Stderr, "done\n")
			}
		case agent.EventToolResult:
			// Show full tool result for bash commands
			if event.ToolName == "bash" {
				fmt.Fprintf(os.Stderr, "\n%s\n", event.ToolResult)
			} else if event.ToolDiff != nil {
				fmt.Fprintf(os.Stderr, "\n[change: %s] +%d -%d (-%s +%s)\n",
					event.ToolDiff.Path,
					event.ToolDiff.Added,
					event.ToolDiff.Deleted,
					formatLineRanges(event.ToolDiff.DeletedLines),
					formatLineRanges(event.ToolDiff.AddedLines),
				)
			}
		case agent.EventPlanUpdate:
			if event.Plan != nil {
				fmt.Fprintf(os.Stderr, "\n%s\n", formatTaskPlan(event.Plan))
			}
		case agent.EventDone:
			// Flush remaining text buffer
			if textBuffer.Len() > 0 {
				flushTextBuffer(&textBuffer, renderer)
			}
			// Show context usage
			if event.ContextUsage != nil && event.ContextUsage.Percent != nil {
				fmt.Fprintf(os.Stderr, "\nContext: %.1f%%/%s\n",
					*event.ContextUsage.Percent,
					formatTokenCount(event.ContextUsage.ContextWindow))
			}
		case agent.EventError:
			// Flush text buffer before error
			if textBuffer.Len() > 0 {
				flushTextBuffer(&textBuffer, renderer)
			}
			if event.Error != nil {
				return event.Error
			}
		case agent.EventUsage:
			if event.ContextUsage != nil && event.ContextUsage.Percent != nil {
				fmt.Fprintf(os.Stderr, "Context: %.1f%%/%s | ",
					*event.ContextUsage.Percent,
					formatTokenCount(event.ContextUsage.ContextWindow))
			}
			if event.Usage != nil {
				cacheInfo := ""
				if info := event.Usage.CacheInfo(); info != "" {
					cacheInfo = " | " + info
				}
				fmt.Fprintf(os.Stderr, "Tokens: %d↓/%d↑ $%.4f%s\n",
					event.Usage.TotalInputTokens(), event.Usage.Output, event.Usage.Cost.Total, cacheInfo)
			}
		case agent.EventCompactionStart:
			fmt.Fprintf(os.Stderr, "\n⏳ Compacting context...\n")
		case agent.EventCompactionEnd:
			if event.Error != nil {
				fmt.Fprintf(os.Stderr, "Compaction failed: %v\n", event.Error)
			} else if event.StatusMessage != "" {
				fmt.Fprintf(os.Stderr, "✅ %s\n", event.StatusMessage)
			} else {
				fmt.Fprintf(os.Stderr, "✅ Context compacted\n")
			}
		}
		return nil
	}))
	if err != nil {
		return err
	}

	return nil
}

func formatTaskPlan(plan *tools.TaskPlan) string {
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

func formatLineRanges(lines []int) string {
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
		ranges = append(ranges, formatLineRange(start, prev))
		start, prev = line, line
	}
	ranges = append(ranges, formatLineRange(start, prev))
	return strings.Join(ranges, ",")
}

func formatLineRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

// flushTextBuffer renders and prints the accumulated text buffer.
func flushTextBuffer(buffer *strings.Builder, renderer *glamour.TermRenderer) {
	text := buffer.String()
	buffer.Reset()

	if renderer != nil {
		rendered, err := renderer.Render(text)
		if err != nil {
			// Fallback to plain text
			fmt.Print(text)
		} else {
			fmt.Print(rendered)
		}
	} else {
		fmt.Print(text)
	}
}

// formatTokenCount formats a token count for display.
func formatTokenCount(count int) string {
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
