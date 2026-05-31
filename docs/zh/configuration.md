# 配置详解

## 配置文件位置

VibeCoding 使用两个配置文件:

| 文件 | 平台 | 范围 | 优先级 |
|------|------|------|--------|
| `~/.vibecoding/settings.json` | Linux | 全局 (所有项目) | 低 |
| `~/Library/Application Support/vibecoding/settings.json` | macOS | 全局 (所有项目) | 低 |
| `%APPDATA%\vibecoding\settings.json` | Windows | 全局 (所有项目) | 低 |
| `.vibe/settings.json` | 全部 | 项目级 | 高 |

> **提示:** 可以通过 `VIBECODING_DIR` 环境变量覆盖全局配置目录。

> **Windows 用户：** `%APPDATA%` 实际展开为 `C:\Users\<用户名>\AppData\Roaming`，所以完整路径通常是 `C:\Users\<用户名>\AppData\Roaming\vibecoding\settings.json`。

项目级配置会覆盖全局配置。当两者同时存在时，标量字段会被项目配置覆盖；`providers` 是按 key 做深度合并的（项目中的 provider 会被添加到全局 providers 或替换同名的 provider，而不是替换整个 map）。

## 配置结构

### 完整示例

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

## 所有配置字段

### 顶层字段速查表

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `providers` | object | *(见下文)* | 提供商配置 (以名称为 key) |
| `defaultProvider` | string | `"deepseek-openai"` | 默认使用的提供商 |
| `defaultModel` | string | `"deepseek-v4-flash"` | 默认使用的模型 ID |
| `defaultMode` | string | `"agent"` | 默认运行模式: `plan`, `agent`, `yolo` |
| `defaultThinkingLevel` | string | `"medium"` | 默认思考级别 |
| `enablePlanTool` | bool | `true` | 是否注册内置 `plan` 工具 |
| `maxContextTokens` | int | `0` (自动) | 覆盖最大上下文 token 数 |
| `maxOutputTokens` | int | `0` (自动) | 覆盖最大输出 token 数 |
| `contextFiles` | object | *(见下文)* | 上下文文件加载设置 |
| `skillsDir` | string | `"~/.vibecoding/skills"` | 全局技能目录路径 |
| `compaction` | object | *(见下文)* | 上下文压缩设置 |
| `sandbox` | object | *(见下文)* | 沙箱执行设置 |
| `sessionDir` | string | `"~/.vibecoding/sessions"` | 会话文件存储目录 |
| `shellPath` | string | `""` (自动) | 自定义 Bash 工具的 shell 路径 |
| `shellCommandPrefix` | string | `""` | 每条 shell 命令前自动追加的前缀 |
| `theme` | string | `"dark"` | UI 主题: `"dark"` 或 `"light"` |
| `retry` | object | *(见下文)* | API 调用重试设置 |
| `approval` | object | *(见下文)* | Bash 命令审批设置 |

---

## 配置项详解

### providers

多提供商配置。每个提供商是一个以用户自定义名称为 key 的对象:

