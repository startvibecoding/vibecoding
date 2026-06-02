# 更新日志


## v0.1.30

### ✨ 新功能

- **Provider 级 HTTP 代理**
  - 新增 `providers.<name>.httpProxy`，支持为不同 provider 配置不同 HTTP 代理
  - 未配置 `httpProxy` 时继续保留默认环境变量代理行为

- **Google Gemini 和 Vertex 厂商适配器**
  - 新增原生 `google-gemini` 和 `google-vertex` provider，使用 Google `streamGenerateContent`
  - 支持 Gemini API 和 Vertex AI 原生 Gemini 端点的 baseUrl 自动识别
  - 新增 Gemini API key 和 Vertex bearer token 的默认 Google provider 模板
  - 更新 provider 文档与识别测试覆盖

- **Hosted Web Search 工具**
  - 为 CLI 和 ACP 运行新增 `--web-search`
  - 新增顶层 `webSearch` 配置，包含 `enabled`、`provider`、`providerType` 和 `model`
  - 仅在启用时注册 hosted `web_search`，并与本地 function tools 保持隔离
  - 新增 OpenAI Responses API 映射到 `web_search`
  - 将 Responses web search 映射改为 provider-neutral 的 `web_search`，兼容 provider 不必命名为 `openai`
  - 新增 Anthropic Messages API 映射到 `web_search_20250305`
  - 将 `webSearch.model` 保留为 provider-neutral metadata，用于后续路由和成本展示扩展

- **默认 Provider 模板**
  - 新增 OpenAI、Anthropic 和 Xiaomi MiMo 默认 provider 配置
  - 保留 DeepSeek providers，并继续使用 `deepseek-openai` 作为默认 provider/model
  - 首次生成的 `settings.json` 现在包含默认关闭的 web search 配置，以及 OpenAI/Anthropic/Xiaomi provider 模板

### 🧪 测试

- 增加 OpenAI Responses 和 Anthropic Messages hosted web search 序列化测试
- 增加 web search 配置默认值、CLI flag 解析和 hosted tool metadata 传递测试
- 增加 macOS 默认配置目录解析测试

### 🐛 Bug 修复

- **macOS 配置目录**
  - 将 macOS 默认全局配置目录与 Linux 统一为 `~/.vibecoding`

- **发布版本号**
  - npm 和发行包版本检测默认不再附加 `dirty` 后缀
  - 将 npm package metadata 规范化为 `0.1.30`

## v0.1.29

### 🐛 Bug 修复

- **NPM 包装修复**
  - 修复 `npm/bin/vibecoding` 入口脚本，确保安装包正确附带可执行包装器
  - 调整 `build-npm.sh` 和 `build-npm-packages.sh` 保证包装器一致性

## v0.1.28

### ✨ 新功能

- **Per-Model 温度/Top-P 配置**
  - 为 `ModelConfig` 和 `Model` 新增 `temperature` 和 `top_p` 字段，支持逐模型参数调优
  - 在 OpenAI 和 Anthropic 提供商中打通，使用 `omitempty` — `nil` 表示使用 API 默认值
  - 在 provider factory、agent loop、ACP 模式中打通
  - Gateway 模式支持请求级 `temperature`/`top_p` 覆盖（通过 `ChatParams`）
  - 未配置时完全省略参数（不会向 API 发送零值）

- **OpenAI Responses API 支持**
  - 新增独立的 OpenAI Responses provider 路径，通过 `api: "openai-responses"` 启用
  - 支持 Responses 流式输出、工具调用、reasoning summary 和 prompt cache 参数
  - 在 provider `responses` 配置中暴露 Responses 专用设置，默认启用 prompt cache
  - 新增模型兼容标志 `supportsPromptCacheKey` 和 `supportsReasoningSummary`

### 🧪 测试

- 提升 OpenAI Responses API 和 Anthropic 请求解析相关测试覆盖
- 将 Anthropic 测试改为内存 HTTP mock，避免依赖本地端口监听

### 📝 文档

- 更新 `AGENTS.md` 版本至 v0.1.28

## v0.1.27

### ✨ 新功能

- **Hermes 模式** (`vibecoding hermes`)
  - 新增消息平台网关模式，支持微信、飞书和 WebSocket
  - 持久化 per-user session，`/new` 时自动归档
  - 默认 `yolo` 模式，适合无人值守场景
  - 智能审批分级策略（low/medium/high 风险等级）
  - 用户白名单访问控制
  - WebSocket 流式推送：text_delta/think_delta/tool_call/tool_result/tool_diff/usage/done

- **A2A 协议** (`vibecoding a2a`)
  - 新增 Agent-to-Agent 协议服务器（JSON-RPC 2.0 over HTTP + SSE 流式）
  - 独立模式：`vibecoding a2a start`（端口 8093）
  - 集成模式：`hermes.json` 中 `a2a.enabled: true`，共享 hermes HTTP 端口
  - Agent Card：`/.well-known/agent.json`
  - Task 生命周期：submitted → working → completed/failed/canceled
  - REST 端点：`/a2a/send`、`/a2a/task`、`/a2a/task/cancel`、`/a2a/events`
  - **A2A Client**：`vibecoding a2a send <message>` 向其他 A2A Server 发送任务
  - **A2A 发现**：`vibecoding a2a discover <url>` 获取远程 Agent Card
  - **A2A 调度**：Cron 任务支持 `--a2a-target` 参数，定时向 A2A Server 发送任务

- **A2A Master 模式** (`--enable-a2a-master`)
  - 通过 `a2a-list.json` 配置多个远程 A2A Agent
  - 注册 `a2a_dispatch` tool，LLM 可自动向远程 agent 分发任务
  - 支持全局（`~/.vibecoding/a2a-list.json`）和项目级（`.vibe/a2a-list.json`）配置
  - `--init-a2a-master-config` 生成示例配置文件
  - 默认关闭，需显式启用

- **A2A 配置初始化**
  - `vibecoding a2a --init-a2a-config` 生成 `a2a.json` 配置模板
  - `vibecoding --init-gateway` 生成 `gateway.json` 配置模板（已有）
  - `vibecoding --init-a2a-master-config` 生成 `a2a-list.json` 配置模板
  - 所有 `--init-*` 支持 `--force` 覆盖已存在的文件

- **场景演示文档**
  - 新增 `docs/scenarios.md`（中英文），覆盖 9 种实际使用场景
  - 涵盖：日常编码、CI 集成、多 Agent、VS Code ACP、A2A 服务器、
    A2A Master 跨机器调度、Gateway HTTP 网关、Hermes 消息平台、组合模式

