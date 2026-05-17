# 配置详解

## 配置文件位置

VibeCoding 使用两个配置文件:

| 文件 | 平台 | 范围 | 优先级 |
|------|------|------|--------|
| `~/.vibecoding/settings.json` | Linux/macOS | 全局 (所有项目) | 低 |
| `%APPDATA%\vibecoding\settings.json` | Windows | 全局 (所有项目) | 低 |
| `.vibe/settings.json` | 全部 | 项目级 | 高 |

> **Windows 用户：** `%APPDATA%` 实际展开为 `C:\Users\<用户名>\AppData\Roaming`，所以完整路径通常是 `C:\Users\<用户名>\AppData\Roaming\vibecoding\settings.json`。

项目级配置会覆盖全局配置。

## 配置结构

### 完整示例

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

## 配置项详解

### providers

多提供商配置。每个提供商包含:

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `baseUrl` | string | ✓ | API 基础 URL |
| `apiKey` | string | - | API 密钥 (可选，也可通过环境变量) |
| `api` | string | - | API 类型: `openai-chat` 或 `anthropic-messages` |
| `thinkingFormat` | string | - | 思考参数格式: `""`, `"openai"`, `"anthropic"`, `"xiaomi"` |
| `models` | array | - | 可用模型列表 |

#### api 字段

- `openai-chat`: OpenAI Chat Completions API 格式
- `anthropic-messages`: Anthropic Messages API 格式

如果未指定，会根据 `baseUrl` 自动检测:
- 包含 "anthropic" → `anthropic-messages`
- 其他 → `openai-chat`

#### thinkingFormat 字段

指定思考/推理参数如何发送到 API:

- `""` (空): 根据 URL 自动检测
- `"openai"`: 使用 OpenAI `reasoning_effort` 格式
- `"anthropic"`: 使用 Anthropic `thinking` 带 `budget_tokens`
- `"xiaomi"`: 使用 `thinking: {type: "enabled"}` 格式 (用于小米 MiMo API)

未设置时，如果 URL 包含 `xiaomimimo` 则自动检测为 `xiaomi` 格式。

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

#### models 数组

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

| 字段 | 类型 | 描述 |
|------|------|------|
| `id` | string | 模型 ID |
| `name` | string | 显示名称 |
| `contextWindow` | int | 上下文窗口大小 (token) |
| `maxTokens` | int | 最大输出 token |
| `reasoning` | bool | 是否支持推理/思考 |
| `input` | []string | 支持的输入类型 (text, image) |
| `cost` | object | 定价 (每百万 token) |

### defaultProvider

默认使用的提供商名称。对应 `providers` 中的键名。

```json
{
  "defaultProvider": "deepseek-openai"
}
```

### defaultModel

默认使用的模型 ID。

```json
{
  "defaultModel": "deepseek-v4-flash"
}
```

### defaultMode

默认运行模式。

```json
{
  "defaultMode": "agent"
}
```

可选值:
- `plan`: 只读分析模式
- `agent`: 标准读写模式 (默认)
- `yolo`: 完全访问模式

### defaultThinkingLevel

默认思考级别。

```json
{
  "defaultThinkingLevel": "medium"
}
```

可选值:
- `off`: 关闭思考
- `minimal`: 最小思考
- `low`: 低级别
- `medium`: 中等级别
- `high`: 高级别
- `xhigh`: 最高级别

### maxOutputTokens

最大输出 token 数量。

```json
{
  "maxOutputTokens": 384000
}
```

### maxContextTokens

最大上下文 token 数量。

```json
{
  "maxContextTokens": 200000
}
```

### compaction

上下文压缩配置，用于管理长对话。

```json
{
  "compaction": {
    "enabled": true,
    "reserveTokens": 16384,
    "keepRecentTokens": 20000
  }
}
```

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | true | 是否启用压缩 |
| `reserveTokens` | int | 16384 | 为模型响应保留的 token |
| `keepRecentTokens` | int | 20000 | 保留的最近消息 token |

### sandbox

沙箱配置。

```json
{
  "sandbox": {
    "enabled": true,
    "level": "standard",
    "allowNetwork": false
  }
}
```

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用沙箱 |
| `level` | string | standard | 沙箱级别 (none, standard, strict) |
| `allowNetwork` | bool | false | 是否允许网络访问 |

### contextFiles

上下文文件配置。

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

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | true | 是否自动加载上下文文件 |
| `extraFiles` | []string | [] | 额外的上下文文件路径 |

