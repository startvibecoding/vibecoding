# Changelog


## v0.1.26

### ✨ Features

- **Gateway Mode** (`vibecoding gateway`)
  - New HTTP server exposing a standard OpenAI Chat Completions API (`/v1/chat/completions`, `/v1/models`, `/health`)
  - Any OpenAI-compatible client (Cursor, Continue, Open WebUI, Python SDK, etc.) can connect directly
  - Streaming (SSE) and non-streaming responses fully supported
  - Backend powered by VibeCoding agent loop with tool execution transparent to the caller

- **Multi-Session Support**
  - Built-in `SessionPool` for concurrent sessions, each with isolated agent, tools, and message history
  - Session association via `x_session_id` in request body; auto-created when absent
  - Configurable idle timeout (`session.idleTimeoutSeconds`) and max session limit (`session.maxSessions`)

- **Sub-Agent Support in Gateway**
  - Optional `enableSubAgents` config to enable multi-agent orchestration in gateway mode
  - Reuses existing `AgentFactory` / `AgentManager` / sub-agent tools with no core agent changes

- **Bearer Token Authentication**
  - Configurable via `gateway.json` with `auth.enabled` and `auth.tokens` list
  - Disabled by default; `/health` endpoint always unauthenticated

- **Slash Commands via API**
  - `/clear`, `/mode`, `/model`, `/models`, `/sessions`, `/compact`, `/status`, `/skill`, `/skills`, `/help`
  - Triggered when the last user message starts with `/`; processed at gateway layer without invoking LLM
  - Responses use standard OpenAI format with `x_command` extension field

- **Tool Visibility Configuration** (`toolVisibility.mode`)
  - `"content"` (default): tool status sent as text in `content` field during streaming
  - `"sse_event"`: tool status sent as extended SSE events for custom clients
  - `"none"`: fully transparent, client sees only final text

- **System Prompt Handling** (`systemPromptMode`)
  - `"append"` (default): client system messages appended to built-in system prompt
  - `"ignore"`: client system messages discarded entirely

- **Security: allowedWorkDirs**
  - Directory whitelist for `x_working_dir` request-level overrides with path-separator-aware prefix matching
  - Three-layer security model: L1 auth + L2 directory control + L3 sandbox (bwrap)

- **Sandbox Support in Gateway**
  - Configurable via `gateway.json` `sandbox.enabled` / `sandbox.level` or `--sandbox` flag
  - Inherits detailed sandbox settings (allowedRead, deniedPaths, etc.) from `settings.json`

- **Gateway Configuration** (`gateway.json`)
  - Independent config file at `~/.config/vibecoding/gateway.json`
  - Covers: listen address, auth, mode, sandbox, workingDir, allowedWorkDirs, session management, CORS, tool visibility, system prompt mode, request timeout, concurrency limit, logging
  - `vibecoding --init-gateway` to generate template; `--force` to overwrite

- **Request Timeout & Concurrency**
  - `requestTimeoutSeconds` (default 300s); streaming keeps alive as long as data flows
  - `maxConcurrentRequests` (default 0 = unlimited)

### 📝 Docs

- Added `docs/gateway-proposal.md` with full architecture, API design, security model, and implementation plan
- Updated `AGENTS.md` version note

## v0.1.25

### ✨ Features

- **Multi-Agent Mode**
  - Added opt-in `--multi-agent` support across CLI, TUI, and ACP mode
  - Added `AgentManager`, `EventRouter`, and per-agent registries so agents have isolated tools, job managers, sessions, messages, and context
  - Added `subagent_spawn`, `subagent_status`, `subagent_send`, and `subagent_destroy` tools for delegated background work
  - Added multi-agent prompt guidance and safeguards that prevent nested sub-agent spawning

- **Cron Task Support**
  - Added `internal/cron` with persistent cron store and scheduler coverage
  - Added `/cron` command entry points in multi-agent TUI workflows

- **Provider Vendor Adapter Layer**
  - Added vendor adapter registration in `internal/provider/vendor*.go`
  - Centralized provider/model creation in `internal/provider/factory`
  - Added vendor detection for DeepSeek, Xiaomi, Kimi, MiniMax, Seed, Qianfan, Bailian, Gitee, OpenRouter, Together, Groq, Fireworks, OpenAI, and Anthropic
  - Preserved existing provider config format while allowing vendor-specific defaults and generic OpenAI/Anthropic-compatible fallback
  - Added model `compat` handling for thinking formats, reasoning effort support, max token field selection, adaptive Anthropic thinking, and DeepSeek/Xiaomi assistant `reasoning_content`

