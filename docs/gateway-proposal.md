# Gateway Mode 方案设计

> 状态: 已确认 (Approved) — v0.1.26 全部新增功能
> 日期: 2026-05-28
> 版本: v0.1.26

## 1. 概述

Gateway 模式将 VibeCoding 作为一个 HTTP 服务启动，对外暴露**标准 OpenAI Chat Completions API** (`/v1/chat/completions`)。
任何兼容 OpenAI SDK 的客户端（Cursor、Continue、Open WebUI、自定义脚本等）都可以直接接入，
后端实际由 VibeCoding agent 完成推理 + tool use 循环，对调用方完全透明。

### 核心特性

| 特性 | 说明 |
|------|------|
| **OpenAI 兼容 API** | 支持 `/v1/chat/completions`（streaming & non-streaming）和 `/v1/models` |
| **多 Session** | 默认支持，每个请求可通过 header / body 关联 session，也可自动创建 |
| **Sub-Agent 能力** | 可选开启（配置 `enableSubAgents: true`），复用现有 multi-agent 体系 |
| **Bearer Token 认证** | 基于 `Authorization: Bearer <token>` header，配置文件控制，默认关闭 |
| **独立配置文件** | `gateway.json`，与 `settings.json` 同目录 (`~/.config/vibecoding/`) |

## 2. 启动方式

```bash
# 启动 gateway（默认 :8080）
vibecoding gateway

# 指定端口
vibecoding gateway --port 9090

# 指定 provider/model（覆盖 settings.json 默认值）
vibecoding gateway --provider deepseek-openai --model deepseek-v4-flash

# 指定默认工作目录
vibecoding gateway --work-dir /home/user/projects

# 指定配置文件路径
vibecoding gateway --config /path/to/gateway.json

# 启用 sub-agent
vibecoding gateway --multi-agent

# 启用 sandbox
vibecoding gateway --sandbox

# 启用 debug
vibecoding gateway --debug --verbose
```

### 初始化配置文件

```bash
# 创建 gateway.json 模板（写入 ~/.config/vibecoding/gateway.json）
vibecoding --init-gateway

# 如果文件已存在，不覆盖，提示用户
vibecoding --init-gateway
# → gateway.json already exists: ~/.config/vibecoding/gateway.json

# 强制覆盖
vibecoding --init-gateway --force
```

`--init-gateway` 是 root command 的 flag（不是 gateway 子命令的），因为用户可能在还没有配置文件时就想生成模板。

CLI 实现为 `rootCmd.AddCommand(gatewayCmd)`，与现有 `acp` 子命令平级。

## 3. 配置文件

### 3.1 路径

`gateway.json` 位于 `config.ConfigDir()` （通常 `~/.config/vibecoding/gateway.json`），与 `settings.json` 同目录。

### 3.2 Schema

```jsonc
{
  // 监听地址
  "listen": ":8080",

  // 认证配置 - 默认关闭
  "auth": {
    "enabled": false,
    // tokens 列表 - 任一匹配即通过
    "tokens": [
      "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    ]
  },

  // 默认 mode（可被每个请求覆盖）
  "defaultMode": "yolo",

  // 默认 thinking level
  "defaultThinkingLevel": "medium",

  // 是否启用 sub-agent 能力
  "enableSubAgents": false,

  // Sandbox 配置
  "sandbox": {
    // 是否启用 sandbox（也可通过 --sandbox flag 开启）
    "enabled": false,
    // sandbox level: "none", "standard", "strict"
    // 为空时根据 mode 自动推导：yolo→none, agent→standard, plan→strict
    "level": ""
    // 其他 sandbox 细节（allowedRead, deniedPaths 等）继承 settings.json 中的 sandbox 配置
  },

  // 工作目录安全
  "allowedWorkDirs": [
    // 允许请求级 x_working_dir 切换到的目录白名单
    // 支持前缀匹配："/home/user/projects" 匹配 "/home/user/projects/foo"
    // 为空 [] 表示仅允许使用 workingDir 默认值，禁止请求级切换
    // 不设置此字段（null）则不做校验
    "/home/user/projects",
    "/opt/repos"
  ],

  // session 管理
  "session": {
    // session 空闲超时（秒），超时后自动清理。0 = 不超时
    "idleTimeoutSeconds": 1800,
    // 最大并发 session 数。0 = 不限制
    "maxSessions": 0
  },

  // 默认工作目录 — agent 执行 tool 时的 cwd
  // 为空时 fallback 到 gateway 进程的 cwd
  "workingDir": "/home/user/projects",

  // 跨域配置
  "cors": {
    "enabled": false,
    "allowOrigins": ["*"]
  },

  // Provider/Model 覆盖（不设置则使用 settings.json 中的默认值）
  "provider": "",
  "model": "",

  // Tool 可见性
  "toolVisibility": {
    // "content": 通过 content 字段发送 tool 状态信息（默认）
    // "sse_event": 通过扩展 SSE event 发送（event: tool_status，不兼容标准 OpenAI SDK）
    // "none": 不发送任何 tool 状态信息
    "mode": "content"
  },

  // System prompt 处理策略
  // "append": 客户端 system message 追加到内置 system prompt 末尾（默认）
  // "ignore": 忽略客户端 system message
  "systemPromptMode": "append",

  // 请求超时（秒）— agent 执行的最大时长
  // streaming 模式下只要有数据流动就不超时
  "requestTimeoutSeconds": 300,

  // 全局并发限制（0 = 不限制）
  "maxConcurrentRequests": 0,

  // 日志级别
  "logLevel": "info"  // "debug", "info", "warn", "error"
}
```