- **文档全面更新**
  - `architecture.md`：补全全部模块（a2a/acp/gateway/hermes/mcp/memory/messaging/vendored）
  - `tools.md`：新增 `a2a_dispatch` 和 `skill_ref` 工具文档
  - `cli-reference.md`：新增 `--enable-a2a-master`、`--init-a2a-master-config`、
    `--init-gateway`、`--force`、`a2a` 子命令文档
  - `README.md`：架构图补全、新增运行模式总览

- **压力系统**
  - Context Pressure：55% context 使用率时触发 `EventContextPressure`（可通过 `context_pressure_threshold` 配置）
  - Budget Pressure：剩余 20% 迭代时触发 `EventBudgetPressure`（可通过 `budget_pressure_threshold` 配置）
  - 一次性触发：每个阈值越界只触发一次，非每轮触发
  - 消息平台通过进度回调接收压力警告

- **智能审批（分级策略）**
  - low 风险：自动批准
  - medium 风险：自动批准 + 通知用户
  - high 风险（WebSocket）：发送 `approval_request`，等待用户 `approval_response`（5 分钟超时）
  - high 风险（消息平台）：自动拒绝 + 通知用户
  - 命令风险分类：基于 bash 命令模式的 low/medium/high 分级

- **Provider/Model 配置**
  - `hermes.json` 新增 `default_provider` / `default_model`（覆盖 `settings.json`）
  - `hermes start` 新增 `-p`/`--provider` 和 `-m`/`--model` CLI 标志
  - 优先级：CLI 标志 > `hermes.json` > `settings.json`

- **多 Agent 模式** (`--multi-agent`)
  - 启用子 Agent 工具（spawn/status/send/destroy）
  - 通过 `hermes.json` 的 `multi_agent` 字段或 `--multi-agent` CLI 标志配置

- **Sandbox 模式** (`--sandbox`)
  - 可选 bwrap 沙箱隔离（默认关闭）
  - 通过 `hermes.json` 的 `sandbox` 字段或 `--sandbox` CLI 标志配置

- **MCP 工具继承**
  - Hermes 自动加载全局/项目 `mcp.json` 中的 MCP 服务器
  - MCP 工具按 session 注册，session 移除时自动关闭连接

- **消息平台进度事件推送**
  - agent 执行过程中实时向微信/飞书推送工具执行进度
  - 格式：`[tool]: args ✅/❌`（工具）、`💭 ...`（思考过程）
  - agent 完成后发送完整总结

- **memory 工具**
  - `memory` 工具支持 read/add/update/delete 操作
  - section 级操作（User Profile、Working Memory、Lessons Learned）
  - 默认写入 `.vibe/memory.md`（项目目录）
  - 查找优先级：`memory.path` 配置 → `.vibe/memory.md` → `<GLOBAL_DIR>/memory.md`
  - `/api/memory` HTTP 端点（GET/PUT）用于 memory 访问

- **Hermes CLI 命令**
  - `hermes start` — 启动守护进程（支持所有 CLI 标志）
  - `hermes stop` — 通过 PID 文件 + SIGTERM 停止守护进程
  - `hermes status` — 通过 PID + HTTP health 检查守护进程状态
  - `hermes client` — WebSocket 客户端（流式输出 + 斜杠命令）
  - `hermes config init/show` — 配置管理
  - `hermes wechat login/status` — 微信 iLink 管理
  - `hermes feishu setup/status` — 飞书配置
  - `hermes webhook list` — webhook 路由查看
  - `hermes memory show/clear` — memory 管理
  - `hermes sessions list` — 活跃 session 列表（查询运行实例）
  - `hermes cron list/add/remove/enable/disable` — 定时任务管理
  - `a2a start/stop/status/card` — A2A 服务器管理

### 📝 变更

- 微信 iLink 协议实现，零外部依赖（5 个文件：types/protocol/auth/crypto/wechat）
- 飞书 Bot 使用官方 SDK + WebSocket 长连接
- Shell Hooks 支持 pre/post tool call 外部脚本（JSON stdin/stdout）
- Webhook 入站路由，支持 HMAC-SHA256 签名验证
- WebSocket 使用 `golang.org/x/net/websocket`（标准库兼容）
- 基于 PID 文件的守护进程管理（hermes stop/status）

### 🐛 问题修复

- **NPM 安装包修复**
  - 修复发布流水线，确保 `vibecoding-installer` 始终包含可执行入口 `bin/vibecoding`。
  - 新增 `scripts/npm-installer-wrapper.js` 作为统一的 wrapper 逻辑源，并被 `scripts/build-npm.sh`
    与 `scripts/build-npm-packages.sh` 复用，避免实现分叉。
  - 调整 `npm/.npmignore` 与 `npm/bin` 的处理方式，避免误打包非发布文件，并通过 `files` 字段显式声明要发布内容。

- **Hermes Webhook 投递与过滤**
  - 当 webhook 路由无法识别事件类型时，除非显式允许 `*`，否则按不匹配处理。
  - 为 webhook 路由新增 `delivery_target`，让微信/飞书投递拥有明确接收者。
  - 路由列表和配置模板会在存在投递目标时一并展示。

- **OpenAI Responses thinking 映射**
  - 将 `--thinking xhigh` 在 OpenAI Responses API 中映射为 `reasoning.effort: "high"`。

### 🧪 测试

- 将 webhook router 测试改为等待 handler 完成，去掉 `time.Sleep` 带来的竞态和不稳定。
- 增加无法推断事件类型时的 webhook 拒收测试。
- 增加 webhook delivery target 相关测试覆盖。

## v0.1.26

### ✨ 新功能

- **Gateway 模式** (`vibecoding gateway`)
  - 新增 HTTP 服务，对外暴露标准 OpenAI Chat Completions API (`/v1/chat/completions`、`/v1/models`、`/health`)
  - 任何兼容 OpenAI SDK 的客户端（Cursor、Continue、Open WebUI、Python SDK 等）可直接接入
  - 完整支持 Streaming (SSE) 和 Non-streaming 响应
  - 后端由 VibeCoding agent 循环驱动，tool 执行对调用方透明

- **多 Session 支持**
  - 内置 `SessionPool` 支持并发 session，每个 session 拥有独立的 agent、工具和消息历史
  - 通过请求体中的 `x_session_id` 关联 session，未指定时自动创建
  - 可配置空闲超时 (`session.idleTimeoutSeconds`) 和最大 session 数 (`session.maxSessions`)

