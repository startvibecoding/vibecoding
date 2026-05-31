# 系统架构

## 项目结构

```
vibecoding/
├── agent/                       # 公共 Agent/Provider 接口与 Builder
├── cmd/vibecoding/              # CLI 入口点
│   └── main.go                  # 主程序
├── internal/
│   ├── a2a/                     # A2A 协议服务器与 Master 模式
│   │   ├── config.go            # A2A 配置与初始化
│   │   ├── handler.go           # JSON-RPC 2.0 handler + SSE
│   │   ├── client.go            # A2A 客户端
│   │   ├── server.go            # HTTP 服务器
│   │   ├── executor.go          # Task → Agent loop 执行器
│   │   ├── agent_card.go        # Agent Card 生成
│   │   ├── task.go              # Task 生命周期管理
│   │   └── master.go            # A2A Master 模式（远程 agent 调度）
│   ├── acp/                     # ACP / MCP 集成
│   ├── agent/                   # 核心 Agent 循环
│   │   ├── agent.go             # Agent 主逻辑
│   │   ├── factory.go           # AgentFactory，统一每个 Agent 的创建
│   │   ├── manager.go           # AgentManager 生命周期管理
│   │   ├── router.go            # EventRouter
│   │   ├── subagent.go          # subagent_* 工具
│   │   ├── events.go            # 事件类型定义
│   │   ├── provider.go          # Provider 接口适配
│   │   └── system_prompt.go     # 系统提示词生成
│   ├── config/                  # 配置管理
│   ├── context/                 # 上下文管理和 token 估算
│   ├── contextfiles/            # 上下文文件加载
│   ├── cron/                    # 定时任务存储和调度器
│   ├── gateway/                 # OpenAI 兼容 HTTP 网关
│   ├── hermes/                  # 消息平台网关 (微信/飞书/WebSocket)
│   ├── mcp/                     # MCP 服务器集成
│   ├── memory/                  # 持久化记忆 (memory.md)
│   ├── messaging/               # 消息平台抽象
│   ├── platform/                # 跨平台兼容工具
│   ├── provider/                # LLM Provider 抽象
│   │   ├── anthropic/           # Anthropic Messages API
│   │   ├── factory/             # 共享 provider/model 创建逻辑
│   │   ├── vendor*.go           # 厂商适配注册和默认值
│   │   └── openai/              # OpenAI Chat Completions API
│   ├── sandbox/                 # 沙箱抽象 (bwrap, none)
│   ├── session/                 # 会话管理 (JSONL)
│   ├── skills/                  # 技能系统
│   ├── tools/                   # 工具实现
│   │   ├── bash.go              # Bash 命令执行
│   │   ├── read.go              # 文件读取
│   │   ├── write.go             # 文件写入
│   │   ├── edit.go              # 文件编辑
│   │   ├── grep.go              # 内容搜索
│   │   ├── find.go              # 文件查找
│   │   ├── ls.go                # 目录列表
│   │   ├── plan.go              # 任务规划
│   │   ├── skill_ref.go         # 技能引用加载
│   │   └── a2a_dispatch.go      # A2A 远程 agent 调度
│   ├── tui/                     # 终端 UI (BubbleTea)
│   ├── ua/                      # User-Agent 字符串生成
│   └── vendored/                # 内嵌二进制 (rg, fd)
└── pkg/sdk/                     # 公共 SDK 接口
```

## 运行模式

VibeCoding 支持 7 种运行模式，共享同一套 Agent、Provider、Tools、Session 基础设施：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        VibeCoding 运行模式                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │
│  │  TUI (默认)   │  │  Print 模式   │  │  ACP stdio   │                  │
│  │  vibecoding   │  │  vibecoding   │  │  vibecoding   │                  │
│  │              │  │  -p "..."     │  │  acp          │                  │
│  └──────────────┘  └──────────────┘  └──────────────┘                  │
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ Gateway 模式  │  │  Hermes 模式  │  │  A2A 独立模式 │  │ A2A Master │ │
│  │  vibecoding   │  │  vibecoding   │  │ vibecoding    │  │ --enable-  │ │
│  │  gateway      │  │  hermes       │  │ a2a start     │  │ a2a-master │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. Provider 系统

