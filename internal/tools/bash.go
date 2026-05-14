package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/startvibecoding/vibecoding/internal/platform"
	"github.com/startvibecoding/vibecoding/internal/sandbox"
)

// BashTool executes shell commands.
type BashTool struct {
	registry   *Registry
	jobManager *JobManager
}

// NewBashTool creates a new bash tool.
func NewBashTool(r *Registry) *BashTool {
	return &BashTool{
		registry:   r,
		jobManager: NewJobManager(),
	}
}

// GetJobManager returns the job manager for background processes.
func (t *BashTool) GetJobManager() *JobManager {
	return t.jobManager
}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	if platform.IsWindows() {
		return "Execute a shell command (PowerShell/cmd). Use this to run commands, scripts, build commands, etc. The command runs in the current working directory. Set timeout for long-running commands (default 120s, max 600s). For long-running services (like servers), use async=true to run in background."
	}
	return "Execute a bash command. Use this to run shell commands, scripts, build commands, etc. The command runs in the current working directory. Set timeout for long-running commands (default 120s, max 600s). For long-running services (like servers), use async=true to run in background."
}

func (t *BashTool) PromptSnippet() string {
	return "Execute bash commands (ls, grep, find, etc.)"
}

func (t *BashTool) PromptGuidelines() []string {
	return []string{
		"Prefer grep/find/ls tools over bash for file exploration (faster, respects .gitignore)",
		"For long-running services (servers, watchers, dev servers), use async=true to run in background",
	}
}

func (t *BashTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			},
			"timeout": {
				"type": "integer",
				"description": "Timeout in seconds (default 120, max 600)"
			},
			"async": {
				"type": "boolean",
				"description": "Run command in background (for long-running services like servers). Returns immediately with a job ID. Use 'jobs' tool to check status."
			}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Check for async mode
	async, _ := params["async"].(bool)

	timeout := 120 * time.Second
	if v, ok := params["timeout"].(float64); ok && v > 0 {
		if v > 600 {
			v = 600
		}
		timeout = time.Duration(v) * time.Second
	}

	// For async commands, use a background context (no timeout unless specified)
	var cmdCtx context.Context
	var cancel context.CancelFunc
	if async {
		cmdCtx, cancel = context.WithCancel(context.Background())
	} else {
		cmdCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Get platform-specific shell
	shell := platform.DefaultShell()
	if s := os.Getenv("SHELL"); s != "" {
		shell = s
	}

	workDir := t.registry.GetWorkDir()

	var cmd *exec.Cmd
	sb := t.registry.GetSandbox()
	if sb != nil && sb.IsAvailable() {
		opts := sandbox.ExecOpts{
			WorkDir: workDir,
			Timeout: timeout,
			EnvVars: make(map[string]string),
		}
		cmd = sb.WrapCommand(cmdCtx, shell, command, opts)
	} else {
		// Use platform-specific shell arguments
		args := platform.ShellArgs(shell, command)
		cmd = exec.CommandContext(cmdCtx, shell, args...)
		cmd.Dir = workDir
		// Detach child process group so background children don't block the shell.
		setSysProcAttr(cmd)
		// If the shell exits while a background child still holds stdio,
		// don't wait forever – give it 100ms then force-close.
		cmd.WaitDelay = 100 * time.Millisecond
	}

	// Async mode: start in background and return immediately
	if async {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("failed to start background command: %w", err)
		}

		job := t.jobManager.AddJob(cmd, command, cancel)

		// Wait in background and mark done when finished
		go func() {
			err := cmd.Wait()
			// Ignore WaitDelay error – background children may still hold stdio.
			if errors.Is(err, exec.ErrWaitDelay) {
				err = nil
			}
			job.MarkDone(stdout.Bytes(), stderr.Bytes(), err)
		}()

		return fmt.Sprintf("Started background job [%d] (PID: %d): %s\nUse 'jobs' tool to check status or 'kill' to stop.", job.ID, job.PID, command), nil
	}

	// Synchronous mode
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "STDERR:\n" + stderr.String()
	}

	// Build result with command info
	var result strings.Builder
	result.WriteString(fmt.Sprintf("$ %s\n", command))
	result.WriteString(fmt.Sprintf("(in %s)\n\n", workDir))

	if output == "" {
		result.WriteString("(no output)")
	} else {
		result.WriteString(output)
	}

	// Truncate large outputs
	const maxOutput = 50000
	resultStr := result.String()
	if len(resultStr) > maxOutput {
		truncated := len(resultStr) - maxOutput
		resultStr = resultStr[:maxOutput] + fmt.Sprintf("\n... (truncated %d bytes)", truncated)
	}

	if err != nil {
		// Ignore WaitDelay error – the shell already exited; we just didn't
		// drain all stdio in time (common with background children).
		if errors.Is(err, exec.ErrWaitDelay) {
			return resultStr, nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("%s\nExit code: %d", resultStr, exitErr.ExitCode()), nil
		}
		return "", fmt.Errorf("command failed: %w\n%s", err, resultStr)
	}

	return resultStr, nil
}

// SetTool is an interface for tools that need sandbox updates.
type SetTool interface {
	SetSandbox(sb sandbox.Sandbox)
}

// FileTool is a base for file-related tools.
type FileTool struct {
	registry *Registry
}

func (t *FileTool) resolvePath(path string) string {
	// Expand home directory
	path = platform.ExpandHome(path)

	// Normalize path separators
	path = platform.NormalizePath(path)

	// Make relative paths absolute
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.registry.GetWorkDir(), path)
	}

	return path
}
