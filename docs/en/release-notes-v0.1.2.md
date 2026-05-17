# VibeCoding v0.1.2 Release Notes

**Release Date**: May 17, 2026

## Overview

VibeCoding v0.1.2 focuses on performance optimization, TUI improvements, and critical bug fixes. This release introduces prompt cache optimization to reduce API costs and adds markdown syntax highlighting for better readability of LLM responses.

## ✨ New Features

### Prompt Cache Optimization

Implemented intelligent prompt caching based on the LLM_Agent_Cache.md strategy. The system now caches static parts of prompts (system prompts, tool definitions, and context files) across multiple conversation turns. When the LLM provider supports prompt caching (like Anthropic's cache_control), repeated prefixes are served from cache instead of being re-processed, significantly reducing token costs for multi-turn conversations.

### TUI Markdown Syntax Highlighting

Assistant messages in the terminal UI now feature markdown syntax highlighting. Code blocks, headers, bold/italic text, lists, and other markdown formatting are visually distinguished with appropriate colors and styling. This greatly improves the readability of LLM responses, especially for code-heavy outputs.

## 🐛 Bug Fixes

### Security & Correctness

- **Critical Security Fixes**: Resolved multiple critical security issues including race conditions and data integrity problems
- **High/Medium Severity**: Addressed numerous high and medium severity correctness issues across the codebase
- **Dead Code Removal**: Cleaned up dead code paths and improved overall code correctness

### TUI Stability

- **Startup Hang Fix**: Fixed a bug where `clearStdin` would block indefinitely on unsupported stdin types (e.g., when stdin is not a terminal), causing the TUI to hang on startup
- **Rendering Fix**: Fixed assistant message rendering that was broken by ANSI escape codes in prefix checks, ensuring messages display correctly even when they contain terminal escape sequences

## 🛠 Improvements

- **Code Quality**: Addressed remaining medium severity issues across the codebase
- **Dependencies**: Updated npm package versions for consistency

## Upgrading

To upgrade to v0.1.2, use one of the following methods:

### npm (recommended)
```bash
npm install -g vibecoding-installer@latest
```

### Manual Installation
Download the appropriate binary for your platform from the [GitHub Releases](https://github.com/startvibecoding/vibecoding/releases) page.

### From Source
```bash
git pull
make build
sudo make install
```

## What's Next

- Enhanced multi-provider support with automatic failover
- Plugin system for custom tools
- Improved session management with search and tagging
- Performance optimizations for large codebases

---

**Full Changelog**: https://github.com/startvibecoding/vibecoding/compare/v0.1.1...v0.1.2
