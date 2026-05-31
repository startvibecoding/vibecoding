# v0.1.27 Hermes 模式 — 研发计划

> **日期**: 2026-05-29
> **目标版本**: v0.1.27
> **状态**: 🔧 开发进行中（核心功能已完成）
> **审核日期**: 2026-05-30
> **整体进度**: 100%（所有功能已实现，文档已完成）
> **v2 修订**: 2026-05-30 — 基于实现审核重新梳理优先级和范围

---

## 1. 概述

VibeCoding 当前提供三种运行模式：**CLI (TUI)**、**ACP (编辑器集成)**、**Gateway (HTTP API)**。

本提案引入第四种运行模式 **`hermes`** — 通过 `vibecoding hermes` 子命令启动，提供**消息平台网关 + 自动化调度 + 持久化记忆**等能力，让 VibeCoding 从"编码助手"扩展为"可部署的自主代理"。

### 设计哲学

- **渐进式采纳**：Hermes 模式是对现有 CLI/Gateway 的增强，不是替代
- **复用优先**：尽量复用已有的 agent loop、provider、tools、session、sandbox 基础设施
- **Go 原生**：VibeCoding 是 Go 项目，不移植 Python 生态，只借鉴架构思路
- **缓存友好**：memory 等动态内容通过 tool call 按需加载（同 `skill_ref`），不注入 system prompt，保护 prompt cache 命中率

---

## 2. 配置目录约定

VibeCoding 使用 **全局 + 项目级** 的两层配置体系，项目级优先级更高。

### 2.1 全局配置目录 `<GLOBAL_DIR>`

存放全局默认配置、凭证、sessions、skills 等。路径因平台而异：

| 平台 | 默认路径 | 来源 |
|------|----------|------|
| **Linux** | `~/.vibecoding/` | `platform.ConfigDir()` |
| **macOS** | `~/Library/Application Support/vibecoding/` | `platform.ConfigDir()` |
| **Windows** | `%APPDATA%\vibecoding\` | `platform.ConfigDir()` |
| **自定义** | `$VIBECODING_DIR` | 环境变量覆盖，优先级最高 |

> 后文中 `<GLOBAL_DIR>` 均指上述路径。Linux 下即 `~/.vibecoding/`。

全局目录下的文件布局：

```
<GLOBAL_DIR>/
├── settings.json               # 全局 agent/provider 配置
├── gateway.json                # 全局 Gateway 配置
├── hermes.json                 # 全局 Hermes 配置（本提案新增）
├── mcp.json                    # MCP 工具服务配置
├── memory.md                   # 全局持久化记忆（本提案新增）
├── wechat-credentials.json     # 微信 iLink 凭证（本提案新增）
├── sessions/                   # JSONL 会话存储
└── skills/                     # 全局 skills
```

### 2.2 项目级配置目录 `.vibe/`

存放项目专属的配置覆盖，位于项目工作目录根下。**项目级配置优先级高于全局配置**，加载顺序：

```
defaults → <GLOBAL_DIR>/<file> → .vibe/<file>
```

即：先加载内置默认值，再加载全局配置，最后用项目级配置覆盖合并。

项目级目录下的文件布局：

```
<project>/
└── .vibe/
    ├── settings.json           # 项目级 agent/provider 配置覆盖
    ├── gateway.json            # 项目级 Gateway 配置覆盖
    ├── hermes.json             # 项目级 Hermes 配置覆盖（本提案新增）
    ├── memory.md               # 项目级持久化记忆（本提案新增）
    └── skills/                 # 项目级 skills
