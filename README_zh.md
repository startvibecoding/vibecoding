<p align="center">
  <img src="docs/assets/logo.svg" alt="VibeCoding" width="128" height="128">
</p>

<h1 align="center">VibeCoding</h1>

<p align="center">
  一个基于终端的 AI 编码助手，使用约 10,000 行 Go 代码编写，灵感来源于 <a href="https://pi.dev">pi.dev</a>
</p>

<p align="center">
  主打渐进式、敏捷开发体验的 VibeCoding 工具，整体打包为单个文件，开箱即用，无需重复搭建部署 Claude Code 、 codex、Claw、Hermes 环境。
</p>

<p align="center">
  <a href="https://www.npmjs.com/package/vibecoding-installer"><img src="https://img.shields.io/npm/dm/vibecoding-installer.svg" alt="npm downloads"></a>
  <a href="https://github.com/startvibecoding/vibecoding/releases/latest"><img src="https://img.shields.io/github/release/startvibecoding/vibecoding.svg" alt="GitHub release"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://goreportcard.com/report/github.com/startvibecoding/vibecoding"><img src="https://goreportcard.com/badge/github.com/startvibecoding/vibecoding" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/startvibecoding/vibecoding"><img src="https://pkg.go.dev/badge/github.com/startvibecoding/vibecoding?status.svg" alt="GoDoc"></a>
  <a href="https://github.com/startvibecoding/vibecoding/network/dependencies"><img src="https://img.shields.io/librariesio/release/github/startvibecoding/vibecoding" alt="Dependencies"></a>
</p>

## 功能特性

- **多提供商支持**：DeepSeek（默认）、OpenAI、Anthropic，以及面向 OpenAI/Anthropic 格式兼容 API 的厂商适配器
- **SSE 流式传输**：实时令牌流式传输，快速响应
- **思考模式**：扩展思考/推理支持（DeepSeek 推理）
- **多 Agent 工作流**：可选 `--multi-agent` 模式，支持委托子 Agent 和 cron 命令入口
- **A2A Master 模式**：可选 `--enable-a2a-master` 模式，通过 `a2a-list.json` 管理多个远程 A2A Agent，注册 `a2a_dispatch` tool 自动分发任务
- **三种模式**：
  - 🗒️ **计划** — 只读分析和规划。沙箱化，无文件写入
  - 🔧 **代理**（默认）— 对项目的受控读写访问。Bash 需要批准（可配置白名单）。沙箱化，无网络
  - 🚀 **YOLO** — 完全系统访问，无限制
