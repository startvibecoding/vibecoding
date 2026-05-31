<p align="center">
  <img src="docs/assets/logo.svg" alt="VibeCoding" width="128" height="128">
</p>

<h1 align="center">VibeCoding</h1>

<p align="center">
  A terminal-based AI coding assistant written in ~10,000 lines of Go, inspired by <a href="https://pi.dev">pi.dev</a>
</p>

<p align="center">
  Progressive and agile vibe-coding tool. No need to re-deploy Claude Code 、 codex、Claw、Hermes; everything is packed into a single file.
</p>

<p align="center">
  <a href="https://www.npmjs.com/package/vibecoding-installer"><img src="https://img.shields.io/npm/dm/vibecoding-installer.svg" alt="npm downloads"></a>
  <a href="https://github.com/startvibecoding/vibecoding/releases/latest"><img src="https://img.shields.io/github/release/startvibecoding/vibecoding.svg" alt="GitHub release"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://goreportcard.com/report/github.com/startvibecoding/vibecoding"><img src="https://goreportcard.com/badge/github.com/startvibecoding/vibecoding" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/startvibecoding/vibecoding"><img src="https://pkg.go.dev/badge/github.com/startvibecoding/vibecoding?status.svg" alt="GoDoc"></a>
  <a href="https://github.com/startvibecoding/vibecoding/network/dependencies"><img src="https://img.shields.io/librariesio/release/github/startvibecoding/vibecoding" alt="Dependencies"></a>
</p>

## Features

- **Multi-Provider Support**: DeepSeek (default), OpenAI, Anthropic, and vendor adapters for compatible OpenAI/Anthropic-format APIs
- **SSE Streaming**: Real-time token streaming for fast response delivery
- **Think Mode**: Extended thinking/reasoning support (DeepSeek reasoning)
- **Multi-Agent Workflows**: Optional `--multi-agent` mode with delegated sub-agents and cron command entry points
- **A2A Master Mode**: Optional `--enable-a2a-master` mode to manage multiple remote A2A agents via `a2a-list.json`, registers `a2a_dispatch` tool for automatic task dispatch
- **Three Modes**:
  - 🗒️ **Plan** — Read-only analysis and planning. Sandboxed, no file writes
  - 🔧 **Agent** (default) — Controlled read/write access to the project. Bash requires approval (configurable whitelist). Sandboxed, no network
  - 🚀 **YOLO** — Full system access with no restrictions