```

### 2.3 各配置文件的层级关系

| 配置文件 | 全局路径 | 项目级路径 | 合并策略 |
|----------|----------|------------|----------|
| `settings.json` | `<GLOBAL_DIR>/settings.json` | `.vibe/settings.json` | 深度合并（已实现） |
| `gateway.json` | `<GLOBAL_DIR>/gateway.json` | `.vibe/gateway.json` | JSON overlay（已实现） |
| `hermes.json` | `<GLOBAL_DIR>/hermes.json` | `.vibe/hermes.json` | ✅ JSON overlay（已实现，`LoadHermesConfig()` 使用 `json.Unmarshal` 覆盖合并） |
| `memory.md` | `<GLOBAL_DIR>/memory.md` | `.vibe/memory.md` | ✅ 项目级存在时**只读项目级**（已实现，`store.go` `Resolve()` 按优先级查找） |

### 2.4 memory.md 查找逻辑

> ✅ **已实现** — `internal/memory/store.go` 的 `Resolve()` 方法完整实现了以下优先级。

memory 工具查找记忆文件时遵循以下优先级：

1. `hermes.json` 中 `memory.path` 显式指定 → 使用指定路径（可以是全局目录）
2. `.vibe/memory.md` 存在 → 使用项目级记忆
3. `<GLOBAL_DIR>/memory.md` → fallback 到全局记忆
4. 均不存在 → 首次写入时创建于 `.vibe/memory.md`（项目上下文中）或 `<GLOBAL_DIR>/memory.md`（无项目上下文时）

> **设计意图**：项目级记忆记录项目相关的上下文（架构决策、代码约定等），全局记忆记录用户偏好和跨项目知识。两者不合并，避免无关项目的记忆干扰当前上下文。
>
> **默认行为**：memory.md 默认写入项目目录（`.vibe/memory.md`），只有在 `hermes.json` 中显式配置 `memory.path` 时才写入全局目录。

---

## 3. 已确认的决策

| 决策项 | 结论 | 备注 |
|--------|------|------|
| 消息平台 v0.1.27 | **微信 (iLink) + 飞书** | 微信参考 iLink 协议自行实现；飞书用官方 SDK 长连接 |
| 消息平台 v0.1.28+ | Telegram → Discord | 延后 |
| 企业微信 | **不做** | 用个人微信 iLink 协议 |
| Web 搜索工具 | **不做** | 用户通过第三方 skill 自行扩展 |
| 记忆存储 | **memory.md** | Markdown 文件，人类可读；项目级 `.vibe/memory.md` 优先于全局 `<GLOBAL_DIR>/memory.md` |
| 记忆注入方式 | **通过 `memory` 工具按需读取**，同 `skill_ref` 模式 | 不注入 system prompt，保护缓存命中 |
| 配置文件 | **hermes.json** — 独立配置文件 | 同 gateway.json 模式，`.vibe/hermes.json` 覆盖 `<GLOBAL_DIR>/hermes.json` |
| Shell Hooks | **外部脚本** — JSON stdin/stdout 通信 | 语言无关 |
| Checkpoints/Rollback | **不做** — 推迟到后续版本 | 降低 v0.1.27 范围 |
| Session 策略 | **单 session + 命令新建** | 每个 `platform:user_id` 默认一个持久 session，`/new` 强制新建；各平台独立不打通 |
| Session 存储 | **`<sessionDir>/hermes/` 隔离** | 与 CLI session 分开存储，行为差异大 |
| A2A 协议 | **采纳** — 独立子命令 `vibecoding a2a`，hermes 通过配置启用 | 详见 §5.3 |
| Cron 实现 | **CLI 命令范围已确定** | list/add/remove/enable/disable 已满足需求，edit/run 不做。底层 cron 实现与项目共享，有 bug 或缺陷仍需修复完善 |
| Smart Approvals | **已实现** | 方案 D 分级策略，WebSocket 高风险阻塞审批，消息平台高风险自动拒绝+通知 |
| Budget Pressure | **已实现** | Event 通知模式，剩余 20% 时触发一次，阈值可配置 |

---

## 4. 能力清单

### 🟢 v0.1.27 采纳

| # | 能力 | 状态 | 实现思路 |
|---|------|------|----------|
| 1 | **微信 Bot (iLink 协议)** | ✅ **已完成** | `internal/messaging/wechat/` — 5 个文件完整实现，纯标准库零外部依赖 |
| 2 | **飞书 Bot** | ✅ **已完成** | `internal/messaging/feishu/feishu.go` — 官方 SDK WebSocket 长连接 |
| 3 | **消息 Session 管理** | ✅ **已完成** | `dispatcher.go` — per-user 单 session + `/new` 归档 |
| 4 | **用户白名单** | ✅ **已完成** | `security.go` CheckUserAllowed() |
| 5 | **Cron** | ✅ **已完成（CLI 范围确定）** | list/add/remove/enable/disable，scheduler 依赖 multi-agent。底层实现与项目共享，有缺陷仍需修复 |
| 6 | **持久化记忆 (memory.md)** | ✅ **已完成** | `memory/store.go` + `tool.go` — 完整 CRUD |
| 7 | **User Profile** | ✅ **已完成** | memory.md 默认模板 |
| 8 | **Budget Pressure** | ✅ **已完成** | `agent.go` loop: 剩余 20% 迭代时触发 `EventBudgetPressure`（一次性），dispatcher 转发到消息平台 |
| 9 | **Context Pressure** | ✅ **已完成** | `agent.go` loop: 55% context 使用率时触发 `EventContextPressure`（一次性），上层决策处理。hermes dispatcher 转发到消息平台 |
| 10 | **Smart Approvals** | ✅ **已完成** | 方案 D 分级策略：low→自动批准 / medium→批准+通知 / high→WebSocket 等待审批(5min超时) / 消息平台自动拒绝+通知 |
| 11 | **Shell Hooks** | ✅ **已完成** | `hooks/hooks.go` pre/post 外部脚本 |
| 12 | **Webhook 入站** | ✅ **已完成** | `webhook/router.go` + `webhook_handler.go` |
| 13 | **A2A 协议 (Server)** | ✅ **已完成** | `internal/a2a/` 独立顶层包，JSON-RPC 2.0 over HTTP + SSE 流式，独立模式 + hermes 集成模式 |
| 14 | **WebSocket 流式推送** | ✅ **已完成** | `wsDispatcherAdapter` 逐事件转换 agent.Event → WSEvent，支持 text_delta/think_delta/tool_call/tool_result/usage/done |
| 15 | **hermes stop/status** | ✅ **已完成** | PID 文件 + SIGTERM 信号 + HTTP health 查询 |
| 16 | **hermes client** | ✅ **已完成** | `internal/hermes/client.go` WebSocket 客户端，支持流式输出 + 斜杠命令 |
| 17 | **webhook/memory/sessions CLI** | ✅ **已完成** | webhook list、memory show/clear、sessions list（查询运行实例）|
| 18 | **/api/memory HTTP** | ✅ **已完成** | GET 读取 memory.md（含 source/path）、PUT 更新 memory.md，集成 MemoryStore |

### 🟡 延后（v0.1.28+）

| 能力 | 原因 |
|------|------|
| Checkpoints / Rollback | 已确认推迟 |
| 其他消息平台 | Email, Matrix, Mattermost 等 |
| 图片生成 / Voice Mode | 非核心 |


### 🔴 不做

| 能力 | 原因 |
|------|------|
| **Web 搜索** | 用户通过第三方 skill 自行扩展 |
| **企业微信** | 用个人微信 iLink 协议代替 |
| WhatsApp / Signal / SMS | 外部依赖重 |
| Python Plugins | Go 项目 |
| RL Training / Batch | Python 生态 |

---

## 5. 消息平台技术方案

### 5.1 微信 iLink（优先级 #1）

**实现方式**: 根据 iLink 协议规范自行实现（参考 `/home/free/src/wechatbot/golang` 中的协议实现），**不引入外部依赖**。协议层约 1600 行纯标准库代码，直接写入 `internal/messaging/wechat/`

| 维度 | 方案 |
|------|------|
| **认证** | QR 码扫码登录，凭证持久化到 `<GLOBAL_DIR>/wechat-credentials.json` |
| **消息接收** | **长轮询** (`getupdates`)，无需公网 IP |
| **消息发送** | `sendmessage` API，支持文本/图片/文件/视频 |
| **Typing 指示** | 支持（`getconfig` → `sendtyping`） |
| **CDN 媒体** | AES-128-ECB 加密上传/下载 |
| **会话恢复** | `context_token` 自动管理；session 过期（errcode -14）自动重新登录 |
| **优势** | 无需公网暴露；个人微信即可；长轮询天然可靠 |

**代码结构**（参考 iLink 协议，VibeCoding 内部包自行实现）：

```
internal/messaging/wechat/
├── wechat.go      # Bot 主体 + 消息处理（实现 messaging.Platform）
├── types.go       # iLink 协议类型定义
├── protocol.go    # iLink HTTP API 调用（getupdates/sendmessage/getconfig 等）
├── auth.go        # QR 码登录 + 凭证持久化
└── crypto.go      # AES-128-ECB CDN 加密/解密
```

全部使用 Go 标准库（`crypto/aes`、`net/http`、`encoding/json`），**零外部依赖**。

**核心 API 端点**（来自 iLink 协议）：

| 端点 | 作用 |
|------|------|
| `GET /ilink/bot/get_bot_qrcode` | 获取 QR 码 |
| `GET /ilink/bot/get_qrcode_status` | 轮询扫码状态 |
| `POST /ilink/bot/getupdates` | 长轮询接收消息 |
| `POST /ilink/bot/sendmessage` | 发送消息 |
| `POST /ilink/bot/getconfig` | 获取 typing ticket |
| `POST /ilink/bot/sendtyping` | 发送/取消打字指示 |

### 5.2 飞书（优先级 #2）

**依赖**: `github.com/larksuite/oapi-sdk-go/v3` — 飞书官方 Go SDK

参考文档: https://open.feishu.cn/document/server-side-sdk/golang-sdk-guide/preparations

| 维度 | 方案 |
|------|------|
| **SDK** | 飞书官方 Go SDK v3 |
| **消息接收** | **长连接** (WebSocket)，无需公网 IP |
| **消息发送** | REST API (飞书 IM 接口) |
| **认证** | App ID + App Secret |
| **消息类型** | 文本、富文本、Markdown、卡片消息 |
| **创建步骤** | 飞书开放平台 → 创建应用 → 开启机器人能力 → 配置事件订阅 |
| **优势** | WebSocket 无需公网；官方 SDK 维护有保障；卡片消息表现力强 |

**飞书长连接模式关键点**：
- 使用 `larkws` 包建立 WebSocket 长连接
- 订阅 `im.message.receive_v1` 事件接收消息
- 无需配置回调 URL，适合内网/开发环境
- 自动断线重连

### 5.3 A2A 协议 (Agent-to-Agent)

> ✅ **已完成** — `internal/a2a/` 独立顶层包，零外部依赖实现 JSON-RPC 2.0 over HTTP + SSE 流式。支持独立模式（`vibecoding a2a start`）和集成模式（hermes + `a2a.enabled: true`）。

**依赖**: `github.com/a2aproject/a2a-go/v2` — Google A2A 官方 Go SDK

**A2A 是什么**：Google 主导的开放协议，让不同框架、不同厂商的 AI Agent 能够互相发现、通信和协作，在不暴露内部状态的前提下完成复杂任务。

#### 命令设计

```
vibecoding a2a
├── start                 # 启动独立 A2A Server（不依赖 hermes）
│   ├── --port <port>     # 监听端口（默认 8093）
│   ├── --work-dir <dir>  # 工作目录
│   ├── -p, --provider    # 默认 provider
│   ├── -m, --model       # 默认 model
│   └── --sandbox         # 启用 sandbox
├── stop                  # 停止 A2A Server
├── status                # 查看 A2A Server 状态
└── card                  # 查看/生成 Agent Card
```

#### 两种运行模式

| 模式 | 命令 | 端口 | 说明 |
|------|------|------|------|
| **独立模式** | `vibecoding a2a start` | 8093 | 独立运行，有自己的 HTTP 端口和 agent loop |
| **集成模式** | `vibecoding hermes start` + `a2a.enabled: true` | 8090 (共享) | A2A 端点挂载到 hermes 的 HTTP 端口上 |

**集成模式**：hermes 启动时，如果 `hermes.json` 中 `a2a.enabled: true`，自动将 A2A 端点注册到 hermes 的 HTTP mux 上：
- `/.well-known/agent.json` → Agent Card
- `/a2a` → JSON-RPC 2.0 handler
- 复用 hermes 的认证、dispatcher、agent loop 基础设施

**独立模式**：`vibecoding a2a start` 启动独立的 HTTP 服务器，适用于不需要消息平台但需要 A2A 能力的场景。

#### 协议细节

| 维度 | 方案 |
|------|------|
| **角色** | A2A Server（接收外部 Agent 的任务请求） |
| **传输** | JSON-RPC 2.0 over HTTP（同步 + SSE 流式） |
| **Agent Card** | `/.well-known/agent.json` 发布能力描述 |
| **Task 生命周期** | submitted → working → completed/failed |
| **认证** | Bearer token（复用 Gateway 的认证机制） |
| **流式响应** | SSE 实时推送 Task 状态和 Artifact 更新 |

**与现有协议的关系**：

| 协议 | 角色 | 关系 |
|------|------|------|
| **ACP** (Agent Client Protocol) | 编辑器 ↔ Agent | 已有，用于 IDE 集成 |
| **MCP** (Model Context Protocol) | Agent ↔ 工具服务 | 已有，让 Agent 调用外部工具 |
| **A2A** (Agent-to-Agent) | Agent ↔ Agent | **新增**，Agent 间对等协作 |
| **Gateway** (OpenAI 兼容) | 应用 ↔ LLM API | 已有，应用调 VibeCoding 当 LLM |

**A2A Server 暴露的能力 (Agent Card)**：

```json
{
  "name": "VibeCoding",
  "description": "AI coding assistant with file editing, terminal, and search capabilities",
  "url": "http://localhost:8093/a2a",
  "version": "0.1.27",
  "capabilities": {
    "streaming": true,
    "pushNotifications": false
  },
  "skills": [
    {
      "id": "code-edit",
      "name": "Code Editing",
      "description": "Read, write, and edit code files with precise text replacement"
    },
    {
      "id": "terminal",
      "name": "Terminal Execution",
      "description": "Execute shell commands, run tests, build projects"
    },
    {
      "id": "code-search",
      "name": "Code Search",
      "description": "Search codebases with ripgrep and fd"
    }
  ]
}
```

**实现方式**：外部 Agent 通过 A2A SendMessage 发送任务 → dispatcher 创建 agent loop 处理 → 通过 SSE 流式返回结果。复用与消息平台相同的 agent 基础设施。

#### 代码结构

```
internal/a2a/                     # 独立于 hermes 的顶层包
├── server.go                    # A2A HTTP server（独立模式 + 集成模式）
├── handler.go                   # JSON-RPC 2.0 handler（SendMessage / GetTask / CancelTask）
├── agent_card.go                # Agent Card 生成 (/.well-known/agent.json)
├── task.go                      # Task 生命周期管理（submitted → working → completed/failed）
├── executor.go                  # AgentExecutor（A2A Task → agent loop）
├── sse.go                       # SSE 流式响应
└── config.go                    # A2A 配置
```

#### hermes.json 集成配置

```jsonc
{
  // hermes.json 中启用 A2A
  "a2a": {
    "enabled": true,           // 启用后将 A2A 端点挂载到 hermes HTTP 端口
    "port": 8093,              // 独立模式端口（集成模式忽略）
    "agent_card": {            // 可选：自定义 Agent Card
      "name": "VibeCoding",
      "description": "AI coding assistant"
    }
  }
}
```

---

## 6. memory.md 设计

### 6.1 核心原则：不破坏缓存命中

> ✅ **已实现** — memory 通过 `memory` 工具按需读写，system prompt 仅有静态提示行。

**关键设计决策**：memory.md 的内容 **不注入 system prompt**。

原因：system prompt 是 prompt cache 的主要命中区域。如果每次都把变化的 memory 内容注入 system prompt，会导致缓存失效，增加成本和延迟。

**实现方式**：memory 通过 `memory` 工具按需读写，与 `skill_ref` 工具的设计模式一致。Agent 在需要时主动调用 `memory(action="read")` 获取记忆，而不是被动接收注入。

### 6.2 文件位置与查找优先级

memory.md 遵循全局/项目级两层配置体系（详见第 2 节）：

| 优先级 | 路径 | 用途 |
|--------|------|------|
| 1 (最高) | `hermes.json` 中 `memory.path` 显式指定 | 自定义路径 |
| 2 | `.vibe/memory.md` | 项目级记忆（项目相关的上下文） |
| 3 | `<GLOBAL_DIR>/memory.md` | 全局记忆（用户偏好、跨项目知识） |

首次写入时：有项目上下文 → 创建 `.vibe/memory.md`；无项目上下文 → 创建 `<GLOBAL_DIR>/memory.md`。

### 6.3 格式

```markdown
# Agent Memory

