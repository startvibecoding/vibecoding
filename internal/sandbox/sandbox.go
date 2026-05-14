package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Level defines the sandbox restriction level.
type Level int

const (
	LevelStrict   Level = iota // Plan mode: read-only project, no network
	LevelStandard              // Agent mode: read-write project, no network
	LevelNone                  // YOLO mode: no restrictions
)

// String returns the string representation of a Level.
func (l Level) String() string {
	switch l {
	case LevelStrict:
		return "strict"
	case LevelStandard:
		return "standard"
	case LevelNone:
		return "none"
	default:
		return "unknown"
	}
}

// ParseLevel parses a string into a Level.
func ParseLevel(s string) (Level, error) {
	switch s {
	case "strict":
		return LevelStrict, nil
	case "standard":
		return LevelStandard, nil
	case "none":
		return LevelNone, nil
	default:
		return LevelNone, fmt.Errorf("unknown sandbox level: %s", s)
	}
}

// ExecOpts contains options for executing a command in a sandbox.
type ExecOpts struct {
	WritablePaths []string          // Additional writable paths
	ReadOnlyPaths []string          // Additional read-only paths (for standard mode)
	NetworkAccess bool              // Enable network access
	EnvVars       map[string]string // Additional environment variables
	WorkDir       string            // Working directory
	Timeout       time.Duration     // Command timeout
}

// Sandbox is the interface for sandbox implementations.
type Sandbox interface {
	// WrapCommand wraps a command for execution inside the sandbox.
	WrapCommand(ctx context.Context, shell, cmd string, opts ExecOpts) *exec.Cmd

	// IsAvailable checks if the sandbox can be used on this system.
	IsAvailable() bool

	// Name returns the sandbox implementation name.
	Name() string

	// Level returns the sandbox level.
	Level() Level
}

// Manager manages sandbox selection based on mode and availability.
type Manager struct {
	sandboxes map[Level]Sandbox
	active    Sandbox
}

// NewManager creates a new sandbox manager.
func NewManager(projectDir string) *Manager {
	m := &Manager{
		sandboxes: make(map[Level]Sandbox),
	}

	// Register sandbox implementations
	m.sandboxes[LevelNone] = NewNoneSandbox()
	m.sandboxes[LevelStandard] = newPlatformSandbox(projectDir, LevelStandard)
	m.sandboxes[LevelStrict] = newPlatformSandbox(projectDir, LevelStrict)

	return m
}

// SetLevel sets the active sandbox level.
func (m *Manager) SetLevel(level Level) error {
	sb, ok := m.sandboxes[level]
	if !ok {
		return fmt.Errorf("no sandbox for level %s", level)
	}
	if !sb.IsAvailable() {
		// Fallback to less restrictive sandbox
		for l := level + 1; l <= LevelNone; l++ {
			if fallback, ok := m.sandboxes[l]; ok && fallback.IsAvailable() {
				m.active = fallback
				return nil
			}
		}
		return fmt.Errorf("sandbox %s not available and no fallback found", level)
	}
	m.active = sb
	return nil
}

// GetActive returns the active sandbox.
func (m *Manager) GetActive() Sandbox {
	if m.active == nil {
		return m.sandboxes[LevelNone]
	}
	return m.active
}

// GetForLevel returns the sandbox for a specific level, checking availability.
func (m *Manager) GetForLevel(level Level) (Sandbox, error) {
	sb, ok := m.sandboxes[level]
	if !ok {
		return nil, fmt.Errorf("no sandbox for level %s", level)
	}
	if !sb.IsAvailable() {
		return nil, fmt.Errorf("sandbox %s not available (bwrap not found)", level)
	}
	return sb, nil
}