- **bwrap Sandbox**: Linux sandboxing via [bubblewrap](https://github.com/containers/bubblewrap) for secure execution
- **Session Management**: JSONL-based session files with tree structure, branching, compaction
- **Context Management**: Automatic context window management and token estimation
- **Rich TUI**: Terminal UI built with BubbleTea, with Markdown rendering and code highlighting
- **Cache Hit Rate**: Real-time cache hit percentage display in footer, with per-turn cache statistics
- **ACP Support**: Run as an Agent Client Protocol (ACP) stdio agent for editor integrations and compatible clients, including VS Code, Zed, and JetBrains IDEs such as IntelliJ IDEA/WebStorm via ACP-compatible plugins
- **Safer Approval Handling**: `bashBlacklist` now takes precedence over whitelist entries, including in YOLO mode, and `--print` fails fast when approval would be required
- **Unified Cache Metrics**: TUI and print mode now use the same cache-aware token accounting and cache hit rate semantics
- **Provider Debugging**: `--debug` now enables provider-level request/response diagnostics consistently, including ACP mode

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
# DeepSeek
export DEEPSEEK_API_KEY=sk-...
```

Or configure directly in `settings.json`:

```json
{
  "providers": {
    "deepseek-openai": {
      "vendor": "deepseek",
      "api": "openai-chat",
      "baseUrl": "https://api.deepseek.com",
      "apiKey": "sk-..."
    }
  }
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
vibecoding --provider deepseek-openai --model deepseek-v4-flash

# Enable sub-agent tools and multi-agent commands
vibecoding --multi-agent

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

| Location | Platform | Scope |
|----------|----------|-------|
| `~/.vibecoding/settings.json` | Linux | Global (all projects) |
| `~/Library/Application Support/vibecoding/settings.json` | macOS | Global (all projects) |
| `%APPDATA%\vibecoding\settings.json` | Windows | Global (all projects) |
| `.vibe/settings.json` | All | Project (overrides global) |

> **Windows users:** `%APPDATA%` resolves to `C:\Users\<Username>\AppData\Roaming`.
> Override the global config directory with `VIBECODING_DIR` environment variable.

### Example Settings

```json
{
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash",
  "defaultThinkingLevel": "medium",
  "defaultMode": "agent",
  "enablePlanTool": true,
  "maxContextTokens": 1000000,
  "maxOutputTokens": 384000,
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
    "bashBlacklist": ["rm -rf", "sudo"],
    "confirmBeforeWrite": true
  }
}
```

For the full list of settings including `cacheControl`, idle compression, sandbox paths, shell configuration, and API key formats, see the [Configuration Guide](docs/en/configuration.md).

### Environment Variables

| Variable | Description |
|----------|-------------|
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `VIBECODING_DIR` | Override config directory |
| `VIBECODING_PROVIDER` | Override default provider |
| `VIBECODING_MODEL` | Override default model |
| `VIBECODING_MODE` | Override default mode |
| `VIBECODING_THINKING` | Override default thinking level |
| `VIBECODING_USER_AGENT` | Custom User-Agent string |
| `VIBECODING_DEBUG` | Enable provider-level request/response debug output |

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
  -p, --provider string    Provider (deepseek-openai, deepseek-anthropic, or custom provider name)
  -m, --model string       Model ID
  -M, --mode string        Mode (plan, agent, yolo)
  -t, --thinking string    Thinking level (off, minimal, low, medium, high, xhigh)
      --multi-agent        Enable multi-agent tools and commands
      --enable-a2a-master   Enable A2A master mode (remote agent dispatch)
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
│   ├── a2a/               # A2A protocol server and master mode
│   ├── acp/               # ACP / MCP integration
│   ├── agent/             # Core agent loop
│   ├── config/            # Configuration system
│   ├── context/           # Context management and token estimation
│   ├── contextfiles/      # Context file discovery (AGENTS.md, CLAUDE.md, etc.)
│   ├── cron/              # Scheduled tasks for multi-agent workflows
│   ├── gateway/           # OpenAI-compatible HTTP gateway
│   ├── hermes/            # Messaging gateway (WeChat/Feishu/WebSocket)
│   ├── mcp/               # MCP server integration
│   ├── memory/            # Persistent memory (memory.md)
│   ├── messaging/         # Messaging platform abstraction
│   ├── platform/          # Cross-platform compatibility utilities
│   ├── provider/          # LLM provider abstraction
│   │   ├── factory/       # Shared provider/model construction
│   │   ├── openai/        # OpenAI Chat Completions API
│   │   ├── anthropic/     # Anthropic Messages API
│   │   └── vendor*.go     # Vendor adapter registry and defaults
│   ├── sandbox/           # Sandbox (bwrap) implementation
│   ├── session/           # Session management (JSONL)
│   ├── skills/            # Skills system
│   ├── tools/             # Tool implementations
│   ├── tui/               # Terminal UI (BubbleTea)
│   ├── ua/                # User-Agent string generation
│   └── vendored/          # Embedded binaries (rg, fd)
└── pkg/sdk/               # Public SDK interface
```

### Running Modes

```
vibecoding                    # Interactive terminal (TUI)
vibecoding -p "..."           # Non-interactive print mode
vibecoding acp                # ACP stdio agent (editor integration)
vibecoding gateway            # OpenAI-compatible HTTP gateway
vibecoding hermes             # Messaging gateway (WeChat/Feishu/WebSocket)
vibecoding a2a start          # A2A protocol server (standalone)
vibecoding --enable-a2a-master  # A2A master mode (remote agent dispatch)
```

## License

MIT
