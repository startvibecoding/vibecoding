# VibeCoding 文档

<p align="center">
  <img src="assets/logo.svg" alt="VibeCoding" width="128" height="128">
</p>

<p align="center">
  <strong>AI 驱动的终端编码助手</strong>
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

---

欢迎来到 VibeCoding 文档中心！

## 什么是 VibeCoding?

VibeCoding 是一个基于终端的 AI 编码助手，帮助你编写、调试、重构和理解代码。它支持多种 LLM 提供商，包括 DeepSeek（默认）、OpenAI、Anthropic，以及通过厂商适配器接入的 OpenAI/Anthropic 兼容 API。

### 核心特性

- 🤖 **多提供商支持** — DeepSeek、OpenAI、Anthropic、厂商适配器及自定义提供商
- 🔧 **内置工具** — 文件操作、代码搜索、命令执行、任务计划和可选子 Agent 工具
- 🧭 **多 Agent 工作流** — `--multi-agent` 模式支持委托子 Agent 和 cron 命令入口
- 🛡️ **沙箱安全** — 通过 bubblewrap 实现进程级隔离
- 📝 **会话管理** — 持久化对话历史，支持分支
- 🎯 **3 种操作模式** — Plan（只读）、Agent（标准）、YOLO（完全访问）
- 🧩 **技能系统** — 可复用的提示片段，用于项目约定
- 💻 **IDE 集成** — ACP 协议支持 VS Code 和 JetBrains
- 🖼️ **图像支持** — 读取和分析图像文件
- ⚡ **提示缓存** — 通过缓存重复前缀降低 API 成本
- ✅ **更安全的审批处理** — `bashBlacklist` 优先于白名单，包括 YOLO 模式；`--print` 遇到需审批命令时直接失败
- 📊 **统一缓存指标** — TUI 与 print 模式使用一致的缓存命中率与 token 统计口径
- 🐞 **一致的调试输出** — `--debug` 会同时开启 provider 级调试，ACP 模式同样适用
- 🎨 **丰富 TUI** — Markdown 渲染、语法高亮、思考显示

## 目录

### 入门指南
- [快速入门](getting-started.md) — 安装、配置和首次运行
- [命令行参考](cli-reference.md) — 完整的 CLI 参数说明

### 配置
- [配置详解](configuration.md) — 设置文件、环境变量、认证

### 架构
- [系统架构](architecture.md) — 项目结构、核心组件、数据流
- [工具系统](tools.md) — 内置工具使用指南
- [技能系统](skills.md) — 可复用提示片段
- [在线Skill市场集成](skillhub.md) — 兼容 SkillHub / ClawHub，技能安装与 Cron 基础设施
- [会话管理](sessions.md) — 会话存储和管理
- [SDK 集成指南](sdk.md) — 将 VibeCoding Agent 嵌入你的 Go 应用

### 安全
- [安全与沙箱](security.md) — 沙箱模式、权限控制、审批机制

### IDE 集成
- [ACP 协议](acp.md) — Agent Client Protocol 支持 VS Code 和 JetBrains

### 网关模式
- [Gateway 模式](gateway.md) — OpenAI 兼容 HTTP 网关
- [Hermes 模式](hermes.md) — 消息平台网关 (微信/飞书/WebSocket)
- [A2A 协议](a2a.md) — Agent-to-Agent 协议服务器与 Master 模式

### 场景演示
- [场景演示](scenarios.md) — 各种模式的实际用法和工作流

### 开发
- [开发指南](development.md) — 贡献代码、测试、构建

### 参考
- [FAQ](faq.md) — 常见问题解答
- [更新日志](changelog.md) — 版本历史和发布说明

## 快速链接

| 主题 | 描述 |
|------|------|
| [快速入门](getting-started.md) | 5 分钟上手 VibeCoding |
| [配置文件](configuration.md) | 自定义提供商、模型和行为 |
| [工具参考](tools.md) | 了解内置工具和可选多 Agent 工具 |
| [安全模型](security.md) | 理解沙箱、模式和权限 |
| [ACP 协议](acp.md) | 通过 Agent Client Protocol 集成 IDE |
| [会话管理](sessions.md) | 对话历史和分支 |
| [技能系统](skills.md) | 创建可复用提示片段 |
| [在线Skill市场集成](skillhub.md) | 兼容 SkillHub / ClawHub，技能安装与 Cron 基础设施 |
| [SDK 集成指南](sdk.md) | 将 VibeCoding Agent 嵌入你的 Go 应用 |
| [场景演示](scenarios.md) | 各种模式的实际用法和工作流 |
| [更新日志](changelog.md) | 查看每个版本的新内容 |

## 支持的 LLM

| 提供商 | 模型 | API 格式 |
|--------|------|----------|
| **DeepSeek**（默认） | deepseek-v4-flash, deepseek-v4-pro | OpenAI Chat / Anthropic Messages |
| **OpenAI** | GPT-4o, o1 等 | OpenAI Chat |
| **Anthropic** | Claude Sonnet, Opus 等 | Anthropic Messages |
| **厂商适配器** | 小米、Kimi、MiniMax、Seed、Qianfan、Bailian、Gitee、OpenRouter、Together、Groq、Fireworks 等 | OpenAI Chat 或 Anthropic Messages |
| **自定义** | 任何兼容模型 | 通用 OpenAI Chat 或 Anthropic Messages fallback |

## 快速安装

```bash
# npm（推荐）
npm install -g vibecoding-installer

# 一键安装（Linux/macOS）
curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash

# Go install
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest
```

## 获取帮助

- 使用 `/help` 命令查看交互式帮助
- 查看 [CLI 参考](cli-reference.md) 了解所有命令
- 阅读 [FAQ](faq.md) 获取常见问题解答
- 访问 [GitHub Issues](https://github.com/startvibecoding/vibecoding/issues) 报告 Bug
