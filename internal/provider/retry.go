package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// RetryConfig controls automatic retry behavior for API calls.
type RetryConfig struct {
	Enabled     bool
	MaxRetries  int
	BaseDelayMs int
}

// IsRetryable determines whether an error or HTTP status code warrants a retry.
// Returns true for transient network errors and server-side overload/status errors.
func IsRetryable(err error, statusCode int) bool {
	// Check HTTP status codes
	if statusCode == http.StatusTooManyRequests || // 429
		statusCode == http.StatusBadGateway || // 502
		statusCode == http.StatusServiceUnavailable || // 503
		statusCode == http.StatusGatewayTimeout { // 504
		return true
	}

	if err == nil {
		return false
	}

	// Context cancellation is never retryable (user abort)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// For the HTTP client 30-minute timeout, this wraps as DeadlineExceeded.
		// However, user-initiated context cancellation also uses this.
		// We treat it as retryable only for the HTTP client timeout case,
		// which is distinguishable by the wrapped net.Error.
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return true
		}
		return false
	}

	// Network-level transient errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true // timeouts, connection refused, etc.
	}

	// Connection reset, broken pipe, etc.
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	// DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Generic "server closed connection" type errors
	errStr := err.Error()
	if strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "EOF") {
		return true
	}

	return false
}

// RetryDelay calculates the delay before the next retry attempt using
// exponential backoff with jitter, capped at 30 seconds.
func RetryDelay(attempt int, baseDelayMs int) time.Duration {
	if baseDelayMs <= 0 {
		baseDelayMs = 2000
	}
	delay := float64(baseDelayMs) * math.Pow(2, float64(attempt))
	if delay > 30000 {
		delay = 30000
	}
	return time.Duration(delay) * time.Millisecond
}

// FormatRetryMessage returns a user-visible message for a retry attempt.
func FormatRetryMessage(attempt, maxRetries int, delay time.Duration, err error) string {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	// Classify the error for a user-friendly message
	var reason string
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "DeadlineExceeded"):
		reason = "request timed out"
	case strings.Contains(errStr, "connection refused"):
		reason = "connection refused"
	case strings.Contains(errStr, "connection reset"):
		reason = "connection reset"
	case strings.Contains(errStr, "429"):
		reason = "rate limited (HTTP 429)"
	case strings.Contains(errStr, "502"):
		reason = "bad gateway (HTTP 502)"
	case strings.Contains(errStr, "503"):
		reason = "service unavailable (HTTP 503)"
	case strings.Contains(errStr, "504"):
		reason = "gateway timeout (HTTP 504)"
	case strings.Contains(errStr, "EOF"):
		reason = "connection closed unexpectedly"
	default:
		reason = fmt.Sprintf("error: %s", truncateErr(errStr, 80))
	}

	return fmt.Sprintf("Retrying (%d/%d): %s — waiting %s...",
		attempt+1, maxRetries, reason, formatDelay(delay))
}

// truncateErr truncates an error string to maxLen characters.
func truncateErr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatDelay formats a duration in a human-readable way.
func formatDelay(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
