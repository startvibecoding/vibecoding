# 场景演示

本文档通过实际场景演示 VibeCoding 的各种使用模式。

---

## 场景 1：日常写代码（单 Agent 模式）

最基础的使用方式——在终端中与 AI 对话，完成日常开发任务。

### 环境准备

```bash
# 设置 API 密钥
export DEEPSEEK_API_KEY=sk-...

# 进入项目目录
cd ~/projects/myapp
```

### 场景 A：代码解释

```bash
$ vibecoding "解释 internal/agent/agent.go 的核心逻辑"
```

VibeCoding 会：
1. 加载上下文文件（`AGENTS.md`、`CLAUDE.md`）
2. 用 `read` 读取文件
3. 用 `ls`、`grep` 探索相关代码
4. 输出结构化的解释

### 场景 B：修复 Bug

```bash
$ vibecoding
```

```
> 修复 handler.go 中的空指针错误

[Agent]
  📄 read("internal/handler.go", offset=45, limit=20)
  → 发现第 52 行未检查 err 是否为 nil

  📝 edit("internal/handler.go", edits=[
    {oldText: "resp, _ := client.Do(req)", newText: "resp, err := client.Do(req)\n\tif err != nil {\n\t\treturn nil, err\n\t}"}
  ])
  → 编辑成功

  🔧 bash("go build ./...")
  → 编译通过

✅ 已修复：在第 52 行添加了错误检查
```

### 场景 C：添加新功能

```
> 给 HTTP handler 添加 /health 端点

[Agent]
  📄 plan(title="添加 /health 端点", steps=[
    {title: "阅读现有 handler 结构", status: "running"},
    {title: "添加 health handler", status: "pending"},
    {title: "注册路由", status: "pending"},
    {title: "添加测试", status: "pending"},
    {title: "验证编译", status: "pending"}
  ])

  📄 read("internal/server/routes.go")
  📝 edit(...)  # 添加 handler
  📝 edit(...)  # 注册路由
  🔧 bash("go test ./internal/server/ -run TestHealth")
  🔧 bash("go build ./...")

  📄 plan(..., steps=[..., {title: "验证编译", status: "done"}])

✅ 完成：/health 端点已添加并测试通过
```

### 模式选择建议

| 任务 | 推荐模式 | 命令 |
|------|---------|------|
| 阅读代码、学习项目 | `plan` | `vibecoding --mode plan` |
| 写代码、修 bug | `agent`（默认） | `vibecoding` |
| 安装依赖、系统操作 | `yolo` | `vibecoding --mode yolo` |

---

## 场景 2：非交互模式（CI/脚本集成）

在 CI 流水线或脚本中使用 VibeCoding。

### 场景 A：代码审查

```bash
# 在 CI 中审查 PR
git diff main..feature | vibecoding -P "审查这个 diff，指出潜在问题"
```

### 场景 B：自动重构

```bash
# 批量重构
vibecoding -P "将所有 fmt.Errorf 调用改为使用 %w 包装错误" --mode yolo
```

### 场景 C：生成文档

```bash
# 为包生成 README
vibecoding -P "为 internal/cache 包生成 README.md，包括使用示例" --mode yolo
```

---

## 场景 3：多 Agent 模式（复杂任务委托）

通过 `--multi-agent` 启用子 Agent 工具，将复杂任务拆分并行执行。

### 启动

```bash
$ vibecoding --multi-agent
```

### 场景：并行重构和测试

```
> 我需要：1) 将 internal/cache 包重命名为 internal/store
>        2) 同时确保所有测试通过

[Agent]
  🤖 subagent_spawn(task="将 internal/cache 重命名为 internal/store，更新所有导入路径",
                     mode="agent",
                     tools=["read", "write", "edit", "bash", "grep", "find"])

  → 返回 handle: "agent-1"

  🤖 subagent_spawn(task="运行完整测试套件，报告失败的测试",
                     mode="agent",
                     tools=["read", "bash", "grep", "find"])

  → 返回 handle: "agent-2"

  ... 等待子 Agent 完成 ...

  🤖 subagent_status(handle="agent-1")
  → 状态: completed
  → 结果: "已将 cache 包重命名为 store，更新了 15 个文件的导入路径"

  🤖 subagent_status(handle="agent-2")
  → 状态: completed
  → 结果: "3 个测试失败：TestCacheGet, TestCacheSet, TestCacheDelete"

  🤖 subagent_send(handle="agent-1", message="修复 agent-2 报告的 3 个失败测试")

  ... 子 Agent 继续工作 ...

✅ 完成：包已重命名，所有测试通过
```

### 子 Agent 工具汇总

| 工具 | 用途 |
|------|------|
| `subagent_spawn` | 启动子 Agent，返回 handle |
| `subagent_status` | 查询子 Agent 状态和结果 |
| `subagent_send` | 向子 Agent 发送后续指令 |
| `subagent_destroy` | 停止并清理子 Agent |

