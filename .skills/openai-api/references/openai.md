# OpenAI Chat Completions Reference

## Contents

- [Endpoint and headers](#endpoint-and-headers)
- [Request body](#request-body)
- [Message schema](#message-schema)
- [Tools and function calling](#tools-and-function-calling)
- [Streaming](#streaming)
- [Usage and caching](#usage-and-caching)
- [Reasoning-model notes](#reasoning-model-notes)
- [Response fields](#response-fields)
- [Project integration notes](#project-integration-notes)

## Endpoint and headers

- Endpoint: `POST /v1/chat/completions`
- Required header: `Authorization: Bearer <API key>`
- Content type: `application/json`
- Streaming uses SSE with `Accept: text/event-stream`

OpenAI also publishes guidance that Chat Completions is the older text-generation interface and that new projects should prefer Responses. This project still uses Chat Completions for compatibility with existing provider code.

## Request body

Core fields used by this project and adjacent compatibility layers:

- `model` - required model ID
- `messages` - required conversation list
- `tools` - function tools
- `tool_choice` - optional tool selection policy
- `stream` - `true` for SSE
- `stream_options` - extra streaming controls
- `max_tokens` - output cap used by this project
- `temperature` - sampling control
- `top_p` - nucleus sampling
- `presence_penalty` - topic-avoidance penalty
- `frequency_penalty` - repetition penalty
- `stop` - one or more stop sequences
- `n` - number of choices
- `logprobs` - log probability output
- `seed` - best-effort reproducibility hint where supported
- `response_format` - text, JSON mode, or JSON schema
- `reasoning_effort` - reasoning-model control where supported
- `metadata` - arbitrary key/value metadata when supported

Notes:

- OpenAI's newer instruction hierarchy uses `developer` for models that support it; older chat-model flows typically use `system`.
- Some parameters are model-specific. If a proxy or alternate backend rejects a field, keep the request narrowly compatible with the target backend.

## Message schema

`messages` is an ordered array of conversation turns.

Supported roles in the current Chat Completions API:

- `system`
- `developer`
- `user`
- `assistant`
- `tool`

Message content can be either a string or an array of typed content parts depending on the role and model.

Common content part types:

- `text`
- `image_url`
- `input_audio`
- `refusal`

This project currently uses:

- plain text strings for simple turns
- arrays of `text` and `image_url` parts for multimodal user messages
- `tool` messages for tool results
- `reasoning_content` when the backend accepts it

Important behavior:

- For older chat models, `system` remains the usual instruction role.
- For newer reasoning-oriented models, OpenAI documents `developer` as the preferred instruction role.
- Assistant messages may omit `content` when `tool_calls` are present, depending on model behavior and request shape.

## Tools and function calling

Tool definitions use:

- `type: "function"`
- `function.name`
- `function.description`
- `function.parameters` as JSON Schema

Assistant responses may include:

- `tool_calls` array
- each tool call has `id`, `type`, and `function`
- `function.arguments` is a JSON string and must be validated before execution

Deprecated function calling still appears in the API docs as `function_call`, but `tool_calls` is the current interface.

Practical handling:

- accumulate `tool_calls[*].function.arguments` across streamed chunks
- wait until the tool call is complete before parsing JSON
- preserve `id` so the tool result can be matched back to the assistant call

## Streaming

OpenAI streams Chat Completions as SSE `data:` lines.

Common stream behavior:

- chunks arrive as `chat.completion.chunk`
- each chunk contains `choices`
- the final SSE marker is `data: [DONE]`
- `choices` may be empty on the usage-only chunk when `stream_options.include_usage` is enabled

`stream_options.include_usage`:

- when `true`, OpenAI sends an additional chunk before `[DONE]`
- that chunk contains usage for the entire request
- intermediate chunks still include a `usage` field, but it is usually `null`
- if the stream is interrupted, the final usage chunk may never arrive

Chunk deltas may include:

- `delta.role`
- `delta.content`
- `delta.tool_calls`
- `delta.reasoning_content` on compatible models/backends

Finish reasons documented for Chat Completions include:

- `stop`
- `length`
- `content_filter`
- `tool_calls`
- `function_call`, deprecated

## Usage and caching

Response usage fields commonly include:

- `prompt_tokens`
- `completion_tokens`
- `total_tokens`
- `prompt_tokens_details.cached_tokens`

Interpretation:

- `prompt_tokens` is the full prompt token count for the request
- `cached_tokens` is the cached subset of the prompt, not extra output
- `completion_tokens` is the generated output token count
- `total_tokens` is the total billed/request token count

Prompt caching notes:

- OpenAI documents prompt caching as a way to reuse identical prompt prefixes
- `cached_tokens` is reported even when the cached portion is zero
- cache behavior is tied to prompt similarity and retention policy, not to output text

For this project:

- keep cache tokens on the usage object as a subset of prompt input
- do not add `cached_tokens` to completion output
- do not recompute totals in the UI if the provider already returned them

## Reasoning-model notes

Reasoning-related controls in current OpenAI docs include:

- `reasoning_effort`
- supported values vary by model family

The docs note that:

- some models support `none`
- older reasoning models often default to `medium`
- `xhigh` is available only on newer reasoning-capable models

This project maps its internal thinking levels to the closest supported request setting and should keep model-specific compatibility checks in the provider layer.

## Response fields

Typical non-stream response fields:

- `id`
- `object`
- `created`
- `model`
- `choices`
- `usage`
- `service_tier`
- `system_fingerprint`

Choice fields:

- `index`
- `message`
- `finish_reason`
- `logprobs`

Assistant message fields may include:

- `role`
- `content`
- `tool_calls`
- `refusal`
- `function_call` for deprecated compatibility

## Project integration notes

The local provider implementation maps project messages to OpenAI Chat Completions as follows:

- `system` prompt becomes the first system message
- `developer` instructions should only be used when the target backend/model supports them
- text blocks become plain text or text parts
- image blocks become `image_url` content parts
- tool-call blocks become assistant `tool_calls`
- tool results become `tool` role messages
- reasoning/thinking content becomes `reasoning_content` only when supported

Operational edge cases:

- OpenAI tool-call arguments can arrive in fragments during streaming
- tool-call JSON must be assembled before parsing
- the final usage chunk may not arrive on abort
- some proxies accept only a subset of the official OpenAI request shape

Official docs:

- Chat Completions reference: https://platform.openai.com/docs/api-reference/chat/create-chat-completion
- Streaming reference: https://platform.openai.com/docs/api-reference/chat-streaming
- Prompt caching: https://platform.openai.com/docs/guides/prompt-caching
- Reasoning guide: https://platform.openai.com/docs/guides/reasoning
