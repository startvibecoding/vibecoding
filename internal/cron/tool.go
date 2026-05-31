package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/startvibecoding/vibecoding/internal/tools"
)

// CronTool provides cron job management for the agent.
type CronTool struct {
	store     CronStore
	scheduler *Scheduler
}

// NewCronTool creates a new cron management tool.
func NewCronTool(store CronStore, scheduler *Scheduler) *CronTool {
	return &CronTool{store: store, scheduler: scheduler}
}

func (t *CronTool) Name() string {
	return "cron"
}

func (t *CronTool) Description() string {
	return "Manage scheduled tasks (cron jobs). Create one-time or periodic background tasks that run via sub-agents."
}

func (t *CronTool) PromptSnippet() string {
	return "Manage scheduled background tasks (one-time or periodic)"
}

func (t *CronTool) PromptGuidelines() []string {
	return []string{
		"The `cron` tool manages scheduled background tasks that run via sub-agents.",
		"Use `cron(action=\"list\")` to see existing tasks.",
		"Use `cron(action=\"create\", name=\"...\", prompt=\"...\", schedule=\"@daily\")` for periodic tasks.",
		"Use `cron(action=\"create\", name=\"...\", prompt=\"...\", oneshot=true)` for one-time tasks.",
		"Schedule formats: `@daily`, `@weekly`, `@monthly`, `@hourly`, `@every 30m`, `@every 2h`, or empty for one-shot.",
		"Use `cron(action=\"run\", id=\"...\")` to trigger a task immediately.",
	}
}