### 多 Agent + Cron 定时任务

```bash
# 每天早上运行代码审查
vibecoding hermes cron add "daily-review" \
  "审查最近 24 小时的 git 变更，输出问题报告" \
  --schedule "@daily"
```

---

## 场景 4：VS Code ACP 集成

在 VS Code 中直接使用 VibeCoding 作为 AI 编码助手。

### 步骤 1：安装

```bash
npm install -g vibecoding-installer
```

### 步骤 2：配置 VS Code

编辑 VS Code 的 `settings.json`：

```json
{
  "acp.agents": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp", "--mode", "agent", "--multi-agent"],
      "description": "VibeCoding AI Assistant"
    }
  }
}
```

### 步骤 3：使用

1. 在 VS Code 中打开项目
2. 打开 ACP 面板（通过扩展）
3. 直接提问或请求代码更改

**VS Code 中的体验：**

```
你: 将 utils.go 中的 ParseConfig 函数改为支持 YAML 格式

VibeCoding:
  [tool_call: read utils.go]
  [tool_call: edit utils.go]
  [tool_call: bash "go test ./..."]
  ✅ 已添加 YAML 支持，所有测试通过
```

### ACP 模式的特殊能力

| 能力 | 说明 |
|------|------|
| 会话管理 | IDE 自动管理会话的创建、加载、继续 |
| 权限请求 | 高风险操作时 IDE 弹窗确认 |
| MCP 集成 | IDE 可传入 MCP 服务器配置 |
| 多 Agent | 通过 `--multi-agent` 启用子 Agent 工具 |

---

## 场景 5：A2A 独立服务器模式

将 VibeCoding 作为 A2A 服务器运行，供其他 Agent 调用。

### 场景 A：启动独立 A2A 服务器

```bash
# 初始化配置
vibecoding a2a --init-a2a-config

# 编辑 a2a.json（可选）
vim ~/.vibecoding/a2a.json

# 启动服务器
vibecoding a2a start --port 8093 --work-dir ~/projects/myapp
```

### 场景 B：其他 Agent 调用

```bash
# 用 vibecoding 客户端
vibecoding a2a send "列出项目中的所有 Go 文件" --target http://localhost:8093

# 用 curl
curl -X POST http://localhost:8093/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "运行所有测试"}]
      }
    },
    "id": 1
  }'

# 发现远程 Agent 能力
vibecoding a2a discover http://localhost:8093
```

### 场景 C：带认证的 A2A 服务器

```bash
# 启动带 Token 认证的服务器
vibecoding a2a start --auth-token "my-secret-token-xxx"

# 客户端调用时传 Token
vibecoding a2a send "review main.go" \
  --target http://remote-server:8093 \
  --auth-token "my-secret-token-xxx"
```

---

## 场景 6：A2A Master 模式（跨机器 Agent 调度）

管理多个远程 A2A Agent，让 LLM 自动分发任务。

### 架构

```
┌─────────────────────────────────────────────────────────┐
│  本机 (VibeCoding + A2A Master)                          │
│                                                         │
│  vibecoding --enable-a2a-master                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │  LLM 自动决策 → a2a_dispatch tool                │   │
│  └─────────────────────────────────────────────────┘   │
│           │                   │                         │
│           ▼                   ▼                         │
│  ┌──────────────┐   ┌──────────────┐                   │
│  │ code-reviewer│   │  ci-agent    │                   │
│  │ 192.168.1.10 │   │ 192.168.1.20 │                   │
│  │ :8093        │   │ :8093        │                   │
│  └──────────────┘   └──────────────┘                   │
└─────────────────────────────────────────────────────────┘
```

### 步骤 1：在远程机器上启动 A2A 服务器

**机器 A（代码审查 Agent）：**
```bash
# 192.168.1.10
vibecoding a2a start --port 8093 --work-dir ~/projects/shared
```

**机器 B（CI Agent）：**
```bash
# 192.168.1.20
vibecoding a2a start --port 8093 --work-dir ~/ci-runner --auth-token "ci-secret"
```

### 步骤 2：本机初始化 Master 配置

```bash
# 生成示例配置
vibecoding --init-a2a-master-config

# 编辑 a2a-list.json
vim ~/.vibecoding/a2a-list.json
```

```json
{
  "agents": [
    {
      "name": "code-reviewer",
      "url": "http://192.168.1.10:8093"
    },
    {
      "name": "ci-agent",
      "url": "http://192.168.1.20:8093",
      "auth_token": "ci-secret"
    }
  ]
}
```

### 步骤 3：启用 Master 模式

```bash
$ vibecoding --enable-a2a-master --verbose
```

