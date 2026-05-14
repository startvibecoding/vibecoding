package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// BwrapSandbox implements sandbox using bubblewrap (bwrap).
type BwrapSandbox struct {
	level      Level
	projectDir string
	bwrapPath  string
	available  *bool // cached availability check
}

// NewBwrapSandbox creates a new bubblewrap sandbox.
func NewBwrapSandbox(projectDir string, level Level) *BwrapSandbox {
	absDir, _ := filepath.Abs(projectDir)

	return &BwrapSandbox{
		level:      level,
		projectDir: absDir,
		bwrapPath:  findBwrap(),
	}
}

// findBwrap locates the bwrap binary.
func findBwrap() string {
	// Check common locations
	candidates := []string{
		"/usr/bin/bwrap",
		"/usr/local/bin/bwrap",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// Try PATH
	if path, err := exec.LookPath("bwrap"); err == nil {
		return path
	}

	return ""
}

// IsAvailable checks if bwrap is available on this system.
func (s *BwrapSandbox) IsAvailable() bool {
	if s.available != nil {
		return *s.available
	}

	// bwrap is Linux only
	if runtime.GOOS != "linux" {
		f := false
		s.available = &f
		return false
	}

	// Check if bwrap binary exists
	if s.bwrapPath == "" {
		f := false
		s.available = &f
		return false
	}

	// Test that bwrap works
	cmd := exec.Command(s.bwrapPath, "--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib", "/bin/true")
	if err := cmd.Run(); err != nil {
		f := false
		s.available = &f
		return false
	}

	t := true
	s.available = &t
	return true
}

// Name returns "bwrap".
func (s *BwrapSandbox) Name() string {
	return "bwrap"
}

// Level returns the sandbox level.
func (s *BwrapSandbox) Level() Level {
	return s.level
}

// WrapCommand wraps a command for execution inside bubblewrap.
func (s *BwrapSandbox) WrapCommand(ctx context.Context, shell, cmd string, opts ExecOpts) *exec.Cmd {
	args := s.buildBwrapArgs(opts, shell, cmd)
	c := exec.CommandContext(ctx, s.bwrapPath, args...)
	c.Dir = opts.WorkDir

	// Pass through allowed environment variables
	c.Env = s.buildEnv(opts)

	return c
}

// buildBwrapArgs constructs the bwrap command arguments.
func (s *BwrapSandbox) buildBwrapArgs(opts ExecOpts, shell, cmd string) []string {
	args := []string{
		// Unshare namespaces
		"--unshare-pid",
		"--unshare-ipc",

		// Die when parent dies
		"--die-with-parent",

		// Proc filesystem (minimal)
		"--proc", "/proc",

		// Dev filesystem (minimal - null, zero, urandom)
		"--dev", "/dev",

		// Tmp filesystem
		"--tmpfs", "/tmp",
	}

	// Network isolation (unless explicitly allowed)
	if !opts.NetworkAccess {
		args = append(args, "--unshare-net")
	}

	// Size limit on tmpfs
	args = append(args, "--size", "100000000") // ~100MB default

	// System libraries (read-only)
	systemPaths := []string{"/usr", "/lib", "/lib64", "/bin", "/sbin"}
	for _, p := range systemPaths {
		if _, err := os.Stat(p); err == nil {
			args = append(args, "--ro-bind", p, p)
		}
	}

	// Additional system paths
	roPaths := []string{
		"/etc/ld.so.cache",
		"/etc/ssl",
		"/etc/ca-certificates",
		"/etc/resolv.conf",
		"/etc/hosts",
		"/etc/nsswitch.conf",
	}
	for _, p := range roPaths {
		if _, err := os.Stat(p); err == nil {
			args = append(args, "--ro-bind", p, p)
		}
	}

	// Home directory: use tmpfs to prevent access to real home
	// NOTE: This must be set BEFORE project directory binding if project is under home
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		args = append(args, "--tmpfs", homeDir)
	}

	// Project directory binding (must be after home tmpfs if project is under home)
	if s.projectDir != "" {
		if s.level == LevelStrict {
			// Read-only in strict mode
			args = append(args, "--ro-bind", s.projectDir, s.projectDir)
		} else {
			// Read-write in standard mode
			args = append(args, "--bind", s.projectDir, s.projectDir)
		}
	}

	// Additional read-only paths from options
	for _, p := range opts.ReadOnlyPaths {
		if _, err := os.Stat(p); err == nil {
			args = append(args, "--ro-bind", p, p)
		}
	}

	// Additional writable paths from options
	for _, p := range opts.WritablePaths {
		if _, err := os.Stat(p); err == nil {
			args = append(args, "--bind", p, p)
		}
	}

	// Set hostname
	args = append(args, "--hostname", "sandbox")

	// Working directory
	if opts.WorkDir != "" {
		args = append(args, "--chdir", opts.WorkDir)
	} else if s.projectDir != "" {
		args = append(args, "--chdir", s.projectDir)
	}

	// Environment variables
	for k, v := range opts.EnvVars {
		args = append(args, "--setenv", k, v)
	}

	// The actual command
	args = append(args, shell, "-c", cmd)

	return args
}

// buildEnv constructs the environment for the sandboxed process.
func (s *BwrapSandbox) buildEnv(opts ExecOpts) []string {
	var env []string

	// Default pass-through variables
	defaultPass := []string{
		"PATH", "LANG", "LC_ALL", "TERM",
		"GOPATH", "GOROOT", "GOPROXY", "GOMODCACHE",
		"NODE_PATH", "NPM_CONFIG_PREFIX",
		"HOME", "USER", "SHELL",
	}

	passVars := make(map[string]bool)
	for _, v := range defaultPass {
		passVars[v] = true
	}

	// Add explicitly passed env vars
	if v := os.Getenv("VIBECODING_SANDBOX_PASS_ENV"); v != "" {
		for _, name := range strings.Split(v, ",") {
			passVars[strings.TrimSpace(name)] = true
		}
	}

	// Copy allowed env vars from current environment
	for _, e := range os.Getenv("PATH") {
		_ = e
	}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]
		if passVars[name] {
			env = append(env, entry)
		}
	}

	// Add extra env vars from options
	for k, v := range opts.EnvVars {
		env = append(env, k+"="+v)
	}

	// Set HOME to tmp
	env = append(env, "HOME=/tmp")

	return env
}

// FormatSandboxInfo returns a human-readable description of the sandbox state.
func FormatSandboxInfo(s Sandbox) string {
	if s == nil || s.Level() == LevelNone {
		return "🔓 No sandbox (YOLO mode)"
	}

	available := "✓"
	if !s.IsAvailable() {
		available = "✗"
	}

	name := s.Name()
	switch s.Level() {
	case LevelStrict:
		return fmt.Sprintf("🔒 Strict sandbox [%s: %s] - read-only project, no network", name, available)
	case LevelStandard:
		return fmt.Sprintf("🔒 Standard sandbox [%s: %s] - read-write project, no network", name, available)
	default:
		return "🔓 No sandbox"
	}
}
