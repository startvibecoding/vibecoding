# Configuration Guide

## Configuration File Locations

VibeCoding uses two configuration files:

| File | Platform | Scope | Priority |
|------|----------|-------|----------|
| `~/.vibecoding/settings.json` | Linux/macOS | Global (all projects) | Low |
| `%APPDATA%\vibecoding\settings.json` | Windows | Global (all projects) | Low |
| `.vibe/settings.json` | All | Project-level | High |

> **Tip:** You can override the global config directory with the `VIBECODING_DIR` environment variable.

> **Windows:** `%APPDATA%` resolves to `C:\Users\<Username>\AppData\Roaming`, so the full path is typically `C:\Users\<Username>\AppData\Roaming\vibecoding\settings.json`.

Project-level configuration overrides global configuration. When both exist, scalar fields from the project file overwrite the global values; the `providers` map is deep-merged per-key (project providers are added to or replace global providers, not the entire map).

## Configuration Structure

### Complete Example

```json
{
  "providers": {
    "deepseek-anthropic": {
      "baseUrl": "https://api.deepseek.com/anthropic",
      "apiKey": "${DEEPSEEK_API_KEY}",
      "api": "anthropic-messages",
      "thinkingFormat": "deepseek",
      "cacheControl": false,
      "models": [
        {
          "id": "deepseek-v4-flash",
          "name": "DeepSeek-V4-Flash",
          "contextWindow": 1000000,
          "maxTokens": 384000,
          "cost": { "input": 0.5, "output": 2.0 }
        },
        {
          "id": "deepseek-v4-pro",
          "name": "DeepSeek-V4-Pro",
          "reasoning": true,
          "contextWindow": 1000000,
          "maxTokens": 384000,
          "cost": { "input": 1, "output": 4 }
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
          "maxTokens": 384000,
          "cost": { "input": 0.5, "output": 2.0 }
        },
        {
          "id": "deepseek-v4-pro",
          "name": "DeepSeek-V4-Pro",
          "reasoning": true,
          "contextWindow": 1000000,
          "maxTokens": 384000,
          "cost": { "input": 1, "output": 4 }
        }
      ]
    }
  },
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash",
  "defaultMode": "agent",
  "defaultThinkingLevel": "medium",
  "enablePlanTool": true,
  "maxContextTokens": 1000000,
  "maxOutputTokens": 384000,
  "contextFiles": {
    "enabled": true,
    "extraFiles": ["/path/to/extra-context.md"]
  },
  "skillsDir": "~/.vibecoding/skills",
  "compaction": {
    "enabled": true,
    "reserveTokens": 16384,
    "keepRecentTokens": 20000,
    "idleCompressionEnabled": false,
    "idleTimeoutSeconds": 90,
    "idleMinTokensForCompress": 150000
  },
  "sandbox": {
    "enabled": false,
    "level": "none",
    "bwrapPath": "",
    "allowNetwork": false,
    "allowedRead": ["/usr", "/lib", "/lib64", "/bin", "/sbin"],
    "allowedWrite": [],
    "deniedPaths": ["/etc/shadow", "/root", "/home"],
    "passEnv": ["PATH", "HOME", "USER", "LANG", "TERM", "SHELL"],
    "tmpSize": "100m"
  },
  "sessionDir": "~/.vibecoding/sessions",
  "shellPath": "/bin/bash",
  "shellCommandPrefix": "",
  "theme": "dark",
  "retry": {
    "enabled": true,
    "maxRetries": 3,
    "baseDelayMs": 2000
  },
  "approval": {
    "bashWhitelist": ["go ", "make ", "git ", "npm ", "yarn ", "node ", "python ", "pip "],
    "bashBlacklist": ["rm -rf", "sudo"],
    "confirmBeforeWrite": true
  }
}
```

## All Configuration Fields

