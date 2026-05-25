# Configuration Guide

## Configuration File Locations

VibeCoding uses two configuration files:

| File | Platform | Scope | Priority |
|------|----------|-------|----------|
| `~/.vibecoding/settings.json` | Linux/macOS | Global (all projects) | Low |
| `%APPDATA%\vibecoding\settings.json` | Windows | Global (all projects) | Low |
| `.vibe/settings.json` | All | Project-level | High |

> **Windows:** `%APPDATA%` resolves to `C:\Users\<Username>\AppData\Roaming`, so the full path is typically `C:\Users\<Username>\AppData\Roaming\vibecoding\settings.json`.

Project-level configuration overrides global configuration.

## Configuration Structure

### Complete Example

```json
{
  "providers": {
    "deepseek-anthropic": {
      "baseUrl": "https://api.deepseek.com/anthropic",
      "apiKey": "${DEEPSEEK_API_KEY}",
      "api": "anthropic-messages",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek-V4-Flash",
          "contextWindow": 1000000,
          "maxTokens": 384000
        },
        {
          "id": "deepseek-v4-pro",
          "name": "DeepSeek-V4-Pro",
          "reasoning": true,
          "contextWindow": 1000000,
          "maxTokens": 384000
        }
      ]
    },
    "deepseek-openai": {
      "baseUrl": "https://api.deepseek.com",
      "apiKey": "${DEEPSEEK_API_KEY}",
      "api": "openai-chat",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek-V4-Flash",
          "contextWindow": 1000000,
          "maxTokens": 384000
        },
        {
          "id": "deepseek-v4-pro",
          "name": "DeepSeek-V4-Pro",
          "reasoning": true,
          "contextWindow": 1000000,
          "maxTokens": 384000
        }
      ]
    },
    "my-custom": {
      "baseUrl": "https://my-api.example.com/v1",
      "api": "openai-chat",
      "models": []
    }
  },
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash",
  "defaultMode": "agent",
  "enablePlanTool": true,
  "defaultThinkingLevel": "medium",
  "maxOutputTokens": 384000,
  "maxContextTokens": 1000000,
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
    "enabled": true,
    "extraFiles": [
      "/path/to/extra-context.md"
    ]
  },
  "skills": {
    "enabled": true,
    "dirs": [
      "~/.vibecoding/skills",
      ".skills"
    ]
  }
}
```

## Configuration Details

### providers

Multi-provider configuration. Each provider contains:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `baseUrl` | string | ✓ | API base URL |
| `apiKey` | string | - | API key (optional, can also use environment variables) |
| `api` | string | - | API type: `openai-chat` or `anthropic-messages` |
| `thinkingFormat` | string | - | Thinking parameter format: `""`, `"openai"`, `"anthropic"`, `"xiaomi"` |
| `models` | array | - | List of available models |

#### api field

The `api` field specifies the **protocol format**, not the service provider. You can point any provider to any compatible endpoint:

- `openai-chat`: OpenAI Chat Completions API format
- `anthropic-messages`: Anthropic Messages API format

For example, DeepSeek offers both formats at different endpoints, and you can also use these formats to connect to the actual OpenAI or Anthropic services.

If not specified, auto-detected based on `baseUrl`:
- Contains "anthropic" → `anthropic-messages`
- Others → `openai-chat`

#### thinkingFormat field

Specifies how thinking/reasoning parameters are sent to the API:

- `""` (empty): Auto-detect based on URL
- `"openai"`: Use OpenAI `reasoning_effort` format
- `"anthropic"`: Use Anthropic `thinking` with `budget_tokens`
- `"xiaomi"`: Use `thinking: {type: "enabled"}` format (for Xiaomi MiMo API)

When not set, automatically detects `xiaomi` format if URL contains `xiaomimimo`.

```json
{
  "providers": {
    "xiaomi": {
      "baseUrl": "https://api.xiaomimimo.com/v1",
      "apiKey": "sk-...",
      "api": "openai-chat",
      "thinkingFormat": "xiaomi"
    }
  }
}
```