### 🐛 Bug Fixes

- Auto-initialized sessions on first append so sub-agents can write session entries without requiring explicit prior initialization
- Fixed sub-agent tests to wait for background runs and clean up spawned agents before temporary directory removal
- Preserved ACP Anthropic cache-control behavior while moving provider creation to the shared factory

### 📝 Docs

- Updated `AGENTS.md` with provider factory and vendor adapter guidance
- Replaced the multi-agent implementation checklist with a completed architecture/status document
- Removed the obsolete root `todo.md`

### 🧪 Testing

- Added coverage for provider vendor resolution, provider factory creation, OpenAI/Anthropic compat behavior, multi-agent manager/router/sub-agent flows, cron storage/scheduler behavior, and session auto-initialization
- Verified with `make test` (`go test -v -race ./...`)

---

## v0.1.24

### ✨ Features

- **API Retry with Exponential Backoff**
  - Automatic retry for transient errors (5xx, network failures, rate limits) on initial HTTP connection
  - Exponential backoff: `baseDelay × 2^attempt`, capped at 30 seconds
  - Does NOT retry on user abort (`context.Canceled`), 4xx client errors, or mid-stream failures
  - Configurable via `retry` settings (`maxRetries`, `baseDelay`, `maxDelay`)
  - Agent forwards retry events as status updates visible in TUI and print mode
  - ACP mode also receives retry configuration

### 🐛 Bug Fixes

- **Anthropic `cache_control` Now Opt-In**
  - Changed default `cache_control` behavior to off (was auto-enabled for official API base URL)
  - Require explicit `cacheControl: true` in provider config to enable prompt caching
  - ACP provider creation explicitly enables `cache_control` for Anthropic

- **Anthropic Tool Result Grouping**
  - Fixed consecutive `toolResult` messages to be grouped into a single `user` message
  - Anthropic API requires all `tool_result` blocks for preceding `tool_use` to appear together before other content
  - Image blocks from tool results are now appended after all result blocks in the same message
  
- **Agent Tool-Only Loop Warning Ordering**
  - Moved the no-text tool-loop warning to be injected after tool results are appended
  - Keeps assistant -> toolResult -> warning message ordering valid for provider and session transcripts
  - Warning messages are now also persisted to session storage

### 📝 Docs

- **Comprehensive Configuration Documentation Rewrite**
  - Added missing settings: `cacheControl`, idle compression, full sandbox fields (`bwrapPath`, `allowedRead`, `allowedWrite`, `deniedPaths`, `passEnv`, `tmpSize`), `shellPath`, `shellCommandPrefix`, `sessionDir`, `skillsDir`, `theme`, `retry`
  - Documented shell command `apiKey` format (`!cmd`) for password manager integration
  - Fixed key resolution order: config `apiKey` first, then derived env var
  - Fixed macOS config path: `~/Library/Application Support/vibecoding/`
  - Added top-level fields reference table with all defaults
  - Added per-platform defaults for sandbox paths and env vars
  - Improved examples with Claude provider `cacheControl`, idle compression, project-level overrides, and custom sandbox paths

### 🧪 Testing

- Added retry tests covering `IsRetryable`, `RetryDelay`, and `FormatRetryMessage`
- Added Anthropic provider tests for consecutive tool result grouping
- Added a regression test covering tool-only warning placement after tool results


---

## v0.1.23

### 🛠 Improvements

- **DeepSeek Thinking Format**
  - Added `thinkingFormat: "deepseek"` for DeepSeek reasoning requests
  - OpenAI-compatible requests now send `thinking: {type: "enabled"}` with `reasoning_effort`
  - Anthropic-compatible requests now send `thinking: {type: "enabled"}` with `output_config.effort`
  - Kept `thinkingFormat: "xiaomi"` as the legacy thinking-only format

### 🧪 Testing

- Added provider tests covering the new `deepseek` thinking format for both OpenAI- and Anthropic-compatible requests

### 📝 Docs

- Updated `anthropic-api` skill and configuration docs for the new `thinkingFormat` option

---

## v0.1.22

### ✨ Features

- **CLI/TUI MCP Auto-Loading**
  - CLI/TUI startup now loads global and project `mcp.json`, connects configured MCP servers, and registers MCP tools before the agent tool list is frozen

### 🐛 Bug Fixes

- **Markdown Rendering Style**
  - Switched CLI print mode and TUI markdown rendering from Glamour auto-style detection to the fixed `dark` style for more consistent terminal output

### 🧪 Testing

- Added MCP config loader coverage for placeholder template filtering

