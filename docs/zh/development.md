# 开发指南

本文档介绍如何为 VibeCoding 贡献代码。

## 开发环境搭建

### 系统要求

- Go 1.24+
- Git
- Make (可选)
- bubblewrap (可选，用于沙箱测试)

### 获取源码

```bash
git clone https://github.com/fuckvibecoding/vibecoding.git
cd vibecoding
```

### 安装依赖

```bash
go mod download
```

### 构建项目

```bash
# 构建
make build

# 安装到 $GOPATH/bin
make install
```

### 运行测试

```bash
# 运行所有测试
make test

# 运行特定包的测试
go test ./internal/tools/

# 运行特定测试
go test -run TestReadTool ./internal/tools/
```

## 项目结构

```
vibecoding/
├── cmd/vibecoding/          # CLI 入口点
│   └── main.go
├── internal/
│   ├── agent/               # 核心 Agent 循环
│   │   ├── agent.go         # Agent 主逻辑
│   │   ├── events.go        # 事件类型定义
│   │   ├── provider.go      # Provider 适配器
│   │   └── system_prompt.go # 系统提示词生成
│   ├── config/              # 配置管理
│   ├── contextfiles/        # 上下文文件加载
│   ├── provider/            # LLM Provider 抽象
│   │   ├── provider.go      # Provider 接口
│   │   ├── anthropic/       # Anthropic 实现
│   │   └── openai/          # OpenAI 实现
│   ├── sandbox/             # 沙箱实现
│   ├── session/             # 会话管理
│   ├── skills/              # 技能系统
│   ├── tools/               # 工具实现
│   │   ├── tool.go          # 工具接口和注册
│   │   ├── bash.go          # Bash 命令
│   │   ├── read.go          # 文件读取
│   │   ├── write.go         # 文件写入
│   │   ├── edit.go          # 文件编辑
│   │   ├── grep.go          # 内容搜索
│   │   ├── find.go          # 文件查找
│   │   └── ls.go            # 目录列表
│   ├── tui/                 # 终端 UI
│   └── util/                # 工具函数
└── pkg/sdk/                 # 公共 SDK (未来)
```

## 核心接口

### Provider 接口

```go
type Provider interface {
    Name() string
    Models() []*Model
    GetModel(id string) *Model
    Chat(ctx context.Context, params ChatParams) <-chan StreamEvent
}
```

### Tool 接口

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, params json.RawMessage) (string, error)
}
```

## 添加新工具

### 步骤 1: 创建工具文件

```go
// internal/tools/mytool.go
package tools

import (
    "context"
    "encoding/json"
)

type MyTool struct {
    workdir string
}

func NewMyTool(workdir string) *MyTool {
    return &MyTool{workdir: workdir}
}

func (t *MyTool) Name() string {
    return "mytool"
}

func (t *MyTool) Description() string {
    return "Description of my tool"
}

func (t *MyTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "param1": {
                "type": "string",
                "description": "First parameter"
            }
        },
        "required": ["param1"]
    }`)
}

func (t *MyTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
    var p struct {
        Param1 string `json:"param1"`
    }
    if err := json.Unmarshal(params, &p); err != nil {
        return "", err
    }
    // 实现工具逻辑
    return "result", nil
}
```

### 步骤 2: 注册工具

在 `internal/tools/tool.go` 的 `RegisterDefaults()` 方法中添加:

```go
func (r *Registry) RegisterDefaults() {
    r.Register(&ReadTool{workdir: r.workdir})
    r.Register(&WriteTool{workdir: r.workdir})
    r.Register(&EditTool{workdir: r.workdir})
    r.Register(&BashTool{workdir: r.workdir, sandbox: r.sandbox})
    r.Register(&GrepTool{workdir: r.workdir})
    r.Register(&FindTool{workdir: r.workdir})
    r.Register(&LsTool{workdir: r.workdir})
    r.Register(&MyTool{workdir: r.workdir}) // 添加新工具
}
```

### 步骤 3: 更新系统提示词

在 `internal/agent/system_prompt.go` 中添加工具描述。

### 步骤 4: 编写测试

