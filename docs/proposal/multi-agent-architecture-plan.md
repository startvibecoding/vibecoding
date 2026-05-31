# Multi-Agent Architecture Status

This document records the implemented multi-agent architecture as of `v0.1.25`.
It replaces the original implementation checklist, which has been retired now
that the core work has landed.

## Decisions

| # | Decision | Status |
|---|----------|--------|
| 1 | Public Agent interface | Implemented in `agent/` |
| 2 | Per-agent Registry isolation | Implemented |
| 3 | Async sub-agent handle workflow | Implemented |
| 4 | Phased implementation | Completed through multi-agent, cron foundation, and provider adapter work |
| 5 | No nested sub-agents | Enforced by policy and registry filtering |
| 6 | Isolated sub-agent context | Implemented with independent messages, context, and session |
| 7 | Frozen prompt and dual-marker cache strategy | Reused by child agents |
| 8 | Multi-agent mode opt-in | Implemented with `--multi-agent` |
| 9 | Cron depends on multi-agent workflows | Foundation implemented; TUI command entry points are wired |
| 10 | Public package for external Agent usage | Implemented in `agent/` |
| 11 | Builder-based Agent creation | Implemented |
| 12 | Provider adapter architecture | Implemented with vendor adapters plus generic protocol providers |
| 13 | Provider selection fallback | Implemented: explicit vendor, base URL detection, generic fallback |
| 14 | Vendor differences via compat flags | Implemented for the currently supported OpenAI/Anthropic-compatible paths |

## Implemented Components

### Public Agent API

- `agent.Agent`, `agent.AgentID`, public event/message/context/provider types
- `agent.Builder` with provider, model, mode, workdir, thinking, tools, sandbox, session, compaction, and approval options
- Internal adapter bridge between public `agent` package and `internal/agent`

### Agent Runtime

- Agent IDs and parent IDs
- Agent event routing with AgentID metadata
- `AgentFactory` for centralized agent creation
- Per-agent `tools.Registry`
- Per-registry `JobManager`
- Sub-agent prompt context
- Sub-agent policy validation

### Multi-Agent Management

- `AgentManager` lifecycle management
- `EventRouter`
- `subagent_spawn`
- `subagent_status`
- `subagent_send`
- `subagent_destroy`
- Parent-to-child approval forwarding
- Registry filtering so sub-agents cannot spawn nested sub-agents

### CLI / TUI / ACP Integration

- `--multi-agent` flag in CLI and ACP
- Multi-agent manager wiring in CLI/TUI/ACP paths
- ACP session runtime support for agent manager/factory usage
- TUI command and event handling for multi-agent workflows

### Cron

- `internal/cron` package
- File-backed cron store
- Scheduler
- `/cron` command entry points in TUI multi-agent mode
- Tests for persistence and scheduling behavior

### Provider Adapter Layer

- Shared provider factory in `internal/provider/factory`
- Vendor adapter registry in `internal/provider/vendor.go`
- Per-vendor adapter files in `internal/provider/vendor_*.go`
- Generic fallback to OpenAI-compatible or Anthropic-compatible providers
- Compat handling for:
  - `thinkingFormat`
  - `supportsReasoningEffort`
  - `maxTokensField`
  - `forceAdaptiveThinking`
  - DeepSeek/Xiaomi assistant `reasoning_content`

## Provider Adapter Notes

Most vendors are protocol-compatible with OpenAI Chat Completions or Anthropic
Messages. Vendor adapter files should apply defaults and compatibility behavior,
while the protocol providers continue to handle request/stream mechanics.

Current vendor detection includes:

- `anthropic`
- `claude`
- `openai`
- `deepseek`
- `xiaomi`
- `xiaomi-token-plan-ams`
- `xiaomi-token-plan-cn`
- `xiaomi-token-plan-sgp`
- `kimi`
- `minimax`
- `seed`
- `qianfan`
- `bailian`
- `gitee`
- `openrouter`
- `together`
- `groq`
- `fireworks`

Adding a vendor should usually mean:

1. Add `internal/provider/vendor_<name>.go`.
2. Register base URL detection and defaults through `RegisterVendorAdapter`.
3. Add compat flags to model config only when a specific model needs protocol tweaks.
4. Keep the existing settings JSON schema stable.
5. Add targeted tests in `internal/provider` or the relevant protocol provider package.

## Acceptance Status

The `v0.1.25` release scope is accepted when:

- [x] Public Agent interface and Builder compile and are covered by tests
- [x] Agent IDs and parent IDs are present on agents and events
- [x] Each agent has isolated registry/job-manager state
- [x] AgentFactory is used for centralized agent creation
- [x] AgentManager supports create/get/destroy/list and parent-child relations
- [x] EventRouter dispatches by AgentID
- [x] Sub-agent tools work and are covered by tests
- [x] Sub-agent nesting is blocked
- [x] Multi-agent mode is opt-in through `--multi-agent`
- [x] Cron store and scheduler are covered by tests
- [x] TUI exposes `/cron` command entry points in multi-agent mode
- [x] Provider vendor adapter layer supports explicit vendor, base URL detection, and generic fallback
- [x] Existing provider config format remains compatible
- [x] OpenAI/Anthropic provider compat behavior is covered by tests
- [x] `make test` passes

## Known Follow-Ups

- Additional native provider protocols such as Google Gemini or Mistral can be
  added later as separate provider implementations.
- More compatibility flags from `/home/free/src/pi/packages/ai` can be wired as
  concrete behavior when a supported model or vendor requires them.
- Full natural-language cron parsing and persistent TUI cron management still
  need product wiring on top of the `internal/cron` foundation.
- Release packaging still needs to be rebuilt from a clean release tag for each
  published version.