### 3.3 配置加载优先级

1. 请求级 `x_working_dir` / `x_mode`（仅部分字段）
2. CLI flags（`--port`, `--multi-agent`, `--work-dir` 等）
3. `gateway.json`
4. `settings.json` 中的默认 provider/model/mode
5. 进程 cwd（workingDir 最终 fallback）

## 4. API 设计

### 4.1 POST /v1/chat/completions

**请求格式**（标准 OpenAI）:

```jsonc
{
  "model": "deepseek-v4-flash",  // 可选，覆盖默认 model
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Read the file main.go and explain it."}
  ],
  "stream": true,                // 支持 true/false
  "temperature": 0.7,            // 透传给后端 provider
  "max_tokens": 4096,            // 透传

  // VibeCoding 扩展字段（可选）
  "x_session_id": "sess-abc123",  // 关联已有 session
  "x_mode": "yolo",               // 覆盖 mode
  "x_working_dir": "/home/user/project"  // 覆盖工作目录
}
```

**Non-streaming 响应**:

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1716883200,
  "model": "deepseek-v4-flash",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Here is the explanation of main.go..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 1234,
    "completion_tokens": 567,
    "total_tokens": 1801
  },
  "x_session_id": "sess-abc123",
  "x_tool_calls": [
    {"name": "read", "args": {"path": "main.go"}, "status": "completed"}
  ]
}
```

**Streaming 响应**（SSE）:

```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1716883200,"model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"role":"assistant","content":"Here"},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1716883200,"model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"content":" is"},"finish_reason":null}]}

...

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1716883200,"model":"deepseek-v4-flash","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1234,"completion_tokens":567,"total_tokens":1801}}

data: [DONE]
```

### 4.2 GET /v1/models

返回当前 provider 可用的模型列表：

```json
{
  "object": "list",
  "data": [
    {
      "id": "deepseek-v4-flash",
      "object": "model",
      "created": 1716883200,
      "owned_by": "vibecoding"
    }
  ]
}
```

### 4.3 GET /health

健康检查端点（无需认证）：

```json
{"status": "ok", "version": "v0.1.26", "sessions": 3}
```

### 4.4 Session 管理端点（扩展，可选）

```
POST   /v1/vibecoding/sessions          创建 session
GET    /v1/vibecoding/sessions           列出 session
GET    /v1/vibecoding/sessions/:id       获取 session 详情
DELETE /v1/vibecoding/sessions/:id       删除 session
```

这些是扩展端点，非 OpenAI 标准，前缀 `/v1/vibecoding/` 以区分。

## 5. 架构设计

### 5.1 模块关系

```
cmd/vibecoding/main.go
  └── gatewayCmd (cobra.Command)
        └── internal/gateway/
              ├── gateway.go         # Server 主逻辑、路由
              ├── config.go          # gateway.json 加载
              ├── handler_chat.go    # /v1/chat/completions 处理
              ├── handler_models.go  # /v1/models
              ├── handler_health.go  # /health
              ├── handler_session.go # session 管理端点
              ├── auth.go            # Bearer Token 中间件
              ├── commands.go        # /xxx 指令处理
              ├── session_mgr.go     # 多 session 管理器
              ├── streaming.go       # SSE streaming 辅助
              └── types.go           # OpenAI API 类型定义
