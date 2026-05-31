# Agent Client Protocol (ACP)

## Overview

VibeCoding implements the **Agent Client Protocol (ACP)**, a JSON-RPC based protocol that allows AI coding agents to be integrated directly into IDEs and editors. ACP enables a client-server architecture where your IDE acts as the client and VibeCoding runs as a background agent.

## Supported IDEs

### Visual Studio Code (VS Code)

VibeCoding can be used as an ACP agent in VS Code through compatible extensions. To configure, add the following to your VS Code settings:

```json
{
  "your-acp-extension.servers": {
    "vibecoding": {
      "type": "agent",
      "command": "vibecoding",
      "args": ["acp"],
      "env": {}
    }
  }
}
```

Make sure `vibecoding` is available in your `PATH`, or use the full path to the binary.

### JetBrains IDEs (IntelliJ IDEA, GoLand, WebStorm, etc.)

JetBrains IDEs also support ACP agents. Configure in your IDE settings:

```json
{
  "acpAgentServers": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp"]
    }
  }
}
```

## Using ACP Mode

### Starting the ACP Server

```bash
# Basic ACP server (stdin/stdout)
vibecoding acp

# With specific provider and model
vibecoding acp --provider deepseek-openai --model deepseek-v4-flash

# With sandbox enabled
vibecoding acp --sandbox

# Specify mode
vibecoding acp --mode agent

# Enable multi-agent tools
vibecoding acp --multi-agent
```

### ACP Command Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--provider` | `-p` | Default from config | LLM provider |
| `--model` | `-m` | Default from config | Model ID |
| `--mode` | `-M` | `agent` | Run mode (plan, agent, yolo) |
| `--thinking` | `-t` | From config | Thinking level |
| `--sandbox` | - | false | Enable sandbox |
| `--verbose` | - | false | Verbose output |
| `--debug` | - | false | Debug logging |
| `--multi-agent` | - | false | Enable sub-agent tools and multi-agent workflows |

## Protocol Details

ACP uses JSON-RPC 2.0 over stdio for communication. The protocol supports:

### Methods

| Method | Description |
|--------|-------------|
| `initialize` | Handshake and capability negotiation |
| `session/new` | Create a new session |
| `session/load` | Load an existing session |
| `session/prompt` | Send a prompt to the agent |
| `session/cancel` | Cancel an active prompt |
| `session/update` | Server notification: session state changes |

### Capabilities

VibeCoding advertises the following ACP capabilities during initialization:

- **Load Session**: Load and continue previous sessions
- **Prompt Capabilities**: Text prompts; ACP prompt image/audio inputs are not advertised
- **Session Capabilities**: Cancel active prompts
- **MCP Capabilities**: stdio / http / sse transport supported
- **Multi-Agent Workflows**: Available when the ACP server is started with `--multi-agent`

### Notifications

The server sends `session/update` notifications with the following event types:

| Update Type | Description |
|-------------|-------------|
| `agent_message_chunk` | text delta from the agent |
| `agent_thought_chunk` | thinking/reasoning delta |
| `user_message_chunk` | historical user message |
| `tool_call` | tool being called |
| `tool_call_update` | tool status update (pending/in_progress/completed/failed) |

## MCP Server Integration

VibeCoding supports connecting to **MCP (Model Context Protocol)** servers during ACP sessions. This allows the agent to access external tools and data sources.

ACP sessions use the same MCP connection and tool-registration runtime as normal CLI/TUI sessions. The difference is that ACP clients pass `mcpServers` during session creation/loading, while normal CLI/TUI sessions load `mcp.json` at process startup.

### Configuring MCP Servers

MCP servers are configured by the IDE client and passed to VibeCoding when creating or loading sessions. The configuration format:

```json
{
  "mcpServers": [
    {
      "name": "my-database",
      "type": "stdio",
      "command": "/absolute/path/to/mcp-server",
      "args": ["--port", "8080"],
      "env": [
        {"name": "DB_URL", "value": "postgres://localhost/mydb"}
      ]
    },
    {
      "name": "remote-tools",
      "type": "http",
      "url": "https://mcp.example.com",
      "headers": [
        {"name": "Authorization", "value": "Bearer ${TOKEN}"}
      ]
    },
    {
      "name": "legacy-sse",
      "type": "sse",
      "url": "https://legacy.example.com/sse",
      "messageUrl": "https://legacy.example.com/messages"
    }
  ]
}
```

### MCP Tool Registration

When an MCP server is connected, VibeCoding automatically discovers and registers all tools exposed by the server. The tools are registered with the naming convention `mcp_<server_name>_<tool_name>`, allowing the agent to use them alongside built-in tools.

Registration happens before the agent freezes its system prompt and tool definitions for the session. MCP server changes therefore require creating/loading a new ACP session with the updated `mcpServers` payload.

In addition to `tools/*`, VibeCoding now also discovers:

- `resources/*`: exposed as MCP resource read tools
- `prompts/*`: exposed as MCP prompt rendering tools

### MCP Transport Support

Supported transports:

- `stdio`: requires absolute `command` path
- `http`: streamable HTTP endpoint via `url`
- `sse`: legacy SSE stream via `url` plus `messageUrl` for client POSTs

Additional notes:

- MCP server names must be unique within one session
- `headers` can be passed for `http` / `sse` transports
- `sampling/createMessage` is bridged to the current ACP provider/model and returns assistant text content
- MCP progress/logging/cancel notifications are surfaced as structured ACP `tool_call_update` events

## Permission System

In ACP mode, the agent can request user permissions for tool execution. The IDE client receives `session/request_permission` notifications and can respond with allow/reject decisions.

```
Client                                    Server (vibecoding acp)
  │                                           │
  │  ── session/request_permission ────────▶  │
  │      (tool_call_id, title, options)       │
  │                                           │
  │  ◀── JSON-RPC response ─────────────────  │
  │      (outcome: allow-once / reject-once)  │
```

## Example: VS Code Integration

### Step 1: Install VibeCoding

```bash
npm install -g vibecoding-installer
# or
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest
```

### Step 2: Configure VS Code

Add to your VS Code `settings.json`:

```json
{
  "acp.agents": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp", "--mode", "agent"],
      "description": "VibeCoding AI Assistant"
    }
  }
}
```

### Step 3: Start using

Open your project in VS Code, launch the ACP agent, and start asking questions or requesting code changes directly from your editor.

## Example: JetBrains IDE Integration

### Step 1: Install VibeCoding

```bash
npm install -g vibecoding-installer
```

### Step 2: Configure in JetBrains IDE

Navigate to `Settings → Tools → ACP Agents` and add a new agent:

- **Name**: VibeCoding
- **Command**: `vibecoding`
- **Arguments**: `acp --mode agent`

Or add to `.idea/workspace.xml`:

```xml
<component name="ACPSettings">
  <option name="agents">
    <list>
      <ACPAgentSetting>
        <option name="name" value="Vibecoding" />
        <option name="command" value="vibecoding" />
        <option name="args" value="acp --mode agent" />
      </ACPAgentSetting>
    </list>
  </option>
</component>
```

### Step 3: Start using

Use the ACP tool window in your JetBrains IDE to interact with VibeCoding.
