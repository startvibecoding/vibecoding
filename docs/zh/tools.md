# 工具系统

VibeCoding 提供了一组内置工具，用于文件操作、代码搜索和命令执行。

## 工具概览

| 工具 | 描述 | 沙箱限制 |
|------|------|----------|
| `read` | 读取文件内容 | 只读目录可访问 |
| `write` | 创建/覆盖文件 | 仅 standard/yolo |
| `edit` | 精确文本替换 | 仅 standard/yolo |
| `bash` | 执行 shell 命令 | 受沙箱限制 |
| `grep` | 正则表达式搜索 | 只读 |
| `find` | 文件名搜索 | 只读 |
| `ls` | 列出目录内容 | 只读 |
| `plan` | 发布任务计划/状态 | 只读 |
| `subagent_spawn` | 启动委托子 Agent 任务 | 仅多 Agent 模式 |
| `subagent_status` | 查询子 Agent 状态/结果 | 仅多 Agent 模式 |
| `subagent_send` | 向子 Agent 发送后续指令 | 仅多 Agent 模式 |
| `subagent_destroy` | 停止并移除子 Agent | 仅多 Agent 模式 |
| `a2a_dispatch` | 向远程 A2A Agent 发送任务 | 仅 A2A Master 模式 |
| `skill_ref` | 加载技能引用文件 | 技能可用时 |

## 工具详解

### read - 文件读取

读取文件内容，支持分页。支持文本文件和图像文件。

**参数:**

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `path` | string | ✓ | 文件路径 |
| `offset` | int | - | 起始行号 (从 1 开始) |
| `limit` | int | - | 最大读取行数 |

**示例:**

```json
{
  "path": "/home/user/project/main.go",
  "offset": 10,
  "limit": 50
}
```

**返回:** 
- 文本文件：文件内容文本
- 图像文件（PNG、JPEG、GIF、WebP）：Base64 编码的图像数据及 MIME 类型信息

**图像支持：**

读取图像文件时，工具返回富内容，包含：
- 文件路径、大小和类型的文本描述
- Base64 编码的图像数据

支持的图像格式：`.png`、`.jpg`、`.jpeg`、`.gif`、`.webp`

---

### plan - 任务计划

发布或更新可见的任务计划。步骤支持 `pending`、`running`、`done` 和 `failed` 状态。

**参数:**

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `title` | string | - | 简短计划标题 |
| `steps` | array | ✓ | 有序计划步骤 |
| `note` | string | - | 可选简短说明 |

**示例:**

```json
{
  "title": "实现结构化 diff",
  "steps": [
    {"title": "阅读工具结果流程", "status": "done"},
    {"title": "更新 write/edit 结果", "status": "running"},
    {"title": "运行 focused tests", "status": "pending"}
  ]
}
```

**返回:** 提供给 TUI、print 模式和 ACP 客户端的结构化计划元数据。

---

### subagent_* - 委托工作

`subagent_*` 工具仅在使用 `--multi-agent` 启动时注册。主 Agent 可通过它们将边界清晰的任务委托给子 Agent；子 Agent 拥有独立的 messages、context、session、registry 和 job manager 状态。

子 Agent 不能继续派生子 Agent。

#### subagent_spawn

异步启动子 Agent，并返回 handle。

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `task` | string | ✓ | 聚焦的委托任务 |
| `mode` | string | - | `plan`、`agent` 或 `yolo`；默认 `agent` |
| `work_dir` | string | - | 子 Agent 工作目录 |
| `tools` | array | - | 可选工具白名单 |
| `max_iterations` | integer | - | 迭代上限 |
| `system_prompt_extra` | string | - | 附加子 Agent 上下文 |

#### subagent_status

查询某个 handle 的状态和最后结果：

```json
{ "handle": "agent-1" }
```

#### subagent_send

向已有子 Agent 发送后续消息：

```json
{ "handle": "agent-1", "message": "接下来关注 provider 测试。" }
```

#### subagent_destroy

销毁子 Agent 并释放资源：

