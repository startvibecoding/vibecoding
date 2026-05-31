# Gateway 模式

## 概述

Gateway 模式将 VibeCoding 作为 HTTP 服务运行，对外暴露**标准 OpenAI Chat Completions API**。任何兼容 OpenAI 的客户端 — Cursor、Continue、Open WebUI、Python SDK、自定义脚本 — 都可以直接接入，VibeCoding agent 在后台透明地执行工具调用。

```bash
vibecoding gateway
```

## 快速开始

```bash
# 生成配置模板
vibecoding --init-gateway

# 启动 gateway（默认 :8080）
vibecoding gateway

# 测试
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "列出当前目录的文件"}],
    "stream": false
  }'
```

## 命令行参数

| 参数 | 说明 |
|------|------|
| `--port` | 监听端口（默认：配置文件或 8080） |
| `--config` | gateway.json 路径 |
| `--work-dir` | 默认工作目录 |
| `--provider` / `-p` | 覆盖 provider |
| `--model` / `-m` | 覆盖 model |
| `--sandbox` | 启用沙箱（bwrap） |
| `--multi-agent` | 启用子 Agent 工具 |
| `--verbose` | 详细输出 |
| `--debug` | 调试日志 |

## 配置

Gateway 使用独立的配置文件 `gateway.json`，与 `settings.json` 分开。

**配置加载优先级**（从高到低）：

1. CLI `--config /path/to/gateway.json`
2. `.vibe/gateway.json`（项目级）
3. `~/.config/vibecoding/gateway.json`（全局）

生成配置模板：

```bash
vibecoding --init-gateway
vibecoding --init-gateway --force  # 强制覆盖
```

### 完整配置参考

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
    "level": ""       // "none", "standard", "strict"；空 = 根据 mode 自动推导
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

## API 端点

### POST /v1/chat/completions

标准 OpenAI Chat Completions API，支持流式和非流式。

**请求：**

```json
{
  "model": "deepseek-v4-flash",
  "messages": [
    {"role": "system", "content": "你是一个编程助手。"},
    {"role": "user", "content": "读取 main.go 并解释。"}
  ],
  "stream": true,
  "max_tokens": 4096,
  "x_session_id": "my-session",
  "x_mode": "yolo",
  "x_working_dir": "/home/user/project"
}
```

扩展字段（`x_*`）为可选：

| 字段 | 说明 |
|------|------|
| `x_session_id` | 关联已有 session（省略则新建） |
| `x_mode` | 覆盖本次请求的 mode |
| `x_working_dir` | 覆盖工作目录（需通过 `allowedWorkDirs` 校验） |

**非流式响应：**

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1716883200,
  "model": "deepseek-v4-flash",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "以下是代码解释..."},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 100, "completion_tokens": 200, "total_tokens": 300},
  "x_session_id": "my-session",
  "x_tool_calls": [
    {"name": "read", "args": {"path": "main.go"}, "status": "completed"}
  ]
}
```

**流式响应**使用标准 SSE 格式，以 `data:` 行发送，`[DONE]` 结束。

### GET /v1/models

返回可用模型列表。

### GET /health

健康检查（无需认证）。

```json
{"status": "ok", "version": "v0.1.26", "sessions": 3}
```

## 斜杠指令

当最后一条用户消息以 `/` 开头时，在 gateway 层直接处理，不调用 LLM。

| 指令 | 说明 |
|------|------|
| `/clear` | 清空 session 上下文 |
| `/mode [plan\|agent\|yolo]` | 查看或切换模式 |
| `/model [model_id]` | 查看或切换模型 |
| `/models` | 列出可用模型 |
| `/sessions` | 列出活跃 session |
| `/sessions del <id>` | 删除 session |
| `/compact` | 触发上下文压缩 |
| `/status` | 查看 session 状态 |
| `/skill <name>` | 激活 skill |
| `/skills` | 列出可用 skills |
| `/help` | 显示所有指令 |

指令返回标准 OpenAI 响应格式，`stream: true` 和 `stream: false` 均支持。

## 工具可见性

控制工具执行在响应内容中的展示方式。

### mode

| `toolVisibility.mode` | 行为 |
|------------------------|------|
| `content`（默认） | 工具输出混入 content 流 |
| `sse_event` | 工具输出通过独立的 `event: tool_status` SSE 事件发送 |
| `none` | 不发送任何工具输出，客户端只见最终文本 |

### detail

| `toolVisibility.detail` | 行为 |
|--------------------------|------|
| `collapsed`（默认） | 一行摘要：`🔧 read: main.go ✅` |
| `expanded` | 完整输出，用代码块包裹并自动检测语言 |

**折叠模式**（默认）：大部分工具显示一行摘要。`edit`/`write` 有 diff 时始终展示 diff。错误始终完整展示。

**展开模式**：工具结果用 fenced code block 包裹，自动检测语言（`.go` → `go`，`.py` → `python`，bash 输出 → `bash`，diff → `diff`）。

## 多 Session

每个请求可通过 `x_session_id` 关联 session。Session 维护独立的 agent 状态、消息历史和工具。

- 无 `x_session_id` → 每请求新建 session（无状态）
- 有 `x_session_id` → 多轮对话（有状态）
- Session 超过 `idleTimeoutSeconds` 自动过期
- 同一 session 内的请求串行处理

## 认证

设置 `auth.enabled: true` 并配置 `auth.tokens`：

```json
{
  "auth": {
    "enabled": true,
    "tokens": ["sk-token-1", "sk-token-2"]
  }
}
```

客户端发送：`Authorization: Bearer sk-token-1`

`/health` 端点始终不需要认证。

## 安全

三层独立防护：

| 层次 | 机制 | 作用 |
|------|------|------|
| L1 | Bearer Token | 阻止未授权访问 |
| L2 | `allowedWorkDirs` | 限制文件系统操作范围 |
| L3 | Sandbox (bwrap) | OS 级隔离 |

### allowedWorkDirs

控制 `x_working_dir` 可切换到哪些目录：

- 未设置（`null`）→ 不限制
- 空 `[]` → 禁止所有切换，只能用 `workingDir`
- 目录列表 → 前缀匹配（路径分隔符边界）

`workingDir` 本身始终被信任（管理员配置的值）。

## 客户端示例

### Python OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="sk-my-token",  # 如果开启了认证
)

response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[
        {"role": "user", "content": "读取 main.go 并解释架构。"},
    ],
    stream=True,
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### 多轮对话（带 session）

```python
response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "读取 main.go"}],
    extra_body={"x_session_id": "my-session"},
)

response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "重构错误处理"}],
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
    "messages": [{"role": "user", "content": "解释 main.go"}],
    "stream": true
  }'
```

## System Prompt 处理

| `systemPromptMode` | 行为 |
|---------------------|------|
| `append`（默认） | 客户端 system message 追加到内置 system prompt 末尾 |
| `ignore` | 忽略客户端 system message |

内置 system prompt 包含工具定义、模式指令和上下文文件。`append` 模式保留所有内置内容，同时接受客户端自定义指令。
