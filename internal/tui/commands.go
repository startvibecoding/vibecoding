package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/session"
)

// handleAgentCommand handles /agent subcommands (multi-agent mode).
func (a *App) handleAgentCommand(parts []string) {
	if !a.multiAgent {
		a.addMessage(errorStyle.Render("Multi-agent mode is not enabled. Use Ctrl+P to toggle."))
		return
	}
	if len(parts) < 2 {
		a.addMessage(statusStyle.Render("Usage: /agent list|switch|destroy"))
		return
	}
	switch parts[1] {
	case "list":
		a.listAgents()
	case "switch":
		if len(parts) < 3 {
			a.addMessage(statusStyle.Render("Usage: /agent switch <id>"))
			return
		}
		a.switchAgent(agentpkg.AgentID(parts[2]))
	case "destroy":
		if len(parts) < 3 {
			a.addMessage(statusStyle.Render("Usage: /agent destroy <id>"))
			return
		}
		a.destroyAgent(agentpkg.AgentID(parts[2]))
	default:
		a.addMessage(errorStyle.Render(fmt.Sprintf("Unknown agent command: %s", parts[1])))
	}
}

func (a *App) listAgents() {
	a.addMessage(statusStyle.Render(fmt.Sprintf("Multi-agent mode: ON (active: %s)", a.activeAgent)))
	if a.agentMgr == nil {
		a.addMessage(statusStyle.Render("  (AgentManager not initialized)"))
		return
	}

	ids := a.agentMgr.List()
	if len(ids) == 0 {
		a.addMessage(statusStyle.Render("  No agents running"))
		return
	}

	for _, id := range ids {
		parentID, hasParent := a.agentMgr.Parent(id)
		children := a.agentMgr.Children(id)
		status := "running"
		if id == a.activeAgent {
			status = "active"
		}

		info := fmt.Sprintf("  %s [%s]", id, status)
		if hasParent {
			info += fmt.Sprintf(" parent=%s", parentID)
		}
		if len(children) > 0 {
			info += fmt.Sprintf(" children=%d", len(children))
		}
		a.addMessage(statusStyle.Render(info))
	}
}

func (a *App) switchAgent(id agentpkg.AgentID) {
	if a.agentMgr == nil {
		a.addMessage(errorStyle.Render("AgentManager not initialized"))
		return
	}

	_, ok := a.agentMgr.Get(id)
	if !ok {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Agent %s not found", id)))
		return
	}

	a.activeAgent = id
	a.addMessage(statusStyle.Render(fmt.Sprintf("Switched to agent: %s", id)))
}

func (a *App) destroyAgent(id agentpkg.AgentID) {
	if id == "main" {
		a.addMessage(errorStyle.Render("Cannot destroy the main agent"))
		return
	}

	if a.agentMgr == nil {
		a.addMessage(errorStyle.Render("AgentManager not initialized"))
		return
	}

	if err := a.agentMgr.Destroy(id); err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Failed to destroy agent %s: %v", id, err)))
		return
	}

	// If we destroyed the active agent, switch to main
	if a.activeAgent == id {
		a.activeAgent = "main"
	}

	a.addMessage(statusStyle.Render(fmt.Sprintf("Agent %s destroyed", id)))
}

// toggleMultiAgent toggles multi-agent mode on/off.
func (a *App) toggleMultiAgent() {
	a.multiAgent = !a.multiAgent
	if a.multiAgent {
		a.addMessage(statusStyle.Render("✅ Multi-agent mode ON (Ctrl+P to toggle)"))
	} else {
		a.addMessage(statusStyle.Render("❌ Multi-agent mode OFF"))
	}
}

