#!/usr/bin/env bash
set -euo pipefail

REPO="BurntSushi/ripgrep"
PINNED_TAG="${RIPGREP_TAG:-15.0.0}"
API_URL="https://api.github.com/repos/${REPO}/releases/tags/${PINNED_TAG}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
DEST_ROOT="${PROJECT_ROOT}/pkgs/ripgrep"

log() {
    printf '[ripgrep] %s\n' "$*"
}

warn() {
    printf '[ripgrep] WARN: %s\n' "$*" >&2
}

fail() {
    printf '[ripgrep] ERROR: %s\n' "$*" >&2
    exit 1
}

has_cmd() {
    command -v "$1" >/dev/null 2>&1
}

http_get() {
    local url="$1"
    local dest="$2"

    if has_cmd curl; then
        local args=(
            -fsSL
            -H "Accept: application/vnd.github+json"
        )
        if [[ -n "${GITHUB_TOKEN:-}" ]]; then
            args+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
        fi
        curl "${args[@]}" -o "$dest" "$url"
        return 0
    fi

    if has_cmd wget; then
        local args=(
            --quiet
            --output-document="$dest"
            --header="Accept: application/vnd.github+json"
        )
        if [[ -n "${GITHUB_TOKEN:-}" ]]; then
            args+=(--header="Authorization: Bearer ${GITHUB_TOKEN}")
        fi
        wget "${args[@]}" "$url"
        return 0
    fi

    fail "curl 或 wget 至少需要一个"
}

find_asset_url() {
    local pattern
    local url
    local filename

    for pattern in "$@"; do
        for url in "${ASSET_URLS[@]}"; do
            filename="${url##*/}"
            if printf '%s\n' "$filename" | grep -Eq "$pattern"; then
                printf '%s\n' "$url"
                return 0
            fi
        done
    done

    return 1
}

find_asset_url_by_name() {
    local expected="$1"
    local url

    for url in "${ASSET_URLS[@]}"; do
        if [[ "${url##*/}" == "$expected" ]]; then
            printf '%s\n' "$url"
            return 0
        fi
    done

    return 1
}

download_target() {
    local target="$1"
    local requiredness="$2"
    shift 2

    local url
    local filename
    local dest
    local checksum_url=""
    local checksum_filename=""
    local checksum_dest=""

    if ! url="$(find_asset_url "$@")"; then
        if [[ "$requiredness" == "optional" ]]; then
            warn "release ${TAG_NAME} 中没有找到 ${target} 对应的安装包，已跳过"
            printf '%s\t%s\t%s\t%s\t%s\n' "$target" "MISSING" "" "" "" >> "$MANIFEST_FILE"
            return 0
        fi
        fail "release ${TAG_NAME} 中没有找到 ${target} 对应的安装包"
    fi

    filename="${url##*/}"
    dest="${RELEASE_DIR}/${filename}"

    if [[ -f "$dest" ]]; then
        log "已存在，跳过: ${filename}"
    else
        log "下载 ${target}: ${filename}"
        http_get "$url" "$dest"
    fi

    if checksum_url="$(find_asset_url_by_name "${filename}.sha256")"; then
        checksum_filename="${filename}.sha256"
        checksum_dest="${RELEASE_DIR}/${checksum_filename}"
        if [[ -f "$checksum_dest" ]]; then
            log "校验文件已存在，跳过: ${checksum_filename}"
        else
            log "下载 ${target} 校验文件: ${checksum_filename}"
            http_get "$checksum_url" "$checksum_dest"
        fi
    else
        warn "未找到 ${filename} 对应的 .sha256 文件"
    fi

    printf '%s\t%s\t%s\t%s\t%s\n' "$target" "$filename" "$checksum_filename" "$url" "$checksum_url" >> "$MANIFEST_FILE"
}

mkdir -p "$DEST_ROOT"

TMP_JSON="$(mktemp)"
trap 'rm -f "$TMP_JSON"' EXIT

log "获取 ripgrep 指定 release 信息: ${PINNED_TAG}"
http_get "$API_URL" "$TMP_JSON"

TAG_NAME="$(grep -oE '"tag_name":[[:space:]]*"[^"]+"' "$TMP_JSON" | head -n 1 | sed -E 's/.*"([^"]+)"/\1/')"

if [[ -z "$TAG_NAME" ]]; then
    fail "无法从 GitHub API 响应中解析 tag_name"
fi

mapfile -t ASSET_URLS < <(
    grep -oE '"browser_download_url":[[:space:]]*"[^"]+"' "$TMP_JSON" \
        | sed -E 's/.*"([^"]+)"/\1/'
)

if [[ "${#ASSET_URLS[@]}" -eq 0 ]]; then
    fail "指定 release (${TAG_NAME}) 没有可下载的资源"
fi

RELEASE_DIR="${DEST_ROOT}/${TAG_NAME}"
MANIFEST_FILE="${RELEASE_DIR}/manifest.tsv"

mkdir -p "$RELEASE_DIR"
cp "$TMP_JSON" "${RELEASE_DIR}/release.json"
printf '%s\n' "$TAG_NAME" > "${DEST_ROOT}/LATEST"
printf 'target\tfilename\tchecksum_filename\turl\tchecksum_url\n' > "$MANIFEST_FILE"

# 当前项目支持的系统/架构：
# - linux/amd64
# - linux/arm64
# - linux-musl/amd64
# - linux-musl/arm64
# - darwin/amd64
# - darwin/arm64
# - windows/amd64
# - windows/arm64
#
# ripgrep 默认固定到 15.0.0，以保证项目所需平台覆盖更稳定。
# 如需临时切换，可设置环境变量 RIPGREP_TAG。
# ripgrep 的发布文件名使用 Rust target triple，而不是本项目的命名规则，
# 所以这里使用显式映射。

download_target "linux/amd64" required \
    '^ripgrep_.*_amd64\.deb$' \
    '^ripgrep-.*-x86_64-unknown-linux-gnu\.tar\.gz$'

download_target "linux/arm64" required \
    '^ripgrep-.*-aarch64-unknown-linux-gnu\.tar\.gz$' \
    '^ripgrep_.*_arm64\.deb$'

download_target "linux-musl/amd64" required \
    '^ripgrep-.*-x86_64-unknown-linux-musl\.tar\.gz$'

download_target "linux-musl/arm64" optional \
    '^ripgrep-.*-aarch64-unknown-linux-musl\.tar\.gz$'

download_target "darwin/amd64" required \
    '^ripgrep-.*-x86_64-apple-darwin\.tar\.gz$'

download_target "darwin/arm64" required \
    '^ripgrep-.*-aarch64-apple-darwin\.tar\.gz$'

download_target "windows/amd64" required \
    '^ripgrep-.*-x86_64-pc-windows-msvc\.zip$' \
    '^ripgrep-.*-x86_64-pc-windows-gnu\.zip$'

download_target "windows/arm64" required \
    '^ripgrep-.*-aarch64-pc-windows-msvc\.zip$'

log "下载完成"
log "版本: ${TAG_NAME}"
log "目录: ${RELEASE_DIR}"
log "清单: ${MANIFEST_FILE}"
