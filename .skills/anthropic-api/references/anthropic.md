# Anthropic Messages API Reference

## Contents

- [Endpoint and headers](#endpoint-and-headers)
- [Claude models](#claude-models)
- [Request body](#request-body)
- [Message model](#message-model)
- [Content blocks](#content-blocks)
- [Tools and tool results](#tools-and-tool-results)
- [Thinking](#thinking)
- [Adaptive thinking](#adaptive-thinking)
- [Manual extended thinking](#manual-extended-thinking)
- [Prompt caching](#prompt-caching)
- [Streaming protocol](#streaming-protocol)
- [Usage semantics](#usage-semantics)
- [Response shape](#response-shape)
- [Project integration notes](#project-integration-notes)

## Endpoint and headers

- Endpoint: `POST /v1/messages`
- Required headers:
  - `x-api-key: <API key>`
  - `anthropic-version: 2023-06-01`
  - `content-type: application/json`
  - `accept: text/event-stream` for streaming

Anthropic also supports beta headers for specific features. Keep those scoped to the feature that requires them.

## Claude models

Model IDs are API strings, not marketing names. Do not infer a model ID by adding dots or spaces to the product name.

Current Claude 4 family notes that matter for this project:

| Model family | Example API model ID pattern | Thinking mode |
| --- | --- | --- |
| Claude Opus 4.7 | `claude-opus-4-7...` | Adaptive thinking |
| Claude Sonnet 4.6 | `claude-sonnet-4-6...` | Adaptive thinking |
| Claude Opus 4.6 | `claude-opus-4-6...` | Adaptive thinking |
| Claude 4 / 4.1 era models | `claude-sonnet-4...`, `claude-opus-4...` | Manual extended thinking when supported |
| Claude 3.x models | `claude-3-...`, `claude-3-5-...` | Model-dependent; many do not support extended thinking |

Project rules:

- Keep the exact configured `model` string when sending requests.
- Do not add thinking fields unless the model config marks `reasoning: true`.
- Treat user-facing names such as "Claude Opus 4.7" as labels; configure the exact model ID in `settings.json`.
- Use `--debug` or `VIBECODING_DEBUG=1` to inspect the final request body when a provider returns a vague 400.

## Request body

Core request fields:

- `model` - required model ID
- `messages` - required conversation list
- `system` - system instructions
- `tools` - tool definitions
- `max_tokens` - required output cap
- `stream` - `true` for SSE
- `thinking` - optional thinking configuration
- `output_config` - optional output configuration used by adaptive thinking and some compatible APIs
- `metadata` - optional request metadata
- `stop_sequences` - optional stop list
- `temperature` - optional sampling control
- `top_p` - optional nucleus sampling
- `top_k` - optional token filtering
- `service_tier` - optional service level selection

Important:

- Anthropic requires `max_tokens`.
- `system` may be a string or an array of content blocks when prompt caching is used.
- Tool use and image support are expressed as content blocks rather than as separate top-level fields.

## Message model

Messages are ordered turns with roles:

- `user`
- `assistant`

Anthropic's current Messages API does not use a `system` role message in the conversation list. System instructions go in the top-level `system` field.

A message's `content` can be:

- a plain string for simple text
- an array of content blocks for multimodal, tool, or reasoning-aware turns

## Content blocks

Common content block types:

- `text`
- `image`
- `thinking`
- `tool_use`
- `tool_result`
- `redacted_thinking`

### `text`

Text blocks contain:

- `type: "text"`
- `text`

### `image`

Image blocks contain:

- `type: "image"`
- `source`

Image `source` is typically:

- `type: "base64"`
- `media_type`
- `data`

### `thinking`

Thinking blocks contain:

- `type: "thinking"`
- `thinking`
- sometimes `signature` in responses

The API can stream thinking deltas when enabled.

### `tool_use`

Tool use blocks contain:

- `type: "tool_use"`
- `id`
- `name`
- `input`

`input` is a JSON object, not a JSON string.

### `tool_result`

Tool result blocks contain:

- `type: "tool_result"`
- `tool_use_id`
- `content`
- `is_error`

`content` can be a string or a content-block array depending on the SDK or wrapper.

## Tools and tool results

Tool definitions use:

- `name`
- `description`
- `input_schema`

`input_schema` is JSON Schema for the tool's input object.

Tool use behavior:

- model emits `tool_use` when it wants the tool to run
- client returns a `tool_result`
- `tool_result.tool_use_id` must match the original tool-use `id`
- preserve the exact input object shape whenever possible
- empty tool inputs should still be represented as `{}` when the endpoint expects an object

Project-specific note:

- this repo forwards tool calls by converting assistant tool-call blocks to Anthropic `tool_use`
- it preserves the tool-call `id`, `name`, and parsed JSON `input`
- tool results are converted back to `tool_result`

## Thinking

Anthropic's thinking parameter family is model-dependent. The request format differs between adaptive thinking and manual extended thinking.

General rules:

- Only send thinking fields for models that support the selected mode.
- Do not send `budget_tokens` to adaptive-thinking models such as Claude Opus 4.7.
- Do not replay thinking blocks without preserving their signatures when the API requires them.
- If a proxy or compatible endpoint has its own thinking format, isolate that behavior behind provider config such as `thinkingFormat`.

Notes:

- not all models or proxies support the same thinking fields
- some compatibility layers accept `thinking: { type: "enabled" }` without `budget_tokens`
- the chosen budget should be aligned with the model's supported range
- vague 400 responses from Anthropic-compatible proxies are often caused by a model/thinking format mismatch

## Adaptive thinking

Claude Opus 4.7 and the Claude 4.6 generation use adaptive thinking. The model decides how many thinking tokens to use based on request complexity. Clients control effort with `output_config.effort`.

Request shape:

```json
{
  "model": "claude-opus-4-7",
  "max_tokens": 8192,
  "messages": [
    { "role": "user", "content": "Analyze this issue." }
  ],
  "thinking": {
    "type": "adaptive",
    "display": "summarized"
  },
  "output_config": {
    "effort": "high"
  }
}
```

Adaptive thinking fields:

- `thinking.type`: use `"adaptive"`
- `thinking.display`: usually `"summarized"` when thinking should be surfaced as summaries
- `output_config.effort`: effort level, commonly mapped from project thinking level

Recommended project mapping:

| Project thinking level | Anthropic adaptive effort |
| --- | --- |
| `minimal` | `low` |
| `low` | `low` |
| `medium` | `medium` |
| `high` | `high` |
| `xhigh` | `xhigh` |

Compatibility rules:

- For `claude-opus-4-7...`, prefer adaptive thinking over manual `budget_tokens`.
- If adaptive thinking causes a provider 400, first verify the exact model ID and whether the endpoint is official Anthropic or a proxy.
- `thinkingFormat: "adaptive"` can be used as an explicit project config override when URL/model auto-detection is not enough.

## Manual extended thinking

Older supported Claude models use manual extended thinking.

Request shape:

```json
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 8192,
  "messages": [
    { "role": "user", "content": "Think through this carefully." }
  ],
  "thinking": {
    "type": "enabled",
    "budget_tokens": 4096
  }
}
```

Manual fields:

- `thinking.type`: use `"enabled"`
- `thinking.budget_tokens`: explicit token budget for thinking

Budget guidance:

- `minimal`: about `1024`
- `low`: about `4096`
- `medium`: about `10240`
- `high`: about `32768`
- `xhigh`: about `65536`

Do not use manual extended thinking for Claude Opus 4.7 unless official docs or the target endpoint explicitly say it supports that mode.

## Prompt caching

Anthropic prompt caching is driven by `cache_control` markers on supported content blocks.

Common cache-control shape:

- `cache_control: { "type": "ephemeral" }`

What it means:

- marked prefix content can be cached
- later requests may reuse that prefix if the cache is warm
- cache markers can appear on `system`, message content blocks, and other supported blocks depending on the API feature set

Important semantics:

- `input_tokens` is the current turn's non-cached input usage
- `cache_read_input_tokens` is prompt input read from cache
- `cache_creation_input_tokens` is prompt input newly written to cache
- cached tokens are input-side tokens, not output-side tokens

Project behavior:

- cache-control markers are passed through only when cache caching is enabled
- official API is treated as cache-capable by default
- proxies that reject array-form `system` content should receive a plain string

## Streaming protocol

Anthropic streams SSE events as JSON payloads in `data:` lines.

Common event sequence:

- `message_start`
- `content_block_start`
- `content_block_delta`
- `content_block_stop`
- `message_delta`
- `message_stop`

Delta types commonly include:

- `text_delta`
- `thinking_delta`
- `input_json_delta`

Event details:

- `message_start` may contain initial usage
- `content_block_start` announces a block and its index/type
- `content_block_delta` carries incremental text, thinking, or JSON fragments
- `content_block_stop` closes the current block
- `message_delta` may carry stop reason and/or additional usage
- `message_stop` ends the logical response

Tool-use streaming:

- `input_json_delta` fragments must be concatenated before parsing
- the final tool JSON may be incomplete until the block closes

## Usage semantics

Usage fields commonly returned by Anthropic:

- `input_tokens`
- `output_tokens`
- `cache_creation_input_tokens`
- `cache_read_input_tokens`

Interpretation:

- `input_tokens` is the prompt input not accounted for by cache read or cache creation fields
- `cache_read_input_tokens` is reused prompt input
- `cache_creation_input_tokens` is newly written cached prompt input
- `output_tokens` is the generated completion token count

The full prompt footprint is the sum of input-side components:

- `input_tokens`
- `cache_read_input_tokens`
- `cache_creation_input_tokens`

Project behavior:

- normalize usage in the provider layer
- compute `TotalTokens` as input-side sum plus output tokens when Anthropic does not provide a separate total
- keep cache stats on the usage object so the UI can display them without re-deriving provider semantics

## Response shape

A streamed response may include objects with fields such as:

- `type`
- `message`
- `index`
- `content_block`
- `delta`
- `usage`

The final message object commonly includes:

- `id`
- `type`
- `role`
- `content`
- `model`
- `stop_reason`
- `stop_sequence`
- `usage`

Stop reasons may include:

- `end_turn`
- `max_tokens`
- `stop_sequence`
- `tool_use`
- `pause_turn`
- `refusal`
- `model_context_window_exceeded`

## Project integration notes

This repository maps project messages to Anthropic Messages API as follows:

- system prompt goes to top-level `system`
- text blocks become `text`
- image blocks become `image`
- thinking blocks become `thinking`
- tool calls become `tool_use`
- tool results become `tool_result`

Compatibility details:

- the provider keeps `cache_control` markers only when enabled
- if the input is an empty tool argument object, the JSON object should still be preserved
- some proxies emit usage in `message_delta` instead of `message_start`
- some proxies do not accept the array form of `system`, so the provider may downgrade to string form
- thinking fields are model-sensitive; only send them when `provider.Model.Reasoning` is true
- Claude Opus 4.7-style IDs should use adaptive thinking and `output_config.effort`, not manual `budget_tokens`
- if users report `API 400` with an empty or nil error type, inspect the debug request body for invalid `thinking`, `output_config`, `system`, or model ID fields

Official docs:

- Messages API reference: https://platform.claude.com/docs/en/api/messages
- Models overview: https://platform.claude.com/docs/en/docs/about-claude/models/overview
- Prompt caching: https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching
- Tool use: https://platform.claude.com/docs/en/docs/build-with-claude/tool-use
- Thinking: https://platform.claude.com/docs/en/docs/build-with-claude/thinking
- Adaptive thinking: https://platform.claude.com/docs/en/build-with-claude/adaptive-thinking
