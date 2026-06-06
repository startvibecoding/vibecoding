package session

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/startvibecoding/vibecoding/internal/provider"
)

const CurrentVersion = 3

// Manager manages a single session's state and persistence.
type Manager struct {
	mu         sync.RWMutex
	file       string
	header     *Header
	entries    []interface{} // all entry types
	leafID     *string
	cwd        string
	sessionDir string
}

// encodePath encodes a directory path for use in a session directory name.
// Uses base64 URL encoding to avoid collisions from different characters mapping
// to the same replacement (e.g. "/" and ":" both mapped to "-").
func encodePath(p string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(p))
}

// New creates a new session manager for a new session.
func New(cwd, sessionDir string) *Manager {
	if sessionDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		sessionDir = filepath.Join(home, ".vibecoding", "sessions")
	}

	return &Manager{
		cwd:        cwd,
		sessionDir: sessionDir,
	}
}

// Open opens an existing session file.
func Open(path string) (*Manager, error) {
	m := &Manager{file: path}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

// ContinueRecent continues the most recent session for a directory, or creates new.
func ContinueRecent(cwd, sessionDir string) (*Manager, error) {
	if sessionDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		sessionDir = filepath.Join(home, ".vibecoding", "sessions")
	}

	sessions, err := ListForDir(cwd, sessionDir)
	if err != nil {
		return nil, err
	}

	if len(sessions) > 0 {
		// Most recent
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].ModTime.After(sessions[j].ModTime)
		})
		return Open(sessions[0].Path)
	}

	m := New(cwd, sessionDir)
	if err := m.Init(); err != nil {
		return nil, err
	}
	return m, nil
}

// OpenByPathOrID opens a session using either an explicit file path or a
// session ID for the supplied working directory.
func OpenByPathOrID(cwd, sessionDir, value string) (*Manager, error) {
	if value == "" {
		return nil, fmt.Errorf("session value is empty")
	}
	if strings.HasSuffix(value, ".jsonl") || strings.ContainsRune(value, os.PathSeparator) {
		return Open(value)
	}
	return OpenByID(cwd, sessionDir, value)
}

// SessionInfo contains metadata about a session file.
type SessionInfo struct {
	Path    string
	ModTime time.Time
	Name    string
}

// ListForDir lists session files for a given working directory.
func ListForDir(cwd, sessionDir string) ([]SessionInfo, error) {
	if sessionDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		sessionDir = filepath.Join(home, ".vibecoding", "sessions")
	}

	// Session files are stored in sessionDir/--<encoded-path>--/
	encoded := encodePath(cwd)
	dir := filepath.Join(sessionDir, "--"+encoded+"--")

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, SessionInfo{
			Path:    filepath.Join(dir, e.Name()),
			ModTime: info.ModTime(),
		})
	}

	return sessions, nil
}

// Init initializes a new session with an auto-generated session ID.
// Must be called before appending entries.
func (m *Manager) Init() error {
	return m.InitWithID("")
}

// InitWithID initializes a new session using the provided session ID.
// If id is empty, a new random ID is generated.
func (m *Manager) InitWithID(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.initWithIDLocked(id)
}

func (m *Manager) initWithIDLocked(id string) error {
	now := time.Now()
	if id == "" {
		id = GenerateID()
	}
	m.header = &Header{
		Type:      EntrySession,
		Version:   CurrentVersion,
		ID:        id,
		Timestamp: now,
		Cwd:       m.cwd,
	}
	m.entries = nil
	m.leafID = nil

	// Create session file
	encoded := encodePath(m.cwd)
	dir := filepath.Join(m.sessionDir, "--"+encoded+"--")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	m.file = filepath.Join(dir, fmt.Sprintf("%s_%s.jsonl", now.Format("20060102-150405"), m.header.ID[:8]))

	// Write header
	return m.writeEntry(m.header)
}

func (m *Manager) ensureInitializedLocked() error {
	if m.file != "" {
		return nil
	}
	return m.initWithIDLocked("")
}

// OpenByID opens the most recent session file for cwd whose session header ID matches sessionID.
func OpenByID(cwd, sessionDir, sessionID string) (*Manager, error) {
	sessions, err := ListForDir(cwd, sessionDir)
	if err != nil {
		return nil, err
	}
	var match *Manager
	for _, s := range sessions {
		mgr, err := Open(s.Path)
		if err != nil {
			continue
		}
		hdr := mgr.GetHeader()
		if hdr == nil {
			continue
		}
		if hdr.ID == sessionID {
			return mgr, nil
		}
		if strings.HasPrefix(hdr.ID, sessionID) || strings.HasPrefix(sessionFileID(s.Path), sessionID) {
			if match != nil {
				return nil, fmt.Errorf("session ID %s is ambiguous for cwd %s", sessionID, cwd)
			}
			match = mgr
		}
	}
	if match != nil {
		return match, nil
	}
	return nil, fmt.Errorf("session %s not found for cwd %s", sessionID, cwd)
}