- **Gateway Sub-Agent 支持**
  - 可选 `enableSubAgents` 配置，在 gateway 模式下启用多 Agent 编排
  - 复用现有 `AgentFactory` / `AgentManager` / 子Agent 工具，无需改动核心 agent 逻辑

- **Bearer Token 认证**
  - 通过 `gateway.json` 的 `auth.enabled` 和 `auth.tokens` 列表配置
  - 默认关闭；`/health` 端点始终不需认证

- **API 指令系统 (Slash Commands)**
  - `/clear`、`/mode`、`/model`、`/models`、`/sessions`、`/compact`、`/status`、`/skill`、`/skills`、`/help`
  - 当最后一条用户消息以 `/` 开头时触发，在 gateway 层直接处理，不调用 LLM
  - 响应使用标准 OpenAI 格式，附加 `x_command` 扩展字段

- **Tool 可见性配置** (`toolVisibility.mode`)
  - `"content"` (默认): streaming 时通过 `content` 字段发送 tool 状态文本
  - `"sse_event"`: 通过扩展 SSE event 发送，适合自定义客户端
  - `"none"`: 完全透明，客户端只见最终文本

- **System Prompt 处理策略** (`systemPromptMode`)
  - `"append"` (默认): 客户端 system message 追加到内置 system prompt 末尾
  - `"ignore"`: 完全忽略客户端 system message

- **安全: allowedWorkDirs 白名单**
  - 请求级 `x_working_dir` 的目录白名单，支持路径分隔符感知的前缀匹配
  - 三层安全模型: L1 认证 + L2 目录管控 + L3 沙箱 (bwrap)

- **Gateway Sandbox 支持**
  - 通过 `gateway.json` 的 `sandbox.enabled` / `sandbox.level` 或 `--sandbox` flag 配置
  - 细节配置（allowedRead、deniedPaths 等）继承 `settings.json`

- **Gateway 配置文件** (`gateway.json`)
  - 独立配置文件，位于 `~/.vibecoding/gateway.json`
  - 覆盖: 监听地址、认证、模式、沙箱、工作目录、目录白名单、session 管理、CORS、tool 可见性、system prompt 策略、请求超时、并发限制、日志
  - `vibecoding --init-gateway` 生成配置模板；`--force` 强制覆盖

- **请求超时与并发控制**
  - `requestTimeoutSeconds` (默认 1800s)；streaming 有数据流动不超时
  - `maxConcurrentRequests` (默认 0 = 不限制)

### 📝 文档

- 新增 `docs/gateway-proposal.md`，包含完整架构、API 设计、安全模型和实现计划
- 更新 `AGENTS.md` 版本标注

## v0.1.25

### ✨ 新功能

- **多 Agent 模式**
  - 在 CLI、TUI、ACP 模式中新增可选的 `--multi-agent` 支持
  - 新增 `AgentManager`、`EventRouter` 和每个 Agent 独立的 registry，隔离工具、job manager、session、messages 与 context
  - 新增 `subagent_spawn`、`subagent_status`、`subagent_send`、`subagent_destroy` 工具，用于派生后台子任务
  - 新增多 Agent system prompt 指引，并限制子 Agent 继续派生子 Agent

- **Cron 定时任务**
  - 新增 `internal/cron`，支持 cron store 持久化与调度器测试覆盖
  - 在多 Agent TUI 工作流中新增 `/cron` 命令入口

- **Provider 厂商适配层**
  - 新增 `internal/provider/vendor*.go` 厂商适配注册机制
  - 将 provider/model 创建逻辑统一到 `internal/provider/factory`
  - 新增 DeepSeek、Xiaomi、Kimi、MiniMax、Seed、Qianfan、Bailian、Gitee、OpenRouter、Together、Groq、Fireworks、OpenAI、Anthropic 等厂商识别
  - 保持现有 provider 配置格式不变，同时支持厂商默认值和通用 OpenAI/Anthropic 兼容 fallback
  - 新增模型 `compat` 处理，覆盖 thinking 格式、reasoning effort、max token 字段、自适应 Anthropic thinking，以及 DeepSeek/Xiaomi assistant `reasoning_content`

### 🐛 问题修复

- session 首次 append 时自动初始化，避免子 Agent 写入 session 前必须显式初始化
- 修复子 Agent 测试中的后台运行清理顺序，确保临时目录删除前已等待并销毁派生 Agent
- 在 provider 创建逻辑迁移到共享 factory 后，保留 ACP Anthropic cache-control 行为

### 📝 文档

- 更新 `AGENTS.md`，补充 provider factory 与 vendor adapter 工作约定
- 将多 Agent 实施 checklist 更新为已落地架构/状态说明
- 删除已过时的根目录 `todo.md`

### 🧪 测试

- 新增 provider vendor 解析、provider factory 创建、OpenAI/Anthropic compat、多 Agent manager/router/sub-agent 流程、cron 存储/调度、session 自动初始化等测试覆盖
- 已通过 `make test`（`go test -v -race ./...`）

---

## v0.1.24

### ✨ 新功能

- **API 重试与指数退避**
  - 对暂时性错误（5xx、网络故障、速率限制）在初始 HTTP 连接阶段自动重试
  - 指数退避策略：`baseDelay × 2^attempt`，上限 30 秒
  - 不会重试：用户中止（`context.Canceled`）、4xx 客户端错误、流传输中途失败
  - 通过 `retry` 配置项（`maxRetries`、`baseDelay`、`maxDelay`）灵活调整
  - Agent 将重试事件作为状态更新透出到 TUI 和 print 模式
  - ACP 模式同样接收重试配置

### 🐛 问题修复

- **Anthropic `cache_control` 改为显式启用**
  - 默认关闭 `cache_control`（此前会根据官方 API base URL 自动启用）
  - 需在 provider 配置中显式设置 `cacheControl: true` 才能启用 prompt 缓存
  - ACP provider 创建时显式为 Anthropic 启用 `cache_control`

- **Anthropic Tool Result 分组**
  - 修复连续 `toolResult` 消息未合并为单条 `user` 消息的问题
  - Anthropic API 要求前一轮 `tool_use` 对应的所有 `tool_result` 块在后续内容之前集中出现
  - 工具结果中的图片块现在会在同一消息中追加到所有结果块之后
  
- **Agent 纯工具循环告警顺序**
  - 将无文本输出的工具循环告警改为在 tool result 追加之后再注入
  - 保持 assistant -> toolResult -> warning 的消息顺序，确保 provider 与 session transcript 都合法
  - 告警消息现在也会持久化写入 session 存储

### 📝 文档

