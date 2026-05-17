---
name: anthropic-api
description: Anthropic Messages API interface notes, usage fields, prompt caching, streaming behavior, and tool-use compatibility for this project.
---

# Anthropic API

Use this skill when working on Anthropic Messages API requests, SSE parsing, tool use blocks, prompt caching, or model-specific compatibility issues in this repository.

## Load order

1. Read this file first.
2. Read [references/anthropic.md](references/anthropic.md) for the full Messages API request/response schema, streaming event flow, tool-use payloads, and prompt-caching semantics.

## Working rules

- Keep `input_tokens`, `cache_read_input_tokens`, and `cache_creation_input_tokens` distinct.
- Treat cached tokens as part of the full prompt footprint, not as extra completion.
- Normalize usage once in the provider layer; avoid re-deriving Anthropic totals in the UI.
- Preserve tool-use payload shape exactly, especially when tool input is empty or streamed in fragments.

## Typical uses

- Parse Anthropic Messages SSE events
- Handle `message_start`, `content_block_*`, and `message_delta`
- Map `tool_use` / `tool_result`
- Work with prompt caching and cache control markers