```

### 5.2 核心组件

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Server                          │
│  (net/http, 无外部框架)                                  │
├──────────┬──────────┬───────────────┬───────────────────┤
│ Auth MW  │ CORS MW  │  Logging MW   │                   │
├──────────┴──────────┴───────────────┴───────────────────┤
│                                                         │
│  /v1/chat/completions ──► ChatHandler                   │
│       │                                                 │
│       ├─► SessionPool.GetOrCreate(sessionID)            │
│       │     └── session.Manager (JSONL)                 │
│       │                                                 │
│       ├─► agent.New(Config{...}) + tools.Registry       │
│       │     └── agent.Run(ctx, userMsg) → <-chan Event  │
│       │                                                 │
│       └─► EventToSSE / EventToJSON                      │
│             └── OpenAI 格式 response                     │
│                                                         │
│  /v1/models ──► ModelsHandler                           │
│       └── provider.Models()                             │
│                                                         │
│  /health ──► HealthHandler                              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 5.3 请求处理流程

```
HTTP Request
    │
    ▼
1. Auth Middleware (如果 auth.enabled)
    │ 检查 Authorization: Bearer <token>
    │ 失败 → 401 Unauthorized
    │
    ▼
2. CORS Middleware (如果 cors.enabled)
    │
    ▼
3. Route Dispatch
    │
    ▼
4. ChatHandler
    │
    ├─ 4a. 解析 OpenAI 格式请求
    │       - messages → provider.Message 转换
    │       - 提取 x_session_id（或生成新 ID）
    │       - 提取 x_mode, x_working_dir
    │
    ├─ 4a.1 校验 x_working_dir
    │       - 有 allowedWorkDirs → 前缀匹配校验
    │       - 不通过 → 403 Forbidden
    │
    ├─ 4a.2 检查最后一条 user message 是否为 /xxx 指令
    │       - 是指令 → 走指令分发（不创建 agent，不调用 LLM）
    │       - 非指令 → 继续正常 agent 流程
    │
    ├─ 4b. 获取/创建 Session
    │       - SessionPool.GetOrCreate(id, workDir)
    │       - 关联 session.Manager, tools.Registry
    │
    ├─ 4c. 构建 Agent
    │       - 复用 agent.Config + agent.New() 模式
    │       - 加载 context files, skills
    │       - 如果 enableSubAgents → AgentFactory + AgentManager
    │
    ├─ 4d. 将 OpenAI messages 转换为 VibeCoding 内部格式
    │       - system message → extraContext / systemPrompt
    │       - user/assistant messages → provider.Message
    │       - 历史 messages → agent.LoadHistoryMessages()
    │
    ├─ 4e. 运行 Agent
    │       - eventCh := agent.Run(ctx, lastUserMessage)
    │
    └─ 4f. 转换输出
            │
            ├── stream=true:
            │   for event := range eventCh:
            │     EventTextDelta → SSE chunk
            │     EventToolCall → (内部处理，不暴露给客户端)
            │     EventDone → final chunk + [DONE]
            │
            └── stream=false:
                收集全部 text → 一次性返回 JSON
```

### 5.4 Session 管理

```go
// SessionPool 管理多个并发 session
type SessionPool struct {
    mu       sync.RWMutex
    sessions map[string]*GatewaySession
    maxSess  int
    idleTTL  time.Duration
}