- **配置文档全面重写**
  - 补充缺失配置项：`cacheControl`、空闲压缩、完整沙箱字段（`bwrapPath`、`allowedRead`、`allowedWrite`、`deniedPaths`、`passEnv`、`tmpSize`）、`shellPath`、`shellCommandPrefix`、`sessionDir`、`skillsDir`、`theme`、`retry`
  - 记录 shell 命令格式的 `apiKey`（`!cmd`），支持密码管理器集成
  - 修正密钥解析顺序：优先使用配置中的 `apiKey`，其次使用推导的环境变量
  - 更新 macOS 配置路径文档
  - 新增顶层字段参考表及所有默认值
  - 新增各平台沙箱路径与环境变量默认值
  - 改进示例：Claude provider `cacheControl`、空闲压缩、项目级覆盖、自定义沙箱路径

### 🧪 测试

- 新增重试测试，覆盖 `IsRetryable`、`RetryDelay` 和 `FormatRetryMessage`
- 新增 Anthropic provider 测试，覆盖连续 tool result 分组
- 新增回归测试，覆盖 tool result 之后的纯工具循环告警插入位置

---

## v0.1.23

### 🛠 改进

- **DeepSeek Thinking 格式**
  - 新增 `thinkingFormat: "deepseek"`，用于 DeepSeek 推理请求
  - OpenAI 兼容请求现在会发送 `thinking: {type: "enabled"}` 和 `reasoning_effort`
  - Anthropic 兼容请求现在会发送 `thinking: {type: "enabled"}` 和 `output_config.effort`
  - 保留 `thinkingFormat: "xiaomi"` 作为旧的 thinking-only 格式

### 🧪 测试

- 新增 provider 测试，覆盖 OpenAI 与 Anthropic 兼容请求下的 `deepseek` thinking 格式

### 📝 文档

- 更新 `anthropic-api` skill 与配置文档中关于 `thinkingFormat` 选项的说明

---

## v0.1.22

### ✨ 新功能

- **CLI/TUI MCP 自动加载**
  - CLI/TUI 启动时现在会加载全局与项目 `mcp.json`，连接已配置的 MCP 服务器，并在 agent 工具列表冻结前注册 MCP 工具

### 🐛 问题修复

- **Markdown 渲染样式**
  - 将 CLI print 模式和 TUI 的 Markdown 渲染从 Glamour 自动样式检测改为固定 `dark` 样式，提升不同终端中的显示一致性

### 🧪 测试

- 新增 MCP 配置加载测试，覆盖模板占位服务器过滤

### 🛠 改进

- **共享 MCP 运行时**
  - 将 MCP 连接与工具注册从 ACP 私有实现提取为共享运行时，ACP 与普通 CLI/TUI 会话复用同一套逻辑
  - 自动启动加载时会忽略 starter 模板中的占位 MCP 服务器

---

## v0.1.21

### ✨ 新功能

- **Plan/Apply 工作流**
  - 新增内置 `plan` 工具，用结构化任务计划表达 `pending`、`running`、`done` 和 `failed` 步骤状态
  - TUI 现在会展示当前任务计划，并把计划更新记录到对话历史中
  - Print 模式和 ACP 现在也会透出计划更新，支持非交互和编辑器客户端流程

- **Apply 确认**
  - 新增 `approval.confirmBeforeWrite`，用于在 Agent 模式下要求 `write` 和 `edit` 执行前审批
  - 新生成的默认配置会启用写入/编辑确认
  - TUI 审批提示会用字节数摘要写入内容，避免直接展示完整文件内容

- **MCP 配置命令**
  - 新增 `/init_mcp`，支持创建项目/全局 `mcp.json`，并提供 `basic`/`full` 模板及 `--force` 覆盖
  - 新增 `/mcps`，用于列出全局与项目 `mcp.json` 中的 MCP 服务器
  - MCP 配置改为独立 `mcp.json`（不与 `settings.json` 混用）

### 🧪 测试

- 新增 `plan` 工具和 write/edit 审批门控测试覆盖
- 新增基于 HTTP 的 MCP 集成测试，覆盖 tool/resource/prompt 注册与回调链路
- 新增基于 SSE 的 MCP 集成测试，覆盖流通知回调与 message endpoint 请求/响应链路

### 🛠 改进

- **ACP MCP 健壮性增强**
  - 新增 `http` 和 `sse` MCP 传输支持（保留现有 `stdio`）
  - 为 MCP 初始化与工具发现增加超时控制，避免 ACP 会话长时间挂起
  - 为 `tools/list` 增加分页拉取与页数上限保护
  - 新增 MCP `resources/*` 与 `prompts/*` 发现和工具注册
  - 增加 MCP 服务器重名检测与 MCP 工具名去重注册
  - 增加 MCP 入站请求/通知处理（`ping`、progress/logging/cancel 通知）
  - 新增入站 `sampling/createMessage` 到当前 ACP provider/model 的桥接
  - 收紧关闭/错误传播行为

---

## v0.1.20

### ✨ 新功能

- **结构化文件变更报告**
  - `write` 和 `edit` 现在会在工具结果中附带结构化文件 diff 元数据
  - TUI 工具详情中展示完整 unified diff，折叠工具行保留简洁的 `+N -N` 摘要
  - Print 模式现在会为非交互运行输出清晰的文件变更摘要
  - ACP 工具更新会在 raw output 中包含 diff 元数据，方便兼容客户端使用

### 🧪 测试

- 新增 `write` 和 `edit` 结构化 diff 元数据测试覆盖

---

## v0.1.19

### ✨ 新功能

- **TUI 工具详情 Modal**
  - 将 `Ctrl+O` 切换展开替换为可滚动的全屏 modal overlay，展示所有工具调用及结果
  - 支持 PgUp/PgDn、Up/Down、Home/End 导航；Esc/Ctrl+O/q 关闭
  - 工具标题现在显示文件路径；移除了工具参数中的内容截断
  - Write 工具结果在摘要行显示 diff 信息
  - Modal 打开时屏蔽键盘输入，防止误操作

- **Write 工具 Diff 摘要**
  - `write` 工具现在在覆盖文件时基于 LCS 算法计算行级 diff
  - 在工具结果中返回结构化 diff 信息（`+N -N` 及行范围）
  - 对超大文件（>20 万行对）跳过 diff 计算，避免内存压力

### 🛠 改进

- **沙箱后端统一 Shell 参数**
  - 所有沙箱后端（`none`、`mac`、`windows`）现在统一使用 `platform.ShellArgs()` 构造 cmd.exe/PowerShell 参数
  - 修复沙箱模式下 Windows cmd.exe 和 PowerShell 命令执行问题
  - `ShellArgs` 现在在匹配前将 shell 名称转为小写

