package ua

import (
	"runtime"
	"strings"
	"testing"
)

func TestUserAgent(t *testing.T) {
	ua := UserAgent()

	if ua == "" {
		t.Fatal("expected non-empty user agent")
	}

	// Should contain Claude-User
	if !strings.Contains(ua, "Claude-User") {
		t.Error("expected user agent to contain 'Claude-User'")
	}

	// Should contain vibecoding
	if !strings.Contains(ua, "vibecoding/") {
		t.Error("expected user agent to contain 'vibecoding/'")
	}

	// Should contain support URL
	if !strings.Contains(ua, "github.com/fuckvibecoding/vibecoding") {
		t.Error("expected user agent to contain support URL")
	}
}

func TestProviderUserAgent(t *testing.T) {
	ua := ProviderUserAgent()

	if ua == "" {
		t.Fatal("expected non-empty provider user agent")
	}

	// Should be same as UserAgent
	if ua != UserAgent() {
		t.Errorf("expected provider user agent to be same as user agent")
	}
}

func TestDetailedUserAgent(t *testing.T) {
	ua := DetailedUserAgent()

	if ua == "" {
		t.Fatal("expected non-empty detailed user agent")
	}

	// Should contain Claude-User
	if !strings.Contains(ua, "Claude-User") {
		t.Error("expected detailed user agent to contain 'Claude-User'")
	}

	// Should contain OS
	if !strings.Contains(ua, runtime.GOOS) {
		t.Errorf("expected detailed user agent to contain '%s'", runtime.GOOS)
	}

	// Should contain architecture
	if !strings.Contains(ua, runtime.GOARCH) {
		t.Errorf("expected detailed user agent to contain '%s'", runtime.GOARCH)
	}
}

func TestVersion(t *testing.T) {
	// Default version should be "dev"
	if Version != "dev" {
		t.Errorf("expected default version to be 'dev', got '%s'", Version)
	}
}