func (t *CronTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"description": "Action: list, create, enable, disable, remove, run",
				"enum": ["list", "create", "enable", "disable", "remove", "run"]
			},
			"id": {
				"type": "string",
				"description": "Job ID (required for enable, disable, remove, run)"
			},
			"name": {
				"type": "string",
				"description": "Short task name (required for create)"
			},
			"prompt": {
				"type": "string",
				"description": "Task prompt for the sub-agent (required for create)"
			},
			"schedule": {
				"type": "string",
				"description": "Schedule: @daily, @weekly, @monthly, @hourly, @every 30m, @every 2h, or empty/omit for one-shot"
			},
			"oneshot": {
				"type": "boolean",
				"description": "If true, run once then auto-disable (default: false). Same as omitting schedule."
			},
			"mode": {
				"type": "string",
				"description": "Agent mode for the task: agent, yolo (default: yolo)",
				"enum": ["agent", "yolo"]
			}
		},
		"required": ["action"]
	}`)
}

func (t *CronTool) Execute(ctx context.Context, params map[string]any) (tools.ToolResult, error) {
	action, _ := params["action"].(string)

	switch action {
	case "list":
		return t.executeList()
	case "create":
		name, _ := params["name"].(string)
		prompt, _ := params["prompt"].(string)
		schedule, _ := params["schedule"].(string)
		oneShot, _ := params["oneshot"].(bool)
		mode, _ := params["mode"].(string)
		return t.executeCreate(name, prompt, schedule, oneShot, mode)
	case "enable":
		id, _ := params["id"].(string)
		return t.executeSetEnabled(id, true)
	case "disable":
		id, _ := params["id"].(string)
		return t.executeSetEnabled(id, false)
	case "remove":
		id, _ := params["id"].(string)
		return t.executeRemove(id)
	case "run":
		id, _ := params["id"].(string)
		return t.executeRun(id)
	default:
		return tools.ToolResult{}, fmt.Errorf("unknown action: %s (use: list, create, enable, disable, remove, run)", action)
	}
}

func (t *CronTool) executeList() (tools.ToolResult, error) {
	jobs, err := t.store.List()
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("list cron jobs: %w", err)
	}
	if len(jobs) == 0 {
		return tools.NewTextToolResult("No cron jobs configured."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cron jobs (%d):\n\n", len(jobs)))
	for _, j := range jobs {
		status := "✅ enabled"
		if !j.Enabled {
			status = "⏸ disabled"
		}
		if j.LastStatus == "failed" {
			status = "❌ failed"
		}
		if j.LastStatus == "running" {
			status = "🔄 running"
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s\n  Status: %s | Mode: %s | Schedule: %s | Runs: %d\n  Prompt: %s\n",
			j.ID, j.Name, status, j.Mode, scheduleStr(j.Schedule, j.OneShot), j.RunCount, truncateStr(j.Prompt, 80)))
		if !j.LastRun.IsZero() {
			sb.WriteString(fmt.Sprintf("  Last run: %s\n", j.LastRun.Format(time.RFC3339)))
		}
		if j.LastError != "" {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", j.LastError))
		}
		sb.WriteString("\n")
	}
	return tools.NewTextToolResult(sb.String()), nil
}

func (t *CronTool) executeCreate(name, prompt, schedule string, oneShot bool, mode string) (tools.ToolResult, error) {
	if name == "" {
		return tools.ToolResult{}, fmt.Errorf("name is required for create")
	}
	if prompt == "" {
		return tools.ToolResult{}, fmt.Errorf("prompt is required for create")
	}
	if mode == "" {
		mode = "yolo"
	}

	// Determine if one-shot: explicit oneshot=true or empty schedule (and not a periodic schedule)
	isOneShot := oneShot
	if !isOneShot && schedule == "" {
		isOneShot = true // Default: no schedule = one-shot
	}

	// Compute NextRun for periodic tasks
	var nextRun time.Time
	if !isOneShot && schedule != "" {
		next, _, err := ParseSchedule(schedule, time.Now())
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("invalid schedule: %w", err)
		}
		nextRun = next
	}

	job, err := t.store.Create(CronJob{
		Name:     name,
		Prompt:   prompt,
		Schedule: schedule,
		OneShot:  isOneShot,
		Enabled:  true,
		Mode:     mode,
		NextRun:  nextRun,
	})
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("create cron job: %w", err)
	}

	kind := "periodic"
	if isOneShot {
		kind = "one-shot"
	}
	nextInfo := ""
	if !nextRun.IsZero() {
		nextInfo = fmt.Sprintf("\n  Next run: %s", nextRun.Format(time.RFC3339))
	}
	return tools.NewTextToolResult(fmt.Sprintf("✅ Cron job created (%s):\n  ID: %s\n  Name: %s\n  Schedule: %s\n  Mode: %s%s\n  Prompt: %s",
		kind, job.ID, job.Name, scheduleStr(job.Schedule, isOneShot), job.Mode, nextInfo, truncateStr(job.Prompt, 100))), nil
}

func scheduleStr(schedule string, oneShot bool) string {
	if oneShot {
		return "(one-shot)"
	}
	if schedule == "" {
		return "(one-shot)"
	}
	return schedule
}

func (t *CronTool) executeSetEnabled(id string, enabled bool) (tools.ToolResult, error) {
	if id == "" {
		return tools.ToolResult{}, fmt.Errorf("id is required")
	}
	job, err := t.store.Get(id)
	if err != nil {
		return tools.ToolResult{}, err
	}
	job.Enabled = enabled
	if err := t.store.Update(*job); err != nil {
		return tools.ToolResult{}, fmt.Errorf("update cron job: %w", err)
	}
	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	return tools.NewTextToolResult(fmt.Sprintf("✅ Cron job %s %s: %s", job.ID, action, job.Name)), nil
}

func (t *CronTool) executeRemove(id string) (tools.ToolResult, error) {
	if id == "" {
		return tools.ToolResult{}, fmt.Errorf("id is required")
	}
	job, err := t.store.Get(id)
	if err != nil {
		return tools.ToolResult{}, err
	}
	name := job.Name
	if err := t.store.Delete(id); err != nil {
		return tools.ToolResult{}, fmt.Errorf("delete cron job: %w", err)
	}
	return tools.NewTextToolResult(fmt.Sprintf("🗑 Cron job removed: %s (%s)", id, name)), nil
}

func (t *CronTool) executeRun(id string) (tools.ToolResult, error) {
	if id == "" {
		return tools.ToolResult{}, fmt.Errorf("id is required")
	}
	job, err := t.store.Get(id)
	if err != nil {
		return tools.ToolResult{}, err
	}
	// Trigger by resetting LastRun so scheduler picks it up on next tick
	job.LastRun = time.Time{}
	if err := t.store.Update(*job); err != nil {
		return tools.ToolResult{}, fmt.Errorf("update cron job: %w", err)
	}
	return tools.NewTextToolResult(fmt.Sprintf("▶ Cron job %s triggered: %s (will run on next scheduler tick)", job.ID, job.Name)), nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
