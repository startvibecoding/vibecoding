package hermes

import (
	"fmt"
	"strings"
)

// Security provides user whitelist validation and smart approval logic for Hermes mode.
type Security struct {
	cfg *HermesConfig
}

// NewSecurity creates a security manager.
func NewSecurity(cfg *HermesConfig) *Security {
	return &Security{cfg: cfg}
}

// CheckUserAllowed returns nil if the user is allowed on the given platform.
// Returns an error with reason if blocked.
func (s *Security) CheckUserAllowed(platform, userID string) error {
	var allowedUsers []string

	switch platform {
	case "wechat":
		allowedUsers = s.cfg.Wechat.AllowedUsers
	case "feishu":
		allowedUsers = s.cfg.Feishu.AllowedUsers
	case "ws":
		// WebSocket clients are authenticated via token, no per-user whitelist
		return nil
	default:
		return nil
	}

	// Empty whitelist = allow all (but warn in logs)
	if len(allowedUsers) == 0 {
		return nil
	}

	for _, allowed := range allowedUsers {
		if allowed == userID {
			return nil
		}
	}

	return fmt.Errorf("user %s not in allowed_users for platform %s", userID, platform)
}

// CheckWorkDirAllowed returns nil if the working directory is allowed.
func (s *Security) CheckWorkDirAllowed(workDir string) error {
	allowed := s.cfg.Security.AllowedWorkDirs
	if len(allowed) == 0 {
		// No restriction
		return nil
	}

	for _, dir := range allowed {
		if workDir == dir || strings.HasPrefix(workDir, dir+"/") {
			return nil
		}
	}

	return fmt.Errorf("working directory %s not in allowed_work_dirs", workDir)
}

// CommandRiskLevel classifies the risk level of a bash command.
// Returns "low", "medium", or "high".
func CommandRiskLevel(command string) string {
	command = strings.TrimSpace(command)

	// High risk: destructive or system-level commands
	highRiskPrefixes := []string{
		"rm -rf", "rm -r",
		"mkfs", "dd ",
		"chmod 777", "chmod -R",
		"chown -R",
		"sudo ", "su ",
		"shutdown", "reboot", "halt",
		"kill -9", "killall",
		"> /dev/", "curl | sh", "curl | bash", "wget | sh",
		"eval ", "exec ",
	}
	for _, prefix := range highRiskPrefixes {
		if strings.HasPrefix(command, prefix) || strings.Contains(command, " "+prefix) {
			return "high"
		}
	}

	// High risk: pipe to shell
	if strings.Contains(command, "| sh") || strings.Contains(command, "| bash") {
		return "high"
	}

	// Medium risk: file modifications, network, package management
	mediumRiskPrefixes := []string{
		"mv ", "cp -r",
		"git push", "git reset --hard", "git clean",
		"npm publish", "go install",
		"apt ", "yum ", "brew ", "pip install",
		"docker ", "kubectl ",
		"curl ", "wget ",
		"ssh ", "scp ",
	}
	for _, prefix := range mediumRiskPrefixes {
		if strings.HasPrefix(command, prefix) {
			return "medium"
		}
	}

	// Low risk: read-only and common dev commands
	lowRiskPrefixes := []string{
		"go ", "make ", "npm ", "yarn ", "node ",
		"python ", "pip ",
		"git status", "git log", "git diff", "git branch",
		"ls", "cat ", "head ", "tail ", "wc ",
		"echo ", "printf ",
		"grep ", "find ", "which ", "type ",
		"cd ", "pwd", "env", "printenv",
	}
	for _, prefix := range lowRiskPrefixes {
		if strings.HasPrefix(command, prefix) {
			return "low"
		}
	}

	return "medium" // default: unknown commands are medium risk
}

// ApprovalDecision represents the result of an approval check.
type ApprovalDecision struct {
	Approved bool
	Reason   string
	RiskLevel string
}

// FormatApprovalNotification formats a notification for medium/high risk tool calls.
func FormatApprovalNotification(toolName string, args map[string]any, riskLevel string, approved bool) string {
	var icon, status string
	if approved {
		icon = "⚠️"
		status = "auto-approved"
	} else {
		icon = "🚫"
		status = "blocked"
	}

	var detail string
	if toolName == "bash" {
		if cmd, ok := args["command"]; ok {
			cmdStr := fmt.Sprintf("%v", cmd)
			if len(cmdStr) > 80 {
				cmdStr = cmdStr[:80] + "..."
			}
			detail = cmdStr
		}
	} else {
		if path, ok := args["path"]; ok {
			detail = fmt.Sprintf("%v", path)
		}
	}

	if detail != "" {
		return fmt.Sprintf("%s [%s] %s %s (%s risk)", icon, toolName, detail, status, riskLevel)
	}
	return fmt.Sprintf("%s [%s] %s (%s risk)", icon, toolName, status, riskLevel)
}

// ShouldAutoApprove returns true if the tool call can be auto-approved in Hermes mode.
// In Hermes mode, bots run unattended so we need stricter auto-approval rules.
func (s *Security) ShouldAutoApprove(toolName string, args map[string]any, mode string) bool {
	if !s.cfg.Security.SmartApprovals {
		// Smart approvals disabled — fall back to mode-based behavior
		return mode == "yolo"
	}

	switch toolName {
	case "read", "ls", "grep", "find", "skill_ref", "memory", "plan", "jobs":
		// Read-only tools: always auto-approve
		return true

	case "write", "edit":
		// File modifications: auto-approve in agent/yolo mode
		return mode == "agent" || mode == "yolo"

	case "bash":
		command, _ := args["command"].(string)
		risk := CommandRiskLevel(command)
		switch mode {
		case "yolo":
			return risk != "high" // yolo still blocks high-risk
		case "agent":
			return risk == "low" // agent only auto-approves low-risk
		default:
			return false
		}

	case "kill":
		return mode == "agent" || mode == "yolo"

	default:
		return mode == "yolo"
	}
}
