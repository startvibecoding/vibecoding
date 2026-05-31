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
	"github.com/startvibecoding/vibecoding/internal/vendored"
)

// limitedBuffer wraps bytes.Buffer with a max size limit.
type limitedBuffer struct {
	buf     bytes.Buffer
	maxSize int
	dropped int
}

func newLimitedBuffer(maxSize int) *limitedBuffer {
	return &limitedBuffer{maxSize: maxSize}
}

func (lb *limitedBuffer) Write(p []byte) (n int, err error) {
	if lb.buf.Len()+len(p) > lb.maxSize {
		keep := lb.maxSize - lb.buf.Len()
		if keep > 0 {
			lb.buf.Write(p[:keep])
		}
		lb.dropped += len(p) - keep
		return len(p), nil
	}
	return lb.buf.Write(p)
}

func (lb *limitedBuffer) Bytes() []byte {
	if lb.dropped > 0 {
		trail := fmt.Sprintf("\n... (truncated %d bytes)", lb.dropped)
		lb.buf.WriteString(trail)
		lb.dropped = 0
	}
	return lb.buf.Bytes()
}

// BashTool executes shell commands.
type BashTool struct {
	registry   *Registry
	jobManager *JobManager
}

// NewBashTool creates a new bash tool with a new JobManager.
func NewBashTool(r *Registry) *BashTool {
	return &BashTool{
		registry:   r,
		jobManager: NewJobManager(),
	}
}

// NewBashToolWithJM creates a new bash tool with an existing JobManager.
func NewBashToolWithJM(r *Registry, jm *JobManager) *BashTool {
	return &BashTool{
		registry:   r,
		jobManager: jm,
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
	return "Execute shell commands when dedicated tools are insufficient"
}

func (t *BashTool) PromptGuidelines() []string {
	return []string{
		"Prefer read/ls/grep/find tools over bash for file inspection and exploration",
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

func (t *BashTool) Execute(ctx context.Context, params map[string]any) (ToolResult, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return ToolResult{}, fmt.Errorf("command is required")
	}

	// Check for async mode
	async, _ := params["async"].(bool)

	// Auto-detect async if command ends with &
	command = strings.TrimSpace(command)
	if strings.HasSuffix(command, "&") && !async {
		async = true
		command = strings.TrimSpace(strings.TrimSuffix(command, "&"))
	}

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
		if isValidShell(s) {
			shell = s
		}
	}

	workDir := t.registry.GetWorkDir()

	// 构建环境变量，将 ~/.vibecoding/bin 加入 PATH
	rgPath := vendored.RgPath()
	vendoredBin := ""
	if rgPath != "" {
		vendoredBin = filepath.Dir(rgPath)
	}
	env := os.Environ()
	if vendoredBin != "" && vendoredBin != "." {
		for i, e := range env {
			// Windows 环境变量不区分大小写，PATH/Path/path 都可以
			if len(e) >= 5 && strings.EqualFold(e[:5], "PATH=") {
				env[i] = "PATH=" + vendoredBin + string(os.PathListSeparator) + e[5:]
				break
			}
		}
	}

	var cmd *exec.Cmd
	sb := t.registry.GetSandbox()
	if sb != nil && sb.IsAvailable() {
		envPath := os.Getenv("PATH")
		if vendoredBin != "" && vendoredBin != "." {
			envPath = vendoredBin + string(os.PathListSeparator) + envPath
		}
		opts := sandbox.ExecOpts{
			WorkDir: workDir,
			Timeout: timeout,
			EnvVars: map[string]string{
				"PATH": envPath,
			},
		}
		cmd = sb.WrapCommand(cmdCtx, shell, command, opts)
	} else {
		// Use platform-specific shell arguments
		args := platform.ShellArgs(shell, command)
		cmd = exec.CommandContext(cmdCtx, shell, args...)
		cmd.Dir = workDir
		cmd.Env = env
		// Detach child process group so background children don't block the shell.
		setSysProcAttr(cmd)
		// If the shell exits while a background child still holds stdio,
		// don't wait forever – give it 100ms then force-close.
		cmd.WaitDelay = 100 * time.Millisecond
	}

	// Async mode: start in background and return immediately
	if async {
		const maxJobOutput = 1000000 // 1 MB limit per stream
		stdout := newLimitedBuffer(maxJobOutput)
		stderr := newLimitedBuffer(maxJobOutput)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			cancel()
			return ToolResult{}, fmt.Errorf("failed to start background command: %w", err)
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

		return NewTextToolResult(fmt.Sprintf("Started background job [%d] (PID: %d): %s\nUse 'jobs' tool to check status or 'kill' to stop.", job.ID, job.PID, command)), nil
	}

	// Synchronous mode
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stdoutStr := strings.TrimRight(stdout.String(), "\n")
	stderrStr := strings.TrimRight(stderr.String(), "\n")
	if stdoutStr == "" {
		stdoutStr = "(no output)"
	}
	if stderrStr == "" {
		stderrStr = "(no output)"
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	var result strings.Builder
	result.WriteString("[command]\n")
	result.WriteString(command)
	result.WriteString("\n[cwd]\n")
	result.WriteString(workDir)
	result.WriteString("\n[stdout]\n")
	result.WriteString(stdoutStr)
	result.WriteString("\n[stderr]\n")
	result.WriteString(stderrStr)
	result.WriteString("\n[exit_code]\n")
	result.WriteString(fmt.Sprintf("%d", exitCode))

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
			return NewTextToolResult(resultStr), nil
		}
		if _, ok := err.(*exec.ExitError); ok {
			return NewTextToolResult(resultStr), nil
		}
		return ToolResult{}, fmt.Errorf("command failed: %w\n%s", err, resultStr)
	}

	return NewTextToolResult(resultStr), nil
}

// SetTool is an interface for tools that need sandbox updates.
type SetTool interface {
	SetSandbox(sb sandbox.Sandbox)
}

// validShellNames is the allowlist of known shell binaries.
var validShellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true, "ksh": true,
}

// isValidShell checks whether the given path is a known shell binary.
func isValidShell(path string) bool {
	name := filepath.Base(path)
	if !validShellNames[name] {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
