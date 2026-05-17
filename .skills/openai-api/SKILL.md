---
name: openai-api
description: OpenAI Chat Completions interface notes, usage fields, prompt caching, streaming behavior, and reasoning/tool-call compatibility for this project.
---

# OpenAI API

Use this skill when working on OpenAI-compatible chat completion requests, SSE streaming, usage parsing, cached tokens, or model-specific request fields in this repository.

## Load order

1. Read this file first.
2. Read [references/openai.md](references/openai.md) for the full Chat Completions request/response schema, streaming event flow, usage accounting, prompt caching, reasoning, and tool-call edge cases.

## Working rules

- Keep `prompt_tokens` and `cached_tokens` semantics straight: `cached_tokens` is a subset of prompt input, not extra output.
- Treat the last streamed usage chunk as optional when requests are cancelled or interrupted.
- Validate tool-call arguments before execution; OpenAI tool JSON can be incomplete or invalid until the final chunk.
- Prefer the project's existing `provider.Usage` helpers when calculating totals and cache display.

## Typical uses

- Parse OpenAI Chat Completions SSE chunks
- Handle `usage.prompt_tokens_details.cached_tokens`
- Map tool calls and function arguments
- Adjust reasoning-related fields only for supported models or compatible proxies
