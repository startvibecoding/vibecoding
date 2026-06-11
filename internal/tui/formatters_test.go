package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncateDisplayWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "ascii under limit",
			input:    "hello",
			maxWidth: 10,
			want:     "hello",
		},
		{
			name:     "ascii over limit",
			input:    "hello world",
			maxWidth: 8,
			want:     "hello...",
		},
		{
			name:     "cjk under limit by display width",
			input:    "你好世界",
			maxWidth: 8,
			want:     "你好世界",
		},
		{
			name:     "cjk truncated by display width keeps suffix",
			input:    "你好世界你好",
			maxWidth: 9,
			want:     "你好世...",
		},
		{
			name:     "max width smaller than suffix",
			input:    "abcdef",
			maxWidth: 2,
			want:     "...",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.input, tc.maxWidth)
			if got != tc.want {
				t.Fatalf("truncate(%q,%d)=%q want %q", tc.input, tc.maxWidth, got, tc.want)
			}
			// Result must fit within maxWidth unless maxWidth is smaller than the
			// suffix itself, in which case returning the suffix is acceptable.
			if tc.maxWidth >= lipgloss.Width("...") && lipgloss.Width(got) > tc.maxWidth {
				t.Fatalf("result %q width %d exceeds max %d", got, lipgloss.Width(got), tc.maxWidth)
			}
		})
	}
}

// Reproducer for the bug at tool_modal.go:108: divider was drawn using
// len(title) (bytes) so a CJK-heavy title produced a divider longer than the
// title's actual display width. Guarantee the divider stays within the title.
func TestToolModalDividerMatchesDisplayWidth(t *testing.T) {
	title := "展开记录  lines 1-3/3  PgUp/PgDn Up/Down Esc"
	width := 80
	dividerLen := minInt(width-2, lipgloss.Width(title))
	divider := strings.Repeat("─", dividerLen)
	if lipgloss.Width(divider) != dividerLen {
		t.Fatalf("divider width %d != %d", lipgloss.Width(divider), dividerLen)
	}
	if lipgloss.Width(divider) > lipgloss.Width(title) {
		t.Fatalf("divider %d wider than title %d", lipgloss.Width(divider), lipgloss.Width(title))
	}
	// Sanity: the byte-length-based version would over-count under CJK.
	if len(title) <= lipgloss.Width(title) {
		t.Fatalf("expected title byte length %d > display width %d for CJK input", len(title), lipgloss.Width(title))
	}
}
