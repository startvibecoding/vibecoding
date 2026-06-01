# VibeCoding Agent Guide

This file is for AI agents working in this repository. Keep changes aligned with the current codebase and prefer concise, minimal edits.

## Project Snapshot

- Language: Go
- UI: Bubble Tea + Lipgloss
- CLI: Cobra
- Default working style: terminal-first, tool-driven
- Main purpose: a terminal AI coding assistant with provider abstraction, sessions, tools, sandboxing, context files, skills, and an OpenAI-compatible HTTP gateway

## Important Directories

- `cmd/vibecoding/` — CLI entry
- `internal/agent/` — agent loop, events, system prompt
- `internal/config/` — settings and defaults
- `internal/context/` — context window and compaction
- `internal/contextfiles/` — `AGENTS.md` / `CLAUDE.md` discovery
- `internal/hermes/` — Hermes messaging gateway mode
- `internal/memory/` — persistent memory (memory.md)
- `internal/messaging/` — messaging platform abstraction (wechat, feishu)
- `internal/provider/` — provider abstraction and implementations
- `internal/provider/factory/` — shared provider/model construction from config
- `internal/provider/vendor*.go` — vendor adapter registry and per-vendor defaults
- `internal/sandbox/` — sandbox backends
- `internal/session/` — JSONL session storage
- `internal/skills/` — skills loading
- `internal/tools/` — built-in tools
- `internal/tui/` — terminal UI
- `internal/acp/` — ACP / MCP related integration
- `internal/a2a/` — A2A (Agent-to-Agent) protocol server and master mode
- `internal/gateway/` — OpenAI-compatible HTTP gateway mode
- `internal/vendored/` — embedded `rg` / `fd`
- `docs/` — documentation

## Architecture Notes

- Providers stream responses through the provider abstraction.
- Provider creation should go through `internal/provider/factory` so CLI and ACP keep the same behavior.
- Vendor-specific behavior belongs in `internal/provider/vendor*.go` adapters and model `compat` flags, not in CLI/ACP wiring.
- Each vendor that needs detection or defaults should have a separate `internal/provider/vendor_<name>.go` file.
- Vendors without special behavior should fall back to the generic OpenAI-compatible or Anthropic-compatible provider based on `api` / base URL detection.
- Do not change the settings JSON schema or the expected meaning of existing provider config fields when adding vendor support.
- The agent loop builds a system prompt, sends messages, handles stream events, executes tools, and continues until completion.
- Tools should stay stateless when possible; shared execution state belongs in registries/managers.
- Context files and skills are first-class prompt inputs.
- Sessions are stored as JSONL with parent/child relationships.

### Gateway Mode

- `internal/gateway/` implements an HTTP server exposing a standard OpenAI Chat Completions API.
- Gateway reuses the same agent loop, provider factory, session, tools, sandbox, and skills as CLI/ACP — no separate agent logic.
- Configuration lives in `gateway.json` (global `~/.config/vibecoding/gateway.json`, project `.vibe/gateway.json`), separate from `settings.json`.
- Project-level `.vibe/gateway.json` overrides global, same pattern as `.vibe/settings.json`.
- Gateway supports slash commands (`/clear`, `/mode`, `/compact`, etc.) processed at the HTTP layer without invoking the LLM.
- Tool output visibility (`toolVisibility.mode` + `toolVisibility.detail`) is configurable: collapsed (default, one-line summary) or expanded (full code fences).
- `edit`/`write` diffs and errors always show in full regardless of detail level.
- When `x_session_id` is empty, the gateway reuses a default session so consecutive requests share context.
- Security: three independent layers — Bearer token auth, `allowedWorkDirs` whitelist, sandbox (bwrap).
- No external HTTP framework; uses `net/http` standard library.

### Hermes Mode

- `internal/hermes/` implements a messaging gateway for WeChat/Feishu/WebSocket with persistent agent sessions.
- Hermes reuses the same agent loop, provider factory, session, tools, sandbox, skills, and MCP as CLI/ACP.
- Configuration lives in `hermes.json` (global `<GLOBAL_DIR>/hermes.json`, project `.vibe/hermes.json`).
- Per-user sessions stored in `<sessionDir>/hermes/<platform>/<user_id>/active.jsonl`.
- Default mode is `yolo` (not `agent`) — messaging platforms are unattended by nature.
- `default_provider` / `default_model` in hermes.json override settings.json; CLI `-p`/`-m` override hermes.json.
- `multi_agent` enables sub-agent tools (spawn/status/send/destroy).
- `sandbox` enables bwrap sandbox (default off).
- MCP servers from global/project `mcp.json` are loaded per-session and auto-closed on removal.
- memory.md defaults to project directory (`.vibe/memory.md`); only uses global when `memory.path` is explicitly set.
- Progress events (tool execution + thinking) are sent to messaging platforms via `InboundMessage.ProgressFunc`.
- The `messaging.InboundMessage.ProgressFunc` callback is set by each platform bot; nil means no progress updates.
- `formatToolProgress` in `dispatcher.go` formats tool events as `[tool]: args ✅/❌`.
- Think deltas are accumulated and flushed as `💭 ...` (truncated to 500 chars) before tool/text events.

## Working Rules

- Read before editing.
- Prefer small, targeted changes.
- Keep behavior consistent with existing patterns.
- Do not introduce broad refactors unless requested.
- Do not add license headers unless the repository already uses them.
- Do not auto-commit. Commit only when the user explicitly asks.

## Go Conventions

- Return errors; do not panic for normal control flow.
- Pass `context.Context` through request/execution paths.
- Keep interfaces and structs consistent with nearby code.
- Follow existing naming and file layout before introducing new abstractions.
- Add tests when changing behavior or fixing bugs if there is an obvious test location.

## Tooling Notes

Built-in tools include:
- `read`, `write`, `edit`
- `bash`, `jobs`, `kill`
- `grep`, `find`, `ls`
- `skill_ref`

`grep` and `find` are backed by embedded `rg` and `fd` binaries in `internal/vendored/`.

## Modes and Safety

- `plan`: read-only tools
- `agent`: file edits allowed; `bash` usually requires approval
- `yolo`: all tools auto-execute

When changing code, prefer the least risky approach that satisfies the request.

## Gateway-Specific Notes

- Gateway-only config belongs in `internal/gateway/config.go`, not in `internal/config/settings.go`.
- Tool output formatting (collapsed/expanded, markdown code fences) belongs in `internal/gateway/tool_format.go`.
- Slash command handlers belong in `internal/gateway/commands.go`, kept separate from TUI commands (different dependencies).
- The `resolveToolEvent()` helper in `handler_chat.go` handles the fact that `EventToolCall` carries tool name in `ev.ToolCall.Name` (not `ev.ToolName`).
- When adding new slash commands, add to both gateway `commands.go` and TUI `commands.go` to keep feature parity.

## Docs and Release Notes

- Put changelog updates only in:
  - `docs/en/changelog.md`
  - `docs/zh/changelog.md`
- Do not create separate release note files.
- Update README files only for user-visible major changes.

## Validation

When appropriate, verify with the smallest useful scope first.
Examples:
- focused package tests
- targeted grep/find checks
- full test suite only when necessary

## Build / Test

Common commands:
- `make build`
- `make test`

## Versioning Note

Current version: `v0.1.27`
Next version: `v0.1.28`