// handleCronCommand handles /cron subcommands (multi-agent mode).
func (a *App) handleCronCommand(parts []string) {
	if !a.multiAgent {
		a.addMessage(errorStyle.Render("Cron commands require multi-agent mode. Use Ctrl+P to toggle."))
		return
	}
	if len(parts) < 2 {
		a.addMessage(statusStyle.Render("Usage: /cron add|list|enable|disable|remove|run"))
		return
	}
	switch parts[1] {
	case "add":
		if len(parts) < 3 {
			a.addMessage(statusStyle.Render("Usage: /cron add <description>"))
			return
		}
		desc := strings.Join(parts[2:], " ")
		a.addMessage(statusStyle.Render(fmt.Sprintf("Cron task added: %s", desc)))
		a.addMessage(statusStyle.Render("  (Full cron integration will be available with LLM parsing)"))
	case "list":
		a.addMessage(statusStyle.Render("Cron tasks: (none configured)"))
	case "enable":
		if len(parts) < 3 {
			a.addMessage(statusStyle.Render("Usage: /cron enable <id>"))
			return
		}
		a.addMessage(statusStyle.Render(fmt.Sprintf("Cron task %s enabled", parts[2])))
	case "disable":
		if len(parts) < 3 {
			a.addMessage(statusStyle.Render("Usage: /cron disable <id>"))
			return
		}
		a.addMessage(statusStyle.Render(fmt.Sprintf("Cron task %s disabled", parts[2])))
	case "remove":
		if len(parts) < 3 {
			a.addMessage(statusStyle.Render("Usage: /cron remove <id>"))
			return
		}
		a.addMessage(statusStyle.Render(fmt.Sprintf("Cron task %s removed", parts[2])))
	case "run":
		if len(parts) < 3 {
			a.addMessage(statusStyle.Render("Usage: /cron run <id>"))
			return
		}
		a.addMessage(statusStyle.Render(fmt.Sprintf("Cron task %s triggered", parts[2])))
	default:
		a.addMessage(errorStyle.Render(fmt.Sprintf("Unknown cron command: %s", parts[1])))
	}
}

