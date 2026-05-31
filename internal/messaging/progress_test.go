package messaging

import (
	"strings"
	"testing"
)

func TestProgressBufferBasic(t *testing.T) {
	var sent []string
	buf := NewProgressBuffer(7, func(text string) {
		sent = append(sent, text)
	})

	buf.Add("line1")
	buf.Add("line2")
	buf.Flush()

	if len(sent) != 1 {
		t.Fatalf("expected 1 flush, got %d", len(sent))
	}
	if !strings.Contains(sent[0], "line1") || !strings.Contains(sent[0], "line2") {
		t.Errorf("unexpected flush content: %s", sent[0])
	}
}

func TestProgressBufferAutoFlush(t *testing.T) {
	var sent []string
	buf := NewProgressBuffer(3, func(text string) {
		sent = append(sent, text)
	})

	buf.Add("a")
	buf.Add("b")
	buf.Add("c") // should trigger auto-flush

	if len(sent) != 1 {
		t.Fatalf("expected 1 auto-flush, got %d", len(sent))
	}
	if !strings.Contains(sent[0], "a") || !strings.Contains(sent[0], "c") {
		t.Errorf("unexpected auto-flush content: %s", sent[0])
	}

	// Buffer should be empty now
	buf.Flush()
	if len(sent) != 1 {
		t.Errorf("expected no additional flush, got %d total", len(sent)-1)
	}
}

func TestProgressBufferMultipleFlushes(t *testing.T) {
	var sent []string
	buf := NewProgressBuffer(2, func(text string) {
		sent = append(sent, text)
	})

	buf.Add("1")
	buf.Add("2") // auto-flush
	buf.Add("3")
	buf.Flush() // manual flush

	if len(sent) != 2 {
		t.Fatalf("expected 2 flushes, got %d", len(sent))
	}
	if !strings.Contains(sent[0], "1") || !strings.Contains(sent[0], "2") {
		t.Errorf("first flush: %s", sent[0])
	}
	if !strings.Contains(sent[1], "3") {
		t.Errorf("second flush: %s", sent[1])
	}
}

func TestProgressBufferEmpty(t *testing.T) {
	called := false
	buf := NewProgressBuffer(7, func(text string) {
		called = true
	})

	buf.Flush()
	if called {
		t.Error("flush on empty buffer should not call sendFunc")
	}
}

func TestProgressBufferNilSendFunc(t *testing.T) {
	buf := NewProgressBuffer(7, nil)
	buf.Add("test")
	buf.Flush() // should not panic
}
