#!/usr/bin/env bash
set -euo pipefail

# prepare-vendored.sh — 从 pkgs/ 压缩包中提取 rg/fd 到 internal/vendored/bin/
# 供 go:embed 使用

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
VENDORED_BIN="${PROJECT_ROOT}/internal/vendored/bin"

log() {
    printf '[prepare-vendored] %s\n' "$*"
}

fail() {
    printf '[prepare-vendored] ERROR: %s\n' "$*" >&2
    exit 1
}

# 提取单个二进制
# 用法: extract_binary <tool> <os-arch> <rust-target>
extract_binary() {
    local tool="$1"
    local os_arch="$2"
    local rust_target="$3"

    local pkgs_dir tool_dir latest archive basename_pattern match

    case "$tool" in
        rg) pkgs_dir="${PROJECT_ROOT}/pkgs/ripgrep" ;;
        fd) pkgs_dir="${PROJECT_ROOT}/pkgs/fd" ;;
        *) fail "未知工具: ${tool}" ;;
    esac

    if [[ ! -f "${pkgs_dir}/LATEST" ]]; then
        fail "未找到 ${pkgs_dir}/LATEST，请先运行 download-${tool}.sh"
    fi

    latest="$(cat "${pkgs_dir}/LATEST")"
    tool_dir="${pkgs_dir}/${latest}"

    # 查找匹配的压缩包
    case "$tool" in
        rg) basename_pattern="ripgrep-*-${rust_target}.tar.gz" ;;
        fd) basename_pattern="fd-*-${rust_target}.tar.gz" ;;
    esac

    match="$(compgen -G "${tool_dir}/${basename_pattern}" 2>/dev/null | head -n 1 || true)"

    if [[ -z "$match" || ! -f "$match" ]]; then
        # 尝试 zip（Windows）
        local zip_pattern="${basename_pattern%.tar.gz}.zip"
        match="$(compgen -G "${tool_dir}/${zip_pattern}" 2>/dev/null | head -n 1 || true)"
    fi

    if [[ -z "$match" || ! -f "$match" ]]; then
        log "WARN: 未找到 ${tool} 的 ${os_arch} 压缩包 (${rust_target})，跳过"
        return 0
    fi

    local dest_dir="${VENDORED_BIN}/${os_arch}"
    local bin_name="$tool"
    local ext=""
    [[ "$os_arch" == windows-* ]] && ext=".exe"
    local dest="${dest_dir}/${bin_name}${ext}"

    mkdir -p "$dest_dir"

    # 解压到临时目录
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' RETURN

    case "$match" in
        *.tar.gz) tar -xzf "$match" -C "$tmpdir" ;;
        *.zip)    unzip -q "$match" -d "$tmpdir" ;;
    esac

    # 查找二进制文件
    local extracted
    extracted="$(find "$tmpdir" -name "${bin_name}" -o -name "${bin_name}.exe" | head -n 1)"

    if [[ -z "$extracted" ]]; then
        log "WARN: 解压后未找到 ${bin_name}，跳过"
        return 0
    fi

    cp "$extracted" "$dest"
    chmod +x "$dest"

    log "已提取: ${os_arch}/${bin_name}${ext} ($(wc -c < "$dest") bytes)"
}

# 主逻辑
log "开始提取 vendored 二进制..."

# 清理旧的 bin 目录
rm -rf "$VENDORED_BIN"
mkdir -p "$VENDORED_BIN"

# Linux
extract_binary rg linux-amd64      x86_64-unknown-linux-musl
extract_binary fd linux-amd64      x86_64-unknown-linux-gnu
extract_binary rg linux-arm64      aarch64-unknown-linux-gnu
extract_binary fd linux-arm64      aarch64-unknown-linux-gnu

# macOS
extract_binary rg darwin-amd64     x86_64-apple-darwin
extract_binary fd darwin-amd64     x86_64-apple-darwin
extract_binary rg darwin-arm64     aarch64-apple-darwin
extract_binary fd darwin-arm64     aarch64-apple-darwin

# Windows
extract_binary rg windows-amd64    x86_64-pc-windows-msvc
extract_binary fd windows-amd64    x86_64-pc-windows-msvc
extract_binary rg windows-arm64    aarch64-pc-windows-msvc
extract_binary fd windows-arm64    aarch64-pc-windows-msvc

log "完成"
log "输出目录: ${VENDORED_BIN}"

# 列出提取结果
echo ""
ls -lhR "$VENDORED_BIN"