Provider 是与 LLM API 交互的抽象层。所有运行模式的 provider 创建都经过
`internal/provider/factory`，先应用厂商适配默认值，再构造通用 OpenAI
兼容或 Anthropic 兼容协议 provider。

```
┌─────────────────────────────────────────────────────────────┐
│                      Provider Interface                      │
├─────────────────────────────────────────────────────────────┤
│  Chat(ctx, params) <-chan StreamEvent                       │
│  Models() []*Model                                          │
│  GetModel(id string) *Model                                 │
│  Name() string                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              │
            ┌─────────────────┴─────────────────┐
            ▼                                   ▼
    ┌───────────────────┐             ┌───────────────────┐
    │ 厂商适配器         │             │ 通用 fallback      │
    │ vendor_*.go       │             │ openai/anthropic  │
    └───────────────────┘             └───────────────────┘
```

厂商选择顺序：

1. provider 配置中的显式 `vendor`
2. 根据 Base URL 自动识别
3. 根据 `api` 回退到通用协议 provider

#### StreamEvent 类型

```go
type StreamEvent struct {
    Type        EventType
    TextDelta   string      // 文本增量
    ThinkingDelta string    // 思考增量
    ToolCall    *ToolCall   // 工具调用
    Usage       *Usage      // Token 使用量
    Error       error       // 错误
}
```

### 2. Agent 循环

Agent 是核心逻辑，协调 Provider、Tools 和 Session。所有运行模式复用同一个
Agent 循环，区别在于输入来源（终端、HTTP、消息平台、stdio）和输出目标。

```
┌─────────────────────────────────────────────────────────────┐
│                       Agent Loop                             │
├─────────────────────────────────────────────────────────────┤
│  1. 构建系统提示词 (模式 + 工具 + 上下文文件 + 技能)         │
│  2. 发送消息到 Provider                                      │
│  3. 处理流式事件 (文本、思考、工具调用)                       │
│  4. 执行工具并收集结果                                        │
│  5. 将工具结果添加到消息                                      │
│  6. 重复直到完成                                              │
└─────────────────────────────────────────────────────────────┘
```

#### 执行流程

```
User Input (TUI / HTTP / Messaging / A2A / ACP stdio)
    │
    ▼
┌───────────────┐
│ Build Context │ ← System Prompt + Tools + Context Files + Skills
└───────┬───────┘
        │
        ▼
┌───────────────┐
│  Provider     │ ← LLM API (OpenAI/Anthropic)
│  Chat()       │
└───────┬───────┘
        │
        ▼
┌───────────────┐     ┌───────────────┐
│ Stream Events │────▶│ Tool Calls?   │
└───────┬───────┘     └───────┬───────┘
        │                     │
        │                     ▼
        │              ┌───────────────┐
        │              │ Execute Tools │
        │              └───────┬───────┘
        │                     │
        │                     ▼
        │              ┌───────────────┐
        └──────────────│ Append Results│
                       └───────────────┘
```

### 3. 多 Agent 运行时

多 Agent 模式通过 `--multi-agent` 显式启用。启用后，主 Agent 会获得
`subagent_spawn`、`subagent_status`、`subagent_send`、`subagent_destroy`
工具。子 Agent 拥有独立的 messages、context、session、registry 和 job
manager 状态。

```
Main Agent
    │
    ├── AgentManager 创建子 Agent
    ├── EventRouter 按 AgentID 路由事件
    └── subagent_* 工具管理异步子任务
```

子 Agent 的 registry 会过滤 `subagent_*` 工具，因此不能继续创建嵌套子 Agent。

### 4. A2A 协议

A2A（Agent-to-Agent）协议使不同的 AI Agent 能够互相发现、通信和协作。

