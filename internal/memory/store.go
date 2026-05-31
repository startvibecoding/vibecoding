// Package memory implements persistent memory storage for Hermes mode.
// Memory is stored as a human-readable Markdown file (memory.md).
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/config"
)

// Store manages reading and writing of memory.md files.
type Store struct {
	// explicitPath overrides auto-discovery when set via config.
	explicitPath string
	// workDir is the project working directory, used as fallback for default write path.
	workDir string
}

// NewStore creates a memory store.
// If explicitPath is non-empty, it overrides the default discovery logic.
// workDir is used as fallback directory for creating new memory files.
func NewStore(explicitPath, workDir string) *Store {
	return &Store{explicitPath: explicitPath, workDir: workDir}
}

// defaultTemplate is the initial content for a new memory.md file.
const defaultTemplate = `# Agent Memory

## User Profile

## Working Memory

## Lessons Learned
`

// Resolve finds the memory.md file to use.
// Priority: explicit path → .vibe/memory.md → <GLOBAL_DIR>/memory.md
// Returns (path, source, error). source is "explicit", "project", "global", or "".
func (s *Store) Resolve() (path string, source string, err error) {
	// 1. Explicit path from config
	if s.explicitPath != "" {
		if _, err := os.Stat(s.explicitPath); err == nil {
			return s.explicitPath, "explicit", nil
		}
		// Explicit path configured but doesn't exist yet — will create here on write
		return s.explicitPath, "explicit", nil
	}

	// 2. Project-level: .vibe/memory.md
	projectPath := filepath.Join(".vibe", "memory.md")
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath, "project", nil
	}

	// 3. Global: <GLOBAL_DIR>/memory.md
	globalPath := filepath.Join(config.ConfigDir(), "memory.md")
	if _, err := os.Stat(globalPath); err == nil {
		return globalPath, "global", nil
	}

	// None exists — return empty (will be created on first write)
	return "", "", nil
}

// Read returns the full content of memory.md.
func (s *Store) Read() (content string, path string, source string, err error) {
	path, source, err = s.Resolve()
	if err != nil {
		return "", "", "", err
	}
	if path == "" {
		return "", "", "", nil // no memory file exists
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", path, source, nil
		}
		return "", "", "", fmt.Errorf("read memory file: %w", err)
	}

	return string(data), path, source, nil
}

// ReadSection returns the content of a specific ## section.
func (s *Store) ReadSection(section string) (string, error) {
	content, _, _, err := s.Read()
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}

	return extractSection(content, section), nil
}

// Add appends a line to a specific section.
func (s *Store) Add(section, entry string) error {
	content, path, _, err := s.Read()
	if err != nil {
		return err
	}

	if path == "" {
		// Create new file
		path = s.defaultWritePath()
		content = defaultTemplate
	}

	updated := addToSection(content, section, entry)
	return s.writeFile(path, updated)
}

// Update replaces old text with new text in a section.
func (s *Store) Update(section, oldText, newText string) error {
	content, path, _, err := s.Read()
	if err != nil {
		return err
	}
	if path == "" || content == "" {
		return fmt.Errorf("no memory file to update")
	}

	sectionContent := extractSection(content, section)
	if sectionContent == "" {
		return fmt.Errorf("section '%s' not found", section)
	}

	if !strings.Contains(sectionContent, oldText) {
		return fmt.Errorf("text not found in section '%s'", section)
	}

	updated := strings.Replace(content, oldText, newText, 1)
	return s.writeFile(path, updated)
}

// Delete removes a line from a section.
func (s *Store) Delete(section, entry string) error {
	content, path, _, err := s.Read()
	if err != nil {
		return err
	}
	if path == "" || content == "" {
		return fmt.Errorf("no memory file to delete from")
	}

	// Remove the line containing the entry
	lines := strings.Split(content, "\n")
	var result []string
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Match "- entry" or "entry" (with or without bullet)
		cleanEntry := strings.TrimPrefix(strings.TrimSpace(entry), "- ")
		cleanLine := strings.TrimPrefix(trimmed, "- ")
		if cleanLine == cleanEntry && !found {
			found = true
			continue // skip this line
		}
		result = append(result, line)
	}

	if !found {
		return fmt.Errorf("entry not found in memory")
	}

	return s.writeFile(path, strings.Join(result, "\n"))
}

// WriteAll overwrites the entire memory.md content.
func (s *Store) WriteAll(content string) error {
	path, _, _, err := s.Read()
	if err != nil {
		return err
	}
	if path == "" {
		path = s.defaultWritePath()
	}
	return s.writeFile(path, content)
}

// --- Helpers ---

// defaultWritePath determines where to create a new memory.md.
// Default: project-level (.vibe/memory.md). Only uses global if explicitly configured.
func (s *Store) defaultWritePath() string {
	if s.explicitPath != "" {
		return s.explicitPath
	}
	// Default to project-level: workDir/.vibe/memory.md
	if s.workDir != "" {
		return filepath.Join(s.workDir, ".vibe", "memory.md")
	}
	// Fallback: cwd/.vibe/memory.md
	return filepath.Join(".vibe", "memory.md")
}

// writeFile writes content to path, creating parent dirs as needed.
func (s *Store) writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0600)
}

// extractSection extracts content under a ## heading.
func extractSection(content, section string) string {
	header := "## " + section
	idx := strings.Index(content, header)
	if idx < 0 {
		return ""
	}

	// Find the start of content after the header line
	afterHeader := content[idx+len(header):]
	nlIdx := strings.Index(afterHeader, "\n")
	if nlIdx < 0 {
		return ""
	}
	afterHeader = afterHeader[nlIdx+1:]

	// Find the next ## heading or end of file
	nextSection := strings.Index(afterHeader, "\n## ")
	if nextSection >= 0 {
		afterHeader = afterHeader[:nextSection]
	}

	return strings.TrimSpace(afterHeader)
}

// addToSection appends an entry to a section. Creates the section if missing.
func addToSection(content, section, entry string) string {
	header := "## " + section

	// Ensure entry has bullet prefix
	trimmedEntry := strings.TrimSpace(entry)
	if !strings.HasPrefix(trimmedEntry, "- ") {
		trimmedEntry = "- " + trimmedEntry
	}

	idx := strings.Index(content, header)
	if idx < 0 {
		// Section doesn't exist — append at end
		return strings.TrimRight(content, "\n") + "\n\n" + header + "\n\n" + trimmedEntry + "\n"
	}

	// Find the end of this section (next ## or EOF)
	afterHeader := content[idx+len(header):]
	nlIdx := strings.Index(afterHeader, "\n")
	if nlIdx < 0 {
		return content + "\n\n" + trimmedEntry + "\n"
	}

	sectionStart := idx + len(header) + nlIdx + 1
	rest := content[sectionStart:]

	nextSection := strings.Index(rest, "\n## ")
	if nextSection >= 0 {
		// Insert before next section
		insertPoint := sectionStart + nextSection
		return content[:insertPoint] + trimmedEntry + "\n" + content[insertPoint:]
	}

	// Append at end
	return strings.TrimRight(content, "\n") + "\n" + trimmedEntry + "\n"
}