func (a *App) handleCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	command := parts[0]

	switch command {
	case "/mode":
		if len(parts) > 1 {
			switch parts[1] {
			case "plan", "agent", "yolo":
				a.mode = parts[1]
				// If agent is currently running, abort it so the new mode takes effect immediately
				if a.isThinking && a.agent != nil {
					a.agent.Abort()
					a.agent = nil
					a.agentHistoryLoaded = false
					a.inputQueueMu.Lock()
					a.inputQueue = a.inputQueue[:0]
					a.lastInputTime = time.Time{}
					a.inputQueueMu.Unlock()
					a.isThinking = false
					a.finishRequestTimer()
					a.addMessage(statusStyle.Render("⏹ Aborted (mode change)"))
				} else {
					a.agent = nil
					a.agentHistoryLoaded = false
				}
				a.addMessage(statusStyle.Render(fmt.Sprintf("Mode: %s", strings.ToUpper(a.mode))))
			default:
				a.addMessage(errorStyle.Render("Invalid mode"))
			}
		} else {
			a.addMessage(statusStyle.Render(fmt.Sprintf("Current mode: %s", strings.ToUpper(a.mode))))
			switch a.mode {
			case "plan":
				a.addMessage(statusStyle.Render("  Permissions: READ only (no modifications)"))
			case "agent":
				a.addMessage(statusStyle.Render("  Permissions: READ/WRITE/EDIT auto | BASH requires approval"))
			case "yolo":
				a.addMessage(statusStyle.Render("  Permissions: ALL tools auto-execute"))
			}
		}
	case "/model":
		if len(parts) > 1 {
			// Switch model
			modelID := parts[1]
			newModel := a.provider.GetModel(modelID)
			if newModel == nil {
				a.addMessage(errorStyle.Render(fmt.Sprintf("Model not found: %s", modelID)))
				// List available models
				models := a.provider.Models()
				if len(models) > 0 {
					var sb strings.Builder
					sb.WriteString("Available models:\n")
					for _, m := range models {
						marker := " "
						if m.ID == a.model.ID {
							marker = "*"
						}
						sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", marker, m.Name, m.ID))
					}
					a.addMessage(statusStyle.Render(sb.String()))
				}
				return nil
			}
			a.model = newModel
			// Reset agent so next message uses the new model
			a.agent = nil
			a.agentHistoryLoaded = false
			a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Model switched to: %s (%s)", newModel.Name, newModel.ID)))
		} else {
			// Show current model and available models
			a.addMessage(statusStyle.Render(fmt.Sprintf("Current model: %s (%s)", a.model.Name, a.model.ID)))
			models := a.provider.Models()
			if len(models) > 0 {
				var sb strings.Builder
				sb.WriteString("Available models (use /model <id> to switch):\n")
				for _, m := range models {
					marker := " "
					if m.ID == a.model.ID {
						marker = "*"
					}
					sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", marker, m.Name, m.ID))
				}
				a.addMessage(statusStyle.Render(sb.String()))
			}
		}
	case "/skills":
		a.listSkills()
	case "/skill":
		if len(parts) > 1 {
			a.activateSkill(parts[1])
		} else {
			a.listSkills()
		}
	case "/compact":
		if a.agent == nil {
			a.addMessage(errorStyle.Render("Nothing to compact: no active conversation."))
		} else {
			msgs := a.agent.GetMessages()
			if len(msgs) < 2 {
				a.addMessage(errorStyle.Render("Nothing to compact: conversation is too short."))
			} else {
				a.agent.SetForceCompact()
				if usage := a.agent.GetContextUsage(); usage != nil && usage.Percent != nil {
					a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Context compaction will be triggered on the next message. (current: %d tokens, %.0f%% used)", usage.Tokens, *usage.Percent)))
				} else {
					a.addMessage(statusStyle.Render("✅ Context compaction will be triggered on the next message."))
				}
			}
		}
	case "/clear":
		a.messages = nil
		a.agent = nil
		a.agentHistoryLoaded = false
		a.contextUsage = nil
		a.totalInputTokens = 0
		a.totalCacheRead = 0
		a.totalCacheWrite = 0
		a.pastes = make(map[int]string)
		a.pasteCounter = 0
		a.activeSkills = make(map[string]string)
		a.extraContext = a.baseExtraContext
		a.updateViewportContent()
		a.addMessage(statusStyle.Render("✅ Conversation cleared"))
	case "/quit":
		return tea.Quit
	case "/sessions":
		a.handleSessionsCommand(parts)
	case "/init_mcp":
		a.handleInitMCPCommand(parts)
	case "/mcps":
		a.handleMCPsCommand()
	case "/agent":
		a.handleAgentCommand(parts)
	case "/cron":
		a.handleCronCommand(parts)
	case "/help":
		a.addMessage(statusStyle.Render("Commands:"))
		a.addMessage(statusStyle.Render("  /mode [plan|agent|yolo] - Switch or show mode"))
		a.addMessage(statusStyle.Render("  /model [model_id]       - Switch or show model"))
		a.addMessage(statusStyle.Render("  /skills                 - List available skills"))
		a.addMessage(statusStyle.Render("  /skill <name>           - Activate a skill"))
		a.addMessage(statusStyle.Render("  /clear                  - Clear conversation"))
		a.addMessage(statusStyle.Render("  /compact                - Trigger context compaction"))
		a.addMessage(statusStyle.Render("  /sessions               - List sessions for this project"))
		a.addMessage(statusStyle.Render("  /sessions ls            - List sessions"))
		a.addMessage(statusStyle.Render("  /sessions set <id>      - Switch to session"))
		a.addMessage(statusStyle.Render("  /sessions clear         - Create a new session"))
		a.addMessage(statusStyle.Render("  /sessions del <id>      - Delete a session"))
		a.addMessage(statusStyle.Render("  /init_mcp [target] [template] [--force]"))
		a.addMessage(statusStyle.Render("                         - Init mcp.json (target: project|global, template: basic|full)"))
		a.addMessage(statusStyle.Render("  /mcps                   - List MCP servers (global/project mcp.json)"))
		a.addMessage(statusStyle.Render("  /agent list              - List all agents (multi-agent mode)"))
		a.addMessage(statusStyle.Render("  /agent switch <id>       - Switch active agent"))
		a.addMessage(statusStyle.Render("  /agent destroy <id>      - Destroy a sub-agent"))
		a.addMessage(statusStyle.Render("  /cron add <description>  - Add scheduled task (multi-agent mode)"))
		a.addMessage(statusStyle.Render("  /cron list               - List scheduled tasks"))
		a.addMessage(statusStyle.Render("  /cron enable <id>        - Enable a task"))
		a.addMessage(statusStyle.Render("  /cron disable <id>       - Disable a task"))
		a.addMessage(statusStyle.Render("  /cron remove <id>        - Remove a task"))
		a.addMessage(statusStyle.Render("  /cron run <id>           - Run a task now"))
		a.addMessage(statusStyle.Render("  /quit                   - Exit"))
		a.addMessage(statusStyle.Render("  /help                   - Show this help"))
		a.addMessage(statusStyle.Render(""))
		a.addMessage(statusStyle.Render("Keyboard shortcuts:"))
		a.addMessage(statusStyle.Render("  Tab       - Cycle mode (plan/agent/yolo)"))
		a.addMessage(statusStyle.Render("  Esc       - Abort current operation"))
		a.addMessage(statusStyle.Render("  Ctrl+O    - Open latest tool details"))
		a.addMessage(statusStyle.Render("  PgUp/PgDn - Page tool details when open"))
		a.addMessage(statusStyle.Render("  Mouse wheel - Scroll terminal history"))
	default:
		// Handle /skill:<name> syntax (colon-separated)
		if strings.HasPrefix(command, "/skill:") {
			skillName := strings.TrimPrefix(command, "/skill:")
			if skillName != "" {
				a.activateSkill(skillName)
			} else {
				a.listSkills()
			}
		} else {
			a.addMessage(errorStyle.Render(fmt.Sprintf("Unknown: %s", command)))
		}
	}

	return nil
}

