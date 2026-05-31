# System Architecture

## Project Structure

```
vibecoding/
├── agent/                       # Public Agent/Provider interfaces and Builder
├── cmd/vibecoding/              # CLI entry point
│   └── main.go                  # Main program
├── internal/
│   ├── a2a/                     # A2A protocol server and master mode
│   │   ├── config.go            # A2A configuration and initialization
│   │   ├── handler.go           # JSON-RPC 2.0 handler + SSE
│   │   ├── client.go            # A2A client
│   │   ├── server.go            # HTTP server
│   │   ├── executor.go          # Task → Agent loop executor
│   │   ├── agent_card.go        # Agent Card generation
│   │   ├── task.go              # Task lifecycle management
│   │   └── master.go            # A2A Master mode (remote agent dispatch)
│   ├── acp/                     # ACP / MCP integration
│   ├── agent/                   # Core Agent loop
│   │   ├── agent.go             # Agent main logic
│   │   ├── factory.go           # AgentFactory for per-agent construction
│   │   ├── manager.go           # AgentManager lifecycle management
│   │   ├── router.go            # EventRouter
│   │   ├── subagent.go          # subagent_* tools
│   │   ├── events.go            # Event type definitions
│   │   ├── provider.go          # Provider interface adapter
│   │   └── system_prompt.go     # System prompt generation
│   ├── config/                  # Configuration management
│   ├── context/                 # Context management and token estimation
│   ├── contextfiles/            # Context file loading
│   ├── cron/                    # Scheduled task store and scheduler
│   ├── gateway/                 # OpenAI-compatible HTTP gateway
│   ├── hermes/                  # Messaging gateway (WeChat/Feishu/WebSocket)
│   ├── mcp/                     # MCP server integration
│   ├── memory/                  # Persistent memory (memory.md)
│   ├── messaging/               # Messaging platform abstraction
│   ├── platform/                # Cross-platform compatibility utilities
│   ├── provider/                # LLM Provider abstraction
│   │   ├── anthropic/           # Anthropic Messages API
│   │   ├── factory/             # Shared provider/model construction
│   │   ├── vendor*.go           # Vendor adapter registry and defaults
│   │   └── openai/              # OpenAI Chat Completions API
│   ├── sandbox/                 # Sandbox abstraction (bwrap, none)
│   ├── session/                 # Session management (JSONL)
│   ├── skills/                  # Skills system
│   ├── tools/                   # Tool implementations
│   │   ├── bash.go              # Bash command execution
│   │   ├── read.go              # File reading
│   │   ├── write.go             # File writing
│   │   ├── edit.go              # File editing
│   │   ├── grep.go              # Content search
│   │   ├── find.go              # File finding
│   │   ├── ls.go                # Directory listing
│   │   ├── plan.go              # Task planning
│   │   ├── skill_ref.go         # Skill reference loading
│   │   └── a2a_dispatch.go      # A2A remote agent dispatch
│   ├── tui/                     # Terminal UI (BubbleTea)
│   ├── ua/                      # User-Agent string generation
│   └── vendored/                # Embedded binaries (rg, fd)
└── pkg/sdk/                     # Public SDK interface
```

## Running Modes

VibeCoding supports 7 running modes, all sharing the same Agent, Provider, Tools,
and Session infrastructure:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        VibeCoding Running Modes                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │
│  │  TUI (default)│  │  Print Mode   │  │  ACP stdio   │                  │
│  │  vibecoding   │  │  vibecoding   │  │  vibecoding   │                  │
│  │              │  │  -p "..."     │  │  acp          │                  │
│  └──────────────┘  └──────────────┘  └──────────────┘                  │
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ Gateway Mode  │  │  Hermes Mode  │  │ A2A Standalone│  │ A2A Master │ │
│  │  vibecoding   │  │  vibecoding   │  │ vibecoding    │  │ --enable-  │ │
│  │  gateway      │  │  hermes       │  │ a2a start     │  │ a2a-master │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Provider System

Provider is an abstraction layer for interacting with LLM APIs. All running modes
use `internal/provider/factory` for provider creation, which applies vendor adapter
defaults before constructing the generic OpenAI-compatible or Anthropic-compatible
protocol provider.

```
┌─────────────────────────────────────────────────────────────┐
│                      Provider Interface                      │
├─────────────────────────────────────────────────────────────┤
│  Chat(ctx, params) <-chan StreamEvent                       │
│  Models() []*Model                                          │
│  GetModel(id string) *Model                                 │
│  Name() string                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              │
            ┌─────────────────┴─────────────────┐
            ▼                                   ▼
    ┌───────────────────┐             ┌───────────────────┐
    │ Vendor Adapters   │             │ Generic Fallback  │
    │ vendor_*.go       │             │ openai/anthropic  │
    └───────────────────┘             └───────────────────┘
```