type GatewaySession struct {
    ID        string
    WorkDir   string
    Manager   *session.Manager
    Registry  *tools.Registry
    AgentMgr  *agent.AgentManager  // 仅 enableSubAgents 时
    LastUsed  time.Time
    mu        sync.Mutex  // 保证单 session 串行处理
}
```

**Session 映射策略**:

1. 客户端通过 `x_session_id` 指定 → 直接使用
2. 未指定 → 每个请求创建新 session（无状态模式）
3. 通过 Authorization header 的 token hash 做 namespace（可选）

**Session 并发控制**:
- 每个 session 内部加锁，确保同一 session 的请求串行处理（agent loop 不支持并发）
- 不同 session 之间完全并行

**Session 生命周期**:
- 创建：首次请求时自动创建
- 活跃：有请求在处理或最近有请求
- 空闲超时清理：后台 goroutine 定期扫描，超过 `idleTimeoutSeconds` 的 session 被销毁
- 手动销毁：通过 DELETE `/v1/x/sessions/:id`

### 5.5 Tool 调用处理

Gateway 模式下 tool 调用对客户端透明，Agent 内部自动执行（mode 默认 `yolo`）。

Tool 执行状态的可见性由 `toolVisibility.mode` 配置控制：

| mode | 行为 | 兼容性 |
|------|------|--------|
| `"content"` (默认) | tool 执行时通过 `content` 字段发送状态信息，如 `[reading main.go...]` | ✅ 完全兼容标准 SDK |
| `"sse_event"` | 通过扩展 SSE event 发送（`event: tool_status`） | ⚠️ 不兼容标准 OpenAI SDK，适合自定义客户端 |
| `"none"` | 不发送任何 tool 状态，客户端只见最终文本 | ✅ 最干净 |

**`content` 模式示例**（streaming）:
```
data: {"choices":[{"delta":{"content":"[reading main.go...]\n"}}]}
data: {"choices":[{"delta":{"content":"[running: go test ./...]\n"}}]}
data: {"choices":[{"delta":{"content":"Here is the analysis..."}}]}
```

**`sse_event` 模式示例**（streaming）:
```
event: tool_status
data: {"tool":"read","status":"running","args":{"path":"main.go"}}

data: {"choices":[{"delta":{"content":"Here is the analysis..."}}]}
```

**Non-streaming 响应**: 无论哪种 mode，tool 执行记录始终可通过扩展字段 `x_tool_calls` 返回。

### 5.6 Sub-Agent 集成

当 `enableSubAgents: true` 时：

```
ChatHandler
    └── 每个 Session 维护独立的 AgentFactory + AgentManager
          └── 主 agent 可调用 subagent_spawn/status/send/destroy
                └── sub-agent 的事件也会收集到主 agent 的输出流中
```

复用现有 `agent.AgentFactory` / `agent.AgentManager` / `agent.SubAgent*Tool`，无需改动核心 agent 逻辑。

### 5.7 指令系统 (Slash Commands)

Gateway 支持通过用户消息内容发送 `/xxx` 指令，与 TUI 中的指令体验对齐。

**触发规则**: 当请求的 messages 中最后一条 `user` 消息以 `/` 开头时，
视为指令调用。指令不经过 agent/LLM，直接在 gateway 层处理，立即返回结果。

**请求示例**:
```jsonc
{
  "model": "deepseek-v4-flash",
  "messages": [
    {"role": "user", "content": "/clear"}
  ],
  "stream": false,
  "x_session_id": "sess-abc123"
}
```

**响应格式**: 始终使用标准 OpenAI 响应结构，指令结果放在 `content` 中，
`finish_reason` 为 `"stop"`，扩展字段 `x_command` 标识这是指令响应：

```json
{
  "id": "chatcmpl-cmd-xxx",
  "object": "chat.completion",
  "created": 1716883200,
  "model": "deepseek-v4-flash",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "✅ Conversation cleared"},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
  "x_command": "/clear",
  "x_session_id": "sess-abc123"
}
```

**支持的指令**:

| 指令 | 说明 | 需要 session |
|------|------|---------------|
| `/clear` | 清空当前 session 的对话上下文（agent 重置，消息清空，session 保留） | 是 |
| `/mode [plan\|agent\|yolo]` | 查看或切换当前 session 的模式 | 是 |
| `/model [model_id]` | 查看或切换模型 | 否 |
| `/models` | 列出可用模型（等同 GET `/v1/models`） | 否 |
| `/sessions` | 列出当前 workDir 下的 session | 否 |
| `/sessions clear` | 创建新 session，返回新 session ID | 否 |
| `/sessions del <id>` | 删除指定 session | 否 |
| `/compact` | 手动触发当前 session 的上下文压缩 | 是 |
| `/status` | 查看当前 session 状态（消息数、上下文占用、mode 等） | 是 |
| `/skill <name>` | 激活 skill | 是 |
| `/skills` | 列出可用 skills | 否 |
| `/help` | 列出所有可用指令 | 否 |

**不支持的 TUI 指令**:
- `/quit` — 无意义，Gateway 是服务进程
- `/agent` 系列 — sub-agent 由 agent 内部管理，客户端无需直接操作
- `/init_mcp` — MCP 配置属于服务端管理，不应通过 API 暴露

**实现位置**: `internal/gateway/commands.go`

```go
// CommandResult 表示指令执行结果
type CommandResult struct {
    Message string // 返回给客户端的文本
    Error   bool   // 是否为错误
}

