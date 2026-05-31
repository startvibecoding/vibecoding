package messaging

import (
	"strings"
	"sync"
)

// ProgressBuffer collects progress lines and flushes them in batches.
// Designed for messaging platforms with per-message reply limits (e.g., WeChat: 10 replies per user message).
//
// Usage:
//
//	buf := NewProgressBuffer(maxLines, sendFunc)
//	// During agent execution:
//	buf.Add("[read]: file.go ✅")   // buffered
//	buf.Add("[bash]: go build ✅")  // buffered, auto-flushes if full
//	// After agent completes:
//	buf.Flush()                      // send remaining lines
type ProgressBuffer struct {
	mu       sync.Mutex
	lines    []string
	maxLines int         // max lines before auto-flush
	reserve  int         // lines reserved for final summary (not counted in maxLines)
	sendFunc func(string) // combined send function
	total    int         // total lines added (for logging)
}

// NewProgressBuffer creates a progress buffer.
//
//	maxLines: max progress lines to collect before auto-flushing (e.g., 7)
//	reserve: lines reserved for final summary, subtracted from platform limit (e.g., 3)
//	sendFunc: function to send combined text (e.g., WeChat SendMessage)
func NewProgressBuffer(maxLines int, sendFunc func(string)) *ProgressBuffer {
	if maxLines <= 0 {
		maxLines = 7
	}
	return &ProgressBuffer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
		reserve:  3,
		sendFunc: sendFunc,
	}
}

// Add adds a progress line. Auto-flushes when buffer is full.
func (b *ProgressBuffer) Add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, line)
	b.total++

	if len(b.lines) >= b.maxLines {
		b.flushLocked()
	}
}

// Flush sends any remaining buffered lines. Call after agent completes.
func (b *ProgressBuffer) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// flushLocked sends buffered lines and clears the buffer. Must hold b.mu.
func (b *ProgressBuffer) flushLocked() {
	if len(b.lines) == 0 || b.sendFunc == nil {
		return
	}
	combined := strings.Join(b.lines, "\n")
	b.sendFunc(combined)
	b.lines = b.lines[:0]
}