Vendor resolution order:

1. Explicit `vendor` field in provider config
2. Base URL detection
3. Generic fallback based on `api`

#### StreamEvent Types

```go
type StreamEvent struct {
    Type        EventType
    TextDelta   string      // Text delta
    ThinkingDelta string    // Thinking delta
    ToolCall    *ToolCall   // Tool call
    Usage       *Usage      // Token usage
    Error       error       // Error
}
```

### 2. Agent Loop

Agent is the core logic that coordinates Provider, Tools, and Session. All running
modes reuse the same Agent loop — the difference is only the input source (terminal,
HTTP, messaging, A2A, stdio) and output target.

```
┌─────────────────────────────────────────────────────────────┐
│                       Agent Loop                             │
├─────────────────────────────────────────────────────────────┤
│  1. Build system prompt (mode + tools + context files + skills)
│  2. Send messages to Provider                               │
│  3. Process stream events (text, thinking, tool calls)      │
│  4. Execute tools and collect results                       │
│  5. Add tool results to messages                            │
│  6. Repeat until complete                                   │
└─────────────────────────────────────────────────────────────┘
```

#### Execution Flow

```
User Input (TUI / HTTP / Messaging / A2A / ACP stdio)
    │
    ▼
┌───────────────┐
│ Build Context │ ← System Prompt + Tools + Context Files + Skills
└───────┬───────┘
        │
        ▼
┌───────────────┐
│  Provider     │ ← LLM API (OpenAI/Anthropic)
│  Chat()       │
└───────┬───────┘
        │
        ▼
┌───────────────┐     ┌───────────────┐
│ Stream Events │────▶│ Tool Calls?   │
└───────┬───────┘     └───────┬───────┘
        │                     │
        │                     ▼
        │              ┌───────────────┐
        │              │ Execute Tools │
        │              └───────┬───────┘
        │                     │
        │                     ▼
        │              ┌───────────────┐
        └──────────────│ Append Results│
                       └───────────────┘
```

### 3. Multi-Agent Runtime

Multi-agent mode is opt-in with `--multi-agent`. When enabled, the main agent
gets the `subagent_spawn`, `subagent_status`, `subagent_send`, and
`subagent_destroy` tools. Child agents have isolated messages, context, session,
registry, and job manager state.

```
Main Agent
    │
    ├── AgentManager creates child agents
    ├── EventRouter routes events by AgentID
    └── subagent_* tools manage async child work
```

Child agents cannot create nested sub-agents because their registries filter out
the `subagent_*` tools.

### 4. A2A Protocol

The A2A (Agent-to-Agent) protocol enables different AI agents to discover,
communicate, and collaborate with each other.

```
┌───────────────────────────────────────────────────────────────────┐
│                     A2A Protocol Architecture                      │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────────┐          ┌──────────────────┐              │
│  │   A2A Server     │          │   A2A Client     │              │
│  │  (vibecoding)    │ ◄──────► │  (any agent)     │              │
│  │                  │  JSON-RPC │                  │              │
│  │  /a2a            │  2.0     │  SendMessage()   │              │
│  │  /a2a/send       │  + SSE   │  GetTask()       │              │
│  │  /a2a/task       │          │  CancelTask()    │              │
│  │  /a2a/events     │          │  GetAgentCard()  │              │
│  └──────────────────┘          └──────────────────┘              │
│                                                                   │
│  Task lifecycle: submitted → working → completed/failed/canceled   │
│                                                                   │
│  Two running modes:                                               │
│  • Standalone: vibecoding a2a start (port 8093)                   │
│  • Integrated: hermes.json a2a.enabled: true (shared port 8090)   │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

#### A2A Master Mode

A2A Master mode is enabled via `--enable-a2a-master`. It loads a remote agent
list from `a2a-list.json` and registers an `a2a_dispatch` tool for the LLM
to automatically dispatch tasks.

```
┌───────────────────────────────────────────────────────────────┐
│                   A2A Master Mode                              │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│  a2a-list.json                                                │
│  ┌─────────────────────────────────────────┐                  │
│  │ agents:                                 │                  │
│  │   - name: code-reviewer                 │                  │
│  │     url: http://review:8093             │                  │
│  │   - name: ci-agent                      │                  │
│  │     url: http://ci:8093                 │                  │
│  └─────────────────────────────────────────┘                  │
│           │                                                   │
│           ▼                                                   │
│  ┌──────────────────┐                                         │
│  │   A2AManager     │ ← loads agent list                      │
│  └────────┬─────────┘                                         │
│           │                                                   │
│           ▼                                                   │
│  ┌──────────────────┐                                         │
│  │  a2a_dispatch    │ ← registered as LLM tool                │
│  │  (agent_name,    │                                         │
│  │   message)       │                                         │
│  └────────┬─────────┘                                         │
│           │                                                   │
│           ▼                                                   │
│  ┌──────────────────┐  ┌──────────────────┐                  │
│  │  code-reviewer   │  │  ci-agent        │                  │
│  │  http://review   │  │  http://ci       │                  │
│  │  :8093           │  │  :8093           │                  │
│  └──────────────────┘  └──────────────────┘                  │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

