# Quick Start

This guide helps you get started with VibeCoding in 5 minutes.

## System Requirements

- **Operating System**: Linux, macOS, Windows (WSL)
- **Go**: 1.24+ (when building from source)
- **Optional**: bubblewrap (for sandbox functionality)

## Installation

### Method 1: One-line Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/fuckvibecoding/vibecoding/main/install.sh | bash
```

Or with custom install directory:

```bash
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/fuckvibecoding/vibecoding/main/install.sh | bash
```

This will automatically download the latest release from GitHub and install the binary to `/usr/local/bin` (or your custom `INSTALL_DIR`).

### Method 2: Go Install

```bash
go install github.com/fuckvibecoding/vibecoding/cmd/vibecoding@latest
```

### Method 3: Build from Source

```bash
# Clone repository
git clone https://github.com/fuckvibecoding/vibecoding.git
cd vibecoding

# Build
make build

# Binary is located at bin/vibecoding
```

### Method 4: Install to System

```bash
# After building from source
make install
```

## Configure API Keys

### Option 1: Environment Variables

```bash
# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...

# OpenAI
export OPENAI_API_KEY=sk-...
```

### Option 2: Authentication File

Create `~/.vibecoding/auth.json`:

```json
{
  "anthropic": {
    "type": "api_key",
    "key": "sk-ant-..."
  },
  "openai": {
    "type": "api_key",
    "key": "sk-..."
  }
}
```

## First Run

### Interactive Mode

```bash
# Start interactive session
vibecoding

# Or use alias
vc
```

### Non-Interactive Mode

```bash
# Single question
vibecoding -p "Explain what this code does"

# Read from stdin
echo "Write a Hello World" | vibecoding -P
```

### Specify Model

```bash
# Use Claude
vibecoding --provider anthropic --model claude-sonnet-4-20250514

# Use GPT-4o
vibecoding --provider openai --model gpt-4o
```

## Choose Mode

VibeCoding provides three modes:

```bash
# Plan mode - read-only analysis
vibecoding --mode plan

# Agent mode - standard read/write (default)
vibecoding --mode agent

# YOLO mode - full access
vibecoding --mode yolo
```

| Mode | File System | Network | Use Case |
|------|------------|---------|----------|
| **Plan** | Read-only | ✗ | Analysis, planning |
| **Agent** | Read/Write | ✗ | Daily development |
| **YOLO** | Full | ✓ | System-level operations |

## Basic Interaction

### Common Commands

```bash
/mode plan      # Switch to Plan mode
/mode agent     # Switch to Agent mode
/model          # View current model
/think          # Toggle thinking level
/clear          # Clear conversation
/help           # Show help
/quit           # Exit
```

### Keyboard Shortcuts

| Shortcut | Function |
|----------|----------|
| `Ctrl+C` | Interrupt / Clear input |
| `Ctrl+D` | Exit |
| `Tab` | Toggle thinking level |
| `Ctrl+T` | Toggle thinking display |

## Usage Examples

### Code Explanation

```bash
vibecoding "Explain the purpose of main.go"
```

### Code Generation

```bash
vibecoding "Write a Go HTTP server"
```

### File Operations

```bash
vibecoding "Create a README.md in the current directory"
```

### Continue Session

```bash
# Continue most recent session
vibecoding --continue

# Resume specific session
vibecoding --resume <session-id>
```

## Next Steps

- Read the [Configuration Guide](configuration.md) to customize settings
- Check the [Tool Reference](tools.md) to learn about available tools
- Understand the [Security Model](security.md) to protect your system