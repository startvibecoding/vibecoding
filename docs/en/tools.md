# Tool System

VibeCoding provides a set of built-in tools for file operations, code search, and command execution.

## Tool Overview

| Tool | Description | Sandbox Restriction |
|------|-------------|-------------------|
| `read` | Read file content | Read-only directories accessible |
| `write` | Create/overwrite files | Only standard/yolo |
| `edit` | Precise text replacement | Only standard/yolo |
| `bash` | Execute shell commands | Subject to sandbox restrictions |
| `grep` | Regex content search | Read-only |
| `find` | Filename search | Read-only |
| `ls` | List directory contents | Read-only |
| `plan` | Publish task plan/status | Read-only |
| `jobs` | List and manage background jobs | Read-only |
| `kill` | Stop a running background job | Only standard/yolo |
| `question` | Ask user multiple-choice questions | Plan mode (TUI only) |
| `memory` | Read/write persistent memory | Hermes mode |
| `cron` | Manage scheduled background tasks | Hermes/multi-agent mode |
| `subagent_spawn` | Start a delegated sub-agent task | Multi-agent mode only |
| `subagent_status` | Query a sub-agent's status/result | Multi-agent mode only |
| `subagent_send` | Send follow-up instructions to a sub-agent | Multi-agent mode only |
| `subagent_destroy` | Stop and remove a sub-agent | Multi-agent mode only |
| `a2a_dispatch` | Send task to remote A2A agent | A2A Master mode only |
| `skill_ref` | Load skill reference file | When skills available |

## Tool Details

### read - File Reading

Read file content with pagination support. Supports both text files and images.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | ✓ | File path |
| `offset` | int | - | Starting line number (from 1) |
| `limit` | int | - | Maximum lines to read |

**Example:**

```json
{
  "path": "/home/user/project/main.go",
  "offset": 10,
  "limit": 50
}
```

**Returns:** 
- For text files: File content as text
- For images (PNG, JPEG, GIF, WebP): Base64-encoded image data with MIME type information

**Image Support:**

When reading image files, the tool returns rich content containing:
- A text description with file path, size, and type
- The image data encoded in base64 format

Supported image formats: `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`

---

### plan - Task Planning

Publish or update a visible task plan. Steps support `pending`, `running`, `done`, and `failed` statuses.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `title` | string | - | Short plan title |
| `steps` | array | ✓ | Ordered plan steps |
| `note` | string | - | Optional short note |

**Example:**

```json
{
  "title": "Implement structured diffs",
  "steps": [
    {"title": "Read tool result flow", "status": "done"},
    {"title": "Update write/edit results", "status": "running"},
    {"title": "Run focused tests", "status": "pending"}
  ]
}
```

**Returns:** Structured plan metadata for TUI, print mode, and ACP clients.

---

### subagent_* - Delegated Work

The `subagent_*` tools are registered only when VibeCoding runs with
`--multi-agent`. They let the main agent delegate bounded work to child agents
that have isolated messages, context, session, registry, and job-manager state.

Child agents cannot spawn further sub-agents.

#### subagent_spawn