```
┌───────────────────────────────────────────────────────────────────┐
│                     A2A 协议架构                                    │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────────┐          ┌──────────────────┐              │
│  │   A2A Server     │          │   A2A Client     │              │
│  │  (vibecoding)    │ ◄──────► │  (任意 Agent)     │              │
│  │                  │  JSON-RPC │                  │              │
│  │  /a2a            │  2.0     │  SendMessage()   │              │
│  │  /a2a/send       │  + SSE   │  GetTask()       │              │
│  │  /a2a/task       │          │  CancelTask()    │              │
│  │  /a2a/events     │          │  GetAgentCard()  │              │
│  └──────────────────┘          └──────────────────┘              │
│                                                                   │
│  Task 生命周期: submitted → working → completed/failed/canceled    │
│                                                                   │
│  两种运行方式:                                                     │
│  • 独立模式: vibecoding a2a start (端口 8093)                      │
│  • 集成模式: hermes.json a2a.enabled: true (共享端口 8090)         │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

#### A2A Master 模式

A2A Master 模式通过 `--enable-a2a-master` 启用，加载 `a2a-list.json`
配置的远程 agent 列表，注册 `a2a_dispatch` tool 让 LLM 自动分发任务。

```
┌───────────────────────────────────────────────────────────────┐
│                   A2A Master 模式                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│  a2a-list.json                                                │
│  ┌─────────────────────────────────────────┐                  │
│  │ agents:                                 │                  │
│  │   - name: code-reviewer                 │                  │
│  │     url: http://review:8093             │                  │
│  │   - name: ci-agent                      │                  │
│  │     url: http://ci:8093                 │                  │
│  └─────────────────────────────────────────┘                  │
│           │                                                   │
│           ▼                                                   │
│  ┌──────────────────┐                                         │
│  │   A2AManager     │ ← 加载 agent 列表                        │
│  └────────┬─────────┘                                         │
│           │                                                   │
│           ▼                                                   │
│  ┌──────────────────┐                                         │
│  │  a2a_dispatch    │ ← 注册为 LLM tool                       │
│  │  (agent_name,    │                                         │
│  │   message)       │                                         │
│  └────────┬─────────┘                                         │
│           │                                                   │
│           ▼                                                   │
│  ┌──────────────────┐  ┌──────────────────┐                  │
│  │  code-reviewer   │  │  ci-agent        │                  │
│  │  http://review   │  │  http://ci       │                  │
│  │  :8093           │  │  :8093           │                  │
│  └──────────────────┘  └──────────────────┘                  │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

### 5. Gateway 模式

`internal/gateway/` 实现 OpenAI 兼容的 HTTP 网关，暴露标准 Chat Completions API。

```
┌─────────────────────────────────────────────────────────────┐
│                    Gateway 架构                               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  OpenAI 兼容客户端 (curl, SDK, 任意工具)                      │
│       │                                                     │
│       ▼                                                     │
│  ┌──────────────────────────────────────────┐               │
│  │  HTTP Gateway (net/http)                 │               │
│  │  POST /v1/chat/completions               │               │
│  └──────────────────────────────────────────┘               │
│       │                                                     │
│       ▼                                                     │
│  ┌──────────────────────────────────────────┐               │
│  │  Agent Loop (复用同一套)                   │               │
│  │  + Tools + Session + Sandbox + Skills     │               │
│  └──────────────────────────────────────────┘               │
│                                                             │
│  配置: gateway.json (全局 ~/.vibecoding/ 或项目 .vibe/)       │
│  安全: Bearer token + allowedWorkDirs + sandbox              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 6. Hermes 消息平台网关

`internal/hermes/` 实现消息平台网关，支持微信、飞书和 WebSocket。

```
┌─────────────────────────────────────────────────────────────┐
│                    Hermes 架构                                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  微信     │  │  飞书     │  │ WebSocket │                  │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘                  │
│        │             │             │                         │
│        └─────────────┼─────────────┘                         │
│                      ▼                                       │
│  ┌──────────────────────────────────────────┐               │
│  │  Hermes Dispatcher                       │               │
│  │  (per-user session, yolo mode default)   │               │
│  └──────────────────────────────────────────┘               │
│       │                                                     │
│       ▼                                                     │
│  ┌──────────────────────────────────────────┐               │
│  │  Agent Loop (复用同一套)                   │               │
│  │  + Tools + Session + Sandbox + Skills     │               │
│  └──────────────────────────────────────────┘               │
│                                                             │
│  配置: hermes.json                                           │
│  Session: <sessionDir>/hermes/<platform>/<user_id>/          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 7. Cron 调度器