### 🛠 Improvements

- **Shared MCP Runtime**
  - Moved MCP connection/tool registration out of ACP-only code into a shared runtime used by ACP and normal CLI/TUI sessions
  - Starter-template placeholder MCP servers are ignored during automatic startup loading

---

## v0.1.21

### ✨ Features

- **Plan/Apply Workflow**
  - Added a built-in `plan` tool for structured task plans with `pending`, `running`, `done`, and `failed` step statuses
  - TUI now shows the current task plan and records plan updates in the transcript
  - Print mode and ACP now surface plan updates for non-interactive and editor-client flows

- **Apply Confirmation**
  - Added `approval.confirmBeforeWrite` to require approval before `write` and `edit` in agent mode
  - Enabled write/edit confirmation by default in generated settings
  - TUI approval prompts summarize write content by byte size instead of dumping full file content

- **MCP Config Commands**
  - Added `/init_mcp` to create project/global `mcp.json` with `basic`/`full` templates and optional `--force`
  - Added `/mcps` to list MCP servers from global and project `mcp.json` files
  - MCP config is now maintained in standalone `mcp.json` (separate from `settings.json`)

### 🧪 Testing

- Added coverage for the `plan` tool and write/edit approval gating
- Added HTTP-based MCP integration tests for tool/resource/prompt registration and callback paths
- Added SSE-based MCP integration tests for stream callbacks and message endpoint request/response flow

### 🛠 Improvements

- **ACP MCP Hardening**
  - Added MCP transport support for `http` and `sse` (alongside existing `stdio`)
  - Added MCP initialize/tool-discovery timeouts to avoid hanging ACP sessions
  - Added paginated `tools/list` fetching with upper page bounds
  - Added MCP `resources/*` and `prompts/*` discovery and tool registration
  - Added duplicate MCP server-name detection and MCP tool-name de-duplication
  - Added MCP inbound request/notification handling (`ping`, progress/logging/cancel notifications)
  - Added bridge for inbound `sampling/createMessage` to the active ACP provider/model
  - Added stricter close/error propagation

---

## v0.1.20

### ✨ Features

- **Structured File Change Reporting**
  - `write` and `edit` now attach structured file diff metadata to tool results
  - TUI tool details show full unified diffs while collapsed tool rows keep a compact `+N -N` summary
  - Print mode now emits clear file change summaries for non-interactive runs
  - ACP tool updates include diff metadata in raw output for compatible clients

### 🧪 Testing

- Added coverage for structured diff metadata from `write` and `edit`

---

## v0.1.19

### ✨ Features

- **TUI Tool Details Modal**
  - Replaced `Ctrl+O` toggle-expand with a scrollable full-screen modal overlay showing all tool calls and results
  - Supports PgUp/PgDn, Up/Down, Home/End navigation; Esc/Ctrl+O/q to close
  - Tool headers now display file paths; removed content truncation in tool args display
  - Write tool results show diff summary in the one-line summary line
  - Key input is blocked while the modal is open to prevent accidental actions

- **Write Tool Diff Summary**
  - `write` tool now computes LCS-based line-level diff when overwriting files
  - Returns structured diff info (`+N -N` with line ranges) in the tool result
  - Skips diff computation for very large files (>200K line pairs) to avoid memory pressure

### 🛠 Improvements

- **Unified Shell Args Across Sandbox Backends**
  - All sandbox backends (`none`, `mac`, `windows`) now use `platform.ShellArgs()` for cmd.exe/PowerShell argument construction
  - Fixes Windows cmd.exe and PowerShell commands in sandboxed execution modes
  - `ShellArgs` now normalizes shell name to lowercase before matching

### 🧪 Testing

- Added `TestNoneSandboxWrapCommandUsesPlatformShellArgs` covering cmd.exe and PowerShell argument generation

---

## v0.1.18

### 🐛 Bug Fixes

- **TUI Nil Pointer Panic**
  - Fixed a nil pointer panic in `printMessageOnce` when `printedMessageIdx` map was not initialized
  - Added nil check before accessing the map in the message printing logic

- **Stream Commit Before Tool Execution**
  - Added `commitActiveStream()` method to flush streaming content (thinking and assistant messages) to output before tool execution
  - Now properly commits active stream before `EventToolCall` and `EventToolApprovalRequest` handling
  - Ensures thinking and partial assistant responses are visible when tools run or approval is requested

### 🧪 Testing

- Added `TestHandleAgentEventCommitsStreamBeforeApproval` regression test for stream commit ordering

---

## v0.1.17

### 🛠 Improvements

