package main

import (
	"os"
	"strings"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

func TestRunPrintFailsWhenApprovalWouldBeRequired(t *testing.T) {
	p := provider.NewMockProvider("mock", []*provider.Model{{ID: "model1", Name: "Model 1", ContextWindow: 128000}}, []provider.StreamEvent{
		{Type: provider.StreamStart},
		{Type: provider.StreamToolCall, ToolCall: &provider.ToolCallBlock{ID: "call_1", Name: "bash", Arguments: []byte(`{"command":"python script.py"}`)}},
		{Type: provider.StreamDone},
	})
	registry := tools.NewRegistry(t.TempDir(), nil)
	registry.RegisterDefaults()
	settings := config.DefaultSettings()
	settings.Approval.BashWhitelist = []string{"go "}

	err := runPrint(
		[]string{"run"},
		p,
		p.Models()[0],
		"agent",
		provider.ThinkingOff,
		settings,
		registry,
		(*session.Manager)(nil),
		"",
		false,
		nil,
	)
	if err == nil {
		t.Fatal("expected runPrint to fail when approval is required")
	}
	if !strings.Contains(err.Error(), "tool approval required in print mode") {
		t.Fatalf("err = %q, want approval error", err)
	}
}

func TestRunSetsProviderDebugEnvWhenDebugFlagSet(t *testing.T) {
	origDebug := debugEnabled
	origEnv, hadEnv := os.LookupEnv("VIBECODING_DEBUG")
	defer func() {
		debugEnabled = origDebug
		if hadEnv {
			_ = os.Setenv("VIBECODING_DEBUG", origEnv)
		} else {
			_ = os.Unsetenv("VIBECODING_DEBUG")
		}
	}()
	_ = os.Unsetenv("VIBECODING_DEBUG")

	cmd := newRootCommand(
		func(args []string, opts runOptions) error {
			debugEnabled = opts.debug
			if opts.debug {
				_ = os.Setenv("VIBECODING_DEBUG", "1")
			}
			return nil
		},
		nil,
	)
	cmd.SetArgs([]string{"--debug", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if got := os.Getenv("VIBECODING_DEBUG"); got != "1" {
		t.Fatalf("VIBECODING_DEBUG = %q, want 1", got)
	}
}