### Top-Level Fields Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `providers` | object | *(see below)* | Provider configurations (keyed by name) |
| `defaultProvider` | string | `"deepseek-openai"` | Which provider to use by default |
| `defaultModel` | string | `"deepseek-v4-flash"` | Which model ID to use by default |
| `defaultMode` | string | `"agent"` | Default run mode: `plan`, `agent`, or `yolo` |
| `defaultThinkingLevel` | string | `"medium"` | Default thinking level |
| `enablePlanTool` | bool | `true` | Register the built-in `plan` tool |
| `maxContextTokens` | int | `0` (auto) | Override maximum context token count |
| `maxOutputTokens` | int | `0` (auto) | Override maximum output token count |
| `contextFiles` | object | *(see below)* | Context file loading settings |
| `skillsDir` | string | `"~/.vibecoding/skills"` | Global skills directory path |
| `compaction` | object | *(see below)* | Context compaction settings |
| `sandbox` | object | *(see below)* | Sandbox execution settings |
| `sessionDir` | string | `"~/.vibecoding/sessions"` | Session file storage directory |
| `shellPath` | string | `""` (auto) | Custom shell path for Bash tool |
| `shellCommandPrefix` | string | `""` | Prefix prepended to every shell command |
| `theme` | string | `"dark"` | UI theme: `"dark"` or `"light"` |
| `retry` | object | *(see below)* | API call retry settings |
| `approval` | object | *(see below)* | Bash command approval settings |
| `webSearch` | object | *(see below)* | Hosted web search settings |

---

## Configuration Details

### providers

