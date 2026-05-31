# Gateway Mode

## Overview

Gateway mode runs VibeCoding as an HTTP server that exposes a **standard OpenAI Chat Completions API**. Any OpenAI-compatible client — Cursor, Continue, Open WebUI, Python SDK, custom scripts — can connect directly, with the VibeCoding agent loop handling tool execution transparently behind the scenes.

```bash
vibecoding gateway
```

## Quick Start

```bash
# Generate config template
vibecoding --init-gateway

# Start the gateway (default :8080)
vibecoding gateway

# Test it
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "list files in current directory"}],
    "stream": false
  }'
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--port` | Listen port (default: from config or 8080) |
| `--config` | Path to gateway.json |
| `--work-dir` | Default working directory |
| `--provider` / `-p` | Override provider |
| `--model` / `-m` | Override model |
| `--sandbox` | Enable sandbox (bwrap) |
| `--multi-agent` | Enable sub-agent tools |
| `--verbose` | Verbose output |
| `--debug` | Debug logging |

## Configuration

Gateway uses its own config file `gateway.json`, separate from `settings.json`.

**Config locations** (highest priority first):

1. CLI `--config /path/to/gateway.json`
2. `.vibe/gateway.json` (project-level)
3. `~/.config/vibecoding/gateway.json` (global)

Generate a template with:

```bash
vibecoding --init-gateway
vibecoding --init-gateway --force  # overwrite existing
```

### Full Config Reference

```jsonc
{
  "listen": ":8080",

  "auth": {
    "enabled": false,
    "tokens": ["sk-your-secret-token"]
  },

  "defaultMode": "yolo",
  "defaultThinkingLevel": "medium",
  "enableSubAgents": false,

  "sandbox": {
    "enabled": false,
    "level": ""       // "none", "standard", "strict"; empty = auto from mode
  },

  "workingDir": "/home/user/projects",

  "allowedWorkDirs": [
    "/home/user/projects",
    "/opt/repos"
  ],

  "session": {
    "idleTimeoutSeconds": 1800,
    "maxSessions": 0
  },

  "toolVisibility": {
    "mode": "content",      // "content", "sse_event", "none"
    "detail": "collapsed"   // "collapsed", "expanded"
  },

  "systemPromptMode": "append",   // "append", "ignore"
  "requestTimeoutSeconds": 1800,
  "maxConcurrentRequests": 0,

  "cors": {
    "enabled": false,
    "allowOrigins": ["*"]
  },

  "provider": "",
  "model": "",
  "logLevel": "info"
}
```

## API Endpoints

### POST /v1/chat/completions

Standard OpenAI Chat Completions API. Supports streaming and non-streaming.

**Request:**

```json
{
  "model": "deepseek-v4-flash",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Read main.go and explain it."}
  ],
  "stream": true,
  "max_tokens": 4096,
  "x_session_id": "my-session",
  "x_mode": "yolo",
  "x_working_dir": "/home/user/project"
}
```

Extension fields (`x_*`) are optional:

| Field | Description |
|-------|-------------|
| `x_session_id` | Associate with an existing session (omit for new) |
| `x_mode` | Override mode for this request |
| `x_working_dir` | Override working directory (must pass `allowedWorkDirs`) |

**Non-streaming response:**

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1716883200,
  "model": "deepseek-v4-flash",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "Here is the explanation..."},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 100, "completion_tokens": 200, "total_tokens": 300},
  "x_session_id": "my-session",
  "x_tool_calls": [
    {"name": "read", "args": {"path": "main.go"}, "status": "completed"}
  ]
}
```

**Streaming response** uses standard SSE format with `data:` lines and `[DONE]` sentinel.

### GET /v1/models

Returns available models.

### GET /health

Health check (no auth required).

```json
{"status": "ok", "version": "v0.1.26", "sessions": 3}
```

## Slash Commands

When the last user message starts with `/`, it is processed as a command at the gateway layer — no LLM is called.

| Command | Description |
|---------|-------------|
| `/clear` | Clear session context |
| `/mode [plan\|agent\|yolo]` | Show or switch mode |
| `/model [model_id]` | Show or switch model |
| `/models` | List available models |
| `/sessions` | List active sessions |
| `/sessions del <id>` | Delete a session |
| `/compact` | Trigger context compaction |
| `/status` | Show session status |
| `/skill <name>` | Activate a skill |
| `/skills` | List available skills |
| `/help` | Show all commands |

Commands return standard OpenAI response format. Works in both `stream: true` and `stream: false`.

## Tool Visibility

Controls how tool execution appears in the response content.

### Mode

| `toolVisibility.mode` | Behavior |
|------------------------|----------|
| `content` (default) | Tool output mixed into content stream |
| `sse_event` | Tool output via separate `event: tool_status` SSE events |
| `none` | No tool output, client sees only final text |

### Detail

| `toolVisibility.detail` | Behavior |
|--------------------------|----------|
| `collapsed` (default) | One-line summary: `🔧 read: main.go ✅` |
| `expanded` | Full output in code fences with language detection |

**Collapsed mode** (default): most tools show a one-line summary. `edit`/`write` with diffs always show the diff. Errors always show in full.

**Expanded mode**: tool results wrapped in fenced code blocks with auto-detected language (`.go` → `go`, `.py` → `python`, bash output → `bash`, diffs → `diff`).

## Multi-Session

Each request can be associated with a session via `x_session_id`. Sessions maintain independent agent state, message history, and tools.

- No `x_session_id` → new session per request (stateless)
- With `x_session_id` → multi-turn conversation (stateful)
- Sessions auto-expire after `idleTimeoutSeconds`
- Requests within the same session are serialized

## Authentication

Set `auth.enabled: true` and configure `auth.tokens`:

```json
{
  "auth": {
    "enabled": true,
    "tokens": ["sk-token-1", "sk-token-2"]
  }
}
```

Clients send: `Authorization: Bearer sk-token-1`

The `/health` endpoint is always unauthenticated.

## Security

Three independent layers:

| Layer | Mechanism | Purpose |
|-------|-----------|---------|
| L1 | Bearer Token | Block unauthorized access |
| L2 | `allowedWorkDirs` | Restrict file system scope |
| L3 | Sandbox (bwrap) | OS-level isolation |

### allowedWorkDirs

Controls which directories `x_working_dir` can switch to:

- Not set (`null`) → no restriction
- Empty `[]` → deny all overrides, only `workingDir` allowed
- List of paths → prefix match with path separator boundary

`workingDir` itself is always trusted (admin-configured).

## Client Examples

### Python OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="sk-my-token",  # if auth enabled
)

response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[
        {"role": "user", "content": "Read main.go and explain it."},
    ],
    stream=True,
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### Multi-turn with Session

```python
response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "read main.go"}],
    extra_body={"x_session_id": "my-session"},
)

response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "now refactor the error handling"}],
    extra_body={"x_session_id": "my-session"},
)
```

### curl

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-my-token" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "explain main.go"}],
    "stream": true
  }'
```

## System Prompt Handling

| `systemPromptMode` | Behavior |
|---------------------|----------|
| `append` (default) | Client system messages appended to built-in system prompt |
| `ignore` | Client system messages discarded |

The built-in system prompt includes tool definitions, mode instructions, and context files. `append` mode preserves all of this while adding client customizations.