// handleCommand 拦截并处理 /xxx 指令
// 返回 nil 表示不是指令，应走正常 agent 流程
func (s *Server) handleCommand(sessionID, cmd string) *CommandResult {
    parts := strings.Fields(cmd)
    switch parts[0] {
    case "/clear":
        // 重置 session 的 agent + 消息历史
    case "/mode":
        // 查看/切换 session 的 mode
    case "/status":
        // 返回 session 状态信息
    // ...
    default:
        return &CommandResult{Message: "Unknown command: " + parts[0], Error: true}
    }
}
```

**与 TUI 指令的关系**:
- Gateway 指令和 TUI 指令分开实现（TUI 依赖 Bubble Tea，无法复用）
- 保持语义一致：相同的指令名、相同的行为
- 未来可抽取共享的指令定义层（Phase 3）

## 6. 认证设计

### 6.1 Bearer Token

```
Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

- `gateway.json` 中配置 `auth.tokens` 列表
- 中间件对每个请求检查 header
- 多 token 支持（团队场景，每人一个 token）
- `/health` 端点不做认证

### 6.2 认证关闭（默认）

`auth.enabled: false` 时跳过所有认证检查。适用于本地开发、内网部署。

### 6.3 未来扩展（本期不做）

- OAuth2 / OIDC
- API Key + Rate Limiting
- mTLS

## 7. 与现有模块的关系

| 现有模块 | Gateway 复用方式 |
|---------|-----------------|
| `internal/config` | 加载 `settings.json`，读取 provider/model 配置 |
| `internal/provider` + `factory` | 创建 LLM provider 实例 |
| `internal/agent` | 核心 agent loop、tool execution、multi-agent |
| `internal/session` | JSONL session 存储（每个 gateway session 一个 Manager） |
| `internal/tools` | tool registry（每个 session 独立 registry） |
| `internal/contextfiles` | 加载 AGENTS.md/CLAUDE.md |
| `internal/skills` | 加载 skills |
| `internal/sandbox` | sandbox 管理 |
| `internal/mcp` | MCP server 连接（可选） |

**新增模块**: `internal/gateway/` — 仅包含 HTTP 层 + session 池 + OpenAI 格式转换，不引入新的 agent 逻辑。

## 8. OpenAI 格式转换

### 8.1 输入转换 (OpenAI → VibeCoding)

```
OpenAI messages[] ──►  VibeCoding 内部
─────────────────      ──────────────────
system message    →    根据 systemPromptMode 处理（见下方）
user message      →    provider.NewUserMessage(text)
assistant message →    provider.NewAssistantMessage(blocks)
                       （含历史 tool_calls 的 assistant message → 跳过或简化）
```

**System prompt 处理**（由 `gateway.json` 的 `systemPromptMode` 控制）:

| systemPromptMode | 行为 |
|------------------|------|
| `"append"` (默认) | 客户端 system message 追加到内置 system prompt 末尾（作为 extraContext）。保留 tool 说明、mode 指令等内置内容，同时尊重客户端的补充指令。 |
| `"ignore"` | 忽略客户端 system message。完全使用 VibeCoding 内置 system prompt，适合不希望客户端干扰 agent 行为的场景。 |

**其他关键决策**:
- 只取最后一条 `user` 消息作为 `agent.Run(ctx, userMsg)` 的输入
- 之前的历史消息通过 `agent.LoadHistoryMessages()` 注入

### 8.2 输出转换 (VibeCoding Event → OpenAI)

```
VibeCoding Event         OpenAI Chunk (toolVisibility 决定)
──────────────           ───────────────
EventTextDelta      →    {"delta": {"content": text}}
EventThinkDelta     →    (不暴露 / 或通过扩展字段)
EventToolCall       →    content: "[reading main.go...]"  (mode=content)
                         event: tool_status               (mode=sse_event)
                         (不发送)                          (mode=none)
EventToolResult     →    (内部处理，不暴露)
EventDone           →    {"finish_reason": "stop"} + usage
EventError          →    HTTP 500 or error chunk
EventUsage          →    usage 字段
```