- **TUI Native Scrollback**
  - Reworked TUI history rendering so completed messages are printed into the terminal's native scrollback instead of a fixed-height viewport
  - Removed the virtual scrollbar and mouse-capture approach; mouse wheel scrolling now uses normal terminal history behavior
  - Kept live streaming content, input, footer, context/cache status, and tool output controls in the Bubble Tea view

- **TUI Request Timers**
  - Added per-request elapsed time display while a response is running
  - Footer now keeps the last request duration after completion

- **Event Loop Decoupling**
  - Added shared agent event consumption helpers
  - Split the TUI agent-event bridge out of the main app file and reused the event loop from CLI print mode

- **Windows Console Compatibility**
  - Enabled Windows virtual terminal console modes where available for better PowerShell rendering on Windows 10

### 🐛 Bug Fixes

- Fixed a TUI startup deadlock caused by printing initial/session history before Bubble Tea had started consuming program messages
- Fixed an agent message-history data race found by `go test -race`
- Fixed mock provider cancellation handling for already-canceled contexts

### 🧪 Testing

- Full `make test` now passes with race detection
- Added TUI regression coverage for startup history printing without blocking
- Hardened tests that depend on local HTTP listeners or default home-directory session paths in restricted environments

---

## v0.1.16

### 🛠 Improvements

- **Session Open by ID or Path**
  - New `OpenByPathOrID` function allows opening sessions by either file path or session ID
  - `OpenByID` now supports prefix matching with ambiguity detection
  - `ContinueRecent` initializes new sessions immediately so they are ready for messages

- **Session Save Error Handling**
  - `AppendMessage` and `AppendCompaction` now return errors to the caller
  - Agent loop surfaces session-save failures as `EventError` instead of silently dropping them

- **Vendored Tool Test Guard**
  - Makefile `test` target now depends on `prepare-vendored` and a new `test-vendored` check
  - Tests fail early with a clear message if `rg`/`fd` binaries are missing for the current platform

### 🧪 Testing

- Added CLI flag parsing tests for root and ACP subcommands
- Added settings merge tests covering project overrides and environment variables
- Added session tests for `OpenByPathOrID`, prefix ambiguity, corrupt lines, and parent chain tracking

---

## v0.1.15

### 🐛 Bug Fixes

- **Vendored Search Tool Availability**
  - Fixed `grep` and `find` so they prepare embedded `rg` / `fd` binaries on demand instead of failing when vendored tools have not been extracted yet
  - Restored executable permissions for already-extracted vendored binaries to avoid `permission denied` failures on reuse

- **Bash Tool Result Handling**
  - Fixed bash tool responses to report stdout, stderr, working directory, and exit code in a stable structured format
  - Preserved non-zero command exits as normal tool results with explicit `exit_code` output instead of mixing shell failures into transport-level errors
  - Standardized empty stdout/stderr rendering as `(no output)` for more predictable downstream handling

---

## v0.1.14

### 🐛 Bug Fixes

- **Session Continue Context Injection (`-c`)**
  - Fixed a TUI state coupling issue where continued sessions could display history but fail to inject that history into the model context for follow-up prompts
  - Split session history state into separate UI-display and agent-injection flags to ensure resumed conversations keep prior context
  - Reset agent history-injection state consistently when the agent is recreated (abort/mode/model/skill/session switches)
  - Added missing TUI handlers for `EventStatus` and `EventMessageStart` so status/warning messages are rendered reliably

### 🧪 Testing

- Added regressions that cover:
  - history injection when UI history is already loaded
  - real startup ordering (`Init()` history load, then follow-up input) for continued sessions

---

## v0.1.13

### 🐛 Bug Fixes

- **Streaming Event and Tool Call Robustness**
  - Preserved terminal agent events in the TUI event listener so done/error/status handling is not dropped during streaming
  - Added Anthropic thinking signature streaming and replay support, and surfaced SSE `error` events as proper stream errors
  - Generated fallback tool call IDs for OpenAI-compatible streamed tool calls when providers omit IDs, with an extra defensive fallback in the agent loop

- **Sandbox Environment Inheritance**
  - Fixed `none` sandbox execution so commands inherit the parent environment, including variables such as `$HOME`
  - Clarified bubblewrap environment override handling to match runtime behavior

### 🛠 Improvements

- **Vendored Tool Build Flow**
  - Unified build and distribution targets around `prepare-vendored`
  - Removed the old `vendored-tools` release step and deprecated the stale extract helper script

- **Documentation Site Layout**
  - Expanded the docs landing page content width for better large-screen readability