`internal/cron` 包提供文件持久化的 cron store 和 scheduler，可通过子 Agent
或远程 A2A Server 执行任务。TUI 在多 Agent 模式下暴露 `/cron` 命令入口。

```
┌─────────────────────────────────────────────────────────────┐
│                    Cron 调度器                                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐                                       │
│  │  CronStore       │ ← cron.json 持久化                     │
│  │  (FileCronStore) │                                       │
│  └────────┬─────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌──────────────────┐                                       │
│  │  Scheduler       │ ← 定时轮询 (默认 30s)                  │
│  └────────┬─────────┘                                       │
│           │                                                 │
│     ┌─────┴─────┐                                           │
│     ▼           ▼                                           │
│  ┌───────┐  ┌───────────┐                                   │
│  │ 子Agent│  │ A2A Server│                                   │
│  │ (本地) │  │ (远程)    │  ← --a2a-target 参数               │
│  └───────┘  └───────────┘                                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 8. 工具系统

工具是 Agent 与外部世界交互的方式。所有运行模式共享同一套工具注册表。

```
┌─────────────────────────────────────────────────────────────┐
│                    Tool Interface                            │
├─────────────────────────────────────────────────────────────┤
│  Name() string                                              │
│  Description() string                                       │
│  Parameters() json.RawMessage                               │
│  Execute(ctx, params) (*ToolResult, error)                  │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  File Tools   │   │  Search Tools │   │  System Tools │
│  - read       │   │  - grep       │   │  - bash       │
│  - write      │   │  - find       │   │  - ls         │
│  - edit       │   │               │   │  - jobs       │
└───────────────┘   └───────────────┘   │  - kill       │
                                        └───────────────┘
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  Planning     │   │  Skills       │   │  A2A Master   │
│  - plan       │   │  - skill_ref  │   │  - a2a_       │
│               │   │               │   │    dispatch   │
└───────────────┘   └───────────────┘   └───────────────┘
```

### 9. 会话管理

会话使用 JSONL 格式存储，支持树状结构和分支。

```
┌─────────────────────────────────────────────────────────────┐
│                    Session Structure                         │
├─────────────────────────────────────────────────────────────┤
│  {                                                          │
│    "id": "session-abc123",                                  │
│    "type": "session",                                       │
│    "timestamp": "2024-01-01T00:00:00Z",                     │
│    "cwd": "/home/user/project",                             │
│    "provider": "anthropic",                                 │
│    "model": "claude-sonnet-4-20250514"                      │
│  }                                                          │
│  {                                                          │
│    "id": "msg-001",                                         │
│    "parentId": "session-abc123",                            │
│    "type": "message",                                       │
│    "role": "user",                                          │
│    "content": "..."                                         │
│  }                                                          │
│  {                                                          │
│    "id": "msg-002",                                         │
│    "parentId": "msg-001",                                   │
│    "type": "message",                                       │
│    "role": "assistant",                                     │
│    "content": "..."                                         │
│  }                                                          │
└─────────────────────────────────────────────────────────────┘
```

#### 会话类型

| type | 描述 |
|------|------|
| `session` | 会话元数据 |
| `message` | 用户/助手消息 |
| `model_change` | 模型切换记录 |
| `compaction` | 上下文压缩记录 |
| `label` | 会话标签 |

### 10. 沙箱系统

沙箱通过 bubblewrap (bwrap) 实现进程隔离。

```
┌─────────────────────────────────────────────────────────────┐
│                     Sandbox Manager                          │
├─────────────────────────────────────────────────────────────┤
│  SetLevel(level)                                            │
│  GetActive() *Sandbox                                       │
│  Execute(cmd) (stdout, stderr, error)                       │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  LevelNone    │   │ LevelStandard │   │  LevelStrict  │
│  (无限制)     │   │ (项目读写)    │   │  (项目只读)   │
└───────────────┘   └───────────────┘   └───────────────┘
```

### 11. TUI 系统

基于 BubbleTea 的终端用户界面。

```
┌─────────────────────────────────────────────────────────────┐
│                        TUI App                              │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Header Bar                          │  │
│  │  Provider: anthropic │ Model: claude-sonnet-4 │ Mode  │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Message Area                        │  │
│  │  User: ...                                            │  │
│  │  Assistant: ...                                        │  │
│  │  [tool: bash] running...                              │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Input Area                          │  │
│  │  > _                                                  │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Status Bar                          │  │
│  │  Thinking: medium │ Tokens: 1234 in / 567 out │ Cost  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 配置文件总览

