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

### 方法二: Go Install

```bash
go install github.com/fuckvibecoding/vibecoding/cmd/vibecoding@latest
```

### 方法三: 从源码构建

```bash
# 克隆仓库
git clone https://github.com/fuckvibecoding/vibecoding.git
cd vibecoding

# 构建
make build

# 二进制文件位于 bin/vibecoding
```

### 方法四: 安装到系统

```bash
# 从源码构建后
make install
```

## 配置 API 密钥

### 方式一: 环境变量

```bash
# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...

# OpenAI
export OPENAI_API_KEY=sk-...
```

### 方式二: 认证文件

创建 `~/.vibecoding/auth.json`:

```json
{
  "anthropic": {
    "type": "api_key",
    "key": "sk-ant-..."
  },
  "openai": {
    "type": "api_key",
    "key": "sk-..."
  }
}
```

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
# 使用 Claude
vibecoding --provider anthropic --model claude-sonnet-4-20250514

# 使用 GPT-4o
vibecoding --provider openai --model gpt-4o
```

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

## 下一步

- 阅读 [配置详解](configuration.md) 自定义设置
- 查看 [工具参考](tools.md) 了解可用工具
- 了解 [安全模型](security.md) 保护你的系统