| 字段 | 类型 | 必填 | 默认值 | 描述 |
|------|------|------|--------|------|
| `baseUrl` | string | ✓ | — | API 基础 URL |
| `vendor` | string | — | 自动检测 | 可选厂商适配器名称 (见下文) |
| `apiKey` | string | — | `""` | API 密钥 (见[认证配置](#认证配置)) |
| `api` | string | — | 自动检测 | API 协议: `"openai-chat"` 或 `"anthropic-messages"` |
| `thinkingFormat` | string | — | 自动检测 | 思考参数格式 (见下文) |
| `cacheControl` | bool | — | `false` | 启用 Anthropic 提示缓存；使用 Claude 模型时设为 `true` |
| `models` | array | — | `[]` | 可用模型列表 |

#### vendor 字段

`vendor` 字段用于选择厂商适配器，不改变现有 provider 配置 schema。该字段可选；未设置时，VibeCoding 会先根据 `baseUrl` 自动识别厂商，再根据 `api` 回退到通用协议 provider。

选择顺序：

1. 显式 `vendor`
2. `baseUrl` 自动识别
3. 通用 fallback：`openai-chat` 或 `anthropic-messages`

内置厂商适配器包括 `openai`、`anthropic`、`claude`、`deepseek`、`xiaomi`、`xiaomi-token-plan-ams`、`xiaomi-token-plan-cn`、`xiaomi-token-plan-sgp`、`kimi`、`minimax`、`seed`、`qianfan`、`bailian`、`gitee`、`openrouter`、`together`、`groq` 和 `fireworks`。

```json
{
  "providers": {
    "custom-deepseek": {
      "vendor": "deepseek",
      "baseUrl": "https://api.deepseek.com",
      "apiKey": "${DEEPSEEK_API_KEY}",
      "api": "openai-chat",
      "models": [
        { "id": "deepseek-v4-flash", "name": "DeepSeek-V4-Flash", "contextWindow": 1000000 }
      ]
    }
  }
}
```

#### api 字段

`api` 字段指定的是**协议格式**，而非服务商。你可以将任意提供商指向任意兼容的端点：

- `openai-chat`: OpenAI Chat Completions API 格式
- `anthropic-messages`: Anthropic Messages API 格式

例如，DeepSeek 在不同端点提供两种格式，你也可以用这些格式去连接真正的 OpenAI 或 Anthropic 服务。

如果未指定，会根据 `baseUrl` 自动检测：
- 包含 "anthropic" → `anthropic-messages`
- 其他 → `openai-chat`

#### thinkingFormat 字段

指定思考/推理参数如何发送到 API：

| 值 | 行为 |
|----|------|
| `""` (空) | 根据 URL 自动检测 |
| `"openai"` | 使用 OpenAI `reasoning_effort` 格式 |
| `"anthropic"` | 使用 Anthropic `thinking` 带 `budget_tokens` |
| `"deepseek"` | 使用 DeepSeek `thinking: {type: "enabled"}` + `reasoning_effort` (OpenAI) 或 `output_config.effort` (Anthropic) |
| `"xiaomi"` | 旧的 thinking-only 格式: `thinking: {type: "enabled"}` |

未设置时自动检测：
- URL 包含 `deepseek` → `"deepseek"`
- URL 包含 `xiaomimimo` → `"xiaomi"`

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

#### cacheControl 字段

启用 Anthropic 风格的提示缓存 (Prompt Caching)。设为 `true` 时，VibeCoding 会在请求中添加缓存控制头。**使用 Claude 模型接入 Anthropic API 时应启用此选项**，可降低费用和延迟。

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

#### models 数组

每个模型字段:

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `id` | string | — | 发送到 API 的模型 ID |
| `name` | string | — | 人类可读的显示名称 |
| `reasoning` | bool | `false` | 是否支持思考/推理 |
| `contextWindow` | int | `0` | 上下文窗口大小 (token) |
| `maxTokens` | int | `0` | 每次响应的最大输出 token |
| `input` | []string | `[]` | 支持的输入模态: `"text"`, `"image"` |
| `cost` | object | `null` | 每百万 token 定价 |
| `compat` | object | `null` | 模型级兼容标志，用于处理 provider 差异 |

`cost` 对象:

| 字段 | 类型 | 描述 |
|------|------|------|
| `input` | float | 每百万输入 token 费用 |
| `output` | float | 每百万输出 token 费用 |
| `cacheRead` | float | 每百万缓存读取 token 费用 (Anthropic) |
| `cacheWrite` | float | 每百万缓存写入 token 费用 (Anthropic) |

`compat` 对象可选，仅在某个模型需要协议兼容调整时设置：

| 字段 | 类型 | 描述 |
|------|------|------|
| `thinkingFormat` | string | 覆盖模型 thinking 格式（`openai`、`deepseek`、`xiaomi`、`anthropic` 等） |
| `requiresReasoningContentOnAssistant` | bool | 回放 assistant 消息时发送空 `reasoning_content` |
| `requiresReasoningContentOnAssistantMessages` | bool | 参考实现中的别名，与上一项等价 |
| `forceAdaptiveThinking` | bool | 强制使用 Anthropic adaptive thinking 格式 |
| `supportsReasoningEffort` | bool | 模型是否接受 `reasoning_effort` |
| `maxTokensField` | string | 使用 `max_tokens` 或 `max_completion_tokens` |
| `supportsDeveloperRole` | bool | 是否支持 developer role 消息 |
| `supportsStore` | bool | 是否支持 OpenAI `store` |
| `supportsStrictMode` | bool | 是否支持严格工具 schema |
| `supportsCacheControlOnTools` | bool | 是否支持在工具定义上使用 cache control |
| `supportsLongCacheRetention` | bool | 是否支持长 prompt cache retention |
| `sendSessionAffinityHeaders` | bool | 是否发送 session affinity headers |
| `supportsEagerToolInputStreaming` | bool | 是否支持 Anthropic eager tool input streaming |

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

默认使用的提供商名称。必须对应 `providers` 中的一个 key。

```json
{ "defaultProvider": "deepseek-openai" }
```

### defaultModel

默认使用的模型 ID。必须对应所选提供商 `models` 列表中的一个 `id`。

```json
{ "defaultModel": "deepseek-v4-flash" }
```

### defaultMode

默认运行模式:

| 值 | 描述 |
|----|------|
| `plan` | 只读分析模式 — 无文件写入，有沙箱 |
| `agent` | 标准读写模式 (默认) — Bash 需要审批 |
| `yolo` | 完全访问模式 — 所有工具自动执行 |

```json
{ "defaultMode": "agent" }
```

### defaultThinkingLevel

默认思考级别:

| 值 | 描述 |
|----|------|
| `off` | 关闭思考 |
| `minimal` | 最小思考 |
| `low` | 低级别 |
| `medium` | 中等级别 (默认) |
| `high` | 高级别 |
| `xhigh` | 最高级别 |

```json
{ "defaultThinkingLevel": "medium" }
```

### enablePlanTool

是否注册内置 `plan` 工具，允许 agent 创建和跟踪结构化任务计划。

```json
{ "enablePlanTool": true }
```

设为 `false` 可禁用（例如不希望 agent 使用结构化计划）。

### maxContextTokens

覆盖最大上下文 token 数。设为 `0` (默认) 时，根据模型的 `contextWindow` 自动确定。

```json
{ "maxContextTokens": 200000 }
```

### maxOutputTokens

覆盖最大输出 token 数。设为 `0` (默认) 时，根据模型的 `maxTokens` 自动确定。

```json
{ "maxOutputTokens": 16384 }
```

---

### contextFiles

上下文文件加载设置。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 是否自动加载上下文文件 |
| `extraFiles` | []string | `[]` | 额外的上下文文件路径 |

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

#### 自动加载的上下文文件

VibeCoding 会自动搜索并加载以下文件:

1. **全局文件** (在全局配置目录中):
   - `AGENTS.md`
   - `CLAUDE.md`

2. **项目文件** (从当前目录向上搜索):
   - `AGENTS.md`
   - `CLAUDE.md`
   - `.vibe/AGENTS.md`
   - `.vibe/CLAUDE.md`

---

### skillsDir

全局技能目录路径。支持 `~` 展开。

| 平台 | 默认值 |
|------|--------|
| Linux | `~/.vibecoding/skills` |
| macOS | `~/Library/Application Support/vibecoding/skills` |
| Windows | `%APPDATA%\vibecoding\skills` |

```json
{ "skillsDir": "~/.vibecoding/skills" }
```

技能加载位置：
- **全局技能**: `<skillsDir>/<name>/SKILL.md`
- **项目技能**: `.skills/<name>/SKILL.md` (覆盖全局)

---

### compaction

上下文压缩配置，用于管理长对话。当上下文窗口快满时，VibeCoding 会自动总结较旧的消息以继续对话。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用自动上下文压缩 |
| `reserveTokens` | int | `16384` | 为模型响应保留的 token |
| `keepRecentTokens` | int | `20000` | 保留的最近消息 token 数 |
| `idleCompressionEnabled` | bool | `false` | 启用空闲期间主动压缩 |
| `idleTimeoutSeconds` | int | `90` | 用户空闲多少秒后触发空闲压缩 |
| `idleMinTokensForCompress` | int | `150000` | 空闲压缩的最低上下文 token 阈值 |

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

#### 空闲压缩

启用后，VibeCoding 会在用户空闲期间（例如阅读输出或思考下一个提示时）主动压缩上下文。这可以减少下一次请求的延迟，因为上下文已经变小了。

- **`idleCompressionEnabled`**: 默认关闭。如果你经常进行长对话，建议开启。
- **`idleTimeoutSeconds`**: 上次交互后等待多久触发空闲压缩。默认 90 秒。
- **`idleMinTokensForCompress`**: 只有当前上下文超过此阈值时才会触发空闲压缩。默认 150,000 token。

---

### sandbox

沙箱执行配置。在 Linux 上使用 [bubblewrap (bwrap)](https://github.com/containers/bubblewrap)。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `false` | 启用沙箱执行 |
| `level` | string | `"none"` | 沙箱级别: `"none"`, `"standard"`, `"strict"` |
| `bwrapPath` | string | `""` (自动) | 自定义 `bwrap` 二进制文件路径 |
| `allowNetwork` | bool | `false` | 沙箱内是否允许网络访问 |
| `allowedRead` | []string | *(平台默认)* | 沙箱内可读路径 |
| `allowedWrite` | []string | `[]` | 沙箱内额外可写路径 |
| `deniedPaths` | []string | *(平台默认)* | 沙箱内明确禁止访问的路径 |
| `passEnv` | []string | *(平台默认)* | 传入沙箱的环境变量 |
| `tmpSize` | string | `"100m"` | 沙箱 `/tmp` tmpfs 挂载的大小限制 |

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

#### 沙箱级别

| 级别 | 文件系统 | 网络 | 用途 |
|------|---------|------|------|
| `none` | 完全访问 | ✓ | 无沙箱 (YOLO 模式默认) |
| `standard` | 项目可读写 | ✗ | 日常开发 (Agent 模式) |
| `strict` | 项目只读 | ✗ | 代码审查/分析 (Plan 模式) |

#### allowedRead 平台默认值

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

#### deniedPaths 平台默认值

**Linux / macOS:**
```json
["/etc/shadow", "/etc/gshadow", "/etc/passwd", "/root", "/home"]
```

**Windows:**
```json
["C:\\Users\\<用户名>\\Documents", "C:\\Users\\<用户名>\\Desktop"]
```

#### passEnv 平台默认值

**所有平台:** `PATH`, `HOME`, `USER`, `LANG`, `LC_ALL`, `TERM`

**Linux 额外:** `SHELL`, `GOPATH`, `GOROOT`, `GOPROXY`, `GOMODCACHE`, `NODE_PATH`

**macOS 额外:** `SHELL`, `TMPDIR`

**Windows 额外:** `APPDATA`, `LOCALAPPDATA`, `COMPUTERNAME`, `USERPROFILE`, `SYSTEMROOT`

---

### sessionDir

会话文件 (JSONL 格式) 存储目录。支持 `~` 展开。

| 平台 | 默认值 |
|------|--------|
| Linux | `~/.vibecoding/sessions` |
| macOS | `~/Library/Application Support/vibecoding/sessions` |
| Windows | `%APPDATA%\vibecoding\sessions` |

```json
{ "sessionDir": "~/.vibecoding/sessions" }
```

---

### shellPath

自定义 Bash 工具使用的 shell 路径。为空 (默认) 时使用平台默认值：

| 平台 | 默认值 |
|------|--------|
| Linux | `$SHELL` 或 `/bin/bash` |
| macOS | `$SHELL` 或 `/bin/zsh` |
| Windows | `powershell.exe` 或 `cmd.exe` |

```json
{ "shellPath": "/usr/bin/fish" }
```

### shellCommandPrefix

每条 shell 命令执行前自动追加的前缀字符串。适用于设置环境或激活虚拟环境。

```json
{ "shellCommandPrefix": "source ~/.venv/bin/activate && " }
```

为空 (默认) 时直接执行命令。

---

### theme

终端界面的 UI 颜色主题。

| 值 | 描述 |
|----|------|
| `"dark"` | 深色背景主题 (默认) |
| `"light"` | 浅色背景主题 |

```json
{ "theme": "dark" }
```

---

### retry

API 调用重试配置，使用指数退避策略。重试仅适用于初始 HTTP 连接阶段（一旦 SSE 流开始，不会重试）。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 遇到瞬态 API 错误时自动重试 |
| `maxRetries` | int | `3` | 最大重试次数 |
| `baseDelayMs` | int | `2000` | 基础延迟 (毫秒)，每次重试翻倍 |

```json
{
  "retry": {
    "enabled": true,
    "maxRetries": 3,
    "baseDelayMs": 2000
  }
}
```

#### 可重试的错误

以下错误会触发自动重试：

| 类别 | 示例 |
|------|------|
| 速率限制 | HTTP 429 |
| 服务器错误 | HTTP 502, 503, 504 |
| 网络错误 | 连接被拒绝、连接重置、DNS 错误 |
| 超时 | HTTP 客户端超时、TCP 超时 |

以下情况**不会**重试：
- 上下文取消（用户按了 Ctrl+C）
- HTTP 4xx 客户端错误（除 429 外）：400、401、403、404
- 连接成功后流中断的错误

#### 退避策略

每次重试等待 `baseDelayMs × 2^attempt` 毫秒，上限 30 秒：

| 次数 | 延迟 (base=2000ms) |
|------|--------------------|
| 第 1 次 | 2 秒 |
| 第 2 次 | 4 秒 |
| 第 3 次 | 8 秒 |

发生重试时，VibeCoding 会在 TUI 中显示状态消息：
```
Retrying (1/3): request timed out — waiting 2.0s...
Retrying (2/3): rate limited (HTTP 429) — waiting 4.0s...
```

#### 禁用重试

```json
{
  "retry": {
    "enabled": false
  }
}
```

---

### approval

Agent 模式审批配置。控制哪些 Bash 命令自动执行，哪些需要用户确认。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `bashWhitelist` | []string | *(见下文)* | agent 模式下自动批准的命令前缀列表 |
| `bashBlacklist` | []string | `[]` | **始终**需要审批的命令前缀列表 |
| `confirmBeforeWrite` | bool | `true` | agent 模式下 `Write`/`Edit` 工具执行前需要用户确认 |

#### 默认白名单

```json
["go ", "make ", "git ", "npm ", "yarn ", "node ", "python ", "pip "]
```

#### 审批流程

```
Agent 请求执行工具
│
▼
检查模式
├─ Plan 模式 → 拒绝 (只读)
├─ Agent 模式 → 继续检查
└─ YOLO 模式 → 自动批准（除非命中黑名单）
│
▼
黑名单检查（最高优先级）：
├─ 命令匹配黑名单 → 需要用户审批
└─ 否则继续
│
▼
Agent 模式下：
├─ Write/Edit 工具 + confirmBeforeWrite=true → 需要用户审批
├─ 非 Bash 工具 → 自动批准
├─ 命令匹配白名单 → 自动批准
└─ 其他 → 需要用户审批
│
▼
在 --print 模式下：
  本应触发审批的命令 → 直接报错退出
```

#### 示例配置

**仅允许 git 和 npm:**
```json
{
  "approval": {
    "bashWhitelist": ["git ", "npm "]
  }
}
```

**自定义黑名单:**
```json
{
  "approval": {
    "bashWhitelist": ["go ", "make ", "git "],
    "bashBlacklist": ["rm -rf", "sudo", "dd "]
  }
}
```

**禁用写入确认 (信任 agent):**
```json
{
  "approval": {
    "confirmBeforeWrite": false
  }
}
```

---

## MCP 配置

MCP 服务器配置保存在独立的 `mcp.json` 文件中，不写入 `settings.json`。

VibeCoding 启动时会从以下位置加载 MCP 配置：

1. 全局配置：Linux 为 `~/.vibecoding/mcp.json`，macOS 为 `~/Library/Application Support/vibecoding/mcp.json`，Windows 为 `%APPDATA%\vibecoding\mcp.json`
2. 项目配置：`.vibe/mcp.json`

可在 TUI 中创建模板：

```text
/init_mcp project full
/init_mcp global basic
/mcps
```

示例：

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

支持的传输类型：

- `stdio`：要求 `command` 为绝对路径
- `http`：通过 `url` 连接 streamable HTTP 端点
- `sse`：通过 `url` 连接 legacy SSE 流，并通过 `messageUrl` 发送请求

MCP 工具会在内置工具和 `skill_ref` 之后、agent 创建之前注册。agent 会冻结当前会话的 system prompt 和工具定义，因此修改 `mcp.json` 后需要重启客户端才会生效。

工具名称采用 `mcp_<server_name>_<tool_name>`。如果名称冲突，VibeCoding 会追加数字后缀，不会覆盖已有工具。自动启动加载会忽略 starter 模板里的占位项，例如 `/absolute/path/to/mcp-server`、`example.com` 和 `replace-me`。

---

## 认证配置

VibeCoding 支持多种方式提供 API 密钥，解析逻辑灵活。

### 密钥解析顺序

VibeCoding 需要某个提供商的 API 密钥时，按以下顺序查找：

1. **提供商 `apiKey` 字段** — 如果在 `settings.json` 中设置了，按下方规则解析
2. **派生的环境变量** — 将提供商名称转换为环境变量：例如 `deepseek-openai` → `DEEPSEEK_OPENAI_API_KEY`

### apiKey 字段格式

`apiKey` 字段支持三种格式：

| 格式 | 示例 | 行为 |
|------|------|------|
| `${VAR}` | `"${DEEPSEEK_API_KEY}"` | 读取环境变量 `VAR` 的值 |
| `!command` | `"!pass show deepseek-key"` | 执行 shell 命令，使用其标准输出 |
| 纯字符串 | `"sk-abc123..."` | 直接使用 (⚠️ 不建议用于共享配置) |

#### 环境变量引用

```json
{
  "providers": {
    "deepseek-openai": {
      "apiKey": "${DEEPSEEK_API_KEY}"
    }
  }
}
```

然后设置环境变量：

```bash
export DEEPSEEK_API_KEY=sk-...
```

#### Shell 命令 (密码管理器集成)

前缀加 `!` 可执行 shell 命令。VibeCoding 在 Linux/macOS 上使用 `sh -c`，在 Windows 上使用 `powershell.exe`。

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

适用于集成 `pass`、`1password-cli`、macOS 钥匙串或其他密钥管理工具。

#### 派生环境变量回退

如果某个提供商未配置 `apiKey`，VibeCoding 会从提供商名称派生环境变量名：

| 提供商名称 | 派生的环境变量 |
|-----------|---------------|
| `deepseek-openai` | `DEEPSEEK_OPENAI_API_KEY` |
| `deepseek-anthropic` | `DEEPSEEK_ANTHROPIC_API_KEY` |
| `my-custom-provider` | `MY_CUSTOM_PROVIDER_API_KEY` |
| `anthropic` | `ANTHROPIC_API_KEY` |
| `openai` | `OPENAI_API_KEY` |

规则：`-` 替换为 `_`，全部大写，末尾追加 `_API_KEY`。

### 认证示例

**方式一：环境变量 (最简单)**

```bash
export DEEPSEEK_API_KEY=sk-...
```

使用默认配置时，VibeCoding 会为 `deepseek-openai` 提供商查找 `DEEPSEEK_OPENAI_API_KEY`。但如果提供商的 `apiKey` 设置为 `${DEEPSEEK_API_KEY}`，则读取该环境变量。

**方式二：配置文件内嵌**

```json
{
  "providers": {
    "deepseek-openai": {
      "apiKey": "sk-..."
    }
  }
}
```

**方式三：密码管理器**

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

## 环境变量覆盖

以下环境变量在运行时覆盖设置：

| 环境变量 | 覆盖的设置 | 示例 |
|---------|-----------|------|
| `VIBECODING_DIR` | 全局配置目录 | `export VIBECODING_DIR=/custom/config` |
| `VIBECODING_PROVIDER` | `defaultProvider` | `export VIBECODING_PROVIDER=anthropic` |
| `VIBECODING_MODEL` | `defaultModel` | `export VIBECODING_MODEL=claude-sonnet-4-20250514` |
| `VIBECODING_MODE` | `defaultMode` | `export VIBECODING_MODE=yolo` |
| `VIBECODING_THINKING` | `defaultThinkingLevel` | `export VIBECODING_THINKING=high` |
| `VIBECODING_DEBUG` | 启用 provider 级请求/响应调试输出 | `export VIBECODING_DEBUG=1` |

---

## 配置示例

### 最小配置

只需设置默认提供商和模型，其余使用合理的默认值。

```json
{
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash"
}
```

### 多提供商配置

可在运行时通过 `/provider` 或 `--provider` 切换提供商：

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

### 自定义 API 端点 / 代理

```json
{
  "providers": {
    "my-proxy": {
      "baseUrl": "https://my-proxy.example.com/v1",
      "api": "openai-chat",
      "apiKey": "${MY_PROXY_API_KEY}",
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

### 启用沙箱并自定义路径

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

### 为长会话启用空闲压缩

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

### 项目级覆盖

放在 `.vibe/settings.json` 中可覆盖特定项目的设置：

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

这会与全局设置合并 — 只有你指定的字段会被覆盖。