- **Package Metadata**
  - Updated npm package versions for installer packages

### 📖 Documentation

- Updated README and docs landing pages to highlight safer approval handling, unified cache metrics, and consistent provider debugging
- Simplified `AGENTS.md` guidance for repository agents

### 🧪 Testing

- Added bash tool output coverage for stdout-only, stderr-only, no-output, and non-zero exit cases
- Added TUI regression tests for status/warning rendering and done/error event passthrough
- Added OpenAI streaming regression coverage for tool calls with missing IDs

---

## v0.1.12

### 🐛 Bug Fixes

- **Unified Cache Hit Rate Semantics**
  - Restored cache hit rate calculation to use the full prompt footprint (`CacheRead / TotalInputTokens()`)
  - Aligned CLI print mode token display with TUI cache-aware totals
  - Updated Anthropic cache tests and shared provider usage tests to match the unified definition

- **Approval Safety in Non-Interactive and YOLO Flows**
  - Made `bashBlacklist` effective in approval checks with higher priority than `bashWhitelist`
  - Blacklisted bash commands now still require approval in `yolo` mode
  - `--print` mode now fails fast instead of auto-approving commands that would require user confirmation

### 🛠 Improvements

- **Debug Output Consistency**
  - `--debug` now also enables provider-level request/response debug output
  - Applied the same behavior to ACP mode

- **Cross-Platform Path Handling**
  - Replaced string-based `.skills` path construction with `filepath.Join(...)`

### 📖 Documentation

- Updated CLI reference to document stricter `--print` behavior and debug output behavior
- Updated configuration guide for approval precedence and `VIBECODING_DEBUG`
- Updated root README and documentation landing pages to highlight safer approval handling, unified cache metrics, and provider debug behavior

### 🧪 Testing

- Added approval behavior tests for whitelist/blacklist and `yolo` mode
- Added print mode regression test for approval-required tool calls
- Expanded cache-related provider tests to cover the unified cache hit rate definition

---

## v0.1.11

### 🛠 Improvements

- **Command Structure Refactoring**
  - Extracted root command creation into separate function for better testability
  - Added unit tests for command initialization and configuration
  - Improved code modularity and maintainability

### 📖 Documentation

- **License & Documentation Updates**
  - Added MIT license file
  - Added Chinese README (README_zh.md) for broader accessibility
  - Updated npm package versions

---

## v0.1.10

### ✨ Features

- **ACP Support Documentation**
  - Added ACP (Agent Client Protocol) support documentation to READMEs
  - VibeCoding can run as an ACP stdio agent for editor integrations
  - Compatible with VS Code, Zed, and JetBrains IDEs (IntelliJ IDEA/WebStorm) via ACP plugins

### 📖 Documentation

- Updated main README.md with ACP support feature
- Updated English README with features section
- Updated Chinese README with features section

---

## v0.1.9

### 🐛 Bug Fixes

- **TUI Deferred Render Goroutine Safety**
  - Fixed `scheduleRender` calling `updateViewportContent` from background goroutine without marshalling back to Bubble Tea's UI goroutine
  - Added `renderRequestMsg` type and `program.Send()` to properly marshal UI updates
  - Added `program *tea.Program` field and `SetProgram()` method for deferred UI scheduling

### 🛠 Improvements

- **TUI Abort Clears Queued Input**
  - Clear input queue and reset input state on manual abort and mode change
  - Prevents buffered keystrokes from executing after abort

- **Assistant Slot Reservation**
  - Added `EventTurnStart` handling to reserve display slot before text deltas arrive
  - Prevents tool output from shifting assistant message index mid-stream
  - Added empty raw markdown check in `updateViewportContent`

- **Tool Prompt Snippets**
  - Added "(preferred for ...)" hints to `read`, `ls`, `grep`, `find` tool descriptions
  - Reordered tool registration: read-only tools registered before write/edit/bash

### 🧪 Testing

- Added `TestHandleAgentEventReservesAssistantSlotBeforeTextDelta` test
- Added `TestAbortClearsQueuedInput` test

---

## v0.1.8

### 🐛 Bug Fixes

- **Token Counting with Cache-Aware TotalTokens**
  - Fixed Anthropic `TotalTokens` calculation to include `CacheRead` and `CacheWrite` tokens
  - Added `PromptTokens()` and `TotalInputTokens()` helper methods to `Usage` struct
  - Updated `CacheInfo()` to use `TotalInputTokens()` as denominator for accurate cache hit rates
  - Updated TUI to display correct token counts including cache tokens

### 🧪 Testing

