# Frequently Asked Questions (FAQ)

## Basic Questions

### Q: What is VibeCoding?

A: VibeCoding is a terminal AI coding assistant that supports OpenAI and Anthropic APIs, providing code writing, debugging, refactoring, and other features.

### Q: Which LLMs are supported?

A: 
- OpenAI: GPT-4o, o1, o3-mini, etc.
- Anthropic: Claude Sonnet 4, Claude 3.5 Sonnet, Haiku, Opus
- Custom: Any OpenAI or Anthropic compatible API

### Q: How to install?

A: 
```bash
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
- Global: `~/.vibecoding/settings.json`
- Project: `.vibe/settings.json`
- Authentication: `~/.vibecoding/auth.json`

### Q: How to set API keys?

A: Three ways:
1. Environment variables: `export ANTHROPIC_API_KEY=sk-ant-...`
2. Authentication file: `~/.vibecoding/auth.json`
3. Configuration file: `providers.<name>.apiKey` in `settings.json`

### Q: How to use custom API?

A: Configure in `settings.json`:

```json
{
  "providers": {
    "my-api": {
      "baseUrl": "https://my-api.example.com/v1",
      "api": "openai-chat",
      "apiKey": "my-key"
    }
  },
  "defaultProvider": "my-api"
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
```

### Q: How to continue the last session?

A: 
```bash
vibecoding --continue
vibecoding -c
```

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

A: `~/.vibecoding/sessions/--<encoded-path>--/`

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

### Q: What to do if tools don't work?

A: 
1. Check sandbox level
2. Check file permissions
3. Use `--debug` for detailed logs

### Q: How to restrict tool permissions?

A: Use Plan mode (read-only) or configure sandbox level.

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