// listSkills displays all available skills.
func (a *App) listSkills() {
	if a.skillsMgr == nil {
		a.addMessage(statusStyle.Render("No skills manager available."))
		return
	}
	skillList := a.skillsMgr.List()
	if len(skillList) == 0 {
		a.addMessage(statusStyle.Render("No skills found."))
		return
	}

	var sb strings.Builder
	sb.WriteString("Available skills:\n")
	for _, s := range skillList {
		marker := " "
		if _, ok := a.activeSkills[s.Name]; ok {
			marker = "*"
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s (%s): %s\n", marker, s.Name, s.Source, s.Description))
	}
	sb.WriteString("\nUse /skill <name> or /skill:<name> to activate a skill.")
	a.addMessage(statusStyle.Render(sb.String()))
}

// activateSkill loads a skill's content into the extra context.
func (a *App) activateSkill(name string) {
	if a.skillsMgr == nil {
		a.addMessage(errorStyle.Render("No skills manager available."))
		return
	}
	skill := a.skillsMgr.Get(name)
	if skill == nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Skill not found: %s", name)))
		return
	}

	// Check if already active
	if _, ok := a.activeSkills[name]; ok {
		a.addMessage(statusStyle.Render(fmt.Sprintf("Skill '%s' is already active.", name)))
		return
	}

	// Add skill content to active skills
	skillCtx := a.skillsMgr.BuildSkillContext(name)
	a.activeSkills[name] = skillCtx

	// Rebuild extraContext from base + all active skills
	a.rebuildExtraContext()

	// Reset agent so next message uses the updated context
	a.agent = nil
	a.agentHistoryLoaded = false

	a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Skill '%s' activated (%s): %s", name, skill.Source, skill.Description)))
}

// rebuildExtraContext rebuilds extraContext from base context + all active skills.
func (a *App) rebuildExtraContext() {
	sb := strings.Builder{}
	sb.WriteString(a.baseExtraContext)
	for _, ctx := range a.activeSkills {
		sb.WriteString(ctx)
	}
	a.extraContext = sb.String()
}

// getSessionDir returns the session directory path.
func (a *App) getSessionDir() string {
	if a.settings != nil {
		return a.settings.GetSessionDir()
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".vibecoding", "sessions")
}

// getCurrentSessionID returns the current session's short ID (first 8 chars).
func (a *App) getCurrentSessionID() string {
	if a.session == nil {
		return ""
	}
	file := a.session.GetFile()
	if file == "" {
		return ""
	}
	base := filepath.Base(file)
	base = strings.TrimSuffix(base, ".jsonl")
	if idx := strings.Index(base, "_"); idx >= 0 {
		return base[idx+1:]
	}
	return ""
}

