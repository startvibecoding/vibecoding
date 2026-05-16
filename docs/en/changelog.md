# Changelog

## v0.1.1

### ✨ Features

- **Cache Hit Rate Display**
  - Footer now shows cumulative cache hit percentage across all turns
  - Cache percentage is highlighted when hit rate ≥ 50% for quick visibility
  - Per-turn cache read/write counts displayed in token usage line

- **Proxy Compatibility**
  - Handle proxies that send usage fields in `message_delta` instead of `message_start`
  - Handle OpenAI proxies that split usage across multiple SSE chunks (first-wins per field)
  - Fixed missing space before `$` in print-mode token summary line

### 🛠 Improvements

- **Code Quality**
  - Extracted `Usage.CacheInfo()` to eliminate 3× duplicated cache display logic
  - npm package versions now use `v`-prefixed format (e.g. `v0.1.1`)
  - Normalized JSON formatting across all npm package.json files

### 🧪 Testing

- Added 37 unit tests for `CacheInfo()`, `formatCachePercent()`, and `renderFooter()` cache section
- Added 12 httptest integration tests for Anthropic and OpenAI SSE cache token parsing

---

## v0.0.9

### ✨ Features

- **Image Support in Tools**
  - `read` tool now supports reading image files (PNG, JPEG, GIF, WebP)
  - Images are returned as base64-encoded data with MIME type information
  - LLMs can now analyze and understand image content
  - Supported formats: `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`

- **Rich Content Tool Results**
  - New `ToolResult` struct supports both plain text and rich content blocks
  - Tools can now return text + images in a single result
  - New factory functions: `NewTextToolResult()` and `NewImageToolResult()`

- **Model Switching**
  - `/model <id>` command allows switching models in interactive mode
  - `/model` without arguments shows current model and available options
  - Agent resets automatically when model is switched

- **Enhanced Help System**
  - `/help` command now shows detailed command descriptions
  - Added keyboard shortcuts reference (Tab, Esc, Ctrl+O, PgUp/PgDn)

### 🛠 Improvements

- **Context Token Estimation**
  - Fixed double-counting issue when both `Content` and `Contents` are present
  - Image tokens estimated as ~1200 tokens per image

- **Provider Message Conversion**
  - OpenAI: Images in tool results sent as supplementary user messages
  - Anthropic: Images sent as separate user messages alongside tool_result

### 🧪 Testing

- Added `TestReadToolImage` test case for image reading functionality
- All tool tests updated for new `ToolResult` return type

---

## v0.0.8

### ✨ Features

- **NPM Multi-Architecture Split Packages**
  - Split the npm package from a single all-platform bundle (~60MB) into 6 platform-specific packages (~10MB each)
  - Users now only download the binary for their current platform, reducing install size by 83%
  - Uses npm `optionalDependencies` + `os`/`cpu` fields for automatic platform matching
  - Main package `vibecoding-installer` is only ~2KB, links the correct platform package via `postinstall`

### 🛠 Improvements

- **Build System**
  - Added `scripts/build-npm-packages.sh` to generate platform-specific npm packages
  - Added `make npm-packages`, `make npm-pack`, `make npm-publish-all` targets
  - `sync-npm-version.sh` now syncs versions across all platform packages

---

## v0.0.7

### ✨ Features

- **Cross-Platform Sandbox Support**
  - Sandbox now supports macOS and Windows in addition to Linux
  - macOS uses `sandbox-exec` for process isolation
  - Windows uses restricted process creation without network access
  - Platform-specific sandbox implementations selected automatically

- **Repository Rename**
  - Module path renamed to `github.com/startvibecoding/vibecoding`
  - All imports, documentation, and scripts updated accordingly

### 🛠 Improvements

- **Platform-Specific Process Handling**
  - Extracted `SysProcAttr` configuration into build-tagged files (`bash_unix.go`, `bash_windows.go`)
  - Background child process cleanup now works correctly on all platforms
  - `Setpgid` only set on Unix systems; Windows uses `CREATE_NEW_PROCESS_GROUP`

### 📖 Documentation

- Updated all GitHub URLs to new repository location
- Added v0.0.6 and v0.0.7 release notes

---

## v0.0.6

### 🛠 Improvements

- **Bash Tool Reliability**
  - Fixed background child process hanging issue
  - Added `WaitDelay` to prevent shell from waiting indefinitely on background children
  - Properly handle `exec.ErrWaitDelay` errors

- **NPM Installation**
  - Added npm package for installation via `npm install -g vibecoding-installer`
  - Automatic binary download during `postinstall`

