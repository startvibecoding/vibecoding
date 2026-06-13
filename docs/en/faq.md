# Frequently Asked Questions (FAQ)

## Basic Questions

### Q: What is VibeCoding?

A: VibeCoding is a terminal AI coding assistant that supports DeepSeek (default), OpenAI, Anthropic, vendor adapters for compatible APIs, and generic OpenAI/Anthropic-format custom endpoints. It provides code writing, debugging, refactoring, delegated multi-agent workflows, and other features.

### Q: What LLMs are supported?

A:
- DeepSeek (default): deepseek-v4-flash, deepseek-v4-pro (1M context, up to 384K output)
- OpenAI: GPT-4o, o1, etc.
- Anthropic: Claude Sonnet, Opus, etc.
- Vendor adapters: Google Gemini, Google Vertex, Xiaomi, Kimi, MiniMax, Seed, Qianfan, Bailian, Gitee, OpenRouter, Together, Groq, Fireworks, and more
- Custom: Any OpenAI Chat or Anthropic Messages compatible API endpoint through generic fallback

### Q: How to install?

A:
```bash
# npm (recommended)
npm install -g vibecoding-installer

# One-line install (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash

# Go install
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest

# Or from source
git clone https://github.com/startvibecoding/vibecoding.git
cd vibecoding
make build
```

## Configuration Questions

### Q: Where is the configuration file?

A: 
- Global:
  - Linux/macOS: `~/.vibecoding/settings.json`
  - Windows: `%APPDATA%\vibecoding\settings.json`
- Project: `.vibe/settings.json`
### Q: How to set API keys?

A: Two ways:
1. Environment variables: `export DEEPSEEK_API_KEY=sk-...`
2. Configuration file: `providers.<name>.apiKey` in `settings.json`

### Q: How to use custom API?

A: Configure in `settings.json`:

```json
{
  "providers": {
    "deepseek-openai": {
      "vendor": "deepseek",
      "baseUrl": "https://api.deepseek.com",
      "api": "openai-chat",
      "apiKey": "sk-..."
    }
  },
  "defaultProvider": "deepseek-openai"
}
```

## Usage Questions

### Q: How to switch modes?

A:
```bash
# Command line
vibecoding --mode plan
vibecoding -M agent

# Interactive
/mode plan
/mode agent
/mode yolo
```

### Q: How to switch models?

A:
```bash
# Command line
vibecoding --provider deepseek-openai --model deepseek-v4-pro

# Interactive
/model deepseek-v4-pro
/model                  # Show current model and available options
```

### Q: What are thinking levels?

A: Thinking levels control how much reasoning the model does before responding:
- `off`: No thinking (default)
- `minimal`: Minimal reasoning
- `low`: Light reasoning
- `medium`: Balanced reasoning
- `high`: Deep reasoning
- `xhigh`: Maximum reasoning

```bash
# Command line
vibecoding --thinking medium

# Interactive
/think           # Cycle through levels
Tab              # Keyboard shortcut to cycle
```

### Q: How to continue the last session?

A:
```bash
vibecoding --continue
vibecoding -c
```

### Q: How to manage sessions?

A: Use the `/sessions` command in interactive mode:
```
/sessions           # List sessions for current project
/sessions ls        # List all sessions across projects
/sessions set abc   # Switch to session starting with 'abc'
/sessions clear     # Create a new fresh session
/sessions del abc   # Delete session starting with 'abc'
```

### Q: How to use skills?

A: Skills are reusable prompt snippets. Use them in interactive mode:
```
/skills             # List available skills
/skill my-skill     # Activate a skill
/skill:my-skill     # Alternative syntax
```

Create skills by adding `SKILL.md` files:
- Global: `~/.vibecoding/skills/<name>/SKILL.md`
- Project: `.skills/<name>/SKILL.md`

See the [Skills System](skills.md) documentation for details.

### Q: How to view the current model?

A:
```bash
# Interactive
/model

# Command line
vibecoding --version
```

### Q: How to clear the conversation?

A:
```bash
/clear
```

## IDE Integration Questions

### Q: Can I use VibeCoding in my IDE?

