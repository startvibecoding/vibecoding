# Development Guide

This document describes how to contribute code to VibeCoding.

## Development Environment Setup

### System Requirements

- Go 1.24+
- Git
- Make (optional)
- bubblewrap (optional, for sandbox testing)

### Get Source Code

```bash
git clone https://github.com/fuckvibecoding/vibecoding.git
cd vibecoding
```

### Install Dependencies

```bash
go mod download
```

### Build Project

```bash
# Build
make build

# Install to $GOPATH/bin
make install
```

### Run Tests

```bash
# Run all tests
make test

# Run tests for specific package
go test ./internal/tools/

# Run specific test
go test -run TestReadTool ./internal/tools/
```

## Project Structure

```
vibecoding/
├── cmd/vibecoding/          # CLI entry point
│   └── main.go
├── internal/
│   ├── agent/               # Core Agent loop
│   │   ├── agent.go         # Agent main logic
│   │   ├── events.go        # Event type definitions
│   │   ├── provider.go      # Provider adapter
│   │   └── system_prompt.go # System prompt generation
│   ├── config/              # Configuration management
│   ├── contextfiles/        # Context file loading
│   ├── provider/            # LLM Provider abstraction
│   │   ├── provider.go      # Provider interface
│   │   ├── anthropic/       # Anthropic implementation
│   │   └── openai/          # OpenAI implementation
│   ├── sandbox/             # Sandbox implementation
│   ├── session/             # Session management
│   ├── skills/              # Skills system
│   ├── tools/               # Tool implementations
│   │   ├── tool.go          # Tool interface and registration
│   │   ├── bash.go          # Bash command
│   │   ├── read.go          # File reading
│   │   ├── write.go         # File writing
│   │   ├── edit.go          # File editing
│   │   ├── grep.go          # Content search
│   │   ├── find.go          # File finding
│   │   └── ls.go            # Directory listing
│   ├── tui/                 # Terminal UI
│   └── util/                # Utility functions
└── pkg/sdk/                 # Public SDK (future)
```

## Core Interfaces

### Provider Interface

```go
type Provider interface {
    Name() string
    Models() []*Model
    GetModel(id string) *Model
    Chat(ctx context.Context, params ChatParams) <-chan StreamEvent
}
```

### Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, params json.RawMessage) (string, error)
}
```

## Adding New Tools

### Step 1: Create Tool File

```go
// internal/tools/mytool.go
package tools

import (
    "context"
    "encoding/json"
)

type MyTool struct {
    workdir string
}

func NewMyTool(workdir string) *MyTool {
    return &MyTool{workdir: workdir}
}

func (t *MyTool) Name() string {
    return "mytool"
}

func (t *MyTool) Description() string {
    return "Description of my tool"
}

func (t *MyTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "param1": {
                "type": "string",
                "description": "First parameter"
            }
        },
        "required": ["param1"]
    }`)
}

func (t *MyTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
    var p struct {
        Param1 string `json:"param1"`
    }
    if err := json.Unmarshal(params, &p); err != nil {
        return "", err
    }
    // Implement tool logic
    return "result", nil
}
```

### Step 2: Register Tool

In `internal/tools/tool.go`'s `RegisterDefaults()` method:

```go
func (r *Registry) RegisterDefaults() {
    r.Register(&ReadTool{workdir: r.workdir})
    r.Register(&WriteTool{workdir: r.workdir})
    r.Register(&EditTool{workdir: r.workdir})
    r.Register(&BashTool{workdir: r.workdir, sandbox: r.sandbox})
    r.Register(&GrepTool{workdir: r.workdir})
    r.Register(&FindTool{workdir: r.workdir})
    r.Register(&LsTool{workdir: r.workdir})
    r.Register(&MyTool{workdir: r.workdir}) // Add new tool
}
```

### Step 3: Update System Prompt

Add tool description in `internal/agent/system_prompt.go`.

### Step 4: Write Tests