Starts a child agent asynchronously and returns a handle.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task` | string | ✓ | Focused delegated task |
| `mode` | string | - | `plan`, `agent`, or `yolo`; defaults to `agent` |
| `work_dir` | string | - | Child working directory |
| `tools` | array | - | Optional allowed tool names |
| `max_iterations` | integer | - | Iteration cap |
| `system_prompt_extra` | string | - | Additional child-agent context |

#### subagent_status

Queries status and last result for a handle:

```json
{ "handle": "agent-1" }
```

#### subagent_send

Sends a follow-up message to an existing sub-agent:

```json
{ "handle": "agent-1", "message": "Focus on provider tests next." }
```

#### subagent_destroy

Destroys a sub-agent and releases its resources:

```json
{ "handle": "agent-1" }
```

---

### a2a_dispatch - A2A Remote Agent Dispatch

Send tasks to remote A2A agents registered in `a2a-list.json`. Only registered when launched with `--enable-a2a-master`.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `agent_name` | string | ✓ | Target agent name (auto-enumerated from config) |
| `message` | string | ✓ | Task message |

**Example:**

```json
{
  "agent_name": "code-reviewer",
  "message": "Review internal/handler.go for code quality"
}
```

**Returns:** Text response from the remote agent

See [A2A Protocol - A2A Master Mode](a2a.md#a2a-master-mode) for details.

---

### skill_ref - Skill Reference Loading

Load reference files from skill directories. Only registered when skills are available.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `skill` | string | ✓ | Skill name (directory name) |
| `ref` | string | ✓ | Reference file path (relative to skill directory) |

**Example:**

```json
{
  "skill": "my-conventions",
  "ref": "references/api-style.md"
}
```

**Returns:** Reference file content

---

### write - File Writing

Create new files or overwrite existing files.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | ✓ | File path |
| `content` | string | ✓ | File content |

**Example:**

```json
{
  "path": "/home/user/project/README.md",
  "content": "# My Project\n\nThis is a new project."
}
```

**Returns:** Success/failure message with structured diff metadata when content changes.

---

### edit - File Editing

Precise text replacement for modifying existing files.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | ✓ | File path |
| `edits` | array | ✓ | List of edit operations |

**edits array elements:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `oldText` | string | ✓ | Exact text to find |
| `newText` | string | ✓ | Replacement text |

**Example:**

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

**Best Practices:**

1. `oldText` must exactly match the text in the file, including spaces and newlines
2. Use `read` first to get file content and ensure `oldText` is correct
3. Use sufficiently long `oldText` to ensure unique matching
4. A single call can contain multiple edit operations

**Returns:** Success/failure message with structured diff metadata when content changes.

---

### bash - Command Execution

Execute shell commands.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `command` | string | ✓ | - | Command to execute |
| `timeout` | int | - | 120 | Timeout in seconds |

**Example:**

```json
{
  "command": "go test ./...",
  "timeout": 300
}
```

**Returns:** stdout and stderr output

**Sandbox Behavior:**

| Sandbox Level | File System | Network | Description |
|---------------|------------|---------|-------------|
| none | Full access | Allowed | No restrictions |
| standard | Project read/write | Disabled | Can only modify project files |
| strict | Project read-only | Disabled | Can only read project files |

---

### grep - Content Search

Search file content using regular expressions.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `pattern` | string | ✓ | - | Regular expression |
| `path` | string | - | Current directory | Search path |
| `include` | string | - | - | File matching pattern (e.g., `*.go`) |
| `maxResults` | int | - | 100 | Maximum number of results |

**Example:**

```json
{
  "pattern": "func\\s+\\w+\\(",
  "path": "/home/user/project",
  "include": "*.go",
  "maxResults": 50
}
```

**Returns:** Matching lines with file paths and line numbers

---

### find - File Search

Search files by filename pattern.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `pattern` | string | ✓ | - | Glob pattern |
| `path` | string | - | Current directory | Search path |
| `maxDepth` | int | - | Unlimited | Maximum directory depth |
| `maxResults` | int | - | 100 | Maximum number of results |

**Example:**

```json
{
  "pattern": "*.go",
  "path": "/home/user/project",
  "maxDepth": 3
}
```

**Returns:** List of matching file paths

---

### ls - Directory Listing

List directory contents.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | - | Current directory | Directory path |

**Example:**

```json
{
  "path": "/home/user/project"
}
```

**Returns:** Directory contents list with file types and sizes

---

### jobs - Background Job Management

List and check status of background jobs started with `bash async=true`.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `jobId` | int | - | Get detailed status of a specific job by ID |
| `cleanup` | bool | - | Remove finished jobs from the list |

**Example:**

```json
{}
```

**Returns:** List of background jobs with status (running/finished), or detailed info for a specific job including PID, elapsed time, stdout, and stderr.

---

### kill - Stop Background Job

Stop a running background job started with `bash async=true`.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `jobId` | int | ✓ | The job ID to kill |

**Example:**

```json
{ "jobId": 3 }
```

**Returns:** Confirmation message with job ID and PID.

---

### question - User Clarification (Plan Mode)

Ask the user a multiple-choice question during plan mode to clarify requirements.
Only registered in TUI + plan mode. Uses `QuestionHandler` optional interface (type assertion); not exposed in Gateway/Hermes/ACP.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `question` | string | ✓ | The question text |
| `options` | array | ✓ | List of option strings |

**Example:**

```json
{
  "question": "Which database should we use?",
  "options": ["PostgreSQL", "SQLite", "MongoDB"]
}
```

**Returns:** User's selected option or custom answer.

---

### memory - Persistent Memory (Hermes)

Read and write persistent memory stored in `memory.md`. Memory persists across sessions. Only available in Hermes mode.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | ✓ | Action: `read`, `add`, `update`, `delete` |
| `section` | string | - | Section name (e.g., `User Profile`, `Working Memory`, `Lessons Learned`). Required for add/update/delete; optional for read. |
| `content` | string | - | Content for add/delete actions |
| `old` | string | - | Old text for update action |
| `new` | string | - | New replacement text for update action |

**Example:**

```json
{
  "action": "add",
  "section": "User Profile",
  "content": "Prefers Go over Python for backend work."
}
```

**Returns:** Action confirmation or section content.

---

### cron - Scheduled Tasks (Hermes / Multi-Agent)

Manage scheduled background tasks that run via sub-agents. Available in Hermes mode and CLI multi-agent mode.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | ✓ | Action: `list`, `create`, `enable`, `disable`, `remove`, `run` |
| `id` | string | - | Job ID (required for enable/disable/remove/run) |
| `name` | string | - | Short task name (required for create) |
| `prompt` | string | - | Task prompt for the sub-agent (required for create) |
| `schedule` | string | - | Schedule: `@daily`, `@weekly`, `@monthly`, `@hourly`, `@every 30m`, `@every 2h`, or empty for one-shot |
| `oneshot` | bool | - | If true, run once then auto-disable |
| `mode` | string | - | Agent mode: `agent` or `yolo` (default: `yolo`) |

**Example:**

```json
{
  "action": "create",
  "name": "daily-check",
  "prompt": "Check for outdated dependencies and report.",
  "schedule": "@daily"
}
```

**Returns:** Job list, creation confirmation, or action result.

---

### MCP Dynamic Tools

Tools, resources, and prompts from MCP (Model Context Protocol) servers are auto-discovered and registered per session. Tool names and parameters are defined by the MCP server, not VibeCoding. MCP tools appear in the tool list alongside built-in tools.

See [Skills](skills.md) and [Configuration](configuration.md) for MCP server setup.

---

## Tool Usage Patterns

### Read-Modify-Write Pattern

This is the most common code editing pattern:

```
1. read   → Get file content
2. edit   → Make precise modifications
3. bash   → Verify changes (e.g., go build)
```

**Example Conversation:**

```
User: Fix the bug in main.go