- Added comprehensive tests for `PromptTokens()` and `TotalInputTokens()` helper methods
- Updated Anthropic provider tests with `TotalTokens` validation

---

## v0.1.7

### 🐛 Bug Fixes

- **Anthropic Provider Tool Use Serialization**
  - Fixed `tool_use` content blocks missing `input` field when tool has no arguments
  - Changed `Input` field from `map[string]interface{}` to `*map[string]interface{}` so `omitempty` only checks nil pointer, not empty map
  - Fixes API errors when using models like Xiaomi MiMo with Anthropic-compatible endpoints

---

## v0.1.6

### ✨ Features

- **Session Management Command**
  - Added `/sessions` command for browsing and managing project sessions
  - Supports listing, switching, clearing, and deleting sessions
  - Shows session details including file path and message count

### 🐛 Bug Fixes

- **Sandbox Initialization**
  - Fixed sandbox initialization validation and bwrap multiarch compatibility
  - Improved error handling for sandbox setup

### 📖 Documentation

- Updated AGENTS.md with current version information
- Formatted Go code for consistency

---

## v0.1.5

### ✨ Features

- **DeepSeek V4 Default Models**
  - Updated default model specs to DeepSeek V4 (Flash and Pro)
  - 1M context window, up to 384K max output tokens
- **Install Script Improvements**
  - Install scripts now show config directory path on completion

### 🐛 Bug Fixes

- **Windows IME Support**
  - Fixed Windows IME (CJK input) support in terminal
  - Fixed shell command resolution on Windows
  - Added config loading diagnostics for troubleshooting
- **Musl Deb Packages**
  - Fixed invalid dpkg architecture names for musl deb packages

### 🛠 Improvements

- **Configuration Simplification**
  - Removed `auth.json` support — all credentials now in `settings.json` only
  - Cleaner config path with single source of truth

### 📖 Documentation

- Clarified that OpenAI/Anthropic API-compatible services are also supported
- Removed all `auth.json` references from docs and install scripts
- Added expanded Windows `%APPDATA%` path examples
- Clearly distinguished Windows vs Linux/macOS config paths

---

## v0.1.4

### ✨ Features

- **Linux musl Build Support**
  - Added `make build-linux-musl` target for statically linked musl binaries (amd64 + aarch64)
  - musl tarballs produced via `dist-tarball` and `dist` targets
  - musl Debian packages produced via `dist-deb` target (amd64-musl / arm64-musl)
  - npm packages: `vibecoding-installer-linux-musl-x64` and `vibecoding-installer-linux-musl-arm64`
  - npm uses `libc` field for proper musl/glibc resolution (npm >=9.4)
  - postinstall.js auto-detects musl vs glibc on Linux

---

## v0.1.3

### ✨ Features

- **Versioning Rules**
  - Added version number management rules with base-10 carry-over (e.g., v0.1.9 -> v0.2.0)
  - Documented changelog rules: only write in docs/en/changelog.md and docs/zh/changelog.md
  - No separate release notes files allowed

---

## v0.1.2

### ✨ Features

- **Prompt Cache Optimization**
  - Implemented prompt cache optimization following LLM_Agent_Cache.md strategy
  - Cache system prompts and static context across multiple turns
  - Reduces API costs by reusing cached tokens for repeated prefixes

- **TUI Markdown Syntax Highlighting**
  - Assistant messages in TUI now have markdown syntax highlighting
  - Code blocks, headers, and formatting are visually distinguished
  - Improves readability of LLM responses

### 🐛 Bug Fixes

- **Security & Correctness**
  - Resolved critical security, race condition, and correctness issues
  - Addressed high and medium severity correctness issues across codebase
  - Removed dead code and improved overall code correctness

- **TUI Stability**
  - Fixed TUI startup hang caused by `clearStdin` blocking on unsupported stdin
  - Fixed TUI assistant message rendering broken by ANSI escape codes in prefix check

### 🛠 Improvements

- **Code Quality**
  - Addressed remaining medium severity issues across codebase
  - npm package versions updated

---

## v0.1.1

### ✨ Features

- **Cache Hit Rate Display**
  - Footer now shows cumulative cache hit percentage across all turns
  - Cache percentage is highlighted when hit rate ≥ 50% for quick visibility
  - Per-turn cache read/write counts displayed in token usage line

- **Proxy Compatibility**
  - Handle proxies that send usage fields in `message_delta` instead of `message_start`
  - Handle OpenAI proxies that split usage across multiple SSE chunks (first-wins per field)
  - Fixed missing space before `$` in print-mode token summary line

### 🛠 Improvements

