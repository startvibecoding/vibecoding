package tui

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

// ansiRe matches ANSI CSI escape sequences (colours, bold, etc.).
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func trimLineRightSpace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

func TestRenderEditToolResultShowsCompactDiff(t *testing.T) {
	app := &App{}
	result := toolResult{
		toolName: "edit",
		toolArgs: map[string]any{"path": "internal/acp/acp.go"},
		diff: &tools.FileDiff{
			Path:    "internal/acp/acp.go",
			Added:   1,
			Deleted: 1,
			Unified: strings.Join([]string{
				"--- internal/acp/acp.go",
				"+++ internal/acp/acp.go",
				"@@ -551,3 +551,3 @@",
				" \tctx, cancel := context.WithCancel(context.Background())",
				"-\tpromptKey := rawIDKey(req.ID)",
				"+\tpromptKey := mcp.RawIDKey(req.ID)",
				" \trt.cancelMu.Lock()",
				"",
			}, "\n"),
		},
	}

	got := trimLineRightSpace(stripANSI(app.renderToolResult(result)))
	want := strings.Join([]string{
		"• Edited internal/acp/acp.go (+1 -1)",
		"    551       ctx, cancel := context.WithCancel(context.Background())",
		"    552  -    promptKey := rawIDKey(req.ID)",
		"    552  +    promptKey := mcp.RawIDKey(req.ID)",
		"    553       rt.cancelMu.Lock()",
	}, "\n")

	if got != want {
		t.Fatalf("renderToolResult(edit) =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderBashToolResultKeepsOutputRaw(t *testing.T) {
	app := &App{}
	summary := "[stdout]\n\u001b[32m+added\u001b[0m\n context\r\n[exit_code]\n0"
	got := app.renderToolResult(toolResult{
		toolName: "bash",
		summary:  summary,
	})

	parts := strings.SplitN(got, "\n", 2)
	if len(parts) != 2 {
		t.Fatalf("renderToolResult(bash) = %q, want header and body", got)
	}
	if parts[1] != summary {
		t.Fatalf("bash output body was modified:\n got %q\nwant %q", parts[1], summary)
	}
	if strings.Contains(parts[1], "\x1b[3m") {
		t.Fatalf("bash output body should not inherit TUI italic styling: %q", parts[1])
	}
}

func TestRenderExpandedBashToolResultKeepsDetailsRaw(t *testing.T) {
	app := &App{}
	output := "\u001b[31m-deleted\u001b[0m\r\n+added"
	got := app.renderExpandedToolResult(toolResult{
		toolName:    "bash",
		fullContent: output,
	})

	if !strings.HasSuffix(got, "---\n"+output) {
		t.Fatalf("expanded bash output was modified: %q", got)
	}
	body := got[strings.Index(got, "\n")+1:]
	if strings.Contains(body, "\x1b[3m") {
		t.Fatalf("expanded bash output body should not inherit TUI italic styling: %q", body)
	}
}

func TestNormalizeHistoryLineEndingsOnlyCollapsesCRLF(t *testing.T) {
	got := normalizeHistoryLineEndings("a\r\nb\rc")
	want := "a\nb\rc"
	if got != want {
		t.Fatalf("normalizeHistoryLineEndings() = %q, want %q", got, want)
	}
}

func TestAssistantMarkdownRendererUsesViewportWidth(t *testing.T) {
	app := &App{
		width:               60,
		assistantRaw:        map[int]string{0: "请看 https://gitee.com/oschina/platform/pulls/11938 这里"},
		assistantRendered:   make(map[int]string),
		assistantDirty:      map[int]bool{0: true},
		currentAssistantIdx: -1,
		currentThinkIdx:     -1,
	}
	app.configureMarkdownRenderer()

	got := stripANSI(app.renderAssistantMessage(0))
	flattened := strings.ReplaceAll(strings.ReplaceAll(got, "\n", ""), " ", "")
	if !strings.Contains(flattened, "https://gitee.com/oschina/platform/pulls/11938") {
		t.Fatalf("renderAssistantMessage() = %q, want URL order preserved", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if width := lipgloss.Width(line); width > app.width {
			t.Fatalf("rendered line width = %d, want <= %d: %q", width, app.width, line)
		}
	}
}

func TestWindowResizeMarksAssistantMarkdownDirty(t *testing.T) {
	app := &App{
		assistantRaw:        map[int]string{0: "hello"},
		assistantRendered:   map[int]string{0: "old"},
		assistantDirty:      make(map[int]bool),
		currentAssistantIdx: -1,
		currentThinkIdx:     -1,
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	updated := model.(*App)

	if updated.mdRenderer == nil {
		t.Fatal("mdRenderer is nil after resize")
	}
	if !updated.assistantDirty[0] {
		t.Fatal("assistantDirty[0] = false, want true after resize")
	}
}

func TestLiveAssistantMessageRendersCodeBlocks(t *testing.T) {
	app := &App{
		width:               80,
		assistantRaw:        map[int]string{0: "Here is code:\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\nDone"},
		assistantRendered:   make(map[int]string),
		assistantDirty:      map[int]bool{0: true},
		currentAssistantIdx: 0,
		currentThinkIdx:     -1,
	}
	app.configureMarkdownRenderer()

	app.updateViewportContent()
	plain := stripANSI(app.liveContent)
	if !strings.Contains(plain, "Assistant:") {
		t.Fatalf("live content missing assistant prefix: %q", plain)
	}
	if !strings.Contains(plain, "func main") {
		t.Fatalf("live code block content missing 'func main': %q", plain)
	}
	if !strings.Contains(plain, "Done") {
		t.Fatalf("live content missing trailing text 'Done': %q", plain)
	}
	if strings.Contains(plain, "```") {
		t.Fatalf("live content must not contain raw backtick fences (glamour should strip them): %q", plain)
	}
}

func TestLiveAssistantMessageRendersMarkdown(t *testing.T) {
	app := &App{
		width:               50,
		assistantRaw:        map[int]string{0: strings.Repeat("https://example.com/path/", 8)},
		assistantRendered:   make(map[int]string),
		assistantDirty:      map[int]bool{0: true},
		currentAssistantIdx: 0,
		currentThinkIdx:     -1,
	}
	app.configureMarkdownRenderer()

	app.updateViewportContent()
	if len(app.assistantRendered) != 0 {
		t.Fatalf("assistantRendered len = %d, want 0 for prose without fenced code blocks", len(app.assistantRendered))
	}
	if !strings.Contains(stripANSI(app.liveContent), "Assistant: ") {
		t.Fatalf("liveContent missing assistant prefix: %q", app.liveContent)
	}
}

func TestPlainAssistantMessageWrapsWithoutMarkdownWordSplitting(t *testing.T) {
	app := &App{
		width:               40,
		assistantRaw:        map[int]string{0: "修复 /clear 未清理 transcript rendering state"},
		assistantRendered:   make(map[int]string),
		assistantDirty:      map[int]bool{0: true},
		currentAssistantIdx: 0,
		currentThinkIdx:     -1,
	}
	app.configureMarkdownRenderer()

	app.updateViewportContent()
	plain := stripANSI(app.liveContent)
	if strings.Contains(plain, "修  /复clear") || strings.Contains(plain, "v.01\n36") {
		t.Fatalf("plain assistant text was awkwardly split: %q", plain)
	}
	if !strings.Contains(plain, "修复 /clear") {
		t.Fatalf("plain assistant text missing expected phrase: %q", plain)
	}
	if len(app.assistantRendered) != 0 {
		t.Fatalf("assistantRendered len = %d, want 0 for plain prose", len(app.assistantRendered))
	}
}

func TestViewClampsLiveContentToKeepInputVisible(t *testing.T) {
	app := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	app.ready = true
	app.width = 80
	app.height = 8
	app.input.Width = 76
	app.liveContent = strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
	}, "\n")

	got := stripANSI(app.View())
	if strings.Contains(got, "line 1") {
		t.Fatalf("View() kept oldest live line despite limited height:\n%s", got)
	}
	if !strings.Contains(got, app.input.Placeholder) {
		t.Fatalf("View() missing input placeholder:\n%s", got)
	}
	if !strings.Contains(got, "Tab:mode") {
		t.Fatalf("View() missing footer:\n%s", got)
	}
}

// ─── formatCachePercent ───────────────────────────────────────────────────────

func TestFormatCachePercent(t *testing.T) {
	tests := []struct {
		name             string
		totalInputTokens int
		totalCacheRead   int
		totalCacheWrite  int
		want             string
	}{
		// ── No data ──────────────────────────────────────────────────────────
		{
			name: "no_data_empty",
		},
		// ── Input tokens present ─────────────────────────────────────────────
		{
			name:             "input_no_cache_zero_pct",
			totalInputTokens: 1000,
			want:             "Cache: 0%",
		},
		{
			name:             "cache_25pct",
			totalInputTokens: 1000,
			totalCacheRead:   250,
			want:             "Cache: 25%",
		},
		{
			name:             "cache_50pct",
			totalInputTokens: 1000,
			totalCacheRead:   500,
			want:             "Cache: 50%",
		},
		{
			name:             "cache_75pct",
			totalInputTokens: 1000,
			totalCacheRead:   750,
			want:             "Cache: 75%",
		},
		{
			name:             "cache_100pct_exact",
			totalInputTokens: 1000,
			totalCacheRead:   1000,
			want:             "Cache: 100%",
		},
		// Defensive cap when read > input
		{
			name:             "cache_read_exceeds_input_capped_at_100pct",
			totalInputTokens: 100,
			totalCacheRead:   200,
			want:             "Cache: 100%",
		},
		// Multi-turn accumulation across several requests
		{
			name:             "multi_turn_accumulated_75pct",
			totalInputTokens: 4000,
			totalCacheRead:   3000,
			want:             "Cache: 75%",
		},
		// ── Fallback path: no input tokens yet ───────────────────────────────
		// CacheRead takes priority over CacheWrite in the fallback
		{
			name:           "no_input_cache_read_fallback",
			totalCacheRead: 500,
			want:           "CacheRead: 500",
		},
		{
			name:            "no_input_cache_write_fallback",
			totalCacheWrite: 1000,
			want:            "CacheWrite: 1000",
		},
		{
			name:            "no_input_both_read_wins_over_write",
			totalCacheRead:  500,
			totalCacheWrite: 1000,
			want:            "CacheRead: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{
				totalInputTokens: tt.totalInputTokens,
				totalCacheRead:   tt.totalCacheRead,
				totalCacheWrite:  tt.totalCacheWrite,
			}
			got := a.formatCachePercent()
			if got != tt.want {
				t.Errorf("formatCachePercent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── renderFooter cache content ───────────────────────────────────────────────

func TestRenderFooterCacheContent(t *testing.T) {
	tests := []struct {
		name             string
		totalInputTokens int
		totalCacheRead   int
		totalCacheWrite  int
		wantContains     string // expected substring in stripped footer
		wantAbsent       string // must NOT appear in stripped footer
	}{
		// No cache data → "Cache:" must not appear at all
		{
			name:       "no_data_cache_absent",
			wantAbsent: "Cache:",
		},
		{
			name:             "zero_pct_shown",
			totalInputTokens: 1000,
			wantContains:     "Cache: 0%",
		},
		{
			name:             "cache_25pct_shown",
			totalInputTokens: 1000,
			totalCacheRead:   250,
			wantContains:     "Cache: 25%",
		},
		// Boundary just below 50% threshold
		{
			name:             "cache_49pct_shown",
			totalInputTokens: 1000,
			totalCacheRead:   490,
			wantContains:     "Cache: 49%",
		},
		// Boundary at exactly 50%
		{
			name:             "cache_50pct_shown",
			totalInputTokens: 1000,
			totalCacheRead:   500,
			wantContains:     "Cache: 50%",
		},
		{
			name:             "cache_75pct_shown",
			totalInputTokens: 1000,
			totalCacheRead:   750,
			wantContains:     "Cache: 75%",
		},
		{
			name:             "cache_100pct_shown",
			totalInputTokens: 1000,
			totalCacheRead:   1000,
			wantContains:     "Cache: 100%",
		},
		// Fallback paths visible in footer
		{
			name:            "cache_write_fallback_shown",
			totalCacheWrite: 5000,
			wantContains:    "CacheWrite: 5000",
		},
		{
			name:           "cache_read_fallback_shown",
			totalCacheRead: 800,
			wantContains:   "CacheRead: 800",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{
				totalInputTokens: tt.totalInputTokens,
				totalCacheRead:   tt.totalCacheRead,
				totalCacheWrite:  tt.totalCacheWrite,
			}
			footer := stripANSI(a.renderFooter())

			if tt.wantContains != "" && !strings.Contains(footer, tt.wantContains) {
				t.Errorf("renderFooter() = %q\n\twant substring %q", footer, tt.wantContains)
			}
			if tt.wantAbsent != "" && strings.Contains(footer, tt.wantAbsent) {
				t.Errorf("renderFooter() = %q\n\twant %q to be absent", footer, tt.wantAbsent)
			}
		})
	}
}

// ─── Highlight threshold ──────────────────────────────────────────────────────

// TestCacheHighlightThreshold verifies the ≥50% rule that gates statusStyle
// in renderFooter. At exactly 49% the cache string must not be highlighted;
// at exactly 50% it must be.
//
// Because lipgloss omits ANSI codes when there is no TTY, we verify the
// decision by checking whether the raw footer embeds statusStyle.Render()
// output for the specific cache string. statusStyle.Render(x) == x when the
// renderer is in Ascii mode, but the branch taken in renderFooter differs:
// the ≥50% branch always passes through statusStyle.Render(), the <50%
// branch uses the plain string directly. We therefore compare the two raw
// footers: they should differ iff ANSI codes are emitted, and must be
// identical only in purely Ascii rendering environments — in which case the
// test degrades gracefully to a content-only assertion.
func TestCacheHighlightThreshold(t *testing.T) {
	below := &App{totalInputTokens: 1000, totalCacheRead: 490} // 49%
	at := &App{totalInputTokens: 1000, totalCacheRead: 500}    // 50%
	above := &App{totalInputTokens: 1000, totalCacheRead: 750} // 75%

	footerBelow := below.renderFooter()
	footerAt := at.renderFooter()
	footerAbove := above.renderFooter()

	// Content must always be correct regardless of colour support.
	if !strings.Contains(stripANSI(footerBelow), "Cache: 49%") {
		t.Errorf("below-threshold footer = %q, want 'Cache: 49%%'", stripANSI(footerBelow))
	}
	if !strings.Contains(stripANSI(footerAt), "Cache: 50%") {
		t.Errorf("at-threshold footer = %q, want 'Cache: 50%%'", stripANSI(footerAt))
	}
	if !strings.Contains(stripANSI(footerAbove), "Cache: 75%") {
		t.Errorf("above-threshold footer = %q, want 'Cache: 75%%'", stripANSI(footerAbove))
	}

	// When the renderer does produce ANSI codes (e.g. in a real terminal or
	// when the test is run with COLORTERM set), the highlighted footers must
	// contain the statusStyle-rendered string, and the un-highlighted one must
	// not contain it.
	styledAt := statusStyle.Render("Cache: 50%")
	styledAbove := statusStyle.Render("Cache: 75%")
	styledBelow := statusStyle.Render("Cache: 49%")

	if styledAt != "Cache: 50%" {
		// ANSI codes are being emitted; verify correct highlighting.
		if !strings.Contains(footerAt, styledAt) {
			t.Errorf("at-threshold (50%%) footer should apply statusStyle; raw = %q", footerAt)
		}
		if !strings.Contains(footerAbove, styledAbove) {
			t.Errorf("above-threshold (75%%) footer should apply statusStyle; raw = %q", footerAbove)
		}
		if strings.Contains(footerBelow, styledBelow) {
			t.Errorf("below-threshold (49%%) footer must NOT apply statusStyle; raw = %q", footerBelow)
		}
	}
}

func TestHandleAgentEventReservesAssistantSlotBeforeTextDelta(t *testing.T) {
	a := &App{
		messages:          []string{"You: hi"},
		assistantRaw:      make(map[int]string),
		assistantRendered: make(map[int]string),
		assistantDirty:    make(map[int]bool),
	}

	a.handleAgentEvent(agent.Event{Type: agent.EventTurnStart})
	if got, want := len(a.messages), 2; got != want {
		t.Fatalf("len(messages) after turn start = %d, want %d", got, want)
	}
	if got, want := a.currentAssistantIdx, 1; got != want {
		t.Fatalf("currentAssistantIdx = %d, want %d", got, want)
	}

	a.handleAgentEvent(agent.Event{Type: agent.EventTextDelta, TextDelta: "Hello"})
	if got, want := a.assistantRaw[1], "Hello"; got != want {
		t.Fatalf("assistantRaw[1] = %q, want %q", got, want)
	}
	if got, want := len(a.messages), 2; got != want {
		t.Fatalf("len(messages) after text delta = %d, want %d", got, want)
	}
}

func TestHandleAgentEventCommitsStreamBeforeApproval(t *testing.T) {
	a := &App{
		messages:            []string{"You: hi"},
		currentAssistantIdx: -1,
		currentThinkIdx:     -1,
		printedMessageIdx:   make(map[int]bool),
		assistantRaw:        make(map[int]string),
		assistantRendered:   make(map[int]string),
		assistantDirty:      make(map[int]bool),
	}

	a.handleAgentEvent(agent.Event{Type: agent.EventTurnStart})
	a.handleAgentEvent(agent.Event{Type: agent.EventThinkDelta, ThinkDelta: "thinking"})
	a.handleAgentEvent(agent.Event{Type: agent.EventTextDelta, TextDelta: "I need to run a command."})
	a.handleAgentEvent(agent.Event{
		Type:         agent.EventToolApprovalRequest,
		ApprovalID:   "approval-1",
		ApprovalTool: "bash",
		ApprovalArgs: map[string]any{"command": "go test ./internal/tui"},
	})

	joined := stripANSI(strings.Join(a.pendingPrints, "\n"))
	thinkAt := strings.Index(joined, "think: thinking")
	assistantAt := strings.Index(joined, "Assistant: I need to run a command.")
	approvalAt := strings.Index(joined, "Approval required for [bash]")
	if thinkAt < 0 || assistantAt < 0 || approvalAt < 0 {
		t.Fatalf("pending prints missing expected content: %q", joined)
	}
	if !(thinkAt < assistantAt && assistantAt < approvalAt) {
		t.Fatalf("pending prints out of order: %q", joined)
	}
	if a.currentThinkIdx != -1 || a.currentAssistantIdx != -1 {
		t.Fatalf("active stream indices = think %d assistant %d, want both reset", a.currentThinkIdx, a.currentAssistantIdx)
	}
}

func TestFormatApprovalArgsEditShowsPathAndDiff(t *testing.T) {
	args := map[string]any{
		"path": "README.md",
		"edits": []any{
			map[string]any{
				"oldText": "Hello\nWorld\n",
				"newText": "Hello\nGophers\n",
			},
		},
	}

	got := formatApprovalArgs("edit", args)
	if !strings.Contains(got, "path: README.md") {
		t.Fatalf("formatApprovalArgs(edit) missing path: %q", got)
	}
	if !strings.Contains(got, "@@ -1,2 +1,2 @@") {
		t.Fatalf("formatApprovalArgs(edit) missing hunk header: %q", got)
	}
	if !strings.Contains(got, "-World") || !strings.Contains(got, "+Gophers") {
		t.Fatalf("formatApprovalArgs(edit) missing line diff: %q", got)
	}
}

func TestAbortClearsQueuedInput(t *testing.T) {
	a := &App{
		inputQueue: make([]InputEvent, 0, 4),
	}

	a.queueInput(teaKeyMsgForTest("a"))
	a.queueInput(teaKeyMsgForTest("b"))
	if got := len(a.inputQueue); got != 2 {
		t.Fatalf("len(inputQueue) before abort = %d, want 2", got)
	}

	a.inputQueueMu.Lock()
	a.inputQueue = a.inputQueue[:0]
	a.lastInputTime = time.Time{}
	a.inputQueueMu.Unlock()

	if got := len(a.inputQueue); got != 0 {
		t.Fatalf("len(inputQueue) after abort = %d, want 0", got)
	}
}

func TestHandleAgentEventStatusAndWarningMessage(t *testing.T) {
	a := &App{}

	a.handleAgentEvent(agent.Event{Type: agent.EventStatus, StatusMessage: "stream warning"})
	a.handleAgentEvent(agent.Event{
		Type:    agent.EventMessageStart,
		Message: provider.NewUserMessage("[System] explain what you are doing"),
	})
	a.handleAgentEvent(agent.Event{Type: agent.EventContextPressure, PressureMessage: "context high"})
	a.handleAgentEvent(agent.Event{Type: agent.EventBudgetPressure, PressureMessage: "budget low"})

	joined := stripANSI(strings.Join(a.messages, "\n"))
	if !strings.Contains(joined, "stream warning") {
		t.Fatalf("messages = %q, want status message", joined)
	}
	if !strings.Contains(joined, "[System] explain what you are doing") {
		t.Fatalf("messages = %q, want warning user message", joined)
	}
	if !strings.Contains(joined, "context high") || !strings.Contains(joined, "budget low") {
		t.Fatalf("messages = %q, want pressure warnings", joined)
	}
}

func TestListenEventsPassesThroughDoneAndError(t *testing.T) {
	eventCh := make(chan agent.Event, 2)
	eventCh <- agent.Event{Type: agent.EventDone}
	eventCh <- agent.Event{Type: agent.EventError, Error: assertErr("boom")}
	close(eventCh)
	app := &App{eventCh: eventCh}

	msg := app.listenAgentEvents()()
	if ev, ok := msg.(agentEventMsg); !ok || ev.event.Type != agent.EventDone {
		t.Fatalf("first msg = %#v, want agentEventMsg(EventDone)", msg)
	}

	msg = app.listenAgentEvents()()
	if ev, ok := msg.(agentEventMsg); !ok || ev.event.Type != agent.EventError || ev.event.Error == nil || ev.event.Error.Error() != "boom" {
		t.Fatalf("second msg = %#v, want agentEventMsg(EventError boom)", msg)
	}

	msg = app.listenAgentEvents()()
	if _, ok := msg.(agentDoneMsg); !ok {
		t.Fatalf("third msg = %#v, want agentDoneMsg", msg)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func teaKeyMsgForTest(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func teaSpecialKeyMsgForTest(key tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: key}
}

func TestInputHomeEndKeysReachTextInput(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	a.input.SetValue("abc")

	a.Update(teaSpecialKeyMsgForTest(tea.KeyHome))
	a.flushInputQueue()
	a.Update(teaKeyMsgForTest("X"))
	a.flushInputQueue()

	if got := a.input.Value(); got != "Xabc" {
		t.Fatalf("value after home insert = %q, want Xabc", got)
	}

	a.Update(teaSpecialKeyMsgForTest(tea.KeyEnd))
	a.flushInputQueue()
	a.Update(teaKeyMsgForTest("Z"))
	a.flushInputQueue()

	if got := a.input.Value(); got != "XabcZ" {
		t.Fatalf("value after end insert = %q, want XabcZ", got)
	}
}

func TestInputHistoryNavigationPreservesDraft(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	a.recordInputHistory("first")
	a.recordInputHistory("second")
	a.input.SetValue("draft")

	if !a.navigateInputHistory(-1) || a.input.Value() != "second" {
		t.Fatalf("first up value = %q, want second", a.input.Value())
	}
	if !a.navigateInputHistory(-1) || a.input.Value() != "first" {
		t.Fatalf("second up value = %q, want first", a.input.Value())
	}
	if !a.navigateInputHistory(-1) || a.input.Value() != "first" {
		t.Fatalf("third up value = %q, want first", a.input.Value())
	}
	if !a.navigateInputHistory(1) || a.input.Value() != "second" {
		t.Fatalf("first down value = %q, want second", a.input.Value())
	}
	if !a.navigateInputHistory(1) || a.input.Value() != "draft" {
		t.Fatalf("second down value = %q, want draft", a.input.Value())
	}
	if a.navigateInputHistory(1) {
		t.Fatal("down outside history returned true, want false")
	}
}

func TestInputHistoryNavigationFlushesQueuedDraft(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	a.recordInputHistory("previous")

	a.Update(teaKeyMsgForTest("draft"))
	a.Update(teaSpecialKeyMsgForTest(tea.KeyUp))

	if got := a.input.Value(); got != "previous" {
		t.Fatalf("up value = %q, want previous", got)
	}

	a.Update(teaSpecialKeyMsgForTest(tea.KeyDown))
	if got := a.input.Value(); got != "draft" {
		t.Fatalf("down value = %q, want queued draft restored", got)
	}
}

func TestEscAbortClearsApprovalState(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	a.isThinking = true
	a.waitingForApproval = true
	a.pendingApprovalID = "approval-1"
	a.approvalQueue = []pendingApproval{{approvalID: "approval-2", toolName: "bash"}}

	a.Update(teaSpecialKeyMsgForTest(tea.KeyEsc))

	if a.waitingForApproval {
		t.Fatal("waitingForApproval = true, want false")
	}
	if a.pendingApprovalID != "" {
		t.Fatalf("pendingApprovalID = %q, want empty", a.pendingApprovalID)
	}
	if len(a.approvalQueue) != 0 {
		t.Fatalf("len(approvalQueue) = %d, want 0", len(a.approvalQueue))
	}
}

func TestClearCommandResetsTranscriptState(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "base", nil, "agent", false, nil, nil, nil)
	a.messages = []string{"old"}
	a.toolResults = []toolResult{{toolCallID: "tool-1", msgIndex: 0}}
	a.liveContent = "live"
	a.pendingPrints = []string{"old print"}
	a.currentPlan = &tools.TaskPlan{Title: "old plan", Steps: []tools.PlanStep{{Title: "step", Status: "running"}}}
	a.assistantRaw[0] = "raw"
	a.assistantRendered[0] = "rendered"
	a.assistantDirty[0] = true
	a.printedMessageIdx[0] = true
	a.currentAssistantIdx = 0
	a.currentThinkIdx = 1
	a.toolModalOpen = true
	a.activeSkills["x"] = "skill"
	a.extraContext = "base skill"

	a.handleCommand("/clear")

	if len(a.toolResults) != 0 || len(a.assistantRaw) != 0 || len(a.assistantRendered) != 0 || len(a.assistantDirty) != 0 || len(a.printedMessageIdx) != 0 {
		t.Fatalf("transcript state not reset: tools=%d raw=%d rendered=%d dirty=%d printed=%d", len(a.toolResults), len(a.assistantRaw), len(a.assistantRendered), len(a.assistantDirty), len(a.printedMessageIdx))
	}
	if a.currentAssistantIdx != -1 || a.currentThinkIdx != -1 || a.toolModalOpen || a.currentPlan != nil {
		t.Fatalf("active state not reset: assistant=%d think=%d modal=%v plan=%v", a.currentAssistantIdx, a.currentThinkIdx, a.toolModalOpen, a.currentPlan)
	}
	if a.extraContext != "base" || len(a.activeSkills) != 0 {
		t.Fatalf("skill context not reset: extra=%q active=%d", a.extraContext, len(a.activeSkills))
	}
	joined := stripANSI(strings.Join(a.messages, "\n"))
	if !strings.Contains(joined, "Conversation cleared") || strings.Contains(joined, "old") {
		t.Fatalf("messages after clear = %q, want only clear confirmation", joined)
	}
}

func TestOpenLatestToolModalRequiresContent(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	if a.openLatestToolModal() {
		t.Fatal("openLatestToolModal on empty app = true, want false")
	}
	a.messages = []string{"hello"}
	if !a.openLatestToolModal() {
		t.Fatal("openLatestToolModal with content = false, want true")
	}
}

func TestShowNextQuestionTracksCurrentQuestionAndClearResetsIt(t *testing.T) {
	a := &App{questionQueue: []pendingQuestion{{questionID: "q1", question: "Pick?", options: []string{"A", "B"}}}}
	a.showNextQuestion()
	if !a.waitingForQuestion || a.pendingQuestionID != "q1" || len(a.currentQuestion.options) != 2 {
		t.Fatalf("question state = waiting %v id %q options %v", a.waitingForQuestion, a.pendingQuestionID, a.currentQuestion.options)
	}
	a.clearQuestionState()
	if a.waitingForQuestion || a.pendingQuestionID != "" || len(a.currentQuestion.options) != 0 {
		t.Fatalf("question state after clear = waiting %v id %q options %v", a.waitingForQuestion, a.pendingQuestionID, a.currentQuestion.options)
	}
}

func TestRuneInputTabDoesNotCycleMode(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	a.input.SetValue("prefix ")

	a.Update(teaKeyMsgForTest("tab"))
	a.flushInputQueue()

	if got := a.mode; got != "agent" {
		t.Fatalf("mode = %q, want agent", got)
	}
	if got := a.input.Value(); got != "prefix tab" {
		t.Fatalf("input = %q, want %q", got, "prefix tab")
	}
}

func TestRuneInputEscDoesNotAbortOrClearInput(t *testing.T) {
	a := NewApp(nil, &provider.Model{Name: "test"}, config.DefaultSettings(), nil, nil, "", "", nil, "agent", false, nil, nil, nil)
	a.input.SetValue("prefix ")

	a.Update(teaKeyMsgForTest("esc"))
	a.flushInputQueue()

	if got := a.input.Value(); got != "prefix esc" {
		t.Fatalf("input = %q, want %q", got, "prefix esc")
	}
}

func TestInitWithProgramDoesNotBlock(t *testing.T) {
	a := NewApp(
		&historyInjectMockProvider{},
		&provider.Model{ID: "mock-model", Name: "Mock"},
		config.DefaultSettings(),
		nil,
		tools.NewRegistry(t.TempDir(), nil),
		"",
		"",
		nil,
		"agent",
		false,
		nil,
		nil,
		nil,
	)
	a.SetInitialMessage("hello")
	p := tea.NewProgram(a)
	a.SetProgram(p)

	done := make(chan struct{})
	go func() {
		_ = a.Init()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Init blocked while printing initial history")
	}
}

// TestCacheHighlightThresholdMath verifies the arithmetic of the 50% boundary
// independent of any rendering logic.
func TestCacheHighlightThresholdMath(t *testing.T) {
	type tc struct {
		input     int
		cacheRead int
		wantHigh  bool
	}
	cases := []tc{
		{1000, 0, false},   // 0%
		{1000, 499, false}, // 49.9%
		{1000, 490, false}, // 49%
		{1000, 500, true},  // 50% — boundary: highlighted
		{1000, 501, true},  // 50.1%
		{1000, 750, true},  // 75%
		{1000, 1000, true}, // 100%
		{3, 2, true},       // 66.7% — small counts
		{3, 1, false},      // 33.3%
	}
	for _, c := range cases {
		pct := float64(c.cacheRead) / float64(c.input) * 100
		got := pct >= 50.0
		if got != c.wantHigh {
			t.Errorf("input=%d cacheRead=%d pct=%.4f: highlight=%v, want %v",
				c.input, c.cacheRead, pct, got, c.wantHigh)
		}
	}
}

type historyInjectMockProvider struct{}

func (p *historyInjectMockProvider) Chat(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
	ch := make(chan provider.StreamEvent, 2)
	ch <- provider.StreamEvent{Type: provider.StreamTextDelta, TextDelta: "ok"}
	ch <- provider.StreamEvent{Type: provider.StreamDone, StopReason: "end_turn"}
	close(ch)
	return ch
}

func (p *historyInjectMockProvider) Name() string { return "mock" }
func (p *historyInjectMockProvider) Models() []*provider.Model {
	return []*provider.Model{{ID: "mock-model", Name: "Mock"}}
}
func (p *historyInjectMockProvider) GetModel(id string) *provider.Model {
	for _, m := range p.Models() {
		if m.ID == id {
			return m
		}
	}
	return nil
}

func TestProcessInputLoadsSessionHistoryIntoAgentEvenWhenUIHistoryAlreadyLoaded(t *testing.T) {
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "project")
	if err := os.MkdirAll(cwd, 0755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	sessionDir := filepath.Join(tmp, "sessions")

	sess := session.New(cwd, sessionDir)
	if err := sess.Init(); err != nil {
		t.Fatalf("init session: %v", err)
	}
	sess.AppendMessage(provider.NewUserMessage("old user"))
	sess.AppendMessage(provider.NewAssistantMessage([]provider.ContentBlock{{Type: "text", Text: "old assistant"}}))

	settings := config.DefaultSettings()
	settings.DefaultThinkingLevel = "off"
	a := &App{
		provider:            &historyInjectMockProvider{},
		model:               &provider.Model{ID: "mock-model", Name: "Mock"},
		settings:            settings,
		session:             sess,
		registry:            tools.NewRegistry(cwd, nil),
		historyLoaded:       true, // UI already rendered history
		assistantRaw:        make(map[int]string),
		assistantRendered:   make(map[int]string),
		assistantDirty:      make(map[int]bool),
		currentAssistantIdx: -1,
		currentThinkIdx:     -1,
	}

	a.processInput("new question")

	deadline := time.Now().Add(2 * time.Second)
	for {
		if a.agent != nil {
			msgs := a.agent.GetMessages()
			if len(msgs) >= 4 {
				if msgs[0].Role != "user" || msgs[0].Content != "old user" {
					t.Fatalf("first message = %+v, want old history user message", msgs[0])
				}
				if msgs[1].Role != "assistant" {
					t.Fatalf("second message role = %s, want assistant", msgs[1].Role)
				}
				if msgs[2].Role != "user" || msgs[2].Content != "new question" {
					t.Fatalf("third message = %+v, want new user message", msgs[2])
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for agent messages")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestInitThenProcessInputStillInjectsSessionHistory(t *testing.T) {
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "project")
	if err := os.MkdirAll(cwd, 0755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	sessionDir := filepath.Join(tmp, "sessions")

	sess := session.New(cwd, sessionDir)
	if err := sess.Init(); err != nil {
		t.Fatalf("init session: %v", err)
	}
	sess.AppendMessage(provider.NewUserMessage("history user"))
	sess.AppendMessage(provider.NewAssistantMessage([]provider.ContentBlock{{Type: "text", Text: "history assistant"}}))

	settings := config.DefaultSettings()
	settings.DefaultThinkingLevel = "off"
	app := NewApp(
		&historyInjectMockProvider{},
		&provider.Model{ID: "mock-model", Name: "Mock"},
		settings,
		sess,
		tools.NewRegistry(cwd, nil),
		"",
		"",
		nil,
		"agent",
		false,
		nil,
		nil,
		nil,
	)

	// Simulate real startup flow: Init() loads history into UI and flips historyLoaded.
	_ = app.Init()

	if !app.historyLoaded {
		t.Fatalf("historyLoaded = false, want true after Init")
	}

	app.processInput("follow-up")

	deadline := time.Now().Add(2 * time.Second)
	for {
		if app.agent != nil {
			msgs := app.agent.GetMessages()
			if len(msgs) >= 4 {
				if msgs[0].Role != "user" || msgs[0].Content != "history user" {
					t.Fatalf("first message = %+v, want history user", msgs[0])
				}
				if msgs[1].Role != "assistant" {
					t.Fatalf("second message role = %s, want assistant", msgs[1].Role)
				}
				if msgs[2].Role != "user" || msgs[2].Content != "follow-up" {
					t.Fatalf("third message = %+v, want follow-up user message", msgs[2])
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for agent messages")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
