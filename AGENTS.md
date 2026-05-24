# VibeCoding Agent Guide

This file is for AI agents working in this repository. Keep changes aligned with the current codebase and prefer concise, minimal edits.

## Project Snapshot

- Language: Go
- UI: Bubble Tea + Lipgloss
- CLI: Cobra
- Default working style: terminal-first, tool-driven
- Main purpose: a terminal AI coding assistant with provider abstraction, sessions, tools, sandboxing, context files, and skills

## Important Directories

- `cmd/vibecoding/` — CLI entry
- `internal/agent/` — agent loop, events, system prompt
- `internal/config/` — settings and defaults
- `internal/context/` — context window and compaction
- `internal/contextfiles/` — `AGENTS.md` / `CLAUDE.md` discovery
- `internal/provider/` — provider abstraction and implementations
- `internal/sandbox/` — sandbox backends
- `internal/session/` — JSONL session storage
- `internal/skills/` — skills loading
- `internal/tools/` — built-in tools
- `internal/tui/` — terminal UI
- `internal/acp/` — ACP / MCP related integration
- `internal/vendored/` — embedded `rg` / `fd`
- `docs/` — documentation

## Architecture Notes

- Providers stream responses through the provider abstraction.
- The agent loop builds a system prompt, sends messages, handles stream events, executes tools, and continues until completion.
- Tools should stay stateless when possible; shared execution state belongs in registries/managers.
- Context files and skills are first-class prompt inputs.
- Sessions are stored as JSONL with parent/child relationships.

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

Current version: `v0.1.18`
Next version: `v0.1.19`