### 🧪 测试

- 新增 `TestNoneSandboxWrapCommandUsesPlatformShellArgs`，覆盖 cmd.exe 和 PowerShell 参数生成

---

## v0.1.18

### 🐛 问题修复

- **TUI Nil 指针 panic**
  - 修复 `printMessageOnce` 在 `printedMessageIdx` map 未初始化时导致的 nil 指针 panic
  - 添加 nil 检查，确保在消息打印逻辑中安全访问 map

- **工具执行前提交流**
  - 添加 `commitActiveStream()` 方法，用于在工具执行前将流式内容（thinking 和 assistant 消息）刷新到输出
  - 现在在 `EventToolCall` 和 `EventToolApprovalRequest` 处理前正确提交活跃的流
  - 确保在工具运行或请求审批时能看到 thinking 和部分 assistant 响应

### 🧪 测试

- 新增 `TestHandleAgentEventCommitsStreamBeforeApproval` 回归测试，覆盖流提交顺序

---

## v0.1.17

### 🛠 改进

- **TUI 原生滚动历史**
  - 重构 TUI 历史渲染：已完成消息会输出到终端原生 scrollback，而不是固定高度 viewport
  - 移除虚拟滚动条与鼠标捕获方案，鼠标滚轮现在使用终端自身的历史滚动行为
  - 保留实时流式内容、输入框、footer、上下文/缓存状态以及工具输出控制

- **TUI 请求计时器**
  - 响应运行期间显示本次请求耗时
  - 请求完成后在 footer 保留上一次请求耗时

- **事件循环解耦**
  - 新增共享的 agent event 消费辅助逻辑
  - 将 TUI 的 agent event bridge 从主 app 文件拆出，并让 CLI print 模式复用同一套事件消费逻辑

- **Windows 控制台兼容性**
  - 在可用时启用 Windows Virtual Terminal 控制台模式，改善 Windows 10 PowerShell 下的显示兼容性

### 🐛 问题修复

- 修复 TUI 启动时在 Bubble Tea 开始消费消息前打印初始/会话历史导致的卡死问题
- 修复 `go test -race` 发现的 agent 消息历史数据竞争
- 修复 mock provider 在 context 已取消时未稳定返回取消错误的问题

### 🧪 测试

- 全量 `make test` 已通过 race detection
- 新增 TUI 启动历史打印不阻塞的回归测试
- 增强受限环境下依赖本地 HTTP listener 或默认 home 目录会话路径的测试稳定性

---

## v0.1.16

### 🛠 改进

- **通过 ID 或路径打开会话**
  - 新增 `OpenByPathOrID` 函数，支持通过文件路径或会话 ID 打开会话
  - `OpenByID` 现在支持前缀匹配，并具备歧义检测
  - `ContinueRecent` 在创建新会话时立即初始化，确保可直接写入消息

- **会话保存错误处理**
  - `AppendMessage` 和 `AppendCompaction` 现在会向调用方返回错误
  - Agent 循环将会话保存失败作为 `EventError` 上报，不再静默丢弃

- **内嵌工具测试守卫**
  - Makefile `test` 目标现在依赖 `prepare-vendored` 和新增的 `test-vendored` 检查
  - 若当前平台缺少 `rg`/`fd` 二进制文件，测试会提前失败并给出明确提示

### 🧪 测试

- 新增 CLI flag 解析测试，覆盖 root 和 ACP 子命令
- 新增配置合并测试，覆盖项目级覆盖和环境变量
- 新增会话测试，覆盖 `OpenByPathOrID`、前缀歧义、损坏行和父链追踪

---

## v0.1.15

### 🐛 问题修复

- **内嵌搜索工具可用性**
  - 修复 `grep` 和 `find`：当内嵌的 `rg` / `fd` 尚未释放到本地时，会按需准备二进制文件，而不是直接失败
  - 为已释放的内嵌二进制补齐可执行权限，避免复用时出现 `permission denied` 错误

- **Bash 工具结果处理**
  - 修复 bash 工具返回内容，稳定输出 stdout、stderr、工作目录和退出码等结构化信息
  - 将命令非零退出保留为正常工具结果，并通过明确的 `exit_code` 字段表达，而不是混入传输级错误
  - 统一将空 stdout/stderr 渲染为 `(no output)`，便于下游稳定处理

---

## v0.1.14

### 🐛 问题修复

- **继续会话上下文注入（`-c`）**
  - 修复 TUI 状态耦合问题：继续会话时可能只显示历史记录，但后续提问未将历史真正注入模型上下文
  - 将会话历史状态拆分为“UI 展示标记”和“Agent 注入标记”，确保恢复会话后可持续携带上下文
  - 在 agent 重建场景（中止/模式切换/模型切换/技能切换/会话切换）统一重置历史注入状态
  - 补充 `EventStatus` 与 `EventMessageStart` 的 TUI 事件处理，确保状态/警告消息稳定渲染

### 🧪 测试

- 新增回归测试覆盖：
  - UI 历史已加载时的历史注入
  - 继续会话真实启动时序（`Init()` 先加载历史，再处理后续输入）

---

## v0.1.13

### 🐛 问题修复

- **流式事件与工具调用健壮性**
  - 保留 TUI 事件监听器中的 agent 事件，避免流式过程中丢失 done/error/status 处理
  - 为 Anthropic 增加 thinking signature 的流式接收与多轮回放支持，并将 SSE `error` 事件正确上报为流错误
  - 当 OpenAI 兼容 provider 在流式工具调用中省略 ID 时，自动生成回退 ID，并在 agent 循环中增加额外防御性回退

- **沙箱环境继承**
  - 修复 `none` 沙箱执行未继承父进程环境的问题，包括 `$HOME` 等环境变量
  - 明确 bubblewrap 环境变量覆盖逻辑，使实现与实际运行行为一致

### 🛠 改进

- **内嵌工具构建流程**
  - 围绕 `prepare-vendored` 统一构建与发包流程
  - 移除旧的 `vendored-tools` 发布步骤，并废弃过时的提取辅助脚本

- **文档站点布局**
  - 扩大文档首页内容区宽度，提升大屏阅读体验

- **包元数据**
  - 更新 npm 安装器相关包版本

### 📖 文档

- 更新 README 与文档首页，突出更安全的审批处理、统一缓存指标和一致的 provider 调试行为
- 精简仓库内 agent 使用说明 `AGENTS.md`

