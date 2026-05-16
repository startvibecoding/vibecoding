# Prompt Cache 优化实现文档

本文档描述了 vibecoding 项目中实现的 Prompt Cache 优化，遵循 [LLM_Agent_Cache.md](/home/free/src/agents/LLM_Agent_Cache.md) 守则。

## 实现概览

### 已实现的准则

| 准则 | 状态 | 说明 |
|------|------|------|
| 准则一：请求前缀分段 | ✅ | 三段式架构：session-stable, append-only, session-volatile |
| 准则二：冻结 System Prompt | ✅ | 构建时一次性生成，session 内不再修改 |
| 准则三：双标记滚动缓冲 | ✅ | 实现了双标记选择算法和 cache_control 标记 |
| 准则四：不换模型压缩 | ✅ | Insert-then-Compress 模式，使用相同的 system prompt |
| 准则五：主动管理 Cache TTL | ✅ | 空闲压缩检测机制（默认关闭） |

## 详细实现

### 1. 冻结 System Prompt (准则二)

**文件**: `internal/agent/agent.go`

System prompt 在 Agent 构造时一次性构建，之后在整个 session 生命周期内不再修改：

```go
// buildFrozenPrompt builds the system prompt and tools once at construction time.
// This implements Rule R2.1: System prompt must be built once and never modified.
func (a *Agent) buildFrozenPrompt() {
    // ... 构建 system prompt 和 tools
    a.frozenSystemPrompt = BuildSystemPrompt(...)
    a.frozenToolDefs = a.registry.ModeTools(a.config.Mode)
}
```

### 2. Session Context 消息 (准则二)

**文件**: `internal/agent/agent.go`

动态信息（日期、模型、工作目录等）通过 `[session context]` 消息注入，而不是写入 system prompt：

```go
func (a *Agent) buildSessionContextMessage() provider.Message {
    context := fmt.Sprintf(`[session context]
- Current date: %s
- Model: %s (%s)
- Working directory: %s
- Mode: %s`, ...)
    
    return provider.NewSystemInjectedUserMessage(context)
}
```

消息标记为 `SystemInjected: true`，cache marker 选择算法会跳过它。

### 3. 双标记滚动缓冲 (准则三)

**文件**: `internal/agent/agent.go`

实现了双标记选择算法，从消息列表尾部向前遍历，跳过注入消息：

```go
func selectCacheMarkers(messages []provider.Message) [2]int {
    // 选择最后两条非注入消息作为 cache marker
    // markers[0] = 第二新的标记
    // markers[1] = 最新的标记
}
```

标记应用函数为选中的消息添加 `cache_control: {type: "ephemeral"}`：

```go
func applyCacheMarkers(messages []provider.Message, markers [2]int) []provider.Message {
    // 深拷贝消息，避免修改原始数据
    // 在最后两个非注入消息上添加 cache_control 标记
}
```

### 4. Insert-then-Compress 压缩 (准则四)

**文件**: `internal/context/compaction.go`

压缩使用与主对话**相同的 system prompt 和 tools**，而不是独立的 LLM 调用：

```go
func GenerateSummaryInsertThenCompress(
    ctx context.Context,
    messages []provider.Message,
    p provider.Provider,
    systemPrompt string,    // 使用相同的 system prompt
    tools []provider.ToolDefinition,  // 使用相同的 tools
    previousSummary string,
    maxTokens int,
) (string, error) {
    // 注入压缩指令作为 system_injected 消息
    compressionMsg := provider.NewSystemInjectedUserMessage(instruction)
    
    // 使用相同的 system prompt 和 tools
    params := provider.ChatParams{
        Messages:     compactionMessages,
        Tools:        tools,
        SystemPrompt: systemPrompt,
        MaxTokens:    maxTokens,
    }
    // ...
}
```

### 5. 空闲压缩检测 (准则五)

**文件**: `internal/context/compaction.go`, `internal/config/settings.go`

空闲压缩机制默认关闭，可通过配置启用：

```json
{
  "compaction": {
    "enabled": true,
    "reserveTokens": 16384,
    "keepRecentTokens": 20000,
    "idleCompressionEnabled": false,
    "idleTimeoutSeconds": 90,
    "idleMinTokensForCompress": 150000
  }
}
```

### 6. Anthropic API Cache Control 支持

**文件**: `internal/provider/anthropic/provider.go`

- System prompt 作为 content block 数组发送，带有 `cache_control: {type: "ephemeral"}`
- 消息中的 cache_control 标记正确传递到 API

```go
// System prompt 以 content block 数组格式发送，支持 cache_control
if params.SystemPrompt != "" {
    sysBlock := anthropicContentBlock{
        Type: "text",
        Text: params.SystemPrompt,
        CacheControl: &anthropicCacheControl{Type: "ephemeral"},
    }
    reqBody.System = []anthropicContentBlock{sysBlock}
}
```

### 7. 消息类型扩展

**文件**: `internal/provider/types.go`

添加了 CacheControl 和 SystemInjected 支持：

```go
type CacheControl struct {
    Type string `json:"type"` // "ephemeral"
}

type ContentBlock struct {
    // ... 其他字段
    CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type Message struct {
    // ... 其他字段
    SystemInjected bool `json:"systemInjected,omitempty"`
}
```

## 请求结构示例

### 优化前
```json
{
  "system": "You are VibeCoding... Working directory: /home/user... Current date: 2024-01-15...",
  "messages": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi"}
  ]
}
```

### 优化后
```json
{
  "system": [
    {
      "type": "text",
      "text": "You are VibeCoding...",  // 不含动态信息
      "cache_control": {"type": "ephemeral"}
    }
  ],
  "messages": [
    {"role": "user", "content": "[session context]\n- Current date: 2024-01-15\n- Model: Claude 4 Sonnet...", "systemInjected": true},
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi", "cache_control": {"type": "ephemeral"}}
  ]
}
```

## Cache 命中率预期

- **System prompt**: 始终命中（冻结不变）
- **Session context**: 每天变化一次，基本稳定
- **历史消息**: append-only 模式，前缀稳定
- **预期 Cache 命中率**: 90%+

## 测试

运行缓存相关测试：

```bash
go test ./internal/agent/ -v -run "TestSelect|TestApply|TestSystem|TestNew"
```

## 注意事项

1. **OpenAI 兼容性**: OpenAI 使用自动缓存，不支持显式 cache_control，相关字段会被忽略
2. **系统提示冻结**: 如需修改 system prompt（如切换 mode），需要创建新的 Agent 实例
3. **空闲压缩**: 默认关闭，生产环境启用前建议测试线程安全性