#### 自动加载的上下文文件

VibeCoding 会自动搜索并加载以下文件:

1. **全局文件** (Linux/macOS: `~/.vibecoding/`, Windows: `%APPDATA%\vibecoding\`):
   - `AGENTS.md`
   - `CLAUDE.md`

2. **项目文件** (从当前目录向上搜索):
   - `AGENTS.md`
   - `CLAUDE.md`
   - `.vibe/AGENTS.md`
   - `.vibe/CLAUDE.md`

### skillsDir

技能目录路径。

```json
{
  "skillsDir": "~/.vibecoding/skills"
}
```

技能文件结构:
- 全局技能:
  - Linux/macOS: `~/.vibecoding/skills/<name>/SKILL.md`
  - Windows: `%APPDATA%\vibecoding\skills\<name>\SKILL.md`
- 项目技能: `.skills/<name>/SKILL.md` (覆盖全局)

### sessionDir

会话文件存储目录。

```json
{
  "sessionDir": "~/.vibecoding/sessions"  // Linux/macOS
  // Windows: "%APPDATA%\\vibecoding\\sessions"
}
```

### shellPath

自定义 shell 路径，用于 bash 工具。

```json
{
  "shellPath": "/bin/bash"
}
```

### shellCommandPrefix

自定义命令前缀。

```json
{
  "shellCommandPrefix": ""
}
```

### theme

UI 主题。

```json
{
  "theme": "dark"
}
```

可选值: `dark`, `light`

### retry

API 调用重试配置。

```json
{
  "retry": {
    "enabled": true,
    "maxRetries": 3,
    "baseDelayMs": 2000
  }
}
```

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | true | 是否启用重试 |
| `maxRetries` | int | 3 | 最大重试次数 |
| `baseDelayMs` | int | 2000 | 基础延迟 (毫秒) |

### approval

Agent 模式审批配置，控制 bash 命令的审批行为。

```json
{
  "approval": {
    "bashWhitelist": ["go ", "make ", "git ", "npm ", "yarn "],
    "bashBlacklist": ["rm -rf", "sudo"]
  }
}
```

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `bashWhitelist` | []string | 见下文 | 自动批准的命令前缀列表 |
| `bashBlacklist` | []string | [] | 始终需要审批的命令前缀列表 |

#### 默认白名单

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

#### 审批流程

```
┌─────────────────────────────────────────────────────────────┐
│                    Approval Flow                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Agent 请求执行 bash 命令                                    │
│  │                                                           │
│  ▼                                                           │
│  检查模式                                                    │
│  ├─ Plan 模式 → 拒绝 (只读)                                  │
│  ├─ Agent 模式 → 继续检查                                    │
│  └─ YOLO 模式 → 自动批准                                     │
│                                                              │
│  Agent 模式下:                                               │
│  ├─ 非 bash 工具 → 自动批准                                  │
│  ├─ 命令匹配白名单 → 自动批准                                │
│  └─ 其他 → 需要用户审批                                      │
│                                                              │
│  用户审批:                                                   │
│  ├─ 输入 y/yes → 执行命令                                    │
│  └─ 输入 n/no → 拒绝执行                                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
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

## 认证配置

### 方式一: 环境变量

```bash
export DEEPSEEK_API_KEY=sk-...
```

### 方式二: 配置文件内嵌

在 `settings.json` 的 providers 中直接配置:

```json
{
  "providers": {
    "deepseek-openai": {
      "apiKey": "sk-..."
    }
  }
}
```

### 密钥解析顺序

1. 环境变量 (`DEEPSEEK_API_KEY`)
2. 配置文件内嵌 (`settings.json` providers.<name>.apiKey)

## 环境变量覆盖

可以通过环境变量覆盖任何设置:

| 环境变量 | 覆盖的配置 |
|----------|-----------|
| `VIBECODING_DIR` | 配置目录 |
| `VIBECODING_PROVIDER` | defaultProvider |
| `VIBECODING_MODEL` | defaultModel |
| `VIBECODING_MODE` | defaultMode |
| `VIBECODING_THINKING` | defaultThinkingLevel |

## 配置示例

### 最小配置

```json
{
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash"
}
```

### 多提供商配置

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

### 自定义 API 端点

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

### 启用沙箱

```json
{
  "sandbox": {
    "enabled": true,
    "level": "standard"
  }
}
```
