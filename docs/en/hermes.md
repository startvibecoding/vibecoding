# Hermes Mode

## Overview

Hermes mode runs VibeCoding as a **messaging gateway daemon** with WebSocket/HTTP API, WeChat, Feishu, and A2A protocol support. It transforms VibeCoding from a coding assistant into a deployable autonomous agent.

```bash
vibecoding hermes start
```

## Quick Start

```bash
# Generate config template
vibecoding hermes config init

# Start hermes (foreground)
vibecoding hermes start

# Start hermes (background)
vibecoding hermes start -d

# Check status
vibecoding hermes status

# Stop hermes
vibecoding hermes stop

# Connect as client
vibecoding hermes client
```

## Architecture

```
                    ┌─────────────────────────────────────┐
                    │         Hermes Gateway (:8090)       │
                    │                                     │
                    │  ┌─────────┐  ┌─────────┐  ┌─────┐ │
   WeChat ─────────►│  │Messaging│  │  HTTP   │  │ A2A │ │
   Feishu ─────────►│  │Platform │  │  REST   │  │     │ │
                    │  └────┬────┘  └────┬────┘  └──┬──┘ │
                    │       │            │          │     │
                    │       └──────┬─────┘──────────┘     │
                    │              ▼                       │
                    │       ┌──────────┐                  │
                    │       │Dispatcher│                  │
                    │       └────┬─────┘                  │
                    │            ▼                        │
                    │  ┌──────────────────┐               │
                    │  │   Agent Loop     │               │
                    │  │  (per-user)      │               │
                    │  └──────────────────┘               │
                    └─────────────────────────────────────┘
```

## CLI Commands

### `hermes start`

Start the Hermes daemon.

| Flag | Description |
|------|-------------|
| `-d` | Run in background |
| `--port` | Listen port (default: from config or 8090) |
| `--work-dir` | Default working directory |
| `-p`, `--provider` | Override default provider |
| `-m`, `--model` | Override default model |
| `--multi-agent` | Enable sub-agent tools |
| `--sandbox` | Enable bwrap sandbox |
| `--config` | Path to hermes.json |
| `--verbose` | Verbose output |
| `--debug` | Debug logging |

### `hermes stop`

Stop the running Hermes daemon via PID file + SIGTERM.

### `hermes status`

Check Hermes daemon status (PID check + HTTP health query).

### `hermes client`

Connect to a running Hermes instance via WebSocket.

| Flag | Description |
|------|-------------|
| `--url` | WebSocket URL (default: `ws://localhost:8090/ws`) |
| `--session` | Session ID to resume |

**Client Commands:**
- `/help` — Show help
- `/new` — Start a new session
- `/clear` — Clear current session
- `/status` — Show session status
- `/sessions` — List active sessions
- `/mode <mode>` — Set mode (plan/agent/yolo)
- `/compact` — Trigger compaction
- `/quit` — Exit

### `hermes config`

Manage Hermes configuration.

```bash
vibecoding hermes config init              # Create global config template
vibecoding hermes config init --project    # Create project config template
vibecoding hermes config show              # Show effective config
```

### `hermes wechat`

Manage WeChat iLink connection.

```bash
vibecoding hermes wechat login             # QR code login
vibecoding hermes wechat login --force     # Force re-login
vibecoding hermes wechat status            # Show connection status
```

### `hermes feishu`

Manage Feishu (Lark) connection.

```bash
vibecoding hermes feishu setup             # Show configuration guide
vibecoding hermes feishu status            # Show connection status
```

### `hermes webhook`

Manage webhook routes.

```bash
vibecoding hermes webhook list             # List configured routes
```

### `hermes memory`

Manage persistent memory.

```bash
vibecoding hermes memory show              # Show memory.md content
vibecoding hermes memory clear             # Reset memory.md
```

### `hermes sessions`

Manage sessions.

```bash
vibecoding hermes sessions list            # List active sessions (queries running instance)
```

### `hermes cron`

Manage cron scheduled tasks.

```bash
vibecoding hermes cron list                # List all cron jobs
vibecoding hermes cron add <name> <prompt> # Add a cron job
vibecoding hermes cron remove <id>         # Remove a cron job
vibecoding hermes cron enable <id>         # Enable a cron job
vibecoding hermes cron disable <id>        # Disable a cron job
```

## Configuration

### `hermes.json`

Configuration file for Hermes mode. Supports global + project-level overlay.

**Locations:**
- Global: `<GLOBAL_DIR>/hermes.json`
- Project: `.vibe/hermes.json` (overrides global)

```jsonc
{
  "server": {
    "port": 8090,
    "host": "0.0.0.0",
    "auth_token": ""
  },
  "default_provider": "",
  "default_model": "",
  "multi_agent": false,
  "sandbox": false,
  "wechat": {
    "enabled": false,
    "cred_path": "",
    "work_dir": "",
    "allowed_users": [],
    "auto_typing": true
  },
  "feishu": {
    "enabled": false,
    "app_id": "",
    "app_secret": "",
    "work_dir": "",
    "allowed_users": []
  },
  "webhooks": {
    "enabled": false,
    "secret": "",
    "routes": []
  },
  "a2a": {
    "enabled": false,
    "port": 8093
  },
  "cron": {
    "enabled": true
  },
  "memory": {
    "enabled": true,
    "path": ""
  },
  "security": {
    "smart_approvals": true,
    "allowed_work_dirs": []
  },
  "hooks": {
    "pre_tool_call": "",
    "post_tool_call": ""
  },
  "agent": {
    "max_turns": 90,
    "budget_pressure": true,
    "context_pressure": true,
    "budget_pressure_threshold": 0.20,
    "context_pressure_threshold": 0.55
  },
  "work_dir": "."
}
```

