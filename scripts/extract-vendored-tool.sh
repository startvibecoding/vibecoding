#!/usr/bin/env bash
set -euo pipefail

# extract-vendored-tool.sh — 从 pkgs/ 目录下的压缩包中提取 rg/fd 二进制到指定路径
#
# 用法: ./scripts/extract-vendored-tool.sh <tool> <os> <arch> <output_path>
# 示例: ./scripts/extract-vendored-tool.sh rg linux amd64 bin/vendored/rg-linux-amd64
#       ./scripts/extract-vendored-tool.sh fd windows amd64 bin/vendored/fd-windows-amd64.exe

TOOL="$1"
OS="$2"
ARCH="$3"
OUTPUT="$4"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

log() {
    printf '[extract] %s\n' "$*"
}

fail() {
    printf '[extract] ERROR: %s\n' "$*" >&2
    exit 1
}

# 根据 tool 名称确定 pkgs 子目录和包内二进制名称
case "$TOOL" in
    rg)
        PKGS_DIR="${PROJECT_ROOT}/pkgs/ripgrep"
        BIN_NAME="rg"
        ;;
    fd)
        PKGS_DIR="${PROJECT_ROOT}/pkgs/fd"
        BIN_NAME="fd"
        ;;
    *)
        fail "未知工具: ${TOOL}（支持 rg、fd）"
        ;;
esac

# 读取最新版本号
LATEST_FILE="${PKGS_DIR}/LATEST"
if [[ ! -f "$LATEST_FILE" ]]; then
    fail "未找到 ${LATEST_FILE}，请先运行 download-${TOOL}.sh"
fi
TAG="$(cat "$LATEST_FILE")"
RELEASE_DIR="${PKGS_DIR}/${TAG}"

if [[ ! -d "$RELEASE_DIR" ]]; then
    fail "release 目录不存在: ${RELEASE_DIR}"
fi

# 根据 os/arch 构建要匹配的文件名模式
# ripgrep: ripgrep-<ver>-<rust-target>.tar.gz / .zip
# fd:      fd-<ver>-<rust-target>.tar.gz / .zip
build_rust_target() {
    local os="$1" arch="$2"
    case "${os}/${arch}" in
        linux/amd64)      echo "x86_64-unknown-linux-gnu" ;;
        linux/arm64)      echo "aarch64-unknown-linux-gnu" ;;
        linux-musl/amd64) echo "x86_64-unknown-linux-musl" ;;
        darwin/amd64)     echo "x86_64-apple-darwin" ;;
        darwin/arm64)     echo "aarch64-apple-darwin" ;;
        windows/amd64)    echo "x86_64-pc-windows-msvc" ;;
        windows/arm64)    echo "aarch64-pc-windows-msvc" ;;
        *)                fail "不支持的目标: ${os}/${arch}" ;;
    esac
}

RUST_TARGET="$(build_rust_target "$OS" "$ARCH")"

# 在 release 目录中查找匹配的压缩包（优先 tar.gz，其次 zip）
find_archive() {
    local dir="$1" tool="$2" target="$3"
    local basename_pattern

    # ripgrep 的版本号没有 v 前缀，fd 的 TAG 有 v 前缀
    case "$tool" in
        rg)
            # ripgrep-15.0.0-x86_64-unknown-linux-gnu.tar.gz
            basename_pattern="ripgrep-*-${target}.tar.gz"
            ;;
        fd)
            # fd-v10.3.0-x86_64-unknown-linux-gnu.tar.gz
            basename_pattern="fd-*-${target}.tar.gz"
            ;;
    esac

    # 尝试 tar.gz
    local match
    match="$(compgen -G "${dir}/${basename_pattern}" 2>/dev/null | head -n 1 || true)"
    if [[ -n "$match" && -f "$match" ]]; then
        printf '%s\n' "$match"
        return 0
    fi

    # 尝试 zip（Windows）
    local zip_pattern="${basename_pattern%.tar.gz}.zip"
    match="$(compgen -G "${dir}/${zip_pattern}" 2>/dev/null | head -n 1 || true)"
    if [[ -n "$match" && -f "$match" ]]; then
        printf '%s\n' "$match"
        return 0
    fi

    return 1
}

ARCHIVE="$(find_archive "$RELEASE_DIR" "$TOOL" "$RUST_TARGET")" || \
    fail "未找到 ${TOOL} 的压缩包 (${OS}/${ARCH}, target=${RUST_TARGET})，目录: ${RELEASE_DIR}"

ARCHIVE_NAME="$(basename "$ARCHIVE")"
log "解压 ${ARCHIVE_NAME} → ${OUTPUT}"

# 创建输出目录
mkdir -p "$(dirname "$OUTPUT")"

# 解压到临时目录，提取二进制文件
TMPDIR_EXTRACT="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_EXTRACT"' EXIT

case "$ARCHIVE_NAME" in
    *.tar.gz)
        tar -xzf "$ARCHIVE" -C "$TMPDIR_EXTRACT"
        ;;
    *.zip)
        unzip -q "$ARCHIVE" -d "$TMPDIR_EXTRACT"
        ;;
    *)
        fail "不支持的归档格式: ${ARCHIVE_NAME}"
        ;;
esac

# 在解压目录中查找二进制文件
# tar.gz 通常包含一个目录，里面是二进制
EXTRACTED_BIN="$(find "$TMPDIR_EXTRACT" -name "$BIN_NAME" -type f | head -n 1)"
if [[ -z "$EXTRACTED_BIN" ]]; then
    # Windows 可能是 .exe
    EXTRACTED_BIN="$(find "$TMPDIR_EXTRACT" -name "${BIN_NAME}.exe" -type f | head -n 1)"
fi

if [[ -z "$EXTRACTED_BIN" ]]; then
    fail "在解压内容中未找到 ${BIN_NAME} 二进制文件"
fi

cp "$EXTRACTED_BIN" "$OUTPUT"
chmod +x "$OUTPUT"

log "已提取: ${OUTPUT} ($(wc -c < "$OUTPUT") bytes)"