- **bwrap 沙箱**：通过 [bubblewrap](https://github.com/containers/bubblewrap) 实现 Linux 沙箱化安全执行
- **会话管理**：基于 JSONL 的会话文件，支持树形结构、分支和压缩
- **上下文管理**：自动上下文窗口管理和令牌估算
- **丰富终端界面**：使用 BubbleTea 构建的终端界面，支持 Markdown 渲染和代码高亮
- **缓存命中率**：页脚实时显示缓存命中百分比，每轮缓存统计
- **ACP 支持**：可作为代理客户端协议（ACP）标准输入输出代理运行，用于编辑器集成和兼容客户端，包括 VS Code、Zed 和 JetBrains IDE（如 IntelliJ IDEA/WebStorm），通过 ACP 兼容插件
- **更安全的审批处理**：`bashBlacklist` 现在优先于白名单生效，包括在 YOLO 模式下；当 `--print` 遇到需要审批的命令时会直接失败退出
- **统一的缓存指标**：TUI 与 print 模式现在使用一致的缓存感知 token 统计与缓存命中率语义
- **更一致的 Provider 调试**：`--debug` 现在会稳定启用 provider 级请求/响应调试输出，ACP 模式同样适用

## 快速开始

### 安装

**选项 1：npm（推荐）**

```bash
npm install -g vibecoding-installer
```

**选项 2：一键安装**

Linux/macOS：

```bash
curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash
```

Windows（PowerShell）：

```powershell
irm https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.ps1 | iex
```

或自定义安装目录：

```bash
# Linux/macOS
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash

# Windows
$env:VIBECODING_INSTALL_DIR="C:\Tools\vibecoding"; irm https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.ps1 | iex
```

**选项 3：Go 安装**

```bash
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest
```

**选项 4：从源码构建**

```bash
git clone https://github.com/startvibecoding/vibecoding.git
cd vibecoding
make build
```

### 交叉编译

```bash
make build-all    # 为 linux/amd64、darwin/amd64、darwin/arm64、windows/amd64 构建
```

### 配置

设置 API 密钥：

```bash
# DeepSeek
export DEEPSEEK_API_KEY=sk-...
```

或直接在 `settings.json` 中配置：

```json
{
  "providers": {
    "deepseek-openai": {
      "vendor": "deepseek",
      "api": "openai-chat",
      "baseUrl": "https://api.deepseek.com",
      "apiKey": "sk-..."
    }
  }
}
```

### 运行

```bash
# 交互模式
vibecoding

# 带初始提示
vibecoding "解释这个代码库"

# 非交互模式（打印模式）
vibecoding -p "用 Go 写一个 hello world"

# 指定提供商和模型
vibecoding --provider deepseek-openai --model deepseek-v4-flash

# 启用子 Agent 工具和多 Agent 命令
vibecoding --multi-agent

# 更改模式
vibecoding --mode plan    # 只读规划
vibecoding --mode agent   # 标准模式（默认）
vibecoding --mode yolo    # 完全访问

# 继续最近会话
vibecoding -c

# 禁用沙箱
vibecoding --no-sandbox
```

## 配置

### 配置文件

| 位置 | 平台 | 范围 |
|------|------|------|
| `~/.vibecoding/settings.json` | Linux/macOS | 全局（所有项目） |
| `%APPDATA%\vibecoding\settings.json` | Windows | 全局（所有项目） |
| `.vibe/settings.json` | 所有 | 项目（覆盖全局） |

> **Windows 用户：** `%APPDATA%` 解析为 `C:\Users\<用户名>\AppData\Roaming`。

### 示例配置

```json
{
  "defaultProvider": "deepseek-openai",
  "defaultModel": "deepseek-v4-flash",
  "defaultThinkingLevel": "medium",
  "defaultMode": "agent",
  "enablePlanTool": true,
  "maxContextTokens": 1000000,
  "maxOutputTokens": 384000,
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
    "enabled": true
  },
  "retry": {
    "enabled": true,
    "maxRetries": 3,
    "baseDelayMs": 2000
  },
  "approval": {
    "bashWhitelist": ["go ", "make ", "git ", "npm ", "yarn "],
    "bashBlacklist": ["rm -rf", "sudo"]
  }
}
```

### 环境变量

| 变量 | 描述 |
|------|------|
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `VIBECODING_DIR` | 覆盖配置目录 |
| `VIBECODING_PROVIDER` | 覆盖默认提供商 |
| `VIBECODING_MODEL` | 覆盖默认模型 |
| `VIBECODING_MODE` | 覆盖默认模式 |
| `VIBECODING_THINKING` | 覆盖默认思考级别 |
| `VIBECODING_USER_AGENT` | 自定义用户代理字符串 |
| `VIBECODING_DEBUG` | 启用 provider 级请求/响应调试输出 |

## 沙箱安全

VibeCoding 使用 [bubblewrap](https://github.com/containers/bubblewrap) 实现 Linux 沙箱化。

| 模式 | 文件系统 | 网络 | bwrap |
|------|----------|------|-------|
| **计划**（严格） | 项目只读 | ✗ | ✓ |
| **代理**（标准） | 项目读写 | ✗ | ✓ |
| **YOLO**（无） | 完全访问 | ✓ | ✗ |

### 安装 bwrap

```bash
# Debian/Ubuntu
sudo apt install bubblewrap

# Fedora
sudo dnf install bubblewrap

# Arch
sudo pacman -S bubblewrap
```

## CLI 参考

```
vibecoding [标志] [消息...]
别名：vc

标志：
  -p, --provider string    提供商 (deepseek-openai, deepseek-anthropic 或自定义提供商名称)
  -m, --model string       模型 ID
  -M, --mode string        模式 (plan, agent, yolo)
  -t, --thinking string    思考级别 (off, minimal, low, medium, high, xhigh)
      --multi-agent        启用多 Agent 工具和命令
      --enable-a2a-master   启用 A2A Master 模式（远程 agent 调度）
  -c, --continue           继续最近会话
  -r, --resume string      通过 ID 或路径恢复会话
      --session string     使用特定会话文件或 ID
      --sandbox            启用沙箱 (bwrap) 进行安全执行
  -P, --print              打印响应并退出（非交互式）
      --verbose            详细输出
      --debug              启用调试日志
  -v, --version            显示版本
  -h, --help               显示帮助
```

### 交互式命令

| 命令 | 描述 |
|------|------|
| `/mode [plan\|agent\|yolo]` | 切换模式 |
| `/model` | 显示当前模型 |
| `/think` | 循环思考级别 |
| `/skills` | 列出已加载技能 |
| `/clear` | 清除对话 |
| `/help` | 显示帮助 |
| `/quit` | 退出 |

### 键盘快捷键

| 按键 | 操作 |
|------|------|
| `Ctrl+C` | 中止/清除输入 |
| `Ctrl+D` | 退出 |
| `Tab` | 循环思考级别 |
| `Ctrl+T` | 切换思考显示 |

## 开发

```bash
make build      # 构建二进制文件
make test       # 运行测试
make lint       # 运行代码检查
make fmt        # 格式化代码
make clean      # 清理构建产物
make build-all  # 为所有平台交叉编译
make dist       # 构建分发包 (.deb, .tar.gz)
```

## 架构

```
vibecoding/
├── cmd/vibecoding/        # CLI 入口点
├── internal/
│   ├── a2a/               # A2A 协议服务器与 Master 模式
│   ├── acp/               # ACP / MCP 集成
│   ├── agent/             # 核心 Agent 循环
│   ├── config/            # 配置系统
│   ├── context/           # 上下文管理和令牌估算
│   ├── contextfiles/      # 上下文文件发现 (AGENTS.md, CLAUDE.md 等)
│   ├── cron/              # 多 Agent 工作流的定时任务
│   ├── gateway/           # OpenAI 兼容 HTTP 网关
│   ├── hermes/            # 消息平台网关 (微信/飞书/WebSocket)
│   ├── mcp/               # MCP 服务器集成
│   ├── memory/            # 持久化记忆 (memory.md)
│   ├── messaging/         # 消息平台抽象
│   ├── platform/          # 跨平台兼容性工具
│   ├── provider/          # LLM 提供商抽象
│   │   ├── factory/       # 共享 provider/model 创建逻辑
│   │   ├── openai/        # OpenAI Chat Completions API
│   │   ├── anthropic/     # Anthropic Messages API
│   │   └── vendor*.go     # 厂商适配注册和默认值
│   ├── sandbox/           # 沙箱 (bwrap) 实现
│   ├── session/           # 会话管理 (JSONL)
│   ├── skills/            # 技能系统
│   ├── tools/             # 工具实现
│   ├── tui/               # 终端界面 (BubbleTea)
│   ├── ua/                # 用户代理字符串生成
│   └── vendored/          # 内嵌二进制 (rg, fd)
└── pkg/sdk/               # 公共 SDK 接口
```

### 运行模式

```
vibecoding                    # 交互式终端 (TUI)
vibecoding -p "..."           # 非交互打印模式
vibecoding acp                # ACP stdio 代理 (编辑器集成)
vibecoding gateway            # OpenAI 兼容 HTTP 网关
vibecoding hermes             # 消息平台网关 (微信/飞书/WebSocket)
vibecoding a2a start          # A2A 协议服务器 (独立模式)
vibecoding --enable-a2a-master  # A2A Master 模式 (远程 agent 调度)
```

## 许可证

MIT
