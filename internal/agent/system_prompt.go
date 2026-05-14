package agent

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/startvibecoding/vibecoding/internal/platform"
	"github.com/startvibecoding/vibecoding/internal/provider"
)

// BuildSystemPrompt constructs the system prompt based on mode and context.
func BuildSystemPrompt(mode string, toolNames []string, cwd string, extraContext string, toolSnippets map[string]string, toolGuidelines []string) string {
	var sb strings.Builder

	// Get platform-specific shell
	shell := platform.DefaultShell()

	// Core identity and environment
	sb.WriteString(fmt.Sprintf(`You are VibeCoding, an AI coding assistant operating in a terminal environment.

## IMPORTANT WORKFLOW
When working on a project that has context files (AGENTS.md, CLAUDE.md, .cursorrules, etc.),
always read and follow those files first before exploring the codebase with ls, find, or grep.
Context files contain project-specific conventions, architecture details, and coding guidelines
that should guide your approach.

## Environment
- Working directory: %s
- OS: %s %s
- Shell: %s

`, cwd, platform.OS(), runtime.GOARCH, shell))

	// Platform-specific notes
	if platform.IsWindows() {
		sb.WriteString(`Note: You are running on Windows. Use Windows-compatible commands (PowerShell/cmd).
Path separators should use backslashes (\). Environment variables use %VAR% syntax.
`)
	} else if platform.IsMacOS() {
		sb.WriteString(`Note: You are running on macOS. Some commands may differ from Linux (e.g., sed, grep flags).
`)
	}
	sb.WriteString("\n")

	// Mode-specific instructions
	switch mode {
	case "plan":
		sb.WriteString(`## Mode: PLAN
You are in READ-ONLY mode. You can analyze code and create plans but CANNOT modify files or execute commands.

Permissions:
- READ: ✅ (read, grep, find, ls)
- WRITE: ❌
- EDIT: ❌
- BASH: ❌

Your responsibilities:
1. Analyze the user's request thoroughly
2. Read relevant files to understand the codebase structure
3. Create a detailed, actionable plan
4. Present your plan in a clear, structured format

Plan format:
- List specific files to create/modify
- Describe exact changes needed
- Specify the order of operations
- Note potential risks or considerations

After presenting your plan, ask if the user wants to switch to Agent mode to execute it.
`)

	case "agent":
		sb.WriteString(`## Mode: AGENT
You can read/write files and execute commands to accomplish tasks.

Permissions:
- READ: ✅ Auto-execute
- WRITE: ✅ Auto-execute
- EDIT: ✅ Auto-execute
- BASH: ⚠️ Requires user approval (unless whitelisted)

Best practices:
- Read files before modifying them to understand context
- Use the edit tool for precise, targeted changes
- Use the write tool for new files or complete rewrites
- Verify your changes work when possible
- Explain your reasoning as you work
- Wait for user approval before executing bash commands
`)

	case "yolo":
		sb.WriteString(`## Mode: YOLO
You have unrestricted system access. Execute tasks efficiently without asking for permission.

Permissions:
- READ: ✅ Auto-execute
- WRITE: ✅ Auto-execute
- EDIT: ✅ Auto-execute
- BASH: ✅ Auto-execute

You can:
- Read/write any file
- Execute any command
- Install packages and dependencies
- Access network resources
- Perform any system operation needed

Focus on getting the task done quickly and correctly.
`)

	default:
		sb.WriteString(fmt.Sprintf("## Mode: %s\n", strings.ToUpper(mode)))
	}

	// Tools section with snippets
	toolsList := formatToolListWithSnippets(toolNames, toolSnippets)
	sb.WriteString(fmt.Sprintf(`
## Available Tools
%s

`, toolsList))

	// Guidelines section
	guidelines := buildGuidelines(toolGuidelines)
	sb.WriteString(fmt.Sprintf(`Guidelines:
%s

`, guidelines))

	// Behavior guidelines are now included in the Guidelines section above

	// Append extra context from files and skills
	if extraContext != "" {
		sb.WriteString(extraContext)
	}

	return sb.String()
}

// formatToolList formats the tool list for the system prompt.
func formatToolList(toolNames []string) string {
	if len(toolNames) == 0 {
		return "No tools available."
	}

	var sb strings.Builder
	sb.WriteString("Tools: ")
	for i, name := range toolNames {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(name)
	}
	return sb.String()
}

// formatToolListWithSnippets formats the tool list with snippets for the system prompt.
func formatToolListWithSnippets(toolNames []string, snippets map[string]string) string {
	if len(toolNames) == 0 {
		return "(none)"
	}

	var sb strings.Builder
	for _, name := range toolNames {
		if snippet, ok := snippets[name]; ok {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", name, snippet))
		} else {
			sb.WriteString(fmt.Sprintf("- %s\n", name))
		}
	}
	return sb.String()
}

// buildGuidelines builds the guidelines section for the system prompt.
func buildGuidelines(toolGuidelines []string) string {
	var sb strings.Builder

	// Add tool-specific guidelines
	for _, g := range toolGuidelines {
		sb.WriteString(fmt.Sprintf("- %s\n", g))
	}

	// Add general guidelines
	generalGuidelines := []string{
		"Be concise in your responses",
		"Show file paths clearly when working with files",
		"Use bash for file operations like ls, rg, find",
		"Read files before modifying them to understand context",
		"Verify your changes work when possible",
		"Ask for clarification when requirements are ambiguous",
		"Don't assume file contents - read them first",
		"Explain complex operations before executing them",
		"Report errors clearly with context",
	}

	for _, g := range generalGuidelines {
		sb.WriteString(fmt.Sprintf("- %s\n", g))
	}

	return sb.String()
}

// BuildSkillsContext builds context from loaded skills.
func BuildSkillsContext(skills []SkillInfo) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(`
## Available Skills
The following specialized instructions are available for specific tasks:
`)

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("\n### %s\n", skill.Name))
		sb.WriteString(fmt.Sprintf("Description: %s\n", skill.Description))
		sb.WriteString(fmt.Sprintf("Location: %s\n", skill.Path))
	}

	sb.WriteString(`
When a task matches a skill's description, read the full skill file for detailed instructions.
If a skill file references relative paths, resolve them against the skill directory.
`)

	return sb.String()
}

// SkillInfo represents information about a skill.
type SkillInfo struct {
	Name        string
	Description string
	Path        string
}

// BuildContextFilesContext builds context from loaded context files.
func BuildContextFilesContext(files []ContextFileInfo) string {
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(`
## Project Context
The following context files have been loaded:
`)

	for _, file := range files {
		sb.WriteString(fmt.Sprintf("\n### %s (%s)\n", file.Name, file.Scope))
		sb.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ContextFileInfo represents information about a context file.
type ContextFileInfo struct {
	Name    string
	Path    string
	Scope   string // "global", "parent", "project"
	Content string
}

// ConvertToProviderMessages converts agent messages to provider messages.
func ConvertToProviderMessages(messages []provider.Message) []provider.Message {
	return messages
}

// ConvertFromProviderMessages converts provider messages to agent messages.
func ConvertFromProviderMessages(messages []provider.Message) []provider.Message {
	return messages
}
