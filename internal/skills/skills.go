package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a loaded skill.
type Skill struct {
	Name        string // skill name (directory name)
	Path        string // absolute path to SKILL.md
	Description string // first line or heading description
	Content     string // full SKILL.md content
	Source      string // "global" or "project"
}

// Manager manages skill discovery and loading.
type Manager struct {
	globalDir  string // ~/.vibecoding/skills
	projectDir string // .skills/ in project root
	skills     map[string]*Skill
}

// NewManager creates a new skills manager.
func NewManager(globalDir, projectDir string) *Manager {
	return &Manager{
		globalDir:  globalDir,
		projectDir: projectDir,
		skills:     make(map[string]*Skill),
	}
}

// Load discovers and loads all skills from global and project directories.
// Project-local skills override global skills with the same name.
func (m *Manager) Load() error {
	// Load global skills first (lower priority)
	if m.globalDir != "" {
		if err := m.loadFromDir(m.globalDir, "global"); err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "Warning: could not load global skills: %v\n", err)
		}
	}

	// Load project skills (higher priority, overrides global)
	if m.projectDir != "" {
		if err := m.loadFromDir(m.projectDir, "project"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load project skills: %v\n", err)
		}
	}

	return nil
}

// loadFromDir loads all skills from a directory.
// Each skill is a subdirectory containing a SKILL.md file.
func (m *Manager) loadFromDir(dir string, source string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // directory doesn't exist, that's ok
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read skills directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(dir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")

		data, err := os.ReadFile(skillFile)
		if err != nil {
			// Try lowercase
			skillFile = filepath.Join(skillDir, "skill.md")
			data, err = os.ReadFile(skillFile)
			if err != nil {
				continue // no SKILL.md, skip
			}
		}

		skill := &Skill{
			Name:    entry.Name(),
			Path:    skillFile,
			Content: string(data),
			Source:  source,
		}

		// Extract description from first heading or first non-empty line
		skill.Description = extractDescription(string(data))

		m.skills[entry.Name()] = skill
	}

	return nil
}

// Get returns a skill by name.
func (m *Manager) Get(name string) *Skill {
	return m.skills[name]
}

// List returns all loaded skills sorted by name.
func (m *Manager) List() []*Skill {
	var result []*Skill
	for _, s := range m.skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListBySource returns skills filtered by source.
func (m *Manager) ListBySource(source string) []*Skill {
	var result []*Skill
	for _, s := range m.skills {
		if s.Source == source {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Names returns a list of all skill names.
func (m *Manager) Names() []string {
	var names []string
	for name := range m.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BuildSkillContext returns the content of a skill for injection into the system prompt.
func (m *Manager) BuildSkillContext(name string) string {
	skill := m.Get(name)
	if skill == nil {
		return ""
	}
	return fmt.Sprintf("\n## Active Skill: %s\n\n%s\n", skill.Name, skill.Content)
}

// BuildAllSkillsContext returns a summary of all available skills for the system prompt.
func (m *Manager) BuildAllSkillsContext() string {
	skills := m.List()
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Available Skills\n\n")
	sb.WriteString("Use `/skill:<name>` to load a skill. Available skills:\n\n")

	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", s.Name, s.Source, s.Description))
	}

	sb.WriteString("\n")
	return sb.String()
}

// extractDescription extracts a short description from skill content.
func extractDescription(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Return first heading
		if strings.HasPrefix(line, "#") {
			return strings.TrimLeft(line, "# ")
		}
		// Or first non-empty line
		return line
	}
	return "(no description)"
}

// CreateProjectSkillsDir creates the .skills directory in the project root.
func CreateProjectSkillsDir(projectDir string) error {
	dir := filepath.Join(projectDir, ".skills")
	return os.MkdirAll(dir, 0755)
}