```go
// internal/tools/mytool_test.go
package tools

import (
    "context"
    "testing"
)

func TestMyTool_Execute(t *testing.T) {
    tool := NewMyTool("/tmp")
    params := `{"param1": "value"}`
    
    result, err := tool.Execute(context.Background(), json.RawMessage(params))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if result != "expected" {
        t.Errorf("expected 'expected', got '%s'", result)
    }
}
```

## Adding New Providers

### Step 1: Create Provider Directory

```bash
mkdir -p internal/provider/myprovider
```

### Step 2: Implement Provider Interface

```go
// internal/provider/myprovider/provider.go
package myprovider

import (
    "context"
    "github.com/fuckvibecoding/vibecoding/internal/provider"
)

type MyProvider struct {
    apiKey  string
    baseURL string
}

func NewProvider(apiKey, baseURL string) *MyProvider {
    return &MyProvider{apiKey: apiKey, baseURL: baseURL}
}

func (p *MyProvider) Name() string {
    return "myprovider"
}

func (p *MyProvider) Models() []*provider.Model {
    return []*provider.Model{
        {ID: "model-1", Name: "Model 1"},
    }
}

func (p *MyProvider) GetModel(id string) *provider.Model {
    for _, m := range p.Models() {
        if m.ID == id {
            return m
        }
    }
    return nil
}

func (p *MyProvider) Chat(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
    ch := make(chan provider.StreamEvent)
    go func() {
        defer close(ch)
        // Implement streaming call
    }()
    return ch
}
```

### Step 3: Register Provider

In `cmd/vibecoding/main.go`'s `createProvider()` function:

```go
case "myprovider":
    apiKey := settings.ResolveKey(providerName)
    p = myprovider.NewProvider(apiKey, pc.BaseURL)
```

## Testing

### Run All Tests

```bash
make test
```

### Run Tests for Specific Package

```bash
go test ./internal/agent/
go test ./internal/tools/
```

### Generate Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Code Standards

### Formatting

```bash
make fmt
```

Or manually:

```bash
gofmt -w .
goimports -w .
```

### Linting

```bash
make lint
```

### Naming Conventions

- Package names: lowercase words, e.g., `tools`, `agent`
- Interface names: verbs or nouns, e.g., `Provider`, `Tool`
- Struct names: PascalCase, e.g., `ReadTool`, `AgentConfig`
- Function names: PascalCase, e.g., `NewAgent`, `Execute`
- Variable names: camelCase, e.g., `workdir`, `maxTokens`

### Error Handling

```go
// Good practice
result, err := doSomething()
if err != nil {
    return fmt.Errorf("do something: %w", err)
}

// Bad practice
result, _ := doSomething()
```

## Git Workflow

### Commit Convention

Use Conventional Commits:

```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `refactor`: Refactoring
- `test`: Tests
- `chore`: Miscellaneous

Examples:

```
feat(tools): add new search tool
fix(agent): fix streaming issue
docs(readme): update installation guide
```

### Pull Request

1. Fork project
2. Create feature branch
3. Commit changes
4. Run tests
5. Create Pull Request

## Debugging

### Enable Debug Logging

```bash
vibecoding --debug
```

### Using dlv

```bash
# Install dlv
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/vibecoding -- --debug
```

## Release Process

### Version Numbers

Use Semantic Versioning:

```
MAJOR.MINOR.PATCH

Example: 1.0.0, 1.1.0, 1.0.1
```

### Create Release

```bash
# Update version number
git tag -a v1.0.0 -m "Release v1.0.0"

# Push tags
git push --tags

# Build release packages
make build-all
```

### Publish to npm

```bash
# Publish release version
make npm-publish

# Publish pre-release version
make npm-publish-pre
```

Users can install via npm:

```bash
npm install -g vibecoding-installer
```

## Frequently Asked Questions

### Q: How to add a new tool?

A: See [Adding New Tools](#adding-new-tools) section.

### Q: How to add a new Provider?

A: See [Adding New Providers](#adding-new-providers) section.

### Q: What to do if tests fail?

A:
1. Check Go version
2. Run `go mod tidy`
3. Check error logs

### Q: How to debug sandbox issues?

A:
1. Use `--debug` flag
2. Check if bwrap is installed: `bwrap --version`
3. Check system logs