// handleSessionsCommand handles the /sessions command and its subcommands.
func (a *App) handleSessionsCommand(parts []string) {
	sub := "ls"
	if len(parts) > 1 {
		sub = strings.ToLower(parts[1])
	}

	switch sub {
	case "ls", "list":
		a.sessionsList()
	case "set", "switch", "use":
		if len(parts) < 3 {
			a.addMessage(errorStyle.Render("Usage: /sessions set <id>"))
			return
		}
		a.sessionsSet(parts[2])
	case "clear", "new":
		a.sessionsClear()
	case "del", "delete", "rm":
		if len(parts) < 3 {
			a.addMessage(errorStyle.Render("Usage: /sessions del <id>"))
			return
		}
		a.sessionsDel(parts[2])
	default:
		a.addMessage(errorStyle.Render(fmt.Sprintf("Unknown subcommand: %s. Use ls, set, clear, del.", sub)))
	}
}

// sessionsList lists all sessions for the current project directory.
func (a *App) sessionsList() {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	sessionDir := a.getSessionDir()
	details, err := session.ListForDirDetailed(cwd, sessionDir)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error listing sessions: %v", err)))
		return
	}

	if len(details) == 0 {
		a.addMessage(statusStyle.Render("No sessions found for this project."))
		return
	}

	currentID := a.getCurrentSessionID()

	var sb strings.Builder
	sb.WriteString("Sessions for this project:\n\n")
	for _, d := range details {
		marker := " "
		if d.ID == currentID {
			marker = "*"
		}
		age := formatAge(d.ModTime)
		preview := ""
		if d.Preview != "" {
			preview = " - " + d.Preview
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s  %d msgs  %s%s\n",
			marker, d.ID, d.MessageCount, age, preview))
	}
	sb.WriteString("\nUse /sessions set <id> to switch. * = current session.")
	a.addMessage(statusStyle.Render(sb.String()))
}

// sessionsSet switches to a different session by ID prefix.
func (a *App) sessionsSet(id string) {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	// Don't switch to the same session
	if id == a.getCurrentSessionID() {
		a.addMessage(statusStyle.Render("Already on this session."))
		return
	}

	sessionDir := a.getSessionDir()
	details, err := session.ListForDirDetailed(cwd, sessionDir)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		return
	}

	// Find matching session by ID prefix
	var match *session.SessionDetail
	for i, d := range details {
		if strings.HasPrefix(d.ID, id) {
			if match != nil {
				a.addMessage(errorStyle.Render(fmt.Sprintf("Ambiguous ID '%s'. Be more specific.", id)))
				return
			}
			match = &details[i]
		}
	}

	if match == nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("No session found matching '%s'.", id)))
		return
	}

	// Open the session
	newSess, err := session.Open(match.Path)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error opening session: %v", err)))
		return
	}

	// Switch session
	a.session = newSess
	a.historyLoaded = false
	a.agentHistoryLoaded = false

	// Reset agent and UI state
	a.agent = nil
	a.messages = nil
	a.toolResults = nil
	a.contextUsage = nil
	a.totalInputTokens = 0
	a.totalCacheRead = 0
	a.totalCacheWrite = 0
	a.assistantRaw = make(map[int]string)
	a.assistantRendered = make(map[int]string)
	a.assistantDirty = make(map[int]bool)
	a.printedMessageIdx = make(map[int]bool)
	a.currentAssistantIdx = -1
	a.currentThinkIdx = -1

	// Load history messages from the new session
	a.LoadHistoryMessages()
	a.updateViewportContent()

	a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Switched to session %s (%d msgs)",
		match.ID, match.MessageCount)))
}

func (a *App) handleInitMCPCommand(parts []string) {
	target := "project"
	template := "full"
	force := false

	for _, p := range parts[1:] {
		switch strings.ToLower(p) {
		case "project", "global":
			target = strings.ToLower(p)
		case "basic", "full":
			template = strings.ToLower(p)
		case "--force":
			force = true
		default:
			a.addMessage(errorStyle.Render("Usage: /init_mcp [project|global] [basic|full] [--force]"))
			return
		}
	}

	path := config.ProjectMCPPath()
	if target == "global" {
		path = config.GlobalMCPPath()
	}

	if !force {
		if _, err := os.Stat(path); err == nil {
			a.addMessage(statusStyle.Render(fmt.Sprintf("MCP config already exists: %s (use --force to overwrite)", path)))
			return
		}
	}

	var cfg *config.MCPConfig
	if template == "basic" {
		cfg = config.DefaultMCPConfig()
	} else {
		cfg = config.FullMCPConfigTemplate()
	}

	if err := config.SaveMCPConfig(path, cfg); err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Init MCP config failed: %v", err)))
		return
	}
	a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Created MCP config: %s", path)))
	a.addMessage(statusStyle.Render(fmt.Sprintf("Template: %s | Target: %s", template, target)))
}

