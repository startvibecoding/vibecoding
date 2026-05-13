package agent

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/fuckvibecoding/vibecoding/internal/provider"
)

// BuildSystemPrompt constructs the system prompt based on mode and context.
func BuildSystemPrompt(mode string, toolNames []string, cwd string, extraContext string) string {
	var sb strings.Builder

	// Core identity and environment
	sb.WriteString(fmt.Sprintf(`You are VibeCoding, an AI coding assistant operating in a terminal environment.

## Environment
- Working directory: %s
- OS: %s %s
- Shell: /bin/bash

`, cwd, runtime.GOOS, runtime.GOARCH))

	// Mode-specific instructions
	switch mode {
	case "plan":
		sb.WriteString(`## Mode: PLAN
You are in READ-ONLY mode. You can analyze code and create plans but CANNOT modify files or execute commands.

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

Best practices:
- Read files before modifying them to understand context
- Use the edit tool for precise, targeted changes
- Use the write tool for new files or complete rewrites
- Verify your changes work when possible
- Explain your reasoning as you work
`)

	case "yolo":
		sb.WriteString(`## Mode: YOLO
You have unrestricted system access. Execute tasks efficiently without asking for permission.

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

	// Tools section
	sb.WriteString(fmt.Sprintf(`
## Available Tools
%s

Tool usage guidelines:
- Provide clear, specific parameters
- Handle errors gracefully and report them
- Verify results when possible
- Use the most appropriate tool for each task

`, formatToolList(toolNames)))

	// Behavior guidelines
	sb.WriteString(`## Behavior
- Be concise and direct in your responses
- Focus on the task at hand
- Ask for clarification when requirements are ambiguous
- Don't assume file contents - read them first
- Explain complex operations before executing them
- Report errors clearly with context
`)

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