### 5. Gateway Mode

`internal/gateway/` implements an OpenAI-compatible HTTP gateway exposing the
standard Chat Completions API.

```
┌─────────────────────────────────────────────────────────────┐
│                    Gateway Architecture                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  OpenAI-compatible clients (curl, SDK, any tool)             │
│       │                                                     │
│       ▼                                                     │
│  ┌──────────────────────────────────────────┐               │
│  │  HTTP Gateway (net/http)                 │               │
│  │  POST /v1/chat/completions               │               │
│  └──────────────────────────────────────────┘               │
│       │                                                     │
│       ▼                                                     │
│  ┌──────────────────────────────────────────┐               │
│  │  Agent Loop (shared)                     │               │
│  │  + Tools + Session + Sandbox + Skills     │               │
│  └──────────────────────────────────────────┘               │
│                                                             │
│  Config: gateway.json (global ~/.vibecoding/ or .vibe/)      │
│  Security: Bearer token + allowedWorkDirs + sandbox          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 6. Hermes Messaging Gateway

`internal/hermes/` implements a messaging gateway supporting WeChat, Feishu,
and WebSocket.

```
┌─────────────────────────────────────────────────────────────┐
│                    Hermes Architecture                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  WeChat   │  │  Feishu   │  │ WebSocket │                  │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘                  │
│        │             │             │                         │
│        └─────────────┼─────────────┘                         │
│                      ▼                                       │
│  ┌──────────────────────────────────────────┐               │
│  │  Hermes Dispatcher                       │               │
│  │  (per-user session, yolo mode default)   │               │
│  └──────────────────────────────────────────┘               │
│       │                                                     │
│       ▼                                                     │
│  ┌──────────────────────────────────────────┐               │
│  │  Agent Loop (shared)                     │               │
│  │  + Tools + Session + Sandbox + Skills     │               │
│  └──────────────────────────────────────────┘               │
│                                                             │
│  Config: hermes.json                                         │
│  Session: <sessionDir>/hermes/<platform>/<user_id>/          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 7. Cron Scheduler

The `internal/cron` package provides a file-backed cron store and scheduler that
can execute jobs through sub-agents or remote A2A servers. The TUI exposes `/cron`
command entry points in multi-agent mode.

```
┌─────────────────────────────────────────────────────────────┐
│                    Cron Scheduler                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐                                       │
│  │  CronStore       │ ← cron.json persistence               │
│  │  (FileCronStore) │                                       │
│  └────────┬─────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌──────────────────┐                                       │
│  │  Scheduler       │ ← periodic polling (default 30s)      │
│  └────────┬─────────┘                                       │
│           │                                                 │
│     ┌─────┴─────┐                                           │
│     ▼           ▼                                           │
│  ┌───────┐  ┌───────────┐                                   │
│  │SubAgent│  │A2A Server │                                   │
│  │(local) │  │(remote)   │  ← --a2a-target flag              │
│  └───────┘  └───────────┘                                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 8. Tool System

Tools are the way Agent interacts with the external world. All running modes
share the same tool registry.

```
┌─────────────────────────────────────────────────────────────┐
│                    Tool Interface                            │
├─────────────────────────────────────────────────────────────┤
│  Name() string                                              │
│  Description() string                                       │
│  Parameters() json.RawMessage                               │
│  Execute(ctx, params) (*ToolResult, error)                  │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  File Tools   │   │  Search Tools │   │  System Tools │
│  - read       │   │  - grep       │   │  - bash       │
│  - write      │   │  - find       │   │  - ls         │
│  - edit       │   │               │   │  - jobs       │
└───────────────┘   └───────────────┘   │  - kill       │
                                        └───────────────┘
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  Planning     │   │  Skills       │   │  A2A Master   │
│  - plan       │   │  - skill_ref  │   │  - a2a_       │
│               │   │               │   │    dispatch   │
└───────────────┘   └───────────────┘   └───────────────┘
```

### 9. Session Management

Sessions use JSONL format with tree structure and branching support.

```
┌─────────────────────────────────────────────────────────────┐
│                    Session Structure                         │
├─────────────────────────────────────────────────────────────┤
│  {                                                          │
│    "id": "session-abc123",                                  │
│    "type": "session",                                       │
│    "timestamp": "2024-01-01T00:00:00Z",                     │
│    "cwd": "/home/user/project",                             │
│    "provider": "anthropic",                                 │
│    "model": "claude-sonnet-4-20250514"                      │
│  }                                                          │
│  {                                                          │
│    "id": "msg-001",                                         │
│    "parentId": "session-abc123",                            │
│    "type": "message",                                       │
│    "role": "user",                                          │
│    "content": "..."                                         │
│  }                                                          │
│  {                                                          │
│    "id": "msg-002",                                         │
│    "parentId": "msg-001",                                   │
│    "type": "message",                                       │
│    "role": "assistant",                                     │
│    "content": "..."                                         │
│  }                                                          │
└─────────────────────────────────────────────────────────────┘
```

#### Session Types

| type | Description |
|------|-------------|
| `session` | Session metadata |
| `message` | User/assistant messages |
| `model_change` | Model change record |
| `compaction` | Context compression record |
| `label` | Session label |

### 10. Sandbox System

Sandbox implements process isolation through bubblewrap (bwrap).

```
┌─────────────────────────────────────────────────────────────┐
│                     Sandbox Manager                          │
├─────────────────────────────────────────────────────────────┤
│  SetLevel(level)                                            │
│  GetActive() *Sandbox                                       │
│  Execute(cmd) (stdout, stderr, error)                       │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  LevelNone    │   │ LevelStandard │   │  LevelStrict  │
│ (Unrestricted)│   │ (Project R/W) │   │ (Project R/O) │
└───────────────┘   └───────────────┘   └───────────────┘
```

### 11. TUI System

Terminal user interface based on BubbleTea.

```
┌─────────────────────────────────────────────────────────────┐
│                        TUI App                              │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Header Bar                          │  │
│  │  Provider: anthropic │ Model: claude-sonnet-4 │ Mode  │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Message Area                        │  │
│  │  User: ...                                            │  │
│  │  Assistant: ...                                        │  │
│  │  [tool: bash] running...                              │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Input Area                          │  │
│  │  > _                                                  │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Status Bar                          │  │
│  │  Thinking: medium │ Tokens: 1234 in / 567 out │ Cost  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Configuration Files

