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

### Session Management

| Parameter | Short | Description |
|-----------|-------|-------------|
| `--continue` | `-c` | Continue most recent session |
| `--resume` | `-r` | Resume session by ID or path |
| `--session` | - | Use specific session file |

### Output Control

| Parameter | Short | Description |
|-----------|-------|-------------|
| `--print` | `-P` | Non-interactive mode, print response and exit |
| `--verbose` | - | Verbose output |
| `--debug` | - | Enable debug logging |

### Security

| Parameter | Description |
|-----------|-------------|
| `--sandbox` | Enable sandbox (bubblewrap) |
| `--no-sandbox` | Disable sandbox (deprecated, disabled by default) |

### Other

| Parameter | Short | Description |
|-----------|-------|-------------|
| `--version` | `-v` | Show version |
| `--help` | `-h` | Show help |

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

# Read from file
cat main.go | vibecoding -p "Explain this file"
```

## Interactive Commands

Commands available during interactive sessions:

| Command | Description |
|---------|-------------|
| `/mode [plan\|agent\|yolo]` | Switch mode |
| `/model` | Show current model |
| `/think` | Cycle thinking level |
| `/clear` | Clear conversation |
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