Assistant:
  1. read("main.go")           # Read file
  2. Analyze code, find bug
  3. edit("main.go", edits)    # Fix bug
  4. bash("go build ./...")    # Verify compilation
```

### Search-Locate-Modify Pattern

When file location is unknown:

```
1. grep   → Search for relevant code
2. read   → View context
3. edit   → Modify code
```

### Project Exploration Pattern

Understanding project structure:

```
1. ls     → List root directory
2. find   → Find specific files
3. read   → Read key files
```

## Tool Error Handling

Tool execution failures return error messages:

```json
{
  "error": "open /path/to/file: no such file or directory"
}
```

Common error types:

| Error | Cause | Solution |
|-------|-------|----------|
| File not found | Wrong path or file deleted | Check path, use `find` to locate |
| Permission denied | Sandbox restriction or file permissions | Check sandbox level, verify file permissions |
| Timeout | Command execution too long | Increase timeout or optimize command |
| Edit failed | `oldText` doesn't match | Re-`read` to get latest content |

## Tool Limitations

### Sandbox Restrictions

In sandbox mode:

- **standard**: Project directory read/write, system directory read-only, no network
- **strict**: All directories read-only, no network

### Timeout Limits

- Default timeout: 120 seconds
- Maximum timeout: 600 seconds
- Long-running commands need larger timeout setting

### Output Limits

- Single output has size limit
- Excess content is truncated
- Use `offset` and `limit` for paginated reading of large files

## Tool Results

Tools return a `ToolResult` struct that supports both plain text and rich content:

```go
type ToolResult struct {
    Text     string                  // Plain text result (always populated)
    Contents []provider.ContentBlock // Rich content blocks (text + images)
}
```

### Creating Tool Results

```go
// Plain text result
return tools.NewTextToolResult("File written successfully"), nil

// Result with image (for tools that return images)
return tools.NewImageToolResult("Image loaded", "image/png", base64Data), nil
```

## Extending Tools

### Custom Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, params map[string]any) (ToolResult, error)
}
```

### Register Custom Tool

```go
registry := tools.NewRegistry(workdir, sandbox)
registry.Register(&MyCustomTool{})
```

## Best Practices

1. **Read before modifying**: Use `read` to view file content, then use `edit` to modify
2. **Precise matching**: `edit`'s `oldText` must match exactly
3. **Verify changes**: Use `bash` only for validation steps that need a shell (e.g., compile, test)
4. **Paginated reading**: Use `offset` and `limit` for large files
5. **Limit searches**: Use `include` and `maxResults` to limit search scope