Multi-provider configuration. Each provider is an object keyed by a user-chosen name:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `baseUrl` | string | ✓ | — | API base URL |
| `vendor` | string | — | auto-detect | Optional vendor adapter name (see below) |
| `apiKey` | string | — | `""` | API key (see [Authentication](#authentication-configuration) below) |
| `api` | string | — | auto-detect | API protocol: `"openai-chat"`, `"openai-responses"`, `"anthropic-messages"`, `"google-gemini"`, or `"google-vertex"` |
| `httpProxy` | string | — | `""` | Optional per-provider HTTP proxy URL, e.g. `"http://127.0.0.1:7890"` |
| `headers` | object | — | `{}` | Optional custom HTTP headers applied to every provider request; values support the same `${ENV}` and `!cmd` resolution as `apiKey` |
| `thinkingFormat` | string | — | auto-detect | Thinking parameter format (see below) |
| `cacheControl` | bool | — | `false` | Enable Anthropic prompt caching; set `true` when using Claude models |
| `models` | array | — | `[]` | List of available models |

#### vendor field

The `vendor` field selects a vendor adapter without changing the provider config schema. It is optional; when omitted, VibeCoding tries to detect the vendor from `baseUrl`, then falls back to the generic protocol provider selected by `api`.

Selection order:

1. Explicit `vendor`
2. Base URL detection
3. Generic fallback: `openai-chat`, `openai-responses`, `anthropic-messages`, `google-gemini`, or `google-vertex`

Built-in vendor adapters include `openai`, `anthropic`, `claude`, `deepseek`, `google-gemini`, `google-vertex`, `xiaomi`, `xiaomi-token-plan-ams`, `xiaomi-token-plan-cn`, `xiaomi-token-plan-sgp`, `kimi`, `minimax`, `seed`, `qianfan`, `bailian`, `gitee`, `openrouter`, `together`, `groq`, and `fireworks`.

```json
{
  "providers": {
    "custom-deepseek": {
      "vendor": "deepseek",
      "baseUrl": "https://api.deepseek.com",
      "apiKey": "${DEEPSEEK_API_KEY}",
      "api": "openai-chat",
      "headers": {
        "X-Request-Source": "vibecoding",
        "X-Gateway-Token": "${MY_GATEWAY_TOKEN}"
      },
      "models": [
        { "id": "deepseek-v4-flash", "name": "DeepSeek-V4-Flash", "contextWindow": 1000000 }
      ]
    }
  }
}
```

### webSearch

Hosted web search settings. This is disabled by default.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | bool | — | `false` | Enable hosted web search registration |
| `provider` | string | — | `defaultProvider` | Provider configuration name to use for hosted web search |
| `providerType` | string | — | auto | Hosted tool type, usually `responses` or `messages` |
| `model` | string | — | `""` | Optional metadata for routing, display, or future provider-specific handling |

```json
{
  "webSearch": {
    "enabled": true,
    "provider": "gpt",
    "providerType": "responses",
    "model": "gpt-5.4"
  }
}
```

When `provider` points to a configured provider name, VibeCoding resolves that provider's `baseUrl`, `api`, and vendor behavior before registering the hosted search tool.

#### api field

The `api` field specifies the **protocol format**, not the service provider. You can point any provider to any compatible endpoint:

- `openai-chat`: OpenAI Chat Completions API format
- `openai-responses`: OpenAI Responses API format (`POST /v1/responses`)
- `anthropic-messages`: Anthropic Messages API format
- `google-gemini`: Native Gemini API `streamGenerateContent` format
- `google-vertex`: Native Vertex AI Gemini `streamGenerateContent` format

For example, DeepSeek offers both formats at different endpoints, and you can also use these formats to connect to the actual OpenAI or Anthropic services.

If not specified, auto-detected based on `baseUrl`:
- Contains `generativelanguage.googleapis.com` → `google-gemini`
- Contains `aiplatform.googleapis.com` → `google-vertex`
- Contains "anthropic" → `anthropic-messages`
- Others → `openai-chat`

Google native providers can be configured directly:

```json
{
  "providers": {
    "google-gemini": {
      "baseUrl": "https://generativelanguage.googleapis.com/v1beta/models",
      "apiKey": "${GOOGLE_API_KEY}",
      "api": "google-gemini",
      "models": [
        { "id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "reasoning": true, "contextWindow": 1000000, "maxTokens": 65536 }
      ]
    },
    "google-vertex": {
      "baseUrl": "https://aiplatform.googleapis.com/v1/publishers/google/models",
      "apiKey": "${GOOGLE_CLOUD_API_KEY}",
      "api": "google-vertex",
      "models": [
        { "id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "reasoning": true, "contextWindow": 1000000, "maxTokens": 65536 }
      ]
    }
  }
}
```

For `google-vertex`, `${GOOGLE_CLOUD_API_KEY}` uses the Vertex AI API key path and does not require `gcloud`, project, or location in the URL. OAuth access tokens are still supported by using a project-scoped URL such as `https://aiplatform.googleapis.com/v1/projects/YOUR_PROJECT/locations/global/publishers/google/models` and an access token value such as `!gcloud auth print-access-token`. Set `VIBECODING_ALLOW_SHELL_CONFIG=1` before using `!command` values.

#### thinkingFormat field

Specifies how thinking/reasoning parameters are sent to the API:

| Value | Behavior |
|-------|----------|
| `""` (empty) | Auto-detect based on URL |
| `"openai"` | Use OpenAI `reasoning_effort` format |
| `"anthropic"` | Use Anthropic `thinking` with `budget_tokens` |
| `"deepseek"` | Use DeepSeek `thinking: {type: "enabled"}` + `reasoning_effort` (OpenAI) or `output_config.effort` (Anthropic) |
| `"xiaomi"` | Legacy thinking-only format: `thinking: {type: "enabled"}` |

When not set, automatically detects:
- URL contains `deepseek` → `"deepseek"`
- URL contains `xiaomimimo` → `"xiaomi"`

```json
{
  "providers": {
    "deepseek-openai": {
      "baseUrl": "https://api.deepseek.com",
      "apiKey": "sk-...",
      "api": "openai-chat",
      "thinkingFormat": "deepseek"
    }
  }
}
```

#### cacheControl field

Enable Anthropic-style prompt caching. When set to `true`, VibeCoding adds cache control headers to requests. **You should enable this when using Claude models through the Anthropic API** to reduce cost and latency.

```json
{
  "providers": {
    "anthropic": {
      "baseUrl": "https://api.anthropic.com",
      "apiKey": "${ANTHROPIC_API_KEY}",
      "api": "anthropic-messages",
      "cacheControl": true,
      "models": [
        {
          "id": "claude-sonnet-4-20250514",
          "name": "Claude Sonnet 4",
          "contextWindow": 200000,
          "maxTokens": 8192,
          "cost": {
            "input": 3,
            "output": 15,
            "cacheRead": 0.3,
            "cacheWrite": 3.75
          }
        }
      ]
    }
  }
}
```

#### models array

Each model in the `models` array:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `id` | string | — | Model ID sent to the API |
| `name` | string | — | Human-readable display name |
| `reasoning` | bool | `false` | Whether the model supports thinking/reasoning |
| `contextWindow` | int | `0` | Context window size (tokens) |
| `maxTokens` | int | `0` | Maximum output tokens per response |
| `input` | []string | `[]` | Supported input modalities: `"text"`, `"image"` |
| `cost` | object | `null` | Pricing per million tokens |
| `compat` | object | `null` | Model-specific compatibility flags for provider quirks |

The `cost` object:

| Field | Type | Description |
|-------|------|-------------|
| `input` | float | Cost per million input tokens |
| `output` | float | Cost per million output tokens |
| `cacheRead` | float | Cost per million cached read tokens (Anthropic) |
| `cacheWrite` | float | Cost per million cached write tokens (Anthropic) |

The `compat` object is optional and should only be set when a model needs protocol-specific adjustments:

| Field | Type | Description |
|-------|------|-------------|
| `thinkingFormat` | string | Override model thinking format (`openai`, `deepseek`, `xiaomi`, `anthropic`, etc.) |
| `requiresReasoningContentOnAssistant` | bool | Send empty `reasoning_content` on replayed assistant messages |
| `requiresReasoningContentOnAssistantMessages` | bool | Alias used by the reference implementation; treated the same as above |
| `forceAdaptiveThinking` | bool | Force Anthropic adaptive thinking format |
| `supportsReasoningEffort` | bool | Whether the model accepts `reasoning_effort` |
| `maxTokensField` | string | Use `max_tokens` or `max_completion_tokens` |
| `supportsDeveloperRole` | bool | Whether developer-role messages are supported |
| `supportsStore` | bool | Whether OpenAI `store` is supported |
| `supportsStrictMode` | bool | Whether strict tool schemas are supported |
| `supportsCacheControlOnTools` | bool | Whether cache control can be applied to tool definitions |
| `supportsLongCacheRetention` | bool | Whether long prompt-cache retention is supported |
| `sendSessionAffinityHeaders` | bool | Whether session affinity headers should be sent |
| `supportsEagerToolInputStreaming` | bool | Whether Anthropic eager tool input streaming is supported |

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

---

### defaultProvider

Default provider name. Must match a key in `providers`.

```json
{ "defaultProvider": "deepseek-openai" }
```

### defaultModel

Default model ID. Must match an `id` in the chosen provider's `models` list.

```json
{ "defaultModel": "deepseek-v4-flash" }
```

### defaultMode

Default run mode:

| Value | Description |
|-------|-------------|
| `plan` | Read-only analysis mode — no file writes, sandboxed |
| `agent` | Standard read/write mode (default) — Bash requires approval |
| `yolo` | Full access mode — all tools auto-execute |

```json
{ "defaultMode": "agent" }
```

### defaultThinkingLevel

Default thinking level for reasoning models:

| Value | Description |
|-------|-------------|
| `off` | Disable thinking |
| `minimal` | Minimal thinking |
| `low` | Low level |
| `medium` | Medium level (default) |
| `high` | High level |
| `xhigh` | Highest level |

```json
{ "defaultThinkingLevel": "medium" }
```

### enablePlanTool

Whether to register the built-in `plan` tool that allows the agent to create and track structured task plans.

```json
{ "enablePlanTool": true }
```

Set to `false` to disable it (e.g., if you prefer the agent not to use structured plans).

### maxContextTokens

Override the maximum context token count. When set to `0` (default), the value is derived from the model's `contextWindow`.

```json
{ "maxContextTokens": 200000 }
```

### maxOutputTokens

Override the maximum output token count. When set to `0` (default), the value is derived from the model's `maxTokens`.

```json
{ "maxOutputTokens": 16384 }
```

---

### contextFiles

Context file loading settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether to automatically load context files |
| `extraFiles` | []string | `[]` | Additional context file paths to load |

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

#### Auto-loaded Context Files

VibeCoding automatically searches for and loads the following files:

1. **Global files** (in the global config directory):
   - `AGENTS.md`
   - `CLAUDE.md`

2. **Project files** (searched upward from current directory):
   - `AGENTS.md`
   - `CLAUDE.md`
   - `.vibe/AGENTS.md`
   - `.vibe/CLAUDE.md`

---

### skillsDir

Path to the global skills directory. Supports `~` expansion.

| Platform | Default |
|----------|---------|
| Linux/macOS | `~/.vibecoding/skills` |
| Windows | `%APPDATA%\vibecoding\skills` |

```json
{ "skillsDir": "~/.vibecoding/skills" }
```

Skills are loaded from:
- **Global skills**: `<skillsDir>/<name>/SKILL.md`
- **Project skills**: `.skills/<name>/SKILL.md` (override global)

---

### compaction

Context compaction (compression) configuration for managing long conversations. When the context window fills up, VibeCoding can automatically summarize older messages to keep the conversation going.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable automatic context compaction |
| `reserveTokens` | int | `16384` | Tokens reserved for the model's response |
| `keepRecentTokens` | int | `20000` | Recent message tokens to keep uncompacted |
| `idleCompressionEnabled` | bool | `false` | Enable proactive compression during idle periods |
| `idleTimeoutSeconds` | int | `90` | Seconds of user inactivity before idle compression triggers |
| `idleMinTokensForCompress` | int | `150000` | Minimum context tokens before idle compression is worthwhile |

```json
{
  "compaction": {
    "enabled": true,
    "reserveTokens": 16384,
    "keepRecentTokens": 20000,
    "idleCompressionEnabled": true,
    "idleTimeoutSeconds": 90,
    "idleMinTokensForCompress": 150000
  }
}
```

#### Idle Compression

When enabled, VibeCoding proactively compresses the context during periods of inactivity (e.g., while you're reading output or thinking about your next prompt). This reduces latency for your next request because the context is already smaller.

- **`idleCompressionEnabled`**: Off by default. Turn it on if you frequently have long conversations.
- **`idleTimeoutSeconds`**: How long VibeCoding waits after the last interaction before triggering idle compression. Default: 90 seconds.
- **`idleMinTokensForCompress`**: Idle compression only triggers if the current context exceeds this threshold. Default: 150,000 tokens.

---

### sandbox

Sandbox configuration for secure command execution. Uses [bubblewrap (bwrap)](https://github.com/containers/bubblewrap) on Linux.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable sandboxed execution |
| `level` | string | `"none"` | Sandbox level: `"none"`, `"standard"`, `"strict"` |
| `bwrapPath` | string | `""` (auto) | Custom path to the `bwrap` binary |
| `allowNetwork` | bool | `false` | Allow network access inside sandbox |
| `allowedRead` | []string | *(platform-specific)* | Paths readable inside the sandbox |
| `allowedWrite` | []string | `[]` | Additional paths writable inside the sandbox |
| `deniedPaths` | []string | *(platform-specific)* | Paths explicitly denied inside the sandbox |
| `passEnv` | []string | *(platform-specific)* | Environment variables passed into the sandbox |
| `tmpSize` | string | `"100m"` | Size limit for the sandbox's `/tmp` tmpfs mount |

```json
{
  "sandbox": {
    "enabled": true,
    "level": "standard",
    "bwrapPath": "/usr/bin/bwrap",
    "allowNetwork": false,
    "allowedRead": ["/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc/ssl"],
    "allowedWrite": ["/tmp/my-build"],
    "deniedPaths": ["/etc/shadow", "/root"],
    "passEnv": ["PATH", "HOME", "USER", "LANG", "TERM", "SHELL", "GOPATH"],
    "tmpSize": "200m"
  }
}
```

#### Sandbox Levels

| Level | File System | Network | Use Case |
|-------|------------|---------|----------|
| `none` | Full access | ✓ | No sandboxing (YOLO mode default) |
| `standard` | Project read-write | ✗ | Everyday development (Agent mode) |
| `strict` | Project read-only | ✗ | Code review / analysis (Plan mode) |

#### Platform Defaults for allowedRead

**Linux:**
```json
["/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc/ld.so.cache", "/etc/ssl", "/etc/ca-certificates", "/dev/null", "/dev/urandom", "/dev/zero", "/proc/self", "/proc/meminfo", "/proc/cpuinfo"]
```

**macOS:**
```json
["/usr", "/lib", "/bin", "/sbin", "/System", "/Library"]
```

**Windows:**
```json
["C:\\Windows", "C:\\Program Files", "C:\\Program Files (x86)"]
```

#### Platform Defaults for deniedPaths

**Linux / macOS:**
```json
["/etc/shadow", "/etc/gshadow", "/etc/passwd", "/root", "/home"]
```

**Windows:**
```json
["C:\\Users\\<Username>\\Documents", "C:\\Users\\<Username>\\Desktop"]
```

#### Platform Defaults for passEnv

**All platforms:** `PATH`, `HOME`, `USER`, `LANG`, `LC_ALL`, `TERM`

**Linux additionally:** `SHELL`, `GOPATH`, `GOROOT`, `GOPROXY`, `GOMODCACHE`, `NODE_PATH`

**macOS additionally:** `SHELL`, `TMPDIR`

**Windows additionally:** `APPDATA`, `LOCALAPPDATA`, `COMPUTERNAME`, `USERPROFILE`, `SYSTEMROOT`

---

### sessionDir

Directory for storing session files (JSONL format). Supports `~` expansion.

| Platform | Default |
|----------|---------|
| Linux/macOS | `~/.vibecoding/sessions` |
| Windows | `%APPDATA%\vibecoding\sessions` |

```json
{ "sessionDir": "~/.vibecoding/sessions" }
```

---

### shellPath

Custom shell path for the Bash tool. When empty (default), VibeCoding uses the platform default:

| Platform | Default |
|----------|---------|
| Linux | `$SHELL` or `/bin/bash` |
| macOS | `$SHELL` or `/bin/zsh` |
| Windows | `powershell.exe` or `cmd.exe` |

```json
{ "shellPath": "/usr/bin/fish" }
```

### shellCommandPrefix

A string prepended to every shell command before execution. Useful for setting up environment or activating virtualenvs.

```json
{ "shellCommandPrefix": "source ~/.venv/bin/activate && " }
```

When empty (default), commands are executed directly.

---

### theme

UI color theme for the terminal interface.

| Value | Description |
|-------|-------------|
| `"dark"` | Dark background theme (default) |
| `"light"` | Light background theme |

```json
{ "theme": "dark" }
```

---

### retry

API call retry configuration with exponential backoff. Retries apply to the initial HTTP connection phase only (once SSE streaming begins, it is not retried).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable automatic retries on transient API errors |
| `maxRetries` | int | `3` | Maximum number of retry attempts |
| `baseDelayMs` | int | `2000` | Base delay in milliseconds (doubles on each retry) |

```json
{
  "retry": {
    "enabled": true,
    "maxRetries": 3,
    "baseDelayMs": 2000
  }
}
```

#### Retryable Errors

The following errors trigger automatic retries:

| Category | Examples |
|----------|----------|
| Rate limiting | HTTP 429 |
| Server errors | HTTP 502, 503, 504 |
| Network errors | connection refused, connection reset, DNS errors |
| Timeouts | HTTP client timeout, TCP timeout |

The following are **not** retried:
- Context cancellation (user pressed Ctrl+C)
- HTTP 4xx client errors (except 429): 400, 401, 403, 404
- Successful connections that fail mid-stream

#### Backoff Strategy

Each retry waits `baseDelayMs × 2^attempt` milliseconds, capped at 30 seconds:

| Attempt | Delay (base=2000ms) |
|---------|--------------------|
| 1st | 2s |
| 2nd | 4s |
| 3rd | 8s |

When a retry occurs, VibeCoding displays a status message in the TUI:
```
Retrying (1/3): request timed out — waiting 2.0s...
Retrying (2/3): rate limited (HTTP 429) — waiting 4.0s...
```

#### Disabling Retries

```json
{
  "retry": {
    "enabled": false
  }
}
```

---

### approval

Agent mode approval configuration. Controls which Bash commands auto-execute and which require user confirmation.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `bashWhitelist` | []string | *(see below)* | Command prefixes that auto-approve in agent mode |
| `bashBlacklist` | []string | `[]` | Command prefixes that **always** require approval |
| `confirmBeforeWrite` | bool | `true` | Require user approval before `Write`/`Edit` tools run in agent mode |

#### Default Whitelist

```json
["go ", "make ", "git ", "npm ", "yarn ", "node ", "python ", "pip "]
```

#### Approval Flow

```
Agent requests tool execution
│
▼
Check mode
├─ Plan mode → Deny (read-only)
├─ Agent mode → Continue checking
└─ YOLO mode → Auto-approve unless blacklisted
│
▼
Blacklist check (highest priority):
├─ Command matches blacklist → Require user approval
└─ Otherwise continue
│
▼
In Agent mode:
├─ Write/Edit tool + confirmBeforeWrite=true → Require user approval
├─ Non-Bash tool → Auto-approve
├─ Command matches whitelist → Auto-approve
└─ Otherwise → Require user approval
│
▼
In --print mode:
  Commands that would need approval → Fail immediately
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

**Disable write confirmation (trust the agent):**
```json
{
  "approval": {
    "confirmBeforeWrite": false
  }
}
```

---

## MCP Configuration

MCP servers are configured in standalone `mcp.json` files, not in `settings.json`.

VibeCoding loads MCP configuration at startup from:

1. Global config: `~/.vibecoding/mcp.json` on Linux/macOS, or `%APPDATA%\vibecoding\mcp.json` on Windows
2. Project config: `.vibe/mcp.json`

Create a template from the TUI:

```text
/init_mcp project full
/init_mcp global basic
/mcps
```

Example:

```json
{
  "mcpServers": [
    {
      "name": "local-tools",
      "type": "stdio",
      "command": "/absolute/path/to/mcp-server",
      "args": ["--port", "8080"],
      "env": [
        {"name": "API_KEY", "value": "sk-..."}
      ]
    },
    {
      "name": "remote-tools",
      "type": "http",
      "url": "https://mcp.example.com",
      "headers": [
        {"name": "Authorization", "value": "Bearer token"}
      ]
    }
  ]
}
```

Supported transports:

- `stdio`: requires an absolute `command` path
- `http`: streamable HTTP endpoint via `url`
- `sse`: legacy SSE stream via `url` plus `messageUrl`

MCP tools are registered after built-in tools and `skill_ref`, but before the agent is created. The agent freezes its system prompt and tool definitions for the session, so changes to `mcp.json` require restarting the client.

Tool names use `mcp_<server_name>_<tool_name>`. If a name already exists, VibeCoding appends a numeric suffix instead of replacing an existing tool. Starter-template placeholders such as `/absolute/path/to/mcp-server`, `example.com`, and `replace-me` are ignored during automatic startup loading.

---

## Authentication Configuration

VibeCoding supports multiple ways to provide API keys, with flexible resolution logic.

### Key Resolution Order

When VibeCoding needs the API key for a provider, it checks in this order:

1. **Provider `apiKey` field** in `settings.json` — if set, resolved using the rules below
2. **Derived environment variable** — provider name is converted to an env var: e.g., `deepseek-openai` → `DEEPSEEK_OPENAI_API_KEY`

### apiKey Field Formats

The `apiKey` field in a provider config supports three formats:

| Format | Example | Behavior |
|--------|---------|----------|
| `${VAR}` | `"${DEEPSEEK_API_KEY}"` | Reads the value of environment variable `VAR` |
| `!command` | `"!pass show deepseek-key"` | Executes a shell command and uses its stdout only when `VIBECODING_ALLOW_SHELL_CONFIG=1` |
| Plain string | `"sk-abc123..."` | Used as-is (⚠️ not recommended for shared configs) |

#### Environment Variable Reference

```json
{
  "providers": {
    "deepseek-openai": {
      "apiKey": "${DEEPSEEK_API_KEY}"
    }
  }
}
```

Then set the environment variable:

```bash
export DEEPSEEK_API_KEY=sk-...
```

#### Shell Command (Password Manager Integration)

Prefix with `!` to run a shell command. VibeCoding uses `sh -c` on Linux/macOS and `powershell.exe` on Windows.

Shell command resolution is disabled by default. To enable it for trusted local configuration, set:

```bash
export VIBECODING_ALLOW_SHELL_CONFIG=1
```

```json
{
  "providers": {
    "anthropic": {
      "apiKey": "!pass show api/anthropic"
    },
    "openai": {
      "apiKey": "!security find-generic-password -s openai-api -w"
    }
  }
}
```

This is useful for integrating with password managers like `pass`, `1password-cli`, macOS Keychain, or any other secret store.

#### Derived Environment Variable Fallback

If no `apiKey` is configured for a provider, VibeCoding derives an environment variable name from the provider name:

| Provider Name | Derived Env Var |
|---------------|-----------------|
| `deepseek-openai` | `DEEPSEEK_OPENAI_API_KEY` |
| `deepseek-anthropic` | `DEEPSEEK_ANTHROPIC_API_KEY` |
| `my-custom-provider` | `MY_CUSTOM_PROVIDER_API_KEY` |
| `anthropic` | `ANTHROPIC_API_KEY` |
| `openai` | `OPENAI_API_KEY` |

The rule: replace `-` with `_`, uppercase everything, append `_API_KEY`.

### Authentication Examples

**Option 1: Environment Variables (simplest)**

```bash
export DEEPSEEK_API_KEY=sk-...
```

With default config, VibeCoding will look for `DEEPSEEK_OPENAI_API_KEY` for the `deepseek-openai` provider. But if the provider's `apiKey` is set to `${DEEPSEEK_API_KEY}`, it reads that env var instead.

**Option 2: Inline in Configuration File**

```json
{
  "providers": {
    "deepseek-openai": {
      "apiKey": "sk-..."
    }
  }
}
```

**Option 3: Password Manager**

```json
{
  "providers": {
    "deepseek-openai": {
      "apiKey": "!pass show deepseek"
    }
  }
}
```

---

## Environment Variable Overrides

These environment variables override settings at runtime:

| Environment Variable | Overrides | Example |
|---------------------|-----------|---------|
| `VIBECODING_DIR` | Global config directory | `export VIBECODING_DIR=/custom/config` |
| `VIBECODING_PROVIDER` | `defaultProvider` | `export VIBECODING_PROVIDER=anthropic` |
| `VIBECODING_MODEL` | `defaultModel` | `export VIBECODING_MODEL=claude-sonnet-4-20250514` |
| `VIBECODING_MODE` | `defaultMode` | `export VIBECODING_MODE=yolo` |
| `VIBECODING_THINKING` | `defaultThinkingLevel` | `export VIBECODING_THINKING=high` |
| `VIBECODING_DEBUG` | Enable provider-level request/response debug output | `export VIBECODING_DEBUG=1` |

---

## Configuration Examples

### Minimal Configuration

Only need to set the default provider and model. Everything else uses sensible defaults.

```json
{
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash"
}
```

### Multi-Provider Configuration

Switch between providers at runtime using `/provider` or `--provider`:

```json
{
  "providers": {
    "deepseek-anthropic": {
      "vendor": "deepseek",
      "baseUrl": "https://api.deepseek.com/anthropic",
      "apiKey": "${DEEPSEEK_API_KEY}",
      "api": "anthropic-messages"
    },
    "deepseek-openai": {
      "vendor": "deepseek",
      "baseUrl": "https://api.deepseek.com",
      "apiKey": "${DEEPSEEK_API_KEY}",
      "api": "openai-chat"
    },
    "anthropic": {
      "vendor": "anthropic",
      "baseUrl": "https://api.anthropic.com",
      "apiKey": "${ANTHROPIC_API_KEY}",
      "api": "anthropic-messages",
      "cacheControl": true,
      "models": [
        {
          "id": "claude-sonnet-4-20250514",
          "name": "Claude Sonnet 4",
          "contextWindow": 200000,
          "maxTokens": 8192,
          "cost": { "input": 3, "output": 15, "cacheRead": 0.3, "cacheWrite": 3.75 }
        }
      ]
    }
  },
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash"
}
```

### Custom API Endpoint / HTTP Proxy / Headers

`baseUrl` points to an API endpoint or API gateway. `httpProxy` configures the network proxy used only by that provider's HTTP client. When `httpProxy` is empty, the provider keeps Go's default `HTTP_PROXY` / `HTTPS_PROXY` environment behavior.

Use `headers` to attach custom HTTP headers to every request for a provider. Header values support the same resolution rules as `apiKey`, including `${ENV_VAR}` and opt-in `!cmd` shell commands. Custom headers are applied after VibeCoding's default provider headers, so they can also override defaults such as `Authorization`, `x-api-key`, or gateway-specific headers when needed.

```json
{
  "providers": {
    "my-proxy": {
      "baseUrl": "https://my-proxy.example.com/v1",
      "api": "openai-chat",
      "apiKey": "${MY_PROXY_API_KEY}",
      "httpProxy": "http://127.0.0.1:7890",
      "headers": {
        "X-Gateway-Token": "${MY_GATEWAY_TOKEN}",
        "X-Request-Source": "vibecoding"
      },
      "models": [
        {
          "id": "gpt-4o",
          "name": "GPT-4o (via proxy)",
          "contextWindow": 128000,
          "maxTokens": 16384
        }
      ]
    }
  },
  "defaultProvider": "my-proxy",
  "defaultModel": "gpt-4o"
}
```

### Enable Sandbox with Custom Paths

```json
{
  "sandbox": {
    "enabled": true,
    "level": "standard",
    "allowNetwork": false,
    "allowedRead": ["/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc/ssl", "/opt/go"],
    "passEnv": ["PATH", "HOME", "USER", "LANG", "TERM", "SHELL", "GOPATH", "GOROOT"],
    "tmpSize": "200m"
  }
}
```

### Enable Idle Compression for Long Sessions

```json
{
  "compaction": {
    "enabled": true,
    "reserveTokens": 16384,
    "keepRecentTokens": 20000,
    "idleCompressionEnabled": true,
    "idleTimeoutSeconds": 60,
    "idleMinTokensForCompress": 100000
  }
}
```

### Project-Level Override

Place in `.vibe/settings.json` to override specific settings for a project:

```json
{
  "defaultMode": "yolo",
  "defaultThinkingLevel": "high",
  "shellCommandPrefix": "source .venv/bin/activate && ",
  "approval": {
    "bashWhitelist": ["python ", "pytest ", "pip ", "make "],
    "confirmBeforeWrite": false
  }
}
```

This merges with your global settings — only the fields you specify are overridden.
