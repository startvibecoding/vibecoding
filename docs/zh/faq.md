# 常见问题 (FAQ)

## 基本问题

### Q: VibeCoding 是什么?

A: VibeCoding 是一个终端 AI 编码助手，支持 DeepSeek（默认）、OpenAI、Anthropic、面向兼容 API 的厂商适配器，以及通过通用 OpenAI/Anthropic 格式接入的自定义端点，提供代码编写、调试、重构、多 Agent 委托工作流等功能。

### Q: 支持哪些 LLM?

A:
- DeepSeek (默认): deepseek-v4-flash, deepseek-v4-pro (1M 上下文，最多 384K 输出)
- OpenAI: GPT-4o, o1 等
- Anthropic: Claude Sonnet, Opus 等
- 厂商适配器: Google Gemini、Google Vertex、小米、Kimi、MiniMax、Seed、Qianfan、Bailian、Gitee、OpenRouter、Together、Groq、Fireworks 等
- 自定义: 任何 OpenAI Chat 或 Anthropic Messages 兼容 API 端点，会回退到通用 provider

### Q: 如何安装?

A:
```bash
# npm（推荐）
npm install -g vibecoding-installer

# 一键安装（Linux/macOS）
curl -fsSL https://raw.githubusercontent.com/startvibecoding/vibecoding/main/install.sh | bash

# Go install
go install github.com/startvibecoding/vibecoding/cmd/vibecoding@latest

# 或从源码
git clone https://github.com/startvibecoding/vibecoding.git
cd vibecoding
make build
```

## 配置问题

### Q: 配置文件在哪里?

A: 
- 全局:
  - Linux/macOS: `~/.vibecoding/settings.json`
  - Windows: `%APPDATA%\vibecoding\settings.json`
- 项目: `.vibe/settings.json`
### Q: 如何设置 API 密钥?

A: 两种方式:
1. 环境变量: `export DEEPSEEK_API_KEY=sk-...`
2. 配置文件: `settings.json` 中的 `providers.<name>.apiKey`

### Q: 如何使用自定义 API?

A: 在 `settings.json` 中配置:

```json
{
  "providers": {
    "deepseek-openai": {
      "vendor": "deepseek",
      "baseUrl": "https://api.deepseek.com",
      "api": "openai-chat",
      "apiKey": "sk-..."
    }
  },
  "defaultProvider": "deepseek-openai"
}
```

## 使用问题

### Q: 如何切换模式?

A:
```bash
# 命令行
vibecoding --mode plan
vibecoding -M agent

# 交互式
/mode plan
/mode agent
/mode yolo
```

### Q: 如何切换模型?

A:
```bash
# 命令行
vibecoding --provider deepseek-openai --model deepseek-v4-pro

# 交互式
/model deepseek-v4-pro
/model                  # 显示当前模型和可用选项
```

### Q: 什么是思考级别?

A: 思考级别控制模型在回答前进行多少推理：
- `off`: 无思考（默认）
- `minimal`: 最少推理
- `low`: 轻度推理
- `medium`: 平衡推理
- `high`: 深度推理
- `xhigh`: 最大推理

```bash
# 命令行
vibecoding --thinking medium

# 交互式
/think           # 循环切换级别
Tab              # 键盘快捷键循环切换
```

### Q: 如何继续上次的会话?

A:
```bash
vibecoding --continue
vibecoding -c
```

### Q: 如何管理会话?

A: 在交互模式下使用 `/sessions` 命令：
```
/sessions           # 列出当前项目的会话
/sessions ls        # 列出所有项目的会话
/sessions set abc   # 切换到以 'abc' 开头的会话
/sessions clear     # 创建新的空白会话
/sessions del abc   # 删除以 'abc' 开头的会话
```

### Q: 如何使用技能?

A: 技能是可复用的提示片段。在交互模式下使用：
```
/skills             # 列出可用技能
/skill my-skill     # 激活技能
/skill:my-skill     # 替代语法
```

创建技能的方式是添加 `SKILL.md` 文件：
- 全局: `~/.vibecoding/skills/<name>/SKILL.md`
- 项目: `.skills/<name>/SKILL.md`

详见 [技能系统](skills.md) 文档。

### Q: 如何查看当前模型?

A:
```bash
# 交互式
/model

# 命令行
vibecoding --version
```

### Q: 如何清空对话?

A:
```bash
/clear
```

## IDE 集成问题

### Q: 可以在 IDE 中使用 VibeCoding 吗?

A: 可以！VibeCoding 支持 Agent Client Protocol (ACP) 用于 IDE 集成。支持的 IDE：
- Visual Studio Code
- JetBrains IDE（IntelliJ IDEA、GoLand、WebStorm 等）

