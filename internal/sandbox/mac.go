//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/startvibecoding/vibecoding/internal/platform"
)

// macSandbox implements sandbox using macOS sandbox-exec (Seatbelt).
type macSandbox struct {
	level      Level
	projectDir string
	availMu    sync.Mutex
	available  *bool
}

// newMacSandbox creates a new macOS sandbox.
func newMacSandbox(projectDir string, level Level) *macSandbox {
	absDir, _ := filepath.Abs(projectDir)

	return &macSandbox{
		level:      level,
		projectDir: absDir,
	}
}

// IsAvailable checks if sandbox-exec is available on this system.
func (s *macSandbox) IsAvailable() bool {
	s.availMu.Lock()
	defer s.availMu.Unlock()

	if s.available != nil {
		return *s.available
	}

	// sandbox-exec is available on all macOS versions
	// Check if we can find it
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		f := false
		s.available = &f
		return false
	}

	t := true
	s.available = &t
	return true
}

// Name returns "sandbox-exec".
func (s *macSandbox) Name() string {
	return "sandbox-exec"
}

// Level returns the sandbox level.
func (s *macSandbox) Level() Level {
	return s.level
}

// WrapCommand wraps a command for execution inside macOS sandbox.
func (s *macSandbox) WrapCommand(ctx context.Context, shell, cmd string, opts ExecOpts) *exec.Cmd {
	// Generate sandbox profile
	profile := s.buildProfile(opts)

	// Create a temporary profile file with a unique name to avoid races
	f, err := os.CreateTemp(os.TempDir(), "vibecoding-sandbox-*.sb")
	if err != nil {
		// Fallback: if we can't create a temp file, return a command that will fail
		return exec.CommandContext(ctx, "false")
	}
	if _, err := f.WriteString(profile); err != nil {
		f.Close()
		os.Remove(f.Name())
		return exec.CommandContext(ctx, "false")
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return exec.CommandContext(ctx, "false")
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return exec.CommandContext(ctx, "false")
	}

	profilePath := f.Name()

	// sandbox-exec -f profile.sb command
	args := append([]string{"-f", profilePath, shell}, platform.ShellArgs(shell, cmd)...)
	c := exec.CommandContext(ctx, "sandbox-exec", args...)
	c.Dir = opts.WorkDir

	// Set environment variables
	c.Env = os.Environ()
	for k, v := range opts.EnvVars {
		c.Env = append(c.Env, k+"="+v)
	}

	return c
}

// buildProfile generates a sandbox profile based on the level.
func (s *macSandbox) buildProfile(opts ExecOpts) string {
	var b strings.Builder

	// Build sandbox profile with default-deny policy.
	// Only explicitly allowed operations are permitted.
	b.WriteString("(version 1)\n(deny default)\n")

	// Allow process execution for common shells and tools
	allowedBins := []string{
		"/bin/sh", "/bin/bash", "/bin/zsh",
		"/usr/bin/env", "/usr/bin/perl", "/usr/bin/python3",
		"/usr/local/bin/*", "/opt/homebrew/bin/*",
	}
	b.WriteString("(allow process-exec\n")
	for _, bin := range allowedBins {
		b.WriteString(fmt.Sprintf("    (subpath \"%s\")\n", bin))
	}
	b.WriteString(")\n")

	// Allow file system access for allowed paths
	var allowedPaths []string
	if s.projectDir != "" {
		allowedPaths = append(allowedPaths, s.projectDir)
	}
	allowedPaths = append(allowedPaths, os.TempDir())

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		allowedPaths = append(allowedPaths,
			filepath.Join(homeDir, ".config"),
			filepath.Join(homeDir, ".cache"),
			filepath.Join(homeDir, ".vibecoding"),
		)
	}
	for _, p := range opts.WritablePaths {
		allowedPaths = append(allowedPaths, p)
	}
	for _, p := range opts.ReadOnlyPaths {
		allowedPaths = append(allowedPaths, p)
	}

	for _, p := range allowedPaths {
		if s.level == LevelStrict && p == s.projectDir {
			b.WriteString(fmt.Sprintf("(allow file-read* (subpath \"%s\"))\n", p))
		} else {
			b.WriteString(fmt.Sprintf("(allow file-read* file-write* (subpath \"%s\"))\n", p))
		}
	}

	// Deny network explicitly
	b.WriteString("(deny network*)\n")
	// Deny process fork
	b.WriteString("(deny process-fork)\n")

	return b.String()
}
