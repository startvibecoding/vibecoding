# A2A Protocol (Agent-to-Agent)

## Overview

The A2A (Agent-to-Agent) protocol enables different AI agents to discover, communicate, and collaborate with each other. VibeCoding implements the A2A protocol as both a **standalone server** and an **integrated mode** within Hermes.

## Quick Start

```bash
# Standalone mode
vibecoding a2a start

# Check status
vibecoding a2a status

# View Agent Card
vibecoding a2a card

# Send task to another A2A server
vibecoding a2a send "list all Go files" --target http://remote:8093

# Discover remote Agent Card
vibecoding a2a discover http://remote:8093

# Stop
vibecoding a2a stop
```

## Running Modes

### Standalone Mode

Runs a dedicated A2A HTTP server on a separate port (default: 8093).

```bash
vibecoding a2a start --port 8093 --work-dir /path/to/project
```

### Integration Mode

A2A endpoints are mounted on the Hermes gateway when `a2a.enabled: true` in `hermes.json`.

```jsonc
{
  "a2a": {
    "enabled": true,
    "port": 8093  // ignored in integration mode (uses hermes port)
  }
}
```

Endpoints are available at:
- `http://localhost:8090/.well-known/agent.json`
- `http://localhost:8090/a2a`
- `http://localhost:8090/a2a/events`

## Protocol Details

- **Transport**: JSON-RPC 2.0 over HTTP
- **Streaming**: SSE (Server-Sent Events) for real-time updates
- **Task Lifecycle**: `submitted` → `working` → `completed`/`failed`/`canceled`

## Agent Card

The Agent Card describes the agent's capabilities and is served at `/.well-known/agent.json`.

```json
{
  "name": "VibeCoding",
  "description": "AI coding assistant with file editing, terminal, and search capabilities",
  "url": "http://localhost:8093/a2a",
  "version": "0.1.27",
  "capabilities": {
    "streaming": true,
    "pushNotifications": false
  },
  "skills": [
    {
      "id": "code-edit",
      "name": "Code Editing",
      "description": "Read, write, and edit code files with precise text replacement"
    },
    {
      "id": "terminal",
      "name": "Terminal Execution",
      "description": "Execute shell commands, run tests, build projects"
    },
    {
      "id": "code-search",
      "name": "Code Search",
      "description": "Search codebases with ripgrep and fd"
    }
  ]
}
```

## JSON-RPC Methods

### `message/send`

Send a message to create or continue a task.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "message/send",
  "params": {
    "task_id": "task_123",  // optional, omit to create new task
    "message": {
      "role": "user",
      "parts": [
        {"type": "text", "text": "Help me refactor main.go"}
      ]
    }
  },
  "id": 1
}
```

**Response (sync):**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "id": "task_123",
    "state": "completed",
    "artifacts": [
      {
        "name": "response",
        "parts": [{"type": "text", "text": "I've analyzed main.go..."}]
      }
    ]
  },
  "id": 1
}
```

**SSE Streaming (add `Accept: text/event-stream` header):**
```
data: {"task_id":"task_123","state":"working","message":{"role":"agent","parts":[{"type":"text","text":"Let me"}]}}

data: {"task_id":"task_123","state":"working","message":{"role":"agent","parts":[{"type":"text","text":" analyze the code..."}]}}

data: {"task_id":"task_123","state":"completed","artifact":{"name":"response","parts":[{"type":"text","text":"Here's the refactored version..."}]}}
```

### `task/get`

Get the current state of a task.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "task/get",
  "params": {
    "task_id": "task_123"
  },
  "id": 2
}
```

### `task/cancel`

Cancel a running task.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "task/cancel",
  "params": {
    "task_id": "task_123"
  },
  "id": 3
}
```

## REST Endpoints