### 🧪 测试

- 为 bash 工具补充仅 stdout、仅 stderr、无输出、非零退出码等输出场景覆盖
- 为 TUI 增加状态/警告渲染与 done/error 事件透传的回归测试
- 为缺失 ID 的 OpenAI 流式工具调用增加回归测试

---

## v0.1.12

### 🐛 问题修复

- **统一缓存命中率语义**
  - 将缓存命中率计算恢复为基于完整 prompt 输入足迹（`CacheRead / TotalInputTokens()`）
  - 让 CLI print 模式的 token 显示与 TUI 的缓存感知总量保持一致
  - 更新 Anthropic 缓存测试与通用 provider usage 测试，使其与统一定义对齐

- **非交互与 YOLO 流程中的审批安全性**
  - 让 `bashBlacklist` 在审批检查中真正生效，且优先级高于 `bashWhitelist`
  - 在 `yolo` 模式下，命中黑名单的 bash 命令仍然要求审批
  - `--print` 模式遇到本应需要用户确认的命令时，改为直接报错退出，而不是自动批准

### 🛠 改进

- **调试输出一致性**
  - `--debug` 现在会同时启用 provider 级请求/响应调试输出
  - ACP 模式下也采用相同行为

- **跨平台路径处理**
  - 将 `.skills` 路径构造从字符串拼接改为 `filepath.Join(...)`

### 📖 文档

- 更新 CLI 参考文档，说明更严格的 `--print` 行为与 debug 输出行为
- 更新配置文档，说明审批优先级与 `VIBECODING_DEBUG`
- 更新根 README 与文档首页，突出更安全的审批处理、统一缓存指标和 provider 调试行为

### 🧪 测试

- 新增白名单/黑名单及 `yolo` 模式下的审批行为测试
- 新增 print 模式中需审批工具调用的回归测试
- 扩展 cache 相关 provider 测试，覆盖统一后的缓存命中率定义

---

## v0.1.11

### 🛠 改进

- **命令结构重构**
  - 将根命令创建提取为独立函数，提升可测试性
  - 新增命令初始化和配置的单元测试
  - 提高代码模块化和可维护性

### 📖 文档

- **许可证与文档更新**
  - 新增 MIT 许可证文件
  - 新增中文 README（README_zh.md），提升中文用户体验
  - 更新 npm 包版本

---

## v0.1.10

### ✨ 新功能

- **ACP 支持文档**
  - 在 README 中添加 ACP（Agent Client Protocol）支持文档
  - VibeCoding 可作为 ACP stdio Agent 运行，用于编辑器集成
  - 兼容 VS Code、Zed 和 JetBrains IDE（IntelliJ IDEA/WebStorm），通过 ACP 兼容插件接入

### 📖 文档

- 更新主 README.md 添加 ACP 支持特性
- 更新英文 README 添加功能特性部分
- 更新中文 README 添加功能特性部分

---

## v0.1.9

### 🐛 问题修复

- **TUI 延迟渲染协程安全**
  - 修复 `scheduleRender` 从后台协程直接调用 `updateViewportContent` 而未归队到 Bubble Tea UI 协程的问题
  - 新增 `renderRequestMsg` 类型和 `program.Send()` 方法，确保 UI 更新正确归队
  - 新增 `program *tea.Program` 字段和 `SetProgram()` 方法支持延迟 UI 调度

### 🛠 改进

- **TUI 中止时清空输入队列**
  - 手动中止和模式切换时清空输入队列并重置输入状态
  - 防止缓冲按键在中止后继续执行

- **助手消息槽位预留**
  - 新增 `EventTurnStart` 处理，在文本增量到达前预留显示槽位
  - 防止工具输出在流式传输过程中改变助手消息索引
  - 在 `updateViewportContent` 中增加空原始 markdown 检查

- **工具提示片段优化**
  - 为 `read`、`ls`、`grep`、`find` 工具描述添加 "(preferred for ...)" 提示
  - 调整工具注册顺序：只读工具优先注册在 write/edit/bash 之前

### 🧪 测试

- 新增 `TestHandleAgentEventReservesAssistantSlotBeforeTextDelta` 测试
- 新增 `TestAbortClearsQueuedInput` 测试

---

## v0.1.8

### 🐛 问题修复

- **缓存感知的 Token 计算修复**
  - 修复 Anthropic `TotalTokens` 计算未包含 `CacheRead` 和 `CacheWrite` 的问题
  - 为 `Usage` 结构体添加 `PromptTokens()` 和 `TotalInputTokens()` 辅助方法
  - 更新 `CacheInfo()` 使用 `TotalInputTokens()` 作为分母，确保缓存命中率准确
  - 更新 TUI 显示正确的 token 计数（包含缓存 token）

### 🧪 测试

- 添加 `PromptTokens()` 和 `TotalInputTokens()` 辅助方法的综合测试
- 更新 Anthropic provider 测试以验证 `TotalTokens`

---

## v0.1.7

### 🐛 问题修复

- **Anthropic Provider Tool Use 序列化**
  - 修复 `tool_use` 内容块在 tool 无参数时缺少 `input` 字段的问题
  - 将 `Input` 字段从 `map[string]interface{}` 改为 `*map[string]interface{}`，使 `omitempty` 仅检查指针是否为 nil，而非空 map
  - 修复使用小米 MiMo 等 Anthropic 兼容端点时的 API 错误

---

## v0.1.6

### ✨ 新功能

- **会话管理命令**
  - 新增 `/sessions` 命令，用于浏览和管理项目会话
  - 支持列出、切换、清除和删除会话
  - 显示会话详情，包括文件路径和消息数量

### 🐛 问题修复

- **沙箱初始化**
  - 修复沙箱初始化验证和 bwrap 多架构兼容性问题
  - 改进沙箱设置的错误处理

### 📖 文档

- 更新 AGENTS.md 中的当前版本信息
- 格式化 Go 代码以保持一致性

---

## v0.1.5

### ✨ 新功能

- **DeepSeek V4 默认模型**
  - 更新默认模型规格为 DeepSeek V4（Flash 和 Pro）
  - 100 万上下文窗口，最高 38.4 万最大输出 token
- **安装脚本改进**
  - 安装完成后显示配置目录路径

### 🐛 问题修复

- **Windows IME 支持**
  - 修复 Windows 终端的 IME（中日韩输入法）支持
  - 修复 Windows 上的 shell 命令解析
  - 新增配置加载诊断信息，便于排查问题
- **Musl Deb 包**
  - 修复 musl deb 包使用无效 dpkg 架构名的问题

