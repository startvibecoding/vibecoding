# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2026-05-16

### Added
- **Cache hit rate display**: Footer shows cumulative cache hit percentage (highlighted when ≥50%)
- **Per-turn cache info**: Token usage line now includes cache read/write counts per turn
- **Usage.CacheInfo()**: Extracted shared cache display logic to eliminate 3× duplication
- **Proxy compatibility**: Handle proxies that send usage fields in `message_delta` instead of `message_start`
- **OpenAI multi-chunk usage**: Handle OpenAI proxies that split usage across multiple SSE chunks (first-wins per field)
- **Print mode fix**: Fixed missing space before `$` in print-mode token summary line
- **Cache tests**: 37 unit tests for `CacheInfo()`, `formatCachePercent()`, and `renderFooter()` cache section
- **Integration tests**: 12 httptest integration tests for Anthropic and OpenAI SSE cache token parsing

### Changed
- npm package versions now use `v`-prefixed format (e.g. `v0.1.1`)
- Normalized JSON formatting across all npm package.json files

## [0.1.0] - 2026-05-15

### Added
- **MiMo thinking format support**: New `thinkingFormat` config option in provider settings to support Xiaomi MiMo API format
- **OpenAI provider**: Use `thinking: {type: "enabled"}` format for xiaomi endpoints
- **Anthropic provider**: Make `budget_tokens` optional (omit for xiaomi endpoints)
- **URL auto-detect**: Automatic detection of xiaomimimo endpoints when `thinkingFormat` not explicitly set
- **Debug logging**: Enable debug output with `VIBECODING_DEBUG` environment variable

### Changed
- `thinkingFormat` configuration is now passed from config to providers instead of relying solely on URL-based detection
- Anthropic `budget_tokens` changed from required to optional (`*int` with `omitempty`)

## [0.0.9] - 2026-05-15

### Added
- **Image support in tools**: `ReadTool` can now read image files (PNG, JPEG, GIF, WebP) and return base64-encoded data
- **Rich content blocks**: New `ToolResult` struct supports both plain text and rich content (text + images)
- **New tool result types**: `NewTextToolResult()` and `NewImageToolResult()` factory functions
- **New message type**: `NewToolResultMessageWithContents()` for rich content messages
- **Model switching**: `/model <id>` command now allows switching models in interactive mode
- **Enhanced help**: `/help` command shows detailed command descriptions and keyboard shortcuts
- **Image test**: Added `TestReadToolImage` test case for image reading functionality

### Changed
- **Tool interface**: `Execute()` now returns `ToolResult` instead of `string`
- **Context estimation**: Fixed double-counting issue when both `Content` and `Contents` are present
- **Provider message conversion**: Both OpenAI and Anthropic providers now handle rich content blocks in tool results

### Fixed
- Context token estimation now correctly prioritizes `Contents` over `Content` to avoid double-counting
- Image tool results are properly converted to provider-specific formats (base64 data URLs for OpenAI, source blocks for Anthropic)

## [0.0.8] - 2026-05-14

### Added
- Session management with JSONL format
- Context compaction when approaching token limits
- Skills system with global and project-level skills
- Context files support (AGENTS.md, CLAUDE.md, .cursorrules, etc.)
- Background job management (async bash commands)
- Three execution modes: Plan (read-only), Agent (controlled), YOLO (full access)
- Sandbox support via bubblewrap (Linux)
- Multi-provider support (OpenAI, Anthropic, custom providers)
- TUI with BubbleTea, Lipgloss, and Glamour
- CLI with Cobra
- Cross-platform builds (Linux, macOS, Windows)
- npm installer packages

## [0.0.7] - 2026-05-13

### Added
- Initial release
- Basic agent loop
- OpenAI and Anthropic provider support
- File tools (read, write, edit)
- Bash tool
- Grep and find tools
- Basic TUI

## [0.0.6] - 2026-05-12

### Added
- Project scaffolding
- Core architecture design