```
A2A master mode enabled: 2 agents loaded from /home/user/.vibecoding/a2a-list.json

> 帮我审查 internal/handler.go 的代码质量，然后运行测试确保没问题

[Agent]
  我需要将任务分发给两个远程 Agent：

  🔧 a2a_dispatch(agent_name="code-reviewer",
                   message="审查 internal/handler.go 的代码质量，关注错误处理、
                           性能和安全性")

  → code-reviewer 返回: "发现 3 个问题：1) 第 45 行未处理超时..."

  🔧 a2a_dispatch(agent_name="ci-agent",
                   message="运行项目的完整测试套件，报告结果")

  → ci-agent 返回: "47/47 测试通过，覆盖率 82%"

✅ 综合结果：
- 代码审查发现 3 个问题（已列出详情）
- 所有测试通过，覆盖率 82%
- 建议优先修复第 45 行的超时处理
```

### 实际对话示例

```
> 分析项目结构，然后让审查 agent 看看有没有架构问题

[Agent]
  📄 ls(".")
  📄 read("go.mod")
  📄 find("*.go", maxDepth=2)

  → 了解了项目结构

  🔧 a2a_dispatch(agent_name="code-reviewer",
                   message="这个 Go 项目的结构如下：[项目结构摘要]。
                           请从架构角度分析是否有改进空间，
                           特别关注包的职责划分和依赖关系。")

  → code-reviewer: "建议：1) internal/api 和 internal/handler 存在职责重叠..."

✅ 以下是架构改进建议...
```

---

## 场景 7：Gateway 模式（HTTP API）

将 VibeCoding 作为 OpenAI 兼容的 HTTP 服务，供其他应用调用。

### 初始化和启动

```bash
# 生成配置模板
vibecoding --init-gateway

# 编辑 gateway.json（设置 token、端口等）
vim ~/.vibecoding/gateway.json

# 启动网关
vibecoding gateway --port 8080 --work-dir ~/projects/myapp
```

### 调用

```bash
# 用 curl（OpenAI 兼容格式）
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [
      {"role": "user", "content": "解释这个项目的架构"}
    ]
  }'

# 用 Python OpenAI SDK
from openai import OpenAI
client = OpenAI(base_url="http://localhost:8080/v1", api_key="your-token")
response = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "写一个 HTTP 中间件"}]
)
```

---

## 场景 8：Hermes 消息平台网关

将 VibeCoding 接入微信/飞书，实现无人值守的 AI 编码助手。

### 启动

```bash
# 配置 hermes.json
vim ~/.vibecoding/hermes.json

# 启动
vibecoding hermes start
```

### 典型配置

```json
{
  "server": { "port": 8090, "auth_token": "my-token" },
  "platforms": {
    "wechat": { "enabled": true },
    "feishu": { "enabled": true, "app_id": "...", "app_secret": "..." }
  },
  "default_mode": "yolo",
  "security": {
    "smart_approvals": true,
    "allowed_work_dirs": ["/srv/projects"]
  },
  "a2a": { "enabled": true },
  "cron": { "enabled": true },
  "memory": { "enabled": true }
}
```

### 在消息平台中使用

```
用户: /new
Bot:   新会话已创建

用户: 帮我给 api 包添加速率限制中间件
Bot:   [执行中...]
       ✅ 已添加速率限制中间件，支持可配置的请求/秒限制
       修改文件：internal/api/middleware.go, internal/api/routes.go

用户: 运行测试
Bot:   [执行 go test ./...]
       ✅ 全部通过 (12/12)
```

---

## 场景 9：组合模式（多工具协同）

将多种模式组合使用，构建完整的开发工作流。

### 示例：开发 + 审查 + 部署

```bash
# 1. 本地开发（TUI 模式）
cd ~/projects/myapp
vibecoding --mode yolo

# 2. 提交前审查（Plan 模式）
vibecoding --mode plan "审查 git diff 中的所有变更"

# 3. 推送后 CI 自动审查（Gateway 模式）
# CI 脚本中：
curl http://gateway:8080/v1/chat/completions \
  -d '{"messages": [{"role": "user", "content": "审查 PR #42 的代码"}]}'

# 4. 定时巡检（Hermes + Cron）
vibecoding hermes cron add "security-scan" \
  "扫描项目中的安全漏洞和敏感信息泄露" \
  --schedule "@weekly"
```

---

## 常用命令速查

| 场景 | 命令 |
|------|------|
| 日常编码 | `vibecoding` |
| 只读分析 | `vibecoding --mode plan` |
| 完全访问 | `vibecoding --mode yolo` |
| 非交互 | `vibecoding -P "..."` |
| 多 Agent | `vibecoding --multi-agent` |
| A2A 服务器 | `vibecoding a2a start` |
| A2A Master | `vibecoding --enable-a2a-master` |
| HTTP 网关 | `vibecoding gateway` |
| 消息平台 | `vibecoding hermes start` |
| IDE 集成 | `vibecoding acp` |
| 继续会话 | `vibecoding -c` |
| 恢复会话 | `vibecoding -r <id>` |
| 生成配置 | `vibecoding --init-gateway` |
| 生成 A2A 配置 | `vibecoding a2a --init-a2a-config` |
| 生成 Master 配置 | `vibecoding --init-a2a-master-config` |
