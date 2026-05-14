package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/startvibecoding/vibecoding/internal/provider"
)

// EntryType identifies the type of a session entry.
type EntryType string

const (
	EntrySession        EntryType = "session"
	EntryMessage        EntryType = "message"
	EntryModelChange    EntryType = "model_change"
	EntryThinkingChange EntryType = "thinking_level_change"
	EntryCompaction     EntryType = "compaction"
	EntryBranchSummary  EntryType = "branch_summary"
	EntryCustom         EntryType = "custom"
	EntryCustomMessage  EntryType = "custom_message"
	EntryLabel          EntryType = "label"
	EntrySessionInfo    EntryType = "session_info"
)

// EntryBase contains common fields for all session entries.
type EntryBase struct {
	Type      EntryType `json:"type"`
	ID        string    `json:"id"`
	ParentID  *string   `json:"parentId"`
	Timestamp time.Time `json:"timestamp"`
}

// Header is the first line of a session file.
type Header struct {
	Type          EntryType `json:"type"`
	Version       int       `json:"version"`
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Cwd           string    `json:"cwd"`
	ParentSession string    `json:"parentSession,omitempty"`
}

// MessageEntry contains a conversation message.
type MessageEntry struct {
	EntryBase
	Message provider.Message `json:"message"`
}

// ModelChangeEntry records a model switch.
type ModelChangeEntry struct {
	EntryBase
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// ThinkingLevelChangeEntry records a thinking level change.
type ThinkingLevelChangeEntry struct {
	EntryBase
	ThinkingLevel string `json:"thinkingLevel"`
}

// CompactionEntry records a context compaction.
type CompactionEntry struct {
	EntryBase
	Summary        string `json:"summary"`
	FirstKeptEntry string `json:"firstKeptEntryId"`
	TokensBefore   int    `json:"tokensBefore"`
}

// BranchSummaryEntry records a branch switch summary.
type BranchSummaryEntry struct {
	EntryBase
	Summary string `json:"summary"`
	FromID  string `json:"fromId"`
}

// LabelEntry records a user-defined label on an entry.
type LabelEntry struct {
	EntryBase
	TargetID string  `json:"targetId"`
	Label    *string `json:"label,omitempty"`
}

// SessionInfoEntry stores session metadata.
type SessionInfoEntry struct {
	EntryBase
	Name string `json:"name"`
}

// GenerateID generates a random 8-character hex ID.
func GenerateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
