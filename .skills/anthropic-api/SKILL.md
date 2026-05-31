---
name: anthropic-api
description: Anthropic Messages API notes, Claude model IDs, adaptive/manual thinking, usage fields, prompt caching, streaming behavior, and tool-use compatibility for this project.
---

# Anthropic API

Use this skill when working on Anthropic Messages API requests, Claude model compatibility, adaptive/manual thinking, SSE parsing, tool use blocks, prompt caching, or model-specific request fields in this repository.

## Load order

1. Read this file first.
2. Read [references/anthropic.md](references/anthropic.md) for the Messages API request/response schema, current Claude model notes, adaptive/manual thinking rules, streaming event flow, tool-use payloads, and prompt-caching semantics.

## Working rules

- Keep `input_tokens`, `cache_read_input_tokens`, and `cache_creation_input_tokens` distinct.
- Treat cached tokens as part of the full prompt footprint, not as extra completion.
- Normalize usage once in the provider layer; avoid re-deriving Anthropic totals in the UI.
- Preserve tool-use payload shape exactly, especially when tool input is empty or streamed in fragments.
- Only send thinking parameters for models that support the selected thinking mode.
- For Claude Opus 4.7, do not send manual `thinking: { "type": "enabled", "budget_tokens": ... }`; use adaptive thinking.

## Typical uses

- Parse Anthropic Messages SSE events
- Handle `message_start`, `content_block_*`, and `message_delta`
- Map `tool_use` / `tool_result`
- Work with prompt caching and cache control markers
- Configure Claude 4.6/4.7 thinking fields and `output_config.effort`