### 🛠 改进

- **配置简化**
  - 移除 `auth.json` 支持 — 所有凭据统一使用 `settings.json`
  - 更简洁的配置路径，单一数据源

### 📖 文档

- 明确说明 OpenAI/Anthropic 兼容 API 服务也受支持
- 从文档和安装脚本中移除所有 `auth.json` 引用
- 新增 Windows `%APPDATA%` 路径的详细示例
- 清晰区分 Windows 与 Linux/macOS 的配置路径

---

## v0.1.4

### ✨ 新功能

- **Linux musl 构建支持**
  - 新增 `make build-linux-musl` 目标，静态链接 musl 二进制文件（amd64 + aarch64）
  - 通过 `dist-tarball` 和 `dist` 目标生成 musl tarball 包
  - 通过 `dist-deb` 目标生成 musl Debian 包（amd64-musl / arm64-musl）
  - npm 包：`vibecoding-installer-linux-musl-x64` 和 `vibecoding-installer-linux-musl-arm64`
  - npm 使用 `libc` 字段实现 musl/glibc 正确解析（npm >=9.4）
  - postinstall.js 自动检测 Linux 上的 musl 与 glibc

---

## v0.1.3

### ✨ 新功能

- **版本规则**
  - 新增版本号管理规则：版本号采用十进制进位（如 v0.1.9 -> v0.2.0）
  - 明确 changelog 编写规则：只在 docs/en/changelog.md 和 docs/zh/changelog.md 中编写
  - 不创建单独的 release notes 文件

---

## v0.1.2

### ✨ 新功能

- **Prompt Cache 优化**
  - 实现了基于 LLM_Agent_Cache.md 策略的提示缓存优化
  - 跨多轮对话缓存系统提示和静态上下文
  - 通过重用缓存 token 减少 API 成本

- **TUI Markdown 语法高亮**
  - TUI 中的助手消息现在支持 markdown 语法高亮
  - 代码块、标题和格式化内容有视觉区分
  - 提升 LLM 响应的可读性

### 🐛 问题修复

- **安全与正确性**
  - 解决了关键的安全、竞态条件和正确性问题
  - 修复了代码库中的高、中严重性正确性问题
  - 移除了死代码，提高了整体代码正确性

- **TUI 稳定性**
  - 修复了在不支持的 stdin 上 `clearStdin` 阻塞导致的 TUI 启动挂起
  - 修复了 ANSI 转义码在前缀检查中导致的 TUI 助手消息渲染损坏

### 🛠 改进

- **代码质量**
  - 修复了代码库中剩余的中等严重性问题
  - 更新了 npm 包版本

---

## v0.1.1

### ✨ 新功能

- **缓存命中率显示**
  - 状态栏现在显示所有轮次的累计缓存命中百分比
  - 缓存命中率 ≥ 50% 时高亮显示，便于快速识别
  - 每轮 token 使用行新增缓存读写数量显示

- **代理兼容性**
  - 支持在 `message_delta` 而非 `message_start` 中发送 usage 字段的代理
  - 支持将 usage 拆分到多个 SSE chunk 的 OpenAI 代理（每个字段取首次出现的值）
  - 修复 print 模式 token 汇总行 `$` 前缺少空格的问题

### 🛠 改进

- **代码质量**
  - 提取 `Usage.CacheInfo()` 消除 3 处重复的缓存显示逻辑
  - npm 包版本号改为 `v` 前缀格式（如 `v0.1.1`）
  - 统一所有 npm package.json 的 JSON 格式

### 🧪 测试

- 新增 37 个单元测试覆盖 `CacheInfo()`、`formatCachePercent()` 和 `renderFooter()` 缓存部分
- 新增 12 个 httptest 集成测试覆盖 Anthropic 和 OpenAI SSE 缓存 token 解析

---

## v0.1.0

### ✨ 新功能

- **小米 MiMo thinking 格式支持**
  - 新增 `thinkingFormat` 配置选项，支持小米 MiMo API 格式
  - OpenAI provider: 小米端点使用 `thinking: {type: "enabled"}` 格式
  - Anthropic provider: 小米端点省略 `budget_tokens`
  - URL 自动检测：未设置 `thinkingFormat` 时自动检测 `xiaomimimo` 端点
  - 调试日志：通过 `VIBECODING_DEBUG` 环境变量启用

### 🛠 改进

- **配置灵活性**
  - `thinkingFormat` 从配置传递到 provider，不再仅依赖 URL 检测
  - Anthropic `budget_tokens` 从必需改为可选（指针类型 + `omitempty`）

---

## v0.0.9

### ✨ 新功能

- **工具图像支持**
  - `read` 工具现在支持读取图像文件（PNG、JPEG、GIF、WebP）
  - 图像以 base64 编码数据和 MIME 类型信息返回
  - LLM 现在可以分析和理解图像内容
  - 支持格式：`.png`、`.jpg`、`.jpeg`、`.gif`、`.webp`

- **富内容工具结果**
  - 新的 `ToolResult` 结构体支持纯文本和富内容块
  - 工具现在可以在单个结果中返回文本 + 图像
  - 新增工厂函数：`NewTextToolResult()` 和 `NewImageToolResult()`

- **模型切换**
  - `/model <id>` 命令允许在交互模式下切换模型
  - `/model` 不带参数显示当前模型和可用选项
  - 切换模型时自动重置 Agent

- **增强的帮助系统**
  - `/help` 命令现在显示详细的命令说明
  - 新增键盘快捷键参考（Tab、Esc、Ctrl+O、PgUp/PgDn）

### 🛠 改进

- **上下文 Token 估算**
  - 修复了同时存在 `Content` 和 `Contents` 时的重复计算问题
  - 图像 token 估算为每张图约 1200 token

- **提供商消息转换**
  - OpenAI：工具结果中的图像作为补充用户消息发送
  - Anthropic：图像作为单独的用户消息与 tool_result 一起发送

### 🧪 测试

- 新增 `TestReadToolImage` 测试用例验证图像读取功能
- 所有工具测试已更新为新的 `ToolResult` 返回类型

---

## v0.0.8

### ✨ 新功能

- **NPM 多架构分包优化**
  - 将 npm 包从单包全平台（~60MB）拆分为 6 个平台独立包（每个 ~10MB）
  - 用户安装时只下载当前平台的二进制文件，体积减少 83%
  - 利用 npm `optionalDependencies` + `os`/`cpu` 字段自动匹配平台
  - 主包 `vibecoding-installer` 仅 ~2KB，通过 `postinstall` 链接正确的平台包

### 🛠 改进

