# Configuration Guide

## Configuration File Locations

VibeCoding uses two configuration files:

| File | Scope | Priority |
|------|-------|----------|
| `~/.vibecoding/settings.json` | Global (all projects) | Low |
| `.vibe/settings.json` | Project-level | High |

Project-level configuration overrides global configuration.

## Configuration Structure

### Complete Example

```json
{
  "providers": {
    "anthropic": {
      "baseUrl": "https://api.anthropic.com",
      "apiKey": "sk-ant-...",
      "api": "anthropic-messages",
      "models": [
        {
          "id": "claude-sonnet-4-20250514",
          "name": "Claude Sonnet 4",
          "contextWindow": 200000,
          "maxTokens": 8192,
          "reasoning": true
        }
      ]
    },
    "openai": {
      "baseUrl": "https://api.openai.com/v1",
      "apiKey": "sk-...",
      "api": "openai-chat",
      "models": [
        {
          "id": "gpt-4o",
          "name": "GPT-4o",
          "contextWindow": 128000,
          "maxTokens": 16384
        }
      ]
    },
    "my-custom": {
      "baseUrl": "https://my-api.example.com/v1",
      "api": "openai-chat",
      "models": []
    }
  },
  "defaultProvider": "anthropic",
  "defaultModel": "claude-sonnet-4-20250514",
  "defaultMode": "agent",
  "defaultThinkingLevel": "medium",
  "maxOutputTokens": 8192,
  "maxContextTokens": 200000,
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

- `openai-chat`: OpenAI Chat Completions API format
- `anthropic-messages`: Anthropic Messages API format

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
  "id": "claude-sonnet-4-20250514",
  "name": "Claude Sonnet 4",
  "contextWindow": 200000,
  "maxTokens": 8192,
  "reasoning": true,
  "input": ["text", "image"],
  "cost": {
    "input": 3.0,
    "output": 15.0,
    "cacheRead": 0.3,
    "cacheWrite": 3.75
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
  "defaultProvider": "anthropic"
}
```

### defaultModel

Default model ID.

```json
{
  "defaultModel": "claude-sonnet-4-20250514"
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
  "maxOutputTokens": 8192
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

1. **Global files** (in `~/.vibecoding/`):
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

## Authentication Configuration

### Option 1: Environment Variables

```bash
export ANTHROPIC_API_KEY=sk-ant-...
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

### Option 3: Inline in Configuration File

Configure directly in `settings.json` providers:

```json
{
  "providers": {
    "anthropic": {
      "apiKey": "sk-ant-..."
    }
  }
}
```

### Key Resolution Order

1. Environment variables (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`)
2. Authentication file (`~/.vibecoding/auth.json`)
3. Inline in configuration file (`settings.json`)

## Environment Variable Overrides

Any setting can be overridden via environment variables:

| Environment Variable | Overridden Setting |
|---------------------|-------------------|
| `VIBECODING_DIR` | Configuration directory |
| `VIBECODING_PROVIDER` | defaultProvider |
| `VIBECODING_MODEL` | defaultModel |
| `VIBECODING_MODE` | defaultMode |
| `VIBECODING_THINKING` | defaultThinkingLevel |

## Configuration Examples

### Minimal Configuration

```json
{
  "defaultProvider": "anthropic",
  "defaultModel": "claude-sonnet-4-20250514"
}
```

### Multi-Provider Configuration

```json
{
  "providers": {
    "anthropic": {
      "baseUrl": "https://api.anthropic.com",
      "api": "anthropic-messages"
    },
    "openai": {
      "baseUrl": "https://api.openai.com/v1",
      "api": "openai-chat"
    }
  },
  "defaultProvider": "anthropic",
  "defaultModel": "claude-sonnet-4-20250514"
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
          "id": "gpt-4o",
          "name": "GPT-4o (via proxy)"
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
    "bashBlacklist": ["rm -rf", "sudo"]
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `bashWhitelist` | []string | See below | Auto-approved command prefix list |
| `bashBlacklist` | []string | [] | Commands always requiring approval |

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
│  └─ YOLO mode → Auto-approve                                 │
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