A: Yes! VibeCoding supports the Agent Client Protocol (ACP) for IDE integration. Supported IDEs:
- Visual Studio Code
- JetBrains IDEs (IntelliJ IDEA, GoLand, WebStorm, etc.)

See the [ACP Protocol](acp.md) documentation for setup instructions.

### Q: How to set up VS Code integration?

A: Add to your VS Code `settings.json`:
```json
{
  "acp.agents": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp", "--mode", "agent"]
    }
  }
}
```

See the [ACP Protocol](acp.md) documentation for detailed instructions.

## Sandbox Questions

### Q: What is a sandbox?

A: A sandbox uses bubblewrap to restrict AI's file system and network access, protecting system security.

### Q: How to enable sandbox?

A: 
```bash
# Command line
vibecoding --sandbox

# Configuration file
{
  "sandbox": {
    "enabled": true,
    "level": "standard"
  }
}
```

### Q: Why doesn't the sandbox work?

A: 
1. Check if bubblewrap is installed: `bwrap --version`
2. Check if on Linux (macOS/Windows not supported)
3. Check if configuration is correct

### Q: Does macOS/Windows support sandbox?

A: No. bubblewrap is Linux-specific. You can use WSL2.

## Session Questions

### Q: Where are sessions stored?

A:
- Linux/macOS: `~/.vibecoding/sessions/--<encoded-path>--/`
- Windows: `%APPDATA%\vibecoding\sessions\--<encoded-path>--/`

### Q: How to restore old sessions?

A: 
```bash
# Resume specific session
vibecoding --resume <session-id>

# Continue most recent session
vibecoding --continue
```

### Q: What to do if session file is corrupted?

A: 
1. Check JSONL format
2. Manually fix or delete corrupted lines
3. Use backup

## Tool Questions

### Q: What tools are available?

A: VibeCoding includes core built-in tools plus optional multi-agent tools:
- `read`: Read file content (including images)
- `write`: Create/overwrite files
- `edit`: Precise text replacement
- `bash`: Execute shell commands
- `grep`: Regex content search
- `find`: Filename search
- `ls`: Directory listing
- `plan`: Publish visible task plans and status updates
- `subagent_*`: Delegate work to child agents when started with `--multi-agent`

See the [Tool System](tools.md) documentation for details.

### Q: How do I use multi-agent workflows?

A: Start VibeCoding with `--multi-agent`:

```bash
vibecoding --multi-agent
vibecoding acp --multi-agent
```

This registers `subagent_*` tools for delegated work. Cron command entry points also rely on multi-agent mode.

### Q: Can VibeCoding read images?

A: Yes! The `read` tool supports PNG, JPEG, GIF, and WebP images. Images are sent as base64-encoded data to the LLM for analysis.

### Q: What to do if tools don't work?

A:
1. Check sandbox level
2. Check file permissions
3. Use `--debug` for detailed logs

### Q: How to restrict tool permissions?

A: Use Plan mode (read-only) or configure sandbox level. In Agent mode, bash commands require approval by default (configurable via whitelist/blacklist).

## Build Questions

### Q: What to do if build fails?

A: 
```bash
# Check Go version
go version

# Update dependencies
go mod tidy

# Clean cache
go clean -cache
make clean
make build
```

### Q: What to do if tests fail?

A: 
```bash
# Run specific test
go test -v ./internal/agent/

# View detailed output
go test -v -run TestName ./...
```

## Other Questions

### Q: How to report a bug?

A: Create an Issue on GitHub, including:
- Operating system and version
- Go version
- Error message
- Steps to reproduce

### Q: How to contribute code?

A: See [Development Guide](development.md).

### Q: Is there community support?

A:
- GitHub Issues: Report bugs
- GitHub Discussions: Ask questions and discuss

### Q: What is the license?

A: MIT License

### Q: How to diagnose environment issues?

A: Use the `doctor` subcommand to check your environment:

```bash
vibecoding doctor
```

This checks OS info, config files, providers, models, sandbox, MCP servers, sessions, skills, and context files. It reports any issues with masked API keys and validates that your default provider can be initialized.

### Q: What is the current version?

A: The current version is v0.1.37. See the [Changelog](changelog.md) for version history.