## 9. 实现计划

### Phase 1: 最小可用 (MVP)

1. **`internal/gateway/config.go`** — gateway.json 加载 + DefaultGatewayConfig() 模板
2. **`internal/gateway/types.go`** — OpenAI API 请求/响应类型
3. **`internal/gateway/auth.go`** — Bearer Token 认证中间件
4. **`internal/gateway/session_mgr.go`** — SessionPool 多 session 管理
5. **`internal/gateway/commands.go`** — /xxx 指令处理
6. **`internal/gateway/handler_chat.go`** — `/v1/chat/completions` 核心处理
7. **`internal/gateway/handler_models.go`** — `/v1/models`
8. **`internal/gateway/handler_health.go`** — `/health`
9. **`internal/gateway/streaming.go`** — SSE 流式输出辅助
10. **`internal/gateway/gateway.go`** — Server 启动、路由组装
11. **`cmd/vibecoding/main.go`** — 添加 `gateway` 子命令 + `--init-gateway` flag

### Phase 2: 增强

11. Sub-Agent 集成
12. Session 管理 API (`/v1/x/sessions`)
13. CORS 支持
14. Graceful shutdown
15. 请求日志 + metrics

### Phase 3: 生产化

16. Rate limiting
17. 请求大小限制
18. Timeout 控制
19. 文档 (docs/en/gateway.md, docs/zh/gateway.md)

## 10. 关键设计决策

### D1: 不引入外部 HTTP 框架

使用 `net/http` 标准库。VibeCoding 定位轻量，不需要 gin/echo/fiber。中间件用 `http.Handler` 包装即可。

### D2: 默认 mode 为 yolo

Gateway 场景不存在 TUI 交互，tool approval 无法实现。默认使用 `yolo` 模式，tool 自动执行。
如果未来需要 approval，可通过 webhook callback 实现。

### D3: Tool 可见性可配置

Agent 内部的 read/write/bash/grep 等 tool 调用的可见性由 `toolVisibility.mode` 控制：
- `"content"` (默认): tool 执行时在 streaming 的 content 中发送状态文本，客户端可感知进度
- `"sse_event"`: 通过扩展 SSE event 发送，适合自定义客户端
- `"none"`: 完全透明，客户端只见最终文本

Non-streaming 响应始终可通过扩展字段 `x_tool_calls` 查看 tool 执行记录。

### D4: Session 映射策略

- 无 `x_session_id` → 每请求新建 session（简单、无状态）
- 有 `x_session_id` → 多轮对话共享 session（有状态）
- Session 不持久化跨重启（重启清空），但 JSONL 文件保留可恢复

### D5: 每个 session 串行处理

同一个 session 的请求串行化（mutex），避免 agent loop 并发问题。
不同 session 完全并行，充分利用多核。

### D6: 消息历史处理

gateway 仅使用 session 内已有的消息历史 + 当前请求的最新消息。
不依赖客户端传入的 messages 数组做完整历史重放（因为 agent 内部已有 session 管理）。

但如果是新 session（无 `x_session_id` 或 session 不存在），
则客户端传入的 messages 数组会被当作完整历史注入。

### D7: allowedWorkDirs 白名单

请求通过 `x_working_dir` 切换工作目录时，必须通过白名单校验：

```
请求 x_working_dir
    │
    ▼
1. allowedWorkDirs 为 null（未设置）→ 放行（不校验）
2. allowedWorkDirs 为 []（空数组）→ 拒绝一切切换，只能用 workingDir 默认值
3. allowedWorkDirs 有条目 → 前缀匹配，任一匹配则放行
    │ 不匹配 → 403 Forbidden
```

**前缀匹配规则**: `filepath.Clean(requestDir)` 必须以 `filepath.Clean(allowedDir)` 开头，
且边界必须在路径分隔符上。例如 `/home/user/projects` 允许 `/home/user/projects/foo`，
但不允许 `/home/user/projects-evil`。

`workingDir` 默认值本身不受白名单限制（它是管理员配置的可信值）。

### D8: Sandbox 与 Gateway 安全分层

Gateway 面向网络，安全模型比 CLI 更严格，采用三层防护：

| 层次 | 机制 | 作用 |
|------|------|------|
| **L1: 认证** | Bearer Token | 阻止未授权访问 |
| **L2: 目录管控** | allowedWorkDirs | 限制 agent 可操作的文件系统范围 |
| **L3: 系统沙箱** | sandbox (bwrap) | OS 级隔离，限制文件读写、网络等 |

