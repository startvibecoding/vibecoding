# VibeCoding - AI Coding Assistant

## Project Overview

VibeCoding is a terminal-based AI coding assistant written in Go, inspired by [pi.dev](https://pi.dev). It supports multiple LLM providers (DeepSeek as the default demo, plus OpenAI, Anthropic, and any custom provider via OpenAI/Anthropic-compatible APIs), sandboxed execution via bubblewrap, and a rich TUI built with BubbleTea.

## Tech Stack

- **Language**: Go 1.24+
- **TUI**: BubbleTea + Lipgloss + Glamour
- **CLI**: Cobra
- **Sandbox**: bubblewrap (bwrap) on Linux

## Project Structure

```
vibecoding/
├── cmd/vibecoding/          # CLI entry point (main.go)
├── docs/                    # Project documentation
│   ├── architecture.md      # Architecture details
│   ├── cli-reference.md     # CLI command reference
│   ├── configuration.md     # Configuration guide
│   ├── development.md       # Development guide
│   ├── security.md          # Security documentation
│   ├── sessions.md          # Session management
│   ├── skills.md            # Skills documentation
│   ├── tools.md             # Tools documentation
│   ├── zh/                  # Chinese documentation
│   └── en/                  # English documentation
├── internal/
│   ├── agent/               # Core agent loop and system prompts
│   ├── config/              # Settings, auth, configuration
│   ├── context/             # Context management and token estimation
│   ├── contextfiles/        # Context file discovery (AGENTS.md, CLAUDE.md, etc.)
│   ├── platform/            # Cross-platform compatibility utilities
│   ├── provider/            # LLM provider abstraction
│   │   ├── anthropic/       # Anthropic Messages API
│   │   └── openai/          # OpenAI Chat Completions API
│   ├── sandbox/             # Sandbox abstraction (bwrap, none)
│   ├── session/             # Session management (JSONL format)
│   ├── skills/              # Skills system
│   ├── tools/               # Tool implementations
│   ├── tui/                 # Terminal UI
│   ├── ua/                  # User-Agent string generation
│   └── vendored/            # Embedded rg/fd binaries (go:embed)
└── pkg/sdk/                 # Public SDK (future)
```

## Architecture

### Provider System
- `provider.Provider` interface: `Chat(ctx, params) <-chan StreamEvent`
- Each provider implements SSE streaming for its API format
- Custom providers configured in `settings.json` with `api` field: `"openai-chat"` or `"anthropic-messages"`

### Agent Loop
1. Build system prompt (mode + tools + context files + skills)
2. Send messages to provider
3. Process stream events (text, thinking, tool calls)
4. Execute tools and append results
5. Repeat until done

### Tools
- `read`: File reading with offset/limit
- `write`: File creation
- `edit`: Precise text replacement
- `bash`: Command execution (through sandbox if enabled)
- `grep`: Content search (uses ripgrep/rg)
- `find`: File search (uses fd)
- `ls`: Directory listing

### Vendored Tools (rg/fd) — ✅ 已实现

grep 和 find 工具使用 ripgrep (rg) 和 fd 作为后端，通过 go:embed 内嵌到二进制中，统一 Windows/Linux/macOS 的表现。

**实现状态**：
- ✅ `internal/vendored/` 包已创建
- ✅ 6 个平台的 embed 文件已创建
- ✅ `scripts/prepare-vendored.sh` 脚本已创建
- ✅ `grep.go` 已改用 rg
- ✅ `find.go` 已改用 fd
- ✅ Makefile 已添加 `prepare-vendored` target
- ✅ 测试全部通过

**目录结构**：
```
internal/vendored/
├── bin/                          # 构建时临时目录，不提交 git
│   ├── linux-amd64/rg, fd
│   ├── linux-arm64/rg, fd
│   ├── darwin-amd64/rg, fd
│   ├── darwin-arm64/rg, fd
│   ├── windows-amd64/rg.exe, fd.exe
│   └── windows-arm64/rg.exe, fd.exe
├── embed_linux_amd64.go         # //go:build linux && amd64
├── embed_linux_arm64.go         # //go:build linux && arm64
├── embed_darwin_amd64.go
├── embed_darwin_arm64.go
├── embed_windows_amd64.go
├── embed_windows_arm64.go
└── vendored.go                  # Ensure() 提取逻辑
```

**工作流程**：
1. 构建时：`scripts/prepare-vendored.sh` 解压 pkgs/ 下载的压缩包到 `internal/vendored/bin/<platform>/`
2. 编译时：`go:embed` 只嵌入当前 GOOS/GOARCH 对应的二进制，不增大体积
3. 运行时：首次启动调用 `vendored.Ensure()` 解压到 `~/.vibecoding/bin/`
4. 工具调用：grep tool → exec rg，find tool → exec fd

**vendored.go 导出接口**：
- `Ensure() error` — 首次运行时解压 rg/fd 到 `~/.vibecoding/bin/`
- `RgPath() string` — 返回 rg 二进制路径
- `FdPath() string` — 返回 fd 二进制路径

**grep tool 参数映射 (rg)**：
| 工具参数 | rg 参数 |
|---------|--------|
| `pattern` | 位置参数 |
| `path` | 目录参数 |
| `include` | `-g "*.go"` |
| `maxResults` | `--max-count 100` |

附加参数：`--no-heading --line-number`（输出格式 `file:line: content`）

**find tool 参数映射 (fd)**：
| 工具参数 | fd 参数 |
|---------|--------|
| `pattern` | 位置参数（正则，非 glob） |
| `path` | 目录参数 |
| `maxDepth` | `--max-depth N` |
| `maxResults` | `--max-results 100` |

注意：fd 使用正则匹配，工具层需将 glob 模式（如 `*.go`）转为正则（`\.go$`）

**musl/glibc 兼容**：统一使用 musl 静态编译的二进制，兼容两种系统

**构建流程**：
```makefile
prepare-vendored:
    ./scripts/prepare-vendored.sh

build: prepare-vendored
    go build ...
```

### TUI Commands
- `/mode [plan|agent|yolo]`: Switch or show mode
- `/model [model_id]`: Switch or show model
- `/skills`: List available skills
- `/skill <name>` or `/skill:<name>`: Activate a skill
- `/sessions`: List sessions for current project
- `/sessions ls`: List all sessions
- `/sessions set <id>`: Switch to a session by ID prefix
- `/sessions clear`: Create a new fresh session
- `/sessions del <id>`: Delete a session by ID prefix
- `/clear`: Clear conversation
- `/quit`: Exit
- `/help`: Show help

### Sandbox Levels
- `none`: No restrictions (default)
- `standard`: Project read-write, no network (via --sandbox)
- `strict`: Project read-only, no network (Plan mode)

### Mode Permissions
- `plan`: Read-only tools only (read, grep, find, ls)
- `agent`: Read/write/edit auto-execute; bash requires user approval (with whitelist support)
- `yolo`: All tools auto-execute without approval

### Approval Configuration
In `settings.json`, configure approval whitelist:
```json
{
  "approval": {
    "bashWhitelist": ["go ", "make ", "git ", "npm ", "yarn "],
    "bashBlacklist": ["rm -rf", "sudo"]
  }
}
```

## Build & Run

```bash
# Build
make build

# Run
./bin/vibecoding

# Install
make install

# Cross-compile for all platforms
make build-all

# Build distribution packages\make dist
```

## Configuration

Config file:
- Linux/macOS: `~/.vibecoding/settings.json`
- Windows: `%APPDATA%\vibecoding\settings.json`

Key settings:
- `providers`: Multi-provider configuration
- `defaultProvider` / `defaultModel`: Default selections
- `defaultMode`: "plan", "agent", or "yolo"
- `defaultThinkingLevel`: "off", "minimal", "low", "medium", "high", "xhigh"
- `maxContextTokens`: Maximum context window size
- `maxOutputTokens`: Maximum output tokens
- `sandbox.enabled`: Enable sandbox (default: false)
- `contextFiles.enabled`: Auto-load context files
- `compaction`: Context compaction settings
- `retry`: Retry settings for API calls
- `theme`: UI theme ("dark" or "light")
- `shellPath`: Custom shell path for bash tool
- `shellCommandPrefix`: Custom command prefix

## Code Conventions

- Use `json.RawMessage` for JSON Schema parameters
- Error handling: return errors, don't panic
- Context propagation: pass `context.Context` through tool execution
- Channel-based streaming: providers return `<-chan StreamEvent`
- Keep tools stateless; registry holds sandbox/workdir references

## Session Format

JSONL files with tree structure:
- `id` / `parentId` for branching
- Entry types: `session`, `message`, `model_change`, `compaction`, `label`
- Stored in:
  - Linux/macOS: `~/.vibecoding/sessions/--<encoded-path>--/`
  - Windows: `%APPDATA%\vibecoding\sessions\--<encoded-path>--/`

## Skills System

Skills are reusable prompt snippets stored as SKILL.md files:
- Global skills:
  - Linux/macOS: `~/.vibecoding/skills/<name>/SKILL.md`
  - Windows: `%APPDATA%\vibecoding\skills\<name>\SKILL.md`
- Project skills: `.skills/<name>/SKILL.md` (overrides global)
- Project skills override global skills with the same name

## Testing

```bash
make test
```

## Git Workflow

- Conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`
- Main branch: `main`
- No force push to main
- **IMPORTANT**: Do NOT auto-commit after code changes. Only commit when the user explicitly requests it (e.g., "提交到git" or "commit").

## Documentation & Changelog

### Changelog Rules

- **Location**: Changelog entries are ONLY written in:
  - `docs/en/changelog.md` (English)
  - `docs/zh/changelog.md` (Chinese)
- **No separate files**: Do NOT create separate release notes files (e.g., `release-notes-vX.X.X.md`)
- **Format**: Follow existing changelog format with sections:
  - ✨ Features
  - 🐛 Bug Fixes
  - 🛠 Improvements
  - 📖 Documentation
  - 🧪 Testing

### When to Update README

Update README files (`docs/en/README.md` and `docs/zh/README.md`) when there are **major feature changes**:
- New major features or tools
- Significant changes to installation or usage
- New configuration options that affect core functionality
- Breaking changes that users need to know about

### Tag Management

- **Never delete existing tags**: Once a tag is created, it should not be deleted or modified
- **Always create new tags**: When releasing a new version, create a new tag with the next version number
- **Version sequence**: Follow the sequence strictly (e.g., v0.1.2 → v0.1.3 → v0.1.4 → ... → v0.2.0)

### Current Version

The current version is `v0.1.11`. The next version should be `v0.1.12`.