- **Code Quality**
  - Extracted `Usage.CacheInfo()` to eliminate 3× duplicated cache display logic
  - npm package versions now use `v`-prefixed format (e.g. `v0.1.1`)
  - Normalized JSON formatting across all npm package.json files

### 🧪 Testing

- Added 37 unit tests for `CacheInfo()`, `formatCachePercent()`, and `renderFooter()` cache section
- Added 12 httptest integration tests for Anthropic and OpenAI SSE cache token parsing

---

## v0.1.0

### ✨ Features

- **Xiaomi MiMo Thinking Format Support**
  - Added `thinkingFormat` configuration option for Xiaomi MiMo API
  - OpenAI provider: MiMo endpoints use `thinking: {type: "enabled"}` format
  - Anthropic provider: MiMo endpoints omit `budget_tokens`
  - URL auto-detection: auto-detects `xiaomimimo` endpoints when `thinkingFormat` is not set
  - Debug logging: enabled via `VIBECODING_DEBUG` environment variable

### 🛠 Improvements

- **Configuration Flexibility**
  - `thinkingFormat` passed from config to provider, no longer relies solely on URL detection
  - Anthropic `budget_tokens` changed from required to optional (pointer type + `omitempty`)

---

## v0.0.9

### ✨ Features

- **Image Support in Tools**
  - `read` tool now supports reading image files (PNG, JPEG, GIF, WebP)
  - Images are returned as base64-encoded data with MIME type information
  - LLMs can now analyze and understand image content
  - Supported formats: `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`

- **Rich Content Tool Results**
  - New `ToolResult` struct supports both plain text and rich content blocks
  - Tools can now return text + images in a single result
  - New factory functions: `NewTextToolResult()` and `NewImageToolResult()`

- **Model Switching**
  - `/model <id>` command allows switching models in interactive mode
  - `/model` without arguments shows current model and available options
  - Agent resets automatically when model is switched

- **Enhanced Help System**
  - `/help` command now shows detailed command descriptions
  - Added keyboard shortcuts reference (Tab, Esc, Ctrl+O, PgUp/PgDn)

### 🛠 Improvements

- **Context Token Estimation**
  - Fixed double-counting issue when both `Content` and `Contents` are present
  - Image tokens estimated as ~1200 tokens per image

- **Provider Message Conversion**
  - OpenAI: Images in tool results sent as supplementary user messages
  - Anthropic: Images sent as separate user messages alongside tool_result

### 🧪 Testing

- Added `TestReadToolImage` test case for image reading functionality
- All tool tests updated for new `ToolResult` return type

---

## v0.0.8

### ✨ Features

- **NPM Multi-Architecture Split Packages**
  - Split the npm package from a single all-platform bundle (~60MB) into 6 platform-specific packages (~10MB each)
  - Users now only download the binary for their current platform, reducing install size by 83%
  - Uses npm `optionalDependencies` + `os`/`cpu` fields for automatic platform matching
  - Main package `vibecoding-installer` is only ~2KB, links the correct platform package via `postinstall`

### 🛠 Improvements

- **Build System**
  - Added `scripts/build-npm-packages.sh` to generate platform-specific npm packages
  - Added `make npm-packages`, `make npm-pack`, `make npm-publish-all` targets
  - `sync-npm-version.sh` now syncs versions across all platform packages

---

## v0.0.7

### ✨ Features

- **Cross-Platform Sandbox Support**
  - Sandbox now supports macOS and Windows in addition to Linux
  - macOS uses `sandbox-exec` for process isolation
  - Windows uses restricted process creation without network access
  - Platform-specific sandbox implementations selected automatically

- **Repository Rename**
  - Module path renamed to `github.com/startvibecoding/vibecoding`
  - All imports, documentation, and scripts updated accordingly

### 🛠 Improvements

- **Platform-Specific Process Handling**
  - Extracted `SysProcAttr` configuration into build-tagged files (`bash_unix.go`, `bash_windows.go`)
  - Background child process cleanup now works correctly on all platforms
  - `Setpgid` only set on Unix systems; Windows uses `CREATE_NEW_PROCESS_GROUP`

### 📖 Documentation

- Updated all GitHub URLs to new repository location
- Added v0.0.6 and v0.0.7 release notes

---

## v0.0.6

### 🛠 Improvements

- **Bash Tool Reliability**
  - Fixed background child process hanging issue
  - Added `WaitDelay` to prevent shell from waiting indefinitely on background children
  - Properly handle `exec.ErrWaitDelay` errors