## User Profile

- 用户偏好使用中文交流
- Go 为主要开发语言
- 项目使用 Cobra + Bubble Tea 技术栈
- 编辑器偏好: VSCode + Vim 键位

## Working Memory

- vibecoding 项目版本当前为 v0.1.26，下一个版本 v0.1.27
- 用户对消息平台的优先级：微信 > 飞书 > Telegram > Discord
- settings.json 中 provider 配置不要随意改动 schema

## Lessons Learned

- edit 工具的 oldText 必须在文件中唯一匹配，不要用太大的上下文
- 用户不喜欢过多的确认提示，yolo 模式下直接执行
- 中文文档要和英文文档同步更新
```

### 6.4 memory 工具设计

> ✅ **已实现** — `internal/memory/tool.go` 完整实现了 read/add/update/delete 四种操作，section 级读写。

```
memory(action="read")
  → 返回 memory.md 全文（Agent 按需调用）

memory(action="read", section="User Profile")
  → 返回指定 section 内容

memory(action="add", section="Working Memory", content="新的记忆条目")
  → 在指定 section 末尾追加条目

memory(action="update", section="Working Memory", old="旧内容", new="新内容")
  → 更新指定条目

memory(action="delete", section="Working Memory", content="要删除的条目")
  → 删除指定条目
```

### 6.5 System Prompt 中的提示（轻量级，不含数据）

> ✅ **已实现** — `internal/memory/tool.go` 的 `PromptGuidelines()` 返回这行静态提示。

在 system prompt 的 Guidelines 中添加一行静态提示（不影响缓存）：

```
- A persistent memory file (memory.md) is available via the `memory` tool. Read it at the start of complex tasks to recall user preferences and prior context. Update it when you learn important facts about the user or project.
```

这行提示是**静态**的，不包含 memory.md 的实际内容，所以不影响 prompt cache。

---

## 7. Session 管理设计

### 7.1 核心原则

> ✅ **已实现** — `dispatcher.go` 的 `resolveSession()` + `RotateSession()` 完整实现了以下逻辑。

**单 session 默认 + 命令强制新建**。消息平台用户习惯连续对话，不应每次发消息都开新 session。

| 决策 | 结论 |
|--------|------|
| 默认行为 | 每个 `platform:user_id` 自动创建一个持久 session，后续消息自动延续 |
| 新建 | 用户发送 `/new` 命令时强制新建 session，旧 session 保留不删除 |
| 跨平台 | **不打通**，同一个人的微信和飞书 session 完全独立 |
| 存储隔离 | Hermes session 存储在 `<sessionDir>/hermes/`，与 CLI session 分开 |
| context 满 | 自动 compaction，不销毁 session |

### 7.2 存储结构

Hermes session 与 CLI session 行为差异大（多用户、长期常驻、无 cwd 概念），因此用独立目录隔离：

```
<sessionDir>/
├── --<base64(cwd)>--/                  # CLI/Gateway sessions（现有，不变）
│   └── 20260529-120000_abc12345.jsonl
│
└── hermes/                             # Hermes sessions（新增）
    ├── wechat/                         # 按平台分
    │   ├── wxid_user1/                 # 按用户分
    │   │   └── active.jsonl            # 当前活跃 session
    │   └── wxid_user2/
    │       └── active.jsonl
    ├── feishu/
    │   └── ou_user1/
    │       └── active.jsonl
    └── ws/                             # WebSocket client sessions
        └── <conn-id>/
            └── active.jsonl
```

**命名规则**：
- `active.jsonl` — 当前活跃 session，每个用户始终只有一个
- `/new` 时：`active.jsonl` → 重命名为 `<timestamp>_<id>.jsonl`（归档），然后创建新的 `active.jsonl`
- 归档的 session 保留在同一用户目录下，可通过 `/sessions` 查看历史

示例：`/new` 之后：

```
hermes/wechat/wxid_user1/
├── active.jsonl                        # 新 session
└── 20260529-120000_abc12345.jsonl       # 归档的旧 session
```

### 7.3 Session 生命周期

```
用户首次发消息
  │
  ├─ 检查 hermes/<platform>/<user_id>/active.jsonl
  │   ├─ 存在 → 加载并继续对话
  │   └─ 不存在 → 创建新 active.jsonl（cwd = 平台配置的 work_dir）
  │
  ├─ 持续对话… (消息追加到 active.jsonl)
  │
  ├─ context 接近上限 → 自动 compaction（不新建 session）
  │
  ├─ 用户发送 /new
  │   ├─ active.jsonl 重命名为 <timestamp>_<id>.jsonl
  │   └─ 创建新的 active.jsonl
  │
  └─ 用户发送 /sessions
      └─ 列出当前 + 历史 sessions
```

### 7.4 消息平台命令

> ⚠️ **部分实现** — `/new`、`/clear`、`/sessions`、`/status`、`/mode` 已实现；`/compact` 是 stub（仅返回字符串，未实际触发 compaction）。

消息平台用户通过发送文本命令管理 session：

| 命令 | 作用 | 状态 |
|------|------|------|
| `/new` | 归档当前 session，创建新的空 session | ✅ 已实现 |
| `/clear` | 清空当前 session 的对话历史（不归档，直接重置） | ✅ 已实现（实际行为是归档+新建，同 `/new`） |
| `/sessions` | 列出当前 + 历史 session（显示创建时间、消息数、预览） | ⚠️ 仅列出活跃 session，不显示历史归档 |
| `/status` | 查看当前 session 状态（模型、token 用量、工作目录） | ⚠️ 显示 session/mode/messages/workdir，无 token 用量 |
| `/compact` | 手动触发 context compaction | ❌ Stub — 仅返回固定字符串 |
| `/mode <mode>` | 切换模式（plan/agent/yolo） | ✅ 已实现 |

### 7.5 与现有 session.Manager 的关系

Hermes 完全复用现有的 `session.Manager` 进行 JSONL 读写，只在上层包装路由逻辑：

```go
// hermes/dispatcher.go