- **构建系统**
  - 新增 `scripts/build-npm-packages.sh` 生成平台独立 npm 包
  - 新增 `make npm-packages`、`make npm-pack`、`make npm-publish-all` 目标
  - `sync-npm-version.sh` 同步更新所有平台包版本

---

## v0.0.7

### ✨ 新功能

- **跨平台沙箱支持**
  - 沙箱现在除 Linux 外还支持 macOS 和 Windows
  - macOS 使用 `sandbox-exec` 进行进程隔离
  - Windows 使用受限进程创建，禁止网络访问
  - 自动选择平台特定的沙箱实现

- **仓库重命名**
  - 模块路径更名为 `github.com/startvibecoding/vibecoding`
  - 所有导入、文档和脚本已同步更新

### 🛠 改进

- **平台特定进程处理**
  - 将 `SysProcAttr` 配置提取到构建标签文件（`bash_unix.go`、`bash_windows.go`）
  - 后台子进程清理现在在所有平台上正常工作
  - `Setpgid` 仅在 Unix 系统上设置；Windows 使用 `CREATE_NEW_PROCESS_GROUP`

### 📖 文档

- 更新所有 GitHub URL 至新仓库地址
- 新增 v0.0.6 和 v0.0.7 发布说明

---

## v0.0.6

### 🛠 改进

- **Bash 工具可靠性**
  - 修复后台子进程挂起问题
  - 添加 `WaitDelay` 防止 shell 无限等待后台子进程
  - 正确处理 `exec.ErrWaitDelay` 错误

- **NPM 安装**
  - 新增 npm 包，支持通过 `npm install -g vibecoding-installer` 安装
  - `postinstall` 时自动下载二进制文件

### 📖 文档

- 新增 npm 安装说明
- 移除 docs 根目录下冗余的 markdown 文件
- 新增 v0.0.5 更新日志

---

## v0.0.5

### ✨ 新功能

- **非 root 安装**
  - `install.sh` 现在支持无需 root 或 sudo 权限安装
  - 自动检测可写安装目录：优先使用 `/usr/local/bin`，若不可写则回退到 `~/.vibecoding/bin`
  - 移除所有 `sudo` 调用 — 用户级安装不再需要提升权限

- **自动 PATH 配置**
  - 自动检测用户 shell（bash、zsh、fish）并在相应配置文件中配置 PATH
  - 支持 `.bashrc`、`.bash_profile`、`.zshrc`、`.zshenv`、`config.fish` 和 `.profile`
  - 若 PATH 条目已存在则跳过配置（避免重复）
  - Fish shell 使用 `set -gx PATH` 语法；bash/zsh 使用 `export PATH=...`

### 🛠 改进

- **环境变量**
  - `INSTALL_DIR` — 覆盖安装目录（不变）
  - `AUTO_SETUP_PATH=0` — 禁用自动 PATH 配置
  - 更好的权限问题错误提示

- **安装体验**
  - 开始时显示安装目录和 PATH 自动配置状态
  - 更清晰的彩色状态消息输出

### 📖 文档

- 新增 v0.0.5 发布说明

---

## v0.0.4

### ✨ 新功能

- **Agent 模式审批机制**
  - Agent 模式下执行 bash 命令需要用户审批
  - 支持 `bashWhitelist` 配置，白名单中的命令自动批准
  - 支持 `bashBlacklist` 配置，黑名单中的命令始终需要审批
  - TUI 中显示审批提示，用户输入 `y`/`yes` 或 `n`/`no` 响应
  - 审批请求支持 `abort` 取消

- **模式权限矩阵**
  - Plan 模式: 只读工具 (read, grep, find, ls)
  - Agent 模式: 读写自动执行，bash 需审批
  - YOLO 模式: 所有工具自动执行
  - 更新系统提示词，明确每个模式的权限

### 🛠 改进

- **默认审批白名单**
  - 默认白名单: `go`, `make`, `git`, `npm`, `yarn`, `node`, `python`, `pip`
  - 可在 `settings.json` 中自定义

- **模式切换反馈**
  - 切换模式时显示详细权限说明
  - `/mode` 命令显示当前模式的完整权限列表

### 📖 文档

- 新增审批配置章节
- 更新安全文档，说明审批机制
- 新增 v0.0.4 发布说明

---

## v0.0.3

### ✨ 新功能

- **会话历史加载**
  - 继续或打开会话时显示会话信息（文件路径和消息数量）
  - 在 TUI 中加载并显示历史会话消息
  - 将历史消息加载到 Agent 上下文中以保持连续性
  - 中止时重置 Agent 以确保下次请求状态干净

### 🛠 改进

- **构建与分发系统**
  - 重构 Makefile，按平台划分构建和分发目标
  - 新增 `dist-linux`、`dist-darwin`、`dist-windows` 目标
  - 新增 `build-zip.sh` 用于 Windows zip 打包
  - 新增 `checksums` 目标用于发布校验
  - 更新 `build-deb.sh` 和 `build-tarball.sh` 支持全平台

### 📖 文档

- 文档网站右上角新增 GitHub 仓库跳转按钮
- 新增 v0.0.2 更新日志

---

## v0.0.2

### ✨ 新功能

- **一键安装脚本**
  - `install.sh` 适用于 Linux/macOS，自动从 GitHub Releases 下载
  - `install.ps1` 适用于 Windows PowerShell，支持通过 `VIBECODING_INSTALL_DIR` 自定义安装目录
  - 两个脚本均可自动检测平台/架构、校验完整性并配置 PATH

- **文档站重新设计**
  - 采用 Google Material Design 风格重新设计
  - 默认语言改为英文
  - 新增 Hash 路由，方便文档分享（如 `#/en/README`、`#/zh/configuration`）
  - 头部和 README 新增 Logo

- **品牌素材**
  - 新增 `docs/assets/icon.svg`（512×512）用于打包
  - 新增 `docs/assets/logo.svg`（128×128）用于 README 和小尺寸显示
  - 简洁专业的石板色调设计

- **构建系统**
  - 新增 `make build-windows` 目标（amd64 + arm64）
  - 新增 `make build-linux` 和 `make build-darwin` 目标
  - 更新 `make build-all` 使用平台专用目标

- **文档**
  - 新增 `docs/en/skills.md` 技能系统文档
  - 更新 README 和快速入门中的安装说明

### 🐛 问题修复

- 将素材移至 `docs/assets/` 以支持 GitHub Pages 部署

---

**完整变更日志**: https://github.com/startvibecoding/vibecoding/compare/v0.1.26...v0.1.27
