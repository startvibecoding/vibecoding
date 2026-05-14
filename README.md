<p align="center">
  <img src="docs/assets/logo.svg" alt="VibeCoding" width="128" height="128">
</p>

<h1 align="center">VibeCoding</h1>

<p align="center">
  A terminal-based AI coding assistant written in ~10,000 lines of Go, inspired by <a href="https://pi.dev">pi.dev</a>
</p>

## Features

- **Multi-Provider Support**: OpenAI (GPT-4o, o1, o3-mini), Anthropic (Claude 4 Sonnet, 3.5 Sonnet, Haiku, Opus), and custom providers
- **SSE Streaming**: Real-time token streaming for fast response delivery
- **Think Mode**: Extended thinking/reasoning support (Anthropic extended thinking, OpenAI reasoning effort)
- **Three Modes**:
  - 🗒️ **Plan** — Read-only analysis and planning. Sandboxed, no file writes
  - 🔧 **Agent** (default) — Controlled read/write access to the project. Bash requires approval (configurable whitelist). Sandboxed, no network
  - 🚀 **YOLO** — Full system access with no restrictions
- **bwrap Sandbox**: Linux sandboxing via [bubblewrap](https://github.com/containers/bubblewrap) for secure execution
- **Session Management**: JSONL-based session files with tree structure, branching, compaction
- **Context Management**: Automatic context window management and token estimation
- **Rich TUI**: Terminal UI built with BubbleTea, with Markdown rendering and code highlighting

## Quick Start

### Install

**Option 1: npm (Recommended)**

```bash
npm install -g vibecoding-installer
```

**Option 2: One-line Install**

Linux/macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.ps1 | iex
```

Or with custom install directory:

```bash
# Linux/macOS
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash

# Windows
$env:VIBECODING_INSTALL_DIR="C:\Tools\vibecoding"; irm https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.ps1 | iex
```

**Option 3: Go Install**

```bash
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest
```

**Option 4: Build from Source**

```bash
git clone https://github.com/startvibecoding/vibecoding.git
cd vibecoding
make build
```

### Cross-compile

```bash
make build-all    # Build for linux/amd64, darwin/amd64, darwin/arm64, windows/amd64
```

### Configure

Set your API key:

```bash
# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...

# OpenAI
export OPENAI_API_KEY=sk-...
```

Or use the auth file (`~/.vibecoding/auth.json`):

```json
{
  "anthropic": { "type": "api_key", "key": "sk-ant-..." },
  "openai": { "type": "api_key", "key": "sk-..." }
}
```

### Run

```bash
# Interactive mode
vibecoding

# With initial prompt
vibecoding "Explain this codebase"

# Non-interactive (print mode)
vibecoding -p "Write a hello world in Go"

# Specify provider and model
vibecoding --provider openai --model gpt-4o

# Change mode
vibecoding --mode plan    # Read-only planning
vibecoding --mode agent   # Standard (default)
vibecoding --mode yolo    # Full access

# Continue most recent session
vibecoding -c

# Disable sandbox
vibecoding --no-sandbox
```

## Configuration

### Settings Files

| Location | Scope |
|----------|-------|
| `~/.vibecoding/settings.json` | Global (all projects) |
| `.vibe/settings.json` | Project (overrides global) |

### Example Settings

```json
{
  "defaultProvider": "anthropic",
  "defaultModel": "claude-sonnet-4-20250514",
  "defaultThinkingLevel": "medium",
  "defaultMode": "agent",
  "maxContextTokens": 200000,
  "maxOutputTokens": 16384,
  "compaction": {
    "enabled": true,
    "reserveTokens": 16384,
    "keepRecentTokens": 20000
  },
  "sandbox": {
    "enabled": true,
    "level": "standard",
    "allowNetwork": false
  },
  "contextFiles": {
    "enabled": true
  },
  "retry": {
    "enabled": true,
    "maxRetries": 3,
    "baseDelayMs": 2000
  },
  "approval": {
    "bashWhitelist": ["go ", "make ", "git ", "npm ", "yarn "],
    "bashBlacklist": ["rm -rf", "sudo"]
  }
}
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `VIBECODING_DIR` | Override config directory |
| `VIBECODING_PROVIDER` | Override default provider |
| `VIBECODING_MODEL` | Override default model |
| `VIBECODING_MODE` | Override default mode |
| `VIBECODING_THINKING` | Override default thinking level |
| `VIBECODING_USER_AGENT` | Custom User-Agent string |

## Sandbox Security

VibeCoding uses [bubblewrap](https://github.com/containers/bubblewrap) for Linux sandboxing.

| Mode | File System | Network | bwrap |
|------|------------|---------|-------|
| **Plan** (strict) | Project read-only | ✗ | ✓ |
| **Agent** (standard) | Project read-write | ✗ | ✓ |
| **YOLO** (none) | Full access | ✓ | ✗ |

### Installing bwrap

```bash
# Debian/Ubuntu
sudo apt install bubblewrap

# Fedora
sudo dnf install bubblewrap

# Arch
sudo pacman -S bubblewrap
```

## CLI Reference

```
vibecoding [flags] [message...]
Aliases: vc

Flags:
  -p, --provider string    Provider (openai, anthropic, or custom provider name)
  -m, --model string       Model ID
  -M, --mode string        Mode (plan, agent, yolo)
  -t, --thinking string    Thinking level (off, minimal, low, medium, high, xhigh)
  -c, --continue           Continue most recent session
  -r, --resume string      Resume session by ID or path
      --session string     Use specific session file or ID
      --sandbox            Enable sandbox (bwrap) for secure execution
  -P, --print              Print response and exit (non-interactive)
      --verbose            Verbose output
      --debug              Enable debug logging
  -v, --version            Show version
  -h, --help               Show help
```

### Interactive Commands

| Command | Description |
|---------|-------------|
| `/mode [plan\|agent\|yolo]` | Switch mode |
| `/model` | Show current model |
| `/think` | Cycle thinking level |
| `/skills` | List loaded skills |
| `/clear` | Clear conversation |
| `/help` | Show help |
| `/quit` | Exit |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+C` | Abort / Clear input |
| `Ctrl+D` | Quit |
| `Tab` | Cycle thinking level |
| `Ctrl+T` | Toggle thinking display |

## Development

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linter
make fmt        # Format code
make clean      # Clean build artifacts
make build-all  # Cross-compile for all platforms
make dist       # Build distribution packages (.deb, .tar.gz)
```

## Architecture

```
vibecoding/
├── cmd/vibecoding/        # CLI entry point
├── internal/
│   ├── agent/             # Core agent loop
│   ├── config/            # Configuration system
│   ├── context/           # Context management and token estimation
│   ├── contextfiles/      # Context file discovery (AGENTS.md, CLAUDE.md, etc.)
│   ├── platform/          # Cross-platform compatibility utilities
│   ├── provider/          # LLM provider abstraction
│   │   ├── openai/        # OpenAI Chat Completions API
│   │   └── anthropic/     # Anthropic Messages API
│   ├── sandbox/           # Sandbox (bwrap) implementation
│   ├── session/           # Session management (JSONL)
│   ├── skills/            # Skills system
│   ├── tools/             # Tool implementations
│   ├── tui/               # Terminal UI (BubbleTea)
│   └── ua/                # User-Agent string generation
└── pkg/sdk/               # Public SDK interface
```

## License

MIT