// resolveSession 查找或创建用户的活跃 session
func (d *Dispatcher) resolveSession(platform, userID string) (*session.Manager, error) {
    dir := filepath.Join(d.sessionDir, "hermes", platform, userID)
    activePath := filepath.Join(dir, "active.jsonl")
    
    // 已有活跃 session → 加载并继续
    if _, err := os.Stat(activePath); err == nil {
        return session.Open(activePath)
    }
    
    // 首次对话 → 创建
    os.MkdirAll(dir, 0700)
    workDir := d.resolveWorkDir(platform)
    mgr := session.New(workDir, dir)  // cwd = 平台的 work_dir
    mgr.Init()
    // 重命名 session 文件为 active.jsonl
    os.Rename(mgr.GetFile(), activePath)
    return session.Open(activePath)
}

// rotateSession 归档当前 session 并新建
func (d *Dispatcher) rotateSession(platform, userID string) (*session.Manager, error) {
    dir := filepath.Join(d.sessionDir, "hermes", platform, userID)
    activePath := filepath.Join(dir, "active.jsonl")
    
    // 归档: active.jsonl → <timestamp>_<id>.jsonl
    if mgr, err := session.Open(activePath); err == nil {
        hdr := mgr.GetHeader()
        archived := filepath.Join(dir, fmt.Sprintf("%s_%s.jsonl",
            time.Now().Format("20060102-150405"), hdr.ID[:8]))
        os.Rename(activePath, archived)
    }
    
    // 创建新的 active.jsonl
    return d.resolveSession(platform, userID)
}
```

**不改动 `session.Manager`** — Hermes 的 session 路由逻辑全部在 `hermes/dispatcher.go` 中，`session.Manager` 保持不变。

---

## 8. 子命令设计

### 8.1 命令树

> ⚠️ **大部分实现** — 仅 Smart Approvals 待讨论，其余均已实现。A2A 新增为独立子命令。

```
vibecoding hermes
├── start                 # ✅ 启动 hermes 守护进程（前台运行）
│   ├── -d                # ✅ 后台启动
│   ├── --port <port>     # ✅ 指定 WebSocket+HTTP 监听端口（默认 8090）
│   ├── --work-dir <dir>  # ✅ 默认工作目录（默认 cwd）
│   ├── -p, --provider    # ✅ 默认 provider（覆盖 hermes.json）
│   ├── -m, --model       # ✅ 默认 model（覆盖 hermes.json）
│   ├── --multi-agent     # ✅ 启用多 Agent 模式（子 Agent 工具）
│   └── --sandbox         # ✅ 启用 sandbox 模式（bwrap，默认关闭）
├── stop                  # ✅ PID 文件 + SIGTERM 停止守护进程
├── status                # ✅ PID 检查 + HTTP health 查询
│
├── client                # ✅ WebSocket 客户端（流式输出 + 斜杠命令）
│   ├── --url <ws-url>    # ✅ 连接地址（默认 ws://localhost:8090/ws）
│   └── --session <id>    # ✅ 指定/恢复 session
│
├── config
│   ├── init              # ✅ 创建 hermes.json 配置模板
│   │   ├── --global      # ✅ 写入 <GLOBAL_DIR>/hermes.json（默认）
│   │   ├── --project     # ✅ 写入 .vibe/hermes.json
│   │   └── --webhook     # ✅ 包含示例 webhook 路由
│   └── show              # ✅ 查看当前生效配置
│
├── wechat
│   ├── login             # ✅ 微信扫码登录
│   │   └── --work-dir    # ❌ 未实现
│   └── status            # ✅ 查看微信连接状态
│
├── feishu
│   ├── setup             # ⚠️ 仅打印配置说明文本
│   │   └── --work-dir    # ❌ 未实现
│   └── status            # ✅ 查看飞书连接状态
│
├── webhook
│   └── list              # ✅ 列出 webhook 路由
│
├── cron
│   ├── list              # ✅ 列出定时任务
│   ├── add               # ✅ 添加
│   ├── delete (remove)   # ✅ 删除
│   ├── enable            # ✅ 启用
│   └── disable           # ✅ 禁用
│
├── memory
│   ├── show              # ✅ 查看 memory.md 内容
│   └── clear             # ✅ 清空 memory.md
│
└── sessions
    └── list              # ✅ 查询运行实例的活跃 session
```

**新增：A2A 独立子命令**（与 hermes 平级）：

```
vibecoding a2a
├── start                 # 🔶 待实现 — 启动独立 A2A Server
│   ├── --port <port>     # 监听端口（默认 8093）
│   ├── --work-dir <dir>  # 工作目录
│   ├── -p, --provider    # 默认 provider
│   ├── -m, --model       # 默认 model
│   └── --sandbox         # 启用 sandbox
├── stop                  # 🔶 待实现 — 停止 A2A Server
├── status                # 🔶 待实现 — 查看 A2A Server 状态
└── card                  # 🔶 待实现 — 查看/生成 Agent Card
```

### 8.2 Hermes 启动流程

`vibecoding hermes start` 启动后做以下事情：

```
vibecoding hermes start
  │
  ├─ 1. 加载配置 ─────────────────────────────────
  │     <GLOBAL_DIR>/hermes.json → .vibe/hermes.json 合并
  │
  ├─ 2. 启动 WebSocket + HTTP 网关（必选，始终启动）
  │     ├── WebSocket  ws://0.0.0.0:8090/ws    # client / 第三方接入
  │     ├── HTTP REST  http://0.0.0.0:8090/    # 状态查询、webhook 入站
  │     └── A2A        http://0.0.0.0:8090/a2a # Agent-to-Agent（如启用）
  │
  ├─ 3. 连接消息平台（可选，按配置启用）
  │     ├── wechat.enabled=true  → 长轮询 iLink（需已 login 过）
  │     └── feishu.enabled=true  → WebSocket 长连接飞书 SDK
  │
  ├─ 4. 启动 Cron 调度器（如启用）
  │
  └─ 5. 就绪 ✓  等待消息
