# 快速入门

本指南帮助你在 5 分钟内开始使用 VibeCoding。

## 系统要求

- **操作系统**: Linux, macOS, Windows (WSL)
- **Go**: 1.24+ (从源码构建时)
- **可选**: bubblewrap (用于沙箱功能)

## 安装

### 方法一: npm 安装 (推荐)

```bash
npm install -g vibecoding-installer
```

这将自动下载适合你平台的二进制文件。

### 方法二: 一键安装

**Linux/macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.ps1 | iex
```

或者指定安装目录:

```bash
# Linux/macOS
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash

# Windows
$env:VIBECODING_INSTALL_DIR="C:\Tools\vibecoding"; irm https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.ps1 | iex
```

这将自动从 GitHub 下载最新版本并安装二进制文件。默认安装位置：
- Linux/macOS: `/usr/local/bin`
- Windows: `%LOCALAPPDATA%\vibecoding`

### 方法三: Go Install

```bash
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest
```

### 方法四: 从源码构建

```bash
# 克隆仓库
git clone https://github.com/startvibecoding/vibecoding.git
cd vibecoding

# 构建
make build

# 二进制文件位于 bin/vibecoding
```

### 方法五: 安装到系统

```bash
# 从源码构建后
make install
```

## 配置 API 密钥

### 方式一: 环境变量

```bash
# DeepSeek
export DEEPSEEK_API_KEY=sk-...
```

### 方式二: 配置文件

或在 settings.json 中直接配置:

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

可选的 `vendor` 字段用于选择厂商适配器。未设置时，VibeCoding 会尽量根据 `baseUrl` 自动识别厂商，否则根据 `api` 回退到通用协议 provider。详见 [配置详解](configuration.md)。

## 首次运行

### 交互模式

```bash
# 启动交互式会话
vibecoding

# 或使用别名
vc
```

### 非交互模式

```bash
# 单次提问
vibecoding -p "解释这段代码的作用"

# 从 stdin 读取
echo "写一个 Hello World" | vibecoding -P
```

### 指定模型

```bash
# 使用 DeepSeek-V4-Flash
vibecoding --provider deepseek-openai --model deepseek-v4-flash

# 使用 DeepSeek-V4-Pro
vibecoding --provider deepseek-openai --model deepseek-v4-pro
```

### 多 Agent 模式

```bash
# 启用子 Agent 工具和多 Agent 命令
vibecoding --multi-agent

# ACP 会话也可以启用
vibecoding acp --multi-agent
```

多 Agent 模式会注册 `subagent_*` 工具，用于委托边界清晰的任务。TUI 多 Agent 工作流中也提供 cron 命令入口。

### A2A Master 模式

```bash
# 生成示例配置
vibecoding --init-a2a-master-config

# 启用 master 模式
vibecoding --enable-a2a-master
```

A2A Master 模式让你管理多个远程 A2A Agent，LLM 可自动通过 `a2a_dispatch` tool 分发任务。详见 [A2A 协议](a2a.md)。

## 选择模式

VibeCoding 提供三种模式:

```bash
# Plan 模式 - 只读分析
vibecoding --mode plan

# Agent 模式 - 标准读写 (默认)
vibecoding --mode agent

# YOLO 模式 - 完全访问
vibecoding --mode yolo
```

| 模式 | 文件系统 | 网络 | 用途 |
|------|---------|------|------|
| **Plan** | 只读 | ✗ | 分析、规划 |
| **Agent** | 读写 | ✗ | 日常开发 |
| **YOLO** | 完全 | ✓ | 系统级操作 |

## 基本交互

### 常用命令

```bash
/mode plan      # 切换到 Plan 模式
/mode agent     # 切换到 Agent 模式
/model          # 查看当前模型
/think          # 切换思考级别
/clear          # 清空对话
/help           # 显示帮助
/quit           # 退出
```

### 键盘快捷键

| 快捷键 | 功能 |
|--------|------|
| `Ctrl+C` | 中断 / 清空输入 |
| `Ctrl+D` | 退出 |
| `Tab` | 切换思考级别 |
| `Ctrl+T` | 切换思考显示 |

## 使用示例

### 代码解释

```bash
vibecoding "解释 main.go 的作用"
```

### 代码生成

```bash
vibecoding "写一个 Go HTTP 服务器"
```

### 文件操作

```bash
vibecoding "在当前目录创建一个 README.md"
```

### 继续会话

```bash
# 继续最近的会话
vibecoding --continue

# 恢复特定会话
vibecoding --resume <session-id>
```

## 技能系统

技能是可复用的提示片段，帮助强制执行项目约定：

```bash
# 列出可用技能
> /skills

# 激活技能
> /skill my-conventions
```

创建技能的方式是添加 `SKILL.md` 文件：
- **全局**: `~/.vibecoding/skills/<name>/SKILL.md`（所有项目可用）
- **项目**: `.skills/<name>/SKILL.md`（项目特定，覆盖全局）

详见 [技能系统](skills.md) 文档。

## IDE 集成

VibeCoding 可以通过 Agent Client Protocol (ACP) 集成到你的 IDE：

### VS Code

在 `settings.json` 中添加：
```json
{
  "acp.agents": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp", "--mode", "agent", "--multi-agent"]
    }
  }
}
```

### JetBrains IDE

导航到 `Settings → Tools → ACP Agents` 并添加：
- **Name**: VibeCoding
- **Command**: `vibecoding`
- **Arguments**: `acp --mode agent`

详见 [ACP 协议](acp.md) 文档。

## 下一步

- 阅读 [配置详解](configuration.md) 自定义设置
- 查看 [工具参考](tools.md) 了解可用工具
- 尝试 [多 Agent 模式](cli-reference.md#多-agent-模式) 进行委托调查和 cron 命令入口
- 了解 [安全模型](security.md) 保护你的系统
- 探索 [技能系统](skills.md) 创建可复用提示片段
- 设置 [IDE 集成](acp.md) 在 VS Code 或 JetBrains 中使用
- 查看 [场景演示](scenarios.md) 了解各模式的实际用法