### 📖 Documentation

- Added npm installation instructions
- Removed redundant markdown files from docs root
- Added v0.0.5 release notes

---

## v0.0.5

### ✨ Features

- **Non-root Installation**
  - `install.sh` now supports installation without root or sudo
  - Auto-detects writable install directory: uses `/usr/local/bin` if writable, otherwise falls back to `~/.vibecoding/bin`
  - Removes all `sudo` calls — user-level installation never requires elevated privileges

- **Automatic PATH Setup**
  - Auto-detects user's shell (bash, zsh, fish) and configures PATH in the appropriate config file
  - Supports `.bashrc`, `.bash_profile`, `.zshrc`, `.zshenv`, `config.fish`, and `.profile`
  - Skips configuration if PATH entry already exists (no duplicates)
  - Fish shell uses `set -gx PATH` syntax; bash/zsh use `export PATH=...`

### 🛠 Improvements

- **Environment Variables**
  - `INSTALL_DIR` — override the install directory (unchanged)
  - `AUTO_SETUP_PATH=0` — disable automatic PATH configuration
  - Better error messages for permission issues

- **Install Experience**
  - Shows install directory and PATH auto-setup status at the start
  - Cleaner output with colored status messages

### 📖 Documentation

- Added v0.0.5 release notes

---

## v0.0.4

### ✨ Features

- **Agent Mode Approval Mechanism**
  - Bash commands in Agent mode now require user approval
  - Configurable `bashWhitelist` for auto-approved command prefixes
  - Configurable `bashBlacklist` for commands always requiring approval
  - TUI displays approval prompt; user responds with `y`/`yes` or `n`/`no`
  - Approval requests can be cancelled via `abort`

- **Mode Permission Matrix**
  - Plan mode: Read-only tools (read, grep, find, ls)
  - Agent mode: Read/write auto-execute, bash requires approval
  - YOLO mode: All tools auto-execute
  - Updated system prompts with explicit permission matrix

### 🛠 Improvements

- **Default Approval Whitelist**
  - Default whitelist: `go`, `make`, `git`, `npm`, `yarn`, `node`, `python`, `pip`
  - Customizable in `settings.json`

- **Mode Switch Feedback**
  - Mode switching now shows detailed permission descriptions
  - `/mode` command displays full permission list for current mode

### 📖 Documentation

- Added approval configuration section
- Updated security docs with approval mechanism details
- Added v0.0.4 release notes

---

## v0.0.3

### ✨ Features

- **Session History Loading**
  - Display session info (file path and message count) when continuing or opening sessions
  - Load and display historical messages from previous sessions in TUI
  - Load history messages into agent context for continuity
  - Reset agent on abort to ensure clean state for next request

### 🛠 Improvements

- **Build & Distribution System**
  - Restructured Makefile with clear per-platform build and dist targets
  - Added `dist-linux`, `dist-darwin`, `dist-windows` targets
  - Added `build-zip.sh` for Windows zip packages
  - Added `checksums` target for release verification
  - Updated `build-deb.sh` and `build-tarball.sh` to support all platforms

### 📖 Documentation

- Added GitHub repository button in documentation site header
- Added v0.0.2 release notes

---

## v0.0.2

### ✨ Features

- **One-line Installation Scripts**
  - `install.sh` for Linux/macOS - downloads from GitHub Releases automatically
  - `install.ps1` for Windows PowerShell - supports custom install directory via `VIBECODING_INSTALL_DIR`
  - Both scripts detect platform/architecture, verify checksums, and configure PATH

- **Documentation Redesign**
  - Redesigned with Google Material Design style
  - Default language changed to English
  - Added hash routing for easy document sharing (e.g., `#/en/README`, `#/zh/configuration`)
  - Added logo to header and README

- **Brand Assets**
  - Added `docs/assets/icon.svg` (512×512) for packaging
  - Added `docs/assets/logo.svg` (128×128) for README and small displays
  - Minimal, professional design with slate color palette

- **Build System**
  - Added `make build-windows` target (amd64 + arm64)
  - Added `make build-linux` and `make build-darwin` targets
  - Updated `make build-all` to use platform-specific targets

- **Documentation**
  - Added `docs/en/skills.md` for Skills system
  - Updated installation instructions in README and getting-started guides

### 🐛 Bug Fixes

- Moved assets to `docs/assets/` for proper GitHub Pages deployment

---

**Full Changelog**: https://github.com/startvibecoding/vibecoding/compare/v0.0.1...v0.0.7