```

**关键设计**：WebSocket + HTTP 网关是 Hermes 的**核心服务**，始终启动。微信/飞书是**可选连接器**，只在配置启用且凭证就绪时才连接。即使不配置任何消息平台，Hermes 也可以通过 `hermes client` 或 WebSocket API 使用。

### 8.3 WebSocket + HTTP API 规范

Hermes 网关在单一端口（默认 `8090`）上提供所有服务，通过路由区分。

#### 8.3.1 路由总览

| 路由 | 协议 | 认证 | 状态 | 说明 |
|------|------|------|------|------|
| `/ws` | WebSocket | 是 | ✅ | 交互式对话（`hermes client` 和第三方客户端） |
| `/api/health` | GET | 否 | ✅ | 健康检查 |
| `/api/status` | GET | 是 | ✅ | 服务状态（平台连接、session 数、版本） |
| `/api/sessions` | GET | 是 | ✅ | 列出所有活跃 session |
| `/api/sessions/{id}` | GET | 是 | ✅ | 查看指定 session 详情 |
| `/api/sessions/{id}` | DELETE | 是 | ✅ | 删除指定 session |
| `/api/memory` | GET | 是 | ✅ | 读取 memory.md（含 source/path/content） |
| `/api/memory` | PUT | 是 | ✅ | 更新 memory.md |
| `/api/platforms` | GET | 是 | ✅ | 查看各消息平台状态 |
| `/webhook/*` | POST | Secret | ✅ | Webhook 入站（GitHub 等） |
| `/a2a` | POST | Bearer | ✅ | A2A JSON-RPC 2.0（message/send, task/get, task/cancel） |
| `/a2a/events` | GET | 是 | ✅ | A2A SSE 事件流（task_id 参数） |
| `/.well-known/agent.json` | GET | 否 | ✅ | A2A Agent Card |

#### 8.3.2 WebSocket 协议 (`/ws`)

> ✅ **已实现流式** — `wsDispatcherAdapter` 逐事件转换 `agent.Event` → `ws.WSEvent`，支持 text_delta/think_delta/tool_call/tool_result/tool_diff/usage/done/status/error。

客户端通过 WebSocket 连接后，与 Hermes 进行双向 JSON 消息通信。

**连接握手**：

```
GET /ws?token=<auth_token>&session=<session_id> HTTP/1.1
Upgrade: websocket
```

| 参数 | 必选 | 说明 |
|------|------|------|
| `token` | 配置了 `auth_token` 时必选 | 认证 token |
| `session` | 否 | 指定 session ID；空 = 使用/创建默认 session |

**客户端 → 服务端消息**：

```jsonc
// 发送用户消息
{
  "type": "message",
  "content": "帮我看下 main.go 的结构"
}

// 发送命令
{
  "type": "command",
  "content": "/new"
}

// 工具审批响应（当 smart_approvals 启用时）
{
  "type": "approval",
  "approval_id": "ap_abc123",
  "approved": true
}

// 心跳
{
  "type": "ping"
}
```

**服务端 → 客户端消息**：

```jsonc
// 连接建立确认
{
  "type": "connected",
  "session_id": "hermes/ws/conn_abc123",
  "version": "0.1.27",
  "model": "deepseek-v4-flash",
  "work_dir": "/home/user/project"
}

// 文本流式增量（agent 响应）
{
  "type": "text_delta",
  "content": "这个文件的主要结构是…"
}

// thinking 流式增量
{
  "type": "think_delta",
  "content": "分析 main.go 的引入包…"
}

// 工具调用开始
{
  "type": "tool_call",
  "tool": "read",
  "call_id": "tc_123",
  "args": {"path": "main.go"}
}

// 工具执行结果
{
  "type": "tool_result",
  "tool": "read",
  "call_id": "tc_123",
  "result": "package main\n\nimport (\n...",
  "error": null
}

// 工具执行产生的文件 diff（edit/write 工具）
{
  "type": "tool_diff",
  "call_id": "tc_456",
  "path": "main.go",
  "diff": "--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@..."
}

// 审批请求（smart_approvals 启用时）
{
  "type": "approval_request",
  "approval_id": "ap_abc123",
  "tool": "bash",
  "args": {"command": "rm -rf /tmp/test"},
  "risk_level": "high"
}

// plan 工具更新
{
  "type": "plan_update",
  "plan": {
    "title": "重构 main.go",
    "steps": [
      {"title": "读取当前代码", "status": "done"},
      {"title": "拆分函数", "status": "running"},
      {"title": "添加测试", "status": "pending"}
    ]
  }
}

// 用量统计
{
  "type": "usage",
  "prompt_tokens": 1200,
  "completion_tokens": 350,
  "total_tokens": 1550,
  "cache_read_tokens": 800,
  "cache_write_tokens": 400
}

// 当前轮完成
{
  "type": "done",
  "stop_reason": "end_turn"
}

// 命令响应（/new, /clear, /status 等）
{
  "type": "command_result",
  "command": "/new",
  "message": "✅ New session created.",
  "error": false
}

// 错误
{
  "type": "error",
  "message": "provider error: rate limited",
  "code": "rate_limit"
}

// 心跳响应
{
  "type": "pong"
}
```

**消息流时序示例**：

> ✅ **已实现** — `agentEventToWSEvent()` 将 agent 事件逐个转换为 WebSocket 消息。

```
client                          server
  |-- {type:"message"} ---------->|
  |                               |-- agent loop 开始
  |<-- {type:"text_delta"} -------|-- 流式输出“让我看看…”
  |<-- {type:"tool_call"} --------|-- 调用 read 工具
  |<-- {type:"tool_result"} ------|-- 工具结果
  |<-- {type:"text_delta"} -------|-- 继续流式输出
  |<-- {type:"text_delta"} -------|   ...
  |<-- {type:"usage"} ------------|-- token 用量
  |<-- {type:"done"} -------------|-- 本轮完成
```

#### 8.3.3 HTTP REST API (`/api/*`)

**认证**：配置了 `server.auth_token` 时，所有 `/api/*` 请求需携带 `Authorization: Bearer <token>` 头。

---

**`GET /api/health`** — 健康检查（无需认证）

```json
// Response 200
{
  "status": "ok",
  "version": "0.1.27",
  "uptime_seconds": 3600
}
```

---

**`GET /api/status`** — 服务状态

```json
// Response 200
{
  "version": "0.1.27",
  "uptime_seconds": 3600,
  "work_dir": "/home/user/project",
  "model": "deepseek-v4-flash",
  "provider": "deepseek-openai",
  "sessions": {
    "active": 3,
    "total": 12
  },
  "platforms": {
    "wechat": {"enabled": true, "connected": true, "users": 2},
    "feishu": {"enabled": false, "connected": false, "users": 0}
  },
  "a2a": {"enabled": true},
  "cron": {"enabled": true, "jobs": 2}
}
```

---

**`GET /api/sessions`** — 列出活跃 session

```json
// Response 200
{
  "sessions": [
    {
      "id": "hermes/wechat/wxid_user1",
      "platform": "wechat",
      "user_id": "wxid_user1",
      "work_dir": "/home/user/project-a",
      "message_count": 42,
      "last_active": "2026-05-29T10:30:00Z",
      "preview": "帮我看下 main.go..."
    },
    {
      "id": "hermes/feishu/ou_user2",
      "platform": "feishu",
      "user_id": "ou_user2",
      "work_dir": "/home/user/project-b",
      "message_count": 8,
      "last_active": "2026-05-29T09:15:00Z",
      "preview": "添加单元测试..."
    }
  ]
}
```

---

**`GET /api/sessions/{id}`** — 查看 session 详情

```json
// Response 200
{
  "id": "hermes/wechat/wxid_user1",
  "platform": "wechat",
  "user_id": "wxid_user1",
  "work_dir": "/home/user/project-a",
  "mode": "agent",
  "model": "deepseek-v4-flash",
  "message_count": 42,
  "created_at": "2026-05-29T08:00:00Z",
  "last_active": "2026-05-29T10:30:00Z",
  "context_tokens": 45000,
  "context_limit": 128000,
  "compaction_count": 1
}
```

---

**`DELETE /api/sessions/{id}`** — 删除 session

```json
// Response 200
{"message": "session deleted", "id": "hermes/wechat/wxid_user1"}
```

---

**`GET /api/memory`** — 读取 memory.md

```json
// Response 200
{
  "path": "/home/user/project/.vibe/memory.md",
  "source": "project",
  "content": "# Agent Memory\n\n## User Profile\n\n- 用户偏好中文...\n"
}
```

---

**`PUT /api/memory`** — 更新 memory.md

```json
// Request
{"content": "# Agent Memory\n\n## User Profile\n\n- updated...\n"}

// Response 200
{"message": "memory updated", "path": "/home/user/project/.vibe/memory.md"}
```

---

**`GET /api/platforms`** — 消息平台状态

```json
// Response 200
{
  "platforms": [
    {
      "name": "wechat",
      "enabled": true,
      "connected": true,
      "work_dir": "/home/user/project-a",
      "active_users": ["wxid_user1", "wxid_user2"],
      "login_status": "logged_in"
    },
    {
      "name": "feishu",
      "enabled": true,
      "connected": true,
      "work_dir": "/home/user/project-b",
      "active_users": ["ou_user1"],
      "login_status": "connected"
    }
  ]
}
```

#### 8.3.4 Webhook 入站 (`/webhook/*`)

根据 `hermes.json` 中配置的路由分发外部事件：

```
POST /webhook/github
X-Hub-Signature-256: sha256=...

{"action": "opened", "pull_request": {...}}
```

验证 `webhooks.secret` 后，根据路由配置中的 `skill` 和 `delivery` 触发 agent 任务，结果通过指定的消息平台推送。

#### 8.3.5 A2A 协议 (`/a2a`)

仅当 `a2a.enabled=true` 时注册。详见 §5.3 A2A 协议设计。

| 端点 | 说明 |
|------|------|
| `GET /.well-known/agent.json` | Agent Card（无需认证） |
| `POST /a2a` | JSON-RPC 2.0（SendMessage / GetTask） |

#### 8.3.6 WebSocket 消息类型汇总

| 方向 | type | 说明 |
|------|------|------|
| **C→S** | `message` | 用户输入 |
| **C→S** | `command` | 斜杠命令（`/new`, `/clear`, `/status` 等） |
| **C→S** | `approval` | 工具审批响应 |
| **C→S** | `ping` | 心跳 |
| **S→C** | `connected` | 连接确认 + session/model 信息 |
| **S→C** | `text_delta` | 文本流式增量 |
| **S→C** | `think_delta` | thinking 流式增量 |
| **S→C** | `tool_call` | 工具调用开始 |
| **S→C** | `tool_result` | 工具执行结果 |
| **S→C** | `tool_diff` | 文件 diff（edit/write） |
| **S→C** | `approval_request` | 工具审批请求 |
| **S→C** | `plan_update` | plan 工具状态更新 |
| **S→C** | `usage` | token 用量统计 |
| **S→C** | `done` | 本轮完成 |
| **S→C** | `command_result` | 命令执行结果 |
| **S→C** | `error` | 错误 |
| **S→C** | `pong` | 心跳响应 |

### 8.4 `hermes client` — 终端接入模式

> ✅ **已实现** — `internal/hermes/client.go` WebSocket 客户端，支持流式输出（text_delta/think_delta/tool_call/tool_result/done）和斜杠命令（/new /clear /status /sessions /mode /compact）。

`vibecoding hermes client` 通过 WebSocket 连接正在运行的 Hermes 网关。

```bash
# 连接本地 hermes
vibecoding hermes client

# 连接远程 hermes
vibecoding hermes client --url ws://192.168.1.100:8090/ws

# 恢复已有 session
vibecoding hermes client --session abc123
```

**与直接运行 `vibecoding` 的区别**：

| 维度 | `vibecoding`（普通 CLI） | `vibecoding hermes client` |
|------|--------------------------|----------------------------|
| **Agent 进程** | 本地独立进程 | 连接 Hermes 守护进程 |
| **通信方式** | 本地函数调用 | WebSocket 流式通信 |
| **Session** | 本地管理 | 服务端管理（per-user，可跨终端恢复） |
| **Memory** | 无 | 共享 Hermes 的 memory.md |
| **工具执行** | 本地执行 | Hermes 服务端执行（受 security/hooks 约束） |
| **工作目录** | 本地 cwd | Hermes 服务端工作目录 |
| **Cron/Webhook** | 无 | 可查看 Hermes 的调度状态 |

**典型使用场景**：
- 开发者想在终端中与已部署的 Hermes 实例交互（而不是通过微信/飞书）
- 调试 Hermes 的行为，实时观察 agent loop 输出
- 远程连接服务器上运行的 Hermes 实例
- 管理 Hermes 的 session、memory 等状态

### 8.5 `config init` — 初始化级别

```
vibecoding hermes config init              # 默认写入 <GLOBAL_DIR>/hermes.json
vibecoding hermes config init --global     # 显式写入 <GLOBAL_DIR>/hermes.json
vibecoding hermes config init --project    # 写入 .vibe/hermes.json（自动创建 .vibe/ 目录）
```

`--global` 和 `--project` 互斥。目标文件已存在时报错，需加 `--force` 覆盖。

项目级模板会省略全局性配置（如微信凭证路径），只包含项目可能需要覆盖的字段（如 `work_dir`、`memory`、`agent`、`security` 等）。

### 8.6 配置文件 `hermes.json`

加载优先级：`defaults` → `<GLOBAL_DIR>/hermes.json` → `.vibe/hermes.json`

```jsonc
{
  // === 网关服务（始终启动） ===

  "server": {
    "port": 8090,                  // WebSocket + HTTP 监听端口
    "host": "0.0.0.0",             // 监听地址（0.0.0.0 = 所有网卡，127.0.0.1 = 仅本地）
    "auth_token": "${HERMES_AUTH_TOKEN}"  // 空 = 无认证（仅本地使用）
  },

  // === 默认 Provider/Model ===

  "default_provider": "",          // 空 = 继承 settings.json 的 defaultProvider
  "default_model": "",             // 空 = 继承 settings.json 的 defaultModel

  // === 多 Agent 模式 ===

  "multi_agent": false,             // 启用后注册子 Agent 工具（spawn/status/send/destroy）

  // === Sandbox ===

  "sandbox": false,                 // 启用 bwrap 沙箱隔离（默认关闭）

  // === 微信 (iLink) ===

  "wechat": {
    "enabled": true,
    "cred_path": "",               // 空 = 默认 <GLOBAL_DIR>/wechat-credentials.json
    "work_dir": "",                // 空 = hermes 启动时的 cwd
    "allowed_users": [],           // 空 = 允许所有人（危险！）
    "auto_typing": true            // 自动显示"正在输入"
  },

  // === 飞书 ===

  "feishu": {
    "enabled": false,
    "app_id": "${FEISHU_APP_ID}",
    "app_secret": "${FEISHU_APP_SECRET}",
    "work_dir": "",                // 空 = hermes 启动时的 cwd
    "allowed_users": []
  },

  // === Webhook 入站 ===

  "webhooks": {
    "enabled": false,
    "secret": "${WEBHOOK_SECRET}",
    "routes": [
      {
        "path": "/github",
        "events": ["push", "pull_request"],
        "skill": "code-review",
        "delivery": "wechat"
      }
    ]
  },

  // === A2A Server ===

  "a2a": {
    "enabled": false
  },

  // === Cron ===

  "cron": {
    "enabled": true
  },

  // === 记忆 ===

  "memory": {
    "enabled": true,
    "path": ""                     // 空 = 按优先级查找: .vibe/memory.md → <GLOBAL_DIR>/memory.md
  },

  // === 安全 ===

  "security": {
    "smart_approvals": true,
    "allowed_work_dirs": []        // 空 = 仅允许 work_dir 及其子目录
  },

  // === Shell Hooks ===

  "hooks": {
    "pre_tool_call": "",           // 外部脚本路径
    "post_tool_call": ""
  },

  // === Agent ===

  "agent": {
    "max_turns": 90,
    "budget_pressure": true,
    "context_pressure": true
  },

  // === 默认工作目录 ===

  "work_dir": "."                  // hermes 启动时的默认工作目录（微信/飞书未单独配置时的 fallback）
}
```

**工作目录解析优先级**：

```
平台级 work_dir (微信/飞书 单独配置)
  → 全局 work_dir (hermes.json 顶层)
    → CLI --work-dir 参数
      → hermes 启动时的 cwd
```

每个消息平台可以有独立的工作目录，适用于“微信管理项目 A，飞书管理项目 B”的场景。

### 8.7 消息平台进度事件推送

Hermes 模式下，agent 执行过程中会实时向消息平台（微信/飞书）推送进度事件，最后再发送完整总结。

#### 推送内容

| 事件类型 | 格式 | 说明 |
|----------|------|------|
| 思考过程 | `💭 <思考内容...>` | 模型推理过程，截断 500 字符 |
| 工具执行 | `[tool]: args ✅/❌` | 工具调用结果，一行摘要 |
| 完整总结 | （完整文本） | agent 最终输出 |

#### 工具进度格式示例

```
💭 用户想了解项目结构，让我先看看目录...
[ls]: . ✅
[read]: .vibe/memory.md ✅
[bash]: go build ./... ✅
[grep]: NewStore ✅
[find]: *.go ✅
[write]: output.txt ✅
[memory] ✅

（完整总结文本）
```

#### 实现机制

- `messaging.InboundMessage` 新增 `ProgressFunc func(text string)` 回调
- 微信/飞书 bot 收到消息时设置 `ProgressFunc`，内部调用 `SendMessage` 推送进度
- `dispatcher.runAgent` 监听 `EventThinkDelta`（累积后推送）和 `EventToolExecutionEnd`（格式化一行进度）
- WebSocket 路径不受影响，仍通过 event channel 流式推送

### 8.8 Provider/Model 配置优先级

```bash
# CLI 标志（最高优先级）
vibecoding hermes start -p openai -m gpt-4o

# hermes.json 配置
{ "default_provider": "openai", "default_model": "gpt-4o" }

# settings.json（最低优先级，继承）
{ "defaultProvider": "deepseek", "defaultModel": "deepseek-chat" }
```

优先级：CLI `-p`/`-m` 标志 > `hermes.json` > `settings.json`

### 8.9 MCP 工具继承

Hermes 自动加载全局和项目的 `mcp.json` 配置，与 CLI 行为一致。MCP 工具注册到每个 session 的 tool registry 中，session 移除/轮转时自动关闭 MCP 连接。

---

## 9. 架构设计

### 9.1 新增包结构

```
internal/
├── messaging/                   # 消息平台层（抽象 + 各平台实现）
│   ├── platform.go              # ✅ Platform 接口 + InboundMessage 等公共类型
│   ├── progress.go              # ✅ ProgressBuffer 批量进度推送（新增，提案未列出）
│   ├── progress_test.go         # ✅
│   ├── wechat/                  # ✅ 微信 iLink 适配器（自行实现，零外部依赖）
│   │   ├── wechat.go            # ✅ Bot 主体，实现 messaging.Platform
│   │   ├── types.go             # ✅ iLink 协议类型定义
│   │   ├── protocol.go          # ✅ iLink HTTP API 调用
│   │   ├── auth.go              # ✅ QR 登录 + 凭证持久化
│   │   └── crypto.go            # ✅ AES-128-ECB CDN 加解密
│   └── feishu/                  # ✅ 飞书适配器
│       └── feishu.go            # ✅ 飞书 SDK 封装（长连接），实现 messaging.Platform
│                               #    ⚠️ session.go 未创建（per-user session 由 dispatcher 统一管理）
│
├── hermes/                      # Hermes 模式编排层
│   ├── server.go                # ✅ 守护进程主循环（组装 gateway + messaging + cron）
│   ├── config.go                # ✅ hermes.json 配置加载（全局 + 项目级合并）
│   ├── config_test.go           # ✅
│   ├── dispatcher.go            # ✅ 消息 → Agent 转发调度器
│   ├── security.go              # ✅ 用户白名单 + 命令风险分类 + 自动审批（新增）
│   ├── security_test.go         # ✅
│   ├── webhook_handler.go       # ✅ Webhook → Agent 任务处理（新增）
│   ├── webhook_handler_test.go  # ✅
│   ├── ws/                      # ✅ WebSocket + HTTP 网关
│   │   ├── server.go            # ✅ net/http 服务器（⚠️ 使用 golang.org/x/net/websocket 而非 gorilla/websocket）
│   │   ├── handler.go           # ✅ WebSocket 消息处理
│   │   └── api.go               # ✅ HTTP REST API
│   ├── a2a/                     # ❌ A2A 协议 Server — 目录不存在，未实现
│   ├── webhook/                 # ✅ Webhook 入站
│   │   └── router.go            # ✅ HMAC-SHA256 验签 + 路由分发
│   └── hooks/                   # ✅ Shell Hooks
│       └── hooks.go             # ✅ 外部脚本调用（JSON stdin/stdout）
│
├── a2a/                         # 🔶 待实现 — A2A 协议（独立于 hermes 的顶层包）
│   ├── server.go                # A2A HTTP server（独立模式 + 集成模式）
│   ├── handler.go               # JSON-RPC 2.0 handler
│   ├── agent_card.go            # Agent Card 生成
│   ├── task.go                  # Task 生命周期管理
│   ├── executor.go              # AgentExecutor（A2A Task → agent loop）
│   ├── sse.go                   # SSE 流式响应
│   └── config.go                # A2A 配置
│
├── memory/                      # 持久化记忆
│   ├── store.go                 # ✅ memory.md 读写（全局/项目级查找逻辑）
│   ├── store_test.go            # ✅
│   └── tool.go                  # ✅ memory 工具定义
│
└── (existing packages unchanged)
```

> **与提案的偏差**：
> 1. `feishu/session.go` 未创建 — per-user session 由 `dispatcher.go` 统一管理，不需要单独的 feishu session 文件
> 2. `ws/server.go` 使用 `golang.org/x/net/websocket` 而非提案中的 `gorilla/websocket`
> 3. 新增了提案未列出的文件：`messaging/progress.go`、`hermes/security.go`、`hermes/webhook_handler.go`
> 4. A2A 从 `internal/hermes/a2a/` 移至 `internal/a2a/`（独立顶层包）

> **架构要点**：
> - `hermes/ws/` 是新增的 **WebSocket + HTTP 网关层**，Hermes 启动后始终运行，是所有客户端（`hermes client`、第三方应用）的接入点。
> - Webhook 和 A2A 复用同一个 HTTP 端口（`server.port`），通过路由区分：`/ws`、`/a2a`、`/webhook/*`、`/api/*`。
> - `internal/messaging/` 是消息平台的**抽象 + 实现**层，纯粹关注"接收消息、发送消息"。每个子包是独立适配器，实现 `messaging.Platform` 接口。
> - `internal/hermes/` 是 Hermes 模式的**编排层**，负责把 gateway、messaging、webhook、cron、agent loop 组装到一起运行。
> - 新增平台只需在 `messaging/` 下加子包，无需改动编排层。

### 9.2 消息平台抽象

> ✅ **已实现** — `internal/messaging/platform.go` 完整实现了以下接口。额外增加了 `IsConnected()` 方法和 `ProgressFunc` 字段。

```go
// internal/messaging/platform.go
package messaging

type Platform interface {
    Name() string
    Start(ctx context.Context, handler MessageHandler) error
    Stop() error
    SendMessage(ctx context.Context, chatID string, text string) error
    IsConnected() bool  // 新增：提案中未列出
}

type MessageHandler func(ctx context.Context, msg InboundMessage) (string, error)

type InboundMessage struct {
    Platform  string
    ChatID    string
    UserID    string
    UserName  string
    Text      string
    Timestamp time.Time
    ProgressFunc func(text string)  // 新增：提案中未列出，用于进度推送
}
```

### 9.3 hermes.json 配置加载（复用已有模式）

```go
// internal/hermes/config.go — 遵循 gateway.json 相同模式

func HermesConfigPath() string {
    return filepath.Join(config.ConfigDir(), "hermes.json")  // <GLOBAL_DIR>/hermes.json
}

func ProjectHermesConfigPath() string {
    return filepath.Join(".vibe", "hermes.json")  // .vibe/hermes.json
}

func LoadHermesConfig() (*HermesConfig, error) {
    cfg, err := loadHermesConfigFrom(HermesConfigPath())    // 1. 加载全局
    if err != nil { return nil, err }
    // 2. 项目级覆盖
    if data, err := os.ReadFile(ProjectHermesConfigPath()); err == nil {
        if err := json.Unmarshal(data, cfg); err != nil {
            return nil, fmt.Errorf("parse project hermes config: %w", err)
        }
    }
    return cfg, nil
}
```

### 9.4 复用关系

```
hermes server (internal/hermes/)
  │
  ├─ 完全复用 ──────────────────────────────
  │   ├── agent.Agent          (agent loop)
  │   ├── provider.*           (OpenAI/Anthropic)
  │   ├── tools.Registry       (所有内置工具)
  │   ├── session.Store        (JSONL 持久化)
  │   ├── sandbox              (bwrap)
  │   ├── skills               (SKILL.md)
  │   ├── context compaction   (压缩)
  │   ├── context files        (AGENTS.md)
  │   └── config.ConfigDir()   (全局配置目录解析)
  │
  ├─ 新增 ──────────────────────────────────
  │   ├── hermes/ws            (WebSocket + HTTP 网关，始终启动)
  │   ├── memory tool          (memory.md 按需读写，不注入 system prompt)
  │   ├── messaging.Platform   (WeChat iLink / Feishu，可选连接)
  │   ├── a2a                  (A2A Server — 独立顶层包，Agent 间协作)
  │   ├── hermes/webhook       (入站 webhook)
  │   ├── hermes.Hooks         (shell hooks)
  │   ├── context pressure     (compaction 层注入) 🔶 待实现
  │   └── smart approvals      (tools 层拦截) 🔶 待讨论
  │
  └─ 增强 ──────────────────────────────────
      └── cron                 (管理 CLI 补齐)
```

### 9.5 Shell Hooks 协议

外部脚本通过 JSON stdin/stdout 通信：

**pre_tool_call — stdin:**
```json
{
  "hook": "pre_tool_call",
  "tool": "bash",
  "args": {"command": "rm -rf /tmp/test"},
  "platform": "wechat",
  "user_id": "wxid_12345"
}
```

**stdout:**
```json
{"action": "allow"}
```
或
```json
{"action": "block", "reason": "destructive command blocked"}
```

---

## 10. 实施阶段

### Phase 1: 骨架 & 配置 & 网关

- [x] `internal/messaging/platform.go` — Platform 接口定义（含 ProgressFunc）
- [x] `internal/hermes/` 编排层骨架
- [x] `internal/hermes/config.go` — hermes.json 配置加载（含 `server` 节、平台 `work_dir`、全局/项目级合并）
- [x] `internal/hermes/ws/` — WebSocket + HTTP 网关骨架（server.go + handler.go + api.go）
- [x] `vibecoding hermes` 子命令注册（start/stop/status/config/client/wechat/feishu/cron）
- [x] Hermes server 主循环框架（启动网关 → 可选连接消息平台）
- [x] `hermes/dispatcher.go` — per-user session 路由（`<sessionDir>/hermes/<platform>/<user_id>/active.jsonl`）
- [x] session 归档逻辑（`/new` → `active.jsonl` 重命名 + 新建）
- [x] CLI 标志: `-p`/`--provider`、`-m`/`--model`、`--multi-agent`、`--sandbox`
- [x] hermes.json 新增字段: `default_provider`、`default_model`、`multi_agent`、`sandbox`
- [x] MCP 服务器加载（继承全局/项目 mcp.json 配置）
- [x] 消息平台进度事件推送（ProgressFunc: 工具执行 + 思考过程逐行发送）

> **偏差**：
> - WebSocket 使用 `golang.org/x/net/websocket` 而非 `gorilla/websocket`
> - WebSocket 消息处理是同步模式（等 agent 完成后一次性返回），非真正的逐事件流式
> - `stop`/`status`/`client` 命令是 stub，未实现

### Phase 2: memory 工具 & 压力系统

- [x] `internal/memory/store.go` — memory.md 读写（含 `.vibe/memory.md` → `<GLOBAL_DIR>/memory.md` 查找逻辑）
- [x] `internal/memory/tool.go` — memory 工具（read/add/update/delete）
- [x] System prompt guidelines 添加静态 memory 提示
- [x] memory.md 默认写入项目目录（只有显式配置 `memory.path` 才写全局）
- [x] Budget Pressure — MaxIterations 从 hermes config `agent.max_turns` 注入
- [ ] Context Pressure — compaction 阈值警告

> **偏差**：
> - Budget Pressure 仅注入了 MaxIterations 上限，**未在 tool result 中注入迭代预算警告**（提案要求「在 tool result 中注入迭代预算警告」）
> - Context Pressure 完全未实现（仅有配置字段）

### Phase 3: 安全层

- [x] Smart Approvals — 命令危险性分类（默认 yolo 模式）
- [x] Shell Hooks — pre/post tool call 外部脚本（已接入 AfterToolCall）
- [x] 用户白名单验证

> **偏差**：Smart Approvals 的 WebSocket `approval_request` 交互流未实现（handler.go 中 approval case 标注 TODO）

### Phase 4: 微信网关

- [x] `internal/messaging/wechat/types.go` — iLink 协议类型定义
- [x] `internal/messaging/wechat/protocol.go` — iLink HTTP API 调用
- [x] `internal/messaging/wechat/auth.go` — QR 登录 + 凭证持久化到 `<GLOBAL_DIR>/wechat-credentials.json`
- [x] `internal/messaging/wechat/crypto.go` — AES-128-ECB CDN 加解密
- [x] `internal/messaging/wechat/wechat.go` — 实现 `messaging.Platform`
- [x] `internal/hermes/dispatcher.go` — 消息 → Agent 转发
- [x] `vibecoding hermes wechat login` — QR 码登录
- [x] 消息平台命令（/new /clear /mode /status /sessions）

> **无偏差** — 微信网关完整实现了提案中所有功能点。

### Phase 5: 飞书网关

- [x] `go get github.com/larksuite/oapi-sdk-go/v3`
- [x] `internal/messaging/feishu/feishu.go` — 实现 `messaging.Platform`（长连接）
- [x] `vibecoding hermes feishu setup` — 交互式配置
- [x] `vibecoding hermes feishu status` — 连接状态

> **偏差**：
> - 提案中的 `feishu/session.go`（per-user Session 管理）**未创建** — session 由 `dispatcher.go` 统一管理
> - `feishu setup` 仅打印配置说明文本，非真正的交互式配置向导

### Phase 6: A2A Server + Webhook + Cron

- [x] `internal/a2a/config.go` — A2A 配置
- [x] `internal/a2a/task.go` — Task 生命周期管理（submitted → working → completed/failed/canceled）
- [x] `internal/a2a/handler.go` — JSON-RPC 2.0 handler（message/send, task/get, task/cancel）+ SSE 流式
- [x] `internal/a2a/agent_card.go` — Agent Card 生成 (/.well-known/agent.json)
- [x] `internal/a2a/executor.go` — DefaultExecutor（A2A Task → agent loop）
- [x] `internal/a2a/server.go` — A2A HTTP server（独立模式 + 集成模式）
- [x] `cmd/vibecoding/main_a2a.go` — `vibecoding a2a` 子命令（start/stop/status/card）
- [x] hermes 集成：`a2a.enabled: true` 时将 A2A 端点挂载到 hermes HTTP mux
- [x] `internal/hermes/webhook/` — HTTP 入站 webhook 路由
- [x] Webhook 路由 → Agent 任务（webhook_handler.go）
- [x] Cron 管理 CLI 命令（list/add/remove/enable/disable）

> **A2A 已完成**：零外部依赖，直接实现 JSON-RPC 2.0 over HTTP + SSE 流式。
> **Cron 已确认**：CLI 命令范围已确定（不做 edit/run），底层 cron 实现与项目共享，有 bug 或缺陷仍需修复完善。

### Phase 7: WebSocket 流式推送 & 补全 CLI

- [x] WebSocket 流式推送：`wsDispatcherAdapter` 改为监听 `chan agent.Event`，逐事件转换为 `WSEvent` 发送
- [x] `hermes stop` — PID 文件 + SIGTERM 信号
- [x] `hermes status` — PID 检查 + HTTP health 查询
- [x] `hermes client` — WebSocket 客户端（流式输出 + 斜杠命令 + session 恢复）
- [x] `hermes webhook list` — webhook 路由查看
- [x] `hermes memory show/clear` — memory 查看和清空
- [x] `hermes sessions list` — 查询运行实例的活跃 session
- [x] `/api/memory` HTTP — 集成 MemoryStore 实现 GET/PUT

### Phase 8: Context Pressure & 压力系统

- [x] Context Pressure — `EventContextPressure` 事件，55% 阈值触发一次，上层决策处理
- [x] Budget Pressure — `EventBudgetPressure` 事件，剩余 20% 时触发一次
- [x] hermes.json 配置：`agent.context_pressure_threshold`（默认 0.55）、`agent.budget_pressure_threshold`（默认 0.20）
- [x] hermes dispatcher 事件转发到消息平台 ProgressFunc
- [ ] WebSocket 流式推送压力事件（依赖 Phase 7 流式改造）

> **设计决策**：
> - Context Pressure 使用 Event 通知模式（方案 C），由上层决定如何处理
> - Budget Pressure 在剩余 20% 时一次性注入（方案 B），不重复打扰
> - 阈值可配置，默认 Context 55%、Budget 剩余 20%

### Phase 9: Smart Approvals

- [x] 方案 D 分级策略实现
  - low risk → 自动批准
  - medium risk → 自动批准 + 通知用户
  - high risk (WebSocket) → 发送 `approval_request`，等待用户 `approval_response`（5 分钟超时）
  - high risk (消息平台) → 自动拒绝 + 通知用户
- [x] `security.go` — `FormatApprovalNotification()` 通知格式化
- [x] `dispatcher.go` — `RegisterApproval()` / `ResolveApproval()` 审批状态管理
- [x] `ws/handler.go` — `approval` 消息处理 → `ResolveApproval()`
- [x] `server.go` — `agentEventToWSEvent` 转换 `EventToolApprovalRequest`

> **设计决策**：
> - 消息平台不支持交互式审批（无法暂停 agent loop 等待用户回复），高风险命令自动拒绝
> - WebSocket 支持完整审批流：`approval_request` → 用户回复 → `approval_response`
> - 审批超时 5 分钟，超时自动拒绝

### Phase 10: 文档 & 测试

- [x] hermes 子命令使用文档 (`docs/en/hermes.md`, `docs/zh/hermes.md`)
- [x] hermes.json 配置文档（含全局/项目级层级说明）
- [x] 微信 iLink / 飞书 Bot 设置指南
- [x] A2A Server 接入文档 (`docs/en/a2a.md`, `docs/zh/a2a.md`)
- [x] `vibecoding a2a` 子命令文档
- [x] 单元测试（schedule, progress buffer, security, config, cron tool, webhook handler）
- [x] Changelog 更新 (`docs/en/changelog.md`, `docs/zh/changelog.md`)
- [ ] 集成测试

---

## 11. 与现有模式的关系

| 维度 | CLI (TUI) | ACP | Gateway | **Hermes (新增)** | **A2A (新增)** |
|------|-----------|-----|---------|-------------------|----------------|
| **入口** | 终端 stdin | Editor stdio | HTTP API | **WebSocket + HTTP 网关** + 消息平台 (微信/飞书) | **JSON-RPC 2.0 over HTTP** |
| **使用者** | 开发者本人 | 编辑器 | 其他应用 | **终端用户 (Bot) / 开发者 (`client`)** | **其他 Agent** |
| **Session** | 本地管理 | 编辑器管理 | 客户端指定 | **服务端管理 (per-user，`client` 可跨终端恢复)** | **Task 生命周期** |
| **认证** | 无 | 无 | Bearer token | **平台用户白名单** | **Bearer token** |
| **常驻** | 否 | 否 | 是 | **是（`client` 按需连接）** | **是** |
| **Cron** | 无 | 无 | 无 | **内置调度器** | 无 |
| **记忆** | 无 | 无 | 无 | **memory.md (tool 按需读写)** | 无 |
| **配置** | `settings.json` | `settings.json` | `gateway.json` | **`hermes.json`** | **`a2a.json` 或 hermes.json 中 a2a 节** |
| **配置层级** | `<GLOBAL_DIR>` + `.vibe/` | `<GLOBAL_DIR>` + `.vibe/` | `<GLOBAL_DIR>` + `.vibe/` | **`<GLOBAL_DIR>` + `.vibe/`** | **`<GLOBAL_DIR>` + `.vibe/`** |
| **A2A** | 无 | 无 | 无 | **集成模式（配置启用）** | **独立模式 + 集成模式** |

---

## 12. 供应链安全原则

| 组件 | 策略 | 说明 |
|------|------|------|
| 微信 iLink | **自行实现** | 参考 iLink 协议规范实现为 internal 包，零外部依赖 |
| 飞书 SDK | **官方 SDK** | `larksuite/oapi-sdk-go` 飞书官方维护，可接受 |
| A2A SDK | **官方 SDK** | `a2aproject/a2a-go` Google/Linux Foundation 维护，可接受 |
| CDN 加密 | **标准库** | `crypto/aes` Go 标准库，无外部依赖 |
| HTTP 调用 | **标准库** | `net/http` Go 标准库 |

> **原则**：能用标准库实现的不引入外部包；必须引入的只用官方/基金会维护的 SDK。

---

## 13. 非目标

1. **Web 搜索** — 用户通过第三方 skill 扩展
2. **Checkpoints / Rollback** — 推迟
3. **企业微信** — 用个人微信 iLink 代替
4. **Memory 注入 system prompt** — 破坏缓存命中，改用 tool 按需读写
5. **Telegram / Discord** — v0.1.28
6. **Python 插件 / RL Training / Voice** — 不做

---

*决策已确认。可以开始开发。*
