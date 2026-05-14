# 常见问题 (FAQ)

## 基本问题

### Q: VibeCoding 是什么?

A: VibeCoding 是一个终端 AI 编码助手，支持 OpenAI 和 Anthropic API，提供代码编写、调试、重构等功能。

### Q: 支持哪些 LLM?

A: 
- OpenAI: GPT-4o, o1, o3-mini 等
- Anthropic: Claude Sonnet 4, Claude 3.5 Sonnet, Haiku, Opus
- 自定义: 任何 OpenAI 或 Anthropic 兼容 API

### Q: 如何安装?

A: 
```bash
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
- 全局: `~/.vibecoding/settings.json`
- 项目: `.vibe/settings.json`
- 认证: `~/.vibecoding/auth.json`

### Q: 如何设置 API 密钥?

A: 三种方式:
1. 环境变量: `export ANTHROPIC_API_KEY=sk-ant-...`
2. 认证文件: `~/.vibecoding/auth.json`
3. 配置文件: `settings.json` 中的 `providers.<name>.apiKey`

### Q: 如何使用自定义 API?

A: 在 `settings.json` 中配置:

```json
{
  "providers": {
    "my-api": {
      "baseUrl": "https://my-api.example.com/v1",
      "api": "openai-chat",
      "apiKey": "my-key"
    }
  },
  "defaultProvider": "my-api"
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
```

### Q: 如何继续上次的会话?

A: 
```bash
vibecoding --continue
vibecoding -c
```

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

A: `~/.vibecoding/sessions/--<编码的路径>--/`

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

### Q: 工具不工作怎么办?

A: 
1. 检查沙箱级别
2. 检查文件权限
3. 使用 `--debug` 查看详细日志

### Q: 如何限制工具权限?

A: 使用 Plan 模式 (只读) 或配置沙箱级别。

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
