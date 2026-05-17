# VibeCoding v0.1.2 发布说明

**发布日期**: 2026年5月17日

## 概述

VibeCoding v0.1.2 专注于性能优化、TUI 改进和关键问题修复。本版本引入了提示缓存优化以降低 API 成本，并添加了 markdown 语法高亮以提升 LLM 响应的可读性。

## ✨ 新功能

### Prompt Cache 优化

实现了基于 LLM_Agent_Cache.md 策略的智能提示缓存。系统现在会跨多轮对话缓存提示的静态部分（系统提示、工具定义和上下文文件）。当 LLM 提供商支持提示缓存（如 Anthropic 的 cache_control）时，重复的前缀将从缓存中提供，而不是重新处理，从而显著降低多轮对话的 token 成本。

### TUI Markdown 语法高亮

终端 UI 中的助手消息现在支持 markdown 语法高亮。代码块、标题、粗体/斜体文本、列表和其他 markdown 格式都会以适当的颜色和样式进行视觉区分。这大大提高了 LLM 响应的可读性，特别是对于代码密集型输出。

## 🐛 问题修复

### 安全与正确性

- **关键安全修复**: 解决了多个关键安全问题，包括竞态条件和数据完整性问题
- **高/中严重性**: 修复了代码库中众多高、中严重性的正确性问题
- **死代码清理**: 清理了死代码路径，提高了整体代码正确性

### TUI 稳定性

- **启动挂起修复**: 修复了在不支持的 stdin 类型上 `clearStdin` 会无限阻塞的错误（例如当 stdin 不是终端时），导致 TUI 启动时挂起
- **渲染修复**: 修复了助手消息渲染被 ANSI 转义码在前缀检查中破坏的问题，确保即使消息包含终端转义序列也能正确显示

## 🛠 改进

- **代码质量**: 修复了代码库中剩余的中等严重性问题
- **依赖更新**: 更新了 npm 包版本以保持一致性

## 升级指南

升级到 v0.1.2，请使用以下方法之一：

### npm（推荐）
```bash
npm install -g vibecoding-installer@latest
```

### 手动安装
从 [GitHub Releases](https://github.com/startvibecoding/vibecoding/releases) 页面下载适合您平台的二进制文件。

### 从源码编译
```bash
git pull
make build
sudo make install
```

## 下一步计划

- 增强多提供商支持，支持自动故障转移
- 自定义工具的插件系统
- 改进会话管理，支持搜索和标签功能
- 针对大型代码库的性能优化

---

**完整变更日志**: https://github.com/startvibecoding/vibecoding/compare/v0.1.1...v0.1.2
