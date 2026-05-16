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
	args := []string{"-f", profilePath, shell, "-c", cmd}
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

	b.WriteString(`(version 1)
(allow default)
(deny network*)
(deny process-fork)
(deny process-exec)
(allow process-exec
`)

	// Allow common shells and tools
	allowedBins := []string{
		"/bin/sh", "/bin/bash", "/bin/zsh",
		"/usr/bin/env", "/usr/bin/perl", "/usr/bin/python3",
		"/usr/local/bin/*", "/opt/homebrew/bin/*",
	}
	for _, bin := range allowedBins {
		b.WriteString(fmt.Sprintf("    (subpath \"%s\")\n", bin))
	}

	b.WriteString(")\n")

	// Project directory access
	if s.projectDir != "" {
		if s.level == LevelStrict {
			b.WriteString(fmt.Sprintf("(allow file-read* (subpath \"%s\"))\n", s.projectDir))
		} else {
			b.WriteString(fmt.Sprintf("(allow file-read* file-write* (subpath \"%s\"))\n", s.projectDir))
		}
	}

	// Temporary directory access
	b.WriteString(fmt.Sprintf("(allow file-read* file-write* (subpath \"%s\"))\n", os.TempDir()))

	// Home directory for config files
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		configPaths := []string{
			filepath.Join(homeDir, ".config"),
			filepath.Join(homeDir, ".cache"),
			filepath.Join(homeDir, ".vibecoding"),
		}
		for _, p := range configPaths {
			b.WriteString(fmt.Sprintf("(allow file-read* file-write* (subpath \"%s\"))\n", p))
		}
	}

	// Allow additional writable paths
	for _, p := range opts.WritablePaths {
		b.WriteString(fmt.Sprintf("(allow file-read* file-write* (subpath \"%s\"))\n", p))
	}

	// Allow additional read-only paths
	for _, p := range opts.ReadOnlyPaths {
		b.WriteString(fmt.Sprintf("(allow file-read* (subpath \"%s\"))\n", p))
	}

	return b.String()
}
