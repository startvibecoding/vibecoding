# 更新日志

## v0.0.5

### ✨ 新功能

- **非 root 安装**
  - `install.sh` 现在支持无需 root 或 sudo 权限安装
  - 自动检测可写安装目录：优先使用 `/usr/local/bin`，若不可写则回退到 `~/.vibecoding/bin`
  - 移除所有 `sudo` 调用 — 用户级安装不再需要提升权限

- **自动 PATH 配置**
  - 自动检测用户 shell（bash、zsh、fish）并在相应配置文件中配置 PATH
  - 支持 `.bashrc`、`.bash_profile`、`.zshrc`、`.zshenv`、`config.fish` 和 `.profile`
  - 若 PATH 条目已存在则跳过配置（避免重复）
  - Fish shell 使用 `set -gx PATH` 语法；bash/zsh 使用 `export PATH=...`

### 🛠 改进

- **环境变量**
  - `INSTALL_DIR` — 覆盖安装目录（不变）
  - `AUTO_SETUP_PATH=0` — 禁用自动 PATH 配置
  - 更好的权限问题错误提示

- **安装体验**
  - 开始时显示安装目录和 PATH 自动配置状态
  - 更清晰的彩色状态消息输出

### 📖 文档

- 新增 v0.0.5 发布说明

---

## v0.0.4

### ✨ 新功能

- **Agent 模式审批机制**
  - Agent 模式下执行 bash 命令需要用户审批
  - 支持 `bashWhitelist` 配置，白名单中的命令自动批准
  - 支持 `bashBlacklist` 配置，黑名单中的命令始终需要审批
  - TUI 中显示审批提示，用户输入 `y`/`yes` 或 `n`/`no` 响应
  - 审批请求支持 `abort` 取消

- **模式权限矩阵**
  - Plan 模式: 只读工具 (read, grep, find, ls)
  - Agent 模式: 读写自动执行，bash 需审批
  - YOLO 模式: 所有工具自动执行
  - 更新系统提示词，明确每个模式的权限

### 🛠 改进

- **默认审批白名单**
  - 默认白名单: `go`, `make`, `git`, `npm`, `yarn`, `node`, `python`, `pip`
  - 可在 `settings.json` 中自定义

- **模式切换反馈**
  - 切换模式时显示详细权限说明
  - `/mode` 命令显示当前模式的完整权限列表

### 📖 文档

- 新增审批配置章节
- 更新安全文档，说明审批机制
- 新增 v0.0.4 发布说明

---

## v0.0.3

### ✨ 新功能

- **会话历史加载**
  - 继续或打开会话时显示会话信息（文件路径和消息数量）
  - 在 TUI 中加载并显示历史会话消息
  - 将历史消息加载到 Agent 上下文中以保持连续性
  - 中止时重置 Agent 以确保下次请求状态干净

### 🛠 改进

- **构建与分发系统**
  - 重构 Makefile，按平台划分构建和分发目标
  - 新增 `dist-linux`、`dist-darwin`、`dist-windows` 目标
  - 新增 `build-zip.sh` 用于 Windows zip 打包
  - 新增 `checksums` 目标用于发布校验
  - 更新 `build-deb.sh` 和 `build-tarball.sh` 支持全平台

### 📖 文档

- 文档网站右上角新增 GitHub 仓库跳转按钮
- 新增 v0.0.2 更新日志

---

## v0.0.2

### ✨ 新功能

- **一键安装脚本**
  - `install.sh` 适用于 Linux/macOS，自动从 GitHub Releases 下载
  - `install.ps1` 适用于 Windows PowerShell，支持通过 `VIBECODING_INSTALL_DIR` 自定义安装目录
  - 两个脚本均可自动检测平台/架构、校验完整性并配置 PATH

- **文档站重新设计**
  - 采用 Google Material Design 风格重新设计
  - 默认语言改为英文
  - 新增 Hash 路由，方便文档分享（如 `#/en/README`、`#/zh/configuration`）
  - 头部和 README 新增 Logo

- **品牌素材**
  - 新增 `docs/assets/icon.svg`（512×512）用于打包
  - 新增 `docs/assets/logo.svg`（128×128）用于 README 和小尺寸显示
  - 简洁专业的石板色调设计

- **构建系统**
  - 新增 `make build-windows` 目标（amd64 + arm64）
  - 新增 `make build-linux` 和 `make build-darwin` 目标
  - 更新 `make build-all` 使用平台专用目标

- **文档**
  - 新增 `docs/en/skills.md` 技能系统文档
  - 更新 README 和快速入门中的安装说明

### 🐛 问题修复

- 将素材移至 `docs/assets/` 以支持 GitHub Pages 部署

---

**完整变更日志**: https://github.com/startvibecoding/vibecoding/compare/v0.0.1...v0.0.5
