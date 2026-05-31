# Agent Client Protocol (ACP)

## 概述

VibeCoding 实现了 **Agent Client Protocol (ACP)**，一个基于 JSON-RPC 的协议，允许 AI 编码代理直接集成到 IDE 和编辑器中。ACP 采用客户端-服务器架构，你的 IDE 作为客户端，VibeCoding 作为后台代理运行。

## 支持的 IDE

### Visual Studio Code (VS Code)

VibeCoding 可以通过兼容的扩展作为 ACP 代理在 VS Code 中使用。在你的 VS Code 设置中添加以下配置：

```json
{
  "your-acp-extension.servers": {
    "vibecoding": {
      "type": "agent",
      "command": "vibecoding",
      "args": ["acp"],
      "env": {}
    }
  }
}
```

确保 `vibecoding` 在你的 `PATH` 环境变量中，或使用二进制文件的完整路径。

### JetBrains IDE (IntelliJ IDEA, GoLand, WebStorm 等)

JetBrains IDE 也支持 ACP 代理。在你的 IDE 设置中配置：

```json
{
  "acpAgentServers": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp"]
    }
  }
}
```

## 使用 ACP 模式

### 启动 ACP 服务器

```bash
# 基础 ACP 服务器 (stdin/stdout)
vibecoding acp

# 指定提供商和模型
vibecoding acp --provider deepseek-openai --model deepseek-v4-flash

# 启用沙箱
vibecoding acp --sandbox

# 指定模式
vibecoding acp --mode agent

# 启用多 Agent 工具
vibecoding acp --multi-agent
```

### ACP 命令行参数

| 参数 | 简写 | 默认值 | 描述 |
|------|------|--------|------|
| `--provider` | `-p` | 配置文件默认值 | LLM 提供商 |
| `--model` | `-m` | 配置文件默认值 | 模型 ID |
| `--mode` | `-M` | `agent` | 运行模式 (plan, agent, yolo) |
| `--thinking` | `-t` | 配置文件默认值 | 思考级别 |
| `--sandbox` | - | false | 启用沙箱 |
| `--verbose` | - | false | 详细输出 |
| `--debug` | - | false | 调试日志 |
| `--multi-agent` | - | false | 启用子 Agent 工具和多 Agent 工作流 |

## 协议细节

ACP 使用 JSON-RPC 2.0 通过 stdio 进行通信。协议支持以下方法：

### 方法

| 方法 | 描述 |
|------|------|
| `initialize` | 握手和能力协商 |
| `session/new` | 创建新会话 |
| `session/load` | 加载已有会话 |
| `session/prompt` | 向代理发送提示 |
| `session/cancel` | 取消活动的提示 |
| `session/update` | 服务器通知：会话状态变更 |

### 能力

VibeCoding 在初始化时声明以下 ACP 能力：

- **加载会话**: 加载和继续之前的会话
- **提示能力**: 文本提示；ACP prompt 不声明图像/音频输入能力
- **会话能力**: 取消活动中的提示
- **MCP 能力**: 支持 stdio / http / sse 传输
- **多 Agent 工作流**: 使用 `--multi-agent` 启动 ACP 服务器后可用

### 通知

服务器发送 `session/update` 通知，包含以下事件类型：

| 更新类型 | 描述 |
|-----------|------|
| `agent_message_chunk` | 代理的文本增量 |
| `agent_thought_chunk` | 思考/推理增量 |
| `user_message_chunk` | 历史用户消息 |
| `tool_call` | 正在调用的工具 |
| `tool_call_update` | 工具状态更新 (pending/in_progress/completed/failed) |

## MCP 服务器集成

VibeCoding 支持在 ACP 会话期间连接 **MCP (Model Context Protocol)** 服务器。这让代理能够访问外部工具和数据源。

ACP 会话与普通 CLI/TUI 会话复用同一套 MCP 连接和工具注册运行时。区别是 ACP 客户端在创建/加载会话时传入 `mcpServers`，普通 CLI/TUI 会话则在进程启动时加载 `mcp.json`。