| File | Location | Purpose |
|------|----------|---------|
| `settings.json` | `~/.vibecoding/` or `.vibe/` | Core settings (provider, model, mode, etc.) |
| `gateway.json` | `~/.vibecoding/` or `.vibe/` | HTTP gateway configuration |
| `hermes.json` | `~/.vibecoding/` or `.vibe/` | Messaging gateway configuration |
| `a2a.json` | `~/.vibecoding/` or `.vibe/` | A2A server configuration |
| `a2a-list.json` | `~/.vibecoding/` or `.vibe/` | A2A Master remote agent list |
| `mcp.json` | `~/.vibecoding/` or `.vibe/` | MCP server configuration |
| `memory.md` | project root or `~/.vibecoding/` | Persistent memory |
| `cron.json` | `~/.vibecoding/` | Cron job persistence |

## Data Flow

### Complete Request Flow

```
1. User input (from TUI / HTTP / Messaging / A2A / ACP stdio)
   │
   ▼
2. Input layer captures
   │
   ▼
3. Agent.Run(ctx, input)
   │
   ▼
4. Build system prompt
   ├── Mode prompt (plan/agent/yolo)
   ├── Tool definitions (JSON Schema)
   ├── Context files (AGENTS.md, CLAUDE.md)
   └── Skills context
   │
   ▼
5. Build message history
   ├── Historical messages (from Session)
   └── New user message
   │
   ▼
6. Provider.Chat(ctx, params)
   │
   ▼
7. SSE streaming response
   ├── TextDelta → Display text
   ├── ThinkingDelta → Display thinking
   └── ToolCall → Execute tool (incl. a2a_dispatch)
   │
   ▼
8. Tool execution (via Sandbox)
   │
   ▼
9. Collect tool results
   │
   ▼
10. Continue conversation (back to step 5)
   │
   ▼
11. Complete, save session
```

## Key Design Decisions

### 1. Interface Abstraction

Use interface abstraction for Provider and Tool, facilitating extension and testing.

### 2. Stream Processing

Use Channel for streaming responses, providing real-time feedback.

### 3. Session Tree

Use tree structure for session storage, supporting branching and restoration.

### 4. Layered Configuration

Support global and project configuration, with project configuration overriding global.

### 5. Sandbox Isolation

Implement process-level isolation through bubblewrap, protecting system security.

### 6. Public SDK Package

The `agent/` package exposes public Go types (`Agent`, `Provider`, `Builder`) so
external applications can embed the agent without depending on internal packages.
See [SDK Integration Guide](sdk.md) for usage details.

### 7. Shared Agent Loop

All running modes (TUI, Gateway, Hermes, A2A, ACP) reuse the same Agent loop.
The only difference is the input source and output target. This ensures behavioral
consistency and avoids logic divergence.