三层独立配置，互不依赖：
- 仅开 L1 → 本地可信用户场景
- L1 + L2 → 多用户/多项目场景
- L1 + L2 + L3 → 面向公网或高安全要求场景

Sandbox 配置复用 `settings.json` 中的 `sandbox` 字段（`allowedRead`, `deniedPaths`, `passEnv` 等），
`gateway.json` 的 `sandbox.enabled` / `sandbox.level` 仅控制是否启用和级别覆盖。
这与 CLI `--sandbox` flag 的行为一致。

### D9: System Prompt 处理可配置

通过 `systemPromptMode` 控制客户端 system message 的处理方式：
- `"append"` (默认): 追加到内置 system prompt 末尾。保留 tool 说明、mode 指令，同时接受客户端补充指令。
- `"ignore"`: 忽略客户端 system message。完全使用内置 prompt，防止客户端干扰 agent 行为。

选择 `"append"` 是因为大多数 OpenAI 客户端都会发 system message（例如 Cursor、Open WebUI），
完全忽略会让用户困惑。追加模式既保留了 VibeCoding 的完整能力，又尊重客户端的自定义指令。

### D10: --init-gateway 配置初始化

`vibecoding --init-gateway` 生成 `gateway.json` 模板到 `~/.config/vibecoding/gateway.json`。

行为：
- 文件不存在 → 创建并写入默认模板
- 文件已存在 → 提示已存在，不覆盖
- `--force` → 强制覆盖

模板内容包含所有字段及注释说明，用户只需取消注释并填写即可。
实现位置: `internal/gateway/config.go` 中的 `DefaultGatewayConfig()` + `SaveGatewayConfig()`。
这与 `ensureConfigExists()` 写 `settings.json` 的模式一致。

## 11. 风险与注意事项

| 风险 | 缓解 |
|------|------|
| Agent loop 挂起（tool 执行超时） | 请求级 context timeout（默认 5 分钟），可配置 |
| 内存膨胀（大量 session） | idleTimeout 自动清理 + maxSessions 限制 |
| 并发安全 | session 级 mutex + pool 级 RWMutex |
| tool 执行安全 | allowedWorkDirs 白名单 + sandbox 可选开启；建议公网部署开启 sandbox |
| 目录穿越 | allowedWorkDirs 前缀匹配 + filepath.Clean 规范化 |
| token 泄露 | gateway.json 建议 0600 权限；token 支持环境变量引用 |
| 长连接 SSE 断开 | client context cancel → agent.Abort() |

## 12. 使用示例

### 本地开发（无认证）

```bash
# 启动
vibecoding gateway

# 测试
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "list files in current directory"}],
    "stream": false
  }'
```

### 有认证

```bash
vibecoding gateway

curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-my-secret-token" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "explain main.go"}],
    "stream": true
  }'
```

### Python OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="sk-my-secret-token",  # 如果开启了认证
)

response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[
        {"role": "system", "content": "You are a coding assistant."},
        {"role": "user", "content": "Read main.go and explain the architecture."},
    ],
    stream=True,
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### 多轮对话（带 session）

```python
# 第一轮
response1 = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "read main.go"}],
    extra_body={"x_session_id": "my-session-1"},
)

# 第二轮（同 session，agent 记住了上下文）
response2 = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "now refactor the error handling"}],
    extra_body={"x_session_id": "my-session-1"},
)
```

## 13. 待讨论

所有原待讨论项均已决定，见下方汇总。如有新议题再追加。

### 已决定事项

| # | 议题 | 决定 | 对应配置字段 |
|---|--------|------|---------------|
| 1 | Tool 可见性 | 默认 `content` 模式（混入 `content` 字段），可配为 `sse_event` 或 `none` | `toolVisibility.mode` |
| 2 | System prompt | 默认 `append`（追加到内置 prompt 末尾），可配为 `ignore` | `systemPromptMode` |
| 3 | Working directory | `allowedWorkDirs` 白名单 + sandbox 双重保护 | `allowedWorkDirs` |
| 4 | 请求超时 | 默认 5 分钟，streaming 有数据流动不超时 | `requestTimeoutSeconds` |
| 5 | 并发限制 | 默认不限制，可配置 | `maxConcurrentRequests` |