```json
{ "handle": "agent-1" }
```

---

### a2a_dispatch - A2A 远程 Agent 调度

向 `a2a-list.json` 中注册的远程 A2A Agent 发送任务。仅在使用 `--enable-a2a-master` 启动时注册。

**参数:**

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `agent_name` | string | ✓ | 目标 agent 名称（从配置自动枚举） |
| `message` | string | ✓ | 任务消息 |

**示例:**

```json
{
  "agent_name": "code-reviewer",
  "message": "审查 internal/handler.go 的代码质量"
}
```

**返回:** 远程 agent 的文本响应

详见 [A2A 协议 - A2A Master 模式](a2a.md#a2a-master-模式)。

---

### skill_ref - 技能引用加载

加载技能目录中的引用文件。仅在有可用技能时注册。

**参数:**

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `skill` | string | ✓ | 技能名称（目录名） |
| `ref` | string | ✓ | 引用文件路径（相对于技能目录） |

**示例:**

```json
{
  "skill": "my-conventions",
  "ref": "references/api-style.md"
}
```

**返回:** 引用文件内容

---

### write - 文件写入

创建新文件或覆盖现有文件。

**参数:**

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `path` | string | ✓ | 文件路径 |
| `content` | string | ✓ | 文件内容 |

**示例:**

```json
{
  "path": "/home/user/project/README.md",
  "content": "# My Project\n\nThis is a new project."
}
```

**返回:** 成功/失败消息；内容变更时附带结构化 diff 元数据。

---

### edit - 文件编辑

精确文本替换，用于修改现有文件。

**参数:**

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `path` | string | ✓ | 文件路径 |
| `edits` | array | ✓ | 编辑操作列表 |

**edits 数组元素:**

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `oldText` | string | ✓ | 要查找的精确文本 |
| `newText` | string | ✓ | 替换文本 |

**示例:**

```json
{
  "path": "/home/user/project/main.go",
  "edits": [
    {
      "oldText": "func main() {\n\tfmt.Println(\"old\")\n}",
      "newText": "func main() {\n\tfmt.Println(\"new\")\n}"
    }
  ]
}
```

**最佳实践:**

1. `oldText` 必须精确匹配文件中的文本，包括空格和换行
2. 先使用 `read` 获取文件内容，确保 `oldText` 正确
3. 尽量使用足够长的 `oldText` 以确保唯一匹配
4. 单次调用可以包含多个编辑操作

**返回:** 成功/失败消息；内容变更时附带结构化 diff 元数据。

---

### bash - 命令执行

执行 shell 命令。

**参数:**

| 参数 | 类型 | 必填 | 默认值 | 描述 |
|------|------|------|--------|------|
| `command` | string | ✓ | - | 要执行的命令 |
| `timeout` | int | - | 120 | 超时时间 (秒) |

**示例:**

```json
{
  "command": "go test ./...",
  "timeout": 300
}
```

**返回:** stdout 和 stderr 输出

**沙箱行为:**

| 沙箱级别 | 文件系统 | 网络 | 说明 |
|----------|---------|------|------|
| none | 完全访问 | 允许 | 无限制 |
| standard | 项目读写 | 禁止 | 只能修改项目文件 |
| strict | 项目只读 | 禁止 | 只能读取项目文件 |

---

### grep - 内容搜索

使用正则表达式搜索文件内容。

**参数:**

| 参数 | 类型 | 必填 | 默认值 | 描述 |
|------|------|------|--------|------|
| `pattern` | string | ✓ | - | 正则表达式 |
| `path` | string | - | 当前目录 | 搜索路径 |
| `include` | string | - | - | 文件匹配模式 (如 `*.go`) |
| `maxResults` | int | - | 100 | 最大结果数 |

**示例:**

```json
{
  "pattern": "func\\s+\\w+\\(",
  "path": "/home/user/project",
  "include": "*.go",
  "maxResults": 50
}
```

**返回:** 匹配的行，包含文件路径和行号

---

### find - 文件搜索

按文件名模式搜索文件。

**参数:**

| 参数 | 类型 | 必填 | 默认值 | 描述 |
|------|------|------|--------|------|
| `pattern` | string | ✓ | - | Glob 模式 |
| `path` | string | - | 当前目录 | 搜索路径 |
| `maxDepth` | int | - | 无限 | 最大目录深度 |
| `maxResults` | int | - | 100 | 最大结果数 |

**示例:**

```json
{
  "pattern": "*.go",
  "path": "/home/user/project",
  "maxDepth": 3
}
```

**返回:** 匹配的文件路径列表

---

### ls - 目录列表

列出目录内容。

**参数:**

| 参数 | 类型 | 必填 | 默认值 | 描述 |
|------|------|------|--------|------|
| `path` | string | - | 当前目录 | 目录路径 |

**示例:**

```json
{
  "path": "/home/user/project"
}
```

**返回:** 目录内容列表，包含文件类型和大小

---

## 工具使用模式

### 读取-修改-写入模式

这是最常见的代码编辑模式：

```
1. read   → 获取文件内容
2. edit   → 精确修改
3. bash   → 验证修改 (如 go build)
```

**示例对话:**

```
用户: 修复 main.go 中的 bug

助手:
  1. read("main.go")           # 读取文件
  2. 分析代码，找到 bug
  3. edit("main.go", edits)    # 修复 bug
  4. bash("go build ./...")    # 验证编译
```

### 搜索-定位-修改模式

当不确定文件位置时：

```
1. grep   → 搜索相关代码
2. read   → 查看上下文
3. edit   → 修改代码
```

### 项目探索模式

了解项目结构：

```
1. ls     → 列出根目录
2. find   → 查找特定文件
3. read   → 读取关键文件
```

## 工具错误处理

工具执行失败时会返回错误信息：

```json
{
  "error": "open /path/to/file: no such file or directory"
}
```

常见错误类型：

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| 文件不存在 | 路径错误或文件已删除 | 检查路径，使用 `find` 查找 |
| 权限拒绝 | 沙箱限制或文件权限 | 检查沙箱级别，确认文件权限 |
| 超时 | 命令执行时间过长 | 增加 timeout 或优化命令 |
| 编辑失败 | `oldText` 不匹配 | 重新 `read` 获取最新内容 |

## 工具限制

### 沙箱限制

在沙箱模式下：

- **standard**: 项目目录可读写，系统目录只读，无网络
- **strict**: 所有目录只读，无网络

### 超时限制

- 默认超时: 120 秒
- 最大超时: 600 秒
- 长时间运行的命令需要设置更大的 timeout

### 输出限制

- 单次输出有大小限制
- 超出部分会被截断
- 使用 `offset` 和 `limit` 分页读取大文件

## 工具结果

工具返回支持纯文本和富内容的 `ToolResult` 结构体：

```go
type ToolResult struct {
    Text     string                  // 纯文本结果（始终填充）
    Contents []provider.ContentBlock // 富内容块（文本 + 图像）
}
```

### 创建工具结果

```go
// 纯文本结果
return tools.NewTextToolResult("文件写入成功"), nil

// 包含图像的结果（用于返回图像的工具）
return tools.NewImageToolResult("图像已加载", "image/png", base64Data), nil
```

## 扩展工具

### 自定义工具接口

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, params map[string]any) (ToolResult, error)
}
```

### 注册自定义工具

```go
registry := tools.NewRegistry(workdir, sandbox)
registry.Register(&MyCustomTool{})
```

## 最佳实践

1. **先读后改**: 使用 `read` 查看文件内容，再用 `edit` 修改
2. **精确匹配**: `edit` 的 `oldText` 必须精确匹配
3. **验证修改**: 仅在需要 shell 的验证步骤中使用 `bash` (如编译、测试)
4. **分页读取**: 大文件使用 `offset` 和 `limit`
5. **限制搜索**: 使用 `include` 和 `maxResults` 限制搜索范围