### Configuration Priority

```
CLI flags > hermes.json (project) > hermes.json (global) > defaults
```

### Working Directory Priority

```
Platform work_dir (wechat/feishu) > Global work_dir > CLI --work-dir > cwd
```

## Messaging Platforms

### WeChat (iLink Protocol)

- Zero external dependencies (Go stdlib only)
- QR code login, credentials saved to `<GLOBAL_DIR>/wechat-credentials.json`
- Long-poll message receiving (no public IP needed)
- Auto-relogin on session expiry
- Typing indicator support

### Feishu (Lark)

- Official SDK: `github.com/larksuite/oapi-sdk-go/v3`
- WebSocket long connection (no public IP needed)
- Text message support
- Auto-reconnect

## WebSocket API

### Connection

```
ws://localhost:8090/ws?token=<auth_token>&session=<session_id>
```

### Client → Server Messages

```jsonc
// Chat message
{"type": "message", "content": "help me with this code"}

// Slash command
{"type": "command", "content": "/new"}

// Approval response
{"type": "approval", "approval_id": "ap_xxx", "approved": true}

// Heartbeat
{"type": "ping"}
```

### Server → Client Messages

```jsonc
// Connection confirmed
{"type": "connected", "session_id": "...", "version": "..."}

// Streaming text
{"type": "text_delta", "content": "Let me help..."}

// Thinking
{"type": "think_delta", "content": "Analyzing..."}

// Tool call
{"type": "tool_call", "tool": "read", "call_id": "...", "args": {"path": "main.go"}}

// Tool result
{"type": "tool_result", "tool": "read", "call_id": "...", "result": "..."}

// File diff
{"type": "tool_diff", "call_id": "...", "path": "main.go", "diff": "..."}

// Approval request (high risk)
{"type": "approval_request", "approval_id": "ap_xxx", "tool": "bash", "args": {...}}

// Usage stats
{"type": "usage", "prompt_tokens": 1200, "completion_tokens": 350}

// Turn complete
{"type": "done", "stop_reason": "end_turn"}

// Status message
{"type": "status", "message": "Compaction triggered"}

// Command response
{"type": "command_result", "command": "/new", "message": "✅ New session created."}

// Error
{"type": "error", "message": "provider error"}

// Heartbeat
{"type": "pong"}
```

## HTTP REST API

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/health` | GET | No | Health check |
| `/api/status` | GET | Yes | Service status |
| `/api/sessions` | GET | Yes | List active sessions |
| `/api/sessions/{id}` | GET | Yes | Session details |
| `/api/sessions/{id}` | DELETE | Yes | Delete session |
| `/api/memory` | GET | Yes | Read memory.md |
| `/api/memory` | PUT | Yes | Update memory.md |
| `/api/platforms` | GET | Yes | Platform status |
| `/webhook/*` | POST | Secret | Webhook ingress |

## Smart Approvals

Tiered risk classification for tool calls:

| Risk Level | WebSocket | Messaging Platform |
|------------|-----------|-------------------|
| Low | Auto-approve | Auto-approve |
| Medium | Auto-approve + notify | Auto-approve + notify |
| High | `approval_request` → wait for response (5min timeout) | Auto-reject + notify |

**Risk Classification:**
- **Low**: `go`, `make`, `npm`, `git status/log/diff`, `ls`, `cat`, `grep`, `find`
- **Medium**: `mv`, `cp -r`, `git push`, `docker`, `curl`, `ssh`
- **High**: `rm -rf`, `sudo`, `shutdown`, `curl | sh`, `eval`, `exec`

## Pressure System

### Context Pressure

Fires `EventContextPressure` when context usage exceeds threshold (default: 55%).

```jsonc
{
  "agent": {
    "context_pressure": true,
    "context_pressure_threshold": 0.55
  }
}
```

### Budget Pressure

Fires `EventBudgetPressure` when remaining iterations reach threshold (default: 20%).

```jsonc
{
  "agent": {
    "budget_pressure": true,
    "budget_pressure_threshold": 0.20
  }
}
```

Both are one-shot events: fire once per threshold crossing, not every turn.

## Memory

Persistent memory stored as `memory.md` (Markdown, human-readable).

**Lookup Priority:**
1. `memory.path` config → explicit path
2. `.vibe/memory.md` → project memory
3. `<GLOBAL_DIR>/memory.md` → global memory

**Sections:**
- `## User Profile` — User preferences
- `## Working Memory` — Current context
- `## Lessons Learned` — Accumulated knowledge

**Default:** Writes to `.vibe/memory.md` (project directory).

## Session Management

- Each `platform:user_id` gets one persistent session
- `/new` archives current session and creates new one
- Sessions stored in `<sessionDir>/hermes/<platform>/<user_id>/active.jsonl`
- Auto-compaction when context window is full

## A2A Protocol

See [A2A Documentation](a2a.md) for Agent-to-Agent protocol details.

## Security

- **User Whitelist**: `allowed_users` per platform
- **Auth Token**: Bearer token for HTTP/WebSocket API
- **Allowed Work Dirs**: Restrict working directories
- **Shell Hooks**: Pre/post tool call external scripts
- **Smart Approvals**: Tiered risk classification
