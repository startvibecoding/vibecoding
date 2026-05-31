# CLI Reference

## Overview

```
vibecoding [flags] [message...]
```

Alias: `vc`

## Command Line Arguments

### Basic Parameters

| Parameter | Short | Default | Description |
|-----------|-------|---------|-------------|
| `--provider` | `-p` | Default from config file | LLM provider (deepseek-openai, deepseek-anthropic or custom name) |
| `--model` | `-m` | Default from config file | Model ID |
| `--mode` | `-M` | `agent` | Run mode (plan, agent, yolo) |
| `--thinking` | `-t` | `off` | Thinking level (off, minimal, low, medium, high, xhigh) |
| `--multi-agent` | - | `false` | Enable multi-agent tools and commands |

### Session Management

| Parameter | Short | Description |
|-----------|-------|-------------|
| `--continue` | `-c` | Continue most recent session |
| `--resume` | `-r` | Resume session by ID or path |
| `--session` | - | Use specific session file |

### Output Control

| Parameter | Short | Description |
|-----------|-------|-------------|
| `--print` | `-P` | Non-interactive mode, print response and exit. If a tool would require approval, the command exits with an error instead of auto-approving. |
| `--verbose` | - | Verbose output |
| `--debug` | - | Enable debug logging (also enables provider request/response debug output) |

### Security

| Parameter | Description |
|-----------|-------------|
| `--sandbox` | Enable sandbox (bubblewrap) |
| `--no-sandbox` | Disable sandbox (deprecated, disabled by default) |

### Other

| Parameter | Short | Description |
|-----------|-------|-------------|
| `--init-gateway` | - | Create `gateway.json` config template |
| `--init-a2a-master-config` | - | Create `a2a-list.json` config template |
| `--enable-a2a-master` | - | Enable A2A master mode (remote agent dispatch) |
| `--force` | - | Force overwrite existing files (used with `--init-*`) |
| `--version` | `-v` | Show version |
| `--help` | `-h` | Show help |

## Subcommands

### `acp` - Agent Client Protocol Server

Run VibeCoding as an ACP-compliant stdio agent for IDE integration.

```
vibecoding acp [flags]
```

Supports VS Code, JetBrains IDEs, and any ACP-compatible editor.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--provider` | `-p` | From config | LLM provider |
| `--model` | `-m` | From config | Model ID |
| `--mode` | `-M` | `agent` | Run mode (plan, agent, yolo) |
| `--thinking` | `-t` | From config | Thinking level |
| `--sandbox` | - | false | Enable sandbox |
| `--verbose` | - | false | Verbose output |
| `--debug` | - | false | Debug logging |
| `--multi-agent` | - | false | Enable multi-agent tools for ACP sessions |

See the [ACP Protocol](acp.md) documentation for IDE integration details.

### `a2a` - A2A Protocol Server

Run the A2A (Agent-to-Agent) protocol server, supporting standalone and integrated modes.

```
vibecoding a2a [command]
```

| Subcommand | Description |
|------------|-------------|
| `start` | Start A2A server |
| `stop` | Stop A2A server |
| `status` | Show server status |
| `card` | Show/generate Agent Card |
| `send <message>` | Send task to remote A2A server |
| `discover <url>` | Discover remote Agent Card |
| `--init-a2a-config` | Create `a2a.json` config template |
| `--force` | Force overwrite existing config file |

See [A2A Protocol](a2a.md) documentation for details.

## Usage Examples

### Basic Usage

```bash
# Interactive mode
vibecoding

# With initial prompt
vibecoding "Explain this codebase"

# Non-interactive mode
vibecoding -p "Write a Hello World"
```

### Specify Provider and Model

```bash
# Use DeepSeek (OpenAI API)
vibecoding --provider deepseek-openai --model deepseek-v4-flash

# Use DeepSeek (Anthropic API)
vibecoding -p deepseek-anthropic -m deepseek-v4-flash

# Use custom provider
vibecoding --provider my-custom-provider
```

### Choose Mode

```bash
# Plan mode - read-only analysis
vibecoding --mode plan

# Agent mode - standard read/write (default)
vibecoding -M agent

# YOLO mode - full access
vibecoding -M yolo
```

### Multi-Agent Mode

```bash
# Enable sub-agent tools and multi-agent commands
vibecoding --multi-agent

# ACP sessions can also opt in
vibecoding acp --multi-agent
```

When enabled, VibeCoding registers the `subagent_*` tools and exposes multi-agent workflows such as delegated background investigation. Cron command entry points also depend on multi-agent mode.

### Thinking Levels

```bash
# Disable thinking
vibecoding --thinking off

# Medium level
vibecoding -t medium

# Highest level
vibecoding --thinking xhigh
```

### Session Management

```bash
# Continue most recent session
vibecoding --continue
vibecoding -c

# Resume specific session
vibecoding --resume session-abc123
vibecoding -r ~/.vibecoding/sessions/my-session.jsonl

# Use specific session file
vibecoding --session ./my-session.jsonl
```

### Sandbox

```bash
# Enable sandbox
vibecoding --sandbox

# Disable sandbox (default)
vibecoding
```

### Pipe Input

```bash
# Read from stdin
echo "Explain this code" | vibecoding -P

# Read from file contents directly
vibecoding -p "Explain this file: main.go"
```

### ACP Server

```bash
# Start ACP server (for IDE integration)
vibecoding acp

# ACP with specific model
vibecoding acp --provider deepseek-openai --model deepseek-v4-flash

# ACP with sandbox
vibecoding acp --sandbox --mode agent
```

## Interactive Commands

Commands available during interactive sessions:

### Mode & Model

| Command | Description |
|---------|-------------|
| `/mode [plan\|agent\|yolo]` | Switch or show current mode |
| `/model [model_id]` | Switch or show current model |
| `/think` | Cycle thinking level |

### Session Management

| Command | Description |
|---------|-------------|
| `/sessions` | List sessions for current project |
| `/sessions ls` | List all sessions across all projects |
| `/sessions set <id>` | Switch to a session by ID prefix |
| `/sessions clear` | Create a new fresh session |
| `/sessions del <id>` | Delete a session by ID prefix |
| `/clear` | Clear conversation |

### Skills

| Command | Description |
|---------|-------------|
| `/skills` | List available skills |
| `/skill <name>` | Activate a skill by name |
| `/skill:<name>` | Activate a skill (alternative syntax) |

### General

| Command | Description |
|---------|-------------|
| `/help` | Show help |
| `/quit` | Exit |

## Keyboard Shortcuts

| Shortcut | Function |
|----------|----------|
| `Ctrl+C` | Interrupt current operation / Clear input |
| `Ctrl+D` | Exit |
| `Tab` | Cycle thinking level |
| `Ctrl+T` | Toggle thinking content display |

## Environment Variables

Default settings can be overridden via environment variables:

| Variable | Description |
|----------|-------------|
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `VIBECODING_DIR` | Override config directory |
| `VIBECODING_PROVIDER` | Override default provider |
| `VIBECODING_MODEL` | Override default model |
| `VIBECODING_MODE` | Override default mode |
| `VIBECODING_THINKING` | Override default thinking level |

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Usage error |
| 130 | User interrupt (Ctrl+C) |