```go
// internal/tools/mytool_test.go
package tools

import (
    "context"
    "testing"
)

func TestMyTool_Execute(t *testing.T) {
    tool := NewMyTool("/tmp")
    params := `{"param1": "value"}`
    
    result, err := tool.Execute(context.Background(), json.RawMessage(params))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if result != "expected" {
        t.Errorf("expected 'expected', got '%s'", result)
    }
}
```

## 添加新 Provider

### 步骤 1: 创建 Provider 目录

```bash
mkdir -p internal/provider/myprovider
```

### 步骤 2: 实现 Provider 接口

```go
// internal/provider/myprovider/provider.go
package myprovider

import (
    "context"
    "github.com/fuckvibecoding/vibecoding/internal/provider"
)

type MyProvider struct {
    apiKey  string
    baseURL string
}

func NewProvider(apiKey, baseURL string) *MyProvider {
    return &MyProvider{apiKey: apiKey, baseURL: baseURL}
}

func (p *MyProvider) Name() string {
    return "myprovider"
}

func (p *MyProvider) Models() []*provider.Model {
    return []*provider.Model{
        {ID: "model-1", Name: "Model 1"},
    }
}

func (p *MyProvider) GetModel(id string) *provider.Model {
    for _, m := range p.Models() {
        if m.ID == id {
            return m
        }
    }
    return nil
}

func (p *MyProvider) Chat(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
    ch := make(chan provider.StreamEvent)
    go func() {
        defer close(ch)
        // 实现流式调用
    }()
    return ch
}
```

### 步骤 3: 注册 Provider

在 `cmd/vibecoding/main.go` 的 `createProvider()` 函数中添加:

```go
case "myprovider":
    apiKey := settings.ResolveKey(providerName)
    p = myprovider.NewProvider(apiKey, pc.BaseURL)
```

## 测试

### 运行所有测试

```bash
make test
```

### 运行特定包的测试

```bash
go test ./internal/agent/
go test ./internal/tools/
```

### 生成覆盖率报告

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 代码规范

### 格式化

```bash
make fmt
```

或手动:

```bash
gofmt -w .
goimports -w .
```

### Lint

```bash
make lint
```

### 命名规范

- 包名: 小写单词，如 `tools`, `agent`
- 接口名: 动词或名词，如 `Provider`, `Tool`
- 结构体名: 大驼峰，如 `ReadTool`, `AgentConfig`
- 函数名: 大驼峰，如 `NewAgent`, `Execute`
- 变量名: 小驼峰，如 `workdir`, `maxTokens`

### 错误处理

```go
// 好的做法
result, err := doSomething()
if err != nil {
    return fmt.Errorf("do something: %w", err)
}

// 不好的做法
result, _ := doSomething()
```

## Git 工作流

### Commit 规范

使用 Conventional Commits:

```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

类型:
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档
- `refactor`: 重构
- `test`: 测试
- `chore`: 杂项

示例:

```
feat(tools): add new search tool
fix(agent): fix streaming issue
docs(readme): update installation guide
```

### Pull Request

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 运行测试
5. 创建 Pull Request

## 调试

### 启用调试日志

```bash
vibecoding --debug
```

### 使用 dlv

```bash
# 安装 dlv
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试
dlv debug ./cmd/vibecoding -- --debug
```

## 发布流程

### 版本号

使用 Semantic Versioning:

```
MAJOR.MINOR.PATCH

例如: 1.0.0, 1.1.0, 1.0.1
```

### 创建发布

```bash
# 更新版本号
git tag -a v1.0.0 -m "Release v1.0.0"

# 推送标签
git push --tags

# 构建发布包
make build-all
```

### 发布到 npm

```bash
# 发布正式版本
make npm-publish

# 发布预发布版本
make npm-publish-pre
```

用户可以通过 npm 安装:

```bash
npm install -g vibecoding-installer
```

## 常见问题

### Q: 如何添加新的工具?

A: 参考 [添加新工具](#添加新工具) 章节。

### Q: 如何添加新的 Provider?

A: 参考 [添加新 Provider](#添加新-provider) 章节。

### Q: 测试失败怎么办?

A:
1. 检查 Go 版本
2. 运行 `go mod tidy`
3. 查看错误日志

### Q: 如何调试沙箱问题?

A:
1. 使用 `--debug` 参数
2. 检查 bwrap 是否安装: `bwrap --version`
3. 查看系统日志
