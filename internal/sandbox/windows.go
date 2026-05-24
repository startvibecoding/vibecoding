//go:build windows

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/startvibecoding/vibecoding/internal/platform"
)

// winSandbox implements a basic sandbox for Windows.
// Note: Full Windows sandboxing requires AppContainers or similar,
// which is complex. This provides basic isolation.
type winSandbox struct {
	level      Level
	projectDir string
	available  *bool
}

// newWinSandbox creates a new Windows sandbox.
func newWinSandbox(projectDir string, level Level) *winSandbox {
	absDir, _ := filepath.Abs(projectDir)

	return &winSandbox{
		level:      level,
		projectDir: absDir,
	}
}

// IsAvailable checks if the Windows sandbox is available.
func (s *winSandbox) IsAvailable() bool {
	if s.available != nil {
		return *s.available
	}

	// Windows sandbox is always available (basic implementation)
	t := true
	s.available = &t
	return true
}

// Name returns "windows-sandbox".
func (s *winSandbox) Name() string {
	return "windows-sandbox"
}

// Level returns the sandbox level.
func (s *winSandbox) Level() Level {
	return s.level
}

// WrapCommand wraps a command for execution inside Windows sandbox.
// This is a basic implementation that restricts environment variables.
func (s *winSandbox) WrapCommand(ctx context.Context, shell, cmd string, opts ExecOpts) *exec.Cmd {
	// Use the specified shell or default
	if shell == "" {
		shell = "cmd.exe"
	}

	c := exec.CommandContext(ctx, shell, platform.ShellArgs(shell, cmd)...)
	c.Dir = opts.WorkDir

	// Build restricted environment
	env := s.buildEnv(opts)
	c.Env = env

	return c
}

// buildEnv constructs a restricted environment for Windows.
func (s *winSandbox) buildEnv(opts ExecOpts) []string {
	var env []string

	// Essential Windows environment variables
	essentialVars := []string{
		"PATH",
		"SystemRoot",
		"SYSTEMROOT",
		"windir",
		"COMSPEC",
		"PATHEXT",
		"TEMP",
		"TMP",
		"HOME",
		"USERPROFILE",
		"USERNAME",
		"APPDATA",
		"LOCALAPPDATA",
		"ProgramFiles",
		"ProgramFiles(x86)",
		"CommonProgramFiles",
		"CommonProgramFiles(x86)",
		"NUMBER_OF_PROCESSORS",
		"PROCESSOR_ARCHITECTURE",
		"PROCESSOR_IDENTIFIER",
		"OS",
		"COMPUTERNAME",
	}

	// Create a map for quick lookup
	essentialMap := make(map[string]bool)
	for _, v := range essentialVars {
		essentialMap[v] = true
	}

	// Copy only essential variables from current environment
	for _, e := range os.Environ() {
		parts := splitEnvVar(e)
		if len(parts) == 2 && essentialMap[parts[0]] {
			env = append(env, e)
		}
	}

	// Add environment variables from options
	for k, v := range opts.EnvVars {
		env = append(env, k+"="+v)
	}

	return env
}

// splitEnvVar splits an environment variable into name and value.
func splitEnvVar(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
