# 更新日志

## v0.1.2

### ✨ 新功能

- **Prompt Cache 优化**
  - 实现了基于 LLM_Agent_Cache.md 策略的提示缓存优化
  - 跨多轮对话缓存系统提示和静态上下文
  - 通过重用缓存 token 减少 API 成本

- **TUI Markdown 语法高亮**
  - TUI 中的助手消息现在支持 markdown 语法高亮
  - 代码块、标题和格式化内容有视觉区分
  - 提升 LLM 响应的可读性

### 🐛 问题修复

- **安全与正确性**
  - 解决了关键的安全、竞态条件和正确性问题
  - 修复了代码库中的高、中严重性正确性问题
  - 移除了死代码，提高了整体代码正确性

- **TUI 稳定性**
  - 修复了在不支持的 stdin 上 `clearStdin` 阻塞导致的 TUI 启动挂起
  - 修复了 ANSI 转义码在前缀检查中导致的 TUI 助手消息渲染损坏

### 🛠 改进

- **代码质量**
  - 修复了代码库中剩余的中等严重性问题
  - 更新了 npm 包版本

---

## v0.1.1

### ✨ 新功能

- **缓存命中率显示**
  - 状态栏现在显示所有轮次的累计缓存命中百分比
  - 缓存命中率 ≥ 50% 时高亮显示，便于快速识别
  - 每轮 token 使用行新增缓存读写数量显示

- **代理兼容性**
  - 支持在 `message_delta` 而非 `message_start` 中发送 usage 字段的代理
  - 支持将 usage 拆分到多个 SSE chunk 的 OpenAI 代理（每个字段取首次出现的值）
  - 修复 print 模式 token 汇总行 `$` 前缺少空格的问题

### 🛠 改进

- **代码质量**
  - 提取 `Usage.CacheInfo()` 消除 3 处重复的缓存显示逻辑
  - npm 包版本号改为 `v` 前缀格式（如 `v0.1.1`）
  - 统一所有 npm package.json 的 JSON 格式

### 🧪 测试

- 新增 37 个单元测试覆盖 `CacheInfo()`、`formatCachePercent()` 和 `renderFooter()` 缓存部分
- 新增 12 个 httptest 集成测试覆盖 Anthropic 和 OpenAI SSE 缓存 token 解析

---

## v0.1.0

### ✨ 新功能

- **小米 MiMo thinking 格式支持**
  - 新增 `thinkingFormat` 配置选项，支持小米 MiMo API 格式
  - OpenAI provider: 小米端点使用 `thinking: {type: "enabled"}` 格式
  - Anthropic provider: 小米端点省略 `budget_tokens`
  - URL 自动检测：未设置 `thinkingFormat` 时自动检测 `xiaomimimo` 端点
  - 调试日志：通过 `VIBECODING_DEBUG` 环境变量启用

### 🛠 改进

- **配置灵活性**
  - `thinkingFormat` 从配置传递到 provider，不再仅依赖 URL 检测
  - Anthropic `budget_tokens` 从必需改为可选（指针类型 + `omitempty`）

---

## v0.0.9

### ✨ 新功能

- **工具图像支持**
  - `read` 工具现在支持读取图像文件（PNG、JPEG、GIF、WebP）
  - 图像以 base64 编码数据和 MIME 类型信息返回
  - LLM 现在可以分析和理解图像内容
  - 支持格式：`.png`、`.jpg`、`.jpeg`、`.gif`、`.webp`

- **富内容工具结果**
  - 新的 `ToolResult` 结构体支持纯文本和富内容块
  - 工具现在可以在单个结果中返回文本 + 图像
  - 新增工厂函数：`NewTextToolResult()` 和 `NewImageToolResult()`

- **模型切换**
  - `/model <id>` 命令允许在交互模式下切换模型
  - `/model` 不带参数显示当前模型和可用选项
  - 切换模型时自动重置 Agent

- **增强的帮助系统**
  - `/help` 命令现在显示详细的命令说明
  - 新增键盘快捷键参考（Tab、Esc、Ctrl+O、PgUp/PgDn）

### 🛠 改进

- **上下文 Token 估算**
  - 修复了同时存在 `Content` 和 `Contents` 时的重复计算问题
  - 图像 token 估算为每张图约 1200 token

- **提供商消息转换**
  - OpenAI：工具结果中的图像作为补充用户消息发送
  - Anthropic：图像作为单独的用户消息与 tool_result 一起发送

### 🧪 测试

- 新增 `TestReadToolImage` 测试用例验证图像读取功能
- 所有工具测试已更新为新的 `ToolResult` 返回类型

---

## v0.0.8

### ✨ 新功能

- **NPM 多架构分包优化**
  - 将 npm 包从单包全平台（~60MB）拆分为 6 个平台独立包（每个 ~10MB）
  - 用户安装时只下载当前平台的二进制文件，体积减少 83%
  - 利用 npm `optionalDependencies` + `os`/`cpu` 字段自动匹配平台
  - 主包 `vibecoding-installer` 仅 ~2KB，通过 `postinstall` 链接正确的平台包

### 🛠 改进

- **构建系统**
  - 新增 `scripts/build-npm-packages.sh` 生成平台独立 npm 包
  - 新增 `make npm-packages`、`make npm-pack`、`make npm-publish-all` 目标
  - `sync-npm-version.sh` 同步更新所有平台包版本

---

## v0.0.7

### ✨ 新功能

- **跨平台沙箱支持**
  - 沙箱现在除 Linux 外还支持 macOS 和 Windows
  - macOS 使用 `sandbox-exec` 进行进程隔离
  - Windows 使用受限进程创建，禁止网络访问
  - 自动选择平台特定的沙箱实现

- **仓库重命名**
  - 模块路径更名为 `github.com/startvibecoding/vibecoding`
  - 所有导入、文档和脚本已同步更新

### 🛠 改进

- **平台特定进程处理**
  - 将 `SysProcAttr` 配置提取到构建标签文件（`bash_unix.go`、`bash_windows.go`）
  - 后台子进程清理现在在所有平台上正常工作
  - `Setpgid` 仅在 Unix 系统上设置；Windows 使用 `CREATE_NEW_PROCESS_GROUP`

### 📖 文档

- 更新所有 GitHub URL 至新仓库地址
- 新增 v0.0.6 和 v0.0.7 发布说明

---

## v0.0.6

### 🛠 改进

- **Bash 工具可靠性**
  - 修复后台子进程挂起问题
  - 添加 `WaitDelay` 防止 shell 无限等待后台子进程
  - 正确处理 `exec.ErrWaitDelay` 错误

- **NPM 安装**
  - 新增 npm 包，支持通过 `npm install -g vibecoding-installer` 安装
  - `postinstall` 时自动下载二进制文件

### 📖 文档

- 新增 npm 安装说明
- 移除 docs 根目录下冗余的 markdown 文件
- 新增 v0.0.5 更新日志

---

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

**完整变更日志**: https://github.com/startvibecoding/vibecoding/compare/v0.0.1...v0.0.7