func sessionFileID(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".jsonl")
	if idx := strings.Index(base, "_"); idx >= 0 {
		return base[idx+1:]
	}
	return ""
}

// AppendMessage adds a message entry.
func (m *Manager) AppendMessage(msg provider.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureInitializedLocked(); err != nil {
		return "", err
	}

	id := GenerateID()
	entry := MessageEntry{
		EntryBase: EntryBase{
			Type:      EntryMessage,
			ID:        id,
			ParentID:  m.leafID,
			Timestamp: time.Now(),
		},
		Message: msg,
	}

	if err := m.writeEntry(entry); err != nil {
		return "", err
	}

	m.entries = append(m.entries, entry)
	m.leafID = &id
	return id, nil
}

// AppendModelChange records a model change.
func (m *Manager) AppendModelChange(providerName, modelID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureInitializedLocked(); err != nil {
		return "", err
	}

	id := GenerateID()
	entry := ModelChangeEntry{
		EntryBase: EntryBase{
			Type:      EntryModelChange,
			ID:        id,
			ParentID:  m.leafID,
			Timestamp: time.Now(),
		},
		Provider: providerName,
		ModelID:  modelID,
	}

	if err := m.writeEntry(entry); err != nil {
		return "", err
	}

	m.entries = append(m.entries, entry)
	m.leafID = &id
	return id, nil
}

// AppendThinkingLevelChange records a thinking level change.
func (m *Manager) AppendThinkingLevelChange(level string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureInitializedLocked(); err != nil {
		return "", err
	}

	id := GenerateID()
	entry := ThinkingLevelChangeEntry{
		EntryBase: EntryBase{
			Type:      EntryThinkingChange,
			ID:        id,
			ParentID:  m.leafID,
			Timestamp: time.Now(),
		},
		ThinkingLevel: level,
	}

	if err := m.writeEntry(entry); err != nil {
		return "", err
	}

	m.entries = append(m.entries, entry)
	m.leafID = &id
	return id, nil
}

// AppendCompaction records a context compaction.
func (m *Manager) AppendCompaction(summary, firstKeptEntryID string, tokensBefore int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureInitializedLocked(); err != nil {
		return "", err
	}

	id := GenerateID()
	entry := CompactionEntry{
		EntryBase: EntryBase{
			Type:      EntryCompaction,
			ID:        id,
			ParentID:  m.leafID,
			Timestamp: time.Now(),
		},
		Summary:        summary,
		FirstKeptEntry: firstKeptEntryID,
		TokensBefore:   tokensBefore,
	}

	if err := m.writeEntry(entry); err != nil {
		return "", err
	}

	m.entries = append(m.entries, entry)
	m.leafID = &id
	return id, nil
}

// AppendSessionInfo records session metadata (e.g. display name).
func (m *Manager) AppendSessionInfo(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureInitializedLocked(); err != nil {
		return "", err
	}

	id := GenerateID()
	entry := SessionInfoEntry{
		EntryBase: EntryBase{
			Type:      EntrySessionInfo,
			ID:        id,
			ParentID:  m.leafID,
			Timestamp: time.Now(),
		},
		Name: name,
	}

	if err := m.writeEntry(entry); err != nil {
		return "", err
	}

	m.entries = append(m.entries, entry)
	m.leafID = &id
	return id, nil
}

// GetMessages extracts all messages from the current branch.
func (m *Manager) GetMessages() []provider.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var messages []provider.Message
	for _, e := range m.entries {
		if msg, ok := e.(MessageEntry); ok {
			messages = append(messages, msg.Message)
		}
	}
	return messages
}

// GetLeafID returns the current leaf entry ID.
func (m *Manager) GetLeafID() *string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.leafID
}

// GetFile returns the session file path.
func (m *Manager) GetFile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.file
}

// GetHeader returns the session header.
func (m *Manager) GetHeader() *Header {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.header
}

