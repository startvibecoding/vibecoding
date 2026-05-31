package a2a

import (
	"encoding/json"
	"net/http"
)

// AgentCard represents the A2A Agent Card (/.well-known/agent.json).
type AgentCard struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	URL          string       `json:"url"`
	Version      string       `json:"version"`
	Capabilities Capabilities `json:"capabilities"`
	Skills       []Skill      `json:"skills"`
}

// Capabilities describes what the agent can do.
type Capabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
}

// Skill describes a specific capability.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// DefaultAgentCard returns the default Agent Card for VibeCoding.
func DefaultAgentCard(version, serverURL string) *AgentCard {
	return &AgentCard{
		Name:        "VibeCoding",
		Description: "AI coding assistant with file editing, terminal, and search capabilities",
		URL:         serverURL + "/a2a",
		Version:     version,
		Capabilities: Capabilities{
			Streaming:         true,
			PushNotifications: false,
		},
		Skills: []Skill{
			{
				ID:          "code-edit",
				Name:        "Code Editing",
				Description: "Read, write, and edit code files with precise text replacement",
			},
			{
				ID:          "terminal",
				Name:        "Terminal Execution",
				Description: "Execute shell commands, run tests, build projects",
			},
			{
				ID:          "code-search",
				Name:        "Code Search",
				Description: "Search codebases with ripgrep and fd",
			},
		},
	}
}

// HandleAgentCard serves the Agent Card at /.well-known/agent.json.
func HandleAgentCard(card *AgentCard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	}
}