For simpler integration, REST-style endpoints are also available:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/.well-known/agent.json` | GET | Agent Card |
| `/a2a` | POST | JSON-RPC 2.0 endpoint |
| `/a2a/send` | POST | Submit task (sync or SSE) |
| `/a2a/task?task_id=xxx` | GET | Get task state |
| `/a2a/task/cancel` | POST | Cancel task |
| `/a2a/events?task_id=xxx` | GET | SSE event stream |

## Task States

```
submitted ─► working ─► completed
                    ─► failed
                    ─► canceled
```

## Examples

### Submit Task (curl)

```bash
# Sync response
curl -X POST http://localhost:8093/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "List all Go files in the project"}]
      }
    },
    "id": 1
  }'

# SSE streaming
curl -X POST http://localhost:8093/a2a \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "Explain the project structure"}]
      }
    },
    "id": 1
  }'
```

### REST API

```bash
# Submit task
curl -X POST http://localhost:8093/a2a/send \
  -H "Content-Type: application/json" \
  -d '{"message": {"role": "user", "parts": [{"type": "text", "text": "Hello"}]}}'

# Get task
curl http://localhost:8093/a2a/task?task_id=task_123

# Cancel task
curl -X POST http://localhost:8093/a2a/task/cancel \
  -H "Content-Type: application/json" \
  -d '{"task_id": "task_123"}'
```

## Security

- **Auth Token**: Bearer token authentication (same as hermes)
- **Agent Card**: Publicly accessible (no auth required)
- **JSON-RPC**: Requires auth token when configured

## A2A Client

Send tasks to other A2A servers.

```bash
# Send a task
vibecoding a2a send "explain the project structure" --target http://remote:8093

# Send with auth token
vibecoding a2a send "run tests" --target http://remote:8093 --auth-token xxx

# Discover what a server can do
vibecoding a2a discover http://remote:8093
```

## A2A Scheduling

Cron jobs can send tasks to A2A servers instead of running local agents.

```bash
# Schedule a daily task to a remote A2A server
vibecoding hermes cron add "daily-review" "review recent changes" \
  --schedule "@daily" \
  --a2a-target http://review-agent:8093

# Schedule with auth
vibecoding hermes cron add "ci-check" "run CI tests" \
  --schedule "@every 1h" \
  --a2a-target http://ci-agent:8093 \
  --a2a-token ${CI_TOKEN}
```

The cron scheduler will send the prompt to the A2A server instead of spawning a local agent.

## A2A Master Mode

A2A Master mode lets you manage multiple remote A2A agents from a single VibeCoding instance and dispatch tasks to them via the `a2a_dispatch` tool.

### Quick Start

```bash
# 1. Generate sample config
vibecoding --init-a2a-master-config

# 2. Edit a2a-list.json with your remote agent details
#    Location: ~/.vibecoding/a2a-list.json or .vibe/a2a-list.json

# 3. Enable master mode
vibecoding --enable-a2a-master
```

### Configuration

`a2a-list.json` structure:

```json
{
  "agents": [
    {
      "name": "code-reviewer",
      "url": "http://localhost:8093"
    },
    {
      "name": "ci-agent",
      "url": "http://ci-server:8093",
      "auth_token": "your-secret-token"
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Agent name (unique identifier, used in tool calls) |
| `url` | string | A2A server URL |
| `auth_token` | string | Bearer token (optional) |

Config file locations (low to high priority):
- `~/.vibecoding/a2a-list.json` (global)
- `.vibe/a2a-list.json` (project-level, overrides global)

### a2a_dispatch Tool

When enabled, the LLM gets an `a2a_dispatch` tool to send tasks to registered remote agents:

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `agent_name` | string | Target agent name (auto-enumerated from config) |
| `message` | string | Task message |

**Examples:**
```
a2a_dispatch(agent_name="code-reviewer", message="review main.go for bugs")
a2a_dispatch(agent_name="ci-agent", message="run all unit tests")
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `--enable-a2a-master` | Enable A2A Master mode (off by default) |
| `--init-a2a-master-config` | Generate sample `a2a-list.json` |
| `--force` | Overwrite existing config file |
