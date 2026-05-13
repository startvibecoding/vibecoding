## What's New in v0.0.2

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

### 📦 Installation

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/fuckvibecoding/vibecoding/main/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/fuckvibecoding/vibecoding/main/install.ps1 | iex
```

**Go Install:**
```bash
go install github.com/fuckvibecoding/vibecoding/cmd/vibecoding@v0.0.2
```

---

**Full Changelog**: https://github.com/fuckvibecoding/vibecoding/compare/v0.0.1...v0.0.2
