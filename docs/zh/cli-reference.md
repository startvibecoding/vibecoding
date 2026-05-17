# 命令行参考

## 概述

```
vibecoding [flags] [message...]
```

别名: `vc`

## 命令行参数

### 基本参数

| 参数 | 简写 | 默认值 | 描述 |
|------|------|--------|------|
| `--provider` | `-p` | 配置文件中的默认值 | LLM 提供商 (deepseek-openai, deepseek-anthropic 或自定义名称) |
| `--model` | `-m` | 配置文件中的默认值 | 模型 ID |
| `--mode` | `-M` | `agent` | 运行模式 (plan, agent, yolo) |
| `--thinking` | `-t` | `off` | 思考级别 (off, minimal, low, medium, high, xhigh) |

### 会话管理

| 参数 | 简写 | 描述 |
|------|------|------|
| `--continue` | `-c` | 继续最近的会话 |
| `--resume` | `-r` | 通过 ID 或路径恢复会话 |
| `--session` | - | 使用特定的会话文件 |

### 输出控制

| 参数 | 简写 | 描述 |
|------|------|------|
| `--print` | `-P` | 非交互模式，打印响应后退出 |
| `--verbose` | - | 详细输出 |
| `--debug` | - | 启用调试日志 |

### 安全

| 参数 | 描述 |
|------|------|
| `--sandbox` | 启用沙箱 (bubblewrap) |
| `--no-sandbox` | 禁用沙箱 (已弃用，默认不启用) |

### 其他

| 参数 | 简写 | 描述 |
|------|------|------|
| `--version` | `-v` | 显示版本 |
| `--help` | `-h` | 显示帮助 |

## 使用示例

### 基本使用

```bash
# 交互模式
vibecoding

# 带初始提示
vibecoding "解释这个代码库"

# 非交互模式
vibecoding -p "写一个 Hello World"
```

### 指定提供商和模型

```bash
# 使用 DeepSeek (OpenAI API)
vibecoding --provider deepseek-openai --model deepseek-v4-flash

# 使用 DeepSeek (Anthropic API)
vibecoding -p deepseek-anthropic -m deepseek-v4-flash

# 使用自定义提供商
vibecoding --provider my-custom-provider
```

### 选择模式

```bash
# Plan 模式 - 只读分析
vibecoding --mode plan

# Agent 模式 - 标准读写 (默认)
vibecoding -M agent

# YOLO 模式 - 完全访问
vibecoding -M yolo
```

### 思考级别

```bash
# 关闭思考
vibecoding --thinking off

# 中等级别
vibecoding -t medium

# 最高级别
vibecoding --thinking xhigh
```

### 会话管理

```bash
# 继续最近的会话
vibecoding --continue
vibecoding -c

# 恢复特定会话
vibecoding --resume session-abc123
vibecoding -r ~/.vibecoding/sessions/my-session.jsonl

# 使用特定会话文件
vibecoding --session ./my-session.jsonl
```

### 沙箱

```bash
# 启用沙箱
vibecoding --sandbox

# 禁用沙箱 (默认)
vibecoding
```

### 管道输入

```bash
# 从 stdin 读取
echo "解释这段代码" | vibecoding -P

# 从文件读取
cat main.go | vibecoding -p "解释这个文件"
```

## 交互式命令

在交互会话中可用的命令:

| 命令 | 描述 |
|------|------|
| `/mode [plan\|agent\|yolo]` | 切换模式 |
| `/model` | 显示当前模型 |
| `/think` | 循环切换思考级别 |
| `/skills` | 列出已加载的技能 |
| `/clear` | 清空对话 |
| `/help` | 显示帮助 |
| `/quit` | 退出 |

## 键盘快捷键

| 快捷键 | 功能 |
|--------|------|
| `Ctrl+C` | 中断当前操作 / 清空输入 |
| `Ctrl+D` | 退出 |
| `Tab` | 循环切换思考级别 |
| `Ctrl+T` | 切换思考内容显示 |

## 环境变量

可以通过环境变量覆盖默认设置:

| 变量 | 描述 |
|------|------|
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `VIBECODING_DIR` | 覆盖配置目录 |
| `VIBECODING_PROVIDER` | 覆盖默认提供商 |
| `VIBECODING_MODEL` | 覆盖默认模型 |
| `VIBECODING_MODE` | 覆盖默认模式 |
| `VIBECODING_THINKING` | 覆盖默认思考级别 |
| `VIBECODING_USER_AGENT` | 自定义 User-Agent 字符串 |

## 退出码

| 码 | 描述 |
|----|------|
| 0 | 成功 |
| 1 | 一般错误 |
| 2 | 用法错误 |
| 130 | 用户中断 (Ctrl+C) |
