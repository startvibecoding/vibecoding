package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"syscall"
	"testing"
	"time"
)

func TestIsRetryable_NetworkErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		want bool
	}{
		{"nil error", nil, 0, false},
		{"429", nil, http.StatusTooManyRequests, true},
		{"502", nil, http.StatusBadGateway, true},
		{"503", nil, http.StatusServiceUnavailable, true},
		{"504", nil, http.StatusGatewayTimeout, true},
		{"500 not retryable", nil, http.StatusInternalServerError, false},
		{"400 not retryable", nil, http.StatusBadRequest, false},
		{"401 not retryable", nil, http.StatusUnauthorized, false},
		{"ECONNRESET", syscall.ECONNRESET, 0, true},
		{"ECONNREFUSED", syscall.ECONNREFUSED, 0, true},
		{"EPIPE", syscall.EPIPE, 0, true},
		{"ETIMEDOUT", syscall.ETIMEDOUT, 0, true},
		{"context canceled", context.Canceled, 0, false},
		{"generic error", errors.New("something"), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err, tt.code)
			if got != tt.want {
				t.Errorf("IsRetryable(%v, %d) = %v, want %v", tt.err, tt.code, got, tt.want)
			}
		})
	}
}

func TestRetryDelay_ExponentialBackoff(t *testing.T) {
	base := 2000

	d0 := RetryDelay(0, base)
	d1 := RetryDelay(1, base)
	d2 := RetryDelay(2, base)

	if d0 != 2000*time.Millisecond {
		t.Errorf("delay(0) = %v, want 2s", d0)
	}
	if d1 != 4000*time.Millisecond {
		t.Errorf("delay(1) = %v, want 4s", d1)
	}
	if d2 != 8000*time.Millisecond {
		t.Errorf("delay(2) = %v, want 8s", d2)
	}
}

func TestRetryDelay_CappedAt30s(t *testing.T) {
	d := RetryDelay(10, 5000)
	if d > 30*time.Second {
		t.Errorf("delay(10, 5000) = %v, want <= 30s", d)
	}
}

func TestRetryDelay_DefaultBase(t *testing.T) {
	d := RetryDelay(0, 0) // baseDelayMs <= 0 defaults to 2000
	if d != 2000*time.Millisecond {
		t.Errorf("delay(0, 0) = %v, want 2s", d)
	}
}

func TestFormatRetryMessage_Timeout(t *testing.T) {
	msg := FormatRetryMessage(0, 3, 2*time.Second, fmt.Errorf("context deadline exceeded"))
	if msg == "" {
		t.Error("expected non-empty message")
	}
	t.Logf("timeout: %s", msg)
}

func TestFormatRetryMessage_RateLimited(t *testing.T) {
	msg := FormatRetryMessage(1, 3, 4*time.Second, fmt.Errorf("HTTP 429: rate limit"))
	if msg == "" {
		t.Error("expected non-empty message")
	}
	t.Logf("rate limited: %s", msg)
}

func TestFormatRetryMessage_ConnectionRefused(t *testing.T) {
	msg := FormatRetryMessage(2, 3, 8*time.Second, fmt.Errorf("connection refused"))
	if msg == "" {
		t.Error("expected non-empty message")
	}
	t.Logf("conn refused: %s", msg)
}

func TestFormatRetryMessage_Generic(t *testing.T) {
	msg := FormatRetryMessage(0, 3, 2*time.Second, fmt.Errorf("some random error"))
	if msg == "" {
		t.Error("expected non-empty message")
	}
	t.Logf("generic: %s", msg)
}
