package gateway

import (
	"fmt"
	"strings"
)

// CommandResult holds the output of a slash command.
type CommandResult struct {
	Message string
	Error   bool
}

// handleCommand processes a /xxx slash command.
// Returns nil if the input is not a command (should go to agent).
func (s *Server) handleCommand(sess *GatewaySession, input string) *CommandResult {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	switch cmd {
	case "/clear":
		return s.cmdClear(sess)
	case "/mode":
		return s.cmdMode(sess, parts)
	case "/model":
		return s.cmdModel(parts)
	case "/models":
		return s.cmdModels()
	case "/sessions":
		return s.cmdSessions(parts)
	case "/status":
		return s.cmdStatus(sess)
	case "/compact":
		return s.cmdCompact(sess)
	case "/skill":
		return s.cmdSkill(parts)
	case "/skills":
		return s.cmdSkills()
	case "/help":
		return s.cmdHelp()
	default:
		return &CommandResult{Message: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd), Error: true}
	}
}

func (s *Server) cmdClear(sess *GatewaySession) *CommandResult {
	if sess == nil {
		return &CommandResult{Message: "No active session to clear.", Error: true}
	}
	// The session manager keeps the JSONL file, but we reset the in-memory state.
	// The caller will set agent=nil so the next request builds a fresh agent.
	return &CommandResult{Message: "✅ Conversation cleared"}
}

func (s *Server) cmdMode(sess *GatewaySession, parts []string) *CommandResult {
	if len(parts) > 1 {
		switch parts[1] {
		case "plan", "agent", "yolo":
			if sess != nil {
				sess.Mode = parts[1]
			}
			return &CommandResult{Message: fmt.Sprintf("Mode: %s", strings.ToUpper(parts[1]))}
		default:
			return &CommandResult{Message: "Invalid mode. Use: plan, agent, yolo", Error: true}
		}
	}
	mode := s.cfg.DefaultMode
	if sess != nil && sess.Mode != "" {
		mode = sess.Mode
	}
	return &CommandResult{Message: fmt.Sprintf("Current mode: %s", strings.ToUpper(mode))}
}

func (s *Server) cmdModel(parts []string) *CommandResult {
	if len(parts) > 1 {
		modelID := parts[1]
		newModel := s.provider.GetModel(modelID)
		if newModel == nil {
			return &CommandResult{Message: fmt.Sprintf("Model not found: %s. Use /models to list available models.", modelID), Error: true}
		}
		s.mu.Lock()
		s.model = newModel
		s.mu.Unlock()
		return &CommandResult{Message: fmt.Sprintf("✅ Model switched to: %s (%s)", newModel.Name, newModel.ID)}
	}
	s.mu.RLock()
	m := s.model
	s.mu.RUnlock()
	return &CommandResult{Message: fmt.Sprintf("Current model: %s (%s)", m.Name, m.ID)}
}

func (s *Server) cmdModels() *CommandResult {
	models := s.provider.Models()
	if len(models) == 0 {
		return &CommandResult{Message: "No models available."}
	}
	var sb strings.Builder
	sb.WriteString("Available models:\n")
	s.mu.RLock()
	currentID := s.model.ID
	s.mu.RUnlock()
	for _, m := range models {
		marker := " "
		if m.ID == currentID {
			marker = "*"
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", marker, m.Name, m.ID))
	}
	return &CommandResult{Message: sb.String()}
}

func (s *Server) cmdSessions(parts []string) *CommandResult {
	sub := "ls"
	if len(parts) > 1 {
		sub = strings.ToLower(parts[1])
	}
	switch sub {
	case "ls", "list":
		ids := s.pool.List()
		if len(ids) == 0 {
			return &CommandResult{Message: "No active sessions."}
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Active sessions (%d):\n", len(ids)))
		for _, id := range ids {
			sb.WriteString(fmt.Sprintf("  - %s\n", id))
		}
		return &CommandResult{Message: sb.String()}
	case "clear", "new":
		return &CommandResult{Message: "✅ Use a new x_session_id to start a fresh session."}
	case "del", "delete", "rm":
		if len(parts) < 3 {
			return &CommandResult{Message: "Usage: /sessions del <id>", Error: true}
		}
		id := parts[2]
		if s.pool.Get(id) == nil {
			return &CommandResult{Message: fmt.Sprintf("Session not found: %s", id), Error: true}
		}
		s.pool.Remove(id)
		return &CommandResult{Message: fmt.Sprintf("✅ Session %s deleted.", id)}
	default:
		return &CommandResult{Message: "Usage: /sessions [ls|clear|del <id>]", Error: true}
	}
}

func (s *Server) cmdStatus(sess *GatewaySession) *CommandResult {
	if sess == nil {
		return &CommandResult{Message: "No active session.", Error: true}
	}
	mode := s.cfg.DefaultMode
	if sess.Mode != "" {
		mode = sess.Mode
	}
	s.mu.RLock()
	modelID := s.model.ID
	s.mu.RUnlock()
	msgCount := 0
	if sess.Manager != nil {
		msgCount = len(sess.Manager.GetMessages())
	}
	msg := fmt.Sprintf("Session: %s\nMode: %s\nModel: %s\nMessages: %d\nWorkDir: %s",
		sess.ID, strings.ToUpper(mode), modelID, msgCount, sess.WorkDir)
	return &CommandResult{Message: msg}
}

func (s *Server) cmdCompact(sess *GatewaySession) *CommandResult {
	if sess == nil {
		return &CommandResult{Message: "No active session.", Error: true}
	}

	// Check if there are enough messages to compact
	if sess.Manager != nil && len(sess.Manager.GetMessages()) < 2 {
		return &CommandResult{Message: "Nothing to compact: conversation is too short.", Error: true}
	}

	// Set the force flag so the next agent run triggers compaction
	sess.ForceCompact = true
	return &CommandResult{Message: "✅ Context compaction will be triggered on the next request."}
}

func (s *Server) cmdSkill(parts []string) *CommandResult {
	if s.skillsMgr == nil {
		return &CommandResult{Message: "No skills available.", Error: true}
	}
	if len(parts) < 2 {
		return s.cmdSkills()
	}
	name := parts[1]
	skill := s.skillsMgr.Get(name)
	if skill == nil {
		return &CommandResult{Message: fmt.Sprintf("Skill not found: %s", name), Error: true}
	}
	return &CommandResult{Message: fmt.Sprintf("✅ Skill '%s' activated: %s", name, skill.Description)}
}

func (s *Server) cmdSkills() *CommandResult {
	if s.skillsMgr == nil {
		return &CommandResult{Message: "No skills available."}
	}
	skillList := s.skillsMgr.List()
	if len(skillList) == 0 {
		return &CommandResult{Message: "No skills found."}
	}
	var sb strings.Builder
	sb.WriteString("Available skills:\n")
	for _, sk := range skillList {
		sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", sk.Name, sk.Source, sk.Description))
	}
	return &CommandResult{Message: sb.String()}
}

func (s *Server) cmdHelp() *CommandResult {
	help := `Available commands:
  /clear                  - Clear conversation context
  /mode [plan|agent|yolo] - Show or switch mode
  /model [model_id]       - Show or switch model
  /models                 - List available models
  /sessions               - List active sessions
  /sessions del <id>      - Delete a session
  /compact                - Trigger context compaction
  /status                 - Show session status
  /skill <name>           - Activate a skill
  /skills                 - List available skills
  /help                   - Show this help`
	return &CommandResult{Message: help}
}