### 配置 MCP 服务器

MCP 服务器由 IDE 客户端配置，并在创建或加载会话时传递给 VibeCoding。配置格式：

```json
{
  "mcpServers": [
    {
      "name": "my-database",
      "type": "stdio",
      "command": "/absolute/path/to/mcp-server",
      "args": ["--port", "8080"],
      "env": [
        {"name": "DB_URL", "value": "postgres://localhost/mydb"}
      ]
    },
    {
      "name": "remote-tools",
      "type": "http",
      "url": "https://mcp.example.com",
      "headers": [
        {"name": "Authorization", "value": "Bearer ${TOKEN}"}
      ]
    },
    {
      "name": "legacy-sse",
      "type": "sse",
      "url": "https://legacy.example.com/sse",
      "messageUrl": "https://legacy.example.com/messages"
    }
  ]
}
```

### MCP 工具注册

当 MCP 服务器连接后，VibeCoding 自动发现并注册服务器暴露的所有工具。工具按照 `mcp_<server_name>_<tool_name>` 的命名约定注册，代理可以像使用内置工具一样使用它们。

注册发生在 agent 冻结当前会话的 system prompt 和工具定义之前。因此 MCP 服务器变更后，需要用更新后的 `mcpServers` payload 创建或加载新的 ACP 会话。

除 `tools/*` 外，VibeCoding 现在还会发现：

- `resources/*`：注册为 MCP 资源读取工具
- `prompts/*`：注册为 MCP Prompt 渲染工具

### MCP 传输支持

支持的传输类型：

- `stdio`：要求 `command` 为绝对路径
- `http`：通过 `url` 连接 streamable HTTP 端点
- `sse`：通过 `url` 连接 legacy SSE 流，并通过 `messageUrl` 发送请求

补充说明：

- 同一会话内 MCP 服务器 `name` 必须唯一
- `http` / `sse` 传输可通过 `headers` 传鉴权头
- `sampling/createMessage` 已桥接到当前 ACP provider/model，并返回 assistant 文本内容
- MCP progress/logging/cancel 通知会以结构化 ACP `tool_call_update` 事件透出

## 权限系统

在 ACP 模式下，代理可以请求用户授权执行工具。IDE 客户端接收 `session/request_permission` 通知，并可以返回允许/拒绝的决定。

```
客户端                                   服务器 (vibecoding acp)
  │                                           │
  │  ── session/request_permission ────────▶  │
  │      (tool_call_id, title, options)       │
  │                                           │
  │  ◀── JSON-RPC 响应 ────────────────────  │
  │      (outcome: allow-once / reject-once)  │
```

## 示例：VS Code 集成

### 步骤 1：安装 VibeCoding

```bash
npm install -g vibecoding-installer
# 或
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest
```

### 步骤 2：配置 VS Code

在你的 VS Code `settings.json` 中添加：

```json
{
  "acp.agents": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp", "--mode", "agent"],
      "description": "VibeCoding AI Assistant"
    }
  }
}
```

### 步骤 3：开始使用

在 VS Code 中打开你的项目，启动 ACP 代理，然后直接在编辑器中提问或请求代码更改。

## 示例：JetBrains IDE 集成

### 步骤 1：安装 VibeCoding

```bash
npm install -g vibecoding-installer
```

### 步骤 2：在 JetBrains IDE 中配置

进入 `设置 → 工具 → ACP Agents` 并添加新代理：

- **名称**: VibeCoding
- **命令**: `vibecoding`
- **参数**: `acp --mode agent`

或添加到 `.idea/workspace.xml`：

```xml
<component name="ACPSettings">
  <option name="agents">
    <list>
      <ACPAgentSetting>
        <option name="name" value="Vibecoding" />
        <option name="command" value="vibecoding" />
        <option name="args" value="acp --mode agent" />
      </ACPAgentSetting>
    </list>
  </option>
</component>
```

### 步骤 3：开始使用

使用 JetBrains IDE 中的 ACP 工具窗口与 VibeCoding 交互。