#### models array

```json
{
  "id": "deepseek-v4-flash",
  "name": "DeepSeek-V4-Flash",
  "contextWindow": 1000000,
  "maxTokens": 384000,
  "reasoning": false,
  "input": ["text"],
  "cost": {
    "input": 0.5,
    "output": 2.0
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Model ID |
| `name` | string | Display name |
| `contextWindow` | int | Context window size (tokens) |
| `maxTokens` | int | Maximum output tokens |
| `reasoning` | bool | Whether reasoning/thinking is supported |
| `input` | []string | Supported input types (text, image) |
| `cost` | object | Pricing (per million tokens) |

### defaultProvider

Default provider name. Corresponds to a key in `providers`.

```json
{
  "defaultProvider": "deepseek-openai"
}
```

### defaultModel

Default model ID.

```json
{
  "defaultModel": "deepseek-v4-flash"
}
```

### defaultMode

Default run mode.

```json
{
  "defaultMode": "agent"
}
```

Options:
- `plan`: Read-only analysis mode
- `agent`: Standard read/write mode (default)
- `yolo`: Full access mode

### enablePlanTool

Whether to register the built-in `plan` tool.

```json
{
  "enablePlanTool": true
}
```

Options:
- `true`: Register `plan` tool (default)
- `false`: Do not register `plan` tool

### defaultThinkingLevel

Default thinking level.

```json
{
  "defaultThinkingLevel": "medium"
}
```

Options:
- `off`: Disable thinking
- `minimal`: Minimal thinking
- `low`: Low level
- `medium`: Medium level
- `high`: High level
- `xhigh`: Highest level

### maxOutputTokens

Maximum output token count.

```json
{
  "maxOutputTokens": 384000
}
```

### maxContextTokens

Maximum context token count.

```json
{
  "maxContextTokens": 200000
}
```

### compaction

Context compression configuration for managing long conversations.

```json
{
  "compaction": {
    "enabled": true,
    "reserveTokens": 16384,
    "keepRecentTokens": 20000
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | true | Whether to enable compression |
| `reserveTokens` | int | 16384 | Tokens reserved for model response |
| `keepRecentTokens` | int | 20000 | Tokens kept for recent messages |

### sandbox

Sandbox configuration.

```json
{
  "sandbox": {
    "enabled": true,
    "level": "standard",
    "allowNetwork": false
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Whether to enable sandbox |
| `level` | string | standard | Sandbox level (none, standard, strict) |
| `allowNetwork` | bool | false | Whether to allow network access |

### contextFiles

Context file configuration.

```json
{
  "contextFiles": {
    "enabled": true,
    "extraFiles": [
      "/path/to/extra-context.md",
      "~/.vibecoding/global-context.md"
    ]
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | true | Whether to automatically load context files |
| `extraFiles` | []string | [] | Extra context file paths |

#### Auto-loaded Context Files

VibeCoding automatically searches for and loads the following files:

1. **Global files** (Linux/macOS: `~/.vibecoding/`, Windows: `%APPDATA%\vibecoding\`):
   - `AGENTS.md`
   - `CLAUDE.md`

2. **Project files** (searched upward from current directory):
   - `AGENTS.md`
   - `CLAUDE.md`
   - `.vibe/AGENTS.md`
   - `.vibe/CLAUDE.md`

### skills

Skill system configuration.

```json
{
  "skills": {
    "enabled": true,
    "dirs": [
      "~/.vibecoding/skills",
      ".skills"
    ]
  }
}
```

The `"~/.vibecoding/skills"` path uses `~` expansion which works on Linux/macOS. On Windows, use `%APPDATA%\vibecoding\skills` or an absolute path.

## Authentication Configuration

### Option 1: Environment Variables

```bash
export DEEPSEEK_API_KEY=sk-...
```

### Option 2: Inline in Configuration File

Configure directly in `settings.json` providers:

```json
{
  "providers": {
    "deepseek-openai": {
      "apiKey": "sk-..."
    }
  }
}
```

### Key Resolution Order

1. Environment variable (`DEEPSEEK_API_KEY`)
2. Inline in configuration file (`settings.json` providers.<name>.apiKey)

## Environment Variable Overrides

Any setting can be overridden via environment variables:

| Environment Variable | Overridden Setting |
|---------------------|-------------------|
| `VIBECODING_DIR` | Configuration directory |
| `VIBECODING_PROVIDER` | defaultProvider |
| `VIBECODING_MODEL` | defaultModel |
| `VIBECODING_MODE` | defaultMode |
| `VIBECODING_THINKING` | defaultThinkingLevel |
| `VIBECODING_DEBUG` | Provider-level request/response debug output |

## Configuration Examples

### Minimal Configuration

```json
{
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash"
}
```

### Multi-Provider Configuration

```json
{
  "providers": {
    "deepseek-anthropic": {
      "baseUrl": "https://api.deepseek.com/anthropic",
      "api": "anthropic-messages"
    },
    "deepseek-openai": {
      "baseUrl": "https://api.deepseek.com",
      "api": "openai-chat"
    }
  },
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash"
}
```

### Custom API Endpoint

```json
{
  "providers": {
    "my-proxy": {
      "baseUrl": "https://my-proxy.example.com/v1",
      "api": "openai-chat",
      "apiKey": "my-key",
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek-V4-Flash (via proxy)"
        }
      ]
    }
  },
  "defaultProvider": "my-proxy"
}
```

### Enable Sandbox

```json
{
  "sandbox": {
    "enabled": true,
    "level": "standard"
  }
}
```

### approval

Agent mode approval configuration, controls bash command approval behavior.

```json
{
  "approval": {
    "bashWhitelist": ["go ", "make ", "git ", "npm ", "yarn "],
    "bashBlacklist": ["rm -rf", "sudo"],
    "confirmBeforeWrite": true
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `bashWhitelist` | []string | See below | Auto-approved command prefix list |
| `bashBlacklist` | []string | [] | Commands always requiring approval |
| `confirmBeforeWrite` | bool | true | Require approval before `write`/`edit` in agent mode |

#### Default Whitelist

```json
[
  "go ",
  "make ",
  "git ",
  "npm ",
  "yarn ",
  "node ",
  "python ",
  "pip "
]
```

#### Approval Flow

- `bashBlacklist` has higher priority than `bashWhitelist`
- In `agent` mode, blacklisted bash commands always require approval even if they also match the whitelist
- In `agent` mode, `write` and `edit` require approval when `confirmBeforeWrite` is enabled
- In `yolo` mode, blacklisted bash commands still require approval
- In `--print` mode, commands that would require approval fail immediately instead of being auto-approved

```
┌─────────────────────────────────────────────────────────────┐
│                    Approval Flow                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Agent requests bash command execution                       │
│  │                                                           │
│  ▼                                                           │
│  Check mode                                                  │
│  ├─ Plan mode → Deny (read-only)                             │
│  ├─ Agent mode → Continue checking                           │
│  └─ YOLO mode → Auto-approve unless blacklisted              │
│                                                              │
│  Blacklist check (highest priority):                         │
│  ├─ Command matches blacklist → Require user approval        │
│  └─ Otherwise continue                                       │
│                                                              │
│  In Agent mode:                                              │
│  ├─ Non-bash tool → Auto-approve                             │
│  ├─ Command matches whitelist → Auto-approve                 │
│  └─ Otherwise → Require user approval                        │
│                                                              │
│  User approval:                                              │
│  ├─ Enter y/yes → Execute command                            │
│  └─ Enter n/no → Deny execution                              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Example Configurations

**Only allow git and npm:**
```json
{
  "approval": {
    "bashWhitelist": ["git ", "npm "]
  }
}
```

**Custom blacklist:**
```json
{
  "approval": {
    "bashWhitelist": ["go ", "make ", "git "],
    "bashBlacklist": ["rm -rf", "sudo", "dd "]
  }
}
```