详见 [ACP 协议](acp.md) 文档了解配置说明。

### Q: 如何设置 VS Code 集成?

A: 在 VS Code 的 `settings.json` 中添加：
```json
{
  "acp.agents": {
    "vibecoding": {
      "command": "vibecoding",
      "args": ["acp", "--mode", "agent"]
    }
  }
}
```

详见 [ACP 协议](acp.md) 文档了解详细说明。

## 沙箱问题

### Q: 沙箱是什么?

A: 沙箱使用 bubblewrap 限制 AI 的文件系统和网络访问，保护系统安全。

### Q: 如何启用沙箱?

A: 
```bash
# 命令行
vibecoding --sandbox

# 配置文件
{
  "sandbox": {
    "enabled": true,
    "level": "standard"
  }
}
```

### Q: 为什么沙箱不工作?

A: 
1. 检查 bubblewrap 是否安装: `bwrap --version`
2. 检查是否在 Linux 上 (macOS/Windows 不支持)
3. 检查配置是否正确

### Q: macOS/Windows 支持沙箱吗?

A: 不支持。bubblewrap 是 Linux 特有的。可以使用 WSL2。

## 会话问题

### Q: 会话存储在哪里?

A:
- Linux/macOS: `~/.vibecoding/sessions/--<编码的路径>--/`
- Windows: `%APPDATA%\vibecoding\sessions\--<编码的路径>--\`

### Q: 如何恢复旧会话?

A: 
```bash
# 恢复特定会话
vibecoding --resume <session-id>

# 继续最近会话
vibecoding --continue
```

### Q: 会话文件损坏怎么办?

A: 
1. 检查 JSONL 格式
2. 手动修复或删除损坏行
3. 使用备份

## 工具问题

### Q: 有哪些可用工具?

A: VibeCoding 包含核心内置工具，以及可选的多 Agent 工具：
- `read`: 读取文件内容（包括图像）
- `write`: 创建/覆盖文件
- `edit`: 精确文本替换
- `bash`: 执行 shell 命令
- `grep`: 正则内容搜索
- `find`: 文件名搜索
- `ls`: 目录列表
- `plan`: 发布可见任务计划和状态更新
- `subagent_*`: 使用 `--multi-agent` 启动时委托任务给子 Agent

详见 [工具系统](tools.md) 文档。

### Q: 如何使用多 Agent 工作流?

A: 使用 `--multi-agent` 启动 VibeCoding：

```bash
vibecoding --multi-agent
vibecoding acp --multi-agent
```

这会注册 `subagent_*` 工具用于委托工作。Cron 命令入口也依赖多 Agent 模式。

### Q: VibeCoding 能读取图像吗?

A: 可以！`read` 工具支持 PNG、JPEG、GIF 和 WebP 图像。图像以 base64 编码发送给 LLM 进行分析。

### Q: 工具不工作怎么办?

A:
1. 检查沙箱级别
2. 检查文件权限
3. 使用 `--debug` 查看详细日志

### Q: 如何限制工具权限?

A: 使用 Plan 模式（只读）或配置沙箱级别。在 Agent 模式下，bash 命令默认需要审批（可通过白名单/黑名单配置）。

## 构建问题

### Q: 构建失败怎么办?

A: 
```bash
# 检查 Go 版本
go version

# 更新依赖
go mod tidy

# 清理缓存
go clean -cache
make clean
make build
```

### Q: 测试失败怎么办?

A: 
```bash
# 运行特定测试
go test -v ./internal/agent/

# 查看详细输出
go test -v -run TestName ./...
```

## 其他问题

### Q: 如何报告 Bug?

A: 在 GitHub 上创建 Issue，包含:
- 操作系统和版本
- Go 版本
- 错误信息
- 复现步骤

### Q: 如何贡献代码?

A: 参考 [开发指南](development.md)。

### Q: 有社区支持吗?

A:
- GitHub Issues: 报告 Bug
- GitHub Discussions: 提问和讨论

### Q: 许可证是什么?

A: MIT License

### Q: 如何诊断环境问题?

A: 使用 `doctor` 子命令检查你的环境：

```bash
vibecoding doctor
```

这会检查系统信息、配置文件、Provider、模型、沙箱、MCP 服务器、会话、技能和上下文文件。报告中会对 API key 进行脱敏显示，并验证默认 Provider 是否可以正常初始化。

### Q: 当前版本是什么?

A: 当前版本是 v0.1.38。详见 [更新日志](changelog.md) 了解版本历史。