- **NPM Installation**
  - Added npm package for installation via `npm install -g vibecoding-installer`
  - Automatic binary download during `postinstall`

### 📖 Documentation

- Added npm installation instructions
- Removed redundant markdown files from docs root
- Added v0.0.5 release notes

---

## v0.0.5

### ✨ Features

- **Non-root Installation**
  - `install.sh` now supports installation without root or sudo
  - Auto-detects writable install directory: uses `/usr/local/bin` if writable, otherwise falls back to `~/.vibecoding/bin`
  - Removes all `sudo` calls — user-level installation never requires elevated privileges

- **Automatic PATH Setup**
  - Auto-detects user's shell (bash, zsh, fish) and configures PATH in the appropriate config file
  - Supports `.bashrc`, `.bash_profile`, `.zshrc`, `.zshenv`, `config.fish`, and `.profile`
  - Skips configuration if PATH entry already exists (no duplicates)
  - Fish shell uses `set -gx PATH` syntax; bash/zsh use `export PATH=...`

### 🛠 Improvements

- **Environment Variables**
  - `INSTALL_DIR` — override the install directory (unchanged)
  - `AUTO_SETUP_PATH=0` — disable automatic PATH configuration
  - Better error messages for permission issues

- **Install Experience**
  - Shows install directory and PATH auto-setup status at the start
  - Cleaner output with colored status messages

### 📖 Documentation

- Added v0.0.5 release notes

---

## v0.0.4

### ✨ Features

- **Agent Mode Approval Mechanism**
  - Bash commands in Agent mode now require user approval
  - Configurable `bashWhitelist` for auto-approved command prefixes
  - Configurable `bashBlacklist` for commands always requiring approval
  - TUI displays approval prompt; user responds with `y`/`yes` or `n`/`no`
  - Approval requests can be cancelled via `abort`

- **Mode Permission Matrix**
  - Plan mode: Read-only tools (read, grep, find, ls)
  - Agent mode: Read/write auto-execute, bash requires approval
  - YOLO mode: All tools auto-execute
  - Updated system prompts with explicit permission matrix

### 🛠 Improvements

- **Default Approval Whitelist**
  - Default whitelist: `go`, `make`, `git`, `npm`, `yarn`, `node`, `python`, `pip`
  - Customizable in `settings.json`

- **Mode Switch Feedback**
  - Mode switching now shows detailed permission descriptions
  - `/mode` command displays full permission list for current mode

### 📖 Documentation

- Added approval configuration section
- Updated security docs with approval mechanism details
- Added v0.0.4 release notes

---

## v0.0.3

### ✨ Features

- **Session History Loading**
  - Display session info (file path and message count) when continuing or opening sessions
  - Load and display historical messages from previous sessions in TUI
  - Load history messages into agent context for continuity
  - Reset agent on abort to ensure clean state for next request

### 🛠 Improvements

- **Build & Distribution System**
  - Restructured Makefile with clear per-platform build and dist targets
  - Added `dist-linux`, `dist-darwin`, `dist-windows` targets
  - Added `build-zip.sh` for Windows zip packages
  - Added `checksums` target for release verification
  - Updated `build-deb.sh` and `build-tarball.sh` to support all platforms

### 📖 Documentation

- Added GitHub repository button in documentation site header
- Added v0.0.2 release notes

---

## v0.0.2

### ✨ Features

- **One-line Installation Scripts**
  - `install.sh` for Linux/macOS - downloads from GitHub Releases automatically
  - `install.ps1` for Windows PowerShell - supports custom install directory via `VIBECODING_INSTALL_DIR`
  - Both scripts detect platform/architecture, verify checksums, and configure PATH

- **Documentation Redesign**
  - Redesigned with Google Material Design style
  - Default language changed to English
  - Added hash routing for easy document sharing (e.g., `#/en/README`, `#/zh/configuration`)
  - Added logo to header and README

- **Brand Assets**
  - Added `docs/assets/icon.svg` (512×512) for packaging
  - Added `docs/assets/logo.svg` (128×128) for README and small displays
  - Minimal, professional design with slate color palette

- **Build System**
  - Added `make build-windows` target (amd64 + arm64)
  - Added `make build-linux` and `make build-darwin` targets
  - Updated `make build-all` to use platform-specific targets

- **Documentation**
  - Added `docs/en/skills.md` for Skills system
  - Updated installation instructions in README and getting-started guides

### 🐛 Bug Fixes

- Moved assets to `docs/assets/` for proper GitHub Pages deployment

---

**Full Changelog**: https://github.com/startvibecoding/vibecoding/compare/v0.0.1...v0.0.7