func (a *App) handleMCPsCommand() {
	type sourceInfo struct {
		label string
		path  string
	}
	sources := []sourceInfo{
		{label: "Global", path: config.GlobalMCPPath()},
		{label: "Project", path: config.ProjectMCPPath()},
	}

	var sb strings.Builder
	sb.WriteString("MCP servers:\n")
	foundAny := false

	for _, src := range sources {
		sb.WriteString(fmt.Sprintf("\n%s (%s):\n", src.label, src.path))
		cfg, err := config.LoadMCPConfig(src.path)
		if err != nil {
			if os.IsNotExist(err) {
				sb.WriteString("  (not configured)\n")
				continue
			}
			sb.WriteString(fmt.Sprintf("  (invalid: %v)\n", err))
			continue
		}
		config.NormalizeMCPConfig(cfg)
		if len(cfg.MCPServers) == 0 {
			sb.WriteString("  (empty)\n")
			continue
		}
		for _, srv := range cfg.MCPServers {
			foundAny = true
			target := srv.Command
			if target == "" {
				target = srv.URL
			}
			if target == "" {
				target = "-"
			}
			sb.WriteString(fmt.Sprintf("  - %s [%s] %s\n", srv.Name, srv.Type, target))
		}
	}

	if !foundAny {
		sb.WriteString("\nUse /init_mcp to create project mcp.json.")
	}
	a.addMessage(statusStyle.Render(sb.String()))
}

// sessionsClear creates a new session, starting fresh.
func (a *App) sessionsClear() {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	sessionDir := a.getSessionDir()
	newSess := session.New(cwd, sessionDir)
	if err := newSess.Init(); err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error creating session: %v", err)))
		return
	}

	a.session = newSess
	a.historyLoaded = false
	a.agentHistoryLoaded = false

	// Reset agent and UI state
	a.agent = nil
	a.messages = nil
	a.toolResults = nil
	a.contextUsage = nil
	a.totalInputTokens = 0
	a.totalCacheRead = 0
	a.totalCacheWrite = 0
	a.assistantRaw = make(map[int]string)
	a.assistantRendered = make(map[int]string)
	a.assistantDirty = make(map[int]bool)
	a.printedMessageIdx = make(map[int]bool)
	a.currentAssistantIdx = -1
	a.currentThinkIdx = -1
	a.updateViewportContent()

	a.addMessage(statusStyle.Render("✅ New session created."))
}

// sessionsDel deletes a session by ID prefix.
func (a *App) sessionsDel(id string) {
	cwd := ""
	if a.session != nil && a.session.GetHeader() != nil {
		cwd = a.session.GetHeader().Cwd
	}
	if cwd == "" {
		if w, err := os.Getwd(); err == nil {
			cwd = w
		}
	}

	// Don't delete the current session
	if id == a.getCurrentSessionID() {
		a.addMessage(errorStyle.Render("Cannot delete the current session. Switch to another session first, or use /sessions clear to start fresh."))
		return
	}

	sessionDir := a.getSessionDir()
	details, err := session.ListForDirDetailed(cwd, sessionDir)
	if err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		return
	}

	// Find matching session by ID prefix
	var match *session.SessionDetail
	for i, d := range details {
		if strings.HasPrefix(d.ID, id) {
			if match != nil {
				a.addMessage(errorStyle.Render(fmt.Sprintf("Ambiguous ID '%s'. Be more specific.", id)))
				return
			}
			match = &details[i]
		}
	}

	if match == nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("No session found matching '%s'.", id)))
		return
	}

	if err := session.DeleteSession(match.Path); err != nil {
		a.addMessage(errorStyle.Render(fmt.Sprintf("Error deleting session: %v", err)))
		return
	}

	a.addMessage(statusStyle.Render(fmt.Sprintf("✅ Deleted session %s.", match.ID)))
}

// formatAge returns a human-readable age string for a time.
func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2006-01-02")
	}
}
