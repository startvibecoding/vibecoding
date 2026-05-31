# SDK Integration Guide

VibeCoding exposes a public Go package (`github.com/startvibecoding/vibecoding/agent`) that lets you embed an AI coding agent into your own applications. This guide covers:

1. [Public Agent Package](#public-agent-package) — types, interfaces, and Builder API
2. [Implementing a Custom Provider](#implementing-a-custom-provider) — bring your own LLM backend
3. [Building and Running an Agent](#building-and-running-an-agent) — creating an agent and processing events
4. [Event Types](#event-types) — understanding the event stream
5. [Sub-Agent Mode](#sub-agent-mode) — delegating tasks to child agents

---

## Public Agent Package

Import path:

```go
import "github.com/startvibecoding/vibecoding/agent"
```

This package contains **only public types and interfaces** — no internal dependencies. It defines:

| Type | Description |
|------|-------------|
| `Agent` | Interface for all agent implementations |
| `Provider` | Interface for LLM backends |
| `Builder` | Fluent API for creating Agent instances |
| `Event` / `EventType` | Agent event stream types |
| `Message` / `ContentBlock` | Conversation message types |
| `ChatParams` / `StreamEvent` | LLM request/response types |
| `ModelInfo` / `ModelCompat` | Model metadata and compatibility flags |
| `BaseProvider` | Embeddable helper for common Provider methods |

### Agent Interface

```go
type Agent interface {
    // ID returns the unique identifier for this agent.
    ID() AgentID

    // ParentID returns the parent agent's ID, or empty if top-level.
    ParentID() AgentID

    // Run processes a user message and streams events back.
    Run(ctx context.Context, userMsg string) <-chan Event

    // RunWithMessages processes with explicit message history.
    RunWithMessages(ctx context.Context, messages []Message) <-chan Event

    // Abort signals the agent to stop processing.
    Abort()

    // GetMessages returns a copy of the current message history.
    GetMessages() []Message

    // SetMessages replaces the message history.
    SetMessages(msgs []Message)

    // GetContext returns a copy of the current agent context.
    GetContext() *AgentContext

    // SetContext replaces the agent context.
    SetContext(ctx *AgentContext)

    // GetContextUsage returns the current context window usage.
    GetContextUsage() *ContextUsage

    // LoadHistoryMessages loads historical messages into agent context.
    LoadHistoryMessages(messages []Message)

    // HandleApprovalResponse processes the user's approval response.
    HandleApprovalResponse(approvalID string, approved bool)
}
```

### Provider Interface

```go
type Provider interface {
    // Chat sends a chat request and returns a channel of streaming events.
    Chat(ctx context.Context, params ChatParams) <-chan StreamEvent

    // Name returns the provider's name (e.g. "openai", "anthropic").
    Name() string

    // Models returns the list of available models.
    Models() []ModelInfo

    // GetModel returns a model by ID, or nil if not found.
    GetModel(id string) *ModelInfo
}
```

---

## Implementing a Custom Provider

To integrate your own LLM backend, implement the `agent.Provider` interface. Embed `agent.BaseProvider` for free `Name()` / `Models()` / `GetModel()` implementations:

```go
package mybackend

import (
    "context"

    "github.com/startvibecoding/vibecoding/agent"
)

type MyProvider struct {
    agent.BaseProvider
    apiKey string
}

func NewMyProvider(apiKey string) *MyProvider {
    models := []agent.ModelInfo{
        {
            ID:            "my-model-v1",
            Name:          "My Model V1",
            Provider:      "mybackend",
            ContextWindow: 128000,
            MaxTokens:     8192,
        },
    }
    return &MyProvider{
        BaseProvider: agent.NewBaseProvider("mybackend", models),
        apiKey:       apiKey,
    }
}

func (p *MyProvider) Chat(ctx context.Context, params agent.ChatParams) <-chan agent.StreamEvent {
    ch := make(chan agent.StreamEvent, 100)

    go func() {
        defer close(ch)

        // 1. Send StreamStart
        ch <- agent.StreamEvent{Type: agent.StreamStart}

        // 2. Call your LLM API, stream responses...
        // For each text chunk:
        ch <- agent.StreamEvent{
            Type:      agent.StreamTextDelta,
            TextDelta: "Hello from my model!",
        }

        // 3. If model requests tool calls:
        // ch <- agent.StreamEvent{
        //     Type: agent.StreamToolCall,
        //     ToolCall: &agent.ToolCallBlock{
        //         ID:        "call_1",
        //         Name:      "bash",
        //         Arguments: []byte(`{"command":"ls"}`),
        //     },
        // }

        // 4. Report usage
        ch <- agent.StreamEvent{
            Type: agent.StreamUsage,
            Usage: &agent.Usage{
                InputTokens:  100,
                OutputTokens: 50,
                TotalTokens:  150,
            },
        }

        // 5. Signal completion
        ch <- agent.StreamEvent{
            Type:       agent.StreamDone,
            StopReason: "end_turn",
        }
    }()

    return ch
}
```

You can also use `WithProviderByName()` on the Builder to resolve a built-in provider by vendor name, base URL, API type, and API key without implementing `Provider` yourself:

```go
a, err := agent.NewBuilder().
    WithProviderByName("openai", "", "openai-chat", os.Getenv("OPENAI_API_KEY")).
    WithModel("gpt-4o").
    Build()
```

---

## Building and Running an Agent

Use the `Builder` fluent API to create an agent:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/startvibecoding/vibecoding/agent"
    _ "github.com/startvibecoding/vibecoding/internal/agent" // register internal builder
)

func main() {
    a, err := agent.NewBuilder().
        WithProvider(mybackend.NewMyProvider(os.Getenv("MY_API_KEY"))).
        WithModel("my-model-v1").
        WithMode("agent").                          // "plan", "agent", or "yolo"
        WithWorkDir("/home/user/project").
        WithThinkingLevel(agent.ThinkingMedium).
        WithMaxTokens(16384).
        WithMaxIterations(200).
        WithToolExecutionMode("parallel").           // "parallel" or "sequential"
        WithSystemPromptExtra("Focus on Go code.").
        WithCompaction(true, 16384).
        WithApprovalHandler(func(toolCallID, toolName string, args map[string]any) bool {
            fmt.Printf("Approve %s? [y/n] ", toolName)
            var input string
            fmt.Scanln(&input)
            return input == "y"
        }).
        Build()
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    events := a.Run(ctx, "List all Go files in this project")

    for event := range events {
        switch event.Type {
        case agent.EventTextDelta:
            fmt.Print(event.TextDelta)
        case agent.EventThinkDelta:
            // thinking content (optional)
        case agent.EventToolCall:
            fmt.Printf("\n[tool: %s]\n", event.ToolCall.Name)
        case agent.EventToolExecutionEnd:
            fmt.Printf("[result: %s]\n", truncate(event.ToolResult, 200))
        case agent.EventToolApprovalRequest:
            // Handle approval (see Builder.WithApprovalHandler)
        case agent.EventError:
            fmt.Fprintf(os.Stderr, "Error: %v\n", event.Error)
        case agent.EventDone:
            fmt.Printf("\n--- Done (reason: %s) ---\n", event.StopReason)
        }
    }
}

func truncate(s string, n int) string {
    if len(s) > n {
        return s[:n] + "..."
    }
    return s
}
```

### Builder Options

| Method | Default | Description |
|--------|---------|-------------|
| `WithProvider(p)` | *required* | LLM provider |
| `WithProviderByName(vendor, baseURL, api, apiKey)` | — | Resolve built-in provider |
| `WithModel(id)` | first model | Model ID |
| `WithMode(mode)` | `"agent"` | `"plan"` / `"agent"` / `"yolo"` |
| `WithWorkDir(dir)` | `os.Getwd()` | Working directory |
| `WithThinkingLevel(level)` | `ThinkingMedium` | `Off` / `Minimal` / `Low` / `Medium` / `High` / `XHigh` |
| `WithMaxTokens(n)` | `16384` | Max output tokens |
| `WithMaxIterations(n)` | `200` | Safety limit for loop iterations |
| `WithToolExecutionMode(m)` | `"parallel"` | `"parallel"` / `"sequential"` |
| `WithTools(names)` | all | Filter available tools |
| `WithSystemPromptExtra(s)` | `""` | Extra system prompt context |
| `WithSandbox(bool)` | `false` | Enable sandbox isolation |
| `WithSessionDir(dir)` | `~/.vibecoding/sessions` | Session persistence |
| `WithCompaction(enabled, reserve)` | `true, 16384` | Context compaction settings |
| `WithMultiAgent(bool)` | `false` | Enable sub-agent tools |
| `WithApprovalHandler(fn)` | nil | Custom tool approval callback |

---

## Event Types

The `Event` stream follows the agent lifecycle:

```
EventAgentStart
  └─ EventTurnStart
       ├─ EventTextDelta (streaming text)
       ├─ EventThinkDelta (streaming thinking)
       ├─ EventToolCall (tool requested)
       ├─ EventToolExecutionStart → EventToolExecutionEnd
       ├─ EventToolResult
       ├─ EventToolApprovalRequest → EventToolApprovalResponse
       ├─ EventPlanUpdate
       └─ EventUsage
  └─ EventTurnEnd
  └─ ... (more turns if tool calls trigger continuation)
  └─ EventDone
EventAgentEnd
```

| EventType | Key Fields | Description |
|-----------|------------|-------------|
| `EventAgentStart` | — | Agent begins processing |
| `EventAgentEnd` | `Messages` | Agent finished, final message history |
| `EventTurnStart` | — | New LLM turn begins |
| `EventTurnEnd` | `TurnMessage`, `ContextUsage` | Turn completed |
| `EventTextDelta` | `TextDelta` | Incremental text from LLM |
| `EventThinkDelta` | `ThinkDelta` | Incremental thinking from LLM |
| `EventToolCall` | `ToolCall`, `ToolArgs` | LLM requests a tool call |
| `EventToolExecutionStart` | `ToolCallID`, `ToolName`, `ToolArgs` | Tool execution begins |
| `EventToolExecutionEnd` | `ToolCallID`, `ToolResult`, `ToolDiff`, `ToolError` | Tool execution completed |
| `EventToolResult` | `ToolCallID`, `ToolResult` | Tool result recorded |
| `EventToolApprovalRequest` | `ApprovalID`, `ApprovalTool`, `ApprovalArgs` | Tool needs user approval |
| `EventPlanUpdate` | `Plan` | Structured task plan update |
| `EventUsage` | `Usage`, `ContextUsage` | Token usage report |
| `EventDone` | `StopReason`, `Usage` | Agent loop completed |
| `EventError` | `Error`, `StopReason` | Error occurred |
| `EventCompactionStart/End` | `StatusMessage` | Context compaction lifecycle |

---

## Sub-Agent Mode

Sub-agent mode allows the main agent to delegate bounded, independent subtasks to child agents running in parallel. Enable it via CLI (`--multi-agent`) or SDK (`WithMultiAgent(true)`).

### Architecture Overview

```
┌─────────────────────────────────────────────────┐
│                 Main Agent                       │
│  - Full system prompt, tools, context           │
│  - Orchestrator role                             │
│  - Has subagent_* tools                         │
├─────────────────────────────────────────────────┤
│         AgentManager                             │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐     │
│   │ SubAgent │  │ SubAgent │  │ SubAgent │     │
│   │    #1    │  │    #2    │  │    #3    │     │
│   │ (search) │  │ (review) │  │ (test)   │     │
│   └──────────┘  └──────────┘  └──────────┘     │
│         ↑            ↑             ↑             │
│     Isolated     Isolated      Isolated          │
│     context,     context,      context,          │
│     registry,    registry,     registry,         │
│     session      session       session           │
└─────────────────────────────────────────────────┘
```

### Key Components

| Component | Package | Description |
|-----------|---------|-------------|
| `AgentManager` | `internal/agent` | Manages lifecycle of all agent instances, tracks parent/child relationships, enforces policies |
| `AgentFactory` | `internal/agent` | Creates agents with consistent configuration and isolated tool registries |
| `EventRouter` | `internal/agent` | Routes events by `AgentID` to agent-specific or global handlers |
| `SubAgentPolicy` | `internal/agent` | Security constraints: max children (5), allowed modes, timeout per agent (10min) |
| `subagent_*` tools | `internal/agent` | Tools the main agent uses to spawn/manage sub-agents |

### Sub-Agent Tools

When multi-agent mode is enabled, the main agent gets four tools:

#### `subagent_spawn`

Create and start a sub-agent for a bounded task.

```json
{
  "task": "Search for all usages of the deprecated function X in src/",
  "mode": "agent",
  "work_dir": "/home/user/project",
  "tools": ["read", "grep", "find", "ls"],
  "max_iterations": 50,
  "system_prompt_extra": "Focus only on the src/ directory"
}
```

Returns a handle for polling:

```json
{
  "handle": "agent-1",
  "status": "running",
  "timeout": "10m0s"
}
```

#### `subagent_status`

Check a sub-agent's status and get results:

```json
{
  "handle": "agent-1"
}
```

Returns:

```json
{
  "handle": "agent-1",
  "status": "done",
  "message_count": 12,
  "last_response": "Found 3 usages of function X: ...",
  "updated_at": "2025-05-28T10:30:00Z"
}
```

Possible status values: `"ready"`, `"running"`, `"done"`, `"error"`.

#### `subagent_send`

Send a follow-up message to a running sub-agent:

```json
{
  "handle": "agent-1",
  "message": "Also check the test/ directory"
}
```

#### `subagent_destroy`

Destroy a finished sub-agent and release resources:

```json
{
  "handle": "agent-1"
}
```

### Sub-Agent Policy and Constraints

| Constraint | Default | Description |
|------------|---------|-------------|
| Max children | 5 | Maximum concurrent sub-agents per parent |
| Allowed modes | `["agent"]` | Sub-agents default to agent mode |
| Timeout per agent | 10 minutes | Each sub-agent has an independent timeout |
| Total timeout | 30 minutes | Global timeout for all sub-agents |
| Nesting | Disabled | Sub-agents **cannot** spawn their own sub-agents |
| Sandbox | Inherited | Sub-agents inherit the parent's sandbox configuration |

### Sub-Agent Isolation

Each sub-agent runs with **fully isolated state**:

- **Own tool registry** — independent `tools.Registry` with its own `workDir`, `Sandbox`, and `JobManager`
- **Own message history** — separate conversation context
- **Own session** — independent session storage
- **Filtered tools** — `subagent_*` tools are removed from sub-agent registries to prevent nesting
- **Extra context** — includes `SubAgentOperatingContract` instructing the sub-agent to stay within scope

### SDK Usage: Enabling Multi-Agent

```go
a, err := agent.NewBuilder().
    WithProvider(myProvider).
    WithModel("claude-sonnet-4-20250514").
    WithMode("agent").
    WithMultiAgent(true). // Enable sub-agent tools
    Build()
```

When `WithMultiAgent(true)` is set, the agent's system prompt includes the sub-agent orchestration instructions and the `subagent_spawn/status/send/destroy` tools become available.

### Event Routing with Sub-Agents

Events from sub-agents carry the sub-agent's `AgentID`. Use the `EventRouter` to dispatch events to the right handler:

```go
// Internal usage example (for reference)
router := agent.NewEventRouter()

// Register handler for a specific agent
router.RegisterAgent("agent-1", agent.RouterEventHandlerFunc(func(e agent.Event) error {
    fmt.Printf("[%s] %v\n", e.AgentID, e.Type)
    return nil
}))

// Register global handler for all agents
router.RegisterGlobal(agent.RouterEventHandlerFunc(func(e agent.Event) error {
    // Log all events across all agents
    return nil
}))
```

### Best Practices for Sub-Agents

1. **Spawn for independent work** — Sub-agents are ideal for parallel code search, review, testing, or investigation tasks that don't depend on each other.
2. **Give clear scope** — Each sub-agent task should include: what to do, where to look, what to produce, and when to stop.
3. **Limit tools** — Restrict tools to what the task needs (e.g., read-only tools for search tasks).
4. **Poll and verify** — Don't trust sub-agent results blindly. Use `subagent_status` to check, then verify important claims.
5. **Clean up** — Always `subagent_destroy` finished agents to release resources.
6. **Avoid over-delegation** — Small, sequential, or highly stateful work is better done inline.

### Approval Forwarding

Sub-agent tool calls that require approval (e.g., `bash` in agent mode) are forwarded to the parent agent's event channel. The parent TUI or approval handler sees `EventToolApprovalRequest` events with the sub-agent's `AgentID`, allowing the user to approve/deny tool calls across all agents from a single interface.

---

## Internal Architecture Reference

For developers who need to understand the internal wiring:

```
agent/                          # Public package (import this)
  ├── types.go                  # Agent, Message, Event types
  ├── provider.go               # Provider, ChatParams, StreamEvent types
  └── builder.go                # Builder API → calls buildInternal

internal/agent/                 # Internal implementation
  ├── agent.go                  # Core agent loop
  ├── factory.go                # AgentFactory (creates agents with isolated registries)
  │   └── init() { SetBuilderFunc(buildFromPublicBuilder) }
  ├── bridge.go                 # Type converters (public ↔ internal)
  │   ├── ProviderAdapter       # Wraps public Provider → internal
  │   └── AgentAdapter          # Wraps internal Agent → public
  ├── manager.go                # AgentManager (lifecycle, parent/child tracking)
  ├── subagent.go               # subagent_spawn/status/send/destroy tools
  ├── router.go                 # EventRouter (per-agent + global dispatch)
  └── system_prompt.go          # System prompt builder
```

The bridge layer in `internal/agent/bridge.go` converts between public and internal types automatically:

- `agent.Builder.Build()` → calls `buildFromPublicBuilder()` → creates internal `Agent` → wraps in `AgentAdapter` → returns `agent.Agent`
- Public `Provider` → `ProviderAdapter` → internal `provider.Provider`
- Internal `Event` → `EventToPublic()` → public `agent.Event`
- Internal `Message` → `MessageToPublic()` → public `agent.Message` (and vice versa)