// load reads a session file into memory.
func (m *Manager) load() error {
	f, err := os.Open(m.file)
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var corruptLines int
	for scanner.Scan() {
		line := scanner.Bytes()

		// Parse just the type field to determine entry type
		var typeField struct {
			Type EntryType `json:"type"`
		}
		if err := json.Unmarshal(line, &typeField); err != nil {
			corruptLines++
			continue
		}

		switch typeField.Type {
		case EntrySession:
			var h Header
			if err := json.Unmarshal(line, &h); err != nil {
				return fmt.Errorf("parse header: %w", err)
			}
			m.header = &h
			m.cwd = h.Cwd

		case EntryMessage:
			var e MessageEntry
			if err := json.Unmarshal(line, &e); err != nil {
				corruptLines++
				continue
			}
			m.entries = append(m.entries, e)
			m.leafID = &e.ID

		case EntryModelChange:
			var e ModelChangeEntry
			if err := json.Unmarshal(line, &e); err != nil {
				corruptLines++
				continue
			}
			m.entries = append(m.entries, e)
			m.leafID = &e.ID

		case EntryThinkingChange:
			var e ThinkingLevelChangeEntry
			if err := json.Unmarshal(line, &e); err != nil {
				corruptLines++
				continue
			}
			m.entries = append(m.entries, e)
			m.leafID = &e.ID

		case EntryCompaction:
			var e CompactionEntry
			if err := json.Unmarshal(line, &e); err != nil {
				corruptLines++
				continue
			}
			m.entries = append(m.entries, e)
			m.leafID = &e.ID

		case EntrySessionInfo:
			var e SessionInfoEntry
			if err := json.Unmarshal(line, &e); err != nil {
				corruptLines++
				continue
			}
			m.entries = append(m.entries, e)
			m.leafID = &e.ID

		case EntryBranchSummary:
			var e BranchSummaryEntry
			if err := json.Unmarshal(line, &e); err != nil {
				corruptLines++
				continue
			}
			m.entries = append(m.entries, e)
			m.leafID = &e.ID
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if corruptLines > 0 {
		log.Printf("[session] warning: skipped %d corrupt line(s) in %s", corruptLines, m.file)
	}
	return nil
}

// writeEntry writes a single entry to the session file.
// DeleteSession deletes a session file if it is under sessionDir.
func DeleteSession(path string, sessionDir string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("resolve session path: %w", err)
	}
	cleanSessionDir, err := filepath.Abs(filepath.Clean(sessionDir))
	if err != nil {
		return fmt.Errorf("resolve session dir: %w", err)
	}
	rel, err := filepath.Rel(cleanSessionDir, cleanPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("session path %s is outside session directory %s", path, sessionDir)
	}
	if filepath.Ext(cleanPath) != ".jsonl" {
		return fmt.Errorf("session path %s is not a .jsonl file", path)
	}
	return os.Remove(path)
}

// SessionDetail contains detailed metadata about a session for display.
type SessionDetail struct {
	SessionInfo
	ID           string
	MessageCount int
	Preview      string // first user message (truncated)
}

// ListForDirDetailed lists sessions with details (ID, message count, preview).
func ListForDirDetailed(cwd, sessionDir string) ([]SessionDetail, error) {
	sessions, err := ListForDir(cwd, sessionDir)
	if err != nil {
		return nil, err
	}

	var details []SessionDetail
	for _, s := range sessions {
		d := SessionDetail{SessionInfo: s}
		// Extract ID from filename: YYYYMMDD-HHMMSS_ID.jsonl
		d.ID = sessionFileID(s.Path)

		// Read session to count messages and get preview
		mgr := &Manager{file: s.Path}
		if err := mgr.load(); err == nil {
			for _, e := range mgr.entries {
				if msg, ok := e.(MessageEntry); ok {
					d.MessageCount++
					if d.Preview == "" && msg.Message.Role == "user" {
						text := msg.Message.Content
						if text == "" {
							for _, b := range msg.Message.Contents {
								if b.Type == "text" && b.Text != "" {
									text = b.Text
									break
								}
							}
						}
						if len(text) > 60 {
							text = text[:60] + "..."
						}
						d.Preview = text
					}
				}
			}
		}

		details = append(details, d)
	}

	// Sort by modification time (newest first)
	sort.Slice(details, func(i, j int) bool {
		return details[i].ModTime.After(details[j].ModTime)
	})

	return details, nil
}

func (m *Manager) writeEntry(entry interface{}) error {
	f, err := os.OpenFile(m.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write session entry: %w", err)
	}
	// fsync to guarantee durability on crash/power loss.
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync session file: %w", err)
	}
	return nil
}