| 文件 | 位置 | 用途 |
|------|------|------|
| `settings.json` | `~/.vibecoding/` 或 `.vibe/` | 核心设置（provider、model、mode 等） |
| `gateway.json` | `~/.vibecoding/` 或 `.vibe/` | HTTP 网关配置 |
| `hermes.json` | `~/.vibecoding/` 或 `.vibe/` | 消息平台网关配置 |
| `a2a.json` | `~/.vibecoding/` 或 `.vibe/` | A2A 服务器配置 |
| `a2a-list.json` | `~/.vibecoding/` 或 `.vibe/` | A2A Master 远程 agent 列表 |
| `mcp.json` | `~/.vibecoding/` 或 `.vibe/` | MCP 服务器配置 |
| `memory.md` | 项目根目录或 `~/.vibecoding/` | 持久化记忆 |
| `cron.json` | `~/.vibecoding/` | 定时任务持久化 |

## 数据流

### 完整请求流程

```
1. 用户输入 (来自 TUI / HTTP / Messaging / A2A / ACP stdio)
   │
   ▼
2. 输入层捕获
   │
   ▼
3. Agent.Run(ctx, input)
   │
   ▼
4. 构建系统提示词
   ├── 模式提示 (plan/agent/yolo)
   ├── 工具定义 (JSON Schema)
   ├── 上下文文件 (AGENTS.md, CLAUDE.md)
   └── 技能上下文
   │
   ▼
5. 构建消息历史
   ├── 历史消息 (from Session)
   └── 新用户消息
   │
   ▼
6. Provider.Chat(ctx, params)
   │
   ▼
7. SSE 流式响应
   ├── TextDelta → 显示文本
   ├── ThinkingDelta → 显示思考
   └── ToolCall → 执行工具 (含 a2a_dispatch)
   │
   ▼
8. 工具执行 (通过 Sandbox)
   │
   ▼
9. 收集工具结果
   │
   ▼
10. 继续对话 (回到步骤 5)
   │
   ▼
11. 完成，保存会话
```

## 关键设计决策

### 1. 接口抽象

使用接口抽象 Provider 和 Tool，便于扩展和测试。

### 2. 流式处理

使用 Channel 实现流式响应，提供实时反馈。

### 3. 会话树

使用树状结构存储会话，支持分支和恢复。

### 4. 分层配置

支持全局和项目配置，项目配置覆盖全局。

### 5. 沙箱隔离

通过 bubblewrap 实现进程级隔离，保护系统安全。

### 6. 公共 SDK 包

`agent/` 包暴露公共 Go 类型（`Agent`、`Provider`、`Builder`），外部应用可以
在不依赖 internal 包的情况下嵌入 Agent。
详见 [SDK 集成指南](sdk.md)。

### 7. 复用 Agent 循环

所有运行模式（TUI、Gateway、Hermes、A2A、ACP）复用同一个 Agent 循环，
区别仅在于输入来源和输出目标。这保证了行为一致性，避免了逻辑分叉。
