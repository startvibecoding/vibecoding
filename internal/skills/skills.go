package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillReference represents a reference file within a skill.
type SkillReference struct {
	Path      string // relative path (e.g. "references/audio.md")
	FullPath  string // absolute path
	Label     string // display label (e.g. "音频")
	AutoLoad  bool   // true if marked [已加载], false if [待按需加载]
	Loaded    bool   // whether this reference has been loaded
	Content   string // loaded content
}

// Skill represents a loaded skill.
type Skill struct {
	Name        string // skill name (directory name)
	Path        string // absolute path to SKILL.md
	Dir         string // skill directory
	Description string // first line or heading description
	Content     string // full SKILL.md content
	Source      string // "global" or "project"
	References  []*SkillReference // parsed references
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
			Dir:     skillDir,
			Content: string(data),
			Source:  source,
		}

		// Extract description from first heading or first non-empty line
		skill.Description = extractDescription(string(data))

		// Parse references from SKILL.md
		skill.References = parseReferences(string(data), skillDir)

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
// It includes the SKILL.md content plus all auto-loaded references.
func (m *Manager) BuildSkillContext(name string) string {
	skill := m.Get(name)
	if skill == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## Active Skill: %s\n\n%s\n", skill.Name, skill.Content))

	// Auto-load references marked as [已加载]
	for _, ref := range skill.References {
		if ref.AutoLoad {
			if content := loadReferenceContent(ref); content != "" {
				ref.Loaded = true
				ref.Content = content
				sb.WriteString(fmt.Sprintf("\n### Reference: %s\n\n%s\n", ref.Label, content))
			}
		}
	}

	// Add reference loading instructions if there are on-demand references
	hasOnDemand := false
	var onDemandRefs []string
	for _, ref := range skill.References {
		if !ref.AutoLoad {
			if !hasOnDemand {
				sb.WriteString("\n### On-Demand References\n\n")
				sb.WriteString("The following references are available but not loaded. " +
					"Use the `skill_ref` tool to load them when needed:\n\n")
				hasOnDemand = true
			}
			onDemandRefs = append(onDemandRefs, fmt.Sprintf("- `%s` (%s)", ref.Path, ref.Label))
		}
	}
	if hasOnDemand {
		sb.WriteString(strings.Join(onDemandRefs, "\n"))
		sb.WriteString("\n")
	}

	return sb.String()
}

// LoadReference loads a specific reference file by path for a skill.
// Returns the content and true if successful.
func (m *Manager) LoadReference(skillName, refPath string) (string, bool) {
	skill := m.Get(skillName)
	if skill == nil {
		return "", false
	}

	// Normalize the path
	refPath = filepath.Clean(refPath)

	for _, ref := range skill.References {
		if ref.Path == refPath || filepath.Clean(ref.Path) == refPath {
			if ref.Loaded {
				return ref.Content, true
			}
			if content := loadReferenceContent(ref); content != "" {
				ref.Loaded = true
				ref.Content = content
				return content, true
			}
			return "", false
		}
	}

	// Try loading directly from the skill directory
	fullPath := filepath.Join(skill.Dir, refPath)
	fullPath = filepath.Clean(fullPath)

	// Validate: path must not escape the skill directory
	skillDir := filepath.Clean(skill.Dir)
	if !strings.HasPrefix(fullPath, skillDir) {
		return "", false
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", false
	}

	content := string(data)
	// Add to references list for tracking
	skill.References = append(skill.References, &SkillReference{
		Path:     refPath,
		FullPath: fullPath,
		Label:    refPath,
		AutoLoad: false,
		Loaded:   true,
		Content:  content,
	})
	return content, true
}

// ListReferences returns the reference files for a skill with their load status.
func (m *Manager) ListReferences(skillName string) []*SkillReference {
	skill := m.Get(skillName)
	if skill == nil {
		return nil
	}
	return skill.References
}

// loadReferenceContent reads the content of a reference file.
func loadReferenceContent(ref *SkillReference) string {
	data, err := os.ReadFile(ref.FullPath)
	if err != nil {
		return ""
	}
	return string(data)
}

// parseReferences parses reference links from SKILL.md content.
// It looks for patterns like:
//   - Section headers: "### N. Label (references/file.md) [已加载]" or "[待按需加载]"
//   - Markdown links: "- [Label](references/file.md)"
func parseReferences(content, skillDir string) []*SkillReference {
	var refs []*SkillReference
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Pattern 1: Section headers with (path) [status]
		// e.g.: "### 1. 基础 (references/base.md) [已加载]"
		if strings.HasPrefix(line, "###") {
			pathStart := strings.Index(line, "(")
			pathEnd := strings.Index(line, ")")
			if pathStart > 0 && pathEnd > pathStart {
				path := line[pathStart+1 : pathEnd]
				if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt") {
					fullPath := filepath.Join(skillDir, path)
					if !seen[path] {
						seen[path] = true
						label := strings.TrimPrefix(line, "#")
						label = strings.TrimSpace(label)
						// Remove the path part
						if idx := strings.Index(label, "("); idx > 0 {
							label = strings.TrimSpace(label[:idx])
							// Remove leading number and dot
							label = strings.TrimLeft(label, "0123456789. ")
						}
						autoLoad := strings.Contains(line, "[已加载]")
						refs = append(refs, &SkillReference{
							Path:     path,
							FullPath: fullPath,
							Label:    label,
							AutoLoad: autoLoad,
						})
					}
				}
			}
		}

		// Pattern 2: Markdown links at bottom
		// e.g.: "- [基础](references/base.md)"
		if strings.HasPrefix(line, "-") && strings.Contains(line, "[") && strings.Contains(line, "](") {
			linkStart := strings.Index(line, "](")
			linkEnd := strings.Index(line[linkStart+2:], ")")
			if linkStart > 0 && linkEnd > 0 {
				path := line[linkStart+2 : linkStart+2+linkEnd]
				if (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt")) && !seen[path] {
					seen[path] = true
					fullPath := filepath.Join(skillDir, path)
					// Extract label
					labelStart := strings.Index(line, "[")
					label := ""
					if labelStart >= 0 && labelStart < linkStart {
						label = line[labelStart+1 : linkStart]
					}
					// Check if this ref was already parsed with autoLoad info from headers
					// If not, default to on-demand
					refs = append(refs, &SkillReference{
						Path:     path,
						FullPath: fullPath,
						Label:    label,
						AutoLoad: false,
					})
				}
			}
		}
	}

	return refs
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
